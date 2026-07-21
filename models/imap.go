package models

import (
	"errors"
	"net"
	"time"

	log "github.com/gophish/gophish/logger"
)

const DefaultIMAPFolder = "INBOX"
const DefaultIMAPFreq = 60 // Every 60 seconds

// Tracking types for IMAP monitoring
const (
	TrackingTypeReport = 0
	TrackingTypeReply  = 1
)

// IMAP contains the attributes needed to handle logging into an IMAP server to check
// for reported emails
type IMAP struct {
	Id                          int64     `json:"id" gorm:"primary_key;auto_increment"`
	Name                        string    `json:"name"`
	UserId                      int64     `json:"user_id" gorm:"column:user_id"`
	Enabled                     bool      `json:"enabled"`
	Host                        string    `json:"host"`
	Port                        uint16    `json:"port,string,omitempty"`
	Username                    string    `json:"username"`
	Password                    string    `json:"password"`
	TLS                         bool      `json:"tls"`
	IgnoreCertErrors            bool      `json:"ignore_cert_errors"`
	Folder                      string    `json:"folder"`
	RestrictDomain              string    `json:"restrict_domain"`
	DeleteReportedCampaignEmail bool      `json:"delete_reported_campaign_email"`
	TrackingType                int       `json:"tracking_type"` // 0=report, 1=reply
	CaptureReplyBody            bool      `json:"capture_reply_body" gorm:"column:capture_reply_body"`
	LastLogin                   time.Time `json:"last_login,omitempty"`
	LoginFailures               int       `json:"login_failures"`
	LastLoginError              time.Time `json:"last_login_error,omitempty"`
	ModifiedDate                time.Time `json:"modified_date"`
	IMAPFreq                    uint32    `json:"imap_freq,string,omitempty"`
}

// ErrIMAPHostNotSpecified is thrown when there is no Host specified
// in the IMAP configuration
var ErrIMAPHostNotSpecified = errors.New("No IMAP Host specified")

// ErrIMAPPortNotSpecified is thrown when there is no Port specified
// in the IMAP configuration
var ErrIMAPPortNotSpecified = errors.New("No IMAP Port specified")

// ErrInvalidIMAPHost indicates that the IMAP server string is invalid
var ErrInvalidIMAPHost = errors.New("Invalid IMAP server address")

// ErrInvalidIMAPPort indicates that the IMAP Port is invalid
var ErrInvalidIMAPPort = errors.New("Invalid IMAP Port")

// ErrIMAPUsernameNotSpecified is thrown when there is no Username specified
// in the IMAP configuration
var ErrIMAPUsernameNotSpecified = errors.New("No Username specified")

// ErrIMAPPasswordNotSpecified is thrown when there is no Password specified
// in the IMAP configuration
var ErrIMAPPasswordNotSpecified = errors.New("No Password specified")

// ErrInvalidIMAPFreq is thrown when the frequency for polling the
// IMAP server is invalid
var ErrInvalidIMAPFreq = errors.New("Invalid polling frequency")

// TableName specifies the database tablename for Gorm to use
func (im IMAP) TableName() string {
	return "imap"
}

// Validate ensures that IMAP configs/connections are valid
func (im *IMAP) Validate() error {
	switch {
	case im.Host == "":
		return ErrIMAPHostNotSpecified
	case im.Port == 0:
		return ErrIMAPPortNotSpecified
	case im.Username == "":
		return ErrIMAPUsernameNotSpecified
	case im.Password == "":
		return ErrIMAPPasswordNotSpecified
	}

	// Set the default value for Folder
	if im.Folder == "" {
		im.Folder = DefaultIMAPFolder
	}

	// Make sure im.Host is an IP or hostname. NB will fail if unable to resolve the hostname.
	ip := net.ParseIP(im.Host)
	_, err := net.LookupHost(im.Host)
	if ip == nil && err != nil {
		return ErrInvalidIMAPHost
	}

	// Make sure 1 >= port <= 65535
	if im.Port < 1 || im.Port > 65535 {
		return ErrInvalidIMAPPort
	}

	// Make sure the polling frequency is between every 30 seconds and every year
	// If not set it to the default
	if im.IMAPFreq < 30 || im.IMAPFreq > 31540000 {
		im.IMAPFreq = DefaultIMAPFreq
	}

	return nil
}

// GetIMAP returns all IMAP servers owned by the given user.
func GetIMAP(uid int64) ([]IMAP, error) {
	im := []IMAP{}
	count := 0
	err := db.Where("user_id=?", uid).Find(&im).Count(&count).Error

	if err != nil {
		log.Error(err)
		return im, err
	}
	return im, nil
}

// GetIMAPById returns a specific IMAP configuration by ID for a user.
func GetIMAPById(id int64, uid int64) (IMAP, error) {
	im := IMAP{}
	err := db.Where("id=? AND user_id=?", id, uid).First(&im).Error
	if err != nil {
		log.Error(err)
		return im, err
	}
	return im, nil
}

