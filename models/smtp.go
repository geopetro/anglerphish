package models

import (
	"context"
	"crypto/tls"
	"errors"
	"net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gophish/gomail"
	"github.com/gophish/gophish/dialer"
	"github.com/gophish/gophish/dmi"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/jinzhu/gorm"
)

// Dialer is a wrapper around a standard gomail.Dialer in order
// to implement the mailer.Dialer interface. This allows us to better
// separate the mailer package as opposed to forcing a connection
// between mailer and gomail.
type Dialer struct {
	*gomail.Dialer
}

// Dial wraps the gomail dialer's Dial command
func (d *Dialer) Dial() (mailer.Sender, error) {
	return d.Dialer.Dial()
}

// SMTP contains the attributes needed to handle the sending of campaign emails
type SMTP struct {
	Id               int64     `json:"id" gorm:"column:id; primary_key:yes"`
	UserId           int64     `json:"-" gorm:"column:user_id"`
	Interface        string    `json:"interface_type" gorm:"column:interface_type"`
	Name             string    `json:"name"`
	Host             string    `json:"host"`
	Username         string    `json:"username,omitempty"`
	Password         string    `json:"password,omitempty"`
	FromAddress      string    `json:"from_address"`
	IgnoreCertErrors bool      `json:"ignore_cert_errors"`
	Headers          []Header  `json:"headers"`
	ModifiedDate     time.Time `json:"modified_date"`

	DMIAuthMethod     string `json:"dmi_auth_method" gorm:"column:dmi_auth_method"`
	DMITenantID       string `json:"dmi_tenant_id" gorm:"column:dmi_tenant_id"`
	DMIClientID       string `json:"dmi_client_id" gorm:"column:dmi_client_id"`
	DMISecret         string `json:"-" gorm:"column:dmi_secret"`
	DMISecretInput    string `json:"dmi_secret,omitempty" gorm:"-"`
	DMISecretSet      bool   `json:"dmi_secret_set" gorm:"-"`
	DMICertificate    string `json:"dmi_certificate,omitempty" gorm:"column:dmi_certificate"` // public part only
	DMIAllowedDomains string `json:"dmi_allowed_domains" gorm:"column:dmi_allowed_domains"`
	DMIFolder         string `json:"dmi_folder" gorm:"column:dmi_folder"`
	DMIFolderName     string `json:"dmi_folder_name" gorm:"column:dmi_folder_name"`
}

// Sending profile types.
const (
	InterfaceSMTP     = "SMTP"
	InterfaceMS365DMI = dmi.ProviderMicrosoft
	InterfaceGWSDMI   = dmi.ProviderGoogle
)

