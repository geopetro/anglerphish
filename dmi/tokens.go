package dmi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// tokenIdleTTL drops sources unused this long, so edited or deleted profiles do not keep secrets in memory.
const tokenIdleTTL = time.Hour

type cachedSource struct {
	ts   oauth2.TokenSource
	used time.Time
}

var (
	tokenMu      sync.Mutex
	tokenSources = map[string]*cachedSource{}
)

// tokenKey hashes the identifying parts so editing a profile invalidates the cache.
func tokenKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func invalidateTokenSource(key string) {
	tokenMu.Lock()
	delete(tokenSources, key)
	tokenMu.Unlock()
}

// existingTokenSource returns a cached source without building one, so callers can skip costly setup.
func existingTokenSource(key string) (oauth2.TokenSource, bool) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	e, ok := tokenSources[key]
	if !ok || time.Since(e.used) > tokenIdleTTL {
		return nil, false
	}
	e.used = time.Now()
	return e.ts, true
}

func cachedTokenSource(key string, build func() oauth2.TokenSource) oauth2.TokenSource {
	return cachedTokenSourceAt(time.Now(), key, build)
}

func cachedTokenSourceAt(now time.Time, key string, build func() oauth2.TokenSource) oauth2.TokenSource {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	for k, e := range tokenSources {
		if now.Sub(e.used) > tokenIdleTTL {
			delete(tokenSources, k)
		}
	}
	e, ok := tokenSources[key]
	if !ok {
		e = &cachedSource{ts: oauth2.ReuseTokenSource(nil, build())}
		tokenSources[key] = e
	}
	e.used = now
	return e.ts
}
