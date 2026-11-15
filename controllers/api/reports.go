package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/reports"
	"github.com/gorilla/mux"
)

// Report services, set up by InitReportServices
var jobService *reports.JobService
var fileManager *reports.FileManager

// ReportStats contains statistics about a user's reports
type ReportStats struct {
	Total      int `json:"total"`
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
}

// ReportSummary contains summary information about a report
type ReportSummary struct {
	ID            int64   `json:"id"`
	Format        string  `json:"format"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
	FileName      string  `json:"file_name,omitempty"`
	FileSize      int64   `json:"file_size,omitempty"`
	CampaignCount int     `json:"campaign_count"`
	CampaignSetID *int64  `json:"campaign_set_id,omitempty"`
	ErrorMessage  string  `json:"error_message,omitempty"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
}

// ReportListResponse contains a list of reports for a user
type ReportListResponse struct {
	Reports []ReportSummary `json:"reports"`
	Stats   ReportStats     `json:"stats"`
}

// InitReportServices initializes and starts the report services. The job
// service and the download handlers share one file manager, so both use
// the configured storage path. Everything else is edited in
// Settings > Report Generation.
func InitReportServices(reportConfig *config.Reports) {
	storagePath := ""
	if reportConfig != nil {
		storagePath = reportConfig.StoragePath
	}
	if reports.LoadSettings().OnlineGeolocation {
		log.Warn("Online IP geolocation is enabled: target IP addresses are sent to freeipapi.com when reports are generated")
	}
	fileManager = reports.NewFileManager(storagePath)
	jobService = reports.NewJobService(fileManager)
	jobService.Start()
}

// StopReportServices stops the report services
func StopReportServices() {
	if jobService != nil {
		jobService.Stop()
	}
}

func requestUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	user, ok := ctx.Get(r, "user").(models.User)
	if !ok {
		log.Error("Error getting user from context")
		JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
	}
	return user, ok
}

func reportIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid report ID"}, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func methodNotAllowed(w http.ResponseWriter) {
	JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
}

// queueReport validates a request and queues it. It writes the response.
func queueReport(w http.ResponseWriter, uid int64, req reports.AsyncReportRequest) {
	if !req.Format.Valid() {
		JSONResponse(w, models.Response{Success: false, Message: "Unsupported format: " + string(req.Format)}, http.StatusBadRequest)
		return
	}

	var jobID int64
	var err error
	if req.CampaignSetID != nil && *req.CampaignSetID > 0 {
		campaignSet, cerr := models.GetCampaignSet(*req.CampaignSetID, uid)
		if cerr != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign set not found or access denied"}, http.StatusBadRequest)
			return
		}
		if len(campaignSet.Campaigns) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign set has no campaigns"}, http.StatusBadRequest)
			return
		}
		ids := make([]int64, len(campaignSet.Campaigns))
		for i, c := range campaignSet.Campaigns {
			ids[i] = c.Id
		}
		jobID, err = jobService.QueueCampaignSetReport(uid, campaignSet.Id, ids, req.Format, req.Options)
	} else {
		if len(req.CampaignIDs) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "No campaigns selected"}, http.StatusBadRequest)
			return
		}
		valid := []int64{}
		seen := map[int64]bool{}
		for _, id := range req.CampaignIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, cerr := models.GetCampaign(id, uid); cerr != nil {
				log.Warnf("Report request for campaign %d by user %d denied: %v", id, uid, cerr)
				continue
			}
			valid = append(valid, id)
		}
		if len(valid) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "No valid campaigns found"}, http.StatusBadRequest)
			return
		}
		jobID, err = jobService.QueueReport(uid, valid, req.Format, req.Options)
	}
	if err != nil {
		log.Errorf("Error queueing report: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to queue report"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, reports.AsyncReportResponse{JobID: jobID, Status: models.ReportStatusQueued, Message: "Report queued successfully"}, http.StatusOK)
}

// QueueReport queues a new report (campaigns or a campaign set) for generation
func (as *Server) QueueReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	var req reports.AsyncReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
		return
	}
	queueReport(w, user.Id, req)
}

// GenerateCampaignSetReport queues a report for a campaign set. Kept for
// API compatibility; it is equivalent to POST /api/reports/queue with a
// campaign_set_id.
func (as *Server) GenerateCampaignSetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	var req reports.AsyncReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
		return
	}
	if req.CampaignSetID == nil || *req.CampaignSetID <= 0 {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_set_id is required"}, http.StatusBadRequest)
		return
	}
	queueReport(w, user.Id, req)
}

// ListReports returns all reports for the current user
func (as *Server) ListReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}

	reportList, err := models.GetReports(user.Id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving reports"}, http.StatusInternalServerError)
		return
	}
	statsMap, err := models.GetReportStats(user.Id)
	if err != nil {
		log.Errorf("Error getting report stats for user %d: %v", user.Id, err)
	}
	stats := ReportStats{
		Total:      statsMap["total"],
		Queued:     statsMap[models.ReportStatusQueued],
		Processing: statsMap[models.ReportStatusProcessing],
		Completed:  statsMap[models.ReportStatusCompleted],
		Failed:     statsMap[models.ReportStatusFailed],
	}

	summaries := make([]ReportSummary, len(reportList))
	for i, report := range reportList {
		summary := ReportSummary{
			ID:            report.Id,
			Format:        report.Format,
			Status:        report.Status,
			CreatedAt:     report.CreatedAt.Format(time.RFC3339),
			FileName:      report.FileName,
			FileSize:      report.FileSize,
			ErrorMessage:  report.ErrorMessage,
			CampaignSetID: report.CampaignSetId,
		}
		if report.StartedAt != nil {
			startedAt := report.StartedAt.Format(time.RFC3339)
			summary.StartedAt = &startedAt
		}
		if report.CompletedAt != nil {
			completedAt := report.CompletedAt.Format(time.RFC3339)
			summary.CompletedAt = &completedAt
		}
		if report.ExpiresAt != nil {
			expiresAt := report.ExpiresAt.Format(time.RFC3339)
			summary.ExpiresAt = &expiresAt
		}
		if ids, err := report.GetCampaignIdsSlice(); err == nil {
			summary.CampaignCount = len(ids)
		}
		summaries[i] = summary
	}
	JSONResponse(w, ReportListResponse{Reports: summaries, Stats: stats}, http.StatusOK)
}

