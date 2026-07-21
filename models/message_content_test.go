package models

import (
	"net/textproto"
	"strings"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestMessageContentUnderCapIsUntouched(c *check.C) {
	mc := NewMessageContent("hello", "<p>hello</p>", nil)
	c.Assert(mc.Text, check.Equals, "hello")
	c.Assert(mc.HTML, check.Equals, "<p>hello</p>")
	c.Assert(mc.Truncated, check.Equals, false)
}

// HTML is dropped first: for replies what matters is what the user typed,
// which is the plain text part.
func (s *ModelsSuite) TestMessageContentDropsHTMLFirst(c *check.C) {
	text := strings.Repeat("a", 1000)
	html := strings.Repeat("b", MaxMessageContentBytes)

	mc := NewMessageContent(text, html, nil)
	c.Assert(mc.Text, check.Equals, text)
	c.Assert(mc.HTML, check.Equals, "")
	c.Assert(mc.Truncated, check.Equals, true)
}

func (s *ModelsSuite) TestMessageContentTruncatesTextWhenStillOver(c *check.C) {
	text := strings.Repeat("a", MaxMessageContentBytes+5000)

	mc := NewMessageContent(text, "", nil)
	c.Assert(len(mc.Text), check.Equals, MaxMessageContentBytes)
	c.Assert(mc.Truncated, check.Equals, true)
}

func (s *ModelsSuite) TestMessageContentEmptyReturnsNil(c *check.C) {
	c.Assert(NewMessageContent("", "", nil), check.IsNil)
}

func (s *ModelsSuite) TestMessageContentKeepsHeaders(c *check.C) {
	headers := textproto.MIMEHeader{
		"Subject":      []string{"Re: Payroll"},
		"Message-Id":   []string{"<abc@corp.com>"},
		"Received":     []string{"from mx1.corp.com", "from mx2.corp.com"},
		"X-Mailer":     []string{"Outlook"},
		"Content-Type": []string{"text/plain"},
	}

	mc := NewMessageContent("body", "", headers)
	c.Assert(mc.Truncated, check.Equals, false)

	// Sorted by name, and a repeated header keeps every occurrence in order.
	names := []string{}
	for _, h := range mc.Headers {
		names = append(names, h.Name)
	}
	c.Assert(names, check.DeepEquals, []string{
		"Content-Type", "Message-Id", "Received", "Received", "Subject", "X-Mailer",
	})
	c.Assert(mc.Headers[2].Value, check.Equals, "from mx1.corp.com")
	c.Assert(mc.Headers[3].Value, check.Equals, "from mx2.corp.com")
}

// Headers get their own budget so a hostile sender cannot pad the header block
// to evict the body the operator actually needs to read.
func (s *ModelsSuite) TestMessageContentHeadersDoNotEvictBody(c *check.C) {
	headers := textproto.MIMEHeader{
		"X-Padding": []string{strings.Repeat("p", MaxMessageHeaderBytes*2)},
	}

	mc := NewMessageContent("the body", "<p>the body</p>", headers)
	c.Assert(mc.Text, check.Equals, "the body")
	c.Assert(mc.HTML, check.Equals, "<p>the body</p>")
	c.Assert(mc.Headers, check.HasLen, 0)
	c.Assert(mc.Truncated, check.Equals, true)
}

func (s *ModelsSuite) TestMessageContentHeadersTruncatedAtBudget(c *check.C) {
	headers := textproto.MIMEHeader{
		"A-First":  []string{strings.Repeat("a", MaxMessageHeaderBytes/2)},
		"B-Second": []string{strings.Repeat("b", MaxMessageHeaderBytes/2)},
		"C-Third":  []string{strings.Repeat("c", MaxMessageHeaderBytes/2)},
	}

	mc := NewMessageContent("body", "", headers)
	c.Assert(mc.Truncated, check.Equals, true)
	// Kept in sorted order until the budget runs out, never a partial value.
	c.Assert(len(mc.Headers) < 3, check.Equals, true)
	for _, h := range mc.Headers {
		c.Assert(strings.HasPrefix(h.Value, strings.Repeat(strings.ToLower(h.Name[:1]), 10)), check.Equals, true)
	}
}

// Headers alone are still worth storing; only a wholly empty message is nil.
func (s *ModelsSuite) TestMessageContentHeadersOnlyIsStored(c *check.C) {
	headers := textproto.MIMEHeader{"Subject": []string{"Re: Payroll"}}

	mc := NewMessageContent("", "", headers)
	c.Assert(mc, check.NotNil)
	c.Assert(mc.Headers, check.HasLen, 1)
}
