package api

import (
	"errors"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/imap"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
)

// NonCampaignReportMessage fetches the content of a single non-campaign report
// on demand from the IMAP server.
//
// This endpoint returns JSON only. The message HTML is carried as a JSON string
// and is never served with a text/html content type — rendering it same-origin
// would put attacker-controlled script inside the admin session.
func (as *Server) NonCampaignReportMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid report id"}, http.StatusBadRequest)
		return
	}

	report, err := models.GetNonCampaignReport(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Report not found"}, http.StatusNotFound)
		return
	}

	if report.ImapUid == 0 && report.MessageId == "" {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Message content is not available for reports created before this feature was added.",
		}, http.StatusGone)
		return
	}

	im, err := models.GetIMAPById(report.ImapId, uid)
	if err != nil {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "The IMAP configuration for this report no longer exists.",
		}, http.StatusGone)
		return
	}

	mailbox := imap.Mailbox{
		Host:             im.Host + ":" + strconv.Itoa(int(im.Port)),
		TLS:              im.TLS,
		IgnoreCertErrors: im.IgnoreCertErrors,
		User:             im.Username,
		Pwd:              im.Password,
		Folder:           im.Folder,
		ReadOnly:         true,
	}

	message, err := mailbox.FetchMessage(uint32(report.ImapUid), uint32(report.ImapUidValidity), report.MessageId)
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case errors.Is(err, imap.ErrMessageNotFound):
			status = http.StatusGone
		case errors.Is(err, imap.ErrMessageAmbiguous), errors.Is(err, imap.ErrMailboxRecreated):
			status = http.StatusConflict
		}
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, status)
		return
	}

	JSONResponse(w, message, http.StatusOK)
}
