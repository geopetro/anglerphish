package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// DraftCampaignSets returns all draft campaign sets for the current user
func (as *Server) DraftCampaignSets(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		dcs, err := models.GetDraftCampaignSets(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error getting draft campaign sets"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, dcs, http.StatusOK)
	case r.Method == "POST":
		dcs := models.DraftCampaignSet{}
		err := json.NewDecoder(r.Body).Decode(&dcs)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		dcs.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostDraftCampaignSet(&dcs, dcs.UserId)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, dcs, http.StatusCreated)
	}
}

// DraftCampaignSet returns a single draft campaign set by ID
func (as *Server) DraftCampaignSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)

	switch {
	case r.Method == "GET":
		dcs, err := models.GetDraftCampaignSet(id, uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Draft campaign set not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, dcs, http.StatusOK)
	case r.Method == "PUT":
		dcs := models.DraftCampaignSet{}
		err := json.NewDecoder(r.Body).Decode(&dcs)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		dcs.Id = id
		dcs.UserId = uid
		dcs.ModifiedDate = time.Now().UTC()
		err = models.UpdateDraftCampaignSet(&dcs, uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, dcs, http.StatusOK)
	case r.Method == "DELETE":
		err := models.DeleteDraftCampaignSet(id, uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Draft campaign set deleted successfully!"}, http.StatusOK)
	}
}

// LaunchDraftCampaignSet launches a draft campaign set, converting it to a real campaign set
func (as *Server) LaunchDraftCampaignSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)

	cs, err := models.LaunchDraftCampaignSet(id, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	// If any campaign in the set is scheduled to launch immediately, send it to the worker.
	// Otherwise, the worker will pick it up at the scheduled time
	if cs.Status == models.CampaignInProgress {
		for _, campaign := range cs.Campaigns {
			if campaign.Status == models.CampaignInProgress {
				log.Infof("Launching campaign %d immediately", campaign.Id)
				go as.worker.LaunchCampaign(campaign)
			}
		}
	}

	JSONResponse(w, cs, http.StatusOK)
}
