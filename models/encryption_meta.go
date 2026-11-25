package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/gophish/gophish/crypto"
)

// encryptionCheckPlaintext is the constant encrypted into the check value.
const encryptionCheckPlaintext = "anglerphish-encryption-check-v1"

// EncryptionMeta stores the encrypted check value used to verify that a key
// matches the one the data was encrypted with. The key is never stored.
type EncryptionMeta struct {
	Id         int64     `json:"id" gorm:"primary_key:yes"`
	CheckValue string    `json:"-" gorm:"column:check_value"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName specifies the database tablename for Gorm to use.
func (e EncryptionMeta) TableName() string {
	return "encryption_meta"
}

// writeCheckValue encrypts the check constant with the loaded key, replacing
// any existing check row.
func writeCheckValue() error {
	if !crypto.IsEncryptionEnabled() {
		return errors.New("encryption is not enabled")
	}
	enc, err := crypto.Encrypt(encryptionCheckPlaintext)
	if err != nil {
		return fmt.Errorf("failed to create check value: %w", err)
	}
	tx := db.Begin()
	if err := tx.Delete(EncryptionMeta{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Save(&EncryptionMeta{CheckValue: enc, CreatedAt: time.Now().UTC()}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// deleteCheckValue removes any stored check value.
func deleteCheckValue() error {
	return db.Delete(EncryptionMeta{}).Error
}

// hasCheckValue reports whether a check value is stored.
func hasCheckValue() (bool, error) {
	var n int
	if err := db.Model(&EncryptionMeta{}).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// verifyKey reports whether key matches the data. With a stored check value,
// the key must decrypt it. Without one, an empty set of encrypted values
// accepts any key; otherwise the first encrypted value found must decrypt.
func verifyKey(key string) (bool, error) {
	var metas []EncryptionMeta
	if err := db.Order("id desc").Limit(1).Find(&metas).Error; err != nil {
		return false, err
	}
	if len(metas) > 0 {
		pt, err := crypto.DecryptWithKey(metas[0].CheckValue, key)
		return err == nil && pt == encryptionCheckPlaintext, nil
	}

	sources := []struct{ table, column string }{
		{"smtp", "password"},
		{"sms_profiles", "provider_config"},
		{"imap", "password"},
		{"events", "details"},
		{"oidc_settings", "client_secret"},
		{"smtp", "dmi_secret"},
		{"dmi_settings", "google_key"},
	}
	for _, s := range sources {
		var values []string
		err := db.Table(s.table).
			Where(s.column+" LIKE ?", "ENC:v1:%").
			Limit(1).
			Pluck(s.column, &values).Error
		if err != nil {
			return false, fmt.Errorf("failed to query %s: %w", s.table, err)
		}
		if len(values) > 0 {
			_, err := crypto.DecryptWithKey(values[0], key)
			return err == nil, nil
		}
	}
	return true, nil
}

// countEncryptedValues returns the number of encrypted values across all
// sensitive fields.
func countEncryptedValues() (int, error) {
	r, err := GetEncryptionReport()
	if err != nil {
		return 0, err
	}
	return r.SMTPPasswords.Encrypted + r.SMSConfigs.Encrypted + r.IMAPPasswords.Encrypted + r.EventDetails.Encrypted + r.OIDCSecrets.Encrypted +
		r.DMISecrets.Encrypted + r.DMIGoogleKeys.Encrypted, nil
}

// countPlaintextValues returns the number of plaintext values across all
// sensitive fields.
func countPlaintextValues() (int, error) {
	r, err := GetEncryptionReport()
	if err != nil {
		return 0, err
	}
	return r.SMTPPasswords.Plaintext + r.SMSConfigs.Plaintext + r.IMAPPasswords.Plaintext + r.EventDetails.Plaintext + r.OIDCSecrets.Plaintext +
		r.DMISecrets.Plaintext + r.DMIGoogleKeys.Plaintext, nil
}
