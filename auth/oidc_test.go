package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUsernameFromEmailLocalPart(t *testing.T) {
	username, err := UsernameFromEmail("firstlast@example.com", UsernameFromEmailLocalPart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "firstlast" {
		t.Fatalf("expected firstlast, got %s", username)
	}
}

func TestUsernameFromEmailFull(t *testing.T) {
	username, err := UsernameFromEmail("firstlast@example.com", UsernameFromEmailFull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "firstlast@example.com" {
		t.Fatalf("expected full email, got %s", username)
	}
}

func TestUsernameFromEmailInvalid(t *testing.T) {
	_, err := UsernameFromEmail("invalid-email", UsernameFromEmailLocalPart)
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestHasRequiredGroup(t *testing.T) {
	groups := []string{"other-group", "anglerphish-admins"}
	if !HasRequiredGroup(groups, "anglerphish-admins") {
		t.Fatal("expected required group to match")
	}
	if HasRequiredGroup(groups, "missing-group") {
		t.Fatal("expected missing group to fail")
	}
	if HasRequiredGroup(nil, "anglerphish-admins") {
		t.Fatal("expected nil groups to fail")
	}
}

func TestHasRequiredGroupWithLeadingSlash(t *testing.T) {
	groups := []string{"/anglerphish-admins", "other-group"}
	if !HasRequiredGroup(groups, "anglerphish-admins") {
		t.Fatal("expected slash-prefixed token group to match")
	}
	if !HasRequiredGroup(groups, "/anglerphish-admins") {
		t.Fatal("expected slash-prefixed required group to match")
	}
	if HasRequiredGroup(groups, "anglerphish") {
		t.Fatal("expected partial group name to fail")
	}
}

func TestClaimsFromMap(t *testing.T) {
	claims, err := claimsFromMap(map[string]interface{}{
		"email":          "firstlast@example.com",
		"email_verified": true,
		"custom_groups":  []interface{}{"anglerphish-admins"},
	}, "custom_groups")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Email != "firstlast@example.com" {
		t.Fatalf("unexpected email: %s", claims.Email)
	}
	if !claims.EmailVerified {
		t.Fatal("expected email_verified to be true")
	}
	if len(claims.Groups) != 1 || claims.Groups[0] != "anglerphish-admins" {
		t.Fatalf("unexpected groups: %v", claims.Groups)
	}
}

func TestValidateEmailClaims(t *testing.T) {
	tests := []struct {
		name            string
		claims          *OIDCClaims
		allowUnverified bool
		wantErr         error
	}{
		{
			name: "verified email allowed",
			claims: &OIDCClaims{
				Email:         "firstlast@example.com",
				EmailVerified: true,
			},
		},
		{
			name: "unverified email rejected",
			claims: &OIDCClaims{
				Email:         "firstlast@example.com",
				EmailVerified: false,
			},
			wantErr: ErrOIDCAccessDenied,
		},
		{
			name: "missing email_verified rejected",
			claims: &OIDCClaims{
				Email: "firstlast@example.com",
			},
			wantErr: ErrOIDCAccessDenied,
		},
		{
			name:    "missing email rejected",
			claims:  &OIDCClaims{EmailVerified: true},
			wantErr: ErrOIDCAccessDenied,
		},
		{
			name: "unverified email allowed when relaxed",
			claims: &OIDCClaims{
				Email:         "firstlast@example.com",
				EmailVerified: false,
			},
			allowUnverified: true,
		},
		{
			name:            "missing email still rejected when relaxed",
			claims:          &OIDCClaims{},
			allowUnverified: true,
			wantErr:         ErrOIDCAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmailClaims(tt.claims, tt.allowUnverified)
			if err != tt.wantErr {
				t.Fatalf("validateEmailClaims() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaimsFromMapPreferredUsernameFallback(t *testing.T) {
	// Microsoft Entra ID omits the email claim, placing the address in
	// preferred_username; the fallback should surface it as the email.
	claims, err := claimsFromMap(map[string]interface{}{
		"preferred_username": "firstlast@example.com",
		"groups":             []interface{}{"anglerphish-admins"},
	}, "groups")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Email != "firstlast@example.com" {
		t.Fatalf("expected email from preferred_username, got %q", claims.Email)
	}
}

func TestClaimsFromMapEmailPreferredOverPreferredUsername(t *testing.T) {
	// When both are present the explicit email claim wins.
	claims, err := claimsFromMap(map[string]interface{}{
		"email":              "real@example.com",
		"preferred_username": "other@example.com",
	}, "groups")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Email != "real@example.com" {
		t.Fatalf("expected explicit email to win, got %q", claims.Email)
	}
}

func TestValidateOIDCConfigDisabled(t *testing.T) {
	if err := ValidateOIDCConfig(OIDCConfig{}); err != nil {
		t.Fatalf("expected no error when disabled: %v", err)
	}
}

func TestValidateOIDCConfigEnabledMissingFields(t *testing.T) {
	err := ValidateOIDCConfig(OIDCConfig{Enabled: true})
	if err == nil {
		t.Fatal("expected validation error for incomplete config")
	}
}

func TestExtractGroups(t *testing.T) {
	groups := extractGroups([]interface{}{"group-a", "group-b"})
	if len(groups) != 2 || groups[0] != "group-a" {
		t.Fatalf("unexpected groups: %v", groups)
	}
}

func TestParseBoolClaim(t *testing.T) {
	tests := []struct {
		name   string
		value  interface{}
		want   bool
		wantOK bool
	}{
		{name: "bool true", value: true, want: true, wantOK: true},
		{name: "bool false", value: false, want: false, wantOK: true},
		{name: "float one", value: float64(1), want: true, wantOK: true},
		{name: "float zero", value: float64(0), want: false, wantOK: true},
		{name: "string true", value: "true", want: true, wantOK: true},
		{name: "string false", value: "false", want: false, wantOK: true},
		{name: "string zero", value: "0", want: false, wantOK: true},
		{name: "unrecognized string", value: "maybe", want: false, wantOK: false},
		{name: "nil", value: nil, want: false, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseBoolClaim(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("parseBoolClaim() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("parseBoolClaim() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOIDCDiscoveryRetriesAfterFailure(t *testing.T) {
	var attempts atomic.Int32
	server := newTestOIDCIssuer(t, &attempts, failOnceDiscovery)
	defer server.Close()

	client, err := NewOIDCClient(OIDCConfig{
		Enabled:       true,
		Issuer:        server.URL,
		ClientID:      "test-client",
		RedirectURL:   "http://localhost/callback",
		RequiredGroup: "anglerphish-admins",
		ClientSecret:  "super-secret-value",
	})
	if err != nil {
		t.Fatalf("NewOIDCClient: %v", err)
	}

	_, err = client.AuthCodeURL(context.Background(), "state-1")
	if err != ErrOIDCDiscoveryFailed {
		t.Fatalf("first AuthCodeURL error = %v, want ErrOIDCDiscoveryFailed", err)
	}

	authURL, err := client.AuthCodeURL(context.Background(), "state-2")
	if err != nil {
		t.Fatalf("second AuthCodeURL: %v", err)
	}
	if authURL == "" || !strings.Contains(authURL, server.URL) {
		t.Fatalf("unexpected auth URL: %s", authURL)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 discovery attempts, got %d", attempts.Load())
	}
}

func TestOIDCDiscoverySuccessCached(t *testing.T) {
	var attempts atomic.Int32
	server := newTestOIDCIssuer(t, &attempts, alwaysSucceedDiscovery)
	defer server.Close()

	client, err := NewOIDCClient(OIDCConfig{
		Enabled:       true,
		Issuer:        server.URL,
		ClientID:      "test-client",
		RedirectURL:   "http://localhost/callback",
		RequiredGroup: "anglerphish-admins",
		ClientSecret:  "super-secret-value",
	})
	if err != nil {
		t.Fatalf("NewOIDCClient: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := client.AuthCodeURL(context.Background(), fmt.Sprintf("state-%d", i)); err != nil {
			t.Fatalf("AuthCodeURL attempt %d: %v", i, err)
		}
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected discovery to run once, got %d attempts", attempts.Load())
	}
}

func TestOIDCDiscoveryConcurrentInit(t *testing.T) {
	var attempts atomic.Int32
	server := newTestOIDCIssuer(t, &attempts, alwaysSucceedDiscovery)
	defer server.Close()

	client, err := NewOIDCClient(OIDCConfig{
		Enabled:       true,
		Issuer:        server.URL,
		ClientID:      "test-client",
		RedirectURL:   "http://localhost/callback",
		RequiredGroup: "anglerphish-admins",
		ClientSecret:  "super-secret-value",
	})
	if err != nil {
		t.Fatalf("NewOIDCClient: %v", err)
	}

	const workers = 20
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			_, err := client.AuthCodeURL(context.Background(), fmt.Sprintf("state-%d", i))
			errs <- err
		}(i)
	}
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent AuthCodeURL: %v", err)
		}
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected one discovery fetch, got %d", attempts.Load())
	}
}

func TestOIDCDiscoveryErrorSanitized(t *testing.T) {
	const secret = "super-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewOIDCClient(OIDCConfig{
		Enabled:       true,
		Issuer:        server.URL,
		ClientID:      "test-client",
		RedirectURL:   "http://localhost/callback",
		RequiredGroup: "anglerphish-admins",
		ClientSecret:  secret,
	})
	if err != nil {
		t.Fatalf("NewOIDCClient: %v", err)
	}

	_, err = client.AuthCodeURL(context.Background(), "state")
	if err == nil {
		t.Fatal("expected discovery failure")
	}
	if err != ErrOIDCDiscoveryFailed {
		t.Fatalf("error = %v, want ErrOIDCDiscoveryFailed", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("discovery error leaked client secret")
	}
	if strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatal("discovery error leaked upstream response body")
	}
}

type discoveryMode int

const (
	alwaysSucceedDiscovery discoveryMode = iota
	failOnceDiscovery
)

func newTestOIDCIssuer(t *testing.T, attempts *atomic.Int32, mode discoveryMode) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		attempt := attempts.Add(1)
		if mode == failOnceDiscovery && attempt == 1 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		issuer := fmt.Sprintf("http://%s", r.Host)
		config := map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/keys",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(config); err != nil {
			t.Errorf("encode discovery document: %v", err)
		}
	}))
}

func TestValidateOIDCConfigRequiresSecret(t *testing.T) {
	err := ValidateOIDCConfig(OIDCConfig{
		Enabled:       true,
		Issuer:        "https://idp.example.com",
		ClientID:      "test-client",
		RedirectURL:   "http://localhost/callback",
		RequiredGroup: "anglerphish-admins",
	})
	if err == nil || !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("expected client secret error, got %v", err)
	}
}

