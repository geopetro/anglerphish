package api

import (
	"encoding/json"
	"net/http"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
)

// GlobalVariables handles GET and PUT requests for per-user global template
// variable overrides.
func (as *Server) GlobalVariables(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch r.Method {
	case "GET":
		gv, err := models.GetGlobalVariables(uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, gv, http.StatusOK)

	case "PUT":
		gv := &models.GlobalVariables{}
		if err := json.NewDecoder(r.Body).Decode(gv); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err := models.PutGlobalVariables(gv, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Global variables updated successfully"}, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusMethodNotAllowed)}, http.StatusMethodNotAllowed)
	}
}
