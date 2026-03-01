package models

import (
	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestNewSMSTemplateContext(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
			Phone:     "+15551234567",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "+15555555555", // Phone number as from address
	}
	got, err := NewSMSTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)

	// Check non-dynamic fields
	c.Assert(got.From, check.Equals, "+15555555555")
	c.Assert(got.URL, check.Equals, "http://example.com?rid=1234567")
	c.Assert(got.TrackingURL, check.Equals, "http://example.com/track?rid=1234567")
	c.Assert(got.RId, check.Equals, "1234567")
	c.Assert(got.BaseURL, check.Equals, "http://example.com")
	c.Assert(got.BaseRecipient, check.DeepEquals, r.BaseRecipient)

	// Check that dynamic date/time fields are populated (not empty)
	c.Assert(got.CurrentDateTime, check.Not(check.Equals), "")
	c.Assert(got.CurrentDate, check.Not(check.Equals), "")
	c.Assert(got.CurrentTime, check.Not(check.Equals), "")
	c.Assert(got.CurrentTime24, check.Not(check.Equals), "")
}

func (s *ModelsSuite) TestSMSTemplateExecution(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
			Phone:     "+15551234567",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "+15555555555", // Phone number as from address
	}
	stx, err := NewSMSTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)

	// Test that the Phone field is correctly used in template execution
	template := "Hello {{.FirstName}}, your phone number is {{.Phone}}. From: {{.From}}"
	expected := "Hello Foo, your phone number is +15551234567. From: +15555555555"
	result, err := ExecuteTemplate(template, stx)
	c.Assert(err, check.Equals, nil)
	c.Assert(result, check.Equals, expected)
}
