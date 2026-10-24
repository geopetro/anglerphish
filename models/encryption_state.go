package models

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gophish/gophish/crypto"
	log "github.com/gophish/gophish/logger"
)

var (
	// ErrEncryptionKeyUnavailable is returned when an encrypted credential is
	// needed while no key is loaded (Locked state).
	ErrEncryptionKeyUnavailable = errors.New("encryption key unavailable")
	// ErrEncryptionAlreadyOn is returned when enabling while a key is loaded.
	ErrEncryptionAlreadyOn = errors.New("encryption is already on")
	// ErrEncryptedDataExists is returned when enabling while encrypted data
	// exists; the existing key must be supplied through unlock instead.
	ErrEncryptedDataExists = errors.New("encrypted data already exists; unlock with the existing key instead")
	// ErrNotLocked is returned by unlock and discard outside the Locked state.
	ErrNotLocked = errors.New("encryption is not locked")
	// ErrWrongKey is returned when a key does not match the stored data.
	ErrWrongKey = errors.New("the key does not match the encrypted data")
	// ErrMigrationRunning is returned when a migration is already in progress.
	ErrMigrationRunning = errors.New("an encryption migration is already running")
	// ErrNoKeyLoaded is returned when an action needs a loaded key.
	ErrNoKeyLoaded = errors.New("no encryption key is loaded")
	// ErrDecryptIncomplete is returned when disabling leaves encrypted values.
	ErrDecryptIncomplete = errors.New("some values could not be decrypted; encryption stays in decrypt-only mode")
	// ErrBadConfirmation is returned when the discard phrase does not match.
	ErrBadConfirmation = errors.New("confirmation phrase does not match")
)

// RequirePlaintextCredential returns ErrEncryptionKeyUnavailable when value is
// still ciphertext, so callers never send an encrypted blob to a remote server.
func RequirePlaintextCredential(value string) error {
	if crypto.IsEncrypted(value) {
		return ErrEncryptionKeyUnavailable
	}
	return nil
}

// DiscardConfirmationPhrase must be typed exactly to discard encrypted data.
const DiscardConfirmationPhrase = "DISCARD ENCRYPTED DATA"

// EncryptionState is the admin-facing encryption state.
type EncryptionState string

const (
	EncryptionStateOff       EncryptionState = "off"
	EncryptionStateOn        EncryptionState = "on"
	EncryptionStateLocked    EncryptionState = "locked"
	EncryptionStateDisabling EncryptionState = "disabling"
)

// Locked reasons.
const (
	lockedMissingKey = "missing_key"
	lockedWrongKey   = "wrong_key"
	lockedInvalidKey = "invalid_key"
)

// EncryptionStatus describes the current encryption state for the admin UI.
type EncryptionStatus struct {
	State            EncryptionState          `json:"state"`
	Source           crypto.KeySource         `json:"source"`
	LockedReason     string                   `json:"locked_reason,omitempty"`
	StartupWarning   string                   `json:"startup_warning,omitempty"`
	Report           *crypto.EncryptionReport `json:"report"`
	PlaintextCount   int                      `json:"plaintext_count"`
	MigrationRunning bool                     `json:"migration_running"`
}

// lockState holds the models-level Locked flag. It is set only at startup
// (or by SetEncryptionLockedForTest) and cleared by unlock or discard.
var lockState struct {
	mu             sync.RWMutex
	locked         bool
	reason         string
	startupWarning string
}

// actionMu serializes the state-changing actions (startup, enable, unlock,
// discard) so they cannot interleave.
var actionMu sync.Mutex

// migrationMu makes migrations (including disable) single-flight.
var migrationMu sync.Mutex

// migrationRunning mirrors migrationMu for status reporting without
// contending for the lock.
var migrationRunning atomic.Bool

func setLocked(reason string) {
	lockState.mu.Lock()
	defer lockState.mu.Unlock()
	lockState.locked = true
	lockState.reason = reason
}

func clearLocked() {
	lockState.mu.Lock()
	defer lockState.mu.Unlock()
	lockState.locked = false
	lockState.reason = ""
}

