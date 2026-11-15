package api

import (
	"errors"
	"net/http"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

// reportSettingsInput is the submitted form of the report settings.
type reportSettingsInput struct {
	OnlineGeolocation bool   `json:"online_geolocation"`
	GeoIPDatabasePath string `json:"geoip_db_path"`
	JobTimeoutMinutes int    `json:"job_timeout_minutes"`
	RetentionDays     int    `json:"retention_days"`
}

// reportSettingsResponse is the API view of the report settings.
type reportSettingsResponse struct {
	reportSettingsInput
	MaxJobTimeoutMinutes int `json:"max_job_timeout_minutes"`
	MaxRetentionDays     int `json:"max_retention_days"`
}

func writeReportSettings(w http.ResponseWriter) {
	rs, err := models.GetReportSettings()
	if err != nil {
		log.Errorf("reports: failed to load settings: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Failed to load report settings"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, reportSettingsResponse{
		reportSettingsInput: reportSettingsInput{
			OnlineGeolocation: rs.OnlineGeolocation,
			GeoIPDatabasePath: rs.GeoIPDatabasePath,
			JobTimeoutMinutes: rs.JobTimeoutMinutes,
			RetentionDays:     rs.RetentionDays,
		},
		MaxJobTimeoutMinutes: models.MaxReportJobTimeoutMinutes,
		MaxRetentionDays:     models.MaxReportRetentionDays,
	}, http.StatusOK)
}

// ReportSettings returns (GET) or updates (PUT) the report generation settings.
func (as *Server) ReportSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeReportSettings(w)
	case http.MethodPut:
		in := reportSettingsInput{}
		if !decodeBody(w, r, &in) {
			return
		}
		err := models.SaveReportSettings(models.ReportSettings{
			OnlineGeolocation: in.OnlineGeolocation,
			GeoIPDatabasePath: in.GeoIPDatabasePath,
			JobTimeoutMinutes: in.JobTimeoutMinutes,
			RetentionDays:     in.RetentionDays,
		})
		if errors.Is(err, models.ErrReportSettingsInvalid) {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Errorf("reports: failed to save settings: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: "Failed to save report settings"}, http.StatusInternalServerError)
			return
		}
		log.Infof("reports: settings updated by %s (online geolocation: %t)", encryptionUser(r).Username, in.OnlineGeolocation)
		writeReportSettings(w)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}
