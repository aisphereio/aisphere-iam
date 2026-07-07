package service

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	transport "github.com/aisphereio/kernel/transportx"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IAMDeps struct {
	Login    authn.LoginService
	Logout   authn.LogoutService
	Tokens   authn.TokenService
	Profile  authn.ProfileService
	Identity authn.IdentityAdmin
	Authz    authz.AdminProvider

	InternalCall authn.InternalServiceTokenConfig
}

type IAMAuthService struct {
	v1.UnimplementedIAMAuthServiceServer
	deps IAMDeps
}

func NewIAMAuthService(deps IAMDeps) *IAMAuthService {
	return &IAMAuthService{deps: deps}
}

func (s *IAMAuthService) BuildLoginURL(ctx context.Context, req *v1.BuildLoginURLRequest) (*v1.BuildLoginURLReply, error) {
	if s.deps.Login == nil {
		return nil, authn.ErrIdentityBackendFailed("login provider is not configured", nil)
	}
	out, err := s.deps.Login.BuildLoginURL(ctx, authn.LoginURLRequest{
		RedirectURI: req.GetRedirectUri(),
		State:       req.GetState(),
		Scope:       req.GetScope(),
		OrgID:       req.GetOrgId(),
		AppID:       req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.BuildLoginURLReply{
		LoginUrl:    out.URL,
		Provider:    out.Provider,
		RedirectUri: out.RedirectURI,
		State:       out.State,
		Scope:       out.Scope,
	}, nil
}

func (s *IAMAuthService) BuildLogoutURL(ctx context.Context, req *v1.BuildLogoutURLRequest) (*v1.BuildLogoutURLReply, error) {
	if s.deps.Logout == nil {
		return nil, authn.ErrIdentityBackendFailed("logout provider is not configured", nil)
	}
	out, err := s.deps.Logout.BuildLogoutURL(ctx, authn.LogoutURLRequest{
		PostLogoutRedirectURI: req.GetPostLogoutRedirectUri(),
		IDTokenHint:           req.GetIdTokenHint(),
		State:                 req.GetState(),
		OrgID:                 req.GetOrgId(),
		AppID:                 req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.BuildLogoutURLReply{
		LogoutUrl:             out.URL,
		Provider:              out.Provider,
		PostLogoutRedirectUri: out.PostLogoutRedirectURI,
		State:                 out.State,
	}, nil
}

func (s *IAMAuthService) ExchangeCode(ctx context.Context, req *v1.ExchangeCodeRequest) (*v1.ExchangeCodeReply, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	tokens, principal, err := s.deps.Tokens.ExchangeCode(ctx, authn.AuthCodeExchangeRequest{
		Code:        req.GetCode(),
		State:       req.GetState(),
		RedirectURI: req.GetRedirectUri(),
		OrgID:       req.GetOrgId(),
		AppID:       req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ExchangeCodeReply{Tokens: tokenSetToProto(tokens), Principal: principalToProto(principal)}, nil
}

func (s *IAMAuthService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.TokenSet, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	tokens, err := s.deps.Tokens.RefreshToken(ctx, authn.RefreshTokenRequest{
		RefreshToken: req.GetRefreshToken(),
		Scope:        req.GetScope(),
		OrgID:        req.GetOrgId(),
		AppID:        req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return tokenSetToProto(tokens), nil
}

func (s *IAMAuthService) VerifyToken(ctx context.Context, req *v1.VerifyTokenRequest) (*v1.Principal, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	principal, err := s.deps.Tokens.VerifyToken(ctx, authn.VerifyTokenRequest{
		Token:     req.GetToken(),
		TokenType: req.GetTokenType(),
		OrgID:     req.GetOrgId(),
		AppID:     req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return principalToProto(principal), nil
}

func (s *IAMAuthService) ExternalAuthorize(ctx context.Context, _ *emptypb.Empty) (*v1.Principal, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}
	principal, err := s.deps.Tokens.VerifyToken(ctx, authn.VerifyTokenRequest{Token: token, TokenType: "access_token"})
	if err != nil {
		return nil, err
	}
	if tr, ok := transport.FromServerContext(ctx); ok && tr.ReplyHeader() != nil {
		identity := map[string]string{}
		authn.InjectTrustedHeaders(identity, principal)
		for key, value := range identity {
			tr.ReplyHeader().Set(key, value)
		}
		if cfg := s.deps.InternalCall.Normalized(); cfg.Enabled && cfg.Token() != "" {
			tr.ReplyHeader().Set(cfg.Header(), cfg.Token())
		}
	}
	return principalToProto(principal), nil
}

func (s *IAMAuthService) RevokeToken(ctx context.Context, req *v1.RevokeTokenRequest) (*emptypb.Empty, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	if err := s.deps.Tokens.RevokeToken(ctx, authn.RevokeTokenRequest{
		Token:     req.GetToken(),
		TokenType: req.GetTokenType(),
		OrgID:     req.GetOrgId(),
		AppID:     req.GetAppId(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IAMAuthService) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.GetMeReply, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		if s.deps.Tokens == nil {
			return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
		}
		token, err := bearerTokenFromContext(ctx)
		if err != nil {
			return nil, err
		}
		principal, err = s.deps.Tokens.VerifyToken(ctx, authn.VerifyTokenRequest{Token: token, TokenType: "access_token"})
		if err != nil {
			return nil, err
		}
	}
	reply := &v1.GetMeReply{Principal: principalToProto(principal)}
	if !req.GetIncludeProfile() || s.deps.Profile == nil {
		return reply, nil
	}
	token, err := bearerTokenFromContext(ctx)
	if err != nil {
		// In Gateway API ExternalAuth + trusted-header mode the verified Principal
		// is authoritative enough for GetMe. Full Casdoor profile hydration requires
		// the original bearer token, so return the Principal-only shape when it is
		// unavailable.
		reply.Warnings = append(reply.Warnings, "casdoor profile hydration skipped: bearer token not available")
		return reply, nil
	}
	profile, err := s.deps.Profile.GetIdentityProfile(ctx, authn.IdentityProfileRequest{
		Principal:                 principal,
		Token:                     token,
		IncludeUser:               true,
		IncludeGroups:             true,
		IncludeCurrentApplication: true,
		AllowPartial:              true,
	})
	if err != nil {
		return nil, err
	}
	reply.Principal = principalToProto(profile.Principal)
	reply.User = userToProto(profile.User)
	reply.Groups = groupsToProto(profile.Groups)
	reply.Application = applicationToProto(profile.CurrentApplication)
	reply.Warnings = append([]string(nil), profile.Warnings...)
	return reply, nil
}

func bearerTokenFromContext(ctx context.Context) (string, error) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr.RequestHeader() == nil {
		return "", authn.ErrMissingCredential("authorization bearer token is required")
	}
	cred, ok := authn.BearerCredential(tr.RequestHeader().Get("Authorization"))
	if !ok || cred.Token == "" {
		return "", authn.ErrMissingCredential("authorization bearer token is required")
	}
	return cred.Token, nil
}

type IAMDirectoryService struct {
	v1.UnimplementedIAMDirectoryServiceServer
	deps IAMDeps
}

func NewIAMDirectoryService(deps IAMDeps) *IAMDirectoryService {
	return &IAMDirectoryService{deps: deps}
}

func (s *IAMDirectoryService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.User, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	user, err := s.deps.Identity.GetUser(ctx, req.GetOrgId(), req.GetUserId())
	if err != nil {
		return nil, err
	}
	return userToProto(user), nil
}

func (s *IAMDirectoryService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersReply, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	users, err := s.deps.Identity.FindUsers(ctx, authn.UserFilter{
		OrgID:   req.GetOrgId(),
		GroupID: req.GetGroupId(),
		Role:    req.GetRole(),
		Limit:   int(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListUsersReply{Users: usersToProto(users)}, nil
}

func (s *IAMDirectoryService) GetOrganization(ctx context.Context, req *v1.GetOrganizationRequest) (*v1.Organization, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	org, err := s.deps.Identity.GetOrganization(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	return organizationToProto(org), nil
}

func (s *IAMDirectoryService) ListGroups(ctx context.Context, req *v1.ListGroupsRequest) (*v1.ListGroupsReply, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	groups, err := s.deps.Identity.ListGroups(ctx, authn.GroupFilter{
		OrgID:    req.GetOrgId(),
		ParentID: req.GetParentId(),
		Type:     req.GetType(),
		UserID:   req.GetUserId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListGroupsReply{Groups: groupsToProto(groups)}, nil
}
