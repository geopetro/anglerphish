package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestGenerateSendDate(c *check.C) {
	campaign := s.createCampaignDependencies(c)
	// Test that if no launch date is provided, the campaign's creation date
	// is used.
	err := PostCampaign(&campaign, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(campaign.LaunchDate, check.Equals, campaign.CreatedDate)

	// For comparing the dates, we need to fetch the campaign again. This is
	// to solve an issue where the campaign object right now has time down to
	// the microsecond, while in MySQL it's rounded down to the second.
	campaign, _ = GetCampaign(campaign.Id, campaign.UserId)

	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	for _, m := range ms {
		c.Assert(m.SendDate, check.Equals, campaign.CreatedDate)
	}

	// Test that if no send date is provided, all the emails are sent at the
	// campaign's launch date
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	err = PostCampaign(&campaign, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	campaign, _ = GetCampaign(campaign.Id, campaign.UserId)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	for _, m := range ms {
		c.Assert(m.SendDate, check.Equals, campaign.LaunchDate)
	}

	// Finally, test that if a send date is provided, the emails are staggered
	// correctly.
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(2 * time.Minute)
	err = PostCampaign(&campaign, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	campaign, _ = GetCampaign(campaign.Id, campaign.UserId)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	sendingOffset := 2 / float64(len(ms))
	for i, m := range ms {
		expectedOffset := int(sendingOffset * float64(i))
		expectedDate := campaign.LaunchDate.Add(time.Duration(expectedOffset) * time.Minute)
		c.Assert(m.SendDate, check.Equals, expectedDate)
	}
}

func (s *ModelsSuite) TestCampaignDateValidation(c *check.C) {
	campaign := s.createCampaignDependencies(c)
	// If both are zero, then the campaign should start immediately with no
	// send by date
	err := campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the launch date is specified, then the send date is optional
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	err = campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the send date is greater than the launch date, then there's no
	//problem
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(1 * time.Minute)
	err = campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the send date is less than the launch date, then there's an issue
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(-1 * time.Minute)
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrInvalidSendByDate)
}

func (s *ModelsSuite) TestSMSCampaignValidation(c *check.C) {
	// Test that an SMS campaign with valid phone numbers passes validation
	campaign := s.createSMSCampaignDependencies(c)
	err := campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// Test that an SMS campaign with missing phone numbers fails validation
	campaign = s.createSMSCampaignDependencies(c)
	// Create a group with targets that don't have phone numbers
	group := Group{Name: "Test Invalid SMS Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "test1@example.com", FirstName: "First", LastName: "Example"}},
		Target{BaseRecipient: BaseRecipient{Email: "test2@example.com", FirstName: "Second", LastName: "Example"}},
	}
	group.UserId = 1
	c.Assert(PostGroup(&group), check.Equals, nil)
	campaign.Groups = []Group{group}
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrInvalidTargetType)
}

func (s *ModelsSuite) TestLaunchCampaignMaillogStatus(c *check.C) {
	// For the first test, ensure that campaigns created with the zero date
	// (and therefore are set to launch immediately) have maillogs that are
	// locked to prevent race conditions.
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, true)
	}

	// Next, verify that campaigns scheduled in the future do not lock the
	// maillogs so that they can be picked up by the background worker.
	campaign = s.createCampaignDependencies(c)
	campaign.Name = "New Campaign"
	campaign.LaunchDate = time.Now().Add(1 * time.Hour)
	c.Assert(PostCampaign(&campaign, campaign.UserId), check.Equals, nil)
	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, false)
	}
}

func (s *ModelsSuite) TestLaunchSMSCampaignSMSlogStatus(c *check.C) {
	// For the first test, ensure that SMS campaigns created with the zero date
	// (and therefore are set to launch immediately) have smslogs that are
	// locked to prevent race conditions.
	campaign := s.createSMSCampaign(c)
	ms, err := GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, true)
	}

	// Next, verify that SMS campaigns scheduled in the future do not lock the
	// smslogs so that they can be picked up by the background worker.
	campaign = s.createSMSCampaignDependencies(c)
	campaign.Name = "New SMS Campaign"
	campaign.LaunchDate = time.Now().Add(1 * time.Hour)
	c.Assert(PostCampaign(&campaign, campaign.UserId), check.Equals, nil)
	ms, err = GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, false)
	}
}

