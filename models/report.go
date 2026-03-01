package models

import (
	"encoding/json"
	"time"

	log "github.com/gophish/gophish/logger"
)

// Report represents a report generation job
type Report struct {
	Id            int64      `json:"id" gorm:"column:id;primary_key;AUTO_INCREMENT"`
	UserId        int64      `json:"user_id" gorm:"column:user_id"`
	CampaignIds   string     `json:"campaign_ids" gorm:"column:campaign_ids"` // JSON array
	CampaignSetId *int64     `json:"campaign_set_id,omitempty" gorm:"column:campaign_set_id"`
	Format        string     `json:"format" gorm:"column:format"`
	Status        string     `json:"status" gorm:"column:status;default:'queued'"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	FilePath      string     `json:"file_path,omitempty" gorm:"column:file_path"`
	FileName      string     `json:"file_name,omitempty" gorm:"column:file_name"`
	FileSize      int64      `json:"file_size,omitempty" gorm:"column:file_size"`
	OptionsJson   string     `json:"options_json,omitempty" gorm:"column:options_json"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"column:error_message"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" gorm:"column:expires_at"`
}

// TableName returns the table name for GORM
func (Report) TableName() string {
	return "reports"
}

// ReportStatus constants
const (
	ReportStatusQueued     = "queued"
	ReportStatusProcessing = "processing"
	ReportStatusCompleted  = "completed"
	ReportStatusFailed     = "failed"
)

// GetCampaignIdsSlice returns the campaign IDs as a slice of int64
func (r *Report) GetCampaignIdsSlice() ([]int64, error) {
	var campaignIds []int64
	if r.CampaignIds == "" {
		return campaignIds, nil
	}
	err := json.Unmarshal([]byte(r.CampaignIds), &campaignIds)
	return campaignIds, err
}

// SetCampaignIdsSlice sets the campaign IDs from a slice of int64
func (r *Report) SetCampaignIdsSlice(campaignIds []int64) error {
	data, err := json.Marshal(campaignIds)
	if err != nil {
		return err
	}
	r.CampaignIds = string(data)
	return nil
}

// GetOptions returns the parsed options
func (r *Report) GetOptions() (map[string]interface{}, error) {
	var options map[string]interface{}
	if r.OptionsJson == "" {
		return options, nil
	}
	err := json.Unmarshal([]byte(r.OptionsJson), &options)
	return options, err
}

// SetOptions sets the options as JSON
func (r *Report) SetOptions(options map[string]interface{}) error {
	data, err := json.Marshal(options)
	if err != nil {
		return err
	}
	r.OptionsJson = string(data)
	return nil
}

// IsExpired checks if the report has expired
func (r *Report) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*r.ExpiresAt)
}

// GetReports returns all reports for a user
func GetReports(uid int64) ([]Report, error) {
	reports := []Report{}
	err := db.Where("user_id = ?", uid).Order("created_at DESC").Find(&reports).Error
	if err != nil {
		log.Error(err)
		return reports, err
	}
	return reports, nil
}

// GetReport returns a report by ID for a specific user
func GetReport(id int64, uid int64) (Report, error) {
	r := Report{}
	err := db.Where("id = ? AND user_id = ?", id, uid).First(&r).Error
	if err != nil {
		log.Error(err)
	}
	return r, err
}

// PostReport creates a new report in the database
func PostReport(r *Report) error {
	// Set defaults
	if r.Status == "" {
		r.Status = ReportStatusQueued
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}

	// Set expiration (30 days from now by default)
	if r.ExpiresAt == nil {
		expiresAt := time.Now().UTC().AddDate(0, 0, 30)
		r.ExpiresAt = &expiresAt
	}

	// Insert the report
	err := db.Create(r).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PutReport updates an existing report
func PutReport(r *Report) error {
	err := db.Save(r).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// DeleteReport deletes a report by ID for a specific user
func DeleteReport(id int64, uid int64) error {
	err := db.Where("id = ? AND user_id = ?", id, uid).Delete(&Report{}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// GetQueuedReports returns all reports with status 'queued'
func GetQueuedReports() ([]Report, error) {
	reports := []Report{}
	err := db.Where("status = ?", ReportStatusQueued).Order("created_at ASC").Find(&reports).Error
	if err != nil {
		log.Error(err)
		return reports, err
	}
	return reports, nil
}

// GetProcessingReports returns all reports with status 'processing'
func GetProcessingReports() ([]Report, error) {
	reports := []Report{}
	err := db.Where("status = ?", ReportStatusProcessing).Order("created_at ASC").Find(&reports).Error
	if err != nil {
		log.Error(err)
		return reports, err
	}
	return reports, nil
}

// UpdateReportStatus updates only the status of a report
func UpdateReportStatus(id int64, status string) error {
	now := time.Now().UTC()
	var err error

	switch status {
	case ReportStatusProcessing:
		err = db.Model(&Report{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     status,
			"started_at": now,
		}).Error
	case ReportStatusCompleted:
		err = db.Model(&Report{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       status,
			"completed_at": now,
		}).Error
	case ReportStatusFailed:
		err = db.Model(&Report{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       status,
			"completed_at": now,
		}).Error
	default:
		err = db.Model(&Report{}).Where("id = ?", id).Update("status", status).Error
	}

	if err != nil {
		log.Error(err)
	}
	return err
}

// UpdateReportFile updates the file information for a report
func UpdateReportFile(id int64, filePath, fileName string, fileSize int64) error {
	err := db.Model(&Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"file_path": filePath,
		"file_name": fileName,
		"file_size": fileSize,
	}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// UpdateReportError updates the error message for a report
func UpdateReportError(id int64, errorMessage string) error {
	err := db.Model(&Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        ReportStatusFailed,
		"error_message": errorMessage,
		"completed_at":  time.Now().UTC(),
	}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// CleanupExpiredReports removes expired reports and their files
func CleanupExpiredReports() (int, error) {
	expired := []Report{}
	err := db.Where("expires_at < ?", time.Now().UTC()).Find(&expired).Error
	if err != nil {
		log.Error(err)
		return 0, err
	}

	count := 0
	for _, report := range expired {
		// Delete the file if it exists
		if report.FilePath != "" {
			// File deletion will be handled by the FileManager
			log.Infof("Cleaning up expired report %d, file: %s", report.Id, report.FilePath)
		}

		// Delete the database record
		err = DeleteReport(report.Id, report.UserId)
		if err != nil {
			log.Errorf("Error deleting expired report %d: %v", report.Id, err)
			continue
		}
		count++
	}

	if count > 0 {
		log.Infof("Cleaned up %d expired reports", count)
	}

	return count, nil
}

// GetReportStats returns statistics about reports for a user
func GetReportStats(uid int64) (map[string]int, error) {
	stats := map[string]int{
		"total":      0,
		"queued":     0,
		"processing": 0,
		"completed":  0,
		"failed":     0,
	}

	// Get total count
	var totalCount int64
	err := db.Model(&Report{}).Where("user_id = ?", uid).Count(&totalCount).Error
	if err != nil {
		log.Error(err)
		return stats, err
	}
	stats["total"] = int(totalCount)

	// Get count by status
	type StatusCount struct {
		Status string
		Count  int
	}

	var statusCounts []StatusCount
	err = db.Model(&Report{}).Select("status, COUNT(*) as count").
		Where("user_id = ?", uid).
		Group("status").
		Scan(&statusCounts).Error
	if err != nil {
		log.Error(err)
		return stats, err
	}

	for _, sc := range statusCounts {
		stats[sc.Status] = sc.Count
	}

	return stats, nil
}
