package dmi

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeGoogle struct {
	mu          sync.Mutex
	srv         *httptest.Server
	claims      []map[string]interface{}
	requests    int
	inserts     int
	labelIDs    []string
	rfc822      []byte
	insertPath  string
	insertQuery string
	labels      string
	tokenStatus int
	tokenBody   string
	insertCode  int
	insertBody  string
	unauth      int // number of Gmail calls answered with 401 before normal behaviour
	tokens      int
	auths       []string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	f := &fakeGoogle{labels: `{"labels":[]}`}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests++
		switch {
		case r.URL.Path == "/token":
			r.ParseForm()
			parts := strings.Split(r.PostForm.Get("assertion"), ".")
			raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var claims map[string]interface{}
			json.Unmarshal(raw, &claims)
			f.claims = append(f.claims, claims)
			f.tokens++
			w.Header().Set("Content-Type", "application/json")
			if f.tokenStatus != 0 {
				w.WriteHeader(f.tokenStatus)
				io.WriteString(w, f.tokenBody)
				return
			}
			io.WriteString(w, fmt.Sprintf(`{"access_token":"tok%d","token_type":"Bearer","expires_in":3600}`, f.tokens))
		case f.unauth > 0:
			f.unauth--
			f.auths = append(f.auths, r.Header.Get("Authorization"))
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"code":401,"message":"Invalid Credentials"}}`)
		case strings.HasSuffix(r.URL.Path, "/labels"):
			io.WriteString(w, f.labels)
		case strings.HasPrefix(r.URL.Path, "/upload/gmail/v1/users/"):
			f.inserts++
			f.insertPath = r.URL.EscapedPath()
			f.insertQuery = r.URL.RawQuery
			if f.insertCode != 0 {
				w.WriteHeader(f.insertCode)
				io.WriteString(w, f.insertBody)
				return
			}
			_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			mr := multipart.NewReader(r.Body, params["boundary"])
			p1, _ := mr.NextPart()
			b1, _ := io.ReadAll(p1)
			var meta struct{ LabelIds []string }
			json.Unmarshal(b1, &meta)
			f.labelIDs = meta.LabelIds
			p2, _ := mr.NextPart()
			f.rfc822, _ = io.ReadAll(p2)
			io.WriteString(w, `{"id":"m1"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func fakeServiceAccount(t *testing.T) string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	b, _ := json.Marshal(map[string]string{
		"type": "service_account", "client_email": "sa@proj.iam.gserviceaccount.com",
		"private_key": pemKey, "private_key_id": "kid1", "client_id": "1234567890",
	})
	return string(b)
}

func (f *fakeGoogle) config(t *testing.T) Config {
	return Config{
		Provider: ProviderGoogle, AuthMethod: AuthCustomerKey, Secret: fakeServiceAccount(t),
		AllowedDomains: []string{"example.com"}, Folder: FolderInbox,
		Endpoints: Endpoints{GoogleToken: f.srv.URL + "/token", Gmail: f.srv.URL},
	}
}

type bytesWriterTo []byte

func (b bytesWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}

func sendGoogle(c Config, to ...string) error {
	d, err := newGoogleDialer(c)
	if err != nil {
		return err
	}
	s, err := d.Dial()
	if err != nil {
		return err
	}
	return s.Send("from@x.com", to, bytesWriterTo("Subject: hi\r\n\r\nbody\r\n"))
}

func TestGoogleInsertInbox(t *testing.T) {
	f := newFakeGoogle(t)
	if err := sendGoogle(f.config(t), "Bob@Example.com"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.labelIDs, ",") != "INBOX,UNREAD" {
		t.Fatalf("labels %v", f.labelIDs)
	}
	if string(f.rfc822) != "Subject: hi\r\n\r\nbody\r\n" {
		t.Fatalf("rfc822 %q", f.rfc822)
	}
	if f.claims[0]["sub"] != "bob@example.com" || f.claims[0]["scope"] != "https://www.googleapis.com/auth/gmail.insert" {
		t.Fatalf("claims %v", f.claims[0])
	}
	if !strings.Contains(f.insertPath, "/users/bob@example.com/messages") && !strings.Contains(f.insertPath, "/users/bob%40example.com/messages") {
		t.Fatalf("path %s", f.insertPath)
	}
	if !strings.Contains(f.insertQuery, "uploadType=multipart") || !strings.Contains(f.insertQuery, "internalDateSource=receivedTime") {
		t.Fatalf("query %s", f.insertQuery)
	}
}

func TestGoogleJunkLabel(t *testing.T) {
	f := newFakeGoogle(t)
	c := f.config(t)
	c.Folder = FolderJunk
	if err := sendGoogle(c, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.labelIDs, ",") != "SPAM,UNREAD" {
		t.Fatalf("labels %v", f.labelIDs)
	}
}

func TestGoogleCustomLabel(t *testing.T) {
	f := newFakeGoogle(t)
	f.labels = `{"labels":[{"id":"Label_9","name":"Training"}]}`
	c := f.config(t)
	c.Folder, c.FolderName = FolderCustom, "Training"
	if err := sendGoogle(c, "carol@example.com"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.labelIDs, ",") != "Label_9,UNREAD" {
		t.Fatalf("labels %v", f.labelIDs)
	}
	if f.claims[0]["scope"] != "https://www.googleapis.com/auth/gmail.insert https://www.googleapis.com/auth/gmail.labels" {
		t.Fatalf("scope %v", f.claims[0]["scope"])
	}
}

