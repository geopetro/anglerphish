package api

import (
	"net/http"
	"strconv"
	"strings"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/dmi"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

const (
	dmiGoogleScope       = "https://www.googleapis.com/auth/gmail.insert"
	dmiGoogleLabelsScope = "https://www.googleapis.com/auth/gmail.labels"
)

type dmiSettingsResponse struct {
	GoogleKeySet      bool   `json:"google_key_set"`
	GoogleClientID    string `json:"google_client_id"`
	GoogleClientEmail string `json:"google_client_email"`
	Scope             string `json:"scope"`
	LabelsScope       string `json:"labels_scope"`
	Error             string `json:"error"`
}

type dmiTestRequest struct {
	Mailbox string `json:"mailbox"`
}

type dmiSettingsRequest struct {
	GoogleKey string `json:"google_key"`
}

// dmiProfile loads the profile named in the URL for the session user.
func dmiProfile(w http.ResponseWriter, r *http.Request) (models.SMTP, bool) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	s, err := models.GetSMTP(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMTP not found"}, http.StatusNotFound)
		return s, false
	}
	return s, true
}

// certificateFilename builds a header-safe download name from the profile name.
func certificateFilename(name string) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '"', 13, 10, '/', 92:
			return -1
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "certificate.cer"
	}
	return clean + ".cer"
}

// SendingProfileDMICertificate downloads (GET) or regenerates (POST) the
// certificate of a Direct Injection profile.
func (as *Server) SendingProfileDMICertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	s, ok := dmiProfile(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		if !s.IsDMI() || s.DMIAuthMethod != dmi.AuthCertificate || s.DMICertificate == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Certificate not found"}, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", `attachment; filename="`+certificateFilename(s.Name)+`"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(s.DMICertificate))
		return
	}
	s, err := models.RegenerateDMICertificate(s.Id, s.UserId)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, s, http.StatusOK)
}

// SendingProfileDMITest checks that a Direct Injection profile can reach a mailbox.
func (as *Server) SendingProfileDMITest(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	s, ok := dmiProfile(w, r)
	if !ok {
		return
	}
	req := dmiTestRequest{}
	if !decodeBody(w, r, &req) {
		return
	}
	req.Mailbox = strings.TrimSpace(req.Mailbox)
	if req.Mailbox == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Mailbox is required"}, http.StatusBadRequest)
		return
	}
	if err := models.TestDMIConnection(s.Id, s.UserId, req.Mailbox); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusOK)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Connection successful"}, http.StatusOK)
}

func writeDMISettings(w http.ResponseWriter) {
	set, clientID, clientEmail, err := models.DMIGoogleKeyInfo()
	resp := dmiSettingsResponse{
		GoogleKeySet:      set,
		GoogleClientID:    clientID,
		GoogleClientEmail: clientEmail,
		Scope:             dmiGoogleScope,
		LabelsScope:       dmiGoogleLabelsScope,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	w.Header().Set("Cache-Control", "no-store")
	JSONResponse(w, resp, http.StatusOK)
}

// DMISettings returns (GET), saves (PUT) or clears (DELETE) the operator Google key.
func (as *Server) DMISettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeDMISettings(w)
	case http.MethodPut:
		req := dmiSettingsRequest{}
		if !decodeBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.GoogleKey) == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Google key is required"}, http.StatusBadRequest)
			return
		}
		if err := models.SaveDMIGoogleKey(req.GoogleKey); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		log.Infof("dmi: Google key updated by %s", encryptionUser(r).Username)
		writeDMISettings(w)
	case http.MethodDelete:
		if err := models.SaveDMIGoogleKey(""); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("dmi: Google key cleared by %s", encryptionUser(r).Username)
		writeDMISettings(w)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}
