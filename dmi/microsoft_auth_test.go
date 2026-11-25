package dmi

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

type fakeLogin struct {
	mu     sync.Mutex
	srv    *httptest.Server
	calls  int
	path   string
	form   map[string][]string
	status int
	body   string
}

func newFakeLogin(t *testing.T) *fakeLogin {
	f := &fakeLogin{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		f.path = r.URL.Path
		r.ParseForm()
		f.form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		if f.status != 0 {
			w.WriteHeader(f.status)
			io.WriteString(w, f.body)
			return
		}
		io.WriteString(w, `{"access_token":"ms-tok","token_type":"Bearer","expires_in":3599}`)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func msConfig(f *fakeLogin, id string) Config {
	return Config{Provider: ProviderMicrosoft, AuthMethod: AuthClientSecret, TenantID: "tenant-" + id, ClientID: "client-" + id,
		Secret: "s3cret-" + id, Endpoints: Endpoints{MicrosoftLogin: f.srv.URL}}
}

func TestMSClientSecretToken(t *testing.T) {
	f := newFakeLogin(t)
	ts, err := microsoftTokenSource(msConfig(f, "a"))
	if err != nil {
		t.Fatal(err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "ms-tok" || time.Until(tok.Expiry) < 50*time.Minute {
		t.Fatalf("token %+v", tok)
	}
	if f.path != "/tenant-a/oauth2/v2.0/token" {
		t.Fatalf("path %s", f.path)
	}
	want := map[string]string{"client_id": "client-a", "client_secret": "s3cret-a", "grant_type": "client_credentials", "scope": "https://graph.microsoft.com/.default"}
	for k, v := range want {
		if got := f.form[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %v want %s", k, got, v)
		}
	}
}

func TestMSCertificateAssertion(t *testing.T) {
	f := newFakeLogin(t)
	certPEM, keyPEM, err := GenerateCertificate("dmi-test")
	if err != nil {
		t.Fatal(err)
	}
	c := msConfig(f, "cert")
	c.AuthMethod, c.Secret, c.Certificate = AuthCertificate, keyPEM, certPEM
	ts, err := microsoftTokenSource(c)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "ms-tok" || time.Until(tok.Expiry) < 50*time.Minute {
		t.Fatalf("token %+v", tok)
	}
	for k, v := range map[string]string{"client_id": "client-cert", "grant_type": "client_credentials",
		"scope": "https://graph.microsoft.com/.default", "client_assertion_type": "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"} {
		if got := f.form[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %v want %s", k, got, v)
		}
	}
	if len(f.form["client_secret"]) != 0 {
		t.Error("client_secret must not be sent")
	}
	blk, _ := pem.Decode([]byte(certPEM))
	cert, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.ParseSigned(f.form["client_assertion"][0])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]interface{}
	if err := parsed.Claims(cert.PublicKey.(*rsa.PublicKey), &claims); err != nil {
		t.Fatalf("signature: %v", err)
	}
	h := parsed.Headers[0]
	thumb, _ := CertificateThumbprint(certPEM)
	if h.Algorithm != string(jose.RS256) || h.ExtraHeaders["typ"] != "JWT" || h.ExtraHeaders["x5t"] != thumb {
		t.Fatalf("header %+v", h)
	}
	aud := f.srv.URL + "/tenant-cert/oauth2/v2.0/token"
	if claims["aud"] != aud || claims["iss"] != "client-cert" || claims["sub"] != "client-cert" {
		t.Fatalf("claims %v", claims)
	}
	exp, nbf := int64(claims["exp"].(float64)), int64(claims["nbf"].(float64))
	if d := exp - nbf; d < 590 || d > 610 {
		t.Fatalf("exp-nbf = %d", d)
	}
	if j, _ := claims["jti"].(string); len(j) < 16 {
		t.Fatalf("jti %v", claims["jti"])
	}
}

func TestMSBadCertificateMessage(t *testing.T) {
	f := newFakeLogin(t)
	f.status = 401
	f.body = `{"error":"invalid_client","error_description":"AADSTS700027: Client assertion contains an invalid signature.","error_codes":[700027]}`
	certPEM, keyPEM, _ := GenerateCertificate("dmi-test")
	c := msConfig(f, "bad")
	c.AuthMethod, c.Secret, c.Certificate = AuthCertificate, keyPEM, certPEM
	ts, _ := microsoftTokenSource(c)
	_, err := ts.Token()
	var e *Error
	if !errors.As(err, &e) || e.Retryable() {
		t.Fatalf("want permanent *Error, got %v", err)
	}
	if !strings.Contains(e.Error(), "Microsoft rejected the certificate. Upload the current certificate to the app registration") {
		t.Fatalf("message %q", e.Error())
	}
}

func TestMSErrorMapping(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		want      string
		retryable bool
	}{
		{401, `{"error":"invalid_client","error_description":"AADSTS7000215: bad","error_codes":[7000215]}`, "Microsoft rejected the client secret. Check it is correct and not expired", false},
		{401, `{"error":"invalid_client","error_description":"AADSTS7000222: expired","error_codes":[7000222]}`, "Microsoft rejected the client secret. Check it is correct and not expired", false},
		{401, `{"error":"unauthorized_client","error_description":"AADSTS700016: nope","error_codes":[700016]}`, "Microsoft could not find the app. Check the Client ID and Tenant ID", false},
		{400, `{"error":"invalid_request","error_description":"AADSTS90002: Tenant not found","error_codes":[90002]}`, "Microsoft could not find the tenant. Check the Tenant ID", false},
		{400, `{"error":"invalid_scope","error_description":"AADSTS70011: odd scope","error_codes":[70011]}`, "AADSTS70011: odd scope", false},
		{503, `{"error":"temporarily_unavailable"}`, "", true},
		{429, `{"error":"throttled"}`, "", true},
	}
	for _, tc := range cases {
		f := newFakeLogin(t)
		f.status, f.body = tc.status, tc.body
		ts, _ := microsoftTokenSource(msConfig(f, "m"))
		_, err := ts.Token()
		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("%d: not *Error: %v", tc.status, err)
		}
		if e.Retryable() != tc.retryable || !strings.Contains(e.Error(), tc.want) {
			t.Errorf("%d: retryable=%v msg=%q", tc.status, e.Retryable(), e.Error())
		}
	}
}

func TestMSCertificateSourceReused(t *testing.T) {
	f := newFakeLogin(t)
	certPEM, keyPEM, err := GenerateCertificate("dmi-reuse")
	if err != nil {
		t.Fatal(err)
	}
	c := msConfig(f, "cert-reuse")
	c.AuthMethod, c.Secret, c.Certificate = AuthCertificate, keyPEM, certPEM
	first, err := microsoftTokenSource(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := existingTokenSource(microsoftTokenKey(c)); !ok {
		t.Fatal("certificate source was not cached")
	}
	second, err := microsoftTokenSource(c)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("a cached certificate source must be reused without rebuilding the signer")
	}
}

func TestMSTokenCached(t *testing.T) {
	f := newFakeLogin(t)
	c := msConfig(f, "cache")
	for i := 0; i < 2; i++ {
		ts, err := microsoftTokenSource(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ts.Token(); err != nil {
			t.Fatal(err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("token calls = %d", f.calls)
	}
}
