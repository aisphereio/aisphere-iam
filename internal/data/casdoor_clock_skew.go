package data

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/casdoor"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
)

const casdoorTokenClockLeeway = 60 * time.Second

var jwtTimeFuncMu sync.Mutex

type casdoorClockSkewProvider struct {
	*casdoor.Client
	sdk *casdoorsdk.Client
}

func newCasdoorClockSkewProvider(cfg casdoor.Config, client *casdoor.Client) *casdoorClockSkewProvider {
	cert := firstNonEmptyString(cfg.JWTCertificate, cfg.Certificate)
	if cert == "" && cfg.JWTCertificateFile != "" {
		if b, err := os.ReadFile(cfg.JWTCertificateFile); err == nil {
			cert = string(b)
		}
	}
	sdk := casdoorsdk.NewClient(
		cfg.Endpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		cert,
		cfg.OrganizationName,
		cfg.ApplicationName,
	)
	return &casdoorClockSkewProvider{Client: client, sdk: sdk}
}

func (p *casdoorClockSkewProvider) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	if strings.TrimSpace(req.Code) == "" {
		return authn.TokenSet{}, authn.Principal{}, authn.ErrInvalidTokenRequest("authorization code is required")
	}
	token, err := p.sdk.GetOAuthToken(req.Code, req.State)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, authn.ErrIdentityBackendFailed("casdoor code exchange failed", err)
	}
	principal, err := p.principalFromAccessToken(ctx, token.AccessToken, req.OrgID, req.AppID)
	if err != nil {
		return authn.TokenSet{}, authn.Principal{}, err
	}
	return tokenSetFromOAuth(token), principal, nil
}

func (p *casdoorClockSkewProvider) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	if strings.TrimSpace(req.Token) == "" {
		return authn.Principal{}, authn.ErrMissingCredential("token is required")
	}
	return p.principalFromAccessToken(ctx, req.Token, req.OrgID, req.AppID)
}

func (p *casdoorClockSkewProvider) principalFromAccessToken(ctx context.Context, token, orgID, appID string) (authn.Principal, error) {
	if err := ctx.Err(); err != nil {
		return authn.Principal{}, err
	}
	claims, err := p.parseJwtTokenWithLeeway(token)
	if err != nil {
		return authn.Principal{}, authn.ErrInvalidCredential("invalid casdoor token")
	}
	principal := principalFromCasdoorClaims(claims, token, orgID, appID)
	if !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrUnauthenticated("casdoor token has no authenticated subject")
	}
	return principal, nil
}

func (p *casdoorClockSkewProvider) parseJwtTokenWithLeeway(token string) (*casdoorsdk.Claims, error) {
	jwtTimeFuncMu.Lock()
	defer jwtTimeFuncMu.Unlock()
	previous := jwt.TimeFunc
	jwt.TimeFunc = func() time.Time { return time.Now().Add(casdoorTokenClockLeeway) }
	defer func() { jwt.TimeFunc = previous }()
	return p.sdk.ParseJwtToken(token)
}

func principalFromCasdoorClaims(claims *casdoorsdk.Claims, accessToken, orgID, appID string) authn.Principal {
	if claims == nil {
		return authn.Anonymous()
	}
	subjectID := firstNonEmptyString(claims.Id, claims.ExternalId, claims.Subject, claims.Name)
	org := firstNonEmptyString(orgID, claims.Owner)
	issuedAt := time.Time{}
	expiresAt := time.Time{}
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return authn.Principal{
		SubjectID:   subjectID,
		SubjectType: authn.SubjectTypeUser,
		Provider:    casdoor.ProviderName,
		ExternalID:  fmt.Sprintf("%s/%s", claims.Owner, claims.Name),
		Issuer:      claims.Issuer,
		Audience:    []string(claims.Audience),
		OrgID:       org,
		AppID:       appID,
		Username:    claims.Name,
		Name:        firstNonEmptyString(claims.DisplayName, claims.Name),
		Email:       claims.Email,
		Phone:       claims.Phone,
		Roles:       casdoorRoleNames(claims.Roles),
		Groups:      append([]string(nil), claims.Groups...),
		AuthMethod:  authn.AuthMethodOIDC,
		Attributes: authn.AttributeSet{
			"casdoor_owner":       claims.Owner,
			"casdoor_name":        claims.Name,
			"casdoor_id":          claims.Id,
			"casdoor_external_id": claims.ExternalId,
			"casdoor_token_type":  claims.TokenType,
			"signin_method":       claims.SigninMethod,
			"access_token":        accessToken,
		},
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}.Normalize()
}

func tokenSetFromOAuth(token *oauth2.Token) authn.TokenSet {
	if token == nil {
		return authn.TokenSet{}
	}
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	raw := authn.AttributeSet{}
	if v := token.Extra("scope"); v != nil {
		raw["scope"] = v
	}
	if v := token.Extra("id_token"); v != nil {
		raw["id_token"] = v
	}
	return authn.TokenSet{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      stringFromOAuthExtra(token.Extra("id_token")),
		TokenType:    tokenType,
		Scope:        stringFromOAuthExtra(token.Extra("scope")),
		ExpiresAt:    token.Expiry,
		Raw:          raw,
	}
}

func casdoorRoleNames(roles []*casdoorsdk.Role) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		if role != nil && role.Name != "" {
			out = append(out, role.Name)
		}
	}
	return out
}

func stringFromOAuthExtra(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return ""
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
