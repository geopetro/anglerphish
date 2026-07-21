package models

// MaxMessageContentBytes caps the combined size of stored message content.
// Reply chains accumulate quoted history and inline content; without a cap a
// single large message bloats the events table and is trivially abusable.
const MaxMessageContentBytes = 262144 // 256KB

// MessageContent is captured message body stored alongside an event. It lives
// inside EventDetails, so it inherits the encryption applied to Event.Details.
type MessageContent struct {
	Text      string `json:"text,omitempty"`
	HTML      string `json:"html,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// NewMessageContent builds a MessageContent, enforcing the size cap.
// HTML is dropped before text is truncated, because for a reply the operator
// cares what the user wrote, which is the plain text part.
// Returns nil when there is nothing to store.
func NewMessageContent(text, html string) *MessageContent {
	if text == "" && html == "" {
		return nil
	}

	mc := &MessageContent{Text: text, HTML: html}

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
