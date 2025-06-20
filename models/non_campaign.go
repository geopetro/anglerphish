package models

import (
	"time"

	log "github.com/gophish/gophish/logger"
)

// NonCampaignReport represents a reported email that was not part of a GoPhish campaign
type NonCampaignReport struct {
	Id            int64     `json:"id" gorm:"primary_key:yes"`
	UserId        int64     `json:"user_id"`
	ImapId        int64     `json:"imap_id"`
	ReporterEmail string    `json:"reporter_email"`
	Subject       string    `json:"subject"`
	ReportedAt    time.Time `json:"reported_at"`
}

// NonCampaignStats represents aggregated statistics about non-campaign reports
type NonCampaignStats struct {
	UserId         int64     `json:"user_id" gorm:"primary_key:yes"`
	ReportCount    int       `json:"report_count"`
	LastReportedAt time.Time `json:"last_reported_at"`
}

// TableName specifies the database tablename for Gorm to use
func (n NonCampaignReport) TableName() string {
	return "non_campaign_reports"
}

// TableName specifies the database tablename for Gorm to use
func (n NonCampaignStats) TableName() string {
	return "non_campaign_stats"
}

// RecordNonCampaignReport creates a new record for a non-campaign email report
// and updates the stats table
func RecordNonCampaignReport(userId int64, imapId int64, reporterEmail string, subject string) error {
	// Begin transaction
	tx := db.Begin()

	// Create report record
	report := &NonCampaignReport{
		UserId:        userId,
		ImapId:        imapId,
		ReporterEmail: reporterEmail,
		Subject:       subject,
		ReportedAt:    time.Now().UTC(),
	}

	if err := tx.Create(report).Error; err != nil {
		tx.Rollback()
		log.Errorf("Failed to create non-campaign report: %v", err)
		return err
	}

	// Update stats table - try to find existing stats first
	var stats NonCampaignStats
	err := tx.Where("user_id = ?", userId).First(&stats).Error

	if err != nil {
		// No existing stats, create a new record
		stats = NonCampaignStats{
			UserId:         userId,
			ReportCount:    1,
			LastReportedAt: time.Now().UTC(),
		}
		if err := tx.Create(&stats).Error; err != nil {
			tx.Rollback()
			log.Errorf("Failed to create non-campaign stats: %v", err)
			return err
		}
	} else {
		// Update existing stats
		stats.ReportCount++
		stats.LastReportedAt = time.Now().UTC()
		if err := tx.Save(&stats).Error; err != nil {
			tx.Rollback()
			log.Errorf("Failed to update non-campaign stats: %v", err)
			return err
		}
	}

	return tx.Commit().Error
}

// GetNonCampaignStats retrieves the non-campaign report statistics for a user
func GetNonCampaignStats(userId int64) (NonCampaignStats, error) {
	stats := NonCampaignStats{}
	err := db.Where("user_id = ?", userId).First(&stats).Error

	// Check if the error is "record not found"
	if err != nil && err.Error() == "record not found" {
		// Return a default empty stats object instead of an error
		return NonCampaignStats{
			UserId:         userId,
			ReportCount:    0,
			LastReportedAt: time.Time{}, // Zero time
		}, nil
	}

	return stats, err
}

// GetRecentNonCampaignReports retrieves the most recent non-campaign reports for a user
// If imapId is > 0, only returns reports from that specific IMAP configuration
func GetRecentNonCampaignReports(userId int64, limit int, imapId int64) ([]NonCampaignReport, error) {
	reports := []NonCampaignReport{}
	query := db.Where("user_id = ?", userId)

	// Filter by IMAP ID if specified
	if imapId > 0 {
		query = query.Where("imap_id = ?", imapId)
	}

	err := query.Order("reported_at DESC").
		Limit(limit).
		Find(&reports).Error
	return reports, err
}

// ClearNonCampaignReports deletes all non-campaign reports for a user
// If imapId is > 0, only deletes reports from that specific IMAP configuration
func ClearNonCampaignReports(userId int64, imapId int64) error {
	// Begin transaction
	tx := db.Begin()

	// Build the query
	query := tx.Where("user_id = ?", userId)

	// Filter by IMAP ID if specified
	if imapId > 0 {
		query = query.Where("imap_id = ?", imapId)
	}

	// Delete reports
	if err := query.Delete(&NonCampaignReport{}).Error; err != nil {
		tx.Rollback()
		log.Errorf("Failed to delete non-campaign reports: %v", err)
		return err
	}

	// Update stats if needed
	if imapId == 0 {
		// If clearing all reports, reset the stats
		if err := tx.Model(&NonCampaignStats{}).Where("user_id = ?", userId).
			Updates(map[string]interface{}{
				"report_count": 0,
			}).Error; err != nil {
			tx.Rollback()
			log.Errorf("Failed to update non-campaign stats: %v", err)
			return err
		}
	} else {
		// If clearing only specific IMAP reports, recalculate the count
		var count int64
		if err := tx.Model(&NonCampaignReport{}).Where("user_id = ?", userId).Count(&count).Error; err != nil {
			tx.Rollback()
			log.Errorf("Failed to count remaining non-campaign reports: %v", err)
			return err
		}

		// Update the stats with the new count
		if err := tx.Model(&NonCampaignStats{}).Where("user_id = ?", userId).
			Updates(map[string]interface{}{
				"report_count": count,
			}).Error; err != nil {
			tx.Rollback()
			log.Errorf("Failed to update non-campaign stats: %v", err)
			return err
		}
	}

	return tx.Commit().Error
}