// Header contains the fields and methods for a sending profile to have
// custom headers
type Header struct {
	Id     int64  `json:"-"`
	SMTPId int64  `json:"-"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// ErrFromAddressNotSpecified is thrown when there is no "From" address
// specified in the SMTP configuration
var ErrFromAddressNotSpecified = errors.New("No From Address specified")

// ErrInvalidFromAddress is thrown when the SMTP From field in the sending
// profiles containes a value that is not an email address
var ErrInvalidFromAddress = errors.New("Invalid SMTP From address because it is not an email address")

// ErrHostNotSpecified is thrown when there is no Host specified
// in the SMTP configuration
var ErrHostNotSpecified = errors.New("No SMTP Host specified")

// ErrInvalidHost indicates that the SMTP server string is invalid
var ErrInvalidHost = errors.New("Invalid SMTP server address")

// TableName specifies the database tablename for Gorm to use
func (s SMTP) TableName() string {
	return "smtp"
}

// Validate ensures that SMTP configs/connections are valid
func (s *SMTP) Validate() error {
	if s.IsDMI() {
		return s.validateDMI()
	}
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case s.Host == "":
		return ErrHostNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	}
	_, err := mail.ParseAddress(s.FromAddress)
	if err != nil {
		return err
	}
	// Make sure addr is in host:port format
	hp := strings.Split(s.Host, ":")
	if len(hp) > 2 {
		return ErrInvalidHost
	} else if len(hp) < 2 {
		hp = append(hp, "25")
	}
	_, err = strconv.Atoi(hp[1])
	if err != nil {
		return ErrInvalidHost
	}
	return err
}

// validateFromAddress validates
func validateFromAddress(email string) bool {
	r, _ := regexp.Compile("^([a-zA-Z0-9_\\-\\.]+)@([a-zA-Z0-9_\\-\\.]+)\\.([a-zA-Z]{2,18})$")
	return r.MatchString(email)
}

// GetDialer returns a dialer for the given SMTP profile
func (s *SMTP) GetDialer() (mailer.Dialer, error) {
	if s.IsDMI() {
		return s.dmiDialer()
	}
	if err := RequirePlaintextCredential(s.Password); err != nil {
		return nil, err
	}
	// Setup the message and dial
	hp := strings.Split(s.Host, ":")
	if len(hp) < 2 {
		hp = append(hp, "25")
	}
	host := hp[0]
	// Any issues should have been caught in validation, but we'll
	// double check here.
	port, err := strconv.Atoi(hp[1])
	if err != nil {
		log.Error(err)
		return nil, err
	}
	dialer := dialer.Dialer()
	d := gomail.NewWithDialer(dialer, host, port, s.Username, s.Password)
	d.TLSConfig = &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: s.IgnoreCertErrors,
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Error(err)
		hostname = "localhost"
	}
	d.LocalName = hostname
	return &Dialer{d}, err
}

// GetSMTPs returns the SMTPs owned by the given user.
func GetSMTPs(uid int64) ([]SMTP, error) {
	ss := []SMTP{}
	err := db.Where("user_id=?", uid).Find(&ss).Error
	if err != nil {
		log.Error(err)
		return ss, err
	}
	for i := range ss {
		err = db.Where("smtp_id=?", ss[i].Id).Find(&ss[i].Headers).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			log.Error(err)
			return ss, err
		}
	}
	return ss, nil
}

// GetSMTP returns the SMTP, if it exists, specified by the given id and user_id.
func GetSMTP(id int64, uid int64) (SMTP, error) {
	s := SMTP{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	err = db.Where("smtp_id=?", s.Id).Find(&s.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
		return s, err
	}
	return s, err
}

// GetSMTPByName returns the SMTP, if it exists, specified by the given name and user_id.
func GetSMTPByName(n string, uid int64) (SMTP, error) {
	s := SMTP{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	err = db.Where("smtp_id=?", s.Id).Find(&s.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
	}
	return s, err
}

// PostSMTP creates a new SMTP in the database.
func PostSMTP(s *SMTP) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	if err := s.prepareDMISecret(nil); err != nil {
		log.Error(err)
		return err
	}
	// Insert into the DB
	err = db.Save(s).Error
	if err != nil {
		log.Error(err)
	}
	// Save custom headers
	for i := range s.Headers {
		s.Headers[i].SMTPId = s.Id
		err := db.Save(&s.Headers[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return err
}

// PutSMTP edits an existing SMTP in the database.
// Per the PUT Method RFC, it presumes all data for a SMTP is provided.
func PutSMTP(s *SMTP) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	stored := &SMTP{}
	if s.IsDMI() {
		var prev []SMTP
		if err := db.Where("id=? AND user_id=?", s.Id, s.UserId).Limit(1).Find(&prev).Error; err != nil {
			log.Error(err)
			return err
		}
		if len(prev) > 0 {
			stored = &prev[0]
		}
	}
	if err := s.prepareDMISecret(stored); err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("id=?", s.Id).Save(s).Error
	if err != nil {
		log.Error(err)
	}
	// Delete all custom headers, and replace with new ones
	err = db.Where("smtp_id=?", s.Id).Delete(&Header{}).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
		return err
	}
	// Save custom headers
	for i := range s.Headers {
		s.Headers[i].SMTPId = s.Id
		err := db.Save(&s.Headers[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return err
}

// DeleteSMTP deletes an existing SMTP in the database.
// An error is returned if a SMTP with the given user id and SMTP id is not found.
func DeleteSMTP(id int64, uid int64) error {
	// Delete all custom headers
	err := db.Where("smtp_id=?", id).Delete(&Header{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("user_id=?", uid).Delete(SMTP{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// BeforeSave is a GORM hook that encrypts the password and DMI secret before saving to the database
func (s *SMTP) BeforeSave() error {
	if s.Password != "" {
		encrypted, err := encryptSMTPPassword(s.Password)
		if err != nil {
			log.Warnf("Failed to encrypt SMTP password: %v", err)
		} else {
			s.Password = encrypted
		}
	}
	if s.DMISecret != "" {
		encrypted, err := encryptField(s.DMISecret)
		if err != nil {
			log.Warnf("Failed to encrypt DMI secret: %v", err)
		} else {
			s.DMISecret = encrypted
		}
	}
	return nil
}

// AfterFind is a GORM hook that decrypts the password and DMI secret after reading from the database
func (s *SMTP) AfterFind() error {
	if s.Password != "" {
		decrypted, err := decryptSMTPPassword(s.Password)
		if err != nil {
			log.Warnf("Failed to decrypt SMTP password: %v", err)
		} else {
			s.Password = decrypted
		}
	}
	if s.DMISecret != "" {
		decrypted, err := decryptField(s.DMISecret)
		if err != nil {
			log.Warnf("Failed to decrypt DMI secret: %v", err)
		} else {
			s.DMISecret = decrypted
		}
	}
	s.DMISecretSet = s.DMISecret != ""
	return nil
}

// encryptSMTPPassword encrypts a password using the crypto package
// This is a wrapper to avoid import cycles
func encryptSMTPPassword(password string) (string, error) {
	// Import crypto package dynamically to avoid circular imports
	// The crypto package handles the case where encryption is not enabled
	return encryptField(password)
}

// decryptSMTPPassword decrypts a password using the crypto package
// This is a wrapper to avoid import cycles
func decryptSMTPPassword(password string) (string, error) {
	return decryptField(password)
}

// IsDMI reports whether the profile delivers by Direct Mail Injection.
func (s *SMTP) IsDMI() bool {
	return s.Interface == InterfaceMS365DMI || s.Interface == InterfaceGWSDMI
}

func (s *SMTP) baseDMIConfig() dmi.Config {
	var domains []string
	for _, d := range strings.Split(s.DMIAllowedDomains, ",") {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	folder := s.DMIFolder
	if folder == "" {
		folder = dmi.FolderInbox
	}
	return dmi.Config{
		Provider:       s.Interface,
		AuthMethod:     s.DMIAuthMethod,
		TenantID:       s.DMITenantID,
		ClientID:       s.DMIClientID,
		Secret:         s.DMISecret,
		Certificate:    s.DMICertificate,
		AllowedDomains: domains,
		Folder:         folder,
		FolderName:     s.DMIFolderName,
	}
}

// DMIConfig builds the delivery config, loading the operator key for
// operator_key profiles.
func (s *SMTP) DMIConfig() (dmi.Config, error) {
	cfg := s.baseDMIConfig()
	if s.DMIAuthMethod == dmi.AuthOperatorKey {
		key, err := operatorGoogleKey()
		if err != nil {
			return cfg, err
		}
		cfg.OperatorKey = key
	}
	return cfg, nil
}

func (s *SMTP) validateDMI() error {
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	}
	if _, err := mail.ParseAddress(s.FromAddress); err != nil {
		return err
	}
	cfg := s.baseDMIConfig()
	switch s.DMIAuthMethod {
	case dmi.AuthCertificate:
		// The key pair is generated on save, so it is never required as input.
		cfg.Secret, cfg.Certificate = "generated", "generated"
		if s.Id != 0 {
			lost, err := dmiCertificateKeyLost(s.Id, s.UserId)
			if err != nil {
				return err
			}
			if lost {
				return ErrDMICertificateKeyLost
			}
		}
	case dmi.AuthOperatorKey:
		st, err := GetDMISettings()
		if err != nil {
			return err
		}
		cfg.OperatorKey = st.GoogleKey
	default:
		if s.DMIAuthMethod == dmi.AuthCustomerKey && s.DMISecretInput != "" {
			if _, err := parseGoogleServiceAccount(s.DMISecretInput); err != nil {
				return err
			}
		}
		if s.DMISecretInput != "" || s.DMISecret != "" {
			cfg.Secret = "set"
		} else if s.Id != 0 {
			var stored []string
			err := db.Table("smtp").Where("id=? AND user_id=? AND dmi_auth_method=? AND dmi_secret<>''", s.Id, s.UserId, s.DMIAuthMethod).Pluck("dmi_secret", &stored).Error
			if err != nil {
				return err
			}
			if len(stored) > 0 {
				cfg.Secret = "set"
			}
		}
	}
	return cfg.Validate()
}

// prepareDMISecret settles the DMI secret before a save. prev is the stored
// row on update and nil on create.
func (s *SMTP) prepareDMISecret(prev *SMTP) error {
	if !s.IsDMI() {
		s.DMISecretInput = ""
		return nil
	}
	sameMethod := prev != nil && prev.DMIAuthMethod == s.DMIAuthMethod
	switch s.DMIAuthMethod {
	case dmi.AuthCertificate:
		if sameMethod && prev.DMISecret != "" {
			s.DMISecret, s.DMICertificate = prev.DMISecret, prev.DMICertificate
		} else {
			cert, key, err := dmi.GenerateCertificate(s.Name)
			if err != nil {
				return err
			}
			s.DMISecret, s.DMICertificate = key, cert
		}
	case dmi.AuthOperatorKey:
		s.DMISecret = ""
	default:
		switch {
		case s.DMISecretInput != "":
			s.DMISecret = s.DMISecretInput
		case sameMethod:
			s.DMISecret = prev.DMISecret
		case prev != nil:
			s.DMISecret = ""
		}
	}
	if s.DMIAuthMethod != dmi.AuthCertificate {
		s.DMICertificate = ""
	}
	s.DMISecretInput = ""
	s.DMISecretSet = s.DMISecret != ""
	return nil
}

// ErrDMICertificateKeyLost is returned when a saved certificate profile lost its
// private key, for example after encrypted data was discarded. Saving would
// silently issue a new certificate that the customer has not uploaded.
var ErrDMICertificateKeyLost = errors.New("The private key of this profile's certificate was removed. Use Regenerate certificate, then send the new certificate to the customer")

// dmiCertificateKeyLost reports whether a stored certificate profile still has its certificate but no key.
func dmiCertificateKeyLost(id, uid int64) (bool, error) {
	var n int
	err := db.Table("smtp").Where("id=? AND user_id=? AND dmi_auth_method=? AND dmi_certificate<>'' AND dmi_secret=''",
		id, uid, dmi.AuthCertificate).Count(&n).Error
	return n > 0, err
}

// ErrDMITestNeedsSavedCertificate is returned when a test email is sent from a
// certificate profile that has no stored key pair yet.
var ErrDMITestNeedsSavedCertificate = errors.New("Save the profile and upload its certificate to Entra before sending a test email")

// PrepareDMITest settles the credentials of a DMI profile submitted with a
// test email. Stored credentials are read only from the user's own profile,
// matched by id.
func (s *SMTP) PrepareDMITest(uid int64) error {
	s.UserId = uid
	var prev *SMTP
	// Only an explicit id reuses a stored secret, so an unsaved form never borrows one by name.
	if s.Id != 0 {
		var rows []SMTP
		if err := db.Where("user_id=? AND id=?", uid, s.Id).Limit(1).Find(&rows).Error; err != nil {
			log.Error(err)
			return err
		}
		if len(rows) > 0 {
			prev = &rows[0]
		}
	}
	s.Id = 0
	if prev != nil {
		s.Id = prev.Id
	}
	if s.DMIAuthMethod == dmi.AuthCertificate && (prev == nil || prev.DMIAuthMethod != dmi.AuthCertificate || prev.DMISecret == "") {
		return ErrDMITestNeedsSavedCertificate
	}
	if err := s.Validate(); err != nil {
		return err
	}
	return s.prepareDMISecret(prev)
}

func (s *SMTP) dmiDialer() (mailer.Dialer, error) {
	if err := RequirePlaintextCredential(s.DMISecret); err != nil {
		return nil, err
	}
	cfg, err := s.DMIConfig()
	if err != nil {
		return nil, err
	}
	return dmi.NewDialer(cfg)
}

// RegenerateDMICertificate replaces the key pair of a certificate profile.
func RegenerateDMICertificate(id, uid int64) (SMTP, error) {
	s, err := GetSMTP(id, uid)
	if err != nil {
		return s, err
	}
	if !s.IsDMI() || s.DMIAuthMethod != dmi.AuthCertificate {
		return s, errors.New("Only certificate profiles have a certificate to regenerate")
	}
	cert, key, err := dmi.GenerateCertificate(s.Name)
	if err != nil {
		return s, err
	}
	enc, err := encryptField(key)
	if err != nil {
		return s, err
	}
	now := time.Now().UTC()
	err = db.Table("smtp").Where("id=? AND user_id=?", id, uid).Updates(map[string]interface{}{
		"dmi_secret":      enc,
		"dmi_certificate": cert,
		"modified_date":   now,
	}).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	s.DMISecret, s.DMICertificate, s.DMISecretSet, s.ModifiedDate = key, cert, true, now
	return s, nil
}

// TestDMIConnection checks that a DMI profile can reach the given mailbox.
func TestDMIConnection(id, uid int64, mailbox string) error {
	s, err := GetSMTP(id, uid)
	if err != nil {
		return err
	}
	if !s.IsDMI() {
		return errors.New("Only Direct Injection profiles can be tested this way")
	}
	if err := RequirePlaintextCredential(s.DMISecret); err != nil {
		return err
	}
	cfg, err := s.DMIConfig()
	if err != nil {
		return err
	}
	return dmi.TestConnection(context.Background(), cfg, mailbox)
}
