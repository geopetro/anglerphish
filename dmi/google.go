package dmi

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"

	"github.com/gophish/gophish/mailer"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	defaultGoogleToken = "https://oauth2.googleapis.com/token"
	defaultGmail       = "https://gmail.googleapis.com"
	scopeGmailInsert   = "https://www.googleapis.com/auth/gmail.insert"
	scopeGmailLabels   = "https://www.googleapis.com/auth/gmail.labels"
)

type serviceAccount struct {
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	ClientID     string `json:"client_id"`
}

type googleDialer struct {
	c      Config
	sa     serviceAccount
	gmail  string
	token  string
	mu     sync.Mutex
	labels map[string]string             // mailbox+"\x00"+name -> label ID
	tokens map[string]oauth2.TokenSource // mailbox -> source, lives as long as the dialer
}

func newGoogleDialer(c Config) (mailer.Dialer, error) {
	return newGoogle(c)
}

func newGoogle(c Config) (*googleDialer, error) {
	raw := c.Secret
	if c.AuthMethod == AuthOperatorKey {
		raw = c.OperatorKey
	}
	var sa serviceAccount
	if err := json.Unmarshal([]byte(raw), &sa); err != nil || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, &Error{Err: errors.New("Service account key is not valid JSON with client_email and private_key")}
	}
	blk, _ := pem.Decode([]byte(sa.PrivateKey))
	if blk == nil {
		return nil, &Error{Err: errors.New("The service account key is invalid")}
	}
	if _, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err != nil {
		if _, err := x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
			return nil, &Error{Err: errors.New("The service account key is invalid")}
		}
	}
	d := &googleDialer{c: c, sa: sa, labels: map[string]string{}, tokens: map[string]oauth2.TokenSource{}}
	d.gmail = strings.TrimRight(c.Endpoints.Gmail, "/")
	if d.gmail == "" {
		d.gmail = defaultGmail
	}
	d.token = c.Endpoints.GoogleToken
	if d.token == "" {
		d.token = defaultGoogleToken
	}
	return d, nil
}

func (d *googleDialer) Dial() (mailer.Sender, error) { return d, nil }
func (d *googleDialer) Close() error                 { return nil }
func (d *googleDialer) Reset() error                 { return nil }

func (d *googleDialer) scopes() []string {
	if d.c.Folder == FolderCustom {
		return []string{scopeGmailInsert, scopeGmailLabels}
	}
	return []string{scopeGmailInsert}
}

func (d *googleDialer) tokenSource(mailbox string) oauth2.TokenSource {
	d.mu.Lock()
	defer d.mu.Unlock()
	if ts, ok := d.tokens[mailbox]; ok {
		return ts
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, d.c.httpClient())
	ts := (&jwt.Config{
		Email:        d.sa.ClientEmail,
		PrivateKey:   []byte(d.sa.PrivateKey),
		PrivateKeyID: d.sa.PrivateKeyID,
		Subject:      mailbox,
		Scopes:       d.scopes(),
		TokenURL:     d.token,
	}).TokenSource(ctx)
	d.tokens[mailbox] = ts
	return ts
}

func (d *googleDialer) dropToken(mailbox string) {
	d.mu.Lock()
	delete(d.tokens, mailbox)
	d.mu.Unlock()
}

func (d *googleDialer) accessToken(mailbox string) (string, error) {
	tok, err := d.tokenSource(mailbox).Token()
	if err != nil {
		return "", d.tokenError(err)
	}
	return tok.AccessToken, nil
}

func (d *googleDialer) tokenError(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		code := re.ErrorCode
		if code == "" {
			var b struct {
				Error string `json:"error"`
			}
			json.Unmarshal(re.Body, &b)
			code = b.Error
		}
		switch code {
		case "unauthorized_client", "invalid_grant", "invalid_client", "access_denied":
			scope := "gmail.insert"
			if d.c.Folder == FolderCustom {
				scope = "gmail.insert and gmail.labels"
			}
			msg := fmt.Sprintf("Google rejected the service account. Check domain-wide delegation for client ID %s with scope %s", d.sa.ClientID, scope)
			desc := re.ErrorDescription
			if desc == "" {
				var b struct {
					Desc string `json:"error_description"`
				}
				json.Unmarshal(re.Body, &b)
				desc = b.Desc
			}
			if desc != "" {
				msg += " (" + desc + ")"
			}
			return &Error{Err: errors.New(msg)}
		}
		if re.Response != nil {
			return classifyHTTP(re.Response.StatusCode, re.Body, re.Response.Header.Get("Retry-After"))
		}
	}
	return &Error{Err: fmt.Errorf("could not get a Google token: %w", err), Temp: true}
}

