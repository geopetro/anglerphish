package dmi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gophish/gophish/mailer"
)

const (
	defaultGraph   = "https://graph.microsoft.com"
	uploadChunkLen = 3 * 1024 * 1024
)

type microsoftDialer struct {
	c       Config
	graph   string
	mu      sync.Mutex
	folders map[string]string // mailbox -> custom folder ID
}

func newMicrosoftDialer(c Config) (mailer.Dialer, error) {
	return newMicrosoft(c)
}

func newMicrosoft(c Config) (*microsoftDialer, error) {
	if _, err := microsoftTokenSource(c); err != nil {
		return nil, err
	}
	g := strings.TrimRight(c.Endpoints.Graph, "/")
	if g == "" {
		g = defaultGraph
	}
	return &microsoftDialer{c: c, graph: g, folders: map[string]string{}}, nil
}

func (d *microsoftDialer) Dial() (mailer.Sender, error) { return d, nil }
func (d *microsoftDialer) Close() error                 { return nil }
func (d *microsoftDialer) Reset() error                 { return nil }

func (d *microsoftDialer) userURL(mailbox string) string {
	return d.graph + "/v1.0/users/" + url.PathEscape(mailbox)
}

// call sends an authenticated Graph request and refreshes the token once on a 401.
func (d *microsoftDialer) call(ctx context.Context, method, u string, body []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		ts, err := microsoftTokenSource(d.c)
		if err != nil {
			return nil, err
		}
		tok, err := ts.Token()
		if err != nil {
			return nil, err
		}
		out, status, err := d.do(ctx, method, u, body, map[string]string{"Authorization": "Bearer " + tok.AccessToken})
		if status == http.StatusUnauthorized && attempt == 0 {
			invalidateTokenSource(microsoftTokenKey(d.c))
			continue
		}
		return out, err
	}
}

func (d *microsoftDialer) do(ctx context.Context, method, u string, body []byte, headers map[string]string) ([]byte, int, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, 0, &Error{Err: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := sendTracked(d.c.httpClient(), req, "Microsoft Graph")
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, resp.StatusCode, graphHTTPError(resp.StatusCode, b, resp.Header.Get("Retry-After"))
	}
	// A truncated success body would make the caller retry a request that already took effect.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, &Error{Err: fmt.Errorf("could not read Microsoft Graph response: %w", err)}
	}
	return b, resp.StatusCode, nil
}

func graphHTTPError(status int, body []byte, retryAfter string) error {
	var b struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	json.Unmarshal(body, &b)
	detail := strings.TrimSpace(b.Error.Code + ": " + b.Error.Message)
	if detail == ":" {
		detail = strings.TrimSpace(string(body))
	}
	switch status {
	case http.StatusForbidden:
		return &Error{Err: fmt.Errorf("Microsoft Graph denied access (HTTP 403) %s. Grant the Mail.ReadWrite application permission with admin consent, and check the RBAC for Applications / Application Access Policy scope includes this mailbox", detail)}
	case http.StatusNotFound:
		return &Error{Err: fmt.Errorf("Mailbox not found (HTTP 404) %s", detail)}
	case http.StatusUnauthorized:
		return &Error{Err: fmt.Errorf("Microsoft Graph rejected the token (HTTP 401) %s", detail)}
	}
	return classifyHTTP(status, body, retryAfter)
}

func (d *microsoftDialer) folderID(ctx context.Context, mailbox string) (string, error) {
	switch d.c.Folder {
	case FolderJunk:
		return "junkemail", nil
	case FolderCustom:
	default:
		return "inbox", nil
	}
	name := d.c.FolderName
	ck := mailbox + "\x00" + name
	d.mu.Lock()
	id, ok := d.folders[ck]
	d.mu.Unlock()
	if ok {
		return id, nil
	}
	var segs []string
	for _, s := range strings.Split(name, "/") {
		if s = strings.TrimSpace(s); s != "" {
			segs = append(segs, s)
		}
	}
	notFound := &Error{Err: fmt.Errorf("Folder %q was not found for %s", name, mailbox)}
	if len(segs) == 0 {
		return "", notFound
	}
	id, err := d.findFolder(ctx, mailbox, "/mailFolders", segs[0])
	if err != nil {
		return "", err
	}
	if id == "" && len(segs) == 1 {
		id, err = d.findFolder(ctx, mailbox, "/mailFolders/inbox/childFolders", segs[0])
		if err != nil {
			return "", err
		}
	}
	for _, s := range segs[1:] {
		if id == "" {
			break
		}
		id, err = d.findFolder(ctx, mailbox, "/mailFolders/"+url.PathEscape(id)+"/childFolders", s)
		if err != nil {
			return "", err
		}
	}
	if id == "" {
		return "", notFound
	}
	d.mu.Lock()
	d.folders[ck] = id
	d.mu.Unlock()
	return id, nil
}

