package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gophish/gophish/models"
)

// BulkDeleteTrackedRequest represents a request to delete multiple tracked requests.
type BulkDeleteTrackedRequest struct {
	Ids []int64 `json:"ids"`
}

// TrackedRequestsEndpoint handles listing, clearing and bulk-deleting tracked
// requests (X-Tracked callbacks that could not be attributed to a result).
//
// Tracked requests carry no recipient id and therefore no user/campaign scope,
// so they are global and visible to any administrator.
func (as *Server) TrackedRequestsEndpoint(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		as.getTrackedRequests(w, r)
	case http.MethodDelete:
		as.clearTrackedRequests(w, r)
	case http.MethodPost:
		as.bulkDeleteTrackedRequests(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

func (as *Server) getTrackedRequests(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	reqs, err := models.GetTrackedRequests(limit)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error getting tracked requests"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, reqs, http.StatusOK)
}

func (as *Server) clearTrackedRequests(w http.ResponseWriter, r *http.Request) {
	if err := models.ClearTrackedRequests(); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error clearing tracked requests"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Tracked requests cleared successfully"}, http.StatusOK)
}

func (as *Server) bulkDeleteTrackedRequests(w http.ResponseWriter, r *http.Request) {
	var req BulkDeleteTrackedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
		return
	}
	if len(req.Ids) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No request IDs provided"}, http.StatusBadRequest)
		return
	}
	if err := models.DeleteTrackedRequestsByIds(req.Ids); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error deleting tracked requests"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Selected tracked requests deleted successfully"}, http.StatusOK)
}

// TrackedRequestsExport streams the tracked requests as a CSV download. This
// replaces the file the server used to write on disk; the data now lives in the
// database and is exported on demand.
func (as *Server) TrackedRequestsExport(w http.ResponseWriter, r *http.Request) {
	reqs, err := models.GetTrackedRequests(0)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error getting tracked requests"}, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"tracked-requests.csv\"")
	cw := csv.NewWriter(w)
	cw.Write([]string{"created_at", "source_ip", "method", "path", "raw_query", "user_agent"})
	for _, req := range reqs {
		cw.Write([]string{
			req.CreatedAt.Format(time.RFC3339),
			req.SourceIP,
			req.Method,
			req.Path,
			req.RawQuery,
			req.UserAgent,
		})
	}
	cw.Flush()
}
