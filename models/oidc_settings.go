package models

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/crypto"
	log "github.com/gophish/gophish/logger"
)

// Supported SSO provider presets.
const (
	OIDCProviderEntra    = "entra"
	OIDCProviderKeycloak = "keycloak"
	OIDCProviderOkta     = "okta"
	OIDCProviderGeneric  = "generic"
)

// OIDCStatus describes whether SSO is running.
type OIDCStatus string

const (
	OIDCStatusEnabled     OIDCStatus = "enabled"
	OIDCStatusDisabled    OIDCStatus = "disabled"
	OIDCStatusUnavailable OIDCStatus = "unavailable"
)

// ErrOIDCInvalid is returned when submitted SSO settings are not valid.
var ErrOIDCInvalid = errors.New("invalid sso settings")

// oidcMu serializes saves and reloads so they cannot interleave.
var oidcMu sync.Mutex

// OIDCSettings is the single row of admin SSO configuration. ClientSecret
// holds the stored value, which is ciphertext when encryption is on.
type OIDCSettings struct {
	Id                   int64     `json:"-" gorm:"column:id; primary_key:yes"`
	Enabled              bool      `json:"enabled" gorm:"column:enabled"`
	Provider             string    `json:"provider" gorm:"column:provider"`
	Issuer               string    `json:"issuer" gorm:"column:issuer"`
	ClientID             string    `json:"client_id" gorm:"column:client_id"`
	ClientSecret         string    `json:"-" gorm:"column:client_secret"`
	RedirectURL          string    `json:"redirect_url" gorm:"column:redirect_url"`
	RequiredGroup        string    `json:"required_group" gorm:"column:required_group"`
	GroupsClaim          string    `json:"groups_claim" gorm:"column:groups_claim"`
	UsernameFromEmail    string    `json:"username_from_email" gorm:"column:username_from_email"`
	AllowUnverifiedEmail bool      `json:"allow_unverified_email" gorm:"column:allow_unverified_email"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the database tablename for Gorm to use.
func (OIDCSettings) TableName() string {
	return "oidc_settings"
}

// OIDCSettingsInput is the submitted form of the SSO settings. An empty
// ClientSecret keeps the stored secret.
type OIDCSettingsInput struct {
	Enabled              bool   `json:"enabled"`
	Provider             string `json:"provider"`
	Issuer               string `json:"issuer"`
	ClientID             string `json:"client_id"`
	ClientSecret         string `json:"client_secret"`
	RedirectURL          string `json:"redirect_url"`
	RequiredGroup        string `json:"required_group"`
	GroupsClaim          string `json:"groups_claim"`
	UsernameFromEmail    string `json:"username_from_email"`
	AllowUnverifiedEmail bool   `json:"allow_unverified_email"`
}

func defaultOIDCSettings() OIDCSettings {
	return OIDCSettings{
		Provider:          OIDCProviderGeneric,
		UsernameFromEmail: auth.UsernameFromEmailLocalPart,
	}
}

// loadOIDCRow returns the lowest-id row, if any.
func loadOIDCRow() (OIDCSettings, bool, error) {
	var rows []OIDCSettings
	if err := db.Order("id asc").Limit(1).Find(&rows).Error; err != nil {
		return OIDCSettings{}, false, err
	}
	if len(rows) == 0 {
		return OIDCSettings{}, false, nil
	}
	return rows[0], true, nil
}

// GetOIDCSettings returns the stored SSO settings, or defaults when none
// have been saved.
func GetOIDCSettings() (OIDCSettings, error) {
	s, found, err := loadOIDCRow()
	if err != nil {
		return OIDCSettings{}, err
	}
	if !found {
		return defaultOIDCSettings(), nil
	}
	return s, nil
}

// OIDCSecretSet reports whether a client secret is stored.
func OIDCSecretSet(s OIDCSettings) bool {
	return s.ClientSecret != ""
}

func invalidOIDC(msg string) error {
	return fmt.Errorf("%w: %s", ErrOIDCInvalid, msg)
}

// SaveOIDCSettings validates and stores the SSO settings, then reloads the
// active client.
func SaveOIDCSettings(in OIDCSettingsInput) error {
	oidcMu.Lock()
	defer oidcMu.Unlock()

	in.Provider = strings.TrimSpace(in.Provider)
	in.ClientSecret = strings.TrimSpace(in.ClientSecret)
	in.Issuer = strings.TrimSpace(in.Issuer)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.RedirectURL = strings.TrimSpace(in.RedirectURL)
	in.RequiredGroup = strings.TrimSpace(in.RequiredGroup)
	in.GroupsClaim = strings.TrimSpace(in.GroupsClaim)
	in.UsernameFromEmail = strings.TrimSpace(in.UsernameFromEmail)
	if in.Provider == "" {
		in.Provider = OIDCProviderGeneric
	}
	// An empty groups claim is kept as-is; auth uses "groups" when it is empty.
	if in.UsernameFromEmail == "" {
		in.UsernameFromEmail = auth.UsernameFromEmailLocalPart
	}

	switch in.Provider {
	case OIDCProviderEntra, OIDCProviderKeycloak, OIDCProviderOkta, OIDCProviderGeneric:
	default:
		return invalidOIDC("unknown provider")
	}
	switch in.UsernameFromEmail {
	case auth.UsernameFromEmailLocalPart, auth.UsernameFromEmailFull:
	default:
		return invalidOIDC("unknown username mapping")
	}

	for _, f := range []struct{ name, value string }{
		{"issuer", in.Issuer},
		{"client id", in.ClientID},
		{"redirect url", in.RedirectURL},
		{"required group", in.RequiredGroup},
		{"groups claim", in.GroupsClaim},
	} {
		if len(f.value) > 255 {
			return invalidOIDC(f.name + " must be 255 characters or fewer")
		}
	}

	row, found, err := loadOIDCRow()
	if err != nil {
		return err
	}

	// The effective secret only needs to be present for validation; an
	// encrypted stored secret counts as set.
	effective := in.ClientSecret
	if effective == "" {
		effective = row.ClientSecret
	}
	if err := auth.ValidateOIDCConfig(auth.OIDCConfig{
		Enabled:       in.Enabled,
		Issuer:        in.Issuer,
		ClientID:      in.ClientID,
		ClientSecret:  effective,
		RedirectURL:   in.RedirectURL,
		RequiredGroup: in.RequiredGroup,
	}); err != nil {
		return invalidOIDC(err.Error())
	}

	if in.ClientSecret != "" {
		enc, err := crypto.Encrypt(in.ClientSecret)
		if err != nil {
			return fmt.Errorf("failed to protect client secret: %w", err)
		}
		row.ClientSecret = enc
	}
	row.Enabled = in.Enabled
	row.Provider = in.Provider
	row.Issuer = in.Issuer
	row.ClientID = in.ClientID
	row.RedirectURL = in.RedirectURL
	row.RequiredGroup = in.RequiredGroup
	row.GroupsClaim = in.GroupsClaim
	row.UsernameFromEmail = in.UsernameFromEmail
	row.AllowUnverifiedEmail = in.AllowUnverifiedEmail
	row.UpdatedAt = time.Now().UTC()

	var saveErr error
	if found && in.ClientSecret == "" {
		// Leave the secret column alone so a concurrent migration or
		// discard cannot be overwritten with a stale value.
		saveErr = db.Omit("client_secret").Save(&row).Error
	} else if found {
		saveErr = db.Save(&row).Error
	} else {
		saveErr = db.Create(&row).Error
	}
	if saveErr != nil {
		return saveErr
	}
	return reloadOIDCLocked()
}

// CurrentOIDCStatus reports whether SSO is running, off, or configured but
// unable to start (locked secret or invalid settings).
func CurrentOIDCStatus() OIDCStatus {
	if auth.ActiveOIDCClient() != nil {
		return OIDCStatusEnabled
	}
	s, err := GetOIDCSettings()
	if err == nil && s.Enabled {
		return OIDCStatusUnavailable
	}
	return OIDCStatusDisabled
}

// ReloadOIDC rebuilds the active SSO client from the stored settings.
func ReloadOIDC() error {
	oidcMu.Lock()
	defer oidcMu.Unlock()
	return reloadOIDCLocked()
}

func reloadOIDCLocked() error {
	s, err := GetOIDCSettings()
	if err != nil {
		return err
	}
	if !s.Enabled {
		auth.SetActiveOIDCClient(nil)
		return nil
	}
	secret, err := crypto.Decrypt(s.ClientSecret)
	if err != nil || strings.HasPrefix(secret, crypto.EncryptedPrefix) {
		auth.SetActiveOIDCClient(nil)
		log.Warn("sso: client secret unavailable, encryption is locked")
		return nil
	}
	client, err := auth.NewOIDCClient(auth.OIDCConfig{
		Enabled:              true,
		Issuer:               s.Issuer,
		ClientID:             s.ClientID,
		ClientSecret:         secret,
		RedirectURL:          s.RedirectURL,
		RequiredGroup:        s.RequiredGroup,
		GroupsClaim:          s.GroupsClaim,
		UsernameFromEmail:    s.UsernameFromEmail,
		AllowUnverifiedEmail: s.AllowUnverifiedEmail,
	})
	if err != nil {
		auth.SetActiveOIDCClient(nil)
		log.Warnf("sso: settings invalid, sso is off: %v", err)
		return err
	}
	auth.SetActiveOIDCClient(client)
	return nil
}
