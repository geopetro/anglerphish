package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/reports"
	"github.com/gorilla/mux"
)

// Global job service instance - will be initialized in main
var jobService *reports.JobService
var fileManager *reports.FileManager

// ReportRequest contains the parameters for generating a report
type ReportRequest struct {
	CampaignIDs []int64               `json:"campaign_ids"`
	Format      reports.ReportFormat  `json:"format"`
	Options     reports.ReportOptions `json:"options"`
}

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

// InitReportServices initializes the report services
func InitReportServices(reportConfig *config.Reports) {
	jobService = reports.NewJobService()

	// Use config path if available, otherwise fall back to default
	if reportConfig != nil && reportConfig.StoragePath != "" {
		fileManager = reports.NewFileManagerWithPath(reportConfig.StoragePath)
	} else {
		fileManager = reports.NewFileManager()
	}

	jobService.Start()
}

// StopReportServices stops the report services
func StopReportServices() {
	if jobService != nil {
		jobService.Stop()
	}
}

// QueueReport queues a new report for async generation
func (as *Server) QueueReport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Parse the request
		var req reports.AsyncReportRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Errorf("Error parsing report request: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
			return
		}

		// Get the current user from the request context first
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Handle campaign set if provided
		campaignIDs := req.CampaignIDs
		if req.CampaignSetID != nil && *req.CampaignSetID > 0 {
			log.Infof("Async report request received for campaign set ID: %d", *req.CampaignSetID)

			// Fetch the campaign set
			campaignSet, err := models.GetCampaignSet(*req.CampaignSetID, uid)
			if err != nil {
				log.Errorf("Campaign set ID %d access check failed: %v", *req.CampaignSetID, err)
				JSONResponse(w, models.Response{Success: false, Message: "Campaign set not found or access denied"}, http.StatusBadRequest)
				return
			}

			// Extract campaign IDs from the campaign set
			if len(campaignSet.Campaigns) == 0 {
				log.Errorf("Campaign set ID %d has no campaigns", *req.CampaignSetID)
				JSONResponse(w, models.Response{Success: false, Message: "Campaign set has no campaigns"}, http.StatusBadRequest)
				return
			}

			campaignIDs = make([]int64, len(campaignSet.Campaigns))
			for i, campaign := range campaignSet.Campaigns {
				campaignIDs[i] = campaign.Id
			}

			log.Infof("Campaign set %s contains %d campaigns", campaignSet.Name, len(campaignIDs))
		}

		// Debug log the request
		campaignIDsStr := ""
		for i, id := range campaignIDs {
			if i > 0 {
				campaignIDsStr += ", "
			}
			campaignIDsStr += fmt.Sprintf("%d", id)
		}
		log.Infof("Async report request received - Format: %s, Campaign IDs: [%s]", req.Format, campaignIDsStr)

		// Validate format
		if req.Format != reports.FormatWord && req.Format != reports.FormatExcel {
			log.Errorf("Invalid report format requested: %s", req.Format)
			JSONResponse(w, models.Response{Success: false, Message: "Unsupported format: " + string(req.Format)}, http.StatusBadRequest)
			return
		}

		// Validate campaign IDs
		if len(campaignIDs) == 0 {
			log.Error("Report request received with no campaign IDs")
			JSONResponse(w, models.Response{Success: false, Message: "No campaigns selected"}, http.StatusBadRequest)
			return
		}

		// Check if user has access to these campaigns
		validCampaignIDs := []int64{}
		for _, id := range campaignIDs {
			campaign, err := models.GetCampaign(id, uid)
			if err != nil {
				log.Errorf("Campaign ID %d access check failed: %v", id, err)
				continue
			}

			log.Infof("Campaign ID %d (%s) validated successfully (status: %s)", id, campaign.Name, campaign.Status)
			validCampaignIDs = append(validCampaignIDs, id)
		}

		// If no valid campaigns found, return error
		if len(validCampaignIDs) == 0 {
			log.Error("No valid campaigns found for report generation")
			JSONResponse(w, models.Response{Success: false, Message: "No valid campaigns found"}, http.StatusBadRequest)
			return
		}

		// Queue the report
		jobID, err := jobService.QueueReport(uid, validCampaignIDs, req.Format, req.Options)
		if err != nil {
			log.Errorf("Error queueing report: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Failed to queue report: " + err.Error()}, http.StatusInternalServerError)
			return
		}

		response := reports.AsyncReportResponse{
			JobID:   jobID,
			Status:  "queued",
			Message: "Report queued successfully",
		}

		JSONResponse(w, response, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// GenerateReportLegacy creates a new report and returns it to the client (legacy sync method)
func (as *Server) GenerateReportLegacy(w http.ResponseWriter, r *http.Request) {
	// Parse the request - could be JSON or form data
	var req ReportRequest
	contentType := r.Header.Get("Content-Type")
	var err error

	if strings.Contains(contentType, "application/json") {
		// Handle JSON request
		err = json.NewDecoder(r.Body).Decode(&req)
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
		strings.Contains(contentType, "multipart/form-data") {
		// Handle form submission
		err = r.ParseForm()
		if err == nil {
			optionsStr := r.FormValue("options")
			if optionsStr == "" {
				log.Error("Missing options field in form data")
				JSONResponse(w, models.Response{Success: false, Message: "Missing options field in form data"}, http.StatusBadRequest)
				return
			}
			err = json.Unmarshal([]byte(optionsStr), &req)
		}
	} else {
		// Handle direct JSON as fallback
		err = json.NewDecoder(r.Body).Decode(&req)
	}

	if err != nil {
		log.Errorf("Error parsing report request: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
		return
	}

	// Debug log the request
	campaignIDsStr := ""
	for i, id := range req.CampaignIDs {
		if i > 0 {
			campaignIDsStr += ", "
		}
		campaignIDsStr += fmt.Sprintf("%d", id)
	}
	log.Infof("Legacy report request received - Format: %s, Campaign IDs: [%s]", req.Format, campaignIDsStr)

	// Validate format
	if req.Format != reports.FormatWord && req.Format != reports.FormatExcel {
		log.Errorf("Invalid report format requested: %s", req.Format)
		JSONResponse(w, models.Response{Success: false, Message: "Unsupported format: " + string(req.Format)}, http.StatusBadRequest)
		return
	}

	// Validate campaign IDs
	if len(req.CampaignIDs) == 0 {
		log.Error("Report request received with no campaign IDs")
		JSONResponse(w, models.Response{Success: false, Message: "No campaigns selected"}, http.StatusBadRequest)
		return
	}

	// Get the current user from the request context
	user, ok := ctx.Get(r, "user").(models.User)
	if !ok {
		log.Error("Error getting user from context")
		JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
		return
	}
	uid := user.Id

	// Check if user has access to these campaigns and that they are completed
	validCampaignIDs := []int64{}
	for _, id := range req.CampaignIDs {
		campaign, err := models.GetCampaign(id, uid)
		if err != nil {
			log.Errorf("Campaign ID %d access check failed: %v", id, err)
			// Instead of immediately failing, we'll collect valid campaigns
			continue
		}

		// Include all campaigns regardless of status
		log.Infof("Campaign ID %d (%s) validated successfully (status: %s)", id, campaign.Name, campaign.Status)
		validCampaignIDs = append(validCampaignIDs, id)
	}

	// If no valid campaigns found, return error
	if len(validCampaignIDs) == 0 {
		log.Error("No valid campaigns found for report generation")
		JSONResponse(w, models.Response{Success: false, Message: "No valid campaigns found"}, http.StatusBadRequest)
		return
	}

	// Replace original campaign IDs with validated ones
	req.CampaignIDs = validCampaignIDs

	// Generate the report
	log.Infof("Generating %s report for %d campaigns", req.Format, len(req.CampaignIDs))
	reportData, err := reports.GenerateReport(req.Format, req.CampaignIDs, req.Options)
	if err != nil {
		log.Errorf("Report generation failed: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Report generation failed: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	log.Infof("Report generated successfully - Size: %d bytes", len(reportData))

	// Verify the report data is not empty
	if len(reportData) == 0 {
		log.Error("Report generation produced empty data")
		JSONResponse(w, models.Response{Success: false, Message: "Report generation produced empty data"}, http.StatusInternalServerError)
		return
	}

	// Set appropriate headers for file download
	filename := "gophish_report"
	extension := ".docx"
	downloadContentType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	if req.Format == reports.FormatExcel {
		extension = ".xlsx"
		downloadContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	// Set CORS headers for XMLHttpRequest
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRFToken")

	w.Header().Set("Content-Type", downloadContentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename+extension)
	w.Header().Set("Content-Length", strconv.Itoa(len(reportData)))

	// Write the report data in chunks to ensure complete transfer
	bytesWritten, err := w.Write(reportData)
	if err != nil {
		log.Errorf("Error writing report data to response: %v", err)
		// At this point headers are already sent, so we can't change the status code
		return
	}

	if bytesWritten != len(reportData) {
		log.Errorf("Not all bytes were written: %d of %d", bytesWritten, len(reportData))
		// Can't do much here as headers are already sent
	} else {
		log.Infof("Successfully wrote all %d bytes to response", bytesWritten)
	}
}

// ListReports returns all reports for the current user and handles legacy report generation
func (as *Server) ListReports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Handle legacy sync report generation (backwards compatibility)
		as.GenerateReportLegacy(w, r)
		return
	case "GET":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Get reports
		reports, err := models.GetReports(uid)
		if err != nil {
			log.Errorf("Error getting reports for user %d: %v", uid, err)
			JSONResponse(w, models.Response{Success: false, Message: "Error retrieving reports"}, http.StatusInternalServerError)
			return
		}

		// Get stats
		statsMap, err := models.GetReportStats(uid)
		if err != nil {
			log.Errorf("Error getting report stats for user %d: %v", uid, err)
			statsMap = map[string]int{"total": 0, "queued": 0, "processing": 0, "completed": 0, "failed": 0}
		}

		stats := ReportStats{
			Total:      statsMap["total"],
			Queued:     statsMap["queued"],
			Processing: statsMap["processing"],
			Completed:  statsMap["completed"],
			Failed:     statsMap["failed"],
		}

		// Convert to summary format
		summaries := make([]ReportSummary, len(reports))
		for i, report := range reports {
			summary := ReportSummary{
				ID:           report.Id,
				Format:       report.Format,
				Status:       report.Status,
				CreatedAt:    report.CreatedAt.Format(time.RFC3339),
				FileName:     report.FileName,
				FileSize:     report.FileSize,
				ErrorMessage: report.ErrorMessage,
			}

			// Add optional timestamps
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

			// Get campaign count
			campaignIds, err := report.GetCampaignIdsSlice()
			if err == nil {
				summary.CampaignCount = len(campaignIds)
			}

			// Set campaign set ID if applicable
			if report.CampaignSetId != nil {
				summary.CampaignSetID = report.CampaignSetId
			}

			summaries[i] = summary
		}

		response := ReportListResponse{
			Reports: summaries,
			Stats:   stats,
		}

		JSONResponse(w, response, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// GetReportStatus returns the current status of a report job
func (as *Server) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Get report ID from URL path
		vars := mux.Vars(r)
		reportId, err := strconv.ParseInt(vars["id"], 0, 64)
		if err != nil {
			log.Errorf("Error parsing report ID: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid report ID"}, http.StatusBadRequest)
			return
		}

		// Get job status
		status, err := jobService.GetJobStatus(reportId, uid)
		if err != nil {
			log.Errorf("Error getting job status for report %d: %v", reportId, err)
			JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
			return
		}

		JSONResponse(w, status, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// DownloadReport serves a completed report for download
func (as *Server) DownloadReport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Get report ID from URL path
		vars := mux.Vars(r)
		reportId, err := strconv.ParseInt(vars["id"], 0, 64)
		if err != nil {
			log.Errorf("Error parsing report ID: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid report ID"}, http.StatusBadRequest)
			return
		}

		// Get the report
		report, err := models.GetReport(reportId, uid)
		if err != nil {
			log.Errorf("Error getting report %d: %v", reportId, err)
			JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
			return
		}

		// Check if report is completed
		if report.Status != models.ReportStatusCompleted {
			log.Errorf("Report %d is not completed (status: %s)", reportId, report.Status)
			JSONResponse(w, models.Response{Success: false, Message: "Report not ready for download"}, http.StatusBadRequest)
			return
		}

		// Check if file exists
		if report.FilePath == "" {
			log.Errorf("Report %d has no file path", reportId)
			JSONResponse(w, models.Response{Success: false, Message: "Report file not found"}, http.StatusNotFound)
			return
		}

		// Read the file
		reportData, err := fileManager.GetReportFile(report.FilePath)
		if err != nil {
			log.Errorf("Error reading report file %s: %v", report.FilePath, err)
			JSONResponse(w, models.Response{Success: false, Message: "Error reading report file"}, http.StatusInternalServerError)
			return
		}

		// Set appropriate headers for download
		var contentType string
		if report.Format == "excel" {
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else {
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}

		fileName := report.FileName
		if fileName == "" {
			// Fallback filename
			extension := "docx"
			if report.Format == "excel" {
				extension = "xlsx"
			}
			fileName = fmt.Sprintf("report_%d.%s", report.Id, extension)
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Header().Set("Content-Length", strconv.Itoa(len(reportData)))

		// Write the report data
		bytesWritten, err := w.Write(reportData)
		if err != nil {
			log.Errorf("Error writing report data to response: %v", err)
			return
		}

		if bytesWritten != len(reportData) {
			log.Errorf("Not all bytes were written: %d of %d", bytesWritten, len(reportData))
		} else {
			log.Infof("Successfully served report %d (%d bytes)", reportId, bytesWritten)
		}

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// DeleteReport deletes a report and its file
func (as *Server) DeleteReport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Get report ID from URL path
		vars := mux.Vars(r)
		reportId, err := strconv.ParseInt(vars["id"], 0, 64)
		if err != nil {
			log.Errorf("Error parsing report ID: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid report ID"}, http.StatusBadRequest)
			return
		}

		// Get the report to check file path
		report, err := models.GetReport(reportId, uid)
		if err != nil {
			log.Errorf("Error getting report %d: %v", reportId, err)
			JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
			return
		}

		// Delete the file if it exists
		if report.FilePath != "" {
			err = fileManager.DeleteReportFile(report.FilePath)
			if err != nil {
				log.Errorf("Error deleting report file %s: %v", report.FilePath, err)
				// Don't fail the request if file deletion fails
			}
		}

		// Delete the database record
		err = models.DeleteReport(reportId, uid)
		if err != nil {
			log.Errorf("Error deleting report %d: %v", reportId, err)
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting report"}, http.StatusInternalServerError)
			return
		}

		log.Infof("Successfully deleted report %d", reportId)
		JSONResponse(w, models.Response{Success: true, Message: "Report deleted successfully"}, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// CheckDependencies checks if all required Python dependencies are installed
func (as *Server) CheckDependencies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		service := reports.NewPythonService()
		status, err := service.CheckDependencies()
		if err != nil {
			log.Errorf("Error checking dependencies: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Error checking dependencies: " + err.Error()}, http.StatusInternalServerError)
			return
		}

		JSONResponse(w, status, http.StatusOK)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
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
	switch r.Method {
	case "POST":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Parse the request
		var req ValidationRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Errorf("Error parsing validation request: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
			return
		}

		// Validate campaign IDs
		if len(req.CampaignIDs) == 0 {
			log.Error("Validation request received with no campaign IDs")
			JSONResponse(w, models.Response{Success: false, Message: "No campaigns selected"}, http.StatusBadRequest)
			return
		}

		// Check user access to campaigns first
		validCampaignIDs := []int64{}
		for _, id := range req.CampaignIDs {
			_, err := models.GetCampaign(id, uid)
			if err != nil {
				log.Errorf("Campaign ID %d access check failed: %v", id, err)
				continue
			}
			validCampaignIDs = append(validCampaignIDs, id)
		}

		if len(validCampaignIDs) == 0 {
			log.Error("No valid campaigns found for validation")
			JSONResponse(w, ValidationResponse{
				Valid:   false,
				Errors:  []string{"No valid campaigns found - access denied or campaigns do not exist"},
				Message: "Validation failed",
			}, http.StatusOK)
			return
		}

		// Perform deep validation
		validation, err := reports.ValidateCampaignData(validCampaignIDs)
		if err != nil {
			log.Errorf("Error validating campaigns: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Error validating campaigns: " + err.Error()}, http.StatusInternalServerError)
			return
		}

		// Build response message
		message := "Validation complete"
		if !validation.Valid {
			message = "Validation failed: campaigns cannot be used for report generation"
		} else if len(validation.Warnings) > 0 {
			message = fmt.Sprintf("Validation passed with %d warning(s)", len(validation.Warnings))
		} else {
			message = "All campaigns are ready for report generation"
		}

		response := ValidationResponse{
			Valid:               validation.Valid,
			Errors:              validation.Errors,
			Warnings:            validation.Warnings,
			IncompleteCampaigns: validation.IncompleteCampaigns,
			MissingTimelineData: validation.MissingTimelineData,
			Message:             message,
		}

		JSONResponse(w, response, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// InstallDependencies installs Python dependencies from requirements.txt
func (as *Server) InstallDependencies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}

		// Check if user has admin permissions
		isAdmin, _ := user.HasPermission(models.PermissionModifySystem)
		if !isAdmin {
			log.Errorf("User %s attempted to install dependencies without admin permissions", user.Username)
			JSONResponse(w, models.Response{Success: false, Message: "Admin permissions required to install dependencies"}, http.StatusForbidden)
			return
		}

		log.Info("Installing all dependencies from requirements.txt")

		service := reports.NewPythonService()
		result, err := service.InstallDependencies(nil)
		if err != nil {
			log.Errorf("Error installing dependencies: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Error installing dependencies: " + err.Error()}, http.StatusInternalServerError)
			return
		}

		JSONResponse(w, result, http.StatusOK)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// GenerateCampaignSetReport creates a new report for a campaign set and returns it to the client
func (as *Server) GenerateCampaignSetReport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Parse the request
		var req reports.CampaignSetReportRequest
		contentType := r.Header.Get("Content-Type")
		var err error

		if strings.Contains(contentType, "application/json") {
			// Handle JSON request
			err = json.NewDecoder(r.Body).Decode(&req)
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
			strings.Contains(contentType, "multipart/form-data") {
			// Handle form submission
			err = r.ParseForm()
			if err == nil {
				optionsStr := r.FormValue("options")
				if optionsStr == "" {
					log.Error("Missing options field in form data")
					JSONResponse(w, models.Response{Success: false, Message: "Missing options field in form data"}, http.StatusBadRequest)
					return
				}
				err = json.Unmarshal([]byte(optionsStr), &req)
			}
		} else {
			// Handle direct JSON as fallback
			err = json.NewDecoder(r.Body).Decode(&req)
		}

		if err != nil {
			log.Errorf("Error parsing campaign set report request: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request format: " + err.Error()}, http.StatusBadRequest)
			return
		}

		// Debug log the request
		log.Infof("Campaign set report request received - Format: %s, Campaign Set ID: %d", req.Format, req.CampaignSetID)

		// Validate format
		if req.Format != reports.FormatWord && req.Format != reports.FormatExcel {
			log.Errorf("Invalid report format requested: %s", req.Format)
			JSONResponse(w, models.Response{Success: false, Message: "Unsupported format: " + string(req.Format)}, http.StatusBadRequest)
			return
		}

		// Get the current user from the request context
		user, ok := ctx.Get(r, "user").(models.User)
		if !ok {
			log.Error("Error getting user from context")
			JSONResponse(w, models.Response{Success: false, Message: "Authentication failed"}, http.StatusUnauthorized)
			return
		}
		uid := user.Id

		// Check if user has access to this campaign set
		campaignSet, err := models.GetCampaignSet(req.CampaignSetID, uid)
		if err != nil {
			log.Errorf("Campaign set ID %d access check failed: %v", req.CampaignSetID, err)
			JSONResponse(w, models.Response{Success: false, Message: "Campaign set not found"}, http.StatusBadRequest)
			return
		}

		// Verify at least one campaign exists in the set
		if len(campaignSet.Campaigns) == 0 {
			log.Errorf("Campaign set ID %d has no campaigns", req.CampaignSetID)
			JSONResponse(w, models.Response{Success: false, Message: "Campaign set has no campaigns"}, http.StatusBadRequest)
			return
		}

		log.Infof("Campaign set ID %d (%s) validated successfully with %d campaigns", req.CampaignSetID, campaignSet.Name, len(campaignSet.Campaigns))

		// Generate the report
		log.Infof("Generating %s report for campaign set %d", req.Format, req.CampaignSetID)
		reportData, err := reports.GenerateReportForCampaignSet(req.Format, req.CampaignSetID, req.Options)
		if err != nil {
			log.Errorf("Report generation failed: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Report generation failed: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("Report generated successfully - Size: %d bytes", len(reportData))

		// Verify the report data is not empty
		if len(reportData) == 0 {
			log.Error("Report generation produced empty data")
			JSONResponse(w, models.Response{Success: false, Message: "Report generation produced empty data"}, http.StatusInternalServerError)
			return
		}

		// Set appropriate headers for file download
		filename := "anglerphish_campaign_set_report"
		extension := ".docx"
		downloadContentType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

		if req.Format == reports.FormatExcel {
			extension = ".xlsx"
			downloadContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}

		// Set CORS headers for XMLHttpRequest
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRFToken")

		w.Header().Set("Content-Type", downloadContentType)
		w.Header().Set("Content-Disposition", "attachment; filename="+filename+extension)
		w.Header().Set("Content-Length", strconv.Itoa(len(reportData)))

		// Write the report data in chunks to ensure complete transfer
		bytesWritten, err := w.Write(reportData)
		if err != nil {
			log.Errorf("Error writing report data to response: %v", err)
			// At this point headers are already sent, so we can't change the status code
			return
		}

		if bytesWritten != len(reportData) {
			log.Errorf("Not all bytes were written: %d of %d", bytesWritten, len(reportData))
			// Can't do much here as headers are already sent
		} else {
			log.Infof("Successfully wrote all %d bytes to response", bytesWritten)
		}
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}
