package controllers

import (
	"net/http"
)

// NonCampaignReports handles the non-campaign reports page
func (as *AdminServer) NonCampaignReports(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Non-Campaign Reports"
	getTemplate(w, "non_campaign_reports").ExecuteTemplate(w, "base", params)
}
