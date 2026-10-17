package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gophish/gophish/models"
)

func getFirstCampaign(t *testing.T) models.Campaign {
	campaigns, err := models.GetCampaigns(1)
	if err != nil {
		t.Fatalf("error getting first campaign from database: %v", err)
	}
	return campaigns[0]
}

func getFirstEmailRequest(t *testing.T) models.EmailRequest {
	campaign := getFirstCampaign(t)
	req := models.EmailRequest{
		TemplateId:    campaign.TemplateId,
		Template:      campaign.Template,
		PageId:        campaign.PageId,
		Page:          campaign.Page,
		URL:           "http://localhost.localdomain",
		UserId:        1,
		BaseRecipient: campaign.Results[0].BaseRecipient,
		SMTP:          campaign.SMTP,
		FromAddress:   campaign.SMTP.FromAddress,
	}
	err := models.PostEmailRequest(&req)
	if err != nil {
		t.Fatalf("error creating email request: %v", err)
	}
	return req
}

func openEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /track endpoint: %v", err)
	}
	expected, err := ioutil.ReadFile("static/images/pixel.png")
	if err != nil {
		t.Fatalf("error reading local transparent pixel: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected tracking pixel data received. expected %#v got %#v", expected, got)
	}
}

func openEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}
}

func openAttachment(t *testing.T, ctx *testContext, rid string) {
	// Marker placed before rid so rid remains the last query parameter, which
	// is how NewPhishingTemplateContext builds the URL (url.Values.Encode sorts
	// "a" before "rid") and what setupContext relies on to find the rid.
	resp, err := http.Get(fmt.Sprintf("%s/track?a=1&%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
}

// executeMacro mimics the VBA snippet: extra fields first, rid last so
// setupContext still finds the recipient parameter.
func executeMacro(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?m=1&user=jdoe&host=PC01&%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
}

func reportedEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNoContent
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func reportEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func clickLink(t *testing.T, ctx *testContext, rid string, expectedHTML string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading payload from / endpoint response: %v", err)
	}
	if !bytes.Equal(got, []byte(expectedHTML)) {
		t.Fatalf("invalid response received from / endpoint. expected %s got %s", got, expectedHTML)
	}
}

func clickLink404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
}

func TestOpenedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
	if lastEvent.Message != models.EventOpened {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventOpened)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestOpenedAttachment(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]

	openAttachment(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventAttachmentOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventAttachmentOpened, result.Status)
	}
	if lastEvent.Message != models.EventAttachmentOpened {
		t.Fatalf("unexpected event status received. expected %s got %s", models.EventAttachmentOpened, lastEvent.Message)
	}
}

func TestEmailOpenDoesNotDowngradeAttachmentOpen(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]

	openAttachment(t, ctx, result.RId)
	openEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventAttachmentOpened {
		t.Fatalf("email open downgraded status. expected %s got %s", models.EventAttachmentOpened, result.Status)
	}
}

func TestXTrackedRequestRecorded(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	// A request with no valid rid but carrying the X-Tracked header is logged
	// to the tracked_requests table rather than the old CSV file.
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/callback?user=jdoe&host=PC01", ctx.phishServer.URL), nil)
	if err != nil {
		t.Fatalf("error building request: %v", err)
	}
	req.Header.Set("X-Tracked", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error requesting phish server: %v", err)
	}
	resp.Body.Close()

	reqs, err := models.GetTrackedRequests(10)
	if err != nil {
		t.Fatalf("error reading tracked requests: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 tracked request, got %d", len(reqs))
	}
	if !strings.Contains(reqs[0].RawQuery, "jdoe") {
		t.Fatalf("tracked request missing query data: %q", reqs[0].RawQuery)
	}
}

func TestUntrackedInvalidRequestNotRecorded(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	// Without the X-Tracked header, an invalid request is a plain 404 and must
	// not create a tracked_requests row.
	resp, err := http.Get(fmt.Sprintf("%s/callback?user=jdoe", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting phish server: %v", err)
	}
	resp.Body.Close()

	reqs, err := models.GetTrackedRequests(10)
	if err != nil {
		t.Fatalf("error reading tracked requests: %v", err)
	}
	if len(reqs) != 0 {
		t.Fatalf("expected 0 tracked requests, got %d", len(reqs))
	}
}

func TestMacroExecuted(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]

	executeMacro(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventMacroExecuted {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventMacroExecuted, result.Status)
	}
	if lastEvent.Message != models.EventMacroExecuted {
		t.Fatalf("unexpected event status received. expected %s got %s", models.EventMacroExecuted, lastEvent.Message)
	}
	// Extra fields sent by the macro are kept in the event details.
	if !strings.Contains(lastEvent.Details, "jdoe") || !strings.Contains(lastEvent.Details, "PC01") {
		t.Fatalf("macro fields missing from event details: %s", lastEvent.Details)
	}
}

