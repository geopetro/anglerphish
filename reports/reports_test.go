package reports

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/models"
)

func TestMain(m *testing.M) {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../db/db_sqlite3/migrations/",
	}
	if err := models.Setup(conf); err != nil {
		fmt.Printf("failed creating database: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

var userCounter int

// createUser creates a non-admin user. Reports used to only work for user 1.
func createUser(t *testing.T) models.User {
	t.Helper()
	role, err := models.GetRoleBySlug(models.RoleUser)
	if err != nil {
		t.Fatalf("error getting role: %v", err)
	}
	userCounter++
	u := models.User{
		Username: fmt.Sprintf("report-user-%d", userCounter),
		Hash:     "x",
		ApiKey:   fmt.Sprintf("report-key-%d", userCounter),
		Role:     role,
		RoleID:   role.ID,
	}
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error creating user: %v", err)
	}
	return u
}

// createCampaign creates a launched email campaign owned by uid, with one
// click and one form submission recorded.
func createCampaign(t *testing.T, uid int64, name string) models.Campaign {
	t.Helper()
	group := models.Group{Name: name + " group", UserId: uid}
	group.Targets = []models.Target{
		{BaseRecipient: models.BaseRecipient{Email: "victim@example.com", FirstName: "Victor", LastName: "Tim"}},
		{BaseRecipient: models.BaseRecipient{Email: "careful@example.com", FirstName: "Carla", LastName: "Ful"}},
	}
	if err := models.PostGroup(&group); err != nil {
		t.Fatalf("error creating group: %v", err)
	}
	template := models.Template{Name: name + " template", Subject: "Hello", Text: "Text", UserId: uid}
	if err := models.PostTemplate(&template); err != nil {
		t.Fatalf("error creating template: %v", err)
	}
	page := models.Page{Name: name + " page", HTML: "<html></html>", UserId: uid}
	if err := models.PostPage(&page); err != nil {
		t.Fatalf("error creating page: %v", err)
	}
	smtp := models.SMTP{Name: name + " smtp", Host: "example.com", FromAddress: "a@example.com", UserId: uid}
	if err := models.PostSMTP(&smtp); err != nil {
		t.Fatalf("error creating smtp: %v", err)
	}
	c := models.Campaign{Name: name, UserId: uid, Template: template, Page: page, SMTP: smtp, Groups: []models.Group{group}}
	if err := models.PostCampaign(&c, uid); err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}
	c.UpdateStatus(models.CampaignEmailsSent)

	results, err := models.GetCampaignResults(c.Id, uid)
	if err != nil || len(results.Results) == 0 {
		t.Fatalf("error getting results: %v", err)
	}
	for i := range results.Results {
		r := results.Results[i]
		if r.Email != "victim@example.com" {
			continue
		}
		if err := r.HandleEmailSent(); err != nil {
			t.Fatalf("error recording send: %v", err)
		}
		details := models.EventDetails{Browser: map[string]string{"address": "203.0.113.50", "user-agent": "Mozilla/5.0"}}
		if err := r.HandleClickedLink(details); err != nil {
			t.Fatalf("error recording click: %v", err)
		}
		details.Payload = url.Values{"password": {"=HYPERLINK(\"http://evil\")"}}
		if err := r.HandleFormSubmit(details); err != nil {
			t.Fatalf("error recording submit: %v", err)
		}
	}
	return c
}

func TestValidateAndFetchUseRequestingUser(t *testing.T) {
	owner := createUser(t)
	other := createUser(t)
	c := createCampaign(t, owner.Id, "Owner campaign")

	v, err := ValidateCampaignData([]int64{c.Id}, owner.Id)
	if err != nil || !v.Valid {
		t.Fatalf("owner validation failed: %v %+v", err, v)
	}
	data, err := FetchCampaignData([]int64{c.Id}, owner.Id)
	if err != nil {
		t.Fatalf("owner fetch failed: %v", err)
	}
	if len(data.Campaigns) != 1 || data.Campaigns[0]["name"] != "Owner campaign" {
		t.Fatalf("unexpected report data: %+v", data.Campaigns)
	}

	if v, _ := ValidateCampaignData([]int64{c.Id}, other.Id); v.Valid {
		t.Fatalf("another user must not validate someone else's campaign")
	}
	if _, err := FetchCampaignData([]int64{c.Id}, other.Id); err == nil {
		t.Fatalf("another user must not fetch someone else's campaign")
	}
}