// findFolder returns the ID of the named folder under the given collection path, or "" if absent.
func (d *microsoftDialer) findFolder(ctx context.Context, mailbox, collection, name string) (string, error) {
	q := url.Values{"$filter": {"displayName eq '" + strings.ReplaceAll(name, "'", "''") + "'"}}
	body, err := d.call(ctx, http.MethodGet, d.userURL(mailbox)+collection+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Value []struct{ ID, DisplayName string } `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", &Error{Err: fmt.Errorf("could not read Graph folders: %w", err), Temp: true}
	}
	if len(out.Value) == 0 {
		return "", nil
	}
	return out.Value[0].ID, nil
}

func (d *microsoftDialer) Send(from string, to []string, msg io.WriterTo) error {
	if len(to) == 0 {
		return &Error{Err: errors.New("No recipient to deliver to")}
	}
	mailbox, err := d.c.CheckRecipient(to[0])
	if err != nil {
		return err
	}
	var raw bytes.Buffer
	if _, err := msg.WriteTo(&raw); err != nil {
		return &Error{Err: fmt.Errorf("could not render message: %w", err)}
	}
	gm, large, err := convertToGraph(raw.Bytes(), time.Now())
	if err != nil {
		return err
	}
	ctx := context.Background()
	folder, err := d.folderID(ctx, mailbox)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(gm)
	if err != nil {
		return &Error{Err: err}
	}
	body, err := d.call(ctx, http.MethodPost, d.userURL(mailbox)+"/mailFolders/"+url.PathEscape(folder)+"/messages", payload)
	if err != nil {
		return err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		// The message exists already, so a retry would duplicate it.
		return &Error{Err: errors.New("Microsoft Graph created the message but returned an unreadable response")}
	}
	msgURL := d.userURL(mailbox) + "/messages/" + url.PathEscape(created.ID)
	for _, a := range large {
		if err := d.attach(ctx, msgURL, a); err != nil {
			if _, perr := d.call(ctx, http.MethodPost, msgURL+"/permanentDelete", nil); perr != nil {
				if _, derr := d.call(ctx, http.MethodDelete, msgURL, nil); derr != nil {
					// The half-built message is still in the mailbox, so a retry would leave two copies.
					return &Error{Err: fmt.Errorf("attachment upload failed and the incomplete message could not be removed from the mailbox. Remove it before resending: %v", err)}
				}
			}
			return err
		}
	}
	return nil
}

func (d *microsoftDialer) attach(ctx context.Context, msgURL string, a graphAttachment) error {
	if len(a.Content) >= uploadChunkLen {
		return d.upload(ctx, msgURL, a)
	}
	// Graph rejects upload sessions for files under 3 MB.
	fa := graphFileAttachment{
		ODataType:    "#microsoft.graph.fileAttachment",
		Name:         a.Name,
		ContentType:  a.ContentType,
		ContentBytes: base64.StdEncoding.EncodeToString(a.Content),
		IsInline:     a.IsInline,
		ContentID:    a.ContentID,
	}
	payload, err := json.Marshal(fa)
	if err != nil {
		return &Error{Err: err}
	}
	_, err = d.call(ctx, http.MethodPost, msgURL+"/attachments", payload)
	return err
}

func (d *microsoftDialer) upload(ctx context.Context, msgURL string, a graphAttachment) error {
	item := map[string]interface{}{
		"attachmentType": "file",
		"name":           a.Name,
		"size":           len(a.Content),
		"contentType":    a.ContentType,
		"isInline":       a.IsInline,
	}
	if a.ContentID != "" {
		item["contentId"] = a.ContentID
	}
	req, _ := json.Marshal(map[string]interface{}{"AttachmentItem": item})
	// UNVERIFIED: createUploadSession is documented for drafts and not confirmed on non-draft items.
	body, err := d.call(ctx, http.MethodPost, msgURL+"/attachments/createUploadSession", req)
	if err != nil {
		return err
	}
	var sess struct {
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.Unmarshal(body, &sess); err != nil || sess.UploadURL == "" {
		return &Error{Err: errors.New("Microsoft Graph returned an unreadable upload session"), Temp: true}
	}
	total := len(a.Content)
	for start := 0; start < total; start += uploadChunkLen {
		end := min(start+uploadChunkLen, total)
		// The upload URL is pre-authenticated, so no bearer token is sent.
		_, _, err := d.do(ctx, http.MethodPut, sess.UploadURL, a.Content[start:end], map[string]string{
			"Content-Range": "bytes " + strconv.Itoa(start) + "-" + strconv.Itoa(end-1) + "/" + strconv.Itoa(total),
			"Content-Type":  "application/octet-stream",
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// testMicrosoft proves the app can read the target folder, nothing is created.
func testMicrosoft(ctx context.Context, c Config, mailbox string) error {
	d, err := newMicrosoft(c)
	if err != nil {
		return err
	}
	folder, err := d.folderID(ctx, mailbox)
	if err != nil {
		return err
	}
	_, err = d.call(ctx, http.MethodGet, d.userURL(mailbox)+"/mailFolders/"+url.PathEscape(folder), nil)
	return err
}