func TestCheckIssuerOK(t *testing.T) {
	var attempts atomic.Int32
	server := newTestOIDCIssuer(t, &attempts, alwaysSucceedDiscovery)
	defer server.Close()
	if err := CheckIssuer(context.Background(), server.URL); err != nil {
		t.Fatalf("CheckIssuer: %v", err)
	}
}

func TestCheckIssuerUnreachable(t *testing.T) {
	err := CheckIssuer(context.Background(), "http://127.0.0.1:1")
	if !errors.Is(err, ErrIssuerUnreachable) {
		t.Fatalf("error = %v, want ErrIssuerUnreachable", err)
	}
}

func TestCheckIssuerNotOIDC(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if err := CheckIssuer(context.Background(), server.URL); !errors.Is(err, ErrIssuerNotOIDC) {
		t.Fatalf("error = %v, want ErrIssuerNotOIDC", err)
	}
	if err := CheckIssuer(context.Background(), "ftp://x"); !errors.Is(err, ErrIssuerNotOIDC) {
		t.Fatalf("ftp error = %v, want ErrIssuerNotOIDC", err)
	}
	if err := CheckIssuer(context.Background(), ""); !errors.Is(err, ErrIssuerNotOIDC) {
		t.Fatalf("empty error = %v, want ErrIssuerNotOIDC", err)
	}
}

func TestActiveOIDCClient(t *testing.T) {
	t.Cleanup(func() { SetActiveOIDCClient(nil) })
	c := &OIDCClient{}
	SetActiveOIDCClient(c)
	if ActiveOIDCClient() != c {
		t.Fatal("expected same client pointer")
	}
	SetActiveOIDCClient(nil)
	if ActiveOIDCClient() != nil {
		t.Fatal("expected nil client")
	}
}
