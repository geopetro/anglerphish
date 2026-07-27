package controllers

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/controllers/api"
	log "github.com/gophish/gophish/logger"
	mid "github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/util"
	"github.com/gophish/gophish/worker"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jordan-wright/unindexed"
)

// AdminServerOption is a functional option that is used to configure the
// admin server
type AdminServerOption func(*AdminServer)

// AdminServer is an HTTP server that implements the administrative Gophish
// handlers, including the dashboard and REST API.
type AdminServer struct {
	server     *http.Server
	worker     worker.Worker
	config     config.AdminServer
	fullConfig *config.Config
	limiter    *ratelimit.PostLimiter
	oidc       *auth.OIDCClient
}

var defaultTLSConfig = &tls.Config{
	PreferServerCipherSuites: true,
	CurvePreferences: []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	},
	MinVersion: tls.VersionTLS12,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		// Kept for backwards compatibility with some clients
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	},
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) AdminServerOption {
	return func(as *AdminServer) {
		as.worker = w
	}
}

// NewAdminServer returns a new instance of the AdminServer with the
// provided config and options applied.
func NewAdminServer(adminConfig config.AdminServer, fullConfig *config.Config, options ...AdminServerOption) *AdminServer {
	defaultWorker, _ := worker.New()
	defaultServer := &http.Server{
		ReadTimeout: 10 * time.Second,
		Addr:        adminConfig.ListenURL,
	}
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &AdminServer{
		worker:     defaultWorker,
		server:     defaultServer,
		limiter:    defaultLimiter,
		config:     adminConfig,
		fullConfig: fullConfig,
	}
	for _, opt := range options {
		opt(as)
	}
	if as.fullConfig != nil && as.fullConfig.OIDC.Enabled {
		oidcClient, err := auth.NewOIDCClient(oidcConfigFrom(as.fullConfig.OIDC))
		if err != nil {
			log.Fatal(err)
		}
		as.oidc = oidcClient
	}
	as.registerRoutes()
	return as
}

