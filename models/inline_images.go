package models

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gophish/gomail"
	"github.com/gophish/gophish/dialer"
	log "github.com/gophish/gophish/logger"
)

const (
	// maxInlineImageBytes caps the size of a single remote image that will be
	// embedded inline, to protect the sender from pathological templates.
	maxInlineImageBytes = 10 << 20 // 10 MB
	// inlineImageTimeout bounds how long we'll wait for a single remote image.
	inlineImageTimeout = 10 * time.Second
)

// inlineImage holds an image fetched from a remote URL that will be embedded
// inline in an email using a Content-ID (cid) reference.
type inlineImage struct {
	cid         string
	contentType string
	data        []byte
}

// inlineImageHTTPClient builds the HTTP client used to fetch remote images. It
// reuses Gophish's SSRF-protected dialer so that templates cannot be used to
// reach internal hosts.
func inlineImageHTTPClient() *http.Client {
	tr := &http.Transport{
		DialContext: dialer.Dialer().DialContext,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   inlineImageTimeout,
	}
}

// embedRemoteImages rewrites the <img> tags in html that reference remote
// images, replacing their src with a cid: reference and returning the images
// that must be embedded in the message. The only image left untouched is the
// open-tracking pixel, identified by trackingURL (its host and path), so that
// open tracking keeps working; everything else is inlined, including
// same-domain images and assets Gophish serves from its static endpoint.
// Base64 image data: URIs embedded directly in the HTML are also converted to
// cid: attachments, so they render in clients that strip data: URIs. Any image
// that cannot be fetched or decoded is left untouched. If html cannot be
// parsed, or no images are inlined, the original html is returned unchanged.
func embedRemoteImages(html, trackingURL string, client *http.Client) (string, []inlineImage) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html, nil
	}

	// Parse the tracking URL once so we can recognize (and skip) the tracking
	// pixel by its host and path, ignoring the per-recipient rid query string.
	var trackHost, trackPath string
	if tu, terr := url.Parse(trackingURL); terr == nil {
		trackHost = tu.Host
		trackPath = tu.Path
	}

	seen := map[string]string{} // remote src -> generated cid
	var images []inlineImage

	doc.Find("img").Each(func(_ int, sel *goquery.Selection) {
		src, ok := sel.Attr("src")
		if !ok {
			return
		}
		src = strings.TrimSpace(src)

		// Reuse an already-embedded source (dedup), including repeated data URIs.
		if cid, done := seen[src]; done {
			sel.SetAttr("src", "cid:"+cid)
			return
		}

		var data []byte
		var ctype string
		switch {
		case strings.HasPrefix(strings.ToLower(src), "data:"):
			// Base64 image data: URIs are decoded and embedded, so they render
			// in clients (e.g. Outlook) that strip data: URIs from HTML email.
			d, ct, dok := decodeImageDataURI(src)
			if !dok {
				return
			}
			data, ctype = d, ct
		default:
			u, err := url.Parse(src)
			if err != nil {
				return
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return
			}
			// Never inline the open-tracking pixel: it must stay remote so that
			// the recipient's client fetching it records an "Email Opened"
			// event. Matched by host and path so a differing rid doesn't matter.
			if trackHost != "" && strings.EqualFold(u.Host, trackHost) && u.Path == trackPath {
				return
			}
			d, ct, ferr := fetchInlineImage(src, client)
			if ferr != nil {
				log.Warnf("unable to inline remote image %q: %v", src, ferr)
				return
			}
			data, ctype = d, ct
		}

		cid := fmt.Sprintf("inline-image-%d", len(images)+1)
		seen[src] = cid
		images = append(images, inlineImage{cid: cid, contentType: ctype, data: data})
		sel.SetAttr("src", "cid:"+cid)
	})

	if len(images) == 0 {
		// Nothing changed; return the original html untouched to avoid any
		// serialization differences for templates with no inlinable images.
		return html, nil
	}

	out, err := doc.Html()
	if err != nil {
		// If we somehow can't serialize the modified document, fall back to
		// the original html and embed nothing so we never leave dangling
		// cid: references.
		log.Warn(err)
		return html, nil
	}
	return out, images
}

// decodeImageDataURI decodes a base64-encoded image data: URI (for example
// "data:image/png;base64,iVBORw0K..."), returning the raw bytes and the media
// type. It returns ok=false for non-image media types, non-base64 payloads, or
// malformed input, so the caller leaves those srcs untouched.
func decodeImageDataURI(src string) (data []byte, contentType string, ok bool) {
	if len(src) < len("data:") || !strings.EqualFold(src[:len("data:")], "data:") {
		return nil, "", false
	}
	meta, payload, found := strings.Cut(src[len("data:"):], ",")
	if !found {
		return nil, "", false
	}

	mediaType := meta
	isBase64 := false
	if i := strings.IndexByte(meta, ';'); i >= 0 {
		mediaType = meta[:i]
		for _, param := range strings.Split(meta[i+1:], ";") {
			if strings.EqualFold(strings.TrimSpace(param), "base64") {
				isBase64 = true
			}
		}
	}
	mediaType = strings.TrimSpace(mediaType)
	if !isBase64 || !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return nil, "", false
	}
	return decoded, mediaType, true
}

// fetchInlineImage retrieves a single remote image, returning its bytes and
// normalized content-type. It rejects non-200 responses, non-image content
// types, and bodies larger than maxInlineImageBytes.
func fetchInlineImage(src string, client *http.Client) ([]byte, string, error) {
	resp, err := client.Get(src)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	ctype := resp.Header.Get("Content-Type")
	if i := strings.IndexByte(ctype, ';'); i >= 0 {
		ctype = ctype[:i]
	}
	ctype = strings.TrimSpace(ctype)
	if !strings.HasPrefix(strings.ToLower(ctype), "image/") {
		return nil, "", fmt.Errorf("content-type %q is not an image", ctype)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxInlineImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxInlineImageBytes {
		return nil, "", fmt.Errorf("image exceeds %d bytes", maxInlineImageBytes)
	}
	return data, ctype, nil
}

// embedInlineImage attaches an inline image to the message with a Content-ID
// matching the cid: reference used in the rewritten HTML body.
func embedInlineImage(msg *gomail.Message, img inlineImage) {
	data := img.data
	msg.Embed(img.cid,
		gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}),
		gomail.SetHeader(map[string][]string{
			"Content-ID":          {"<" + img.cid + ">"},
			"Content-Type":        {img.contentType},
			"Content-Disposition": {"inline"},
		}),
	)
}
