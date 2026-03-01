package crypto

import (
	"fmt"
)

// FieldStatus represents the encryption status of a field type
type FieldStatus struct {
	Total     int
	Encrypted int
	Plaintext int
	Empty     int
}

// EncryptionReport represents the encryption status of all sensitive fields
type EncryptionReport struct {
	EncryptionEnabled bool
	SMTPPasswords     FieldStatus
	SMSConfigs        FieldStatus
	IMAPPasswords     FieldStatus
	EventDetails      FieldStatus
}

// String returns a formatted string representation of the encryption report
func (r *EncryptionReport) String() string {
	status := "No"
	if r.EncryptionEnabled {
		status = "Yes"
	}
	return fmt.Sprintf(`Encryption Enabled: %s
SMTP Passwords: %d encrypted, %d plaintext, %d empty (total: %d)
SMS Provider Configs: %d encrypted, %d plaintext, %d empty (total: %d)
IMAP Passwords: %d encrypted, %d plaintext, %d empty (total: %d)
Event Details (captured data): %d encrypted, %d plaintext, %d empty (total: %d)`,
		status,
		r.SMTPPasswords.Encrypted, r.SMTPPasswords.Plaintext, r.SMTPPasswords.Empty, r.SMTPPasswords.Total,
		r.SMSConfigs.Encrypted, r.SMSConfigs.Plaintext, r.SMSConfigs.Empty, r.SMSConfigs.Total,
		r.IMAPPasswords.Encrypted, r.IMAPPasswords.Plaintext, r.IMAPPasswords.Empty, r.IMAPPasswords.Total,
		r.EventDetails.Encrypted, r.EventDetails.Plaintext, r.EventDetails.Empty, r.EventDetails.Total,
	)
}

// MigrationResult represents the result of a migration operation
type MigrationResult struct {
	Table   string
	Field   string
	Updated int
	Skipped int
	Errors  int
}

// String returns a formatted string representation of the migration result
func (r *MigrationResult) String() string {
	return fmt.Sprintf("%s.%s: %d updated, %d skipped, %d errors",
		r.Table, r.Field, r.Updated, r.Skipped, r.Errors)
}
