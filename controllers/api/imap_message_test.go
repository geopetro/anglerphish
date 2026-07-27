package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophish/gophish/models"
)

// A user must never be able to read another user's report. Scoping the lookup
// by user id means an unauthorized id is indistinguishable from a missing one.
func TestGetMessageRejectsOtherUsersReport(t *testing.T) {
	ctx := setupTest(t)

	// Seed a report owned by a user other than the authenticated admin (id 1).
	err := models.RecordNonCampaignReport(9999, 1, "jane@corp.com", "Not yours", 1, 1, "<x@y>")
	if err != nil {
		t.Fatalf("failed seeding report: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/imap/non_campaign_reports/1/message", nil)
	req.Header.Set("Authorization", "Bearer "+ctx.apiKey)
	response := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(response, req)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another user's report, got %d", response.Code)
	}
}

func TestGetMessageReturns404ForUnknownReport(t *testing.T) {
	ctx := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/imap/non_campaign_reports/424242/message", nil)
	req.Header.Set("Authorization", "Bearer "+ctx.apiKey)
	response := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(response, req)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}

// A report with no stored identifiers predates the feature. It must say so
// rather than attempting a lookup that cannot succeed.
func TestGetMessageReturns410ForLegacyReport(t *testing.T) {
	ctx := setupTest(t)

	err := models.RecordNonCampaignReport(1, 1, "jane@corp.com", "Legacy", 0, 0, "")
	if err != nil {
		t.Fatalf("failed seeding report: %v", err)
	}
	reports, err := models.GetRecentNonCampaignReports(1, 1, 0)
	if err != nil || len(reports) == 0 {
		t.Fatalf("failed reading back seeded report: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/imap/non_campaign_reports/%d/message", reports[0].Id), nil)
	req.Header.Set("Authorization", "Bearer "+ctx.apiKey)
	response := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(response, req)

	if response.Code != http.StatusGone {
		t.Fatalf("expected 410 for a report with no identifiers, got %d", response.Code)
	}
}