func setStartupWarning(w string) {
	lockState.mu.Lock()
	defer lockState.mu.Unlock()
	lockState.startupWarning = w
}

// SetEncryptionLockedForTest is for tests only: true puts encryption in the
// Locked state with reason missing_key, false clears the Locked flag.
func SetEncryptionLockedForTest(locked bool) {
	if locked {
		setLocked(lockedMissingKey)
	} else {
		clearLocked()
	}
}

// resetEncryptionStateForTest unloads the key and clears all models-level
// encryption state.
func resetEncryptionStateForTest() {
	crypto.Unload()
	SetEncryptionLockedForTest(false)
	setStartupWarning("")
}

// IsEncryptionLocked reports whether encryption is Locked. It does not touch
// the database.
func IsEncryptionLocked() bool {
	lockState.mu.RLock()
	defer lockState.mu.RUnlock()
	return lockState.locked
}

// IsSessionKeyActive reports whether the loaded key was supplied through the
// UI this session (and so will be lost on restart). It does not touch the
// database.
func IsSessionKeyActive() bool {
	return crypto.CurrentSource() == crypto.SourceSession
}

// InitEncryptionAtStartup sets the encryption state from the environment key
// and the database. Key problems never fail startup; they lead to the Locked
// state or a startup warning. Only database errors are returned.
func InitEncryptionAtStartup() error {
	actionMu.Lock()
	defer actionMu.Unlock()

	crypto.Unload()
	clearLocked()
	setStartupWarning("")

	key, present := crypto.ReadEnvKey()
	if !present {
		n, err := countEncryptedValues()
		if err != nil {
			return err
		}
		if n > 0 {
			setLocked(lockedMissingKey)
			log.Errorf("encryption: locked at startup, %d encrypted values exist but %s is not set", n, crypto.EncryptionKeyEnvVar)
			return nil
		}
		// No key and no ciphertext: a leftover check value would make a later
		// different env key look wrong, so remove it.
		return deleteCheckValue()
	}

	if err := crypto.ValidateKey(key); err != nil {
		n, cerr := countEncryptedValues()
		if cerr != nil {
			return cerr
		}
		if n > 0 {
			setLocked(lockedInvalidKey)
			log.Errorf("encryption: locked at startup, %s is malformed and %d encrypted values exist: %v", crypto.EncryptionKeyEnvVar, n, err)
		} else {
			setStartupWarning(fmt.Sprintf("%s is set but is not a valid key; encryption is off", crypto.EncryptionKeyEnvVar))
			log.Errorf("encryption: %s is malformed, encryption is off: %v", crypto.EncryptionKeyEnvVar, err)
		}
		return nil
	}

	ok, err := verifyKey(key)
	if err != nil {
		return err
	}
	if !ok {
		setLocked(lockedWrongKey)
		log.Errorf("encryption: locked at startup, %s does not match the encrypted data", crypto.EncryptionKeyEnvVar)
		return nil
	}
	if err := crypto.LoadKey(key, crypto.SourceEnv); err != nil {
		return err
	}
	if err := ensureCheckValue(); err != nil {
		return err
	}
	log.Infof("encryption: on, key loaded from %s", crypto.EncryptionKeyEnvVar)
	return nil
}

// ensureCheckValue writes a check value if none is stored.
func ensureCheckValue() error {
	has, err := hasCheckValue()
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return writeCheckValue()
}

// currentState derives the state from the Locked flag and the crypto mode.
func currentState() EncryptionState {
	if IsEncryptionLocked() {
		return EncryptionStateLocked
	}
	switch crypto.CurrentMode() {
	case crypto.ModeOn:
		return EncryptionStateOn
	case crypto.ModeDecryptOnly:
		return EncryptionStateDisabling
	default:
		return EncryptionStateOff
	}
}

// GetEncryptionStatusInfo returns the current state, key source and counts.
func GetEncryptionStatusInfo() (EncryptionStatus, error) {
	report, err := GetEncryptionReport()
	if err != nil {
		return EncryptionStatus{}, err
	}
	lockState.mu.RLock()
	reason, warning := lockState.reason, lockState.startupWarning
	lockState.mu.RUnlock()
	return EncryptionStatus{
		State:          currentState(),
		Source:         crypto.CurrentSource(),
		LockedReason:   reason,
		StartupWarning: warning,
		Report:         report,
		PlaintextCount: report.SMTPPasswords.Plaintext + report.SMSConfigs.Plaintext +
			report.IMAPPasswords.Plaintext + report.EventDetails.Plaintext,
		MigrationRunning: migrationRunning.Load(),
	}, nil
}

