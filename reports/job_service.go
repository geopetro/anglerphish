package reports

import (
	"fmt"
	"sync"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// JobService handles background processing of report generation jobs
type JobService struct {
	pythonService *PythonService
	fileManager   *FileManager
	running       bool
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mutex         sync.RWMutex
}

// JobStatus represents the current status of a job
type JobStatus struct {
	ID          int64   `json:"id"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"` // 0-100
	Message     string  `json:"message"`
	CreatedAt   string  `json:"created_at"`
	StartedAt   *string `json:"started_at,omitempty"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// NewJobService creates a new job service instance
func NewJobService() *JobService {
	return &JobService{
		pythonService: NewPythonService(),
		fileManager:   NewFileManager(),
		stopChan:      make(chan struct{}),
	}
}

// Start begins the job processing loop
func (js *JobService) Start() {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	if js.running {
		return
	}

	js.running = true
	log.Info("Starting report job service")

	js.wg.Add(1)
	go js.processJobsLoop()
}

// Stop gracefully stops the job processing
func (js *JobService) Stop() {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	if !js.running {
		return
	}

	log.Info("Stopping report job service")
	close(js.stopChan)
	js.running = false

	// Wait for background goroutine to finish
	js.wg.Wait()
	log.Info("Report job service stopped")
}

// IsRunning returns whether the service is currently running
func (js *JobService) IsRunning() bool {
	js.mutex.RLock()
	defer js.mutex.RUnlock()
	return js.running
}

// processJobsLoop is the main processing loop that runs in a goroutine
func (js *JobService) processJobsLoop() {
	defer js.wg.Done()

	ticker := time.NewTicker(5 * time.Second) // Check for new jobs every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-js.stopChan:
			log.Info("Job processing loop stopped")
			return
		case <-ticker.C:
			js.processQueuedJobs()
		}
	}
}

// processQueuedJobs processes all jobs in the queue
func (js *JobService) processQueuedJobs() {
	queuedReports, err := models.GetQueuedReports()
	if err != nil {
		log.Errorf("Error getting queued reports: %v", err)
		return
	}

	if len(queuedReports) == 0 {
		return
	}

	log.Infof("Processing %d queued reports", len(queuedReports))

	for _, report := range queuedReports {
		select {
		case <-js.stopChan:
			log.Info("Job processing interrupted by stop signal")
			return
		default:
			js.processReport(report)
		}
	}
}

// processReport processes a single report
func (js *JobService) processReport(report models.Report) {
	log.Infof("Processing report %d for user %d", report.Id, report.UserId)

	// Update status to processing
	err := models.UpdateReportStatus(report.Id, models.ReportStatusProcessing)
	if err != nil {
		log.Errorf("Error updating report %d status to processing: %v", report.Id, err)
		return
	}

	// Generate the report
	err = js.generateReport(report)
	if err != nil {
		log.Errorf("Error generating report %d: %v", report.Id, err)
		models.UpdateReportError(report.Id, fmt.Sprintf("Generation failed: %v", err))
		return
	}

	log.Infof("Successfully completed report %d", report.Id)
}

// generateReport handles the actual report generation
func (js *JobService) generateReport(report models.Report) error {
	// Parse options
	options, err := report.GetOptions()
	if err != nil {
		return fmt.Errorf("error parsing report options: %v", err)
	}

	// Convert options to ReportOptions struct
	reportOptions := ReportOptions{
		AnonymizeEmails:      getBoolFromOptions(options, "anonymize_emails", false),
		AnonymizeIPs:         getBoolFromOptions(options, "anonymize_ips", false),
		IncludeGDPRStatement: getBoolFromOptions(options, "include_gdpr_statement", false),
		IncludeTOC:           getBoolFromOptions(options, "include_toc", false),
	}

	// Determine the format
	format := FormatWord
	if report.Format == "excel" {
		format = FormatExcel
	}

	var reportData []byte

	// Generate report for either campaign set or individual campaigns
	if report.CampaignSetId != nil {
		log.Infof("Generating report for campaign set %d", *report.CampaignSetId)
		// Note: Validation for campaign sets would need to be implemented separately
		// For now, we proceed with generation
		reportData, err = GenerateReportForCampaignSet(format, *report.CampaignSetId, reportOptions)
	} else {
		log.Infof("Generating report for campaigns: %s", report.CampaignIds)
		campaignIds, err := report.GetCampaignIdsSlice()
		if err != nil {
			return fmt.Errorf("error parsing campaign IDs: %v", err)
		}

		// Validate campaigns before generation
		validation, err := ValidateCampaignData(campaignIds)
		if err != nil {
			return fmt.Errorf("error validating campaigns: %v", err)
		}

		// If validation found errors, fail the report
		if !validation.Valid {
			errorMsg := "Campaign validation failed"
			if len(validation.Errors) > 0 {
				errorMsg = fmt.Sprintf("%s: %s", errorMsg, validation.Errors[0])
			}
			return fmt.Errorf("%s", errorMsg)
		}

		// Log warnings but proceed with generation
		if len(validation.Warnings) > 0 {
			log.Warnf("Report generation proceeding with %d warnings:", len(validation.Warnings))
			for _, warning := range validation.Warnings {
				log.Warnf("  - %s", warning)
			}
		}

		reportData, err = GenerateReport(format, campaignIds, reportOptions)
	}

	if err != nil {
		return fmt.Errorf("report generation failed: %v", err)
	}

	if len(reportData) == 0 {
		return fmt.Errorf("report generation produced empty data")
	}

	// Save the report file
	fileName := fmt.Sprintf("report_%d_%d.%s", report.Id, time.Now().Unix(), getFileExtension(format))
	filePath, err := js.fileManager.SaveReport(report.Id, fileName, reportData)
	if err != nil {
		return fmt.Errorf("error saving report file: %v", err)
	}

	// Update the report with file information
	err = models.UpdateReportFile(report.Id, filePath, fileName, int64(len(reportData)))
	if err != nil {
		return fmt.Errorf("error updating report file info: %v", err)
	}

	// Mark as completed
	err = models.UpdateReportStatus(report.Id, models.ReportStatusCompleted)
	if err != nil {
		return fmt.Errorf("error updating report status to completed: %v", err)
	}

	return nil
}

// GetJobStatus returns the current status of a job
func (js *JobService) GetJobStatus(reportId int64, userId int64) (*JobStatus, error) {
	report, err := models.GetReport(reportId, userId)
	if err != nil {
		return nil, err
	}

	status := &JobStatus{
		ID:        report.Id,
		Status:    report.Status,
		CreatedAt: report.CreatedAt.Format(time.RFC3339),
		Error:     report.ErrorMessage,
	}

	// Set progress based on status
	switch report.Status {
	case models.ReportStatusQueued:
		status.Progress = 0
		status.Message = "Report queued for generation"
	case models.ReportStatusProcessing:
		status.Progress = 50
		status.Message = "Generating report..."
	case models.ReportStatusCompleted:
		status.Progress = 100
		status.Message = "Report completed successfully"
	case models.ReportStatusFailed:
		status.Progress = 0
		status.Message = "Report generation failed"
	}

	if report.StartedAt != nil {
		startedAt := report.StartedAt.Format(time.RFC3339)
		status.StartedAt = &startedAt
	}

	if report.CompletedAt != nil {
		completedAt := report.CompletedAt.Format(time.RFC3339)
		status.CompletedAt = &completedAt
	}

	return status, nil
}

// QueueReport creates a new report job and queues it for processing
func (js *JobService) QueueReport(userId int64, campaignIds []int64, format ReportFormat, options ReportOptions) (int64, error) {
	report := models.Report{
		UserId: userId,
		Format: string(format),
		Status: models.ReportStatusQueued,
	}

	// Set campaign IDs
	err := report.SetCampaignIdsSlice(campaignIds)
	if err != nil {
		return 0, fmt.Errorf("error setting campaign IDs: %v", err)
	}

	// Set options
	optionsMap := map[string]interface{}{
		"anonymize_emails":       options.AnonymizeEmails,
		"anonymize_ips":          options.AnonymizeIPs,
		"include_gdpr_statement": options.IncludeGDPRStatement,
		"include_toc":            options.IncludeTOC,
	}
	err = report.SetOptions(optionsMap)
	if err != nil {
		return 0, fmt.Errorf("error setting options: %v", err)
	}

	// Create the report
	err = models.PostReport(&report)
	if err != nil {
		return 0, fmt.Errorf("error creating report: %v", err)
	}

	log.Infof("Queued report %d for user %d with %d campaigns", report.Id, userId, len(campaignIds))
	return report.Id, nil
}

// QueueCampaignSetReport creates a new campaign set report job and queues it for processing
func (js *JobService) QueueCampaignSetReport(userId, campaignSetId int64, format ReportFormat, options ReportOptions) (int64, error) {
	report := models.Report{
		UserId:        userId,
		CampaignSetId: &campaignSetId,
		Format:        string(format),
		Status:        models.ReportStatusQueued,
		CampaignIds:   "[]", // Empty array for campaign set reports
	}

	// Set options
	optionsMap := map[string]interface{}{
		"anonymize_emails":       options.AnonymizeEmails,
		"anonymize_ips":          options.AnonymizeIPs,
		"include_gdpr_statement": options.IncludeGDPRStatement,
		"include_toc":            options.IncludeTOC,
	}
	err := report.SetOptions(optionsMap)
	if err != nil {
		return 0, fmt.Errorf("error setting options: %v", err)
	}

	// Create the report
	err = models.PostReport(&report)
	if err != nil {
		return 0, fmt.Errorf("error creating report: %v", err)
	}

	log.Infof("Queued campaign set report %d for user %d, campaign set %d", report.Id, userId, campaignSetId)
	return report.Id, nil
}

// CleanupExpiredReports removes expired reports using the file manager
func (js *JobService) CleanupExpiredReports() (int, error) {
	count, err := models.CleanupExpiredReports()
	if err != nil {
		return 0, err
	}

	// The file cleanup will be handled by the FileManager through periodic cleanup
	return count, nil
}

// Helper functions

func getBoolFromOptions(options map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := options[key]; exists {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}

func getFileExtension(format ReportFormat) string {
	switch format {
	case FormatExcel:
		return "xlsx"
	case FormatWord:
		return "docx"
	default:
		return "docx"
	}
}
