package dmi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"path"
	"strings"
	"time"
)

// Base64 budget for attachments sent inline with the create request.
const maxInlineGraphBytes = 3 * 1024 * 1024

type graphEmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type graphRecipient struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type graphFileAttachment struct {
	ODataType    string `json:"@odata.type"`
	Name         string `json:"name"`
	ContentType  string `json:"contentType"`
	ContentBytes string `json:"contentBytes"`
	IsInline     bool   `json:"isInline"`
	ContentID    string `json:"contentId,omitempty"`
}

type graphExtendedProperty struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type graphMessage struct {
	Subject                       string                  `json:"subject"`
	Body                          graphBody               `json:"body"`
	From                          graphRecipient          `json:"from"`
	Sender                        graphRecipient          `json:"sender"`
	ToRecipients                  []graphRecipient        `json:"toRecipients"`
	CcRecipients                  []graphRecipient        `json:"ccRecipients,omitempty"`
	ReplyTo                       []graphRecipient        `json:"replyTo,omitempty"`
	IsRead                        bool                    `json:"isRead"`
	Attachments                   []graphFileAttachment   `json:"attachments,omitempty"`
	SingleValueExtendedProperties []graphExtendedProperty `json:"singleValueExtendedProperties"`
}

type graphAttachment struct {
	Name        string
	ContentType string
	ContentID   string
	IsInline    bool
	Content     []byte
}

type mimeContent struct {
	html, text  []byte
	hasHTML     bool
	hasText     bool
	attachments []graphAttachment
}

func permanent(format string, args ...any) error {
	return &Error{Err: fmt.Errorf(format, args...)}
}

func graphRecipients(list []*mail.Address) []graphRecipient {
	out := make([]graphRecipient, 0, len(list))
	for _, a := range list {
		out = append(out, graphRecipient{EmailAddress: graphEmailAddress{Name: a.Name, Address: a.Address}})
	}
	return out
}

func convertToGraph(raw []byte, now time.Time) (graphMessage, []graphAttachment, error) {
	var msg graphMessage

	hdrEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if hdrEnd < 0 {
		return msg, nil, permanent("malformed message: no header terminator")
	}
	headerBlock := string(raw[:hdrEnd])

	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return msg, nil, permanent("malformed message: %v", err)
	}
	h := parsed.Header

	from, err := mail.ParseAddress(h.Get("From"))
	if err != nil {
		return msg, nil, permanent("invalid From header: %v", err)
	}
	to, err := mail.ParseAddressList(h.Get("To"))
	if err != nil {
		return msg, nil, permanent("invalid To header: %v", err)
	}
	var cc, replyTo []*mail.Address
	if v := h.Get("Cc"); v != "" {
		if cc, err = mail.ParseAddressList(v); err != nil {
			return msg, nil, permanent("invalid Cc header: %v", err)
		}
	}
	if v := h.Get("Reply-To"); v != "" {
		if replyTo, err = mail.ParseAddressList(v); err != nil {
			return msg, nil, permanent("invalid Reply-To header: %v", err)
		}
	}

	subject := h.Get("Subject")
	if dec, err := new(mime.WordDecoder).DecodeHeader(subject); err == nil {
		subject = dec
	}

	var c mimeContent
	if err := walkPart(h, parsed.Body, &c); err != nil {
		return msg, nil, permanent("malformed MIME body: %v", err)
	}

	msg.Subject = subject
	switch {
	case c.hasHTML:
		msg.Body = graphBody{ContentType: "html", Content: string(c.html)}
	default:
		msg.Body = graphBody{ContentType: "text", Content: string(c.text)}
	}
	msg.From = graphRecipient{EmailAddress: graphEmailAddress{Name: from.Name, Address: from.Address}}
	msg.Sender = msg.From
	msg.ToRecipients = graphRecipients(to)
	// Cc is display only, the message is created in the target mailbox alone.
	msg.CcRecipients = graphRecipients(cc)
	msg.ReplyTo = graphRecipients(replyTo)

	ts := now.UTC().Format(time.RFC3339)
	msg.SingleValueExtendedProperties = []graphExtendedProperty{
		// UNVERIFIED: 0 (not unsent) is assumed to make the message appear as received, pending a real tenant check.
		{ID: "Integer 0x0E07", Value: "0"},
		{ID: "String 0x007D", Value: headerBlock},
		{ID: "String 0x1035", Value: h.Get("Message-Id")},
		{ID: "SystemTime 0x0E06", Value: ts},
		{ID: "SystemTime 0x0039", Value: ts},
	}

	ordered := make([]graphAttachment, 0, len(c.attachments))
	for _, a := range c.attachments {
		if a.IsInline {
			ordered = append(ordered, a)
		}
	}
	for _, a := range c.attachments {
		if !a.IsInline {
			ordered = append(ordered, a)
		}
	}

	var large []graphAttachment
	total := 0
	overflow := false
	for _, a := range ordered {
		size := base64.StdEncoding.EncodedLen(len(a.Content))
		if overflow || total+size > maxInlineGraphBytes {
			overflow = true
			large = append(large, a)
			continue
		}
		total += size
		msg.Attachments = append(msg.Attachments, graphFileAttachment{
			ODataType:    "#microsoft.graph.fileAttachment",
			Name:         a.Name,
			ContentType:  a.ContentType,
			ContentBytes: base64.StdEncoding.EncodeToString(a.Content),
			IsInline:     a.IsInline,
			ContentID:    a.ContentID,
		})
	}
	return msg, large, nil
}

