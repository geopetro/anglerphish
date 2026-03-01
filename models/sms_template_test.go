package models

import (
	"time"

	"github.com/jinzhu/gorm"

	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestPostSMSTemplate(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template with {{.FirstName}} and {{.URL}} sent to {{.Phone}}",
		UserId: 1,
	}
	err := PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)
	c.Assert(template.CharCount, check.Equals, len(template.Text))
	c.Assert(template.ModifiedDate, check.Not(check.Equals), time.Time{})

	ts, err := GetSMSTemplates(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ts), check.Equals, 1)
	c.Assert(ts[0].Name, check.Equals, template.Name)
	c.Assert(ts[0].Text, check.Equals, template.Text)
	c.Assert(ts[0].CharCount, check.Equals, template.CharCount)
}

func (s *ModelsSuite) TestPostSMSTemplateNoName(c *check.C) {
	template := SMSTemplate{
		Text:   "This is a test SMS template",
		UserId: 1,
	}
	err := template.Validate()
	c.Assert(err, check.Equals, ErrSMSTemplateNameNotSpecified)
}

func (s *ModelsSuite) TestPostSMSTemplateNoText(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		UserId: 1,
	}
	err := template.Validate()
	c.Assert(err, check.Equals, ErrSMSTemplateTextNotSpecified)
}

func (s *ModelsSuite) TestGetSMSTemplate(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template",
		UserId: 1,
	}
	err := PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	t, err := GetSMSTemplate(template.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(t.Name, check.Equals, template.Name)
	c.Assert(t.Text, check.Equals, template.Text)
	c.Assert(t.CharCount, check.Equals, template.CharCount)
}

func (s *ModelsSuite) TestGetSMSTemplateByName(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template",
		UserId: 1,
	}
	err := PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	t, err := GetSMSTemplateByName(template.Name, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(t.Name, check.Equals, template.Name)
	c.Assert(t.Text, check.Equals, template.Text)
	c.Assert(t.CharCount, check.Equals, template.CharCount)
}

func (s *ModelsSuite) TestGetInvalidSMSTemplate(c *check.C) {
	_, err := GetSMSTemplate(-1, 1)
	c.Assert(err, check.Not(check.Equals), nil)
}

func (s *ModelsSuite) TestPutSMSTemplate(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template",
		UserId: 1,
	}
	err := PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	template.Name = "Updated SMS Template"
	template.Text = "This is an updated SMS template"
	err = PutSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)
	c.Assert(template.CharCount, check.Equals, len(template.Text))

	t, err := GetSMSTemplate(template.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(t.Name, check.Equals, "Updated SMS Template")
	c.Assert(t.Text, check.Equals, "This is an updated SMS template")
	c.Assert(t.CharCount, check.Equals, len(t.Text))
}

func (s *ModelsSuite) TestDeleteSMSTemplate(c *check.C) {
	template := SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template",
		UserId: 1,
	}
	err := PostSMSTemplate(&template)
	c.Assert(err, check.Equals, nil)

	err = DeleteSMSTemplate(template.Id, 1)
	c.Assert(err, check.Equals, nil)

	_, err = GetSMSTemplate(template.Id, 1)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}