func TestLowerEventsDoNotDowngradeMacroExecuted(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]

	executeMacro(t, ctx, result.RId)
	openEmail(t, ctx, result.RId)
	openAttachment(t, ctx, result.RId)
	clickLink(t, ctx, result.RId, campaign.Page.HTML)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventMacroExecuted {
		t.Fatalf("status downgraded. expected %s got %s", models.EventMacroExecuted, result.Status)
	}
}

func TestSubmitUpgradesMacroExecuted(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]

	executeMacro(t, ctx, result.RId)
	resp, err := http.PostForm(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, result.RId), url.Values{"username": {"test"}, "password": {"test"}})
	if err != nil {
		t.Fatalf("error submitting data: %v", err)
	}
	resp.Body.Close()

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventDataSubmit {
		t.Fatalf("expected %s got %s", models.EventDataSubmit, result.Status)
	}
}

func TestReportedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	reportedEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]

	if result.Reported != true {
		t.Fatalf("unexpected result report status received. expected %v got %v", true, result.Reported)
	}
	if lastEvent.Message != models.EventReported {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventReported)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestClickedPhishingLinkAfterOpen(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)
	clickLink(t, ctx, result.RId, campaign.Page.HTML)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventClicked {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventClicked, result.Status)
	}
	if lastEvent.Message != models.EventClicked {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventClicked)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestNoRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/track", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}

	resp, err = http.Get(ctx.phishServer.URL)
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	got = resp.StatusCode
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
}

func TestInvalidRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	rid := "XXXXXXXXXX"
	openEmail404(t, ctx, rid)
	clickLink404(t, ctx, rid)
	reportEmail404(t, ctx, rid)
}

func TestCompletedCampaignClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}

	models.CompleteCampaign(campaign.Id, 1)
	openEmail404(t, ctx, result.RId)
	clickLink404(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
}

func TestRobotsHandler(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/robots.txt", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /robots.txt endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusOK
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expected := []byte("User-agent: *\nDisallow: /\n")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /robots.txt endpoint: %v", err)
	}
	if !bytes.Equal(body, expected) {
		t.Fatalf("invalid robots.txt response received. expected %s got %s", expected, body)
	}
}

func TestInvalidPreviewID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	bogusRId := fmt.Sprintf("%sbogus", models.PreviewPrefix)
	openEmail404(t, ctx, bogusRId)
	clickLink404(t, ctx, bogusRId)
	reportEmail404(t, ctx, bogusRId)
}

func TestPreviewTrack(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	openEmail(t, ctx, req.RId)
}

func TestPreviewClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	clickLink(t, ctx, req.RId, req.Page.HTML)
}

func TestRedirectTemplating(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	p := models.Page{
		Name:        "Redirect Page",
		HTML:        "<html>Test</html>",
		UserId:      1,
		RedirectURL: "http://example.com/{{.RId}}",
	}
	err := models.PostPage(&p)
	if err != nil {
		t.Fatalf("error posting new page: %v", err)
	}
	smtp, _ := models.GetSMTP(1, 1)
	template, _ := models.GetTemplate(1, 1)
	group, _ := models.GetGroup(1, 1)

	campaign := models.Campaign{Name: "Redirect campaign"}
	campaign.UserId = 1
	campaign.Template = template
	campaign.Page = p
	campaign.SMTP = smtp
	campaign.Groups = []models.Group{group}
	err = models.PostCampaign(&campaign, campaign.UserId)
	if err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	result := campaign.Results[0]
	resp, err := client.PostForm(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, result.RId), url.Values{"username": {"test"}, "password": {"test"}})
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusFound
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expectedURL := fmt.Sprintf("http://example.com/%s", result.RId)
	gotURL, err := resp.Location()
	if err != nil {
		t.Fatalf("error getting Location header from response: %v", err)
	}
	if gotURL.String() != expectedURL {
		t.Fatalf("invalid redirect received. expected %s got %s", expectedURL, gotURL)
	}
}

func createBasicAuthCampaign(t *testing.T, capturePasswords bool) models.Campaign {
	p := models.Page{
		Name:               "Basic Auth Page",
		HTML:               "<html>Corporate Portal</html>",
		UserId:             1,
		RedirectURL:        "http://example.com/{{.RId}}",
		CaptureCredentials: true,
		CapturePasswords:   capturePasswords,
	}
	if err := models.PostPage(&p); err != nil {
		t.Fatalf("error posting new page: %v", err)
	}
	smtp, _ := models.GetSMTP(1, 1)
	template, _ := models.GetTemplate(1, 1)
	group, _ := models.GetGroup(1, 1)

	campaign := models.Campaign{Name: "Basic Auth campaign", HTTPAuth: true}
	campaign.UserId = 1
	campaign.Template = template
	campaign.Page = p
	campaign.SMTP = smtp
	campaign.Groups = []models.Group{group}
	if err := models.PostCampaign(&campaign, campaign.UserId); err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}
	return campaign
}

