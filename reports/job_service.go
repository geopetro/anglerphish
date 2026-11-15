package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/jinzhu/gorm"
)

const (
	pollInterval    = 5 * time.Second
	cleanupInterval = time.Hour
	maxErrorLength  = 4000
)

// JobService handles background processing of report generation jobs
type JobService struct {
	python *PythonService
	files  *FileManager
	// settings returns the current settings; it is called for every job so
	// changes made in Settings > Report Generation apply without a restart.
	settings func() Settings

	mutex   sync.Mutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wake    chan struct{}
	wg      sync.WaitGroup
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

// NewJobService creates a job service that stores files through files.
func NewJobService(files *FileManager) *JobService {
	return &JobService{
		python:   NewPythonService(),
		files:    files,
		settings: LoadSettings,
		wake:     make(chan struct{}, 1),
	}
}

// Python returns the Python service used by this job service.
func (js *JobService) Python() *PythonService {
	return js.python
}

// Start begins the job processing loop. Reports left "processing" by a
// previous run are queued again first.
func (js *JobService) Start() {
	js.mutex.Lock()
	defer js.mutex.Unlock()
	if js.running {
		return
	}
	if n, err := models.RequeueProcessingReports(); err != nil {
		log.Errorf("Error re-queueing interrupted reports: %v", err)
	} else if n > 0 {
		log.Infof("Re-queued %d report(s) interrupted by a previous shutdown", n)
	}

	js.ctx, js.cancel = context.WithCancel(context.Background())
	js.running = true
	log.Info("Starting report job service")
	js.wg.Add(1)
	go js.processJobsLoop(js.ctx)
}

// Stop cancels any running report (it is queued again for the next start)
// and waits for the processing loop to exit.
func (js *JobService) Stop() {
	js.mutex.Lock()
	if !js.running {
		js.mutex.Unlock()
		return
	}
	log.Info("Stopping report job service")
	js.running = false
	js.cancel()
	js.mutex.Unlock()

	js.wg.Wait()
	log.Info("Report job service stopped")
}

// IsRunning returns whether the service is currently running
func (js *JobService) IsRunning() bool {
	js.mutex.Lock()
	defer js.mutex.Unlock()
	return js.running
}

func (js *JobService) notify() {
	select {
	case js.wake <- struct{}{}:
	default:
	}
}

func (js *JobService) processJobsLoop(ctx context.Context) {
	defer js.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	cleanup := time.NewTicker(cleanupInterval)
	defer cleanup.Stop()

	js.CleanupExpiredReports()
	js.processQueuedJobs(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			js.processQueuedJobs(ctx)
		case <-js.wake:
			js.processQueuedJobs(ctx)
		case <-cleanup.C:
			js.CleanupExpiredReports()
		}
	}
}

func (js *JobService) processQueuedJobs(ctx context.Context) {
	queued, err := models.GetQueuedReports()
	if err != nil {
		log.Errorf("Error getting queued reports: %v", err)
		return
	}
	for _, report := range queued {
		if ctx.Err() != nil {
			return
		}
		js.processReport(ctx, report)
	}
}

func (js *JobService) processReport(ctx context.Context, report models.Report) {
	claimed, err := models.ClaimReport(report.Id)
	if err != nil || !claimed {
		return
	}
	log.Infof("Processing report %d for user %d", report.Id, report.UserId)

	settings := js.settings()
	jobCtx, cancel := context.WithTimeout(ctx, settings.JobTimeout)
	defer cancel()

	err = js.generateReport(jobCtx, report, settings)
	switch {
	case err == nil:
		log.Infof("Successfully completed report %d", report.Id)
	case ctx.Err() != nil:
		// The service is stopping: run this report again on the next start
		log.Infof("Report %d interrupted by shutdown; it will run again on restart", report.Id)
		models.RequeueReport(report.Id)
	case jobCtx.Err() == context.DeadlineExceeded:
		models.UpdateReportError(report.Id, fmt.Sprintf("Generation timed out after %s", settings.JobTimeout))
	default:
		log.Errorf("Error generating report %d: %v", report.Id, err)
		models.UpdateReportError(report.Id, truncate(fmt.Sprintf("Generation failed: %v", err), maxErrorLength))
	}
}

