package dmi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type graphReq struct {
	method, path, query, auth, rng, ct string
	body                               []byte
}

type fakeGraph struct {
	mu      sync.Mutex
	srv     *httptest.Server
	reqs    []graphReq
	login   *fakeLogin
	handler func(w http.ResponseWriter, r *http.Request, n int) bool
	folders string
}

func newFakeGraph(t *testing.T) *fakeGraph {
	f := &fakeGraph{login: newFakeLogin(t), folders: `{"value":[{"id":"AAMk-custom","displayName":"x"}]}`}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.reqs = append(f.reqs, graphReq{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Content-Range"), r.Header.Get("Content-Type"), b})
		if f.handler != nil && f.handler(w, r, len(f.reqs)) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/createUploadSession"):
			io.WriteString(w, `{"uploadUrl":"`+f.srv.URL+`/upload/xyz"}`)
		case strings.HasPrefix(r.URL.Path, "/upload/"):
			w.WriteHeader(200)
		case r.Method == http.MethodPost:
			w.WriteHeader(201)
			io.WriteString(w, `{"id":"MSG1"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/mailFolders"):
			io.WriteString(w, f.folders)
		default:
			io.WriteString(w, `{"id":"x"}`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGraph) config(id string) Config {
	c := msConfig(f.login, id)
	c.AllowedDomains = []string{"contoso.com"}
	c.Endpoints.Graph = f.srv.URL
	return c
}

func sendMS(c Config, raw string) error {
	d, err := NewDialer(c)
	if err != nil {
		return err
	}
	s, err := d.Dial()
	if err != nil {
		return err
	}
	return s.Send("from@x.com", []string{"Alice@Contoso.com"}, bytesWriterTo(raw))
}

const plainMsg = "Subject: hi\r\nFrom: a@b.com\r\nTo: alice@contoso.com\r\n\r\nbody\r\n"

func TestMSInjectInbox(t *testing.T) {
	f := newFakeGraph(t)
	if err := sendMS(f.config("inbox"), plainMsg); err != nil {
		t.Fatal(err)
	}
	r := f.reqs[0]
	if r.method != "POST" || r.path != "/v1.0/users/alice@contoso.com/mailFolders/inbox/messages" || r.auth != "Bearer ms-tok" {
		t.Fatalf("%+v", r)
	}
	if !bytes.Contains(r.body, []byte(`"subject":"hi"`)) {
		t.Fatalf("body %s", r.body)
	}
}

func TestMSInjectJunk(t *testing.T) {
	f := newFakeGraph(t)
	c := f.config("junk")
	c.Folder = FolderJunk
	if err := sendMS(c, plainMsg); err != nil {
		t.Fatal(err)
	}
	if f.reqs[0].path != "/v1.0/users/alice@contoso.com/mailFolders/junkemail/messages" {
		t.Fatalf("%+v", f.reqs[0])
	}
}

func TestMSCustomFolderLookup(t *testing.T) {
	f := newFakeGraph(t)
	c := f.config("custom")
	c.Folder, c.FolderName = FolderCustom, "Bob's Stuff"
	d, _ := NewDialer(c)
	s, _ := d.Dial()
	send := func() error { return s.Send("f", []string{"alice@contoso.com"}, bytesWriterTo(plainMsg)) }
	if err := send(); err != nil {
		t.Fatal(err)
	}
	g := f.reqs[0]
	if g.method != "GET" || g.path != "/v1.0/users/alice@contoso.com/mailFolders" {
		t.Fatalf("%+v", g)
	}
	if got := mustUnescape(t, g.query); got != "$filter=displayName eq 'Bob''s Stuff'" {
		t.Fatalf("query %q", got)
	}
	if f.reqs[1].path != "/v1.0/users/alice@contoso.com/mailFolders/AAMk-custom/messages" {
		t.Fatalf("%+v", f.reqs[1])
	}
	if err := send(); err != nil || len(f.reqs) != 3 {
		t.Fatalf("lookup must be cached: %v %d", err, len(f.reqs))
	}
}

func TestMSCustomFolderMissing(t *testing.T) {
	f := newFakeGraph(t)
	f.folders = `{"value":[]}`
	c := f.config("custommissing")
	c.Folder, c.FolderName = FolderCustom, "Nope"
	err := sendMS(c, plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Nope") {
		t.Fatalf("err %v", err)
	}
}

func largeMsg(size int) string {
	data := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, size))
	var b strings.Builder
	for i := 0; i < len(data); i += 76 {
		b.WriteString(data[i:min(i+76, len(data))] + "\r\n")
	}
	return "Subject: big\r\nFrom: a@b.com\r\nTo: alice@contoso.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n" +
		"--B\r\nContent-Type: text/plain\r\n\r\nhello\r\n" +
		"--B\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"big.bin\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		b.String() + "--B--\r\n"
}

func TestMSLargeAttachmentUpload(t *testing.T) {
	f := newFakeGraph(t)
	size := 4 * 1024 * 1024
	if err := sendMS(f.config("large"), largeMsg(size)); err != nil {
		t.Fatal(err)
	}
	if len(f.reqs) != 4 {
		t.Fatalf("%d requests", len(f.reqs))
	}
	if f.reqs[0].path != "/v1.0/users/alice@contoso.com/mailFolders/inbox/messages" {
		t.Fatalf("%+v", f.reqs[0])
	}
	s := f.reqs[1]
	if s.path != "/v1.0/users/alice@contoso.com/messages/MSG1/attachments/createUploadSession" || s.auth != "Bearer ms-tok" ||
		!strings.Contains(string(s.body), `"size":4194304`) || !strings.Contains(string(s.body), `"name":"big.bin"`) {
		t.Fatalf("%+v %s", s, s.body)
	}
	p1, p2 := f.reqs[2], f.reqs[3]
	if p1.method != "PUT" || p1.path != "/upload/xyz" || p1.auth != "" || p1.rng != "bytes 0-3145727/4194304" || p1.ct != "application/octet-stream" || len(p1.body) != 3*1024*1024 {
		t.Fatalf("%+v", p1.rng)
	}
	if p2.rng != "bytes 3145728-4194303/4194304" || len(p2.body) != 1024*1024 || p2.auth != "" {
		t.Fatalf("%+v", p2.rng)
	}
}

func TestMSUploadFailureDeletesMessage(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if strings.HasPrefix(r.URL.Path, "/upload/") {
			w.WriteHeader(500)
			return true
		}
		return false
	}
	err := sendMS(f.config("uploadfail"), largeMsg(4*1024*1024))
	var e *Error
	if !errors.As(err, &e) || !e.Retryable() {
		t.Fatalf("err %v", err)
	}
	last := f.reqs[len(f.reqs)-1]
	if last.method != "POST" || last.path != "/v1.0/users/alice@contoso.com/messages/MSG1/permanentDelete" {
		t.Fatalf("%+v", last)
	}
}

func TestMS429RetryAfter(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(429)
		return true
	}
	err := sendMS(f.config("429"), plainMsg)
	var e *Error
	if !errors.As(err, &e) || !e.Retryable() || e.RetryAfter.Seconds() != 7 {
		t.Fatalf("err %v", err)
	}
}

func TestMS403Permission(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(403)
		io.WriteString(w, `{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`)
		return true
	}
	err := sendMS(f.config("403"), plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() {
		t.Fatalf("err %v", err)
	}
	for _, s := range []string{"Mail.ReadWrite", "admin consent", "Application Access Policy", "ErrorAccessDenied", "Access is denied."} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("missing %q in %v", s, err)
		}
	}
}

func TestMS404Mailbox(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(404)
		io.WriteString(w, `{"error":{"code":"ErrorInvalidUser","message":"gone"}}`)
		return true
	}
	err := sendMS(f.config("404"), plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Mailbox not found") {
		t.Fatalf("err %v", err)
	}
}

func TestMS401RefreshOnce(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(401)
		return true
	}
	err := sendMS(f.config("401"), plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() {
		t.Fatalf("err %v", err)
	}
	if len(f.reqs) != 2 || f.login.calls != 2 {
		t.Fatalf("graph requests %d token calls %d", len(f.reqs), f.login.calls)
	}
}

func TestMS401RefreshThenSucceeds(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if n == 1 {
			w.WriteHeader(401)
			return true
		}
		return false
	}
	if err := sendMS(f.config("401ok"), plainMsg); err != nil {
		t.Fatal(err)
	}
	if len(f.reqs) != 2 || f.login.calls != 2 {
		t.Fatalf("graph requests %d token calls %d", len(f.reqs), f.login.calls)
	}
}

func TestMSSendRejectsEmptyAndDisallowed(t *testing.T) {
	f := newFakeGraph(t)
	d, _ := NewDialer(f.config("reject"))
	s, _ := d.Dial()
	if err := s.Send("a", nil, bytesWriterTo(plainMsg)); err == nil {
		t.Fatal("empty to")
	}
	if err := s.Send("a", []string{"x@evil.com"}, bytesWriterTo(plainMsg)); err == nil {
		t.Fatal("disallowed domain")
	}
	if len(f.reqs) != 0 || f.login.calls != 0 {
		t.Fatal("no requests expected")
	}
}

func TestMSTestConnection(t *testing.T) {
	f := newFakeGraph(t)
	if err := TestConnection(t.Context(), f.config("tc"), "Alice@contoso.com"); err != nil {
		t.Fatal(err)
	}
	if len(f.reqs) != 1 || f.reqs[0].method != "GET" || f.reqs[0].path != "/v1.0/users/alice@contoso.com/mailFolders/inbox" {
		t.Fatalf("%+v", f.reqs)
	}
}

func TestMSTestConnectionUnknownMailbox(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(404)
		return true
	}
	err := TestConnection(t.Context(), f.config("tc404"), "alice@contoso.com")
	if err == nil || !strings.Contains(err.Error(), "Mailbox not found") {
		t.Fatalf("err %v", err)
	}
}

func TestMSUploadFailureFallsBackToDelete(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if strings.HasPrefix(r.URL.Path, "/upload/") || strings.HasSuffix(r.URL.Path, "/permanentDelete") {
			w.WriteHeader(500)
			return true
		}
		return false
	}
	sendMS(f.config("delfallback"), largeMsg(4*1024*1024))
	last := f.reqs[len(f.reqs)-1]
	if last.method != "DELETE" || last.path != "/v1.0/users/alice@contoso.com/messages/MSG1" {
		t.Fatalf("%+v", last)
	}
}

func twoAttachMsg(big, small int) string {
	enc := func(n int) string {
		data := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, n))
		var b strings.Builder
		for i := 0; i < len(data); i += 76 {
			b.WriteString(data[i:min(i+76, len(data))] + "\r\n")
		}
		return b.String()
	}
	part := func(name string, n int) string {
		return "--B\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"" + name + "\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + enc(n)
	}
	return "Subject: big\r\nFrom: a@b.com\r\nTo: alice@contoso.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n" +
		"--B\r\nContent-Type: text/plain\r\n\r\nhello\r\n" + part("big.bin", big) + part("small.bin", small) + "--B--\r\n"
}

func TestMSSmallSpilledAttachmentUsesDirectPost(t *testing.T) {
	f := newFakeGraph(t)
	if err := sendMS(f.config("smallspill"), twoAttachMsg(3500*1024, 50*1024)); err != nil {
		t.Fatal(err)
	}
	var sessions, posts int
	for _, r := range f.reqs {
		switch {
		case strings.HasSuffix(r.path, "/createUploadSession"):
			sessions++
		case r.method == "POST" && r.path == "/v1.0/users/alice@contoso.com/messages/MSG1/attachments":
			posts++
			if !bytes.Contains(r.body, []byte(`"@odata.type":"#microsoft.graph.fileAttachment"`)) || !bytes.Contains(r.body, []byte(`"name":"small.bin"`)) {
				t.Fatalf("body %.200s", r.body)
			}
		}
	}
	if sessions != 1 || posts != 1 {
		t.Fatalf("sessions %d posts %d", sessions, posts)
	}
}

func TestMSNestedFolderPath(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if r.Method != "GET" {
			return false
		}
		switch r.URL.Path {
		case "/v1.0/users/alice@contoso.com/mailFolders":
			io.WriteString(w, `{"value":[{"id":"TOP"}]}`)
		case "/v1.0/users/alice@contoso.com/mailFolders/TOP/childFolders":
			io.WriteString(w, `{"value":[{"id":"CHILD"}]}`)
		default:
			return false
		}
		return true
	}
	c := f.config("nested")
	c.Folder, c.FolderName = FolderCustom, "Training/2026"
	if err := sendMS(c, plainMsg); err != nil {
		t.Fatal(err)
	}
	if mustUnescape(t, f.reqs[1].query) != "$filter=displayName eq '2026'" || f.reqs[2].path != "/v1.0/users/alice@contoso.com/mailFolders/CHILD/messages" {
		t.Fatalf("%+v", f.reqs)
	}
}

func TestMSInboxFallback(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if r.Method != "GET" {
			return false
		}
		if strings.HasSuffix(r.URL.Path, "/mailFolders") {
			io.WriteString(w, `{"value":[]}`)
		} else if strings.HasSuffix(r.URL.Path, "/mailFolders/inbox/childFolders") {
			io.WriteString(w, `{"value":[{"id":"UNDERINBOX"}]}`)
		} else {
			return false
		}
		return true
	}
	c := f.config("inboxfb")
	c.Folder, c.FolderName = FolderCustom, "Phish"
	if err := sendMS(c, plainMsg); err != nil {
		t.Fatal(err)
	}
	if f.reqs[2].path != "/v1.0/users/alice@contoso.com/mailFolders/UNDERINBOX/messages" {
		t.Fatalf("%+v", f.reqs)
	}
}

func TestMSNestedNotFoundNamesPath(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		if strings.HasSuffix(r.URL.Path, "/childFolders") {
			io.WriteString(w, `{"value":[]}`)
			return true
		}
		return false
	}
	c := f.config("nestedmissing")
	c.Folder, c.FolderName = FolderCustom, "Training/2026"
	err := sendMS(c, plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Training/2026") {
		t.Fatalf("err %v", err)
	}
}

func TestMSLargeCreateResponse(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(201)
		io.WriteString(w, `{"id":"MSG1","pad":"`+strings.Repeat("x", 2<<20)+`"}`)
		return true
	}
	if err := sendMS(f.config("bigresp"), plainMsg); err != nil {
		t.Fatal(err)
	}
}

func TestMSGarbageCreateResponsePermanent(t *testing.T) {
	f := newFakeGraph(t)
	f.handler = func(w http.ResponseWriter, r *http.Request, n int) bool {
		w.WriteHeader(201)
		io.WriteString(w, `not json`)
		return true
	}
	err := sendMS(f.config("garbage"), plainMsg)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() {
		t.Fatalf("err %v", err)
	}
}
