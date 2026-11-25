package dmi

import (
	"net/url"
	"testing"
)

func mustUnescape(t *testing.T, q string) string {
	t.Helper()
	s, err := url.QueryUnescape(q)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewDialerRejectsInvalidConfig(t *testing.T) {
	c := Config{Provider: ProviderMicrosoft, AuthMethod: AuthClientSecret}
	want := c.Validate()
	_, err := NewDialer(c)
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("err %v want %v", err, want)
	}
	if _, err := NewDialer(Config{Provider: "nope"}); err == nil {
		t.Fatal("unknown provider must fail")
	}
}

func TestTestConnectionChecksRecipient(t *testing.T) {
	f := newFakeGraph(t)
	if err := TestConnection(t.Context(), f.config("tcdom"), "x@evil.com"); err == nil || len(f.reqs) != 0 {
		t.Fatalf("err %v", err)
	}
}

func TestTestConnectionGoogleCustomResolvesLabel(t *testing.T) {
	f := newFakeGoogle(t)
	f.labels = `{"labels":[{"id":"Label_9","name":"Phish"}]}`
	c := f.config(t)
	c.Folder, c.FolderName = FolderCustom, "Phish"
	if err := TestConnection(t.Context(), c, "hank@example.com"); err != nil || f.inserts != 0 {
		t.Fatalf("err %v", err)
	}
	c.FolderName = "Missing"
	if err := TestConnection(t.Context(), c, "hank@example.com"); err == nil {
		t.Fatal("missing label must fail")
	}
}
