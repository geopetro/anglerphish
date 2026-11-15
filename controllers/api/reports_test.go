package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/reports"
	"github.com/gorilla/mux"
)

// setupReports gives the handlers a job service that is not started, so
// queued reports stay queued during the test.
func setupReports(t *testing.T) {
	storage := t.TempDir()
	fileManager = reports.NewFileManager(storage)
	jobService = reports.NewJobService(fileManager)
}

func reportRequest(method, path string, body interface{}, user models.User, vars map[string]string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req = ctx.Set(req, "user", user)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	return req
}

func TestQueueReportOnlyAcceptsOwnCampaigns(t *testing.T) {
	testCtx := setupTest(t)
	setupReports(t)
	createTestData(t) // campaign 1, owned by the admin
	other := createUnpriviledgedUser(t, models.RoleUser)

	// Another user can't report on the admin's campaign
	w := httptest.NewRecorder()
	testCtx.apiServer.QueueReport(w, reportRequest(http.MethodPost, "/api/reports/queue",
		map[string]interface{}{"campaign_ids": []int64{1}, "format": "word"}, *other, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for someone else's campaign, got %d", w.Code)
	}

	// The owner can
	w = httptest.NewRecorder()
	testCtx.apiServer.QueueReport(w, reportRequest(http.MethodPost, "/api/reports/queue",
		map[string]interface{}{"campaign_ids": []int64{1, 1}, "format": "excel",
			"options": map[string]bool{"anonymize_ips": true}}, testCtx.admin, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp reports.AsyncReportResponse
	json.NewDecoder(w.Body).Decode(&resp)
	report, err := models.GetReport(resp.JobID, testCtx.admin.Id)
	if err != nil {
		t.Fatalf("queued report not found: %v", err)
	}
	if report.CampaignIds != "[1]" {
		t.Fatalf("duplicate campaign IDs should be removed, got %s", report.CampaignIds)
	}
	if !strings.Contains(report.OptionsJson, `"anonymize_ips":true`) {
		t.Fatalf("options not stored: %s", report.OptionsJson)
	}

	// Unsupported formats are rejected
	w = httptest.NewRecorder()
	testCtx.apiServer.QueueReport(w, reportRequest(http.MethodPost, "/api/reports/queue",
		map[string]interface{}{"campaign_ids": []int64{1}, "format": "pdf"}, testCtx.admin, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported format, got %d", w.Code)
	}
}

func TestReportSettingsEndpoint(t *testing.T) {
	testCtx := setupTest(t)
	defer models.SaveReportSettings(models.ReportSettings{JobTimeoutMinutes: 30, RetentionDays: 30})

	w := httptest.NewRecorder()
	testCtx.apiServer.ReportSettings(w, reportRequest(http.MethodGet, "/api/report_settings", nil, testCtx.admin, nil))
	var got map[string]interface{}
	json.NewDecoder(w.Body).Decode(&got)
	if w.Code != http.StatusOK || got["online_geolocation"] != false || got["retention_days"] != float64(30) {
		t.Fatalf("unexpected defaults: %d %v", w.Code, got)
	}

	w = httptest.NewRecorder()
	testCtx.apiServer.ReportSettings(w, reportRequest(http.MethodPut, "/api/report_settings",
		map[string]interface{}{"online_geolocation": true, "job_timeout_minutes": 0, "retention_days": 30}, testCtx.admin, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid timeout, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	testCtx.apiServer.ReportSettings(w, reportRequest(http.MethodPut, "/api/report_settings",
		map[string]interface{}{"online_geolocation": true, "job_timeout_minutes": 15, "retention_days": 7}, testCtx.admin, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	rs, _ := models.GetReportSettings()
	if !rs.OnlineGeolocation || rs.JobTimeoutMinutes != 15 || rs.RetentionDays != 7 {
		t.Fatalf("settings not saved: %+v", rs)
	}
}

func TestListReportsRejectsLegacyPost(t *testing.T) {
	testCtx := setupTest(t)
	setupReports(t)
	w := httptest.NewRecorder()
	testCtx.apiServer.ListReports(w, reportRequest(http.MethodPost, "/api/reports/", map[string]interface{}{}, testCtx.admin, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCampaignSetEndpointRequiresSetID(t *testing.T) {
	testCtx := setupTest(t)
	setupReports(t)
	w := httptest.NewRecorder()
	testCtx.apiServer.GenerateCampaignSetReport(w, reportRequest(http.MethodPost, "/api/reports/campaign_set",
		map[string]interface{}{"format": "word"}, testCtx.admin, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDownloadReportStreamsFileWithSafeName(t *testing.T) {
	testCtx := setupTest(t)
	setupReports(t)

	path, err := fileManager.SaveReport("report_x.xlsx", []byte("xlsx-bytes"))
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	report := models.Report{UserId: testCtx.admin.Id, Format: "excel", CampaignIds: "[1]", Status: models.ReportStatusCompleted,
		FilePath: path, FileName: "Anglerphish_Q3_2026-11-15.xlsx", FileSize: 10}
	if err := models.PostReport(&report); err != nil {
		t.Fatalf("post report: %v", err)
	}
	vars := map[string]string{"id": fmt.Sprint(report.Id)}

	w := httptest.NewRecorder()
	testCtx.apiServer.DownloadReport(w, reportRequest(http.MethodGet, "/api/reports/1/download", nil, testCtx.admin, vars))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "xlsx-bytes" {
		t.Fatalf("unexpected body %q", body)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename=Anglerphish_Q3_2026-11-15.xlsx` {
		t.Fatalf("unexpected Content-Disposition %q", cd)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("downloads must not set CORS headers")
	}

	// Another user can't download it
	other := createUnpriviledgedUser(t, models.RoleUser)
	w = httptest.NewRecorder()
	testCtx.apiServer.DownloadReport(w, reportRequest(http.MethodGet, "/api/reports/1/download", nil, *other, vars))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another user, got %d", w.Code)
	}

	// A record pointing outside the storage directory is refused
	report.FilePath = t.TempDir()
	models.PutReport(&report)
	w = httptest.NewRecorder()
	testCtx.apiServer.DownloadReport(w, reportRequest(http.MethodGet, "/api/reports/1/download", nil, testCtx.admin, vars))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a path outside storage, got %d", w.Code)
	}
}
