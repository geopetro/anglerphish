package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// URLTemplates handles requests for the /api/url_templates/ endpoint
func (as *Server) URLTemplates(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ts, err := models.GetURLTemplates(ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, ts, http.StatusOK)
	case r.Method == "POST":
		t := models.URLTemplate{}
		err := json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// Validate required fields
		if t.Name == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Template name is required"}, http.StatusBadRequest)
			return
		}
		if t.URL == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Template URL is required"}, http.StatusBadRequest)
			return
		}
		// Set the user ID from context
		t.UserId = ctx.Get(r, "user_id").(int64)
		// Set default category if not provided
		if t.Category == "" {
			t.Category = "Custom"
		}
		err = models.PostURLTemplate(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, t, http.StatusCreated)
	}
}

// URLTemplate handles requests for the /api/url_templates/:id endpoint
func (as *Server) URLTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	switch {
	case r.Method == "GET":
		t, err := models.GetURLTemplate(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Template not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, t, http.StatusOK)
	case r.Method == "PUT":
		t := models.URLTemplate{}
		err := json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// Validate required fields
		if t.Name == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Template name is required"}, http.StatusBadRequest)
			return
		}
		if t.URL == "" {
			JSONResponse(w, models.Response{Success: false, Message: "Template URL is required"}, http.StatusBadRequest)
			return
		}
		t.Id = id
		t.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutURLTemplate(&t, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, t, http.StatusOK)
	case r.Method == "DELETE":
		err := models.DeleteURLTemplate(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "URL template deleted successfully"}, http.StatusOK)
	}
}
