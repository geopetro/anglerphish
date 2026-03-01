package models

import (
	"errors"
	"time"
)

// ErrSMSTemplateNameNotSpecified indicates the name for the SMS template was not specified
var ErrSMSTemplateNameNotSpecified = errors.New("SMS template name not specified")

// ErrSMSTemplateTextNotSpecified indicates the text for the SMS template was not specified
var ErrSMSTemplateTextNotSpecified = errors.New("SMS template text not specified")

// SMSTemplate contains the attributes for an SMS template
type SMSTemplate struct {
	Id           int64     `json:"id"`
	UserId       int64     `json:"-"`
	Name         string    `json:"name"`
	From         string    `json:"from" gorm:"column:from_sender"` // Optional sender override
	Text         string    `json:"text"`
	CharCount    int       `json:"char_count"`
	ModifiedDate time.Time `json:"modified_date"`
}

// Validate performs validation on an SMS template
func (s *SMSTemplate) Validate() error {
	if s.Name == "" {
		return ErrSMSTemplateNameNotSpecified
	}
	if s.Text == "" {
		return ErrSMSTemplateTextNotSpecified
	}
	return nil
}

// GetSMSTemplates returns the SMS templates owned by the given user.
func GetSMSTemplates(uid int64) ([]SMSTemplate, error) {
	ts := []SMSTemplate{}
	err := db.Where("user_id=?", uid).Find(&ts).Error
	return ts, err
}

// GetSMSTemplate returns the SMS template, if it exists, specified by the given id and user_id.
func GetSMSTemplate(id int64, uid int64) (SMSTemplate, error) {
	t := SMSTemplate{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&t).Error
	return t, err
}

// GetSMSTemplateByName returns the SMS template, if it exists, specified by the given name and user_id.
func GetSMSTemplateByName(n string, uid int64) (SMSTemplate, error) {
	t := SMSTemplate{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&t).Error
	return t, err
}

// PostSMSTemplate creates a new SMS template in the database.
func PostSMSTemplate(t *SMSTemplate) error {
	// Calculate the character count
	t.CharCount = len(t.Text)
	t.ModifiedDate = time.Now().UTC()
	return db.Save(t).Error
}

// PutSMSTemplate edits an existing SMS template in the database.
// Uses Updates() with a map to allow setting empty string values for the From field.
func PutSMSTemplate(t *SMSTemplate) error {
	// Calculate the character count
	t.CharCount = len(t.Text)
	t.ModifiedDate = time.Now().UTC()
	// Use a map to explicitly set all fields including empty strings
	// GORM's Update(struct) omits zero values, but Updates(map) includes them
	return db.Model(&SMSTemplate{}).Where("id=? and user_id=?", t.Id, t.UserId).Updates(map[string]interface{}{
		"name":          t.Name,
		"from_sender":   t.From,
		"text":          t.Text,
		"char_count":    t.CharCount,
		"modified_date": t.ModifiedDate,
	}).Error
}

// DeleteSMSTemplate deletes an existing SMS template in the database.
func DeleteSMSTemplate(id int64, uid int64) error {
	return db.Delete(SMSTemplate{Id: id, UserId: uid}).Error
}

// DeleteSMSTemplates deletes multiple SMS templates by their IDs for a given user.
func DeleteSMSTemplates(ids []int64, uid int64) error {
	return db.Where("id IN (?) AND user_id = ?", ids, uid).Delete(&SMSTemplate{}).Error
}

// TableName specifies the database tablename for Gorm to use
func (t SMSTemplate) TableName() string {
	return "sms_templates"
}
