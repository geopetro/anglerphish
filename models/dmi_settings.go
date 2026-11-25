package models

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"sync"
	"time"
)

var dmiSettingsMu sync.Mutex

// DMISettings is the single row of operator Direct Injection settings.
// GoogleKey holds the stored value, which is ciphertext when encryption is on.
type DMISettings struct {
	Id        int64     `json:"-" gorm:"column:id; primary_key:yes"`
	GoogleKey string    `json:"-" gorm:"column:google_key"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the database tablename for Gorm to use.
func (DMISettings) TableName() string {
	return "dmi_settings"
}

type googleServiceAccount struct {
	ClientEmail string `json:"client_email"`
	ClientID    string `json:"client_id"`
	PrivateKey  string `json:"private_key"`
}

func parseGoogleServiceAccount(keyJSON string) (googleServiceAccount, error) {
	var sa googleServiceAccount
	if err := json.Unmarshal([]byte(keyJSON), &sa); err != nil || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return sa, errors.New("Service account key is not valid JSON with client_email and private_key")
	}
	blk, _ := pem.Decode([]byte(sa.PrivateKey))
	if blk == nil {
		return sa, errors.New("The service account key is invalid")
	}
	if _, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err != nil {
		if _, err := x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
			return sa, errors.New("The service account key is invalid")
		}
	}
	return sa, nil
}

// GetDMISettings returns the stored settings, or an empty row when none
// have been saved. GoogleKey is returned as stored.
func GetDMISettings() (DMISettings, error) {
	var rows []DMISettings
	if err := db.Order("id asc").Limit(1).Find(&rows).Error; err != nil {
		return DMISettings{}, err
	}
	if len(rows) == 0 {
		return DMISettings{}, nil
	}
	return rows[0], nil
}

// SaveDMIGoogleKey validates and stores the operator Google service account
// key. An empty key clears it.
func SaveDMIGoogleKey(keyJSON string) error {
	keyJSON = strings.TrimSpace(keyJSON)
	stored := ""
	if keyJSON != "" {
		if _, err := parseGoogleServiceAccount(keyJSON); err != nil {
			return err
		}
		enc, err := encryptField(keyJSON)
		if err != nil {
			return errors.New("Failed to protect the service account key")
		}
		stored = enc
	}

	dmiSettingsMu.Lock()
	defer dmiSettingsMu.Unlock()
	row, err := GetDMISettings()
	if err != nil {
		return err
	}
	row.GoogleKey = stored
	row.UpdatedAt = time.Now().UTC()
	return db.Save(&row).Error
}

// operatorGoogleKey returns the decrypted operator key, or "" when none is
// saved.
func operatorGoogleKey() (string, error) {
	st, err := GetDMISettings()
	if err != nil || st.GoogleKey == "" {
		return "", err
	}
	key, err := decryptField(st.GoogleKey)
	if err != nil {
		return "", ErrEncryptionKeyUnavailable
	}
	if err := RequirePlaintextCredential(key); err != nil {
		return "", err
	}
	return key, nil
}

// DMIGoogleKeyInfo reports whether an operator key is saved and its public
// identifiers. When the key is saved but encryption is locked it returns
// set=true with ErrEncryptionKeyUnavailable.
func DMIGoogleKeyInfo() (set bool, clientID, clientEmail string, err error) {
	st, err := GetDMISettings()
	if err != nil || st.GoogleKey == "" {
		return false, "", "", err
	}
	key, err := operatorGoogleKey()
	if err != nil {
		return true, "", "", err
	}
	sa, err := parseGoogleServiceAccount(key)
	if err != nil {
		return true, "", "", err
	}
	return true, sa.ClientID, sa.ClientEmail, nil
}
