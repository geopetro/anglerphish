package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// bulkDeleteRequest is used to parse the JSON body for bulk campaign deletion
type bulkDeleteRequest struct {
	Ids []int64 `json:"ids"`
}

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//DELETE: Bulk delete campaigns
	case r.Method == "DELETE":
		var req bulkDeleteRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if len(req.Ids) == 0 {
			JSONResponse(w, models.Response{Success: false, Message: "No campaign IDs provided"}, http.StatusBadRequest)
			return
		}
		count, err := models.DeleteCampaigns(req.Ids, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaigns"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: strconv.Itoa(count) + " campaign(s) deleted successfully!"}, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time.
		// Note: Generic campaigns don't need the worker - they don't send emails/SMS.
		if c.Status == models.CampaignInProgress && c.Type != "generic" {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}

// linkRequest is used to parse the JSON body for creating a new link
type linkRequest struct {
	Name string `json:"name"`
}

// CampaignLinks handles generating new tracking links for generic campaigns.
// POST creates a new tracking link and returns the result with the URL.
func (as *Server) CampaignLinks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)

	switch {
	case r.Method == "POST":
		// Parse the optional link name from request body
		var req linkRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil && err.Error() != "EOF" {
			// Allow empty body for backward compatibility
			log.Warn("Error decoding link request body: ", err)
		}

		// Generate a new tracking link for this generic campaign
		result, err := models.GenerateCampaignLink(id, uid, req.Name)
		if err != nil {
			if err == models.ErrCampaignNotGeneric {
				JSONResponse(w, models.Response{Success: false, Message: "This operation is only available for generic campaigns"}, http.StatusBadRequest)
				return
			}
			if err == models.ErrCampaignCompleted {
				JSONResponse(w, models.Response{Success: false, Message: "Cannot generate links for a completed campaign"}, http.StatusBadRequest)
				return
			}
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error generating campaign link"}, http.StatusInternalServerError)
			return
		}

		// Get the campaign to include the URL in response
		c, err := models.GetCampaign(id, uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error retrieving campaign"}, http.StatusInternalServerError)
			return
		}

		// Build the full tracking URL
		urlParam := c.URLParam
		if urlParam == "" {
			urlParam = "rid"
		}
		trackingURL := c.URL + "?" + urlParam + "=" + result.RId

		// Return the result with the tracking URL
		response := struct {
			*models.Result
			TrackingURL string `json:"tracking_url"`
		}{
			Result:      result,
			TrackingURL: trackingURL,
		}

		JSONResponse(w, response, http.StatusCreated)
	}
}

// CampaignResendFailed re-queues all errored sends for a campaign.
func (as *Server) CampaignResendFailed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "POST":
		c, err := models.GetCampaign(id, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		var count int
		if c.Type == "sms" {
			count, err = models.ResendFailedSMSInCampaign(id, uid)
		} else {
			count, err = models.ResendFailedEmailsInCampaign(id, uid)
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		noun := "email(s)"
		if c.Type == "sms" {
			noun = "message(s)"
		}
		msg := strconv.Itoa(count) + " " + noun + " queued for resend"
		if count == 0 {
			msg = "No failed sends found"
		}
		JSONResponse(w, models.Response{Success: true, Message: msg}, http.StatusOK)
	}
}

// CampaignResultResend re-queues a single failed send by result ID.
func (as *Server) CampaignResultResend(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	rid := vars["rid"]
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "POST":
		c, err := models.GetCampaign(id, uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		if c.Type == "sms" {
			err = models.ResendFailedSMS(id, rid, uid)
		} else {
			err = models.ResendFailedEmail(id, rid, uid)
		}
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Result not found"}, http.StatusNotFound)
				return
			}
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		msg := "Email queued for resend"
		if c.Type == "sms" {
			msg = "Message queued for resend"
		}
		JSONResponse(w, models.Response{Success: true, Message: msg}, http.StatusOK)
	}
}
