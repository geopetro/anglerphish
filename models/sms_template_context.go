package models

import (
	"net/url"
	"path"
	"time"
)

// SMSTemplateContext is the context that is sent to any SMS template.
type SMSTemplateContext struct {
	From            string
	URL             string
	TrackingURL     string
	RId             string
	BaseURL         string
	CurrentDateTime string // Current date and time - e.g. "Nov 23, 2025 7:39 PM"
	CurrentDate     string // Current date only - e.g. "November 23, 2025"
	CurrentTime     string // Current time (12-hour) - e.g. "7:39 PM"
	CurrentTime24   string // Current time (24-hour) - e.g. "19:39"
	BaseRecipient
}

// NewSMSTemplateContext returns a populated SMSTemplateContext,
// parsing the correct fields from the provided TemplateContext and recipient.
func NewSMSTemplateContext(ctx TemplateContext, r BaseRecipient, rid string) (SMSTemplateContext, error) {
	// For SMS, we don't need to parse the from address as an email
	from := ctx.getFromAddress()

	// Create a temporary context with RId for template execution
	tempContext := struct {
		BaseRecipient
		RId string
	}{
		BaseRecipient: r,
		RId:           rid,
	}

	templateURL, err := ExecuteTemplate(ctx.getBaseURL(), tempContext)
	if err != nil {
		return SMSTemplateContext{}, err
	}

	// For the base URL, we'll reset the the path and the query
	// This will create a URL in the form of http://example.com
	baseURL, err := url.Parse(templateURL)
	if err != nil {
		return SMSTemplateContext{}, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""

	phishURL, _ := url.Parse(templateURL)
	q := phishURL.Query()
	encodedQuery := q.Encode()
	if encodedQuery == "" {
		encodedQuery = RecipientParameter + "=" + rid
	} else {
		encodedQuery += "&" + RecipientParameter + "=" + rid
	}
	phishURL.RawQuery = encodedQuery

	trackingURL, _ := url.Parse(templateURL)
	trackingURL.Path = path.Join(trackingURL.Path, "/track")
	trackingURL.RawQuery = encodedQuery

	// Get current time for template variables
	now := time.Now()

	return SMSTemplateContext{
		BaseRecipient:   r,
		BaseURL:         baseURL.String(),
		URL:             phishURL.String(),
		TrackingURL:     trackingURL.String(),
		From:            from,
		RId:             rid,
		CurrentDateTime: now.Format("Jan 2, 2006 3:04 PM"),
		CurrentDate:     now.Format("January 2, 2006"),
		CurrentTime:     now.Format("3:04 PM"),
		CurrentTime24:   now.Format("15:04"),
	}, nil
}