func (js *JobService) generateReport(ctx context.Context, report models.Report, settings Settings) error {
	var options ReportOptions
	if report.OptionsJson != "" {
		if err := json.Unmarshal([]byte(report.OptionsJson), &options); err != nil {
			return fmt.Errorf("error parsing report options: %v", err)
		}
	}
	format := ReportFormat(report.Format)
	if !format.Valid() {
		return fmt.Errorf("unsupported report format %q", report.Format)
	}

	var data *ReportData
	var err error
	if report.CampaignSetId != nil {
		data, err = FetchCampaignSetData(*report.CampaignSetId, report.UserId)
	} else {
		campaignIDs, perr := report.GetCampaignIdsSlice()
		if perr != nil {
			return fmt.Errorf("error parsing campaign IDs: %v", perr)
		}
		validation, verr := ValidateCampaignData(campaignIDs, report.UserId)
		if verr != nil {
			return fmt.Errorf("error validating campaigns: %v", verr)
		}
		if !validation.Valid {
			msg := "Campaign validation failed"
			if len(validation.Errors) > 0 {
				msg = fmt.Sprintf("%s: %s", msg, validation.Errors[0])
			}
			return errors.New(msg)
		}
		for _, warning := range validation.Warnings {
			log.Warnf("Report %d: %s", report.Id, warning)
		}
		data, err = FetchCampaignData(campaignIDs, report.UserId)
	}
	if err != nil {
		return err
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error encoding report data: %v", err)
	}
	reportData, err := js.python.GenerateReport(ctx, format, payload, options, settings)
	if err != nil {
		return err
	}

	// Stored under a unique server-side name; downloaded under a readable one
	diskName := fmt.Sprintf("report_%d_%d.%s", report.Id, time.Now().Unix(), format.Extension())
	filePath, err := js.files.SaveReport(diskName, reportData)
	if err != nil {
		return fmt.Errorf("error saving report file: %v", err)
	}
	displayName := DownloadFileName(data.reportTitle(), time.Now(), format)
	err = models.UpdateReportFile(report.Id, filePath, displayName, int64(len(reportData)))
	if err != nil {
		js.files.DeleteReportFile(filePath)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Infof("Report %d was deleted during generation; discarded its file", report.Id)
			return nil
		}
		return fmt.Errorf("error updating report file info: %v", err)
	}
	return models.UpdateReportStatus(report.Id, models.ReportStatusCompleted)
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// DownloadFileName builds a safe, readable file name such as
// "Anglerphish_Q3_Finance_Campaign_2026-11-15.docx".
func DownloadFileName(title string, when time.Time, format ReportFormat) string {
	name := strings.Trim(unsafeFileChars.ReplaceAllString(title, "_"), "_.")
	if len(name) > 60 {
		name = strings.TrimRight(name[:60], "_.")
	}
	if name == "" {
		name = "Report"
	}
	return fmt.Sprintf("Anglerphish_%s_%s.%s", name, when.Format("2006-01-02"), format.Extension())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
	switch report.Status {
	case models.ReportStatusQueued:
		status.Message = "Report queued for generation"
	case models.ReportStatusProcessing:
		status.Progress = 50
		status.Message = "Generating report..."
	case models.ReportStatusCompleted:
		status.Progress = 100
		status.Message = "Report completed successfully"
	case models.ReportStatusFailed:
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

func (js *JobService) newReport(userId int64, campaignIds []int64, format ReportFormat, options ReportOptions) (models.Report, error) {
	expiresAt := time.Now().UTC().Add(js.settings().Retention)
	report := models.Report{
		UserId:    userId,
		Format:    string(format),
		Status:    models.ReportStatusQueued,
		ExpiresAt: &expiresAt,
	}
	if err := report.SetCampaignIdsSlice(campaignIds); err != nil {
		return report, fmt.Errorf("error setting campaign IDs: %v", err)
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return report, fmt.Errorf("error setting options: %v", err)
	}
	report.OptionsJson = string(optionsJSON)
	return report, nil
}

// QueueReport creates a new report job for individual campaigns
func (js *JobService) QueueReport(userId int64, campaignIds []int64, format ReportFormat, options ReportOptions) (int64, error) {
	report, err := js.newReport(userId, campaignIds, format, options)
	if err != nil {
		return 0, err
	}
	if err := models.PostReport(&report); err != nil {
		return 0, fmt.Errorf("error creating report: %v", err)
	}
	log.Infof("Queued report %d for user %d with %d campaigns", report.Id, userId, len(campaignIds))
	js.notify()
	return report.Id, nil
}

// QueueCampaignSetReport creates a new report job for a campaign set.
// campaignIds records the set's campaigns at queue time (for display).
func (js *JobService) QueueCampaignSetReport(userId, campaignSetId int64, campaignIds []int64, format ReportFormat, options ReportOptions) (int64, error) {
	report, err := js.newReport(userId, campaignIds, format, options)
	if err != nil {
		return 0, err
	}
	report.CampaignSetId = &campaignSetId
	if err := models.PostReport(&report); err != nil {
		return 0, fmt.Errorf("error creating report: %v", err)
	}
	log.Infof("Queued campaign set report %d for user %d, campaign set %d", report.Id, userId, campaignSetId)
	js.notify()
	return report.Id, nil
}

// CleanupExpiredReports deletes expired reports and their files.
func (js *JobService) CleanupExpiredReports() int {
	expired, err := models.GetExpiredReports()
	if err != nil {
		log.Errorf("Error finding expired reports: %v", err)
		return 0
	}
	count := 0
	for _, report := range expired {
		if report.Status == models.ReportStatusProcessing {
			continue
		}
		if report.FilePath != "" {
			if err := js.files.DeleteReportFile(report.FilePath); err != nil {
				log.Errorf("Error deleting file of expired report %d: %v", report.Id, err)
				continue
			}
		}
		if err := models.DeleteReport(report.Id, report.UserId); err != nil {
			log.Errorf("Error deleting expired report %d: %v", report.Id, err)
			continue
		}
		count++
	}
	if count > 0 {
		log.Infof("Cleaned up %d expired report(s)", count)
	}
	return count
}
