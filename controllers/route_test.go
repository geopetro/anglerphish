package controllers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/crypto"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
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

// getDashboardBody logs in as the given user and returns the body of GET /.
func getDashboardBody(t *testing.T, ctx *testContext, username, password string) string {
	t.Helper()
	u, err := models.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("error getting user %s: %v", username, err)
	}
	u.PasswordChangeRequired = false
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error updating user %s: %v", username, err)
	}
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp := attemptLogin(t, ctx, noRedirect, username, password, "")
	resp.Body.Close()
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/", ctx.adminServer.URL), nil)
	if err != nil {
		t.Fatalf("error creating dashboard request: %v", err)
	}
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	resp, err = noRedirect.Do(req)
	if err != nil {
		t.Fatalf("error requesting dashboard: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected dashboard status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading dashboard body: %v", err)
	}
	return string(body)
}

func createBannerTestUser(t *testing.T) {
	t.Helper()
	hash, err := auth.GeneratePasswordHash("gophish")
	if err != nil {
		t.Fatalf("error hashing password: %v", err)
	}
	role, err := models.GetRoleBySlug(models.RoleUser)
	if err != nil {
		t.Fatalf("error getting user role: %v", err)
	}
	u := models.User{Username: "regular", Hash: hash, ApiKey: "bannerkey", Role: role, RoleID: role.ID}
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error creating regular user: %v", err)
	}
}

func TestEncryptionLockedBannerAdmin(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	models.SetEncryptionLockedForTest(true)
	defer models.SetEncryptionLockedForTest(false)
	body := getDashboardBody(t, ctx, "admin", "gophish")
	if !strings.Contains(body, "Unlock in Settings") {
		t.Fatal("expected admin locked banner")
	}
	if !strings.Contains(body, "/settings#encryptionSettings") {
		t.Fatal("expected unlock link target")
	}
	if strings.Contains(body, "contact an administrator") {
		t.Fatal("admin should not see the non-admin notice")
	}
}

func TestEncryptionLockedBannerNonAdmin(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	createBannerTestUser(t)
	models.SetEncryptionLockedForTest(true)
	defer models.SetEncryptionLockedForTest(false)
	body := getDashboardBody(t, ctx, "regular", "gophish")
	if !strings.Contains(body, "contact an administrator") {
		t.Fatal("expected non-admin locked notice")
	}
	if !strings.Contains(body, "unavailable &mdash; contact") {
		t.Fatal("expected em dash entity in non-admin notice")
	}
	if strings.Contains(body, "Unlock in Settings") {
		t.Fatal("non-admin must not see the unlock link")
	}
}

func TestEncryptionBannerAbsentWhenUnlocked(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	body := getDashboardBody(t, ctx, "admin", "gophish")
	for _, s := range []string{"Unlock in Settings", "contact an administrator", "active for this session only"} {
		if strings.Contains(body, s) {
			t.Fatalf("unexpected banner text %q", s)
		}
	}
}

func TestEncryptionSessionKeyBanner(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	createBannerTestUser(t)
	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("error generating key: %v", err)
	}
	if err := crypto.LoadKey(key, crypto.SourceSession); err != nil {
		t.Fatalf("error loading key: %v", err)
	}
	defer crypto.Unload()
	defer models.SetEncryptionLockedForTest(false)
	if body := getDashboardBody(t, ctx, "admin", "gophish"); !strings.Contains(body, "active for this session only") {
		t.Fatal("expected session key banner for admin")
	}
	if body := getDashboardBody(t, ctx, "regular", "gophish"); strings.Contains(body, "active for this session only") {
		t.Fatal("non-admin must not see session key banner")
	}
}

// TestPageTemplatesParse guards against page templates that only fail when
// someone opens them, e.g. documentation examples containing literal "{{".
func TestPageTemplatesParse(t *testing.T) {
	origPath, _ := os.Getwd()
	if err := os.Chdir("../"); err != nil {
		t.Fatalf("error changing directories: %v", err)
	}
	defer os.Chdir(origPath)

	pages := []string{
		"api_documentation", "campaign_results", "campaign_sets", "campaigns",
		"dashboard", "groups", "landing_pages", "non_campaign_reports",
		"qr_code_generator", "reports", "reset_password", "sending_profiles",
		"settings", "templates", "users", "webhooks",
		// sms_templates is omitted: SMSTemplates is unrouted and its file does not exist.
	}
	for _, page := range pages {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("template %s failed to parse: %v", page, r)
				}
			}()
			getTemplate(nil, page)
		}()
	}
}

func TestOIDCDisableLiveRemovesRestriction(t *testing.T) {
	ctx := setupOIDCTest(t)
	defer tearDown(t, ctx)
	createBannerTestUser(t)

	loginBody := func() string {
		resp, err := http.Get(fmt.Sprintf("%s/login", ctx.adminServer.URL))
		if err != nil {
			t.Fatalf("error requesting /login: %v", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("error reading response: %v", err)
		}
		return string(b)
	}
	if !strings.Contains(loginBody(), "/auth/oidc/login") {
		t.Fatal("expected SSO link while enabled")
	}
	err := models.SaveOIDCSettings(models.OIDCSettingsInput{Enabled: false, Provider: "keycloak"})
	if err != nil {
		t.Fatalf("error disabling sso: %v", err)
	}
	if strings.Contains(loginBody(), "/auth/oidc/login") {
		t.Fatal("expected no SSO link after disabling")
	}
	resp := attemptLogin(t, ctx, nil, "regular", "gophish", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected non-admin password login to succeed, got %d", resp.StatusCode)
	}
}

func TestOIDCRoutesNotFoundWhenDisabled(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/auth/oidc/login", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting oidc login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
