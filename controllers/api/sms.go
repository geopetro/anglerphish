package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// SMSProfiles handles requests for the /api/sms/ endpoint
func (as *Server) SMSProfiles(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ss, err := models.GetSMSs(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ss, http.StatusOK)
	//POST: Create a new SMS profile and return it as JSON
	case r.Method == "POST":
		s := models.SMS{}
		// Put the request into a page
		err := json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// Check to make sure the name is unique
		existingProfile, err := models.GetSMSByName(s.Name, ctx.Get(r, "user_id").(int64))
		if err != nil && err != gorm.ErrRecordNotFound {
			// This is an unexpected error
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error checking SMS profile name"}, http.StatusInternalServerError)
			return
		}

		// If we found a profile with the same name, return a conflict error
		if err == nil && existingProfile.Id != 0 {
			JSONResponse(w, models.Response{Success: false, Message: "SMS profile name already in use"}, http.StatusConflict)
			return
		}
		s.ModifiedDate = time.Now().UTC()
		s.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostSMS(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, s, http.StatusCreated)
	}
}

// SMSProfile contains functions to handle the GET'ing, DELETE'ing, and PUT'ing
// of a SMS profile object
func (as *Server) SMSProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	s, err := models.GetSMS(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMS profile not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, s, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteSMS(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting SMS profile"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "SMS Profile Deleted Successfully"}, http.StatusOK)
	case r.Method == "PUT":
		s = models.SMS{}
		err = json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			log.Error(err)
		}
		if s.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and /:sms_id mismatch"}, http.StatusBadRequest)
			return
		}
		err = s.Validate()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		s.ModifiedDate = time.Now().UTC()
		s.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutSMS(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating SMS profile"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, s, http.StatusOK)
	}
}

// SMSProfileBalance handles requests for the /api/sms/:id/balance endpoint
// It retrieves the current account balance from the SMS provider
func (as *Server) SMSProfileBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	s, err := models.GetSMS(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMS profile not found"}, http.StatusNotFound)
		return
	}

	// Get balance from the provider
	balance, err := mailer.GetSMSBalance(s.Provider, s.ProviderConfig)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error fetching balance: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, balance, http.StatusOK)
}