func (d *googleDialer) httpError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusForbidden && gmailRateLimited(body) {
		e := classifyHTTP(http.StatusTooManyRequests, body, resp.Header.Get("Retry-After")).(*Error)
		e.Err = fmt.Errorf("Gmail rate limit (HTTP 403): %s", strings.TrimSpace(string(body)))
		return e
	}
	if resp.StatusCode == http.StatusForbidden {
		return &Error{Err: fmt.Errorf("Gmail denied access (HTTP 403). Check that the service account client ID %s has domain-wide delegation with the gmail.insert scope: %s", d.sa.ClientID, strings.TrimSpace(string(body)))}
	}
	return classifyHTTP(resp.StatusCode, body, resp.Header.Get("Retry-After"))
}

func (d *googleDialer) do(req *http.Request) (*http.Response, error) {
	return sendTracked(d.c.httpClient(), req, "Gmail")
}

// call sends a Gmail request for the mailbox and refreshes the token once on a 401.
func (d *googleDialer) call(mailbox string, build func() *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		tok, err := d.accessToken(mailbox)
		if err != nil {
			return nil, err
		}
		req := build()
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := d.do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			d.dropToken(mailbox)
			continue
		}
		return resp, nil
	}
}

func (d *googleDialer) labelID(mailbox string) (string, error) {
	switch d.c.Folder {
	case FolderJunk:
		return "SPAM", nil
	case FolderCustom:
	default:
		return "INBOX", nil
	}
	ck := mailbox + "\x00" + d.c.FolderName
	d.mu.Lock()
	id, ok := d.labels[ck]
	d.mu.Unlock()
	if ok {
		return id, nil
	}
	resp, err := d.call(mailbox, func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, d.gmail+"/gmail/v1/users/"+url.PathEscape(mailbox)+"/labels", nil)
		return req
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", d.httpError(resp)
	}
	var out struct {
		Labels []struct{ ID, Name string } `json:"labels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", &Error{Err: fmt.Errorf("could not read Gmail labels: %w", err), Temp: true}
	}
	for _, l := range out.Labels {
		if l.Name == d.c.FolderName {
			d.mu.Lock()
			d.labels[ck] = l.ID
			d.mu.Unlock()
			return l.ID, nil
		}
	}
	return "", &Error{Err: fmt.Errorf("Gmail label %q was not found for %s", d.c.FolderName, mailbox)}
}

func (d *googleDialer) Send(from string, to []string, msg io.WriterTo) error {
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
	label, err := d.labelID(mailbox)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	meta, _ := json.Marshal(map[string][]string{"labelIds": {label, "UNREAD"}})
	p, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	p.Write(meta)
	p, _ = mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"message/rfc822"}})
	p.Write(raw.Bytes())
	mw.Close()

	u := d.gmail + "/upload/gmail/v1/users/" + url.PathEscape(mailbox) + "/messages?uploadType=multipart&internalDateSource=receivedTime"
	resp, err := d.call(mailbox, func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, u, bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())
		return req
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return d.httpError(resp)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// testGoogle proves delegation works by minting a token for the mailbox, nothing is inserted.
func testGoogle(ctx context.Context, c Config, mailbox string) error {
	d, err := newGoogle(c)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return &Error{Err: err, Temp: true}
	}
	if _, err := d.accessToken(mailbox); err != nil {
		return err
	}
	if c.Folder == FolderCustom {
		_, err = d.labelID(mailbox)
	}
	return err
}

func gmailRateLimited(body []byte) bool {
	var b struct {
		Error struct {
			Errors []struct{ Reason string } `json:"errors"`
		} `json:"error"`
	}
	json.Unmarshal(body, &b)
	for _, e := range b.Error.Errors {
		if e.Reason == "rateLimitExceeded" || e.Reason == "userRateLimitExceeded" {
			return true
		}
	}
	return false
}
