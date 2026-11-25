package dmi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gophish/gomail"
)

var convNow = time.Date(2026, 11, 17, 10, 30, 0, 0, time.UTC)

func buildMIME(t *testing.T, setup func(m *gomail.Message)) []byte {
	t.Helper()
	m := gomail.NewMessage()
	m.SetAddressHeader("From", "sender@example.com", "Sender Name")
	m.SetHeader("To", m.FormatAddress("victim@example.com", "Victim"))
	m.SetHeader("Subject", "Hello")
	m.SetHeader("Message-Id", "<abc123@example.com>")
	setup(m)
	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func attachBytes(data []byte) gomail.FileSetting {
	return gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func extProps(m graphMessage) map[string]string {
	out := map[string]string{}
	for _, p := range m.SingleValueExtendedProperties {
		out[p.ID] = p.Value
	}
	return out
}

func TestConvertBasic(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetBody("text/plain", "plain body")
		m.AddAlternative("text/html", "<p>html body</p>")
	})
	msg, large, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(large) != 0 {
		t.Fatalf("unexpected large: %d", len(large))
	}
	if msg.Subject != "Hello" || msg.Body.ContentType != "html" || !strings.Contains(msg.Body.Content, "<p>html body</p>") {
		t.Fatalf("bad subject/body: %+v", msg)
	}
	if msg.From.EmailAddress.Address != "sender@example.com" || msg.From.EmailAddress.Name != "Sender Name" || msg.Sender != msg.From {
		t.Fatalf("bad from/sender: %+v", msg)
	}
	if len(msg.ToRecipients) != 1 || msg.ToRecipients[0].EmailAddress.Address != "victim@example.com" || msg.ToRecipients[0].EmailAddress.Name != "Victim" {
		t.Fatalf("bad to: %+v", msg.ToRecipients)
	}
	if msg.IsRead {
		t.Fatal("isRead must be false")
	}
	js, _ := json.Marshal(msg)
	if !strings.Contains(string(js), `"isRead":false`) || strings.Contains(string(js), `"attachments"`) {
		t.Fatalf("bad json: %s", js)
	}
	props := extProps(msg)
	for _, id := range []string{"Integer 0x0E07", "String 0x007D", "String 0x1035", "SystemTime 0x0E06", "SystemTime 0x0039"} {
		if _, ok := props[id]; !ok {
			t.Errorf("missing property %s", id)
		}
	}
	if len(msg.SingleValueExtendedProperties) != 5 {
		t.Errorf("want 5 props, got %d", len(msg.SingleValueExtendedProperties))
	}
	if props["String 0x1035"] != "<abc123@example.com>" {
		t.Errorf("message id: %q", props["String 0x1035"])
	}
	if props["SystemTime 0x0E06"] != "2026-11-17T10:30:00Z" || props["SystemTime 0x0039"] != "2026-11-17T10:30:00Z" {
		t.Errorf("times: %v", props)
	}
	hdr := props["String 0x007D"]
	if !strings.HasPrefix(hdr, "Mime-Version") && !strings.Contains(hdr, "\r\nSubject:") && !strings.Contains(hdr, "Subject:") {
		t.Errorf("header block: %q", hdr)
	}
	if strings.HasSuffix(hdr, "\r\n") || strings.Contains(hdr, "\r\n\r\n") {
		t.Errorf("header block must not contain blank line: %q", hdr)
	}
	if !bytes.HasPrefix(raw, []byte(hdr+"\r\n\r\n")) {
		t.Errorf("header block is not the raw prefix")
	}
}

func TestConvertEncodedHeaders(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetAddressHeader("From", "sender@example.com", "Γιώργος Πέτρου")
		m.SetHeader("Subject", "Καλημέρα κόσμε")
		m.SetBody("text/plain", "x")
	})
	if !strings.Contains(string(raw), "=?UTF-8?") {
		t.Fatalf("expected encoded words in raw message")
	}
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Καλημέρα κόσμε" {
		t.Errorf("subject: %q", msg.Subject)
	}
	if msg.From.EmailAddress.Name != "Γιώργος Πέτρου" {
		t.Errorf("name: %q", msg.From.EmailAddress.Name)
	}
}

func TestConvertQuotedPrintableBody(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetBody("text/html", `<a href="https://example.com/?a=1&b=2">link</a> café`)
	})
	if !strings.Contains(string(raw), "=3D") {
		t.Fatalf("expected quoted-printable in raw message:\n%s", raw)
	}
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.Body.Content, "=3D") || !strings.Contains(msg.Body.Content, `href="https://example.com/?a=1&b=2"`) || !strings.Contains(msg.Body.Content, "café") {
		t.Errorf("body: %q", msg.Body.Content)
	}
}

func TestConvertInlineImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfakeimage")
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetBody("text/plain", "plain")
		m.AddAlternative("text/html", `<img src="cid:logo.png">`)
		m.Embed("logo.png", attachBytes(png))
		m.Attach("doc.txt", attachBytes([]byte("hello doc")))
	})
	msg, large, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(large) != 0 || len(msg.Attachments) != 2 {
		t.Fatalf("attachments: msg=%d large=%d", len(msg.Attachments), len(large))
	}
	a := msg.Attachments[0]
	if a.ODataType != "#microsoft.graph.fileAttachment" || a.Name != "logo.png" || !a.IsInline || a.ContentID != "logo.png" {
		t.Errorf("inline: %+v", a)
	}
	if a.ContentType != "image/png" {
		t.Errorf("inline content type: %q", a.ContentType)
	}
	if dec, _ := base64.StdEncoding.DecodeString(a.ContentBytes); !bytes.Equal(dec, png) {
		t.Errorf("inline bytes mismatch")
	}
	b := msg.Attachments[1]
	if b.Name != "doc.txt" || b.IsInline || b.ContentID != "" {
		t.Errorf("regular: %+v", b)
	}
	js, _ := json.Marshal(b)
	if strings.Contains(string(js), "contentId") {
		t.Errorf("contentId must be omitted: %s", js)
	}
	if msg.Body.ContentType != "html" || !strings.Contains(msg.Body.Content, "cid:logo.png") {
		t.Errorf("body: %+v", msg.Body)
	}
}

func TestConvertTextOnly(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) { m.SetBody("text/plain", "just text") })
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body.ContentType != "text" || !strings.Contains(msg.Body.Content, "just text") {
		t.Errorf("body: %+v", msg.Body)
	}
}

func TestConvertLargeAttachment(t *testing.T) {
	big := bytes.Repeat([]byte{0x41, 0x00, 0xff, 0x10}, 1024*1024)
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetBody("text/html", "<p>x</p>")
		m.Attach("big.bin", attachBytes(big))
	})
	msg, large, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Attachments) != 0 || len(large) != 1 {
		t.Fatalf("msg=%d large=%d", len(msg.Attachments), len(large))
	}
	if large[0].Name != "big.bin" || !bytes.Equal(large[0].Content, big) || large[0].IsInline {
		t.Errorf("large attachment wrong: %s %d", large[0].Name, len(large[0].Content))
	}
}

func TestConvertCustomHeaderPreserved(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetHeader("X-Campaign", "q4")
		m.SetBody("text/plain", "x")
	})
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extProps(msg)["String 0x007D"], "X-Campaign: q4") {
		t.Errorf("header missing: %q", extProps(msg)["String 0x007D"])
	}
}

func TestConvertMalformed(t *testing.T) {
	_, _, err := convertToGraph([]byte("not an email at all"), convNow)
	var de *Error
	if !errors.As(err, &de) || de.Temp {
		t.Fatalf("want permanent *Error, got %v", err)
	}
}

func TestConvertInlineImageNoFilename(t *testing.T) {
	png := []byte("fakepngbytes")
	cid := "img-1a2b3c"
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetBody("text/html", `<img src="cid:`+cid+`">`)
		m.Embed(cid, attachBytes(png), gomail.SetHeader(map[string][]string{
			"Content-ID":          {"<" + cid + ">"},
			"Content-Type":        {"image/png"},
			"Content-Disposition": {"inline"},
		}))
	})
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("attachments: %d", len(msg.Attachments))
	}
	a := msg.Attachments[0]
	if a.Name != cid+".png" || !a.IsInline || a.ContentID != cid || a.ContentType != "image/png" {
		t.Errorf("got %+v", a)
	}
}

func TestFallbackName(t *testing.T) {
	for _, c := range []struct{ cid, ct, want string }{
		{"logo.png", "image/png", "logo.png"},
		{"x", "image/jpeg", "x.jpg"},
		{"", "image/gif", "attachment.gif"},
		{"", "application/x-unknown-zzz", "attachment"},
	} {
		if got := fallbackName(c.cid, c.ct); got != c.want {
			t.Errorf("%q %q: got %q want %q", c.cid, c.ct, got, c.want)
		}
	}
}

func TestConvertReplyToAndCc(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetHeader("Reply-To", m.FormatAddress("replies@phish.example.com", "Help Desk"))
		m.SetHeader("Cc", "boss@example.com", m.FormatAddress("peer@example.com", "Peer"))
		m.SetBody("text/plain", "body")
	})
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ReplyTo) != 1 || msg.ReplyTo[0].EmailAddress != (graphEmailAddress{Name: "Help Desk", Address: "replies@phish.example.com"}) {
		t.Fatalf("replyTo: %+v", msg.ReplyTo)
	}
	if len(msg.CcRecipients) != 2 || msg.CcRecipients[0].EmailAddress.Address != "boss@example.com" || msg.CcRecipients[1].EmailAddress.Name != "Peer" {
		t.Fatalf("cc: %+v", msg.CcRecipients)
	}
	b, _ := json.Marshal(msg)
	if !strings.Contains(string(b), `"replyTo":[{"emailAddress":{"name":"Help Desk","address":"replies@phish.example.com"}}]`) || !strings.Contains(string(b), `"ccRecipients":[`) {
		t.Fatalf("json: %s", b)
	}
}

func TestConvertNoReplyToOmitted(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) { m.SetBody("text/plain", "body") })
	msg, _, err := convertToGraph(raw, convNow)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(msg)
	if strings.Contains(string(b), "replyTo") || strings.Contains(string(b), "ccRecipients") {
		t.Fatalf("empty recipient lists serialised: %s", b)
	}
}

func TestConvertInvalidReplyTo(t *testing.T) {
	raw := buildMIME(t, func(m *gomail.Message) {
		m.SetHeader("Reply-To", "not an address")
		m.SetBody("text/plain", "body")
	})
	_, _, err := convertToGraph(raw, convNow)
	var e *Error
	if !errors.As(err, &e) || e.Retryable() || !strings.Contains(err.Error(), "Reply-To") {
		t.Fatalf("err %v", err)
	}
}