// GetReportStatus returns the current status of a report job
func (as *Server) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	reportID, ok := reportIDFromRequest(w, r)
	if !ok {
		return
	}
	status, err := jobService.GetJobStatus(reportID, user.Id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
		return
	}
	JSONResponse(w, status, http.StatusOK)
}

// DownloadReport streams a completed report file
func (as *Server) DownloadReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	reportID, ok := reportIDFromRequest(w, r)
	if !ok {
		return
	}
	report, err := models.GetReport(reportID, user.Id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
		return
	}
	if report.Status != models.ReportStatusCompleted {
		JSONResponse(w, models.Response{Success: false, Message: "Report not ready for download"}, http.StatusBadRequest)
		return
	}
	if report.FilePath == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Report file not found"}, http.StatusNotFound)
		return
	}

	f, info, err := fileManager.OpenReportFile(report.FilePath)
	if err != nil {
		log.Errorf("Error opening report file %s: %v", report.FilePath, err)
		JSONResponse(w, models.Response{Success: false, Message: "Report file not found"}, http.StatusNotFound)
		return
	}
	defer f.Close()

	format := reports.ReportFormat(report.Format)
	fileName := report.FileName
	if fileName == "" {
		fileName = fmt.Sprintf("report_%d.%s", report.Id, format.Extension())
	}
	w.Header().Set("Content-Type", format.ContentType())
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, fileName, info.ModTime(), f)
}

// DeleteReport deletes a report and its file
func (as *Server) DeleteReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	reportID, ok := reportIDFromRequest(w, r)
	if !ok {
		return
	}
	report, err := models.GetReport(reportID, user.Id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
		return
	}
	if report.FilePath != "" {
		if err := fileManager.DeleteReportFile(report.FilePath); err != nil {
			log.Errorf("Error deleting report file %s: %v", report.FilePath, err)
		}
	}
	if err := models.DeleteReport(reportID, user.Id); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error deleting report"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Report deleted successfully"}, http.StatusOK)
}

// CheckDependencies checks if all required Python dependencies are installed
func (as *Server) CheckDependencies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	status, err := jobService.Python().CheckDependencies(r.Context())
	if err != nil {
		log.Errorf("Error checking dependencies: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Error checking dependencies: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, status, http.StatusOK)
}

// ValidationRequest represents a request to validate campaigns
type ValidationRequest struct {
	CampaignIDs []int64 `json:"campaign_ids"`
}

// ValidationResponse wraps the validation result for API responses
type ValidationResponse struct {
	Valid               bool     `json:"valid"`
	Errors              []string `json:"errors"`
	Warnings            []string `json:"warnings"`
	IncompleteCampaigns []string `json:"incomplete_campaigns"`
	MissingTimelineData []string `json:"missing_timeline_data"`
	Message             string   `json:"message"`
}

// ValidateCampaigns validates campaigns before report generation
func (as *Server) ValidateCampaigns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
		return
	}
	if len(req.CampaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaigns selected"}, http.StatusBadRequest)
		return
	}
	validation, err := reports.ValidateCampaignData(req.CampaignIDs, user.Id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error validating campaigns: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	message := "All campaigns are ready for report generation"
	if !validation.Valid {
		message = "Validation failed: campaigns cannot be used for report generation"
	} else if len(validation.Warnings) > 0 {
		message = fmt.Sprintf("Validation passed with %d warning(s)", len(validation.Warnings))
	}
	JSONResponse(w, ValidationResponse{
		Valid:               validation.Valid,
		Errors:              validation.Errors,
		Warnings:            validation.Warnings,
		IncompleteCampaigns: validation.IncompleteCampaigns,
		MissingTimelineData: validation.MissingTimelineData,
		Message:             message,
	}, http.StatusOK)
}

// InstallDependencies installs Python dependencies from requirements.txt
func (as *Server) InstallDependencies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := requestUser(w, r)
	if !ok {
		return
	}
	if isAdmin, _ := user.HasPermission(models.PermissionModifySystem); !isAdmin {
		log.Errorf("User %s attempted to install dependencies without admin permissions", user.Username)
		JSONResponse(w, models.Response{Success: false, Message: "Admin permissions required to install dependencies"}, http.StatusForbidden)
		return
	}

	// Not tied to the request: aborting pip midway would leave a broken venv
	result, err := jobService.Python().InstallDependencies(context.Background())
	if errors.Is(err, reports.ErrInstallInProgress) {
		JSONResponse(w, models.Response{Success: false, Message: "A dependency installation is already running or a report is being generated. Try again shortly."}, http.StatusConflict)
		return
	}
	if err != nil {
		log.Errorf("Error installing dependencies: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Error installing dependencies: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, result, http.StatusOK)
}
