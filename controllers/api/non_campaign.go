package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
)

// BulkDeleteNonCampaignRequest represents a request to delete multiple non-campaign reports
type BulkDeleteNonCampaignRequest struct {
	Ids []int64 `json:"ids"`
}

// NonCampaignReportsEndpoint handles the API endpoint for non-campaign reports
func (as *Server) NonCampaignReportsEndpoint(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		as.getNonCampaignReports(w, r)
	case r.Method == "DELETE":
		as.clearNonCampaignReports(w, r)
	case r.Method == "POST":
		as.bulkDeleteNonCampaignReports(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// bulkDeleteNonCampaignReports handles the API endpoint for deleting selected non-campaign reports
func (as *Server) bulkDeleteNonCampaignReports(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Parse the request body
	var req BulkDeleteNonCampaignRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
		return
	}

	// Validate that we have IDs to delete
	if len(req.Ids) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No report IDs provided"}, http.StatusBadRequest)
		return
	}

	// Delete the reports
	err = models.DeleteNonCampaignReportsByIds(uid, req.Ids)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error deleting non-campaign reports"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "Selected reports deleted successfully"}, http.StatusOK)
}

// clearNonCampaignReports handles the API endpoint for clearing non-campaign reports
func (as *Server) clearNonCampaignReports(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Check if we have an IMAP ID filter
	var imapId int64 = 0
	if imapIDParam := r.URL.Query().Get("imap_id"); imapIDParam != "" {
		// Try to parse the string to int64
		if parsedID, err := strconv.ParseInt(imapIDParam, 10, 64); err == nil {
			imapId = parsedID
		}
	}

	// Clear the reports
	err := models.ClearNonCampaignReports(uid, imapId)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error clearing non-campaign reports"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "Non-campaign reports cleared successfully"}, http.StatusOK)
}

// getNonCampaignReports returns the non-campaign reports for the current user
func (as *Server) getNonCampaignReports(w http.ResponseWriter, r *http.Request) {
	// Get the user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Check if we have an IMAP ID filter
	var imapId int64 = 0
	if imapIDParam := r.URL.Query().Get("imap_id"); imapIDParam != "" {
		// Try to parse the string to int64
		if parsedID, err := strconv.ParseInt(imapIDParam, 10, 64); err == nil {
			imapId = parsedID
		}
	}

	// Get the stats
	stats, err := models.GetNonCampaignStats(uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error getting non-campaign stats"}, http.StatusInternalServerError)
		return
	}

	// Get the recent reports (limited to 100)
	reports, err := models.GetRecentNonCampaignReports(uid, 100, imapId)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error getting non-campaign reports"}, http.StatusInternalServerError)
		return
	}

	// Get list of IMAP configurations that have reports
	imaps, err := models.GetImapConfigsWithReports(uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error getting IMAP configurations"}, http.StatusInternalServerError)
		return
	}

	// Create a response object
	response := struct {
		Stats          models.NonCampaignStats    `json:"stats"`
		Reports        []models.NonCampaignReport `json:"reports"`
		ImapConfigs    []models.IMAP              `json:"imap_configs"`
		SelectedImapId int64                      `json:"selected_imap_id"`
	}{
		Stats:          stats,
		Reports:        reports,
		ImapConfigs:    imaps,
		SelectedImapId: imapId,
	}

	JSONResponse(w, response, http.StatusOK)
}