func headerGet(h map[string][]string, key string) string {
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func decodeTransfer(cte string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	}
	return r
}

func walkPart(h map[string][]string, body io.Reader, c *mimeContent) error {
	ctHeader := headerGet(h, "Content-Type")
	if ctHeader == "" {
		ctHeader = "text/plain"
	}
	ct, params, err := mime.ParseMediaType(ctHeader)
	if err != nil {
		return err
	}

	if strings.HasPrefix(ct, "multipart/") {
		if params["boundary"] == "" {
			return errors.New("multipart without boundary")
		}
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			// multipart.Part already decodes quoted-printable and drops that header.
			if err := walkPart(p.Header, p, c); err != nil {
				return err
			}
		}
	}

	data, err := io.ReadAll(decodeTransfer(headerGet(h, "Content-Transfer-Encoding"), body))
	if err != nil {
		return err
	}

	disp, dparams, _ := mime.ParseMediaType(headerGet(h, "Content-Disposition"))
	name := dparams["filename"]
	if name == "" {
		name = params["name"]
	}
	if dec, err := new(mime.WordDecoder).DecodeHeader(name); err == nil {
		name = dec
	}
	cid := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(headerGet(h, "Content-Id")), "<"), ">")

	isBody := (ct == "text/html" || ct == "text/plain") && disp != "attachment" && name == ""
	if isBody {
		if ct == "text/html" && !c.hasHTML {
			c.html, c.hasHTML = data, true
		} else if ct == "text/plain" && !c.hasText {
			c.text, c.hasText = data, true
		}
		return nil
	}

	if name == "" {
		name = fallbackName(cid, ct)
	}

	c.attachments = append(c.attachments, graphAttachment{
		Name:        name,
		ContentType: ct,
		ContentID:   cid,
		IsInline:    cid != "",
		Content:     data,
	})
	return nil
}

// Graph rejects empty attachment names, and embedded inline images carry none.
func fallbackName(cid, ct string) string {
	name := cid
	if name == "" {
		name = "attachment"
	}
	if path.Ext(name) != "" {
		return name
	}
	exts, _ := mime.ExtensionsByType(ct)
	ext := ""
	for _, e := range exts {
		if e == ".jpg" {
			ext = e
			break
		}
	}
	if ext == "" && len(exts) > 0 {
		ext = exts[0]
	}
	return name + ext
}
