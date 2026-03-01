package models

import (
	"fmt"

	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestSMSRequestValidate(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Set up test SMS template
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template with {{.FirstName}} and {{.URL}}",
		UserId: 1,
	}
	err = PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	// Set up test page
	p := Page{
		Name:   "Test Page",
		HTML:   "<html>Test</html>",
		UserId: 1,
	}
	err = PostPage(&p)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	req := SMSRequest{
		SMS:         sms,
		SMSId:       sms.Id,
		SMSTemplate: template,
		Page:        p,
		URL:         "http://example.com/{{.RId}}",
		BaseRecipient: BaseRecipient{
			Email:     "john@example.com",
			Phone:     "+15551234567",
			FirstName: "John",
			LastName:  "Doe",
		},
	}

	err = req.Validate()
	c.Assert(err, check.Equals, nil)
}

func (s *ModelsSuite) TestSMSRequestValidateNoPhone(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Create an invalid SMS request (no phone number)
	req := SMSRequest{
		SMS:   sms,
		SMSId: sms.Id,
		URL:   "http://example.com/{{.RId}}",
	}

	err = req.Validate()
	c.Assert(err.Error(), check.Equals, "No phone number specified")
}

func (s *ModelsSuite) TestSMSRequestValidateNoFrom(c *check.C) {
	// Create an invalid SMS request (no from number)
	req := SMSRequest{
		SMS: SMS{},
		URL: "http://example.com/{{.RId}}",
		BaseRecipient: BaseRecipient{
			Email: "john@example.com",
			Phone: "+15551234567",
		},
	}

	err := req.Validate()
	c.Assert(err.Error(), check.Equals, "No from number specified")
}

func (s *ModelsSuite) TestSMSRequestGenerate(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Set up test SMS template
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "Hello {{.FirstName}}, please check {{.URL}}. Your phone number is {{.Phone}}",
		UserId: 1,
	}
	err = PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	// Set up test page
	p := Page{
		Name:   "Test Page",
		HTML:   "<html>Test</html>",
		UserId: 1,
	}
	err = PostPage(&p)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	rid := "test_rid"
	req := SMSRequest{
		SMS:           sms,
		SMSId:         sms.Id,
		SMSTemplate:   template,
		SMSTemplateId: template.Id,
		Page:          p,
		PageId:        p.Id,
		URL:           "http://example.com/{{.RId}}",
		RId:           rid,
		BaseRecipient: BaseRecipient{
			Email:     "john@example.com",
			Phone:     "+15551234567",
			FirstName: "John",
			LastName:  "Doe",
		},
	}

	// Test Generate method
	text, err := req.Generate()
	c.Assert(err, check.Equals, nil)
	c.Assert(text, check.Equals, "Hello John, please check http://example.com/test_rid?rid=test_rid. Your phone number is +15551234567")
}

func (s *ModelsSuite) TestPostSMSRequest(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Set up test SMS template
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "Hello {{.FirstName}}, please check {{.URL}}",
		UserId: 1,
	}
	err = PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	// Set up test page
	p := Page{
		Name:   "Test Page",
		HTML:   "<html>Test</html>",
		UserId: 1,
	}
	err = PostPage(&p)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	req := SMSRequest{
		SMS:           sms,
		SMSId:         sms.Id,
		SMSTemplate:   template,
		SMSTemplateId: template.Id,
		Page:          p,
		PageId:        p.Id,
		URL:           "http://example.com/{{.RId}}",
		BaseRecipient: BaseRecipient{
			Email:     "john@example.com",
			Phone:     "+15551234567",
			FirstName: "John",
			LastName:  "Doe",
		},
	}

	// Test PostSMSRequest
	err = PostSMSRequest(&req)
	c.Assert(err, check.Equals, nil)
	c.Assert(req.RId, check.Not(check.Equals), "")
	c.Assert(req.RId[:len(SMSPreviewPrefix)], check.Equals, SMSPreviewPrefix)
}

func (s *ModelsSuite) TestGetSMSRequestByResultId(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Set up test SMS template
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "Hello {{.FirstName}}, please check {{.URL}}",
		UserId: 1,
	}
	err = PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	// Set up test page
	p := Page{
		Name:   "Test Page",
		HTML:   "<html>Test</html>",
		UserId: 1,
	}
	err = PostPage(&p)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	req := SMSRequest{
		SMS:           sms,
		SMSId:         sms.Id,
		SMSTemplate:   template,
		SMSTemplateId: template.Id,
		Page:          p,
		PageId:        p.Id,
		URL:           "http://example.com/{{.RId}}",
		BaseRecipient: BaseRecipient{
			Email:     "john@example.com",
			Phone:     "+15551234567",
			FirstName: "John",
			LastName:  "Doe",
		},
	}

	// Save the request
	err = PostSMSRequest(&req)
	c.Assert(err, check.Equals, nil)

	// Test GetSMSRequestByResultId
	fetchedReq, err := GetSMSRequestByResultId(req.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(fetchedReq.RId, check.Equals, req.RId)
	c.Assert(fetchedReq.SMSId, check.Equals, req.SMSId)
	c.Assert(fetchedReq.SMSTemplateId, check.Equals, req.SMSTemplateId)
	c.Assert(fetchedReq.PageId, check.Equals, req.PageId)
}

func (s *ModelsSuite) TestSMSRequestBackoffAndError(c *check.C) {
	req := SMSRequest{
		ErrorChan: make(chan error),
	}

	// Test Backoff method
	go func() {
		err := req.Backoff(fmt.Errorf("test error"))
		c.Assert(err, check.IsNil)
	}()
	err := <-req.ErrorChan
	c.Assert(err.Error(), check.Equals, "test error")

	// Test Error method
	go func() {
		err := req.Error(fmt.Errorf("another test error"))
		c.Assert(err, check.IsNil)
	}()
	err = <-req.ErrorChan
	c.Assert(err.Error(), check.Equals, "another test error")

	// Test Success method
	go func() {
		err := req.Success()
		c.Assert(err, check.IsNil)
	}()
	err = <-req.ErrorChan
	c.Assert(err, check.IsNil)
}

func (s *ModelsSuite) TestSMSRequestGetDialer(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	req := SMSRequest{
		SMS:   sms,
		SMSId: sms.Id,
		BaseRecipient: BaseRecipient{
			Email: "john@example.com",
			Phone: "+15551234567",
		},
	}

	// Test GetDialer method
	dialer, err := req.GetDialer()
	c.Assert(err, check.IsNil)
	c.Assert(dialer, check.NotNil)
}

func (s *ModelsSuite) TestSMSRequestGetSMSFromAndTo(c *check.C) {
	// Set up test SMS profile
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	// Create a valid SMS request
	req := SMSRequest{
		SMS:   sms,
		SMSId: sms.Id,
		BaseRecipient: BaseRecipient{
			Email: "john@example.com",
			Phone: "+15551234567",
		},
	}

	// Test GetSMSFrom method
	from, err := req.GetSMSFrom()
	c.Assert(err, check.IsNil)
	c.Assert(from, check.Equals, "+15555555555")

	// Test GetSMSTo method
	to, err := req.GetSMSTo()
	c.Assert(err, check.IsNil)
	c.Assert(to, check.Equals, "+15551234567")
}
