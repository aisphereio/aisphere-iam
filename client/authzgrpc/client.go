// Package authzgrpc exposes IAM's permission service as Kernel's
// provider-neutral runtime authorization service.
package authzgrpc

import (
	"context"
	"fmt"
	"strings"

	iamv1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type permissionClient interface {
	CheckPermission(context.Context, *iamv1.CheckPermissionRequest, ...grpc.CallOption) (*iamv1.CheckPermissionReply, error)
	BatchCheckPermissions(context.Context, *iamv1.BatchCheckPermissionsRequest, ...grpc.CallOption) (*iamv1.BatchCheckPermissionsReply, error)
	WriteRelationships(context.Context, *iamv1.WriteRelationshipsRequest, ...grpc.CallOption) (*iamv1.WriteRelationshipsReply, error)
	DeleteRelationships(context.Context, *iamv1.DeleteRelationshipsRequest, ...grpc.CallOption) (*iamv1.DeleteRelationshipsReply, error)
	ReadRelationships(context.Context, *iamv1.ListRelationshipsRequest, ...grpc.CallOption) (*iamv1.ListRelationshipsReply, error)
	LookupResources(context.Context, *iamv1.LookupResourcesRequest, ...grpc.CallOption) (*iamv1.LookupResourcesReply, error)
	LookupSubjects(context.Context, *iamv1.LookupSubjectsRequest, ...grpc.CallOption) (*iamv1.LookupSubjectsReply, error)
}

type Client struct {
	raw  permissionClient
	conn *grpc.ClientConn
}

func New(cfg Config) (*Client, error) {
	cfg = cfg.normalize()
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("iam authz grpc endpoint is required")
	}
	clientCfg := grpcx.DefaultClientConfig("iam-authz")
	clientCfg.Logger = cfg.Logger
	clientCfg.Metrics = cfg.Metrics
	clientCfg.Timeout = cfg.Timeout
	clientCfg.EnableMetrics = cfg.MetricsEnabled
	clientCfg.EnableRetry = cfg.RetryMax > 1
	clientCfg.RetryMax = cfg.RetryMax
	fallbackPrincipal := authn.Principal{SubjectID: cfg.CallerService, SubjectType: authn.SubjectTypeService, Provider: "internal"}
	clientCfg.ExtraUnary = append(clientCfg.ExtraUnary, principalUnaryClientInterceptor(fallbackPrincipal))
	opts := grpcx.DialOptions(clientCfg)
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(cfg.Endpoint, opts...)
	if err != nil {
		return nil, mapError(err)
	}
	return &Client{raw: iamv1.NewIAMPermissionServiceClient(conn), conn: conn}, nil
}

func principalUnaryClientInterceptor(fallback authn.Principal) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(outgoingPrincipalContext(ctx, fallback), method, req, reply, cc, opts...)
	}
}

func outgoingPrincipalContext(ctx context.Context, fallback authn.Principal) context.Context {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		principal = fallback
	}
	if !principal.IsAuthenticated() {
		return ctx
	}
	headers := map[string]string{}
	authn.InjectTrustedHeaders(headers, principal)
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	for _, name := range authn.TrustedHeaderNames() {
		md.Delete(strings.ToLower(name))
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			md.Set(strings.ToLower(key), value)
		}
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func NewFromClient(raw permissionClient) *Client { return &Client{raw: raw} }

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Check(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	out, err := c.raw.CheckPermission(ctx, checkRequestToProto(req))
	if err != nil {
		return authz.Decision{}, mapError(err)
	}
	return decisionFromProto(out), nil
}

func (c *Client) BatchCheck(ctx context.Context, req authz.BatchCheckRequest) (authz.BatchCheckResult, error) {
	checks := make([]*iamv1.CheckPermissionRequest, 0, len(req.Checks))
	for _, check := range req.Checks {
		checks = append(checks, checkRequestToProto(check))
	}
	out, err := c.raw.BatchCheckPermissions(ctx, &iamv1.BatchCheckPermissionsRequest{Checks: checks})
	if err != nil {
		return authz.BatchCheckResult{}, mapError(err)
	}
	result := authz.BatchCheckResult{Decisions: make([]authz.Decision, 0, len(out.GetDecisions()))}
	for _, decision := range out.GetDecisions() {
		result.Decisions = append(result.Decisions, decisionFromProto(decision))
	}
	return result, nil
}

