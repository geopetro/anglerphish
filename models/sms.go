package models

import (
	"encoding/json"
	"errors"
	"time"

	log "github.com/gophish/gophish/logger"
)

// SMS contains the attributes needed to handle the sending of campaign SMS messages
type SMS struct {
	Id             int64     `json:"id" gorm:"column:id; primary_key:yes"`
	UserId         int64     `json:"-" gorm:"column:user_id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"` // "twilio", "nexmo", etc.
	From           string    `json:"from"`     // Sender phone number/ID
	ProviderConfig string    `json:"provider_config"`
	ModifiedDate   time.Time `json:"modified_date"`
}

// Provider-specific configuration structs
type TwilioConfig struct {
	AccountSid string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
}

type NexmoConfig struct {
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
}

// ErrFromNotSpecified is thrown when there is no "From" number
// specified in the SMS configuration
var ErrFromNotSpecified = errors.New("No From number specified")

// ErrProviderNotSpecified is thrown when there is no Provider specified
// in the SMS configuration
var ErrProviderNotSpecified = errors.New("No SMS Provider specified")

// ErrProviderConfigNotSpecified is thrown when there is no Provider Config specified
// in the SMS configuration
var ErrProviderConfigNotSpecified = errors.New("No Provider Configuration specified")

// TableName specifies the database tablename for Gorm to use
func (s SMS) TableName() string {
	return "sms_profiles"
}

// Validate ensures that SMS configs/connections are valid
func (s *SMS) Validate() error {
	switch {
	case s.From == "":
		return ErrFromNotSpecified
	case s.Provider == "":
		return ErrProviderNotSpecified
	case s.ProviderConfig == "":
		return ErrProviderConfigNotSpecified
	}

	// Validate provider-specific configuration
	switch s.Provider {
	case "twilio":
		var config TwilioConfig
		err := json.Unmarshal([]byte(s.ProviderConfig), &config)
		if err != nil {
			return err
		}
		if config.AccountSid == "" || config.AuthToken == "" {
			return errors.New("Twilio Account SID and Auth Token are required")
		}
	case "nexmo":
		var config NexmoConfig
		err := json.Unmarshal([]byte(s.ProviderConfig), &config)
		if err != nil {
			return err
		}
		if config.ApiKey == "" || config.ApiSecret == "" {
			return errors.New("Nexmo API Key and API Secret are required")
		}
	default:
		return errors.New("Unsupported SMS provider")
	}

	return nil
}

// GetSMSs returns the SMS profiles owned by the given user.
func GetSMSs(uid int64) ([]SMS, error) {
	ss := []SMS{}
	err := db.Where("user_id=?", uid).Find(&ss).Error
	if err != nil {
		log.Error(err)
		return ss, err
	}
	return ss, nil
}

// GetSMS returns the SMS profile, if it exists, specified by the given id and user_id.
func GetSMS(id int64, uid int64) (SMS, error) {
	s := SMS{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	return s, err
}

// GetSMSByName returns the SMS profile, if it exists, specified by the given name and user_id.
func GetSMSByName(n string, uid int64) (SMS, error) {
	s := SMS{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	return s, err
}

// PostSMS creates a new SMS profile in the database.
func PostSMS(s *SMS) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	// Insert into the DB
	err = db.Save(s).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// PutSMS edits an existing SMS profile in the database.
func PutSMS(s *SMS) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("id=?", s.Id).Save(s).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// DeleteSMS deletes an existing SMS profile in the database.
// An error is returned if a SMS profile with the given user id and SMS id is not found.
func DeleteSMS(id int64, uid int64) error {
	err := db.Where("user_id=?", uid).Delete(SMS{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// BeforeSave is a GORM hook that encrypts the provider config before saving to the database
func (s *SMS) BeforeSave() error {
	if s.ProviderConfig != "" {
		encrypted, err := encryptField(s.ProviderConfig)
		if err != nil {
			log.Warnf("Failed to encrypt SMS provider config: %v", err)
			// Continue without encryption rather than failing
			return nil
		}
		s.ProviderConfig = encrypted
	}
	return nil
}

// AfterFind is a GORM hook that decrypts the provider config after reading from the database
func (s *SMS) AfterFind() error {
	if s.ProviderConfig != "" {
		decrypted, err := decryptField(s.ProviderConfig)
		if err != nil {
			log.Warnf("Failed to decrypt SMS provider config: %v", err)
			// Return original value if decryption fails
			return nil
		}
		s.ProviderConfig = decrypted
	}
	return nil
}
