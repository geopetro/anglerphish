package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophish/gophish/models"
)

func postSMTP(t *testing.T, ctx *testContext, s models.SMTP) *httptest.ResponseRecorder {
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal smtp: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/smtp/", bytes.NewReader(body))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.apiKey))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(w, r)
	return w
}

// TestPostSMTPInvalidIsBadRequest verifies that creating a sending profile
// with invalid input is a 400 with the validation message, like updating one,
// rather than a 500.
func TestPostSMTPInvalidIsBadRequest(t *testing.T) {
	ctx := setupTest(t)
	cases := []struct {
		name string
		smtp models.SMTP
		want error
	}{
		{"display-name from address", models.SMTP{Name: "a", Host: "127.0.0.1:25", FromAddress: "IT <it@example.com>"}, models.ErrInvalidFromAddress},
		{"missing from address", models.SMTP{Name: "b", Host: "127.0.0.1:25"}, models.ErrFromAddressNotSpecified},
		{"missing host", models.SMTP{Name: "c", FromAddress: "it@example.com"}, models.ErrHostNotSpecified},
	}
	for _, tc := range cases {
		w := postSMTP(t, ctx, tc.smtp)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
		resp := models.Response{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("%s: decode response: %v", tc.name, err)
		}
		if resp.Message != tc.want.Error() {
			t.Fatalf("%s: expected message %q, got %q", tc.name, tc.want.Error(), resp.Message)
		}
	}
}

func TestPostSMTPValidIsCreated(t *testing.T) {
	ctx := setupTest(t)
	w := postSMTP(t, ctx, models.SMTP{Name: "ok", Host: "127.0.0.1:25", FromAddress: "it@example.com"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