func (c *Client) WriteRelationships(ctx context.Context, rels ...authz.Relationship) (authz.WriteResult, error) {
	inputs := make([]*iamv1.Relationship, 0, len(rels))
	for _, rel := range rels {
		inputs = append(inputs, relationshipToProto(rel))
	}
	out, err := c.raw.WriteRelationships(ctx, &iamv1.WriteRelationshipsRequest{Relationships: inputs})
	if err != nil {
		return authz.WriteResult{}, mapError(err)
	}
	return authz.WriteResult{Written: int(out.GetWritten()), ConsistencyToken: out.GetConsistencyToken()}, nil
}

func (c *Client) DeleteRelationships(ctx context.Context, filter authz.RelationshipFilter) (authz.WriteResult, error) {
	out, err := c.raw.DeleteRelationships(ctx, &iamv1.DeleteRelationshipsRequest{Filter: filterToProto(filter)})
	if err != nil {
		return authz.WriteResult{}, mapError(err)
	}
	return authz.WriteResult{Deleted: int(out.GetDeleted()), ConsistencyToken: out.GetConsistencyToken()}, nil
}

func (c *Client) ReadRelationships(ctx context.Context, filter authz.RelationshipFilter) ([]authz.Relationship, error) {
	out, err := c.raw.ReadRelationships(ctx, listRequestToProto(filter))
	if err != nil {
		return nil, mapError(err)
	}
	rels := make([]authz.Relationship, 0, len(out.GetRelationships()))
	for _, rel := range out.GetRelationships() {
		rels = append(rels, relationshipFromProto(rel))
	}
	return rels, nil
}

func (c *Client) LookupResources(ctx context.Context, req authz.LookupResourcesRequest) (authz.LookupResourcesResult, error) {
	out, err := c.raw.LookupResources(ctx, &iamv1.LookupResourcesRequest{Subject: subjectToProto(req.Subject), ResourceType: req.ResourceType, Permission: req.Permission, Limit: int32(req.Limit), Cursor: req.Cursor})
	if err != nil {
		return authz.LookupResourcesResult{}, mapError(err)
	}
	result := authz.LookupResourcesResult{NextCursor: out.GetNextCursor(), ConsistencyToken: out.GetConsistencyToken(), Resources: make([]authz.ObjectRef, 0, len(out.GetResources()))}
	for _, ref := range out.GetResources() {
		result.Resources = append(result.Resources, objectFromProto(ref))
	}
	return result, nil
}

func (c *Client) LookupSubjects(ctx context.Context, req authz.LookupSubjectsRequest) (authz.LookupSubjectsResult, error) {
	out, err := c.raw.LookupSubjects(ctx, &iamv1.LookupSubjectsRequest{Resource: objectToProto(req.Resource), Permission: req.Permission, SubjectType: req.SubjectType, Limit: int32(req.Limit), Cursor: req.Cursor})
	if err != nil {
		return authz.LookupSubjectsResult{}, mapError(err)
	}
	result := authz.LookupSubjectsResult{NextCursor: out.GetNextCursor(), ConsistencyToken: out.GetConsistencyToken(), Subjects: make([]authz.SubjectRef, 0, len(out.GetSubjects()))}
	for _, ref := range out.GetSubjects() {
		result.Subjects = append(result.Subjects, subjectFromProto(ref))
	}
	return result, nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.PermissionDenied:
		return authz.ErrPermissionDenied(status.Convert(err).Message())
	case codes.InvalidArgument:
		return authz.ErrInvalidRequest(status.Convert(err).Message())
	default:
		return authz.ErrBackendFailed("iam authz grpc request failed", err)
	}
}

// Keep the published client compatible with Kernel v0.4.1, which exposes the
// runtime capabilities as granular interfaces. The client intentionally does
// not implement authz.SchemaManager: IAM owns schema publication internally and
// only exposes data-plane authorization operations to business services.
var (
	_ authz.Authorizer         = (*Client)(nil)
	_ authz.BatchAuthorizer    = (*Client)(nil)
	_ authz.ResourceLookup     = (*Client)(nil)
	_ authz.SubjectLookup      = (*Client)(nil)
	_ authz.RelationshipStore  = (*Client)(nil)
)
