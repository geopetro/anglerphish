package models

import (
	"errors"
	"fmt"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
)

// SMSPreviewPrefix is the standard prefix added to the rid parameter when sending
// test SMS messages.
const SMSPreviewPrefix = "sms-preview-"

// SMSRequest is the structure of a request
// to send a test SMS to test an SMS provider connection.
// This type implements the mailer.SMSMail interface.
type SMSRequest struct {
	Id            int64        `json:"-"`
	SMSTemplate   SMSTemplate  `json:"sms_template"`
	SMSTemplateId int64        `json:"-"`
	Page          Page         `json:"page"`
	PageId        int64        `json:"-"`
	SMS           SMS          `json:"sms"`
	SMSId         int64        `json:"-"`
	URL           string       `json:"url"`
	Tracker       string       `json:"tracker" gorm:"-"`
	TrackingURL   string       `json:"tracking_url" gorm:"-"`
	UserId        int64        `json:"-"`
	ErrorChan     chan (error) `json:"-" gorm:"-"`
	RId           string       `json:"id"`
	BaseRecipient
}

// TableName specifies the database tablename for Gorm to use
func (s SMSRequest) TableName() string {
	return "sms_requests"
}

// Implementation of the TemplateContext interface
func (s *SMSRequest) getBaseURL() string {
	return s.URL
}

func (s *SMSRequest) getFromAddress() string {
	return s.SMS.From
}

func (s *SMSRequest) getQRSize() string {
	return ""
}

// Validate ensures the SMSRequest structure is valid.
func (s *SMSRequest) Validate() error {
	switch {
	case s.Phone == "":
		return errors.New("No phone number specified")
	case s.SMS.From == "":
		return errors.New("No from number specified")
	}
	return nil
}

// Backoff treats temporary errors as permanent since this is expected to be a
// synchronous operation. It returns any errors given back to the ErrorChan
func (s *SMSRequest) Backoff(reason error) error {
	s.ErrorChan <- reason
	return nil
}

// Error returns an error on the ErrorChan.
func (s *SMSRequest) Error(err error) error {
	s.ErrorChan <- err
	return nil
}

// Success returns nil on the ErrorChan to indicate that the SMS was sent
// successfully.
func (s *SMSRequest) Success() error {
	s.ErrorChan <- nil
	return nil
}

// Generate returns the generated SMS message as a string
func (s *SMSRequest) Generate() (string, error) {
	// For SMS requests, ensure the Phone field is set correctly
	// Phone numbers should always be stored in the Phone field

	// Create the template context using the SMS-specific context
	stx, err := NewSMSTemplateContext(s, s.BaseRecipient, s.RId)
	if err != nil {
		return "", err
	}

	url, err := ExecuteTemplate(s.URL, stx)
	if err != nil {
		return "", err
	}
	s.URL = url

	// Parse the SMS template
	text, err := ExecuteTemplate(s.SMSTemplate.Text, stx)
	if err != nil {
		log.Error(err)
		return "", err
	}

	return text, nil
}

// GetDialer returns a mailer.SMSDialer based on the SMS profile
func (s *SMSRequest) GetDialer() (mailer.SMSDialer, error) {
	dialer, err := mailer.GetSMSDialer(s.SMS.Provider, s.SMS.ProviderConfig)
	if err != nil {
		return nil, err
	}
	return dialer, nil
}

// GetSMSFrom returns the SMS sender's phone number
func (s *SMSRequest) GetSMSFrom() (string, error) {
	return s.SMS.From, nil
}

// GetSMSTo returns the recipient's phone number
func (s *SMSRequest) GetSMSTo() (string, error) {
	return s.Phone, nil
}

// PostSMSRequest stores a SMSRequest in the database.
func PostSMSRequest(s *SMSRequest) error {
	// Generate an ID to be used in the underlying Result object
	rid, err := generateResultId()
	if err != nil {
		return err
	}
	s.RId = fmt.Sprintf("%s%s", SMSPreviewPrefix, rid)
	return db.Save(&s).Error
}

// GetSMSRequestByResultId retrieves the SMSRequest by the underlying rid
// parameter.
func GetSMSRequestByResultId(id string) (SMSRequest, error) {
	s := SMSRequest{}
	err := db.Table("sms_requests").Where("r_id=?", id).First(&s).Error
	return s, err
}
