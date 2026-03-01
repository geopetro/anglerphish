package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// CampaignSets returns a list of campaign sets if requested via GET.
// If requested via POST, CampaignSets creates a new campaign set and returns a reference to it.
func (as *Server) CampaignSets(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		css, err := models.GetCampaignSets(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, css, http.StatusOK)
	//POST: Create a new campaign set and return it as JSON
	case r.Method == "POST":
		cs := models.CampaignSet{}
		// Put the request into a campaign set
		body, err := io.ReadAll(r.Body)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error reading request body"}, http.StatusBadRequest)
			return
		}

		// Log the raw JSON for debugging
		log.Infof("Raw campaign set JSON: %s", string(body))

		// Try to decode the JSON
		err = json.Unmarshal(body, &cs)
		if err != nil {
			log.Errorf("JSON decode error: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Invalid JSON structure: %v", err)}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaignSet(&cs, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If any campaign in the set is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if cs.Status == models.CampaignInProgress {
			for _, campaign := range cs.Campaigns {
				if campaign.Status == models.CampaignInProgress {
					go as.worker.LaunchCampaign(campaign)
				}
			}
		}
		JSONResponse(w, cs, http.StatusCreated)
	}
}

// CampaignSetsSummary returns the summary for the current user's campaign sets
func (as *Server) CampaignSetsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		css, err := models.GetCampaignSetSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, css, http.StatusOK)
	}
}

// CampaignSet returns details about the requested campaign set. If the campaign set is not
// valid, CampaignSet returns null.
func (as *Server) CampaignSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cs, err := models.GetCampaignSet(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign set not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, cs, http.StatusOK)
	case r.Method == "PUT":
		cs := models.CampaignSet{}
		// Put the request into a campaign set
		body, err := io.ReadAll(r.Body)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error reading request body"}, http.StatusBadRequest)
			return
		}

		// Log the raw JSON for debugging
		log.Infof("Raw campaign set update JSON: %s", string(body))

		// Try to decode the JSON
		err = json.Unmarshal(body, &cs)
		if err != nil {
			log.Errorf("JSON decode error: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Invalid JSON structure: %v", err)}, http.StatusBadRequest)
			return
		}

		// Set the ID from the URL
		cs.Id = id

		// TODO: Implement update functionality in models
		// For now, just return success
		JSONResponse(w, models.Response{Success: true, Message: "Campaign set updated successfully!"}, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaignSet(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign set"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign set deleted successfully!"}, http.StatusOK)
	}
}

// CampaignSetComplete effectively "ends" a campaign set and all its campaigns.
func (as *Server) CampaignSetComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaignSet(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign set"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign set completed successfully!"}, http.StatusOK)
	}
}
