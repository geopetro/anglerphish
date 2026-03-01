package models

import (
	"bytes"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"text/template"
	"time"
)

// TemplateContext is an interface that allows both campaigns and email
// requests to have a PhishingTemplateContext generated for them.
type TemplateContext interface {
	getFromAddress() string
	getBaseURL() string
	getQRSize() string
}

// PhishingTemplateContext is the context that is sent to any template, such
// as the email or landing page content.
type PhishingTemplateContext struct {
	From            string
	URL             string
	Tracker         string
	TrackingURL     string
	RId             string
	BaseURL         string
	QRBase64        string
	QRName          string
	QR              string
	QRSize          string // QR code size in pixels
	QRImageData     []byte // Raw PNG data for Office document embedding
	QRFallbackText  string // URL text for fallbacks in Office documents
	CurrentDateTime string // Current date and time - e.g. "Nov 23, 2025 7:39 PM"
	CurrentDate     string // Current date only - e.g. "November 23, 2025"
	CurrentTime     string // Current time (12-hour) - e.g. "7:39 PM"
	CurrentTime24   string // Current time (24-hour) - e.g. "19:39"
	BaseRecipient
}

// NewPhishingTemplateContext returns a populated PhishingTemplateContext,
// parsing the correct fields from the provided TemplateContext and recipient.
func NewPhishingTemplateContext(ctx TemplateContext, r BaseRecipient, rid string) (PhishingTemplateContext, error) {
	f, err := mail.ParseAddress(ctx.getFromAddress())
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	fn := f.Name
	if fn == "" {
		fn = f.Address
	}
	templateURL, err := ExecuteTemplate(ctx.getBaseURL(), r)
	if err != nil {
		return PhishingTemplateContext{}, err
	}

	// For the base URL, we'll reset the the path and the query
	// This will create a URL in the form of http://example.com
	baseURL, err := url.Parse(templateURL)
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""

	phishURL, _ := url.Parse(templateURL)
	q := phishURL.Query()
	// q.Set(RecipientParameter, rid)
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

	// Prepare QR code
	qrBase64 := ""
	qrName := ""
	qr := ""
	var qrImageData []byte
	qrFallbackText := phishURL.String() // Use the phishing URL as fallback text
	qrSize := ctx.getQRSize()
	if qrSize != "" {
		qrBase64, qrName, err = generateQRCode(phishURL.String(), qrSize)
		if err != nil {
			return PhishingTemplateContext{}, err
		}
		qr = "<img src=\"cid:" + qrName + "\">"

		// Generate raw image data for Office documents
		qrImageData, err = generateQRImageData(phishURL.String(), qrSize)
		if err != nil {
			// If we can't generate image data, we'll fall back to text
			qrImageData = nil
		}
	}

	// Get current time for template variables
	now := time.Now()

	return PhishingTemplateContext{
		BaseRecipient:   r,
		BaseURL:         baseURL.String(),
		URL:             phishURL.String(),
		TrackingURL:     trackingURL.String(),
		Tracker:         "<img alt='' style='display: none' src='" + trackingURL.String() + "'/>",
		From:            fn,
		RId:             rid,
		QRBase64:        qrBase64,
		QRName:          qrName,
		QR:              qr,
		QRSize:          qrSize,
		QRImageData:     qrImageData,
		QRFallbackText:  qrFallbackText,
		CurrentDateTime: now.Format("Jan 2, 2006 3:04 PM"),
		CurrentDate:     now.Format("January 2, 2006"),
		CurrentTime:     now.Format("3:04 PM"),
		CurrentTime24:   now.Format("15:04"),
	}, nil
}

// ExecuteTemplate creates a templated string based on the provided
// template body and data.
func ExecuteTemplate(text string, data interface{}) (string, error) {
	buff := bytes.Buffer{}
	tmpl, err := template.New("template").Parse(text)
	if err != nil {
		return buff.String(), err
	}
	err = tmpl.Execute(&buff, data)
	return buff.String(), err
}

// An upgraded ExecuteTemplate specifically for the Attachments
// this was created to handle the issue of replacing {{.URL}} and {{.TrackingURL}} placeholders.
// More specifically, if the link contains ampersand (&) symbol, it leads to corrupted documents as it is an XML reserved symbol.
// This function tackles the issue by replacing & with &amp;
func ExecuteAttachmentsTemplate(text string, data PhishingTemplateContext) (string, error) {
	buff := bytes.Buffer{}
	tmpl, err := template.New("template").Parse(text)
	if err != nil {
		return buff.String(), err
	}

	// data.URL = trimQueryContent(data.URL)
	// data.TrackingURL = trimQueryContent(data.TrackingURL)
	data.URL = strings.ReplaceAll(data.URL, "&", "&amp;")
	data.TrackingURL = strings.ReplaceAll(data.TrackingURL, "&", "&amp;")

	// For Office documents, the QR field should be handled by the attachment processor
	// Don't override it here as it may contain proper image XML or fallback text

	err = tmpl.Execute(&buff, data)
	return buff.String(), err
}

// ValidationContext is used for validating templates and pages
type ValidationContext struct {
	FromAddress string
	BaseURL     string
	QRSize      string
}

func (vc ValidationContext) getFromAddress() string {
	return vc.FromAddress
}

func (vc ValidationContext) getBaseURL() string {
	return vc.BaseURL
}

func (vc ValidationContext) getQRSize() string {
	return vc.QRSize
}

// ValidateTemplate ensures that the provided text in the page or template
// uses the supported template variables correctly.
func ValidateTemplate(text string) error {
	vc := ValidationContext{
		FromAddress: "foo@bar.com",
		BaseURL:     "http://example.com",
	}
	td := Result{
		BaseRecipient: BaseRecipient{
			Email:     "foo@bar.com",
			Phone:     "+15551234567",
			FirstName: "Foo",
			LastName:  "Bar",
			Position:  "Test",
			Custom:    "CustomValue",
		},
		RId: "123456",
	}
	ptx, err := NewPhishingTemplateContext(vc, td.BaseRecipient, td.RId)
	if err != nil {
		return err
	}
	_, err = ExecuteTemplate(text, ptx)
	if err != nil {
		return err
	}
	return nil
}
