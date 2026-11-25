package dmi

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func cacheHas(key string) bool {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	_, ok := tokenSources[key]
	return ok
}

func TestTokenCacheEvictsIdleSources(t *testing.T) {
	build := func() oauth2.TokenSource { return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "x"}) }
	start := time.Now()
	old, fresh := tokenKey("test", "old"), tokenKey("test", "fresh")
	cachedTokenSourceAt(start, old, build)
	cachedTokenSourceAt(start.Add(tokenIdleTTL/2), fresh, build)
	cachedTokenSourceAt(start.Add(tokenIdleTTL+time.Minute), fresh, build)
	if cacheHas(old) {
		t.Fatal("idle source was not evicted")
	}
	if !cacheHas(fresh) {
		t.Fatal("recently used source was evicted")
	}
	invalidateTokenSource(fresh)
	if cacheHas(fresh) {
		t.Fatal("invalidate did not remove the source")
	}
}

func TestGoogleTokensStayOnDialer(t *testing.T) {
	f := newFakeGoogle(t)
	tokenMu.Lock()
	before := len(tokenSources)
	tokenMu.Unlock()
	d, err := newGoogle(f.config(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"a@example.com", "b@example.com", "a@example.com"} {
		if err := d.Send("from@x.com", []string{to}, bytesWriterTo("Subject: hi\r\n\r\nbody\r\n")); err != nil {
			t.Fatal(err)
		}
	}
	tokenMu.Lock()
	after := len(tokenSources)
	tokenMu.Unlock()
	if after != before {
		t.Fatalf("global cache grew from %d to %d", before, after)
	}
	if f.tokens != 2 {
		t.Fatalf("expected one token per mailbox, got %d", f.tokens)
	}
}
