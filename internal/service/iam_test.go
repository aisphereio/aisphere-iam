package service

import (
	"context"
	"testing"
	"time"

	v1 "aisphere-iam/api/iam/v1"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/gatewayx"
)

func TestIAMAuthServiceBuildLoginURLUsesKernelLoginProvider(t *testing.T) {
	svc := NewIAMAuthService(IAMDeps{Login: fakeLoginProvider{}})

	reply, err := svc.BuildLoginURL(context.Background(), &v1.BuildLoginURLRequest{
		RedirectUri: "http://localhost:18000/callback",
		State:       "state-1",
		Scope:       "openid profile",
		OrgId:       "aisphere",
		AppId:       "aisphere",
	})

	if err != nil {
		t.Fatalf("BuildLoginURL returned error: %v", err)
	}
	if reply.LoginUrl != "http://casdoor.example/login?state=state-1" {
		t.Fatalf("login URL = %q", reply.LoginUrl)
	}
	if reply.Provider != "casdoor" || reply.RedirectUri != "http://localhost:18000/callback" {
		t.Fatalf("unexpected login reply: %+v", reply)
	}
}

func TestIAMAuthServiceGetMeLoadsProfileWhenRequested(t *testing.T) {
	svc := NewIAMAuthService(IAMDeps{
		Tokens:  fakeTokenProvider{},
		Profile: fakeProfileProvider{},
	})

	ctx := gatewayx.GRPCServerContextFromDispatch(context.Background(), gatewayx.DispatchRequest{
		Headers: map[string]string{"Authorization": "Bearer access-token"},
	}, gatewayx.RouteMatch{})
	reply, err := svc.GetMe(ctx, &v1.GetMeRequest{IncludeProfile: true})

	if err != nil {
		t.Fatalf("GetMe returned error: %v", err)
	}
	if reply.Principal.SubjectId != "user-1" {
		t.Fatalf("subject id = %q", reply.Principal.SubjectId)
	}
	if reply.User.Username != "alice" {
		t.Fatalf("profile user = %+v", reply.User)
	}
	if len(reply.Groups) != 1 || reply.Groups[0].ParentId != "root" {
		t.Fatalf("groups = %+v", reply.Groups)
	}
}

func TestIAMAuthServiceGetMeRequiresBearerHeader(t *testing.T) {
	svc := NewIAMAuthService(IAMDeps{
		Tokens:  fakeTokenProvider{},
		Profile: fakeProfileProvider{},
	})

	_, err := svc.GetMe(context.Background(), &v1.GetMeRequest{IncludeProfile: true})

	if err == nil {
		t.Fatal("GetMe expected missing credential error")
	}
}

func TestIAMPermissionServiceWritesAndChecksRelationship(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	admin := memoryAuthzAdmin{MemoryRelationshipStore: store}
	svc := NewIAMPermissionService(IAMDeps{Authz: admin})

	_, err := svc.WriteRelationship(context.Background(), &v1.WriteRelationshipRequest{
		Relationship: &v1.Relationship{
			Resource: &v1.ObjectRef{Type: "organization", Id: "aisphere"},
			Relation: "read",
			Subject:  &v1.SubjectRef{Type: "user", Id: "user-1"},
		},
	})
	if err != nil {
		t.Fatalf("WriteRelationship returned error: %v", err)
	}

	reply, err := svc.CheckPermission(context.Background(), &v1.CheckPermissionRequest{
		Subject:    &v1.SubjectRef{Type: "user", Id: "user-1"},
		Resource:   &v1.ObjectRef{Type: "organization", Id: "aisphere"},
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("CheckPermission returned error: %v", err)
	}
	if !reply.Allowed {
		t.Fatalf("permission denied: %+v", reply)
	}
}

type fakeLoginProvider struct{}

func (fakeLoginProvider) BuildLoginURL(ctx context.Context, req authn.LoginURLRequest) (authn.LoginURL, error) {
	_ = ctx
	return authn.LoginURL{
		URL:         "http://casdoor.example/login?state=" + req.State,
		RedirectURI: req.RedirectURI,
		State:       req.State,
		Scope:       req.Scope,
		Provider:    "casdoor",
		OrgID:       req.OrgID,
		AppID:       req.AppID,
	}, nil
}
func (fakeLoginProvider) HandleCallback(context.Context, authn.CallbackRequest) (authn.CallbackResult, error) {
	return authn.CallbackResult{}, nil
}

type fakeTokenProvider struct{}

func (fakeTokenProvider) ExchangeCode(context.Context, authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return authn.TokenSet{}, authn.Principal{}, nil
}
func (fakeTokenProvider) RefreshToken(context.Context, authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, nil
}
func (fakeTokenProvider) VerifyToken(context.Context, authn.VerifyTokenRequest) (authn.Principal, error) {
	return authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
		Provider:    "casdoor",
		OrgID:       "aisphere",
		Username:    "alice",
		IssuedAt:    time.Unix(100, 0),
		ExpiresAt:   time.Unix(3600, 0),
	}, nil
}
func (fakeTokenProvider) RevokeToken(context.Context, authn.RevokeTokenRequest) error { return nil }

type fakeProfileProvider struct{}

func (fakeProfileProvider) GetIdentityProfile(context.Context, authn.IdentityProfileRequest) (authn.IdentityProfile, error) {
	return authn.IdentityProfile{
		Principal: authn.Principal{SubjectID: "user-1", SubjectType: authn.SubjectTypeUser, Username: "alice", OrgID: "aisphere"},
		User:      authn.User{ID: "user-1", Username: "alice", OrgID: "aisphere", Enabled: true},
		Groups:    []authn.Group{{ID: "group1", Name: "group1", OrgID: "aisphere", ParentID: "root"}},
	}, nil
}

type memoryAuthzAdmin struct {
	*authz.MemoryRelationshipStore
}

func (m memoryAuthzAdmin) Check(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	return authz.NewMemoryAuthorizer(m.MemoryRelationshipStore).Check(ctx, req)
}
func (m memoryAuthzAdmin) BatchCheck(ctx context.Context, req authz.BatchCheckRequest) (authz.BatchCheckResult, error) {
	out := authz.BatchCheckResult{Decisions: make([]authz.Decision, 0, len(req.Checks))}
	for _, check := range req.Checks {
		decision, err := m.Check(ctx, check)
		if err != nil {
			return authz.BatchCheckResult{}, err
		}
		out.Decisions = append(out.Decisions, decision)
	}
	return out, nil
}
func (m memoryAuthzAdmin) LookupResources(context.Context, authz.LookupResourcesRequest) (authz.LookupResourcesResult, error) {
	return authz.LookupResourcesResult{}, nil
}
func (m memoryAuthzAdmin) LookupSubjects(context.Context, authz.LookupSubjectsRequest) (authz.LookupSubjectsResult, error) {
	return authz.LookupSubjectsResult{}, nil
}
func (m memoryAuthzAdmin) ReadSchema(context.Context) (authz.Schema, error) {
	return authz.Schema{}, nil
}
func (m memoryAuthzAdmin) WriteSchema(context.Context, authz.Schema) error { return nil }
func (m memoryAuthzAdmin) ValidateSchema(context.Context, authz.Schema) error {
	return nil
}
