package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophish/gophish/models"
)

func peopleRequest(ctx *testContext, method, url string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.apiKey))
	w := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(w, r)
	return w
}

func TestPeopleEndpointReturnsJSON(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodGet, "/api/people/")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	stats := []models.PersonStats{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("expected a JSON array: %v", err)
	}
}

func TestPeopleEndpointBadDate(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodGet, "/api/people/?from=nope")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPeopleEndpointBadIds(t *testing.T) {
	ctx := setupTest(t)
	for _, q := range []string{"group_id=abc", "group_id=-1", "campaign_set_id=x"} {
		w := peopleRequest(ctx, http.MethodGet, "/api/people/?"+q)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", q, w.Code)
		}
	}
}

func TestPeopleEndpointForeignGroup(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodGet, "/api/people/?group_id=9999")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPeopleEndpointMethodNotAllowed(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodPost, "/api/people/")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPeopleTimelineRequiresKey(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodGet, "/api/people/timeline")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPeopleTimelineReturnsJSON(t *testing.T) {
	ctx := setupTest(t)
	w := peopleRequest(ctx, http.MethodGet, "/api/people/timeline?key=nobody@example.com&to=2030-01-01")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	tl := []models.PersonCampaign{}
	if err := json.NewDecoder(w.Body).Decode(&tl); err != nil {
		t.Fatalf("expected a JSON array: %v", err)
	}
}
