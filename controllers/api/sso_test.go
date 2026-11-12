package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/models"
)

func resetSSOForTest(t *testing.T) {
	t.Helper()
	resetEncryptionForTest(t)
	auth.SetActiveOIDCClient(nil)
	t.Cleanup(func() { auth.SetActiveOIDCClient(nil) })
}

func decodeMap(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	m := map[string]interface{}{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("error decoding body: %v (%s)", err, w.Body.String())
	}
	return m
}

func TestSSOEndpointsForbiddenForNonAdmin(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	user := createUnpriviledgedUser(t, models.RoleUser)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/sso"},
		{http.MethodPut, "/api/sso"},
		{http.MethodPost, "/api/sso/test"},
	}
	for _, rt := range routes {
		w := encryptionRequest(t, tc, user.ApiKey, rt.method, rt.path, map[string]string{})
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d", rt.method, rt.path, w.Code)
		}
	}
}

func TestSSOGetDefaults(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/sso", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("expected Cache-Control no-store")
	}
	m := decodeMap(t, w)
	if m["enabled"] != false || m["provider"] != "generic" || m["secret_set"] != false || m["status"] != "disabled" {
		t.Fatalf("unexpected defaults: %v", m)
	}
}

func TestSSOPutRoundTripNeverReturnsSecret(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	srv := discoveryServer(t)
	body := map[string]interface{}{
		"enabled":        true,
		"provider":       "generic",
		"issuer":         srv.URL,
		"client_id":      "cid",
		"client_secret":  "top-secret-value",
		"required_group": "admins",
		"redirect_url":   "http://localhost:3333/auth/oidc/callback",
	}
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPut, "/api/sso", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "top-secret-value") {
		t.Fatalf("PUT response leaks secret")
	}
	m := decodeMap(t, w)
	if m["secret_set"] != true || m["status"] != "enabled" {
		t.Fatalf("unexpected response: %v", m)
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/sso", nil)
	if strings.Contains(w.Body.String(), "top-secret-value") {
		t.Fatalf("GET response leaks secret")
	}
	if m := decodeMap(t, w); m["secret_set"] != true {
		t.Fatalf("expected secret_set true: %v", m)
	}
}

func TestSSOPutInvalid(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPut, "/api/sso", map[string]interface{}{"enabled": true, "issuer": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if m := decodeMap(t, w); !strings.Contains(fmt.Sprint(m["message"]), "issuer") {
		t.Fatalf("expected message to mention issuer: %v", m)
	}
}

func TestSSOMethodNotAllowed(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	for _, rt := range []struct{ method, path string }{
		{http.MethodDelete, "/api/sso"},
		{http.MethodGet, "/api/sso/test"},
	} {
		w := encryptionRequest(t, tc, tc.apiKey, rt.method, rt.path, nil)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: expected 405, got %d", rt.method, rt.path, w.Code)
		}
	}
}

func discoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q}`,
			srv.URL, srv.URL+"/auth", srv.URL+"/token", srv.URL+"/keys")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSSOTestIssuer(t *testing.T) {
	tc := setupTest(t)
	resetSSOForTest(t)
	srv := discoveryServer(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/sso/test", map[string]string{"issuer": srv.URL})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/sso/test", map[string]string{"issuer": "http://127.0.0.1:1"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if m := decodeMap(t, w); m["message"] != "Could not reach the issuer" {
		t.Fatalf("unexpected message: %v", m)
	}
}