func (s *ModelsSuite) TestDeleteCampaignAlsoDeletesMailLogs(c *check.C) {
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = DeleteCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestDeleteSMSCampaignAlsoDeletesSMSLogs(c *check.C) {
	campaign := s.createSMSCampaign(c)
	ms, err := GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = DeleteCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	ms, err = GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestCompleteCampaignAlsoDeletesMailLogs(c *check.C) {
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = CompleteCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestCompleteSMSCampaignAlsoDeletesSMSLogs(c *check.C) {
	campaign := s.createSMSCampaign(c)
	ms, err := GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = CompleteCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	ms, err = GetSMSLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestCampaignGetResults(c *check.C) {
	campaign := s.createCampaign(c)
	got, err := GetCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(campaign.Results), check.Equals, len(got.Results))
}

func (s *ModelsSuite) TestCampaignNewFields(c *check.C) {
	// Test new campaign fields: URLParam, QRSize, HTTPAuth
	campaign := s.createCampaignDependencies(c)
	campaign.URLParam = "custom_param"
	campaign.QRSize = "256x256"
	campaign.HTTPAuth = true

	err := PostCampaign(&campaign, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	// Fetch the campaign and verify the new fields are saved
	saved, err := GetCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(saved.URLParam, check.Equals, "custom_param")
	c.Assert(saved.QRSize, check.Equals, "256x256")
	c.Assert(saved.HTTPAuth, check.Equals, true)
}

func (s *ModelsSuite) TestCampaignDefaultURLParam(c *check.C) {
	// Test that URLParam defaults to "rid" when empty
	campaign := s.createCampaignDependencies(c)
	// Don't set URLParam, should default to "rid"

	err := PostCampaign(&campaign, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	// The RecipientParameter should be set to "rid" by default
	c.Assert(RecipientParameter, check.Equals, "rid")
}

func (s *ModelsSuite) TestGetCampaignMailContext(c *check.C) {
	// Test the GetCampaignMailContext function
	campaign := s.createCampaign(c)

	mailContext, err := GetCampaignMailContext(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(mailContext.Id, check.Equals, campaign.Id)
	c.Assert(mailContext.Name, check.Equals, campaign.Name)
	c.Assert(mailContext.Template.Name, check.Equals, campaign.Template.Name)
	c.Assert(mailContext.SMTP.Name, check.Equals, campaign.SMTP.Name)
}

func (s *ModelsSuite) TestGetCampaignSMSContext(c *check.C) {
	// Test the GetCampaignSMSContext function
	campaign := s.createSMSCampaign(c)

	smsContext, err := GetCampaignSMSContext(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(smsContext.Id, check.Equals, campaign.Id)
	c.Assert(smsContext.Name, check.Equals, campaign.Name)
	c.Assert(smsContext.SMSTemplate.Name, check.Equals, "Test SMS Template")
	c.Assert(smsContext.SMS.Name, check.Equals, "Test SMS Profile")
}

func (s *ModelsSuite) TestGetCampaignMailContextWithSMSCampaign(c *check.C) {
	// Test that GetCampaignMailContext returns error for SMS campaigns
	campaign := s.createSMSCampaign(c)

	_, err := GetCampaignMailContext(campaign.Id, campaign.UserId)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "attempted to get email context for an SMS campaign")
}

func (s *ModelsSuite) TestGetCampaignSMSContextWithEmailCampaign(c *check.C) {
	// Test that GetCampaignSMSContext returns error for email campaigns
	campaign := s.createCampaign(c)

	_, err := GetCampaignSMSContext(campaign.Id, campaign.UserId)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "attempted to get SMS context for a non-SMS campaign")
}

func (s *ModelsSuite) TestCampaignValidateEmailType(c *check.C) {
	// Test email campaign validation
	campaign := s.createCampaignDependencies(c)
	campaign.Type = "email" // Explicitly set type

	err := campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// Test missing template
	campaign.Template.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrTemplateNotSpecified)

	// Reset template and test missing page
	campaign.Template.Name = "Test Template"
	campaign.Page.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrPageNotSpecified)

	// Reset page and test missing SMTP
	campaign.Page.Name = "Test Page"
	campaign.SMTP.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrSMTPNotSpecified)
}

func (s *ModelsSuite) TestCampaignValidateSMSType(c *check.C) {
	// Test SMS campaign validation
	campaign := s.createSMSCampaignDependencies(c)

	err := campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// Test missing SMS template
	campaign.SMSTemplate.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrSMSTemplateNotSpecified)

	// Reset SMS template and test missing page
	campaign.SMSTemplate.Name = "Test SMS Template"
	campaign.Page.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrPageNotSpecified)

	// Reset page and test missing SMS profile
	campaign.Page.Name = "Test Page"
	campaign.SMS.Name = ""
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrSMSNotSpecified)
}

