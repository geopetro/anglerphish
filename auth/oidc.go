package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCClientSecretEnv is the environment variable for the OIDC client secret.
const OIDCClientSecretEnv = "GOPHISH_OIDC_CLIENT_SECRET"

const (
	// UsernameFromEmailLocalPart maps firstlast@example.com to username firstlast.
	UsernameFromEmailLocalPart = "local_part"
	// UsernameFromEmailFull uses the entire email as the username.
	UsernameFromEmailFull = "full_email"
)

// ErrOIDCAccessDenied is returned when OIDC authentication fails authorization checks.
var ErrOIDCAccessDenied = errors.New("access denied")

// OIDCConfig is the runtime configuration for OIDC admin login.
type OIDCConfig struct {
	Enabled           bool
	Issuer            string
	ClientID          string
	RedirectURL       string
	RequiredGroup     string
	GroupsClaim       string
	UsernameFromEmail string
}

// OIDCClaims holds identity information extracted from an OIDC token.
type OIDCClaims struct {
	Email  string
	Groups []string
}

// OIDCClient performs OIDC authorization-code login against an identity provider.
type OIDCClient struct {
	config   OIDCConfig
	initOnce sync.Once
	initErr  error
	provider *oidc.Provider
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// ValidateOIDCConfig checks static OIDC settings when OIDC is enabled.
func ValidateOIDCConfig(cfg OIDCConfig) error {
	if !cfg.Enabled {
		return nil
	}
	switch {
	case strings.TrimSpace(cfg.Issuer) == "":
		return errors.New("oidc.issuer is required when oidc is enabled")
	case strings.TrimSpace(cfg.ClientID) == "":
		return errors.New("oidc.client_id is required when oidc is enabled")
	case strings.TrimSpace(cfg.RedirectURL) == "":
		return errors.New("oidc.redirect_url is required when oidc is enabled")
	case strings.TrimSpace(cfg.RequiredGroup) == "":
		return errors.New("oidc.required_group is required when oidc is enabled")
	case os.Getenv(OIDCClientSecretEnv) == "":
		return fmt.Errorf("%s is required when oidc is enabled", OIDCClientSecretEnv)
	}
	return nil
}

// NewOIDCClient creates an OIDC client. Provider discovery is deferred until first use.
func NewOIDCClient(cfg OIDCConfig) (*OIDCClient, error) {
	if err := ValidateOIDCConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	if cfg.UsernameFromEmail == "" {
		cfg.UsernameFromEmail = UsernameFromEmailLocalPart
	}
	return &OIDCClient{config: cfg}, nil
}

func (c *OIDCClient) init(ctx context.Context) error {
	c.initOnce.Do(func() {
		provider, err := oidc.NewProvider(ctx, c.config.Issuer)
		if err != nil {
			c.initErr = err
			return
		}
		c.provider = provider
		c.oauth2 = &oauth2.Config{
			ClientID:     c.config.ClientID,
			ClientSecret: os.Getenv(OIDCClientSecretEnv),
			RedirectURL:  c.config.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}
		c.verifier = provider.Verifier(&oidc.Config{ClientID: c.config.ClientID})
	})
	return c.initErr
}

// AuthCodeURL returns the provider authorization URL for the given OAuth state.
func (c *OIDCClient) AuthCodeURL(state string) (string, error) {
	if err := c.init(context.Background()); err != nil {
		return "", err
	}
	return c.oauth2.AuthCodeURL(state), nil
}

// Exchange verifies the authorization code and returns validated OIDC claims.
func (c *OIDCClient) Exchange(ctx context.Context, code string) (*OIDCClaims, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	oauth2Token, err := c.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("id_token missing from token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	claims, err := parseIDTokenClaims(idToken, c.config.GroupsClaim)
	if err != nil {
		return nil, err
	}

	if len(claims.Groups) == 0 {
		userInfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
		if err == nil {
			userInfoClaims, err := parseUserInfoClaims(userInfo, c.config.GroupsClaim)
			if err == nil {
				if userInfoClaims.Email != "" {
					claims.Email = userInfoClaims.Email
				}
				claims.Groups = userInfoClaims.Groups
			}
		}
	}

	if !HasRequiredGroup(claims.Groups, c.config.RequiredGroup) {
		return nil, ErrOIDCAccessDenied
	}

	email := strings.TrimSpace(claims.Email)
	if email == "" {
		return nil, ErrOIDCAccessDenied
	}

	claims.Email = email
	return claims, nil
}

// UsernameMapping returns the configured email-to-username mapping mode.
func (c *OIDCClient) UsernameMapping() string {
	return c.config.UsernameFromEmail
}

func parseIDTokenClaims(idToken *oidc.IDToken, groupsClaim string) (*OIDCClaims, error) {
	var raw map[string]interface{}
	if err := idToken.Claims(&raw); err != nil {
		return nil, err
	}
	return claimsFromMap(raw, groupsClaim)
}

func parseUserInfoClaims(userInfo *oidc.UserInfo, groupsClaim string) (*OIDCClaims, error) {
	var raw map[string]interface{}
	if err := userInfo.Claims(&raw); err != nil {
		return nil, err
	}
	return claimsFromMap(raw, groupsClaim)
}

func claimsFromMap(raw map[string]interface{}, groupsClaim string) (*OIDCClaims, error) {
	claims := &OIDCClaims{}
	if email, ok := raw["email"].(string); ok {
		claims.Email = email
	}
	claims.Groups = extractGroups(raw[groupsClaim])
	return claims, nil
}

func extractGroups(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		groups := make([]string, 0, len(v))
		for _, item := range v {
			if group, ok := item.(string); ok && group != "" {
				groups = append(groups, group)
			}
		}
		return groups
	default:
		return nil
	}
}

// UsernameFromEmail derives a local username from an email address.
func UsernameFromEmail(email, mode string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New("email is required")
	}
	switch mode {
	case UsernameFromEmailFull:
		return email, nil
	case UsernameFromEmailLocalPart, "":
		parts := strings.SplitN(email, "@", 2)
		if len(parts) != 2 || parts[0] == "" {
			return "", errors.New("invalid email address")
		}
		return parts[0], nil
	default:
		return "", fmt.Errorf("unsupported username_from_email mode: %s", mode)
	}
}

// HasRequiredGroup reports whether required appears in groups.
func HasRequiredGroup(groups []string, required string) bool {
	required = strings.TrimPrefix(strings.TrimSpace(required), "/")
	if required == "" {
		return false
	}
	for _, group := range groups {
		if strings.TrimPrefix(strings.TrimSpace(group), "/") == required {
			return true
		}
	}
	return false
}

// GenerateOAuthState returns a random OAuth state value.
func GenerateOAuthState() string {
	return GenerateSecureKey(16)
}