// basicAuthRequest requests the landing page, optionally with an
// Authorization header, without following redirects.
func basicAuthRequest(t *testing.T, ctx *testContext, rid string, withAuth bool, username, password string) *http.Response {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid), nil)
	if err != nil {
		t.Fatalf("error building request: %v", err)
	}
	if withAuth {
		req.SetBasicAuth(username, password)
	}
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	resp.Body.Close()
	return resp
}

func getCampaignEvents(t *testing.T, id int64) []models.Event {
	c, err := models.GetCampaign(id, 1)
	if err != nil {
		t.Fatalf("error getting campaign: %v", err)
	}
	return c.Events
}

func TestBasicAuthNoHeaderRecordsClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := createBasicAuthCampaign(t, true)
	result := campaign.Results[0]

	resp := basicAuthRequest(t, ctx, result.RId, false, "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d got %d", http.StatusUnauthorized, resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="Corporate Portal"` {
		t.Fatalf("unexpected WWW-Authenticate header, got %q", got)
	}
	events := getCampaignEvents(t, campaign.Id)
	if last := events[len(events)-1]; last.Message != models.EventClicked {
		t.Fatalf("expected last event %s got %s", models.EventClicked, last.Message)
	}
}

func TestBasicAuthRealm(t *testing.T) {
	long := strings.Repeat("a", maxRealmLength+50)
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{"plain text", "Corporate VPN", "Corporate VPN"},
		{"tags stripped and whitespace collapsed", "<html><body>\n  <h1>Corporate</h1>\n\t<p>VPN</p>\n</body></html>", "Corporate VPN"},
		{"quotes escaped", `Say "hi"`, `Say \"hi\"`},
		{"backslashes escaped", `DOMAIN\user \`, `DOMAIN\\user \\`},
		{"entities decoded", "Tom &amp; Jerry&#39;s", "Tom & Jerry's"},
		{"control characters removed", "a\x00b\x07c", "a b c"},
		{"length capped", long, long[:maxRealmLength]},
		{"cap applied before escaping", strings.Repeat(`"`, maxRealmLength+5), strings.Repeat(`\"`, maxRealmLength)},
		{"empty page", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := basicAuthRealm(tc.html); got != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, got)
			}
		})
	}
}

func TestBasicAuthEmptyCredentialsNotRecorded(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := createBasicAuthCampaign(t, true)
	result := campaign.Results[0]
	before := len(getCampaignEvents(t, campaign.Id))

	for _, creds := range [][2]string{{"", ""}, {"user", ""}, {"", "pass"}} {
		resp := basicAuthRequest(t, ctx, result.RId, true, creds[0], creds[1])
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("credentials %q: expected status %d got %d", creds, http.StatusUnauthorized, resp.StatusCode)
		}
	}
	if after := len(getCampaignEvents(t, campaign.Id)); after != before {
		t.Fatalf("expected no new events for empty credentials, got %d new", after-before)
	}
}

func TestBasicAuthCredentialsSubmitted(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := createBasicAuthCampaign(t, true)
	result := campaign.Results[0]

	resp := basicAuthRequest(t, ctx, result.RId, true, "jdoe", "hunter2")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d got %d", http.StatusFound, resp.StatusCode)
	}
	expectedURL := fmt.Sprintf("http://example.com/%s", result.RId)
	if got := resp.Header.Get("Location"); got != expectedURL {
		t.Fatalf("expected redirect to %s got %s", expectedURL, got)
	}

	events := getCampaignEvents(t, campaign.Id)
	last := events[len(events)-1]
	if last.Message != models.EventDataSubmit {
		t.Fatalf("expected last event %s got %s", models.EventDataSubmit, last.Message)
	}
	var details models.EventDetails
	if err := json.Unmarshal([]byte(last.Details), &details); err != nil {
		t.Fatalf("error decoding event details: %v", err)
	}
	if details.Payload.Get("Username") != "jdoe" || details.Payload.Get("Password") != "hunter2" {
		t.Fatalf("unexpected payload recorded: %v", details.Payload)
	}
}

func TestBasicAuthPasswordNotCaptured(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := createBasicAuthCampaign(t, false)
	result := campaign.Results[0]

	basicAuthRequest(t, ctx, result.RId, true, "jdoe", "hunter2")

	events := getCampaignEvents(t, campaign.Id)
	last := events[len(events)-1]
	if last.Message != models.EventDataSubmit {
		t.Fatalf("expected last event %s got %s", models.EventDataSubmit, last.Message)
	}
	var details models.EventDetails
	if err := json.Unmarshal([]byte(last.Details), &details); err != nil {
		t.Fatalf("error decoding event details: %v", err)
	}
	if details.Payload.Get("Username") != "jdoe" {
		t.Fatalf("expected username to be recorded, got %v", details.Payload)
	}
	if _, ok := details.Payload["Password"]; ok {
		t.Fatalf("password recorded although CapturePasswords is disabled: %v", details.Payload)
	}
}
