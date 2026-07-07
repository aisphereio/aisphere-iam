package service

import (
	"context"
	"errors"
	"strings"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	transport "github.com/aisphereio/kernel/transportx"
	"google.golang.org/protobuf/types/known/emptypb"
)

var externalAuthInternalCall authn.InternalServiceTokenConfig

// ConfigureExternalAuthInternalCall configures the Gateway -> backend trust token
// returned by IAM's ExternalAuth endpoint. The token is injected by Envoy only
// after IAM has authenticated the external caller.
func ConfigureExternalAuthInternalCall(cfg authn.InternalServiceTokenConfig) {
	externalAuthInternalCall = cfg
}

// ExternalAuthorize is the Gateway API / Envoy ExternalAuth endpoint.
//
// It validates the external Authorization bearer token, then returns
// Gateway-controlled trusted identity headers in the response. Envoy must strip
// client-supplied X-Aisphere-* headers before calling this endpoint and only
// forward the headers returned by this endpoint to backend services.
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
		identityHeaders := map[string]string{}
		authn.InjectTrustedHeaders(identityHeaders, principal)
		for key, value := range identityHeaders {
			tr.ReplyHeader().Set(key, value)
		}
if cfg := externalAuthInternalCall.Normalized(); cfg.Enabled && cfg.Token != "" {
					tr.ReplyHeader().Set(cfg.HeaderName, cfg.Token)
			}
		}
		return principalToProto(principal), nil
	}

// bearerTokenFromContext extracts the Bearer token from the Authorization
// header via the kernel transport context.
func bearerTokenFromContext(ctx context.Context) (string, error) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr == nil {
		return "", errors.New("no transport context")
	}
	auth := tr.RequestHeader().Get("Authorization")
	if auth == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid Authorization header format")
	}
	return strings.TrimSpace(parts[1]), nil
}
