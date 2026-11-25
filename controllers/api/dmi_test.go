package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/gophish/gophish/dmi"
	"github.com/gophish/gophish/models"
)

func createDMIProfile(t *testing.T, tc *testContext, name, method string) models.SMTP {
	t.Helper()
	body := map[string]interface{}{
		"name":                name,
		"interface_type":      models.InterfaceMS365DMI,
		"from_address":        "sender@example.com",
		"dmi_auth_method":     method,
		"dmi_tenant_id":       "tenant",
		"dmi_client_id":       "client",
		"dmi_allowed_domains": "example.com",
	}
	if method == dmi.AuthClientSecret {
		body["dmi_secret"] = "super-secret-value"
	}
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/smtp/", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	s := models.SMTP{}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

func dmiPath(s models.SMTP, suffix string) string {
	return fmt.Sprintf("/api/smtp/%d/dmi/%s", s.Id, suffix)
}

func testServiceAccountKey(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "op@project.iam.gserviceaccount.com",
		"client_id":    "1234567890",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	})
	return string(b)
}

func TestDMIProfileGetNeverReturnsSecret(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms secret", dmi.AuthClientSecret)
	for _, path := range []string{"/api/smtp/", fmt.Sprintf("/api/smtp/%d", s.Id)} {
		w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, path, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "super-secret-value") {
			t.Fatalf("%s leaks the secret", path)
		}
		if !regexp.MustCompile(`"dmi_secret_set": ?true`).MatchString(w.Body.String()) {
			t.Fatalf("%s: expected dmi_secret_set true: %s", path, w.Body.String())
		}
	}
}

func TestDMICertificateDownload(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, dmiPath(s, "certificate"), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-x509-ca-cert" {
		t.Errorf("unexpected content type %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="ms cert.cer"` {
		t.Errorf("unexpected disposition %q", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "-----BEGIN CERTIFICATE-----") || strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestDMICertificateDownloadNotFound(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	secret := createDMIProfile(t, tc, "ms secret", dmi.AuthClientSecret)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, dmiPath(secret, "certificate"), nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("non-certificate profile: expected 404, got %d", w.Code)
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/smtp/9999/dmi/certificate", nil)
	if w.Code != http.StatusNotFound || decodeMap(t, w)["message"] != "SMTP not found" {
		t.Errorf("missing profile: got %d %s", w.Code, w.Body.String())
	}
}

func TestDMIRoutesRequireOwnership(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	other := createUnpriviledgedUser(t, models.RoleUser)
	for _, rt := range []struct{ method, suffix string }{
		{http.MethodGet, "certificate"},
		{http.MethodPost, "certificate"},
		{http.MethodPost, "test"},
	} {
		w := encryptionRequest(t, tc, other.ApiKey, rt.method, dmiPath(s, rt.suffix), map[string]string{"mailbox": "a@example.com"})
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: expected 404, got %d", rt.method, rt.suffix, w.Code)
		}
	}
}

func TestDMICertificateFilenameSanitized(t *testing.T) {
	cases := map[string]string{
		`a"b/c\d`:       "abcd.cer",
		"line\r\nbreak": "linebreak.cer",
		`"/\`:           "certificate.cer",
		"  ":            "certificate.cer",
		"Plain Profile": "Plain Profile.cer",
	}
	for in, want := range cases {
		if got := certificateFilename(in); got != want {
			t.Errorf("certificateFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDMICertificateRegenerate(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, dmiPath(s, "certificate"), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := models.SMTP{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Id != s.Id || got.DMICertificate == "" || got.DMICertificate == s.DMICertificate {
		t.Fatalf("expected a new certificate on the same profile")
	}
	if strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Fatalf("response leaks the private key")
	}
	secret := createDMIProfile(t, tc, "ms secret", dmi.AuthClientSecret)
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodPost, dmiPath(secret, "certificate"), nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("non-certificate profile: expected 400, got %d", w.Code)
	}
}

func TestDMITestEndpoint(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, dmiPath(s, "test"), map[string]string{"mailbox": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty mailbox: expected 400, got %d", w.Code)
	}
	// A disallowed domain fails locally, so no provider is contacted.
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodPost, dmiPath(s, "test"), map[string]string{"mailbox": "user@blocked.test"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	m := decodeMap(t, w)
	want := models.TestDMIConnection(s.Id, 1, "user@blocked.test")
	if want == nil {
		t.Fatalf("expected the model to reject the mailbox")
	}
	if m["success"] != false || m["message"] != want.Error() {
		t.Fatalf("expected the provider error to pass through, got %v (want %q)", m, want.Error())
	}
}

func TestDMITestEndpointMethodNotAllowed(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, dmiPath(s, "test"), nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodDelete, dmiPath(s, "certificate"), nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDMISettingsForbiddenForNonAdmin(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	user := createUnpriviledgedUser(t, models.RoleUser)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		w := encryptionRequest(t, tc, user.ApiKey, method, "/api/dmi_settings", map[string]string{})
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", method, w.Code)
		}
	}
}

func TestDMISettingsRoundTripNeverReturnsKey(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/dmi_settings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	m := decodeMap(t, w)
	if m["google_key_set"] != false || m["scope"] != "https://www.googleapis.com/auth/gmail.insert" ||
		m["labels_scope"] != "https://www.googleapis.com/auth/gmail.labels" || m["error"] != "" {
		t.Fatalf("unexpected defaults: %v", m)
	}

	key := testServiceAccountKey(t)
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodPut, "/api/dmi_settings", map[string]string{"google_key": key})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "PRIVATE KEY") || strings.Contains(w.Body.String(), "private_key") {
		t.Fatalf("PUT response leaks the key")
	}
	m = decodeMap(t, w)
	if m["google_key_set"] != true || m["google_client_email"] != "op@project.iam.gserviceaccount.com" || m["google_client_id"] != "1234567890" {
		t.Fatalf("unexpected PUT response: %v", m)
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/dmi_settings", nil)
	if strings.Contains(w.Body.String(), "PRIVATE KEY") || decodeMap(t, w)["google_key_set"] != true {
		t.Fatalf("unexpected GET response: %s", w.Body.String())
	}

	w = encryptionRequest(t, tc, tc.apiKey, http.MethodDelete, "/api/dmi_settings", nil)
	if w.Code != http.StatusOK || decodeMap(t, w)["google_key_set"] != false {
		t.Fatalf("unexpected DELETE response: %d %s", w.Code, w.Body.String())
	}
}

func TestDMISettingsPutInvalid(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	for _, key := range []string{"", "not json"} {
		w := encryptionRequest(t, tc, tc.apiKey, http.MethodPut, "/api/dmi_settings", map[string]string{"google_key": key})
		if w.Code != http.StatusBadRequest {
			t.Errorf("key %q: expected 400, got %d", key, w.Code)
		}
	}
}

func TestDMISettingsMethodNotAllowed(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/dmi_settings", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
