package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var successHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
})

func reachLimit(t *testing.T, handler http.Handler, method string, limit int) {
	t.Helper()
	r := httptest.NewRequest(method, "/", nil)
	r.RemoteAddr = "127.0.0.1:"
	for i := 0; i < limit; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("no 200 on req %d got %d", i, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("no 429")
	}
	if got := strings.TrimSpace(w.Body.String()); got != http.StatusText(http.StatusTooManyRequests) {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestRateLimitEnforcement(t *testing.T) {
	expectedLimit := 3
	limiter := NewPostLimiter(WithRequestsPerMinute(expectedLimit))
	handler := limiter.Limit(successHandler)
	reachLimit(t, handler, http.MethodPost, expectedLimit)
}

func TestRateLimitGETEnforcement(t *testing.T) {
	expectedLimit := 3
	limiter := NewPostLimiter(WithRequestsPerMinute(expectedLimit))
	handler := limiter.LimitGET(successHandler)
	reachLimit(t, handler, http.MethodGet, expectedLimit)
}

func TestRateLimitCleanup(t *testing.T) {
	expectedLimit := 3
	limiter := NewPostLimiter(WithRequestsPerMinute(expectedLimit))
	handler := limiter.Limit(successHandler)
	reachLimit(t, handler, http.MethodPost, expectedLimit)
	bucket, exists := limiter.visitors["127.0.0.1"]
	if !exists {
		t.Fatalf("doesn't exist for some reason")
	}
	bucket.lastSeen = bucket.lastSeen.Add(-limiter.expiry)
	limiter.Cleanup()
	_, exists = limiter.visitors["127.0.0.1"]
	if exists {
		t.Fatalf("exists for some reason")
	}
	reachLimit(t, handler, http.MethodPost, expectedLimit)
}