func TestReportDataIncludesTimeline(t *testing.T) {
	owner := createUser(t)
	c := createCampaign(t, owner.Id, "Timeline campaign")
	data, err := FetchCampaignData([]int64{c.Id}, owner.Id)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	raw, _ := json.Marshal(data)
	if !strings.Contains(string(raw), `"timeline"`) {
		t.Fatalf("timeline missing from report data")
	}
}

func TestFileManagerRejectsPathsOutsideStorage(t *testing.T) {
	dir := t.TempDir()
	fm := NewFileManager(dir)

	path, err := fm.SaveReport("report_1.docx", []byte("data"))
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %s", path)
	}
	f, info, err := fm.OpenReportFile(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "data" || info.Size() != 4 {
		t.Fatalf("unexpected content %q", b)
	}

	outside := filepath.Join(filepath.Dir(dir), "elsewhere.docx")
	for _, p := range []string{outside, dir, filepath.Join(dir, "..", "x"), filepath.Join(dir, "..")} {
		if _, _, err := fm.OpenReportFile(p); err != ErrOutsideStorage {
			t.Errorf("expected ErrOutsideStorage for %s, got %v", p, err)
		}
	}
	// A file whose name merely starts with ".." is inside the storage directory
	if _, err := fm.SaveReport("..odd.docx", []byte("x")); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := fm.DeleteReportFile(path); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := fm.DeleteReportFile(path); err != nil {
		t.Fatalf("deleting a missing file should not fail: %v", err)
	}
}

func TestDownloadFileName(t *testing.T) {
	when := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	got := DownloadFileName(`Q3 "Finance"/Payroll: test`, when, FormatExcel)
	if got != "Anglerphish_Q3_Finance_Payroll_test_2026-11-15.xlsx" {
		t.Fatalf("unexpected file name %q", got)
	}
	if got := DownloadFileName("", when, FormatWord); got != "Anglerphish_Report_2026-11-15.docx" {
		t.Fatalf("unexpected file name %q", got)
	}
}

func TestQueueClaimRequeueAndExpire(t *testing.T) {
	owner := createUser(t)
	c := createCampaign(t, owner.Id, "Queue campaign")
	js := NewJobService(NewFileManager(t.TempDir()))
	js.settings = func() Settings { return Settings{Retention: time.Hour}.withDefaults() }

	id, err := js.QueueReport(owner.Id, []int64{c.Id}, FormatExcel, ReportOptions{AnonymizeIPs: true})
	if err != nil {
		t.Fatalf("queue failed: %v", err)
	}
	report, _ := models.GetReport(id, owner.Id)
	if report.ExpiresAt == nil || time.Until(*report.ExpiresAt) > time.Hour {
		t.Fatalf("retention not applied: %v", report.ExpiresAt)
	}

	claimed, err := models.ClaimReport(id)
	if err != nil || !claimed {
		t.Fatalf("first claim should succeed: %v", err)
	}
	if claimed, _ := models.ClaimReport(id); claimed {
		t.Fatalf("second claim must fail")
	}
	if n, err := models.RequeueProcessingReports(); err != nil || n < 1 {
		t.Fatalf("requeue failed: %d %v", n, err)
	}
	report, _ = models.GetReport(id, owner.Id)
	if report.Status != models.ReportStatusQueued {
		t.Fatalf("expected queued after requeue, got %s", report.Status)
	}

	// Expire it: the record and its file are removed
	path, _ := js.files.SaveReport("expired.xlsx", []byte("x"))
	models.UpdateReportFile(id, path, "expired.xlsx", 1)
	models.UpdateReportStatus(id, models.ReportStatusCompleted)
	past := time.Now().UTC().Add(-time.Minute)
	report, _ = models.GetReport(id, owner.Id)
	report.ExpiresAt = &past
	models.PutReport(&report)
	if n := js.CleanupExpiredReports(); n < 1 {
		t.Fatalf("expected an expired report to be cleaned up")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expired report file should be deleted")
	}
	if _, err := models.GetReport(id, owner.Id); err == nil {
		t.Fatalf("expired report record should be deleted")
	}
}

