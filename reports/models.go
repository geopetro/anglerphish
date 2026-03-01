package reports

// ReportFormat defines the supported report formats
type ReportFormat string

const (
	// FormatWord represents a Word document format
	FormatWord ReportFormat = "word"

	// FormatExcel represents an Excel spreadsheet format
	FormatExcel ReportFormat = "excel"
)

// ReportOptions contains configurable options for report generation
type ReportOptions struct {
	// GDPR compliance options
	AnonymizeEmails      bool `json:"anonymize_emails"`
	AnonymizeIPs         bool `json:"anonymize_ips"`
	IncludeGDPRStatement bool `json:"include_gdpr_statement"`

	// Word-specific options
	IncludeTOC bool `json:"include_toc,omitempty"`

	// Excel format is still supported but without additional options
}

// ReportRequest represents a request to generate a report
type ReportRequest struct {
	CampaignIDs []int64       `json:"campaign_ids"`
	Format      ReportFormat  `json:"format"`
	Options     ReportOptions `json:"options"`
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

// CampaignSetReportRequest represents a request to generate a report for a campaign set
type CampaignSetReportRequest struct {
	CampaignSetID int64         `json:"campaign_set_id"`
	Format        ReportFormat  `json:"format"`
	Options       ReportOptions `json:"options"`
}

// AsyncCampaignSetReportRequest represents a request to queue a campaign set report
type AsyncCampaignSetReportRequest struct {
	CampaignSetID int64         `json:"campaign_set_id"`
	Format        ReportFormat  `json:"format"`
	Options       ReportOptions `json:"options"`
}

// ReportListResponse contains a list of reports for a user
type ReportListResponse struct {
	Reports []ReportSummary `json:"reports"`
	Stats   ReportStats     `json:"stats"`
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

// ReportStats contains statistics about a user's reports
type ReportStats struct {
	Total      int `json:"total"`
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
}

// ReportStatusResponse contains the current status of a report job
type ReportStatusResponse struct {
	ID          int64   `json:"id"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"` // 0-100
	Message     string  `json:"message"`
	CreatedAt   string  `json:"created_at"`
	StartedAt   *string `json:"started_at,omitempty"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
	FileName    string  `json:"file_name,omitempty"`
	FileSize    int64   `json:"file_size,omitempty"`
}
