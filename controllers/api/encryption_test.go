package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gophish/gophish/crypto"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/sirupsen/logrus"
)

func resetEncryptionForTest(t *testing.T) {
	reset := func() {
		crypto.Unload()
		models.SetEncryptionLockedForTest(false)
		unlockLimiter.reset()
	}
	reset()
	t.Cleanup(reset)
}

func encryptionRequest(t *testing.T, tc *testContext, apiKey, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("error encoding body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	tc.apiServer.ServeHTTP(w, req)
	return w
}

func decodeStatus(t *testing.T, w *httptest.ResponseRecorder) models.EncryptionStatus {
	t.Helper()
	st := models.EncryptionStatus{}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("error decoding status: %v (%s)", err, w.Body.String())
	}
	return st
}

func generateKey(t *testing.T, tc *testContext) string {
	t.Helper()
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/generate", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("generate: expected 200, got %d", w.Code)
	}
	out := struct {
		Key string `json:"key"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("error decoding key: %v", err)
	}
	return out.Key
}

func TestEncryptionEndpointsForbiddenForNonAdmin(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	user := createUnpriviledgedUser(t, models.RoleUser)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/encryption/status"},
		{http.MethodPost, "/api/encryption/generate"},
		{http.MethodPost, "/api/encryption/enable"},
		{http.MethodPost, "/api/encryption/unlock"},
		{http.MethodPost, "/api/encryption/migrate"},
		{http.MethodPost, "/api/encryption/disable"},
		{http.MethodPost, "/api/encryption/discard"},
	}
	for _, rt := range routes {
		w := encryptionRequest(t, tc, user.ApiKey, rt.method, rt.path, map[string]string{})
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d", rt.method, rt.path, w.Code)
		}
	}
}

func TestGenerateReturnsKeyNoStore(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/generate", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", cc)
	}
	out := struct {
		Key string `json:"key"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Key) != 44 {
		t.Fatalf("expected 44 char key, got %d", len(out.Key))
	}
	st := decodeStatus(t, encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/encryption/status", nil))
	if st.State != models.EncryptionStateOff {
		t.Fatalf("expected state off after generate, got %s", st.State)
	}
}

func TestEnableThenStatus(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	key := generateKey(t, tc)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": key})
	if w.Code != http.StatusOK {
		t.Fatalf("enable: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), key) {
		t.Fatal("enable response contains the key")
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodGet, "/api/encryption/status", nil)
	st := decodeStatus(t, w)
	if st.State != models.EncryptionStateOn || st.Source != crypto.SourceSession {
		t.Fatalf("unexpected status: %s / %s", st.State, st.Source)
	}
	if strings.Contains(w.Body.String(), key) {
		t.Fatal("status response contains the key")
	}
}

func TestEnableInvalidKey(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": "abc"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "abc") {
		t.Fatalf("error body echoes input: %s", w.Body.String())
	}
}

func TestUnlockWrongKeyAndRateLimit(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	// Write a check value with one key, then lock and unlock with another.
	right := generateKey(t, tc)
	encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": right})
	crypto.Unload()
	models.SetEncryptionLockedForTest(true)
	wrong := generateKey(t, tc)
	for i := 1; i <= 5; i++ {
		w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/unlock", map[string]string{"key": wrong})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %d: %s", i, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), wrong) {
			t.Fatal("body contains submitted key")
		}
	}
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/unlock", map[string]string{"key": wrong})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: expected 429, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), wrong) {
		t.Fatal("body contains submitted key")
	}
}

func TestUnlockMalformedKeyCountsAsFailure(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	models.SetEncryptionLockedForTest(true)
	for i := 0; i < 5; i++ {
		w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/unlock", map[string]string{"key": "short"})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	}
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/unlock", map[string]string{"key": "short"})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestUnlockWhenNotLocked(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/unlock", map[string]string{"key": generateKey(t, tc)})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestMigrateBadDirection(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/migrate", map[string]interface{}{"direction": "sideways"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "sideways") {
		t.Fatal("error body echoes input")
	}
}

func TestMigrateEncryptRefusedWhileDisabling(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.LoadKey(key, crypto.SourceSession); err != nil {
		t.Fatal(err)
	}
	crypto.SetDecryptOnly()
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/migrate", map[string]interface{}{"direction": "encrypt"})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMigrateDryRunWhenOn(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	key := generateKey(t, tc)
	encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": key})
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/migrate", map[string]interface{}{"direction": "encrypt", "dry_run": true})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	out := map[string]json.RawMessage{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["results"]; !ok {
		t.Fatal("missing results")
	}
	if _, ok := out["status"]; !ok {
		t.Fatal("missing status")
	}
}

func TestDiscardWrongPhrase(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	models.SetEncryptionLockedForTest(true)
	before, err := models.GetEncryptionReport()
	if err != nil {
		t.Fatal(err)
	}
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/discard", map[string]string{"confirmation": "discard"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	after, err := models.GetEncryptionReport()
	if err != nil {
		t.Fatal(err)
	}
	if *before != *after {
		t.Fatalf("counts changed: %+v -> %+v", before, after)
	}
	if !models.IsEncryptionLocked() {
		t.Fatal("expected still locked")
	}
}

func TestKeyNeverLogged(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	var buf bytes.Buffer
	oldOut := log.Logger.Out
	oldLevel := log.Logger.Level
	log.Logger.SetOutput(&buf)
	log.Logger.SetLevel(logrus.DebugLevel)
	defer func() {
		log.Logger.SetOutput(oldOut)
		log.Logger.SetLevel(oldLevel)
	}()

	key := generateKey(t, tc)
	encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": key})
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/disable", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("disable: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if buf.Len() == 0 {
		t.Fatal("expected audit log output")
	}
	if strings.Contains(buf.String(), key) {
		t.Fatal("log output contains the key")
	}
}

func TestDisableWithUndecryptableValueReturnsConflict(t *testing.T) {
	tc := setupTest(t)
	resetEncryptionForTest(t)
	key := generateKey(t, tc)
	w := encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/enable", map[string]string{"key": key})
	if w.Code != http.StatusOK {
		t.Fatalf("enable: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := crypto.LoadKey(generateKey(t, tc), crypto.SourceSession); err != nil {
		t.Fatal(err)
	}
	other, err := crypto.Encrypt("captured")
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.LoadKey(key, crypto.SourceSession); err != nil {
		t.Fatal(err)
	}
	if err := models.AddEvent(&models.Event{Email: "a@example.com", Details: other}, 1); err != nil {
		t.Fatal(err)
	}
	w = encryptionRequest(t, tc, tc.apiKey, http.MethodPost, "/api/encryption/disable", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("disable: expected 409, got %d: %s", w.Code, w.Body.String())
	}
	out := struct {
		Data struct {
			Results []string `json:"results"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data.Results) == 0 {
		t.Fatalf("expected results in body: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), key) {
		t.Fatal("response contains the key")
	}
}