func setupCampaignDependencies(b *testing.B, size int) {
	group := Group{Name: "Test Group"}
	// Create a large group of 5000 members
	for i := 0; i < size; i++ {
		group.Targets = append(group.Targets, Target{BaseRecipient: BaseRecipient{Email: fmt.Sprintf("test%d@example.com", i), FirstName: "User", LastName: fmt.Sprintf("%d", i)}})
	}
	group.UserId = 1
	err := PostGroup(&group)
	if err != nil {
		b.Fatalf("error posting group: %v", err)
	}

	// Add a template
	template := Template{Name: "Test Template"}
	template.Subject = "{{.RId}} - Subject"
	template.Text = "{{.RId}} - Text"
	template.HTML = "{{.RId}} - HTML"
	template.UserId = 1
	err = PostTemplate(&template)
	if err != nil {
		b.Fatalf("error posting template: %v", err)
	}

	// Add a landing page
	p := Page{Name: "Test Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = 1
	err = PostPage(&p)
	if err != nil {
		b.Fatalf("error posting page: %v", err)
	}

	// Add a sending profile
	smtp := SMTP{Name: "Test Page"}
	smtp.UserId = 1
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	err = PostSMTP(&smtp)
	if err != nil {
		b.Fatalf("error posting smtp: %v", err)
	}
}

// setupCampaign sets up the campaign dependencies as well as posting the
// actual campaign
func setupCampaign(b *testing.B, size int) Campaign {
	setupCampaignDependencies(b, size)
	campaign := Campaign{Name: "Test campaign"}
	campaign.UserId = 1
	campaign.Template = Template{Name: "Test Template"}
	campaign.Page = Page{Name: "Test Page"}
	campaign.SMTP = SMTP{Name: "Test Page"}
	campaign.Groups = []Group{Group{Name: "Test Group"}}
	PostCampaign(&campaign, 1)
	return campaign
}

func BenchmarkCampaign100(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserId = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		db.Delete(Result{})
		db.Delete(MailLog{})
		db.Delete(Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkCampaign1000(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserId = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		db.Delete(Result{})
		db.Delete(MailLog{})
		db.Delete(Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkCampaign10000(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserId = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		db.Delete(Result{})
		db.Delete(MailLog{})
		db.Delete(Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign100(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserId)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign1000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserId)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign5000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserId)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign10000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserId)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func (s *ModelsSuite) TestEventDetailsRoundTripsMessage(c *check.C) {
	details := EventDetails{
		Message: NewMessageContent("I sent my password", "<p>I sent my password</p>"),
	}
	encoded, err := json.Marshal(details)
	c.Assert(err, check.Equals, nil)

	decoded := EventDetails{}
	c.Assert(json.Unmarshal(encoded, &decoded), check.Equals, nil)
	c.Assert(decoded.Message, check.NotNil)
	c.Assert(decoded.Message.Text, check.Equals, "I sent my password")
}

// Events without captured content must serialize exactly as before.
func (s *ModelsSuite) TestEventDetailsOmitsAbsentMessage(c *check.C) {
	encoded, err := json.Marshal(EventDetails{})
	c.Assert(err, check.Equals, nil)
	c.Assert(strings.Contains(string(encoded), "message"), check.Equals, false)
}
