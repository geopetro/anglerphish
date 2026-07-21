package api

import (
	"net/http"

	mid "github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/worker"
	"github.com/gorilla/mux"
)

// ServerOption is an option to apply to the API server.
type ServerOption func(*Server)

// Server represents the routes and functionality of the Gophish API.
// It's not a server in the traditional sense, in that it isn't started and
// stopped. Rather, it's meant to be used as an http.Handler in the
// AdminServer.
type Server struct {
	handler http.Handler
	worker  worker.Worker
	limiter *ratelimit.PostLimiter
}

// NewServer returns a new instance of the API handler with the provided
// options applied.
func NewServer(options ...ServerOption) *Server {
	defaultWorker, _ := worker.New()
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &Server{
		worker:  defaultWorker,
		limiter: defaultLimiter,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) ServerOption {
	return func(as *Server) {
		as.worker = w
	}
}

func WithLimiter(limiter *ratelimit.PostLimiter) ServerOption {
	return func(as *Server) {
		as.limiter = limiter
	}
}

func (as *Server) registerRoutes() {
	root := mux.NewRouter()
	root = root.StrictSlash(true)
	router := root.PathPrefix("/api/").Subrouter()
	router.Use(mid.RequireAPIKey)
	router.Use(mid.EnforceViewOnly)
	router.HandleFunc("/imap/", as.IMAPServer)
	router.HandleFunc("/imap/{id:[0-9]+}", as.IMAPServerById)
	router.HandleFunc("/imap/validate", as.IMAPServerValidate)
	router.HandleFunc("/imap/non_campaign_reports", as.NonCampaignReportsEndpoint)
	router.HandleFunc("/imap/non_campaign_reports/{id:[0-9]+}/message", as.NonCampaignReportMessage)
	router.HandleFunc("/imap/replies", as.IMAPReplies)
	router.HandleFunc("/reset", as.Reset)
	router.HandleFunc("/campaigns/", as.Campaigns)
	router.HandleFunc("/campaigns/summary", as.CampaignsSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}", as.Campaign)
	router.HandleFunc("/campaigns/{id:[0-9]+}/results", as.CampaignResults)
	router.HandleFunc("/campaigns/{id:[0-9]+}/summary", as.CampaignSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}/complete", as.CampaignComplete)
	router.HandleFunc("/campaigns/{id:[0-9]+}/links", as.CampaignLinks)
	router.HandleFunc("/campaigns/{id:[0-9]+}/resend", as.CampaignResendFailed)
	router.HandleFunc("/campaigns/{id:[0-9]+}/results/{rid}/resend", as.CampaignResultResend)
	router.HandleFunc("/campaign_sets/", as.CampaignSets)
	router.HandleFunc("/campaign_sets/summary", as.CampaignSetsSummary)
	router.HandleFunc("/campaign_sets/{id:[0-9]+}", as.CampaignSet)
	router.HandleFunc("/campaign_sets/{id:[0-9]+}/complete", as.CampaignSetComplete)
	router.HandleFunc("/draft_campaign_sets/", as.DraftCampaignSets)
	router.HandleFunc("/draft_campaign_sets/{id:[0-9]+}", as.DraftCampaignSet)
	router.HandleFunc("/draft_campaign_sets/{id:[0-9]+}/launch", as.LaunchDraftCampaignSet)
	router.HandleFunc("/groups/", as.Groups)
	router.HandleFunc("/groups/summary", as.GroupsSummary)
	router.HandleFunc("/groups/{id:[0-9]+}", as.Group)
	router.HandleFunc("/groups/{id:[0-9]+}/summary", as.GroupSummary)
	router.HandleFunc("/groups/{id:[0-9]+}/lock", as.GroupLock)
	router.HandleFunc("/templates/", as.Templates)
	router.HandleFunc("/templates/{id:[0-9]+}", as.Template)
	router.HandleFunc("/sms_templates/", as.SMSTemplates)
	router.HandleFunc("/sms_templates/{id:[0-9]+}", as.SMSTemplate)
	router.HandleFunc("/pages/", as.Pages)
	router.HandleFunc("/pages/{id:[0-9]+}", as.Page)
	router.HandleFunc("/pages/mfa_template", as.MFADefaultTemplate)
	router.HandleFunc("/smtp/", as.SendingProfiles)
	router.HandleFunc("/smtp/{id:[0-9]+}", as.SendingProfile)
	router.HandleFunc("/sms/", as.SMSProfiles)
	router.HandleFunc("/sms/{id:[0-9]+}", as.SMSProfile)
	router.HandleFunc("/sms/{id:[0-9]+}/balance", as.SMSProfileBalance)
	router.HandleFunc("/users/", mid.Use(as.Users, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/users/{id:[0-9]+}", mid.Use(as.User))
	router.HandleFunc("/util/send_test_email", as.SendTestEmail)
	router.HandleFunc("/util/send_test_sms", as.SendTestSMS)
	router.HandleFunc("/import/group", as.ImportGroup)
	router.HandleFunc("/import/email", as.ImportEmail)
	router.HandleFunc("/import/site", as.ImportSite)
	router.HandleFunc("/url_templates/", as.URLTemplates)
	router.HandleFunc("/url_templates/{id:[0-9]+}", as.URLTemplate)
	router.HandleFunc("/webhooks/", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}/validate", mid.Use(as.ValidateWebhook, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}", mid.Use(as.Webhook, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/error_pages/404", mid.Use(as.ErrorPage, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/error_pages/404/reset", mid.Use(as.ErrorPageReset, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/global_variables", mid.Use(as.GlobalVariables))
	router.HandleFunc("/qr_code/", as.Qr_code)                            // QR code endpoint
	router.HandleFunc("/qr_code/{id:[0-9]+}", as.DeleteQRCode)            // Delete QR code endpoint
	router.HandleFunc("/qr_code/{id:[0-9]+}/download", as.DownloadQRCode) // Download QR code endpoint

	// Async report management endpoints
	router.HandleFunc("/reports/", as.ListReports)                        // List reports (GET)
	router.HandleFunc("/reports/queue", as.QueueReport)                   // Queue report for async generation (POST)
	router.HandleFunc("/reports/{id:[0-9]+}/status", as.GetReportStatus)  // Get report status (GET)
	router.HandleFunc("/reports/{id:[0-9]+}/download", as.DownloadReport) // Download report (GET)
	router.HandleFunc("/reports/{id:[0-9]+}", as.DeleteReport)            // Delete report (DELETE)

	// Legacy and utility report endpoints
	router.HandleFunc("/reports/campaign_set", as.GenerateCampaignSetReport)   // Campaign Set Reports endpoint (sync)
	router.HandleFunc("/reports/dependencies", as.CheckDependencies)           // Check Python dependencies endpoint
	router.HandleFunc("/reports/dependencies/install", as.InstallDependencies) // Install Python dependencies endpoint
	as.handler = router
}

func (as *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	as.handler.ServeHTTP(w, r)
}
