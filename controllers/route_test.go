package controllers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gophish/gophish/middleware/ratelimit"
)

func attemptLogin(t *testing.T, ctx *testContext, client *http.Client, username, password, optionalPath string) *http.Response {
	resp, err := http.Get(fmt.Sprintf("%s/login", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		t.Fatalf("error parsing /login response body")
	}
	elem := doc.Find("input[name='csrf_token']").First()
	token, ok := elem.Attr("value")
	if !ok {
		t.Fatal("unable to find csrf_token value in login response")
	}
	if client == nil {
		client = &http.Client{}
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/login%s", ctx.adminServer.URL, optionalPath), strings.NewReader(url.Values{
		"username":   {username},
		"password":   {password},
		"csrf_token": {token},
	}.Encode()))
	if err != nil {
		t.Fatalf("error creating new /login request: %v", err)
	}

	req.Header.Set("Cookie", resp.Header.Get("Set-Cookie"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}
	return resp
}

func TestLoginCSRF(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.PostForm(fmt.Sprintf("%s/login", ctx.adminServer.URL),
		url.Values{
			"username": {"admin"},
			"password": {"gophish"},
		})

	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}

	got := resp.StatusCode
	expected := http.StatusForbidden
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestInvalidCredentials(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "admin", "bogus", "")
	got := resp.StatusCode
	expected := http.StatusUnauthorized
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestSuccessfulLogin(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "admin", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestSuccessfulRedirect(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	next := "/campaigns"
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	resp := attemptLogin(t, ctx, client, "admin", "gophish", fmt.Sprintf("?next=%s", next))
	got := resp.StatusCode
	expected := http.StatusFound
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
	url, err := resp.Location()
	if err != nil {
		t.Fatalf("error parsing response Location header: %v", err)
	}
	if url.Path != next {
		t.Fatalf("unexpected Location header received. expected %s got %s", next, url.Path)
	}
}

func TestAccountLocked(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "houdini", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusUnauthorized
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestPasswordLoginBlockedWhenOIDCEnabled(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "houdini", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusUnauthorized
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestAdminPasswordLoginAllowedWhenOIDCEnabled(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "admin", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestOIDCLoginShowsSSOButton(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/login", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting /login: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}
	if !strings.Contains(string(body), "/auth/oidc/login") {
		t.Fatal("expected SSO login link on login page when OIDC is enabled")
	}
}

func TestOIDCCallbackInvalidState(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)

	loginResp, err := http.Get(fmt.Sprintf("%s/login", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting /login: %v", err)
	}
	loginResp.Body.Close()

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/auth/oidc/callback?code=test&state=invalid", ctx.adminServer.URL), nil)
	if err != nil {
		t.Fatalf("error creating callback request: %v", err)
	}
	req.Header.Set("Cookie", loginResp.Header.Get("Set-Cookie"))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error requesting callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestOIDCLoginRateLimited(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)

	url := fmt.Sprintf("%s/auth/oidc/login", ctx.adminServer.URL)
	for i := 0; i < ratelimit.DefaultRequestsPerMinute; i++ {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on request %d", i+1)
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("final request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if strings.TrimSpace(string(body)) != http.StatusText(http.StatusTooManyRequests) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestOIDCCallbackRateLimited(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)

	url := fmt.Sprintf("%s/auth/oidc/callback?code=test&state=test", ctx.adminServer.URL)
	for i := 0; i < ratelimit.DefaultRequestsPerMinute; i++ {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on request %d", i+1)
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("final request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if strings.TrimSpace(string(body)) != http.StatusText(http.StatusTooManyRequests) {
		t.Fatalf("unexpected body: %q", body)
	}
}
