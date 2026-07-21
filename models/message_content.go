package models

import (
	"net/textproto"
	"sort"
)

// MaxMessageContentBytes caps the combined size of stored message content.
// Reply chains accumulate quoted history and inline content; without a cap a
// single large message bloats the events table and is trivially abusable.
const MaxMessageContentBytes = 262144 // 256KB

// MaxMessageHeaderBytes caps the stored header block. Headers are budgeted
// separately from the body rather than sharing its cap, so that a sender who
// pads the header block cannot evict the body an operator needs to read.
const MaxMessageHeaderBytes = 16384 // 16KB

// MessageHeader is a single header occurrence. A slice rather than a map
// because headers legitimately repeat — a Received chain is the whole point of
// looking at them.
type MessageHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MessageContent is captured message body stored alongside an event. It lives
// inside EventDetails, so it inherits the encryption applied to Event.Details.
type MessageContent struct {
	Text      string          `json:"text,omitempty"`
	HTML      string          `json:"html,omitempty"`
	Headers   []MessageHeader `json:"headers,omitempty"`
	Truncated bool            `json:"truncated,omitempty"`
}

// NewMessageContent builds a MessageContent, enforcing the size caps.
// HTML is dropped before text is truncated, because for a reply the operator
// cares what the user wrote, which is the plain text part.
// Returns nil when there is nothing to store.
func NewMessageContent(text, html string, headers textproto.MIMEHeader) *MessageContent {
	stored, headersTruncated := flattenHeaders(headers)

	if text == "" && html == "" && len(stored) == 0 {
		return nil
	}

	mc := &MessageContent{
		Text:      text,
		HTML:      html,
		Headers:   stored,
		Truncated: headersTruncated,
	}

	if len(mc.Text)+len(mc.HTML) <= MaxMessageContentBytes {
		return mc
	}

	mc.HTML = ""
	mc.Truncated = true

	if len(mc.Text) > MaxMessageContentBytes {
		mc.Text = mc.Text[:MaxMessageContentBytes]
	}

	return mc
}

// flattenHeaders turns a MIMEHeader map into an ordered slice within the header
// budget, reporting whether anything was dropped.
//
// textproto.MIMEHeader is a map, so the original wire order is already gone by
// the time we see it. Sorting by name at least makes storage and display
// deterministic instead of varying per Go map iteration.
func flattenHeaders(headers textproto.MIMEHeader) ([]MessageHeader, bool) {
	if len(headers) == 0 {
		return nil, false
	}

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	stored := []MessageHeader{}
	used := 0
	truncated := false
	for _, name := range names {
		for _, value := range headers[name] {
			size := len(name) + len(value)
			if used+size > MaxMessageHeaderBytes {
				// Drop whole headers rather than storing a partial value, which
				// would misrepresent what the message actually carried.
				truncated = true
				continue
			}
			stored = append(stored, MessageHeader{Name: name, Value: value})
			used += size
		}
	}

	return stored, truncated
}
