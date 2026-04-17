package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gophish/gophish/models"
)

const errorPagePath = "templates/404.html"

// defaultNotFoundHTML is the fallback content if the file is reset to default
const defaultNotFoundHTML = `<!DOCTYPE html>
<html>
<head>
    <title>404 Not Found</title>
</head>
<body>
    <h1>404 Not Found</h1>
    <p>The requested URL was not found on this server.</p>
    <hr>
</body>
</html>`

// errorPageRequest represents the payload for updating an error page
type errorPageRequest struct {
	HTML string `json:"html"`
}

// ErrorPage handles GET and PUT requests for the 404 error page content.
// Only users with the ModifySystem permission may use this endpoint.
func (as *Server) ErrorPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		content, err := os.ReadFile(errorPagePath)
		if err != nil {
			// If the file doesn't exist return the default
			JSONResponse(w, map[string]string{"html": defaultNotFoundHTML}, http.StatusOK)
			return
		}
		JSONResponse(w, map[string]string{"html": string(content)}, http.StatusOK)

	case "PUT":
		req := &errorPageRequest{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		html := strings.TrimSpace(req.HTML)
		if html == "" {
			JSONResponse(w, models.Response{Success: false, Message: "HTML content cannot be empty"}, http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(errorPagePath, []byte(html), 0644); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Failed to save 404 page: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "404 page updated successfully"}, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusMethodNotAllowed)}, http.StatusMethodNotAllowed)
	}
}

// ErrorPageReset resets the 404 error page back to the default content.
// Only users with the ModifySystem permission may use this endpoint.
func (as *Server) ErrorPageReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusMethodNotAllowed)}, http.StatusMethodNotAllowed)
		return
	}
	if err := os.WriteFile(errorPagePath, []byte(defaultNotFoundHTML), 0644); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Failed to reset 404 page: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "404 page reset to default successfully",
		"html":    defaultNotFoundHTML,
	}, http.StatusOK)
}