// EnableEncryptionWithKey loads key as the session key and writes the check
// value. Existing data is not migrated.
func EnableEncryptionWithKey(key string) error {
	actionMu.Lock()
	defer actionMu.Unlock()

	if crypto.HasKey() {
		log.Warnf("encryption: enable refused, a key is already loaded")
		return ErrEncryptionAlreadyOn
	}
	if IsEncryptionLocked() {
		log.Warnf("encryption: enable refused, encryption is locked")
		return ErrEncryptedDataExists
	}
	n, err := countEncryptedValues()
	if err != nil {
		log.Warnf("encryption: enable failed: %v", err)
		return err
	}
	if n > 0 {
		log.Warnf("encryption: enable refused, %d encrypted values exist", n)
		return ErrEncryptedDataExists
	}
	if err := crypto.LoadKey(key, crypto.SourceSession); err != nil {
		log.Warnf("encryption: enable failed, invalid key format")
		return err
	}
	if err := writeCheckValue(); err != nil {
		crypto.Unload()
		log.Warnf("encryption: enable failed writing check value: %v", err)
		return err
	}
	log.Infof("encryption: enabled with a session key")
	return nil
}

// UnlockEncryption leaves the Locked state by loading key as the session key,
// provided it matches the stored data.
func UnlockEncryption(key string) error {
	actionMu.Lock()
	defer actionMu.Unlock()

	if !IsEncryptionLocked() {
		log.Warnf("encryption: unlock refused, not locked")
		return ErrNotLocked
	}
	if err := crypto.ValidateKey(key); err != nil {
		log.Warnf("encryption: unlock failed, invalid key format")
		return err
	}
	ok, err := verifyKey(key)
	if err != nil {
		log.Warnf("encryption: unlock failed: %v", err)
		return err
	}
	if !ok {
		log.Warnf("encryption: unlock failed, wrong key")
		return ErrWrongKey
	}
	if err := crypto.LoadKey(key, crypto.SourceSession); err != nil {
		log.Warnf("encryption: unlock failed: %v", err)
		return err
	}
	if err := ensureCheckValue(); err != nil {
		crypto.Unload()
		log.Warnf("encryption: unlock failed writing check value: %v", err)
		return err
	}
	clearLocked()
	log.Infof("encryption: unlocked with a session key")
	return nil
}

// tryLockMigration acquires the single-flight migration lock.
func tryLockMigration() bool {
	if !migrationMu.TryLock() {
		return false
	}
	migrationRunning.Store(true)
	return true
}

func unlockMigration() {
	migrationRunning.Store(false)
	migrationMu.Unlock()
}

// RunEncryptionMigration encrypts or decrypts existing data, or previews
// that when dryRun is set. Only one migration runs at a time.
func RunEncryptionMigration(encrypt bool, dryRun bool) ([]string, error) {
	direction := "decrypt"
	if encrypt {
		direction = "encrypt"
	}
	if !crypto.HasKey() {
		log.Warnf("encryption: %s migration (dry_run=%t) refused, no key loaded", direction, dryRun)
		return nil, ErrNoKeyLoaded
	}
	if !tryLockMigration() {
		log.Warnf("encryption: %s migration (dry_run=%t) refused, another migration is running", direction, dryRun)
		return nil, ErrMigrationRunning
	}
	defer unlockMigration()

	log.Infof("encryption: %s migration (dry_run=%t) started", direction, dryRun)
	var results []string
	var err error
	switch {
	case encrypt && dryRun:
		results, err = DryRunMigrateToEncrypted()
	case encrypt:
		results, err = MigrateToEncrypted()
	case dryRun:
		results, err = DryRunMigrateToDecrypted()
	default:
		results, err = MigrateToDecrypted()
	}
	if err != nil {
		log.Warnf("encryption: %s migration (dry_run=%t) failed: %v; results: %v", direction, dryRun, err, results)
		return results, err
	}
	log.Infof("encryption: %s migration (dry_run=%t) finished: %v", direction, dryRun, results)
	return results, nil
}

