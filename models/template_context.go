package models

import (
	"bytes"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"text/template"
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
	From        string
	URL         string
	Tracker     string
	TrackingURL string
	RId         string
	BaseURL     string
	QRBase64    string
	QRName      string
	QR          string
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
	qrSize := ctx.getQRSize()
	if qrSize != "" {
		qrBase64, qrName, err = generateQRCode(phishURL.String(), qrSize)
		if err != nil {
			return PhishingTemplateContext{}, err
		}
		qr = "<img src=\"cid:" + qrName + "\">"
	}

	return PhishingTemplateContext{
		BaseRecipient: r,
		BaseURL:       baseURL.String(),
		URL:           phishURL.String(),
		TrackingURL:   trackingURL.String(),
		Tracker:       "<img alt='' style='display: none' src='" + trackingURL.String() + "'/>",
		From:          fn,
		RId:           rid,
		QRBase64:      qrBase64,
		QRName:        qrName,
		QR:            qr,
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