// Start launches the admin server, listening on the configured address.
func (as *AdminServer) Start() {
	// Initialize report services
	var reportsConfig *config.Reports
	if as.fullConfig != nil {
		reportsConfig = &as.fullConfig.ReportsConf
	}
	api.InitReportServices(reportsConfig)

	if as.worker != nil {
		go as.worker.Start()
	}
	if as.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		as.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(as.config.CertPath, as.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting admin server at https://%s", as.config.ListenURL)
		log.Fatal(as.server.ListenAndServeTLS(as.config.CertPath, as.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting admin server at http://%s", as.config.ListenURL)
	log.Fatal(as.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (as *AdminServer) Shutdown() error {
	// Stop report services
	api.StopReportServices()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return as.server.Shutdown(ctx)
}

// SetupAdminRoutes creates the routes for handling requests to the web interface.
// This function returns an http.Handler to be used in http.ListenAndServe().
func (as *AdminServer) registerRoutes() {
	router := mux.NewRouter()
	// Base Front-end routes
	router.HandleFunc("/", mid.Use(as.Base, mid.RequireLogin))
	router.HandleFunc("/login", mid.Use(as.Login, as.limiter.Limit))
	if as.oidc != nil {
		router.HandleFunc("/auth/oidc/login", mid.Use(as.OIDCLogin, as.limiter.LimitGET))
		router.HandleFunc("/auth/oidc/callback", mid.Use(as.OIDCCallback, as.limiter.LimitGET))
	}
	router.HandleFunc("/logout", mid.Use(as.Logout, mid.RequireLogin))
	router.HandleFunc("/reset_password", mid.Use(as.ResetPassword, mid.RequireLogin))
	router.HandleFunc("/campaigns", mid.Use(as.Campaigns, mid.RequireLogin))
	router.HandleFunc("/campaigns/{id:[0-9]+}", mid.Use(as.CampaignID, mid.RequireLogin))
	router.HandleFunc("/campaign_sets", mid.Use(as.CampaignSets, mid.RequireLogin))
	router.HandleFunc("/templates", mid.Use(as.Templates, mid.RequireLogin))
	router.HandleFunc("/groups", mid.Use(as.Groups, mid.RequireLogin))
	router.HandleFunc("/landing_pages", mid.Use(as.LandingPages, mid.RequireLogin))
	router.HandleFunc("/sending_profiles", mid.Use(as.SendingProfiles, mid.RequireLogin))
	router.HandleFunc("/non_campaign_reports", mid.Use(as.NonCampaignReports, mid.RequireLogin))
	router.HandleFunc("/reports", mid.Use(as.Reports, mid.RequireLogin))
	router.HandleFunc("/qr_code_generator", mid.Use(as.QRGenerator, mid.RequireLogin))
	router.HandleFunc("/settings", mid.Use(as.Settings, mid.RequireLogin))
	router.HandleFunc("/api_documentation", mid.Use(as.APIDocumentation, mid.RequireLogin))
	router.HandleFunc("/users", mid.Use(as.UserManagement, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/webhooks", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/impersonate", mid.Use(as.Impersonate, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	// Create the API routes
	api := api.NewServer(
		api.WithWorker(as.worker),
		api.WithLimiter(as.limiter),
	)
	router.PathPrefix("/api/").Handler(api)

	// Setup static file serving
	router.PathPrefix("/").Handler(http.FileServer(unindexed.Dir("./static/")))

	// Setup CSRF Protection
	csrfKey := []byte(as.config.CSRFKey)
	if len(csrfKey) == 0 {
		csrfKey = []byte(auth.GenerateSecureKey(auth.APIKeyLength))
	}
	csrfHandler := csrf.Protect(csrfKey,
		csrf.FieldName("csrf_token"),
		csrf.Secure(as.config.UseTLS),
		csrf.TrustedOrigins(as.config.TrustedOrigins))
	adminHandler := csrfHandler(router)
	adminHandler = mid.Use(adminHandler.ServeHTTP, mid.CSRFExceptions, mid.GetContext, mid.ApplySecurityHeaders)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	adminHandler = gzipWrapper(adminHandler)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	adminHandler = handlers.ProxyHeaders(adminHandler)

	// Setup logging
	adminHandler = handlers.CombinedLoggingHandler(log.Writer(), adminHandler)
	as.server.Handler = adminHandler
}

type templateParams struct {
	Title              string
	Flashes            []interface{}
	User               models.User
	Token              string
	Version            string
	AnglerPhishVersion string
	ModifySystem       bool
}

// newTemplateParams returns the default template parameters for a user and
// the CSRF token.
func newTemplateParams(r *http.Request) templateParams {
	user := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	modifySystem, _ := user.HasPermission(models.PermissionModifySystem)
	return templateParams{
		Token:              csrf.Token(r),
		User:               user,
		ModifySystem:       modifySystem,
		Version:            config.Version,
		AnglerPhishVersion: config.AnglerPhishVersion,
		Flashes:            session.Flashes(),
	}
}

// Base handles the default path and template execution
func (as *AdminServer) Base(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Dashboard"
	getTemplate(w, "dashboard").ExecuteTemplate(w, "base", params)
}

// Campaigns handles the default path and template execution
func (as *AdminServer) Campaigns(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaigns"
	getTemplate(w, "campaigns").ExecuteTemplate(w, "base", params)
}

// CampaignID handles the default path and template execution
func (as *AdminServer) CampaignID(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaign Results"
	getTemplate(w, "campaign_results").ExecuteTemplate(w, "base", params)
}

// CampaignSets handles the campaign sets page
func (as *AdminServer) CampaignSets(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaign Sets"
	getTemplate(w, "campaign_sets").ExecuteTemplate(w, "base", params)
}

// Templates handles the default path and template execution
func (as *AdminServer) Templates(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Templates"
	getTemplate(w, "templates").ExecuteTemplate(w, "base", params)
}

// SMSTemplates handles the default path and template execution for SMS templates
func (as *AdminServer) SMSTemplates(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "SMS Templates"
	getTemplate(w, "sms_templates").ExecuteTemplate(w, "base", params)
}

// Groups handles the default path and template execution
func (as *AdminServer) Groups(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Users & Groups"
	getTemplate(w, "groups").ExecuteTemplate(w, "base", params)
}

// LandingPages handles the default path and template execution
func (as *AdminServer) LandingPages(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Landing Pages"
	getTemplate(w, "landing_pages").ExecuteTemplate(w, "base", params)
}

// SendingProfiles handles the default path and template execution
func (as *AdminServer) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Sending Profiles"
	getTemplate(w, "sending_profiles").ExecuteTemplate(w, "base", params)
}

// QRGenerator handles the default path and template execution
func (as *AdminServer) QRGenerator(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "QR Code Generator"
	getTemplate(w, "qr_code_generator").ExecuteTemplate(w, "base", params)
}

// Reports handles the reports page
func (as *AdminServer) Reports(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Reports"
	getTemplate(w, "reports").ExecuteTemplate(w, "base", params)
}

// Settings handles the changing of settings
func (as *AdminServer) Settings(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		params := newTemplateParams(r)
		params.Title = "Settings"
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Save(r, w)
		getTemplate(w, "settings").ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		u := ctx.Get(r, "user").(models.User)
		currentPw := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_new_password")
		// Check the current password
		err := auth.ValidatePassword(currentPw, u.Hash)
		msg := models.Response{Success: true, Message: "Settings Updated Successfully"}
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		u.Hash = string(newHash)
		if err = models.PutUser(&u); err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusInternalServerError)
			return
		}
		api.JSONResponse(w, msg, http.StatusOK)
	}
}

// UserManagement is an admin-only handler that allows for the registration
// and management of user accounts within Gophish.
func (as *AdminServer) UserManagement(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "User Management"
	getTemplate(w, "users").ExecuteTemplate(w, "base", params)
}

func (as *AdminServer) nextOrIndex(w http.ResponseWriter, r *http.Request) {
	next := "/"
	url, err := url.Parse(r.FormValue("next"))
	if err == nil {
		path := url.EscapedPath()
		if path != "" {
			next = "/" + strings.TrimLeft(path, "/")
		}
	}
	http.Redirect(w, r, next, http.StatusFound)
}

type loginParams struct {
	User        models.User
	Title       string
	Flashes     []interface{}
	Token       string
	OIDCEnabled bool
}

func (as *AdminServer) oidcEnabled() bool {
	return as.oidc != nil
}

func oidcConfigFrom(cfg config.OIDC) auth.OIDCConfig {
	return auth.OIDCConfig{
		Enabled:              cfg.Enabled,
		Issuer:               cfg.Issuer,
		ClientID:             cfg.ClientID,
		RedirectURL:          cfg.RedirectURL,
		RequiredGroup:        cfg.RequiredGroup,
		GroupsClaim:          cfg.GroupsClaim,
		UsernameFromEmail:    cfg.UsernameFromEmail,
		AllowUnverifiedEmail: cfg.AllowUnverifiedEmail,
	}
}

func (as *AdminServer) renderLogin(w http.ResponseWriter, r *http.Request, params loginParams, status int) {
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(status)
	template.Must(templates, err).ExecuteTemplate(w, "base", params)
}

func (as *AdminServer) handleInvalidLogin(w http.ResponseWriter, r *http.Request, message string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	Flash(w, r, "danger", message)
	params := loginParams{Title: "Login", Token: csrf.Token(r), OIDCEnabled: as.oidcEnabled()}
	params.Flashes = session.Flashes()
	session.Save(r, w)
	as.renderLogin(w, r, params, http.StatusUnauthorized)
}

func (as *AdminServer) handleOIDCLoginFailure(w http.ResponseWriter, r *http.Request, message string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	Flash(w, r, "danger", message)
	params := loginParams{Title: "Login", Token: csrf.Token(r), OIDCEnabled: as.oidcEnabled()}
	params.Flashes = session.Flashes()
	session.Save(r, w)
	as.renderLogin(w, r, params, http.StatusForbidden)
}

// Webhooks is an admin-only handler that handles webhooks
func (as *AdminServer) Webhooks(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Webhooks"
	getTemplate(w, "webhooks").ExecuteTemplate(w, "base", params)
}

// APIDocumentation handles the API documentation page
func (as *AdminServer) APIDocumentation(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "API Documentation"
	getTemplate(w, "api_documentation").ExecuteTemplate(w, "base", params)
}

// Impersonate allows an admin to login to a user account without needing the password
func (as *AdminServer) Impersonate(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		username := r.FormValue("username")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Values["id"] = u.Id
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// OIDCLogin starts the OIDC authorization-code flow.
func (as *AdminServer) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if as.oidc == nil {
		http.NotFound(w, r)
		return
	}
	session := ctx.Get(r, "session").(*sessions.Session)
	state := auth.GenerateOAuthState()
	session.Values["oidc_state"] = state
	if err := session.Save(r, w); err != nil {
		log.Error(err)
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	authURL, err := as.oidc.AuthCodeURL(r.Context(), state)
	if err != nil {
		switch err {
		case auth.ErrOIDCDiscoveryFailed:
			log.Warn("OIDC provider discovery failed")
		default:
			log.Error(err)
		}
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback completes the OIDC flow and creates an admin session.
func (as *AdminServer) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if as.oidc == nil {
		http.NotFound(w, r)
		return
	}
	session := ctx.Get(r, "session").(*sessions.Session)
	expectedState, _ := session.Values["oidc_state"].(string)
	delete(session.Values, "oidc_state")
	if expectedState == "" || r.URL.Query().Get("state") != expectedState {
		log.Warn("OIDC callback received invalid state")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		log.Warn("OIDC provider returned error")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Warn("OIDC callback missing authorization code")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	claims, err := as.oidc.Exchange(r.Context(), code)
	if err != nil {
		switch err {
		case auth.ErrOIDCAccessDenied:
			log.Warn("OIDC login denied")
		case auth.ErrOIDCDiscoveryFailed:
			log.Warn("OIDC provider discovery failed")
		default:
			log.Error(err)
		}
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	username, err := auth.UsernameFromEmail(claims.Email, as.oidc.UsernameMapping())
	if err != nil {
		log.Warn("OIDC login could not map email to username")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	u, err := models.GetUserByUsername(username)
	if err != nil {
		log.Warn("OIDC login for unprovisioned user")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	if u.AccountLocked {
		log.Warn("OIDC login for locked account")
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	u.LastLogin = time.Now().UTC()
	if err = models.PutUser(&u); err != nil {
		log.Error(err)
	}
	session.Values["id"] = u.Id
	session.Values["auth_method"] = "oidc"
	if err = session.Save(r, w); err != nil {
		log.Error(err)
		as.handleOIDCLoginFailure(w, r, "Access denied")
		return
	}
	as.nextOrIndex(w, r)
}

// Login handles the authentication flow for a user. If credentials are valid,
// a session is created
func (as *AdminServer) Login(w http.ResponseWriter, r *http.Request) {
	params := loginParams{Title: "Login", Token: csrf.Token(r), OIDCEnabled: as.oidcEnabled()}
	session := ctx.Get(r, "session").(*sessions.Session)
	switch {
	case r.Method == "GET":
		params.Flashes = session.Flashes()
		session.Save(r, w)
		as.renderLogin(w, r, params, http.StatusOK)
	case r.Method == "POST":
		// Find the user with the provided username
		username, password := r.FormValue("username"), r.FormValue("password")
		if as.oidcEnabled() && username != models.DefaultAdminUsername {
			as.handleInvalidLogin(w, r, "Use SSO to sign in")
			return
		}
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		// Validate the user's password
		err = auth.ValidatePassword(password, u.Hash)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		if u.AccountLocked {
			as.handleInvalidLogin(w, r, "Account Locked")
			return
		}
		u.LastLogin = time.Now().UTC()
		err = models.PutUser(&u)
		if err != nil {
			log.Error(err)
		}
		// If we've logged in, save the session and redirect to the dashboard
		delete(session.Values, "auth_method")
		session.Values["id"] = u.Id
		session.Save(r, w)
		as.nextOrIndex(w, r)
	}
}

// Logout destroys the current user session
func (as *AdminServer) Logout(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)
	delete(session.Values, "id")
	delete(session.Values, "auth_method")
	Flash(w, r, "success", "You have successfully logged out")
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ResetPassword handles the password reset flow when a password change is
// required either by the Gophish system or an administrator.
//
// This handler is meant to be used when a user is required to reset their
// password, not just when they want to.
//
// This is an important distinction since in this handler we don't require
// the user to re-enter their current password, as opposed to the flow
// through the settings handler.
//
// To that end, if the user doesn't require a password change, we will
// redirect them to the settings page.
func (as *AdminServer) ResetPassword(w http.ResponseWriter, r *http.Request) {
	u := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	if !u.PasswordChangeRequired {
		Flash(w, r, "info", "Please reset your password through the settings page")
		session.Save(r, w)
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
		return
	}
	params := newTemplateParams(r)
	params.Title = "Reset Password"
	switch {
	case r.Method == http.MethodGet:
		params.Flashes = session.Flashes()
		session.Save(r, w)
		getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
		return
	case r.Method == http.MethodPost:
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusBadRequest)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		u.PasswordChangeRequired = false
		u.Hash = newHash
		if err = models.PutUser(&u); err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusInternalServerError)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		// TODO: We probably want to flash a message here that the password was
		// changed successfully. The problem is that when the user resets their
		// password on first use, they will see two flashes on the dashboard-
		// one for their password reset, and one for the "no campaigns created".
		//
		// The solution to this is to revamp the empty page to be more useful,
		// like a wizard or something.
		as.nextOrIndex(w, r)
	}
}

// TODO: Make this execute the template, too
func getTemplate(_ http.ResponseWriter, tmpl string) *template.Template {
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/base.html", "templates/nav.html", "templates/"+tmpl+".html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	return template.Must(templates, err)
}

// Flash handles the rendering flash messages
func Flash(w http.ResponseWriter, r *http.Request, t string, m string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	session.AddFlash(models.Flash{
		Type:    t,
		Message: m,
	})
}