func TestGoogleCustomLabelMissing(t *testing.T) {
	f := newFakeGoogle(t)
	c := f.config(t)
	c.Folder, c.FolderName = FolderCustom, "Training"
	err := sendGoogle(c, "dave@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Training") {
		t.Fatalf("err %v", err)
	}
	if f.inserts != 0 {
		t.Fatal("inserted")
	}
}

func TestGoogle429(t *testing.T) {
	f := newFakeGoogle(t)
	f.insertCode, f.insertBody = 429, "slow down"
	err := sendGoogle(f.config(t), "erin@example.com")
	var e *Error
	if !errors.As(err, &e) || !e.Retryable() {
		t.Fatalf("err %v", err)
	}
}

func TestGoogle403(t *testing.T) {
	f := newFakeGoogle(t)
	f.insertCode, f.insertBody = 403, "nope"
	err := sendGoogle(f.config(t), "erin@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "gmail.insert") || !strings.Contains(err.Error(), "1234567890") {
		t.Fatalf("err %v", err)
	}
}

func TestGoogleUnauthorizedClient(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus = 401
	f.tokenBody = `{"error":"unauthorized_client","error_description":"Client is unauthorized"}`
	err := sendGoogle(f.config(t), "frank@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "domain-wide delegation") {
		t.Fatalf("err %v", err)
	}
}

func TestGoogleToken5xxRetryable(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus, f.tokenBody = 503, "down"
	err := sendGoogle(f.config(t), "gina@example.com")
	var e *Error
	if !errors.As(err, &e) || !e.Retryable() {
		t.Fatalf("err %v", err)
	}
}

func TestGoogleRecipientOutsideDomains(t *testing.T) {
	f := newFakeGoogle(t)
	err := sendGoogle(f.config(t), "x@other.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || f.requests != 0 {
		t.Fatalf("err %v requests %d", err, f.requests)
	}
	if err := sendGoogle(f.config(t)); err == nil || f.requests != 0 {
		t.Fatal("empty to must fail without requests")
	}
}

func TestGoogleTestConnection(t *testing.T) {
	f := newFakeGoogle(t)
	if err := testGoogle(t.Context(), f.config(t), "hank@example.com"); err != nil {
		t.Fatal(err)
	}
	if f.inserts != 0 || len(f.claims) != 1 || f.claims[0]["sub"] != "hank@example.com" {
		t.Fatalf("inserts %d claims %v", f.inserts, f.claims)
	}
}

func TestGoogle403RateLimitRetryable(t *testing.T) {
	f := newFakeGoogle(t)
	f.insertCode = 403
	f.insertBody = `{"error":{"code":403,"errors":[{"reason":"userRateLimitExceeded"}]}}`
	err := sendGoogle(f.config(t), "erin@example.com")
	var e *Error
	if !errors.As(err, &e) || !e.Retryable() {
		t.Fatalf("err %v", err)
	}
}

func TestGoogleBadPrivateKey(t *testing.T) {
	f := newFakeGoogle(t)
	c := f.config(t)
	c.Secret = `{"client_email":"a@b.c","private_key":"garbage","client_id":"1"}`
	err := sendGoogle(c, "x@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || f.requests != 0 {
		t.Fatalf("err %v requests %d", err, f.requests)
	}
	if err := testGoogle(t.Context(), c, "x@example.com"); err == nil || f.requests != 0 {
		t.Fatal("testGoogle must fail without requests")
	}
}

func TestGoogleCustomUnauthorizedNamesLabels(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus = 401
	f.tokenBody = `{"error":"unauthorized_client"}`
	c := f.config(t)
	c.Folder, c.FolderName = FolderCustom, "Training"
	err := sendGoogle(c, "x@example.com")
	if err == nil || !strings.Contains(err.Error(), "gmail.labels") {
		t.Fatalf("err %v", err)
	}
}

func TestGoogleInvalidGrantDescription(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus = 400
	f.tokenBody = `{"error":"invalid_grant","error_description":"Invalid email or User ID"}`
	err := sendGoogle(f.config(t), "x@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Invalid email or User ID") {
		t.Fatalf("err %v", err)
	}
}

func TestGoogle401RefreshesTokenOnce(t *testing.T) {
	f := newFakeGoogle(t)
	f.unauth = 1
	if err := sendGoogle(f.config(t), "hank@example.com"); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 2 || f.inserts != 1 || len(f.auths) != 1 || f.auths[0] != "Bearer tok1" {
		t.Fatalf("tokens %d inserts %d auths %v", f.tokens, f.inserts, f.auths)
	}
}

func TestGoogle401TwiceIsPermanent(t *testing.T) {
	f := newFakeGoogle(t)
	f.unauth = 5
	err := sendGoogle(f.config(t), "hank@example.com")
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err %v", err)
	}
	if f.tokens != 2 || f.unauth != 3 {
		t.Fatalf("tokens %d remaining 401s %d", f.tokens, f.unauth)
	}
}

func TestGoogleCustomLabel401RefreshesToken(t *testing.T) {
	f := newFakeGoogle(t)
	f.labels = `{"labels":[{"id":"Label_1","name":"Phish"}]}`
	f.unauth = 1
	c := f.config(t)
	c.Folder, c.FolderName = FolderCustom, "Phish"
	if err := sendGoogle(c, "hank@example.com"); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 2 || len(f.labelIDs) == 0 || f.labelIDs[0] != "Label_1" {
		t.Fatalf("tokens %d labels %v", f.tokens, f.labelIDs)
	}
}
