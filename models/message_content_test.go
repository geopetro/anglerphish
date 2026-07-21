package models

import (
	"strings"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestMessageContentUnderCapIsUntouched(c *check.C) {
	mc := NewMessageContent("hello", "<p>hello</p>")
	c.Assert(mc.Text, check.Equals, "hello")
	c.Assert(mc.HTML, check.Equals, "<p>hello</p>")
	c.Assert(mc.Truncated, check.Equals, false)
}

// HTML is dropped first: for replies what matters is what the user typed,
// which is the plain text part.
func (s *ModelsSuite) TestMessageContentDropsHTMLFirst(c *check.C) {
	text := strings.Repeat("a", 1000)
	html := strings.Repeat("b", MaxMessageContentBytes)

	mc := NewMessageContent(text, html)
	c.Assert(mc.Text, check.Equals, text)
	c.Assert(mc.HTML, check.Equals, "")
	c.Assert(mc.Truncated, check.Equals, true)
}

func (s *ModelsSuite) TestMessageContentTruncatesTextWhenStillOver(c *check.C) {
	text := strings.Repeat("a", MaxMessageContentBytes+5000)

	mc := NewMessageContent(text, "")
	c.Assert(len(mc.Text), check.Equals, MaxMessageContentBytes)
	c.Assert(mc.Truncated, check.Equals, true)
}

func (s *ModelsSuite) TestMessageContentEmptyReturnsNil(c *check.C) {
	c.Assert(NewMessageContent("", ""), check.IsNil)
}
