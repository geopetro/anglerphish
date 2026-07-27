package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophish/gophish/models"
)

// TestCampaignSetSummaryNotFound exercises the route end-to-end through the
// router so that mux vars and the auth middleware are actually applied.
//
// Asserting on the response BODY matters here: an unregistered route also
// returns 404, but with mux's plain-text "404 page not found". Only our handler
// emits the JSON error body, so this is what proves the route is really wired.
func TestCampaignSetSummaryNotFound(t *testing.T) {
	ctx := setupTest(t)

	url := fmt.Sprintf("/api/campaign_sets/%d/summary", 999999)
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.apiKey))
	w := httptest.NewRecorder()

	ctx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a nonexistent set, got %d", w.Code)
	}

	response := models.Response{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("expected a JSON error body from the handler, got %q", w.Body.String())
	}
	if response.Message != "Campaign set not found" {
		t.Fatalf("unexpected error message: %q", response.Message)
	}
}

// TestCampaignSetSummaryRequiresAuth confirms the endpoint sits behind the
// RequireAPIKey middleware like every other /api route.
func TestCampaignSetSummaryRequiresAuth(t *testing.T) {
	ctx := setupTest(t)

	url := fmt.Sprintf("/api/campaign_sets/%d/summary", 1)
	r := httptest.NewRequest(http.MethodGet, url, nil)
	// Deliberately no Authorization header.
	w := httptest.NewRecorder()

	ctx.apiServer.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatalf("expected an auth failure without an API key, got 200")
	}
}
