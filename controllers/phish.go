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
	"github.com/gophish/gophish/controllers/api"
	log "github.com/gophish/gophish/logger"
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

// TransparencyResponse is the JSON response provided when a third-party
// makes a request to the transparency handler.
type TransparencyResponse struct {
	Server         string    `json:"server"`
	ContactAddress string    `json:"contact_address"`
	SendDate       time.Time `json:"send_date"`
}

// TransparencySuffix (when appended to a valid result ID), will cause Gophish
// to return a transparency response.
const TransparencySuffix = "+"

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

// WithContactAddress sets the contact address used by the transparency
// handlers
func WithContactAddress(addr string) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.contactAddress = addr
	}
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
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

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
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailReport(d)
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

	// In order to identify if campaign uses Basic Auth
	c := ctx.Get(r, "campaign").(models.Campaign)

	w.Header().Set("X-Server", config.ServerName) // Useful for checking if this is a GoPhish server (e.g. for campaign reporting plugins)
	var ptx models.PhishingTemplateContext
	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		ptx, err = models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
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

		if !c.HTTPAuth {
			renderPhishResponse(w, r, ptx, p)
		} else {
			renderBasicAuth(w, r, ptx, p)
		}
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	c = ctx.Get(r, "campaign").(models.Campaign)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

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
			err = rs.HandleFormSubmit(d)
			if err != nil {
				log.Error(err)
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

	ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
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

// TransparencyHandler returns a TransparencyResponse for the provided result
// and campaign.
func (ps *PhishingServer) TransparencyHandler(w http.ResponseWriter, r *http.Request) {
	rs := ctx.Get(r, "result").(models.Result)
	tr := &TransparencyResponse{
		Server:         config.ServerName,
		SendDate:       rs.SendDate,
		ContactAddress: ps.contactAddress,
	}
	api.JSONResponse(w, tr, http.StatusOK)
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

	// Since we want to support the common case of adding a "+" to indicate a
	// transparency request, we need to take care to handle the case where the
	// request ends with a space, since a "+" is technically reserved for use
	// as a URL encoding of a space.
	if strings.HasSuffix(rid, " ") {
		// We'll trim off the space
		rid = strings.TrimRight(rid, " ")

		// Then we'll add the transparency suffix
		rid = fmt.Sprintf("%s%s", rid, TransparencySuffix)
	}
	// Finally, if this is a transparency request, we'll need to verify that
	// a valid rid has been provided, so we'll look up the result with a
	// trimmed parameter.
	id := strings.TrimSuffix(rid, TransparencySuffix)

	// Check to see if this is a preview or a real result
	if strings.HasPrefix(id, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(id)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}

	rs, err := models.GetResult(id)
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