func TestReportStats(t *testing.T) {
	owner := createUser(t)
	js := NewJobService(NewFileManager(t.TempDir()))
	c := createCampaign(t, owner.Id, "Stats campaign")
	js.QueueReport(owner.Id, []int64{c.Id}, FormatWord, ReportOptions{})
	id, _ := js.QueueReport(owner.Id, []int64{c.Id}, FormatWord, ReportOptions{})
	models.UpdateReportError(id, "boom")
	stats, err := models.GetReportStats(owner.Id)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats["total"] != 2 || stats[models.ReportStatusQueued] != 1 || stats[models.ReportStatusFailed] != 1 {
		t.Fatalf("unexpected stats %v", stats)
	}
}

func TestSettingsComeFromTheDatabaseForEachJob(t *testing.T) {
	defer models.SaveReportSettings(models.ReportSettings{JobTimeoutMinutes: 30, RetentionDays: 30})

	if s := LoadSettings(); s.OnlineGeolocation || s.JobTimeout != DefaultJobTimeout || s.Retention != DefaultRetention {
		t.Fatalf("unexpected defaults: %+v", s)
	}
	if err := models.SaveReportSettings(models.ReportSettings{OnlineGeolocation: true, JobTimeoutMinutes: 5, RetentionDays: 2}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	s := LoadSettings()
	if !s.OnlineGeolocation || s.JobTimeout != 5*time.Minute || s.Retention != 48*time.Hour {
		t.Fatalf("saved settings not loaded: %+v", s)
	}

	// A running service picks the change up for the next report, no restart
	owner := createUser(t)
	c := createCampaign(t, owner.Id, "Settings campaign")
	js := NewJobService(NewFileManager(t.TempDir()))
	id, err := js.QueueReport(owner.Id, []int64{c.Id}, FormatWord, ReportOptions{})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	report, _ := models.GetReport(id, owner.Id)
	if report.ExpiresAt == nil || time.Until(*report.ExpiresAt) > 48*time.Hour || time.Until(*report.ExpiresAt) < 47*time.Hour {
		t.Fatalf("retention from settings not applied: %v", report.ExpiresAt)
	}
	models.DeleteReport(id, owner.Id)
}

// pythonWithOpenpyxl returns a Python interpreter that can build Excel reports.
func pythonWithOpenpyxl(t *testing.T) string {
	py := findPythonExecutable()
	if py == "" {
		t.Skip("Python not available")
	}
	if err := exec.Command(py, "-c", "import openpyxl").Run(); err != nil {
		t.Skip("openpyxl not installed for " + py)
	}
	return py
}

// TestEndToEndExcelReportForNonAdmin runs the whole pipeline: queue,
// worker, Python generator, file storage. It checks the anonymized output
// and that captured values can't become formulas.
func TestEndToEndExcelReportForNonAdmin(t *testing.T) {
	py := pythonWithOpenpyxl(t)
	owner := createUser(t)
	c := createCampaign(t, owner.Id, "E2E campaign")

	js := NewJobService(NewFileManager(t.TempDir()))
	js.python.PythonPath = py

	id, err := js.QueueReport(owner.Id, []int64{c.Id}, FormatExcel, ReportOptions{AnonymizeEmails: true, AnonymizeIPs: true})
	if err != nil {
		t.Fatalf("queue failed: %v", err)
	}
	js.processQueuedJobs(context.Background())

	report, err := models.GetReport(id, owner.Id)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if report.Status != models.ReportStatusCompleted {
		t.Fatalf("report not completed: %s %s", report.Status, report.ErrorMessage)
	}
	if !strings.HasPrefix(report.FileName, "Anglerphish_E2E_campaign_") {
		t.Fatalf("unexpected download name %q", report.FileName)
	}

	zr, err := zip.OpenReader(report.FilePath)
	if err != nil {
		t.Fatalf("report is not a valid xlsx: %v", err)
	}
	defer zr.Close()
	var all strings.Builder
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		all.Write(b)
		if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.Contains(string(b), "<f>") {
			t.Fatalf("worksheet %s contains a formula", f.Name)
		}
	}
	content := all.String()
	for _, leak := range []string{"victim@example.com", "203.0.113.50", "Victor", "HYPERLINK"} {
		if strings.Contains(content, leak) {
			t.Errorf("anonymized report leaks %q", leak)
		}
	}
}
