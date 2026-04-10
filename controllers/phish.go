package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/util"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jordan-wright/unindexed"
	bm "github.com/microcosm-cc/bluemonday"
)

// ErrInvalidRequest is thrown when a request with an invalid structure is
// received
var ErrInvalidRequest = errors.New("Invalid request")

// ErrCampaignComplete is thrown when an event is received for a campaign that
// has already been marked as complete.
var ErrCampaignComplete = errors.New("Event received on completed campaign")

// PhishingServerOption is a functional option that is used to configure the
// the phishing server
type PhishingServerOption func(*PhishingServer)

// PhishingServer is an HTTP server that implements the campaign event
// handlers, such as email open tracking, click tracking, and more.
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
}

// NewPhishingServer returns a new instance of the phishing server with
// provided options applied.
func NewPhishingServer(config config.PhishServer, options ...PhishingServerOption) *PhishingServer {
	defaultServer := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         config.ListenURL,
	}
	ps := &PhishingServer{
		server: defaultServer,
		config: config,
	}
	for _, opt := range options {
		opt(ps)
	}
	ps.registerRoutes()
	return ps
}

// Overwrite net.https Error with a custom one to set our own headers
// Go's internal Error func returns text/plain so browser's won't render the html
func customError(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Server", "Microsoft-IIS/10.0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src https:")
	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

// Overwrite go's internal not found to allow templating the not found page
// The templating string is currently not passed in, therefore there is no templating yet
// If I need it in the future, it's a 5 minute change...
func customNotFound(w http.ResponseWriter, r *http.Request) {
	tmpl404, err := template.ParseFiles("templates/404.html")
	if err != nil {
		log.Fatal(err)
	}
	var b bytes.Buffer
	err = tmpl404.Execute(&b, "")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	customError(w, b.String(), http.StatusNotFound)
}

// Start launches the phishing server, listening on the configured address.
func (ps *PhishingServer) Start() {
	if ps.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		ps.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(ps.config.CertPath, ps.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting phishing server at https://%s", ps.config.ListenURL)
		log.Fatal(ps.server.ListenAndServeTLS(ps.config.CertPath, ps.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting phishing server at http://%s", ps.config.ListenURL)
	log.Fatal(ps.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (ps *PhishingServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return ps.server.Shutdown(ctx)
}

// CreatePhishingRouter creates the router that handles phishing connections.
func (ps *PhishingServer) registerRoutes() {
	router := mux.NewRouter()
	fileServer := http.FileServer(unindexed.Dir("./static/endpoint/"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))
	router.HandleFunc("/track", ps.TrackHandler)
	router.HandleFunc("/robots.txt", ps.RobotsHandler)
	router.HandleFunc("/{path:.*}/track", ps.TrackHandler)
	router.HandleFunc("/{path:.*}/report", ps.ReportHandler)
	router.HandleFunc("/report", ps.ReportHandler)
	router.HandleFunc("/{path:.*}/replied", ps.RepliedHandler)
	router.HandleFunc("/replied", ps.RepliedHandler)
	router.HandleFunc("/{path:.*}", ps.PhishHandler)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	phishHandler := gzipWrapper(router)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	phishHandler = handlers.ProxyHeaders(phishHandler)

	// Setup logging
	phishHandler = handlers.CombinedLoggingHandler(log.Writer(), phishHandler)
	ps.server.Handler = phishHandler
}

// TrackHandler tracks emails as they are opened, updating the status for the given Result
func (ps *PhishingServer) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		http.ServeFile(w, r, "static/images/pixel.png")
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	d := ctx.Get(r, "details").(models.EventDetails)

	// We can only track opens for email campaigns (via tracking pixels)
	// SMS opens cannot be tracked, only clicks can be tracked
	err = rs.HandleEmailOpened(d)

	if err != nil {
		log.Error(err)
	}
	http.ServeFile(w, r, "static/images/pixel.png")
}

// ReportHandler tracks emails as they are reported, updating the status for the given Result
func (ps *PhishingServer) ReportHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*") // To allow Chrome extensions (or other pages) to report a campaign without violating CORS
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	d := ctx.Get(r, "details").(models.EventDetails)

	err = rs.HandleEmailReport(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// RepliedHandler tracks emails as they are replied to, updating the status for the given Result
func (ps *PhishingServer) RepliedHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	d := ctx.Get(r, "details").(models.EventDetails)

	err = rs.HandleEmailReply(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// PhishHandler handles incoming client connections and registers the associated actions performed
// (such as clicked link, etc.)
func (ps *PhishingServer) PhishHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		if r.Header.Get("X-Tracked") == "true" { // Handle Custom Requests with X-Tracked header with value true
			customTrackedPhishRequests(r)
		}
		customNotFound(w, r)
		return
	}

	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		// For email previews
		ptx, err := models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
		p, err := models.GetPage(preview.PageId, preview.UserId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}

		// For previews, we don't have campaign context, so assume no HTTPAuth
		renderPhishResponse(w, r, ptx, p)
		return
	}

	// For non-preview requests, get campaign and result
	rs := ctx.Get(r, "result").(models.Result)
	c := ctx.Get(r, "campaign").(models.Campaign)
	d := ctx.Get(r, "details").(models.EventDetails)

	p, err := models.GetPage(c.PageId, c.UserId)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
		return
	}

	if !c.HTTPAuth {
		switch {
		case r.Method == "GET":
			err = rs.HandleClickedLink(d)
			if err != nil {
				log.Error(err)
			}
		case r.Method == "POST":
			// Check if this is an MFA code submission (not initial credentials)
			mfaCode := r.FormValue("mfa_code")

			if p.EnableMFA && mfaCode != "" {
				// This is an MFA verification attempt - don't record as form submit
				mfaResult := handleMFAVerification(w, r, rs, p, d, c, mfaCode)
				if mfaResult {
					return
				}
			} else {
				// This is a normal form submission (credentials)
				// Record the form submission first
				err = rs.HandleFormSubmit(d)
				if err != nil {
					log.Error(err)
				}

				// Then check if MFA is enabled to initiate MFA flow
				if p.EnableMFA {
					mfaResult := handleMFAFlow(w, r, rs, p, d, c)
					if mfaResult {
						// MFA flow handled the response, return early
						return
					}
					// MFA flow returned false means proceed normally (no phone, MFA skipped, etc.)
				}
			}
		}
	} else {
		username, password, ok := r.BasicAuth()
		if !ok && (username == "" || password == "") {
			err = rs.HandleClickedLink(d)
			if err != nil {
				log.Error(err)
			}
		} else if ok && (username == "" || password == "") {
			// If credentials are empty do nothing
			// Don't keep recording them as clicks
			// and don't allow proceeding to recording empty credentials
		} else {
			// d contains a Payload member of type net.url.Values
			// which itself is just map[string][]string
			// Manually overwrite it with basic auth data
			var payload map[string][]string
			if p.CapturePasswords {
				payload = map[string][]string{"Username": {username}, "Password": {password}}
			} else {
				payload = map[string][]string{"Username": {username}}
			}
			d.Payload = payload
			err = rs.HandleFormSubmit(d)
			if err != nil {
				log.Error(err)
			}
		}
	}

	var ptx models.PhishingTemplateContext

	// Create the appropriate template context based on campaign type
	switch c.Type {
	case "sms":
		// For SMS campaigns, use SMSTemplateContext
		stx, err := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
		// Convert to a PhishingTemplateContext for rendering
		ptx = models.PhishingTemplateContext{
			BaseRecipient: rs.BaseRecipient,
			RId:           rs.RId,
			URL:           stx.URL,
			TrackingURL:   stx.TrackingURL,
			Tracker:       "<img alt='' style='display: none' src='" + stx.TrackingURL + "'/>",
			From:          stx.From,
			BaseURL:       stx.BaseURL,
		}
	case "generic":
		// For generic campaigns, create a minimal template context
		// Generic campaigns don't have email templates, they just serve the landing page
		trackingURL, err := models.ExecuteTemplate(c.URL, nil)
		if err != nil {
			trackingURL = c.URL
		}
		ptx = models.PhishingTemplateContext{
			BaseRecipient: rs.BaseRecipient,
			RId:           rs.RId,
			URL:           c.URL,
			TrackingURL:   trackingURL + "/track?rid=" + rs.RId,
			Tracker:       "<img alt='' style='display: none' src='" + trackingURL + "/track?rid=" + rs.RId + "'/>",
			From:          "",
			BaseURL:       c.URL,
		}
	default:
		// For email campaigns, use PhishingTemplateContext
		ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
	}

	if !c.HTTPAuth {
		renderPhishResponse(w, r, ptx, p)
	} else {
		renderBasicAuth(w, r, ptx, p)
	}
}

// This function serves the purpose of handling campaign responses in cases that user response id is not known.
// Such cases can include requests made by .doc, .xls files with macros, or any type of malicious file that makes an HTTP request
// from which we want to capture the users that interacted with it but we cannot have their Rid embedded
func customTrackedPhishRequests(r *http.Request) {

	data := strings.ReplaceAll(r.URL.String(), "/", "")
	data = strings.ReplaceAll(data, "?", ",")
	data = strings.ReplaceAll(data, "&", ",")

	if err := appendToFile(data); err != nil {
		fmt.Println("Error appending to file:", err)
	}
}

// Used by customTrackedPhishRequests() to write data in a csv file.
func appendToFile(data string) error {
	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile("tracked-data.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Append data to the file
	if _, err := file.WriteString(data + "\n"); err != nil {
		return err
	}
	return nil
}

// renderPhishResponse handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderPhishResponse(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	// If the request was a form submit and a redirect URL was specified, we
	// should send the user to that URL
	if r.Method == "POST" {
		if p.RedirectURL != "" {
			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				customNotFound(w, r)
				return
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	}
	// Otherwise, we just need to write out the templated HTML
	html, err := models.ExecuteTemplate(p.HTML, ptx)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
		return
	}
	w.Write([]byte(html))
}

// renderBasicAuth handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderBasicAuth(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	uname, passwd, ok := r.BasicAuth()

	// If the request contains a Basic Auth header and credentials are not empty, send the user to the redirect URL
	if ok && uname != "" && passwd != "" {
		redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}
	// Otherwise, send a response containing the WWW-Authenticate header and
	// render the template as string there
	stp := bm.StripTagsPolicy()
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, stp.Sanitize(p.HTML)))
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"message": "You are not authorized to view this page."}`))
	// w.Write([]byte(`<h1>Unauthorized</h1><p>You are not authorized to view this page.</p>`))
}

// RobotsHandler prevents search engines, etc. from indexing phishing materials
func (ps *PhishingServer) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

// setupContext handles some of the administrative work around receiving a new
// request, such as checking the result ID, the campaign, etc.
func setupContext(r *http.Request) (*http.Request, error) {
	err := r.ParseForm()
	if err != nil {
		log.Error(err)
		return r, err
	}

	// This is used to handle the different Recipient Parameters
	// that can be used per campaign. Since the parameter will not always be rid,
	// below we identify the parameter name by getting the first key from the key-value pair
	// of the HTTP request
	var urlparam string

	// Ensures to get the last url parameter (i.e., rid)
	// Especially for the case a custom url is provided with several parameters
	queryString := r.URL.RawQuery // Get the query string from the URL

	// Default to "rid" as the parameter name if we can't find one
	urlparam = "rid"

	// Only try to parse if we have a query string
	if queryString != "" {
		pairs := strings.Split(queryString, "&") // Split the query string into key-value pairs based on "&" delimiter

		if len(pairs) > 0 {
			lastPair := pairs[len(pairs)-1] // Get the last key-value pair

			// Make sure the pair has a key=value format
			if strings.Contains(lastPair, "=") {
				keyValue := strings.Split(lastPair, "=") // Split the last pair into key and value based on "=" delimiter
				if len(keyValue) > 0 && keyValue[0] != "" {
					urlparam = keyValue[0] // Extract the parameter name (left part of "=")
				}
			}
		}
	}

	rid := r.URL.Query().Get(urlparam)
	if rid == "" {
		return r, ErrInvalidRequest
	}

	// Check to see if this is a preview or a real result
	if strings.HasPrefix(rid, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(rid)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}

	rs, err := models.GetResult(rid)
	if err != nil {
		return r, err
	}
	c, err := models.GetCampaign(rs.CampaignId, rs.UserId)
	if err != nil {
		log.Error(err)
		return r, err
	}
	// Don't process events for completed campaigns
	if c.Status == models.CampaignComplete {
		return r, ErrCampaignComplete
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Handle post processing such as GeoIP
	err = rs.UpdateGeo(ip)
	if err != nil {
		log.Error(err)
	}
	d := models.EventDetails{
		Payload: r.Form,
		Browser: make(map[string]string),
	}
	d.Browser["address"] = ip
	d.Browser["user-agent"] = r.Header.Get("User-Agent")

	r = ctx.Set(r, "rid", rid)
	r = ctx.Set(r, "result", rs)
	r = ctx.Set(r, "campaign", c)
	r = ctx.Set(r, "details", d)
	return r, nil
}

// handleMFAFlow handles the MFA verification flow for landing pages
// Returns true if the MFA flow handled the response (caller should return early)
// Returns false if MFA should be skipped (no phone available, etc.)
func handleMFAFlow(w http.ResponseWriter, r *http.Request, rs models.Result, p models.Page, d models.EventDetails, c models.Campaign) bool {
	// Check if this is an MFA code submission (has mfa_code field)
	mfaCode := r.FormValue("mfa_code")
	if mfaCode != "" {
		// This is an MFA verification attempt
		return handleMFAVerification(w, r, rs, p, d, c, mfaCode)
	}

	// This is an initial credential submission - check if we should initiate MFA
	// First, get the phone number: priority is result.Phone (pre-loaded), then form data
	phone := rs.Phone
	if phone == "" {
		// Try to get phone from form submission
		phone = r.FormValue("phone")
		if phone == "" {
			phone = r.FormValue("Phone")
		}
	}

	// If no phone is available, skip MFA and proceed normally
	if phone == "" {
		log.Warnf("MFA enabled for page %d but no phone available for rid %s, skipping MFA", p.Id, rs.RId)
		return false
	}

	// Get the SMS profile for sending the MFA code
	if p.MFASMSProfileId == 0 {
		log.Warnf("MFA enabled for page %d but no SMS profile configured, skipping MFA", p.Id)
		return false
	}

	smsProfile, err := models.GetSMS(p.MFASMSProfileId, p.UserId)
	if err != nil {
		log.Errorf("Failed to get SMS profile %d for MFA: %v", p.MFASMSProfileId, err)
		return false
	}

	// Generate MFA code
	codeLength := p.MFACodeLength
	if codeLength <= 0 {
		codeLength = 6
	}
	codeType := p.MFACodeType
	if codeType == "" {
		codeType = models.MFACodeTypeNumeric
	}

	code, err := models.GenerateMFACode(codeLength, codeType)
	if err != nil {
		log.Errorf("Failed to generate MFA code for rid %s: %v", rs.RId, err)
		return false
	}

	// Save the MFA code
	_, err = models.SaveMFACode(rs.RId, code, phone)
	if err != nil {
		log.Errorf("Failed to save MFA code for rid %s: %v", rs.RId, err)
		return false
	}

	// Generate the SMS message
	message := models.GenerateMFAMessage(p.MFAMessage, code)

	// Send the SMS - use MFAFrom if specified, otherwise use profile default
	fromSender := p.MFAFrom
	if fromSender == "" {
		fromSender = smsProfile.From
	}
	err = sendMFASMS(smsProfile, phone, message, fromSender)
	if err != nil {
		log.Errorf("Failed to send MFA SMS to %s for rid %s: %v", phone, rs.RId, err)
		// Record the failure as a visible event in campaign results
		if recordErr := rs.HandleMFACodeSendError(err); recordErr != nil {
			log.Errorf("Failed to record MFA send error event for rid %s: %v", rs.RId, recordErr)
		}
		// Still show the MFA page even if SMS failed - user can retry
	} else {
		log.Infof("MFA code sent to %s for rid %s (from: %s)", phone, rs.RId, fromSender)
		// Only record "MFA Code Sent" when the SMS was actually delivered to the provider
		mfaSentDetails := models.EventDetails{
			Payload: make(map[string][]string),
			Browser: d.Browser,
		}
		mfaSentDetails.Payload["mfa_phone"] = []string{phone}
		mfaSentDetails.Payload["mfa_from"] = []string{fromSender}
		if recordErr := rs.HandleMFACodeSent(mfaSentDetails); recordErr != nil {
			log.Errorf("Failed to record MFA code sent event for rid %s: %v", rs.RId, recordErr)
		}
	}

	// Show the MFA verification page
	if p.MFAInjectPage {
		// Use the injected MFA page (custom if set, otherwise default)
		mfaHTML := models.RenderMFAPage(p.MFAPageHTML, rs.RId, "")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mfaHTML))
	} else {
		// Re-render the original page - user's JS should handle showing MFA input
		// We'll add a special header or hidden field to indicate MFA is pending
		var ptx models.PhishingTemplateContext
		switch c.Type {
		case "sms":
			stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           stx.URL,
				TrackingURL:   stx.TrackingURL,
				Tracker:       "<img alt='' style='display: none' src='" + stx.TrackingURL + "'/>",
				From:          stx.From,
				BaseURL:       stx.BaseURL,
			}
		case "generic":
			trackingURL, _ := models.ExecuteTemplate(c.URL, nil)
			if trackingURL == "" {
				trackingURL = c.URL
			}
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           c.URL,
				TrackingURL:   trackingURL + "/track?rid=" + rs.RId,
				Tracker:       "<img alt='' style='display: none' src='" + trackingURL + "/track?rid=" + rs.RId + "'/>",
				From:          "",
				BaseURL:       c.URL,
			}
		default:
			ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
		}

		// Add a hidden indicator that MFA is pending
		html, err := models.ExecuteTemplate(p.HTML, ptx)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return true
		}

		// Inject MFA pending indicator before </body>
		mfaIndicator := `<input type="hidden" id="mfa_pending" value="true"><script>window.mfaPending=true;</script>`
		html = strings.Replace(html, "</body>", mfaIndicator+"</body>", 1)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}

	return true
}

// handleMFAVerification handles the verification of an MFA code
func handleMFAVerification(w http.ResponseWriter, r *http.Request, rs models.Result, p models.Page, d models.EventDetails, c models.Campaign, submittedCode string) bool {
	// Verify the MFA code
	verified, err := models.VerifyMFACode(rs.RId, submittedCode)
	if err != nil {
		log.Warnf("MFA verification error for rid %s: %v", rs.RId, err)
	}

	if verified {
		// Mark the code as verified
		models.MarkMFACodeVerified(rs.RId)

		log.Infof("MFA code verified successfully for rid %s", rs.RId)

		// Record the MFA verification event
		mfaVerifiedDetails := models.EventDetails{
			Payload: make(map[string][]string),
			Browser: d.Browser,
		}
		mfaVerifiedDetails.Payload["mfa_verified"] = []string{"true"}
		err = rs.HandleMFACodeVerified(mfaVerifiedDetails)
		if err != nil {
			log.Error(err)
		}

		// Proceed to redirect or show the final page
		if p.RedirectURL != "" {
			var ptx models.PhishingTemplateContext
			switch c.Type {
			case "sms":
				stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
				ptx = models.PhishingTemplateContext{
					BaseRecipient: rs.BaseRecipient,
					RId:           rs.RId,
					URL:           stx.URL,
					TrackingURL:   stx.TrackingURL,
					From:          stx.From,
					BaseURL:       stx.BaseURL,
				}
			case "generic":
				ptx = models.PhishingTemplateContext{
					BaseRecipient: rs.BaseRecipient,
					RId:           rs.RId,
					URL:           c.URL,
					BaseURL:       c.URL,
				}
			default:
				ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
			}

			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				customNotFound(w, r)
				return true
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return true
		}

		// No redirect, just show success or re-render page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Verified</title></head><body><h1>Verification successful</h1></body></html>`))
		return true
	}

	// Verification failed
	log.Warnf("MFA code verification failed for rid %s", rs.RId)

	// Record the MFA failure event
	mfaFailedDetails := models.EventDetails{
		Payload: make(map[string][]string),
		Browser: d.Browser,
	}
	mfaFailedDetails.Payload["mfa_verified"] = []string{"false"}
	mfaFailedDetails.Payload["mfa_code_submitted"] = []string{submittedCode}
	err = rs.HandleMFACodeFailed(mfaFailedDetails)
	if err != nil {
		log.Error(err)
	}

	// Show the MFA page again with an error
	if p.MFAInjectPage {
		mfaHTML := models.RenderMFAPage(p.MFAPageHTML, rs.RId, "Invalid verification code. Please try again.")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mfaHTML))
	} else {
		// Re-render page with error indicator
		var ptx models.PhishingTemplateContext
		switch c.Type {
		case "sms":
			stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           stx.URL,
				TrackingURL:   stx.TrackingURL,
				From:          stx.From,
				BaseURL:       stx.BaseURL,
			}
		case "generic":
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           c.URL,
				BaseURL:       c.URL,
			}
		default:
			ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
		}

		html, err := models.ExecuteTemplate(p.HTML, ptx)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return true
		}

		// Inject MFA error indicator
		mfaIndicator := `<input type="hidden" id="mfa_error" value="Invalid code"><script>window.mfaError="Invalid code";</script>`
		html = strings.Replace(html, "</body>", mfaIndicator+"</body>", 1)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}

	return true
}

// sendMFASMS sends an MFA code via SMS using the provided SMS profile
// fromSender allows overriding the sender ID (if empty, uses profile default)
func sendMFASMS(smsProfile models.SMS, phone string, message string, fromSender string) error {
	// Ensure phone number is in E.164 format (starts with +)
	if phone != "" && !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	// Get the SMS dialer
	dialer, err := mailer.GetSMSDialer(smsProfile.Provider, smsProfile.ProviderConfig)
	if err != nil {
		return fmt.Errorf("failed to get SMS dialer: %w", err)
	}

	// Dial to get the provider
	provider, err := dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial SMS provider: %w", err)
	}
	defer provider.Close()

	// Send the SMS with the specified sender
	err = provider.Send(fromSender, phone, message)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	return nil
}
