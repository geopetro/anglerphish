package reports

import (
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// ReportFormat defines the supported report formats
type ReportFormat string

const (
	// FormatWord represents a Word document format
	FormatWord ReportFormat = "word"

	// FormatExcel represents an Excel spreadsheet format
	FormatExcel ReportFormat = "excel"
)

// Valid reports whether the format is supported.
func (f ReportFormat) Valid() bool {
	return f == FormatWord || f == FormatExcel
}

// Extension returns the file extension (without dot) for the format.
func (f ReportFormat) Extension() string {
	if f == FormatExcel {
		return "xlsx"
	}
	return "docx"
}

// ContentType returns the MIME type for the format.
func (f ReportFormat) ContentType() string {
	if f == FormatExcel {
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

// ReportOptions contains configurable options for report generation
type ReportOptions struct {
	// AnonymizeEmails masks emails, phone numbers, names and submitted values
	AnonymizeEmails bool `json:"anonymize_emails"`
	// AnonymizeIPs masks IP addresses and coordinates and reduces locations
	// to country level
	AnonymizeIPs bool `json:"anonymize_ips"`
}

// Settings configures report generation. They are edited in
// Settings > Report Generation and read again for every report.
type Settings struct {
	OnlineGeolocation bool
	GeoIPDatabasePath string
	JobTimeout        time.Duration
	Retention         time.Duration
}

// Default values for Settings
const (
	DefaultJobTimeout = models.DefaultReportJobTimeoutMinutes * time.Minute
	DefaultRetention  = models.DefaultReportRetentionDays * 24 * time.Hour
)

// LoadSettings returns the current report settings from the database,
// falling back to the defaults if they can't be read.
func LoadSettings() Settings {
	rs, err := models.GetReportSettings()
	if err != nil {
		log.Errorf("Error loading report settings, using defaults: %v", err)
	}
	return Settings{
		OnlineGeolocation: rs.OnlineGeolocation,
		GeoIPDatabasePath: rs.GeoIPDatabasePath,
		JobTimeout:        time.Duration(rs.JobTimeoutMinutes) * time.Minute,
		Retention:         time.Duration(rs.RetentionDays) * 24 * time.Hour,
	}.withDefaults()
}

func (s Settings) withDefaults() Settings {
	if s.JobTimeout <= 0 {
		s.JobTimeout = DefaultJobTimeout
	}
	if s.Retention <= 0 {
		s.Retention = DefaultRetention
	}
	return s
}

// AsyncReportRequest represents a request to queue a report for async generation
type AsyncReportRequest struct {
	CampaignIDs   []int64       `json:"campaign_ids"`
	CampaignSetID *int64        `json:"campaign_set_id,omitempty"`
	Format        ReportFormat  `json:"format"`
	Options       ReportOptions `json:"options"`
}

// AsyncReportResponse represents the response when queueing a report
type AsyncReportResponse struct {
	JobID   int64  `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ReportData contains all the campaign data needed for report generation
type ReportData struct {
	// Campaign information
	Campaigns []map[string]interface{} `json:"campaigns"`

	// Campaign set information (if this is a set report)
	SetID            int64       `json:"set_id,omitempty"`
	SetName          string      `json:"set_name,omitempty"`
	SetCreatedDate   interface{} `json:"set_created_date,omitempty"`
	SetLaunchDate    interface{} `json:"set_launch_date,omitempty"`
	SetCompletedDate interface{} `json:"set_completed_date,omitempty"`
	SetStatus        string      `json:"set_status,omitempty"`
	SetURL           string      `json:"set_url,omitempty"`
}
