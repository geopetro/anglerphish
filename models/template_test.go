package models

import (
	"encoding/base64"
	"strings"

	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestTemplateTrackAttachmentsPersists(ch *check.C) {
	t := Template{Name: "Track Attachments Template", UserId: 1}
	t.Subject = "s"
	t.Text = "t"
	t.TrackAttachments = true
	ch.Assert(PostTemplate(&t), check.Equals, nil)

	got, err := GetTemplate(t.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.TrackAttachments, check.Equals, true)
}

func (s *ModelsSuite) TestTemplateValidateNamesBadAttachment(ch *check.C) {
	t := Template{Name: "Bad Attachment Template", UserId: 1}
	t.Subject = "s"
	t.Text = "t"
	t.Attachments = []Attachment{{
		Name:    "broken.docx",
		Content: base64.StdEncoding.EncodeToString([]byte("this is not a zip")),
	}}
	err := t.Validate()
	ch.Assert(err, check.NotNil)
	ch.Assert(strings.Contains(err.Error(), "broken.docx"), check.Equals, true)
}
