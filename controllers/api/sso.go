package api

import (
	"errors"
	"net/http"

	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/crypto"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// ssoResponse is the API view of the SSO settings. It never carries the
// client secret.
type ssoResponse struct {
	Enabled              bool   `json:"enabled"`
	Provider             string `json:"provider"`
	Issuer               string `json:"issuer"`
	ClientID             string `json:"client_id"`
	RedirectURL          string `json:"redirect_url"`
	RequiredGroup        string `json:"required_group"`
	GroupsClaim          string `json:"groups_claim"`
	UsernameFromEmail    string `json:"username_from_email"`
	AllowUnverifiedEmail bool   `json:"allow_unverified_email"`
	SecretSet            bool   `json:"secret_set"`
	Status               string `json:"status"`
	EncryptionEnabled    bool   `json:"encryption_enabled"`
	EncryptionLocked     bool   `json:"encryption_locked"`
}

type ssoTestRequest struct {
	Issuer string `json:"issuer"`
}

func ssoFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrOIDCInvalid) {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	log.Errorf("sso: failed to save settings: %v", err)
	JSONResponse(w, models.Response{Success: false, Message: "Failed to save SSO settings"}, http.StatusInternalServerError)
}

func writeSSOSettings(w http.ResponseWriter) {
	s, err := models.GetOIDCSettings()
	if err != nil {
		log.Errorf("sso: failed to load settings: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to load SSO settings"}, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	JSONResponse(w, ssoResponse{
		Enabled:              s.Enabled,
		Provider:             s.Provider,
		Issuer:               s.Issuer,
		ClientID:             s.ClientID,
		RedirectURL:          s.RedirectURL,
		RequiredGroup:        s.RequiredGroup,
		GroupsClaim:          s.GroupsClaim,
		UsernameFromEmail:    s.UsernameFromEmail,
		AllowUnverifiedEmail: s.AllowUnverifiedEmail,
		SecretSet:            models.OIDCSecretSet(s),
		Status:               string(models.CurrentOIDCStatus()),
		EncryptionEnabled:    crypto.IsEncryptionEnabled(),
		EncryptionLocked:     models.IsEncryptionLocked(),
	}, http.StatusOK)
}

// SSO returns (GET) or updates (PUT) the SSO settings.
func (as *Server) SSO(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeSSOSettings(w)
	case http.MethodPut:
		in := models.OIDCSettingsInput{}
		if !decodeBody(w, r, &in) {
			return
		}
		if err := models.SaveOIDCSettings(in); err != nil {
			ssoFailure(w, err)
			return
		}
		log.Infof("sso: settings updated by %s", encryptionUser(r).Username)
		writeSSOSettings(w)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// SSOTest checks that an issuer serves OIDC discovery.
func (as *Server) SSOTest(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req := ssoTestRequest{}
	if !decodeBody(w, r, &req) {
		return
	}
	err := auth.CheckIssuer(r.Context(), req.Issuer)
	msg := ""
	switch {
	case err == nil:
		JSONResponse(w, models.Response{Success: true, Message: "Issuer found. Sign in with SSO in a private window to check the full login."}, http.StatusOK)
		return
	case errors.Is(err, auth.ErrIssuerUnreachable):
		msg = "Could not reach the issuer"
	case errors.Is(err, auth.ErrIssuerTLS):
		msg = "TLS error connecting to the issuer"
	default:
		msg = "This URL is not an OIDC issuer"
	}
	JSONResponse(w, models.Response{Success: false, Message: msg}, http.StatusBadRequest)
}
