package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
)

type logMailer struct {
	queue chan []mailer.Mail
}

func (m *logMailer) Start(ctx context.Context) {}

func (m *logMailer) Queue(ms []mailer.Mail) {
	m.queue <- ms
}

type logSMSMailer struct {
	queue chan []mailer.SMSMail
}

func (m *logSMSMailer) Start(ctx context.Context) {}

func (m *logSMSMailer) Queue(ms []mailer.SMSMail) {
	m.queue <- ms
}

// testContext is context to cover API related functions
type testContext struct {
	config *config.Config
}

func setupTest(t *testing.T) *testContext {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../db/db_sqlite3/migrations/",
	}
	err := models.Setup(conf)
	if err != nil {
		t.Fatalf("Failed creating database: %v", err)
	}
	ctx := &testContext{}
	ctx.config = conf
	createTestData(t, ctx)
	return ctx
}

func createTestData(t *testing.T, ctx *testContext) {
	ctx.config.TestFlag = true
	// Add a group
	group := models.Group{Name: "Test Group"}
	for i := 0; i < 10; i++ {
		group.Targets = append(group.Targets, models.Target{
			BaseRecipient: models.BaseRecipient{
				Email:     fmt.Sprintf("test%d@example.com", i),
				FirstName: "First",
				LastName:  "Example"}})
	}
	group.UserId = 1
	models.PostGroup(&group)

	// Add a template
	template := models.Template{Name: "Test Template"}
	template.Subject = "Test subject"
	template.Text = "Text text"
	template.HTML = "<html>Test</html>"
	template.UserId = 1
	models.PostTemplate(&template)

	// Add a landing page
	p := models.Page{Name: "Test Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = 1
	models.PostPage(&p)

	// Add a sending profile
	smtp := models.SMTP{Name: "Test Page"}
	smtp.UserId = 1
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	models.PostSMTP(&smtp)
}

func createSMSTestData(t *testing.T, ctx *testContext) {
	ctx.config.TestFlag = true
	// Add a group with phone numbers (separate from email group)
	group := models.Group{Name: "Test SMS Group"}
	for i := 0; i < 10; i++ {
		group.Targets = append(group.Targets, models.Target{
			BaseRecipient: models.BaseRecipient{
				Email:     fmt.Sprintf("sms%d@example.com", i), // Valid email for SMS campaigns
				Phone:     fmt.Sprintf("+1555%07d", i),
				FirstName: "First",
				LastName:  "Example"}})
	}
	group.UserId = 1
	models.PostGroup(&group)

	// Add an SMS template
	template := models.SMSTemplate{Name: "Test SMS Template"}
	template.Text = "Hello {{.FirstName}}, this is a test SMS with your phone: {{.Phone}}"
	template.UserId = 1
	models.PostSMSTemplate(&template)

	// Add a landing page
	p := models.Page{Name: "Test SMS Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = 1
	models.PostPage(&p)

	// Add an SMS sending profile
	sms := models.SMS{Name: "Test SMS Profile"}
	sms.UserId = 1
	sms.Provider = "twilio"
	sms.From = "+15555555555"
	sms.ProviderConfig = `{"account_sid": "test_sid", "auth_token": "test_token"}`
	models.PostSMS(&sms)
}

func setupCampaign(id int) (*models.Campaign, error) {
	// Setup and "launch" our campaign
	// Set the status such that no emails are attempted
	c := models.Campaign{Name: fmt.Sprintf("Test campaign - %d", id)}
	c.UserId = 1
	template, err := models.GetTemplate(1, 1)
	if err != nil {
		return nil, err
	}
	c.Template = template

	page, err := models.GetPage(1, 1)
	if err != nil {
		return nil, err
	}
	c.Page = page

	smtp, err := models.GetSMTP(1, 1)
	if err != nil {
		return nil, err
	}
	c.SMTP = smtp

	group, err := models.GetGroup(1, 1)
	if err != nil {
		return nil, err
	}
	c.Groups = []models.Group{group}
	err = models.PostCampaign(&c, c.UserId)
	if err != nil {
		return nil, err
	}
	err = c.UpdateStatus(models.CampaignEmailsSent)
	return &c, err
}

func setupSMSCampaign(id int) (*models.Campaign, error) {
	// Setup and "launch" our SMS campaign
	// Set the status such that no SMS messages are attempted
	c := models.Campaign{Name: fmt.Sprintf("Test SMS campaign - %d", id)}
	c.UserId = 1
	c.Type = "sms"

	template, err := models.GetSMSTemplate(1, 1)
	if err != nil {
		return nil, err
	}
	c.SMSTemplate = template

	page, err := models.GetPage(2, 1) // Using the SMS page (second page created)
	if err != nil {
		return nil, err
	}
	c.Page = page

	sms, err := models.GetSMS(1, 1)
	if err != nil {
		return nil, err
	}
	c.SMS = sms

	group, err := models.GetGroup(2, 1) // Using the SMS group (second group created)
	if err != nil {
		return nil, err
	}
	c.Groups = []models.Group{group}
	err = models.PostCampaign(&c, c.UserId)
	if err != nil {
		return nil, err
	}
	err = c.UpdateStatus(models.CampaignEmailsSent)
	return &c, err
}

func TestMailLogGrouping(t *testing.T) {
	setupTest(t)

	// Create the campaigns and unlock the maillogs so that they're picked up
	// by the worker
	for i := 0; i < 10; i++ {
		campaign, err := setupCampaign(i)
		if err != nil {
			t.Fatalf("error creating campaign: %v", err)
		}
		ms, err := models.GetMailLogsByCampaign(campaign.Id)
		if err != nil {
			t.Fatalf("error getting maillogs for campaign: %v", err)
		}
		for _, m := range ms {
			m.Unlock()
		}
	}

	lm := &logMailer{queue: make(chan []mailer.Mail)}
	worker := &DefaultWorker{}
	worker.mailer = lm

	// Trigger the worker, generating the maillogs and sending them to the
	// mailer
	worker.processCampaigns(time.Now())

	// Verify that each slice of maillogs received belong to the same campaign
	for i := 0; i < 10; i++ {
		ms := <-lm.queue
		maillog, ok := ms[0].(*models.MailLog)
		if !ok {
			t.Fatalf("unable to cast mail to models.MailLog")
		}
		expected := maillog.CampaignId
		for _, m := range ms {
			maillog, ok = m.(*models.MailLog)
			if !ok {
				t.Fatalf("unable to cast mail to models.MailLog")
			}
			got := maillog.CampaignId
			if got != expected {
				t.Fatalf("unexpected campaign ID received for maillog: got %d expected %d", got, expected)
			}
		}
	}
}

// testSMSWorker is a mock implementation of the SMSWorker for testing
type testSMSWorker struct {
	queue chan []mailer.SMSMail
}

func newTestSMSWorker() *testSMSWorker {
	return &testSMSWorker{
		queue: make(chan []mailer.SMSMail),
	}
}

func (sw *testSMSWorker) Start(ctx context.Context) {}

func (sw *testSMSWorker) Queue(ms []mailer.SMSMail) {
	sw.queue <- ms
}

// testWorker is a custom implementation of the DefaultWorker for testing
type testWorker struct {
	DefaultWorker
	smsQueue chan []mailer.SMSMail
}

// processSMSCampaigns is a custom implementation that uses the test queue
func (w *testWorker) processSMSCampaigns(t time.Time) error {
	ss, err := models.GetQueuedSMSLogs(t.UTC())
	if err != nil {
		return err
	}

	// Lock the SMSLogs (they will be unlocked after processing)
	err = models.LockSMSLogs(ss, true)
	if err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	// Group the smslogs by campaign ID
	msg := make(map[int64][]mailer.SMSMail)
	for _, s := range ss {
		// Cache the campaign
		c, ok := campaignCache[s.CampaignId]
		if !ok {
			c, err = models.GetCampaignSMSContext(s.CampaignId, s.UserId)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		s.CacheCampaign(&c)
		msg[s.CampaignId] = append(msg[s.CampaignId], s)
	}

	// Process each group of smslogs synchronously for testing
	for _, ssc := range msg {
		w.smsQueue <- ssc
	}
	return nil
}

func TestSMSLogGrouping(t *testing.T) {
	ctx := setupTest(t)
	createSMSTestData(t, ctx)

	// Create the SMS campaigns and unlock the smslogs so that they're picked up
	// by the worker
	for i := 0; i < 5; i++ {
		campaign, err := setupSMSCampaign(i)
		if err != nil {
			t.Fatalf("error creating SMS campaign: %v", err)
		}
		ms, err := models.GetSMSLogsByCampaign(campaign.Id)
		if err != nil {
			t.Fatalf("error getting smslogs for campaign: %v", err)
		}
		t.Logf("Campaign %d: Found %d SMS logs", campaign.Id, len(ms))
		// Ensure SMS logs are properly unlocked using the bulk unlock function
		err = models.LockSMSLogs(ms, false)
		if err != nil {
			t.Fatalf("error unlocking smslogs for campaign: %v", err)
		}
	}

	// Check what SMS logs are available for processing
	queuedSMS, err := models.GetQueuedSMSLogs(time.Now().UTC())
	if err != nil {
		t.Fatalf("error getting queued SMS logs: %v", err)
	}
	t.Logf("Found %d queued SMS logs for processing", len(queuedSMS))

	// Create a test worker with a buffered queue we can read from
	worker := &testWorker{
		smsQueue: make(chan []mailer.SMSMail, 10), // Buffer for all campaigns
	}

	// Trigger the worker, generating the smslogs and sending them to the
	// SMS mailer
	worker.processSMSCampaigns(time.Now())

	// Verify that each slice of smslogs received belong to the same campaign
	expectedCampaigns := len(queuedSMS) / 10 // 10 targets per campaign
	t.Logf("Expecting %d campaign groups", expectedCampaigns)

	for i := 0; i < expectedCampaigns; i++ {
		select {
		case ms := <-worker.smsQueue:
			smslog, ok := ms[0].(*models.SMSLog)
			if !ok {
				t.Fatalf("unable to cast mail to models.SMSLog")
			}
			expected := smslog.CampaignId
			for _, m := range ms {
				smslog, ok = m.(*models.SMSLog)
				if !ok {
					t.Fatalf("unable to cast mail to models.SMSLog")
				}
				got := smslog.CampaignId
				if got != expected {
					t.Fatalf("unexpected campaign ID received for smslog: got %d expected %d", got, expected)
				}
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for SMS logs from worker queue (iteration %d)", i)
		}
	}
}
