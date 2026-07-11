package authzgrpc

import (
	"context"
	"testing"

	iamv1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestOutgoingPrincipalContextPropagatesTrustedIdentity(t *testing.T) {
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: authn.SubjectTypeUser, Provider: "casdoor", OrgID: "aisphere",
	})
	ctx = outgoingPrincipalContext(ctx, authn.Principal{})
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get("x-aisphere-subject")) != 1 || md.Get("x-aisphere-subject")[0] != "alice" {
		t.Fatalf("trusted principal metadata = %#v", md)
	}
}

func TestOutgoingPrincipalContextUsesServiceIdentityForBackgroundWork(t *testing.T) {
	ctx := outgoingPrincipalContext(context.Background(), authn.Principal{
		SubjectID: "aisphere-hub", SubjectType: authn.SubjectTypeService, Provider: "internal",
	})
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get("x-aisphere-subject")) != 1 || md.Get("x-aisphere-subject")[0] != "aisphere-hub" {
		t.Fatalf("service principal metadata = %#v", md)
	}
}

func TestClientImplementsRuntimeAuthzOverIAM(t *testing.T) {
	raw := &fakePermissionClient{}
	client := NewFromClient(raw)
	ctx := context.Background()

	decision, err := client.Check(ctx, authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: "user", ID: "alice"},
		Resource:   authz.ObjectRef{Type: "skill", ID: "demo"},
		Permission: "edit",
	})
	if err != nil || !decision.IsAllowed() {
		t.Fatalf("Check = (%+v, %v)", decision, err)
	}

	batch, err := client.BatchCheck(ctx, authz.BatchCheckRequest{Checks: []authz.CheckRequest{{
		Subject: authz.SubjectRef{Type: "user", ID: "alice"}, Resource: authz.ObjectRef{Type: "skill", ID: "demo"}, Permission: "view",
	}}})
	if err != nil || len(batch.Decisions) != 1 || !batch.Decisions[0].IsAllowed() {
		t.Fatalf("BatchCheck = (%+v, %v)", batch, err)
	}

	write, err := client.WriteRelationships(ctx, authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "demo"},
		Relation: "owner",
		Subject:  authz.SubjectRef{Type: "user", ID: "alice"},
	})
	if err != nil || write.Written != 1 {
		t.Fatalf("WriteRelationships = (%+v, %v)", write, err)
	}

	rels, err := client.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "skill", ResourceID: "demo"})
	if err != nil || len(rels) != 1 || rels[0].Relation != "owner" {
		t.Fatalf("ReadRelationships = (%+v, %v)", rels, err)
	}
}

type fakePermissionClient struct{}

func (*fakePermissionClient) CheckPermission(context.Context, *iamv1.CheckPermissionRequest, ...grpc.CallOption) (*iamv1.CheckPermissionReply, error) {
	return &iamv1.CheckPermissionReply{Allowed: true, Effect: "allow", Reason: "test", ConsistencyToken: "z1"}, nil
}
func (*fakePermissionClient) BatchCheckPermissions(_ context.Context, in *iamv1.BatchCheckPermissionsRequest, _ ...grpc.CallOption) (*iamv1.BatchCheckPermissionsReply, error) {
	out := make([]*iamv1.CheckPermissionReply, len(in.GetChecks()))
	for i := range out {
		out[i] = &iamv1.CheckPermissionReply{Allowed: true, Effect: "allow"}
	}
	return &iamv1.BatchCheckPermissionsReply{Decisions: out}, nil
}
func (*fakePermissionClient) WriteRelationships(_ context.Context, in *iamv1.WriteRelationshipsRequest, _ ...grpc.CallOption) (*iamv1.WriteRelationshipsReply, error) {
	return &iamv1.WriteRelationshipsReply{Written: int32(len(in.GetRelationships())), ConsistencyToken: "z2"}, nil
}
func (*fakePermissionClient) DeleteRelationships(context.Context, *iamv1.DeleteRelationshipsRequest, ...grpc.CallOption) (*iamv1.DeleteRelationshipsReply, error) {
	return &iamv1.DeleteRelationshipsReply{Deleted: 1}, nil
}
func (*fakePermissionClient) ReadRelationships(context.Context, *iamv1.ListRelationshipsRequest, ...grpc.CallOption) (*iamv1.ListRelationshipsReply, error) {
	return &iamv1.ListRelationshipsReply{Relationships: []*iamv1.Relationship{
		{Resource: &iamv1.ObjectRef{Type: "skill", Id: "demo"}, Relation: "owner", Subject: &iamv1.SubjectRef{Type: "user", Id: "alice"}},
	}}, nil
}
func (*fakePermissionClient) LookupResources(context.Context, *iamv1.LookupResourcesRequest, ...grpc.CallOption) (*iamv1.LookupResourcesReply, error) {
	return &iamv1.LookupResourcesReply{}, nil
}
func (*fakePermissionClient) LookupSubjects(context.Context, *iamv1.LookupSubjectsRequest, ...grpc.CallOption) (*iamv1.LookupSubjectsReply, error) {
	return &iamv1.LookupSubjectsReply{}, nil
}