// DisableEncryption stops encrypting new values, decrypts existing data and,
// once no encrypted values remain, removes the check value and unloads the
// key. If values remain it stays in decrypt-only mode (state disabling) and
// returns ErrDecryptIncomplete.
func DisableEncryption() ([]string, error) {
	if !crypto.HasKey() {
		log.Warnf("encryption: disable refused, no key loaded")
		return nil, ErrNoKeyLoaded
	}
	if !tryLockMigration() {
		log.Warnf("encryption: disable refused, a migration is running")
		return nil, ErrMigrationRunning
	}
	defer unlockMigration()

	crypto.SetDecryptOnly()
	log.Infof("encryption: disable started, new values are no longer encrypted")

	// A write that was encrypted just before SetDecryptOnly may land after
	// the first pass has scanned its table, so a second pass picks it up.
	var results []string
	var remaining int
	for pass := 0; pass < 2; pass++ {
		r, err := MigrateToDecrypted()
		results = append(results, r...)
		if err != nil {
			log.Warnf("encryption: disable failed during decryption: %v; results: %v", err, results)
			return results, err
		}
		remaining, err = countEncryptedValues()
		if err != nil {
			log.Warnf("encryption: disable failed counting remaining values, staying in decrypt-only mode: %v", err)
			return results, err
		}
		if remaining == 0 {
			break
		}
	}
	if remaining > 0 {
		log.Warnf("encryption: disable incomplete, %d encrypted values remain; staying in decrypt-only mode; results: %v", remaining, results)
		return results, ErrDecryptIncomplete
	}
	if err := deleteCheckValue(); err != nil {
		log.Warnf("encryption: disable failed removing check value: %v", err)
		return results, err
	}
	crypto.Unload()
	log.Infof("encryption: disabled, key unloaded: %v", results)
	return results, nil
}

// discardTargets lists the columns that may hold encrypted values.
var discardTargets = []struct{ table, column string }{
	{"smtp", "password"},
	{"sms_profiles", "provider_config"},
	{"imap", "password"},
	{"events", "details"},
}

// DiscardEncryptedData blanks every encrypted value, removes the check value
// and leaves the Locked state. It is only allowed while Locked and requires
// DiscardConfirmationPhrase. It returns the counts from before the discard.
func DiscardEncryptedData(confirmation string) (*crypto.EncryptionReport, error) {
	actionMu.Lock()
	defer actionMu.Unlock()

	if !IsEncryptionLocked() {
		log.Warnf("encryption: discard refused, not locked")
		return nil, ErrNotLocked
	}
	if confirmation != DiscardConfirmationPhrase {
		log.Warnf("encryption: discard refused, confirmation phrase mismatch")
		return nil, ErrBadConfirmation
	}
	report, err := GetEncryptionReport()
	if err != nil {
		log.Warnf("encryption: discard failed: %v", err)
		return nil, err
	}

	tx := db.Begin()
	for _, t := range discardTargets {
		err := tx.Exec(fmt.Sprintf("UPDATE %s SET %s = '' WHERE %s LIKE ?", t.table, t.column, t.column), crypto.EncryptedPrefix+"%").Error
		if err != nil {
			tx.Rollback()
			log.Warnf("encryption: discard failed on %s: %v", t.table, err)
			return nil, err
		}
	}
	if err := tx.Delete(EncryptionMeta{}).Error; err != nil {
		tx.Rollback()
		log.Warnf("encryption: discard failed removing check value: %v", err)
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		log.Warnf("encryption: discard failed to commit: %v", err)
		return nil, err
	}
	clearLocked()
	log.Infof("encryption: discarded encrypted data (smtp passwords: %d, sms configs: %d, imap passwords: %d, event details: %d)",
		report.SMTPPasswords.Encrypted, report.SMSConfigs.Encrypted, report.IMAPPasswords.Encrypted, report.EventDetails.Encrypted)
	return report, nil
}
