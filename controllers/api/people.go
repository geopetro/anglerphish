package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

const peopleDateLayout = "2006-01-02"

// parsePeopleFilter builds a PeopleFilter from the shared query parameters.
// "from" is the start of the given day (UTC) and "to" the last instant of it,
// so a date-only "to" covers the whole day.
func parsePeopleFilter(r *http.Request) (models.PeopleFilter, error) {
	f := models.PeopleFilter{}
	q := r.URL.Query()
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(peopleDateLayout, v)
		if err != nil {
			return f, fmt.Errorf("invalid from date")
		}
		f.From = t.UTC()
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(peopleDateLayout, v)
		if err != nil {
			return f, fmt.Errorf("invalid to date")
		}
		f.To = t.UTC().Add(24*time.Hour - time.Nanosecond)
	}
	for name, dst := range map[string]*int64{"campaign_set_id": &f.CampaignSetId, "group_id": &f.GroupId} {
		if v := q.Get(name); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil || id < 0 {
				return f, fmt.Errorf("invalid %s", name)
			}
			*dst = id
		}
	}
	return f, nil
}

// People returns per-person phishing stats.
func (as *Server) People(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	f, err := parsePeopleFilter(r)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	stats, err := models.GetPeopleStats(ctx.Get(r, "user_id").(int64), f)
	if err != nil {
		peopleError(w, err)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// PeopleTimeline returns the campaign history of a single person.
func (as *Server) PeopleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing key"}, http.StatusBadRequest)
		return
	}
	f, err := parsePeopleFilter(r)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	timeline, err := models.GetPersonTimeline(ctx.Get(r, "user_id").(int64), key, f)
	if err != nil {
		peopleError(w, err)
		return
	}
	JSONResponse(w, timeline, http.StatusOK)
}

func peopleError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrPeopleFilterNotFound) {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign set or group not found"}, http.StatusNotFound)
		return
	}
	log.Error(err)
	JSONResponse(w, models.Response{Success: false, Message: "Error getting people"}, http.StatusInternalServerError)
}
