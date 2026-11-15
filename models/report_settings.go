package models

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Report generation setting defaults and limits.
const (
	DefaultReportJobTimeoutMinutes = 30
	DefaultReportRetentionDays     = 30
	MaxReportJobTimeoutMinutes     = 24 * 60
	MaxReportRetentionDays         = 3650
)

// ErrReportSettingsInvalid is returned when submitted report settings are not valid.
var ErrReportSettingsInvalid = errors.New("invalid report settings")

var reportSettingsMu sync.Mutex

// ReportSettings is the single row of report generation settings. They are
// read each time a report is generated, so changes apply without a restart.
type ReportSettings struct {
	Id int64 `json:"-" gorm:"column:id; primary_key:yes"`
	// OnlineGeolocation sends target IP addresses to freeipapi.com to look
	// up their location. The local GeoLite2 database is used either way.
	OnlineGeolocation bool `json:"online_geolocation" gorm:"column:online_geolocation"`
	// GeoIPDatabasePath points to a GeoLite2-City.mmdb file on the server.
	GeoIPDatabasePath string `json:"geoip_db_path" gorm:"column:geoip_db_path"`
	// JobTimeoutMinutes limits how long one report may take to generate.
	JobTimeoutMinutes int `json:"job_timeout_minutes" gorm:"column:job_timeout_minutes"`
	// RetentionDays is how long generated reports are kept. It applies to
	// reports queued after the change.
	RetentionDays int       `json:"retention_days" gorm:"column:retention_days"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the database tablename for Gorm to use.
func (ReportSettings) TableName() string {
	return "report_settings"
}

func defaultReportSettings() ReportSettings {
	return ReportSettings{
		JobTimeoutMinutes: DefaultReportJobTimeoutMinutes,
		RetentionDays:     DefaultReportRetentionDays,
	}
}

// GetReportSettings returns the stored report settings, or the defaults
// when none have been saved.
func GetReportSettings() (ReportSettings, error) {
	var rows []ReportSettings
	if err := db.Order("id asc").Limit(1).Find(&rows).Error; err != nil {
		return defaultReportSettings(), err
	}
	if len(rows) == 0 {
		return defaultReportSettings(), nil
	}
	return rows[0], nil
}

func invalidReportSettings(msg string) error {
	return fmt.Errorf("%w: %s", ErrReportSettingsInvalid, msg)
}

// SaveReportSettings validates and stores the report settings.
func SaveReportSettings(in ReportSettings) error {
	in.GeoIPDatabasePath = strings.TrimSpace(in.GeoIPDatabasePath)
	if in.JobTimeoutMinutes < 1 || in.JobTimeoutMinutes > MaxReportJobTimeoutMinutes {
		return invalidReportSettings(fmt.Sprintf("the job timeout must be between 1 and %d minutes", MaxReportJobTimeoutMinutes))
	}
	if in.RetentionDays < 1 || in.RetentionDays > MaxReportRetentionDays {
		return invalidReportSettings(fmt.Sprintf("reports must be kept between 1 and %d days", MaxReportRetentionDays))
	}
	if in.GeoIPDatabasePath != "" {
		info, err := os.Stat(in.GeoIPDatabasePath)
		if err != nil || info.IsDir() {
			return invalidReportSettings("no file was found at the GeoLite2 database path")
		}
	}

	reportSettingsMu.Lock()
	defer reportSettingsMu.Unlock()
	var rows []ReportSettings
	if err := db.Order("id asc").Limit(1).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 0 {
		in.Id = rows[0].Id
	} else {
		in.Id = 0
	}
	in.UpdatedAt = time.Now().UTC()
	return db.Save(&in).Error
}
