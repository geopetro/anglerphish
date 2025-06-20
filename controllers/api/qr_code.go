package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// QRCodeResponse represents the response sent back to the client
// when a QR code is generated
type QRCodeResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
	QRCodeBase64 string        `json:"qr_code_base64,omitempty"`
	Filename     string        `json:"filename,omitempty"`
	QRCode       models.QRCode `json:"qr_code,omitempty"`
}

// QRCodesResponse represents the response sent back to the client
// when a list of QR codes is requested
type QRCodesResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	QRCodes []models.QRCode `json:"qr_codes"`
}

// Handles requests for the /api/qr_code/ endpoint
func (as *Server) Qr_code(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		as.QRCodes(w, r)
	case r.Method == "POST":
		as.CreateQRCode(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// QRCodes returns a list of all QR codes for the current user
func (as *Server) QRCodes(w http.ResponseWriter, r *http.Request) {
	// Get the current user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Get all QR codes for the current user
	qrcodes, err := models.GetQRCodes(uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Return the QR codes
	response := QRCodesResponse{
		Success: true,
		Message: "QR codes retrieved successfully",
		QRCodes: qrcodes,
	}
	JSONResponse(w, response, http.StatusOK)
}

// CreateQRCode creates a new QR code and optionally saves it to the database
func (as *Server) CreateQRCode(w http.ResponseWriter, r *http.Request) {
	// Check content type for JSON
	contentType := r.Header.Get("Content-Type")
	var qr models.QRCode
	var storeInDb bool

	if contentType == "application/json" {
		// Parse JSON data
		type RequestData struct {
			URL       string `json:"url"`
			Size      string `json:"size"`
			StoreInDb bool   `json:"storeInDb"`
		}
		var req RequestData
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON data: " + err.Error()}, http.StatusBadRequest)
			return
		}
		qr.URL = req.URL
		qr.Size = req.Size
		storeInDb = req.StoreInDb
	} else {
		// Parse form data
		err := r.ParseForm()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid form data"}, http.StatusBadRequest)
			return
		}

		qr.URL = r.FormValue("url")
		qr.Size = r.FormValue("size")
		storeInDb = r.FormValue("storeInDb") == "true"
	}

	// Check if we have the required fields
	if qr.URL == "" || qr.Size == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing required fields: url and size"}, http.StatusBadRequest)
		return
	}

	// Generate the QR code and get the image data
	qrData, filename, err := models.PostQRCode(&qr)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	// Set the filename
	qr.Filename = filename

	// If storeInDb is true, save the QR code to the database
	if storeInDb {
		// Get the current user ID from the context
		uid := ctx.Get(r, "user_id").(int64)

		// Save the QR code to the database
		err = qr.Save(uid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	// Convert the QR code data to base64
	base64QRCode := base64.StdEncoding.EncodeToString(qrData)

	// Return the base64-encoded QR code in the response
	response := QRCodeResponse{
		Success:      true,
		Message:      "QR code generated successfully",
		QRCodeBase64: base64QRCode,
		Filename:     filename,
		QRCode:       qr,
	}

	JSONResponse(w, response, http.StatusOK)
}

// DeleteQRCode deletes a QR code from the database
func (as *Server) DeleteQRCode(w http.ResponseWriter, r *http.Request) {
	// Get the current user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Get the QR code ID from the URL
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid QR code ID"}, http.StatusBadRequest)
		return
	}

	// Get the QR code
	qr, err := models.GetQRCode(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Delete the QR code
	err = qr.Delete()
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Return success
	JSONResponse(w, models.Response{Success: true, Message: "QR code deleted successfully"}, http.StatusOK)
}

// DownloadQRCode generates a QR code from a saved record and returns it for download
func (as *Server) DownloadQRCode(w http.ResponseWriter, r *http.Request) {
	// Get the current user ID from the context
	uid := ctx.Get(r, "user_id").(int64)

	// Get the QR code ID from the URL
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid QR code ID"}, http.StatusBadRequest)
		return
	}

	// Get the QR code
	qr, err := models.GetQRCode(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Generate the QR code and get the image data
	qrData, _, err := models.PostQRCode(&qr)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	// Convert the QR code data to base64
	base64QRCode := base64.StdEncoding.EncodeToString(qrData)

	// Return the base64-encoded QR code in the response
	response := QRCodeResponse{
		Success:      true,
		Message:      "QR code generated successfully",
		QRCodeBase64: base64QRCode,
		Filename:     qr.Filename,
	}

	JSONResponse(w, response, http.StatusOK)
}