// PostIMAP creates a new IMAP configuration for a user in the database.
func PostIMAP(im *IMAP, uid int64) error {
	err := im.Validate()
	if err != nil {
		log.Error(err)
		return err
	}

	// Make sure the user ID is set correctly
	im.UserId = uid

	// Add a default name if none provided
	if im.Name == "" {
		im.Name = "IMAP Configuration " + time.Now().Format("2006-01-02 15:04:05")
	}

	// Insert settings into the DB
	err = db.Save(im).Error
	if err != nil {
		log.Error("Unable to save to database: ", err.Error())
	}
	return err
}

// UpdateIMAP updates an existing IMAP configuration in the database.
func UpdateIMAP(im *IMAP, uid int64) error {
	err := im.Validate()
	if err != nil {
		log.Error(err)
		return err
	}

	// Add a signal to force IMAP monitor to recognize the changes right away
	im.ModifiedDate = time.Now().UTC()

	// Ensure the user can only update their own IMAP configurations
	existingIm := IMAP{}
	err = db.Where("id=? AND user_id=?", im.Id, uid).First(&existingIm).Error
	if err != nil {
		log.Errorf("Cannot find IMAP configuration %d for user %d", im.Id, uid)
		return err
	}

	// Log the enabled status change if it's different
	if existingIm.Enabled != im.Enabled {
		log.Infof("IMAP configuration %d enabled status changing from %v to %v", im.Id, existingIm.Enabled, im.Enabled)
	}

	// Update the configuration, keeping the user ID the same
	im.UserId = uid
	im.ModifiedDate = time.Now().UTC()

	// First update specific fields to ensure proper type conversion
	updateMap := map[string]interface{}{
		"name":                           im.Name,
		"enabled":                        im.Enabled,
		"host":                           im.Host,
		"port":                           im.Port,
		"username":                       im.Username,
		"password":                       im.Password,
		"tls":                            im.TLS,
		"ignore_cert_errors":             im.IgnoreCertErrors,
		"folder":                         im.Folder,
		"restrict_domain":                im.RestrictDomain,
		"delete_reported_campaign_email": im.DeleteReportedCampaignEmail,
		"tracking_type":                  im.TrackingType,
		"imap_freq":                      im.IMAPFreq,
		"modified_date":                  im.ModifiedDate,
	}

	err = db.Model(&IMAP{}).Where("id=? AND user_id=?", im.Id, uid).Updates(updateMap).Error
	if err != nil {
		log.Error("Unable to update IMAP configuration: ", err.Error())
		return err
	}

	// Verify the update was successful by retrieving the updated record
	updatedIm := IMAP{}
	err = db.Where("id=? AND user_id=?", im.Id, uid).First(&updatedIm).Error
	if err != nil {
		log.Errorf("Error verifying IMAP update: %v", err)
		return err
	}

	if updatedIm.Enabled != im.Enabled {
		log.Errorf("IMAP enabled status wasn't properly updated! Expected: %v, Got: %v",
			im.Enabled, updatedIm.Enabled)
		return errors.New("failed to update enabled status correctly")
	}

	log.Infof("IMAP configuration %d updated successfully. Enabled: %v", im.Id, updatedIm.Enabled)
	return nil
}

// DeleteIMAPById deletes a specific IMAP configuration by ID.
func DeleteIMAPById(id int64, uid int64) error {
	err := db.Where("id=? AND user_id=?", id, uid).Delete(&IMAP{}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// DeleteIMAP deletes all IMAP configurations for a user (for backwards compatibility).
func DeleteIMAP(uid int64) error {
	err := db.Where("user_id=?", uid).Delete(&IMAP{}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// RecordLoginFailure increments the login failure counter and updates the last login error timestamp
func (im *IMAP) RecordLoginFailure() error {
	im.LoginFailures++
	im.LastLoginError = time.Now().UTC()
	err := db.Model(im).Updates(map[string]interface{}{
		"login_failures":   im.LoginFailures,
		"last_login_error": im.LastLoginError,
	}).Error
	if err != nil {
		log.Error("Unable to update login failure data: ", err.Error())
	}
	return err
}

func SuccessfulLogin(im *IMAP) error {
	// Reset login failures on successful login
	err := db.Model(im).Where("id = ?", im.Id).Updates(map[string]interface{}{
		"last_login":     time.Now().UTC(),
		"login_failures": 0, // Reset failures counter on successful login
	}).Error
	if err != nil {
		log.Error("Unable to update database: ", err.Error())
	}
	return err
}

// BeforeSave is a GORM hook that encrypts the password before saving to the database
func (im *IMAP) BeforeSave() error {
	if im.Password != "" {
		encrypted, err := encryptField(im.Password)
		if err != nil {
			log.Warnf("Failed to encrypt IMAP password: %v", err)
			// Continue without encryption rather than failing
			return nil
		}
		im.Password = encrypted
	}
	return nil
}

// AfterFind is a GORM hook that decrypts the password after reading from the database
func (im *IMAP) AfterFind() error {
	if im.Password != "" {
		decrypted, err := decryptField(im.Password)
		if err != nil {
			log.Warnf("Failed to decrypt IMAP password: %v", err)
			// Return original value if decryption fails
			return nil
		}
		im.Password = decrypted
	}
	return nil
}
