package models

import (
	"fmt"

	check "gopkg.in/check.v1"
)

type mockTemplateContext struct {
	URL         string
	FromAddress string
}

// getQRSize implements TemplateContext.
func (m mockTemplateContext) getQRSize() string {
	return ""
}

func (m mockTemplateContext) getFromAddress() string {
	return m.FromAddress
}

func (m mockTemplateContext) getBaseURL() string {
	return m.URL
}

func (s *ModelsSuite) TestNewTemplateContext(c *check.C) {
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
		FromAddress: "From Address <from@example.com>",
	}
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)

	// Check non-dynamic fields
	c.Assert(got.From, check.Equals, "From Address")
	c.Assert(got.URL, check.Equals, fmt.Sprintf("%s?rid=%s", ctx.URL, r.RId))
	c.Assert(got.TrackingURL, check.Equals, fmt.Sprintf("%s/track?rid=%s", ctx.URL, r.RId))
	c.Assert(got.RId, check.Equals, r.RId)
	c.Assert(got.BaseURL, check.Equals, ctx.URL)
	c.Assert(got.BaseRecipient, check.DeepEquals, r.BaseRecipient)
	c.Assert(got.QRFallbackText, check.Equals, fmt.Sprintf("%s?rid=%s", ctx.URL, r.RId))
	c.Assert(got.Tracker, check.Equals, "<img alt='' style='display: none' src='"+got.TrackingURL+"'/>")

	// Check that dynamic date/time fields are populated (not empty)
	c.Assert(got.CurrentDateTime, check.Not(check.Equals), "")
	c.Assert(got.CurrentDate, check.Not(check.Equals), "")
	c.Assert(got.CurrentTime, check.Not(check.Equals), "")
	c.Assert(got.CurrentTime24, check.Not(check.Equals), "")
}

func (s *ModelsSuite) TestTemplateExecutionWithPhone(c *check.C) {
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
		FromAddress: "From Address <from@example.com>",
	}
	ptx, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)

	// Test that the Phone field is correctly used in template execution
	template := "Hello {{.FirstName}}, your phone number is {{.Phone}}"
	expected := "Hello Foo, your phone number is +15551234567"
	result, err := ExecuteTemplate(template, ptx)
	c.Assert(err, check.Equals, nil)
	c.Assert(result, check.Equals, expected)
}
