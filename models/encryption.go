package models

import (
	"fmt"

	"github.com/gophish/gophish/crypto"
)

// encryptField encrypts a sensitive field using the crypto package.
// This function is used by GORM hooks in various models.
func encryptField(plaintext string) (string, error) {
	return crypto.Encrypt(plaintext)
}

// decryptField decrypts a sensitive field using the crypto package.
// This function is used by GORM hooks in various models.
func decryptField(ciphertext string) (string, error) {
	return crypto.Decrypt(ciphertext)
}

// isFieldEncrypted checks if a field value is encrypted.
func isFieldEncrypted(value string) bool {
	return crypto.IsEncrypted(value)
}

// getFieldEncryptionStatus returns the encryption status of a field.
func getFieldEncryptionStatus(value string) string {
	return crypto.GetEncryptionStatus(value)
}

// GetEncryptionStatus returns a report of the encryption status of all sensitive fields
func GetEncryptionStatus() (string, error) {
	report := &crypto.EncryptionReport{
		EncryptionEnabled: crypto.IsEncryptionEnabled(),
	}

	// Check SMTP passwords
	var smtpRecords []struct {
		Password string
	}
	err := db.Table("smtp").Select("password").Scan(&smtpRecords).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP records: %w", err)
	}
	for _, r := range smtpRecords {
		report.SMTPPasswords.Total++
		switch crypto.GetEncryptionStatus(r.Password) {
		case "encrypted":
			report.SMTPPasswords.Encrypted++
		case "plaintext":
			report.SMTPPasswords.Plaintext++
		case "empty":
			report.SMTPPasswords.Empty++
		}
	}

	// Check SMS provider configs
	var smsRecords []struct {
		ProviderConfig string `gorm:"column:provider_config"`
	}
	err = db.Table("sms_profiles").Select("provider_config").Scan(&smsRecords).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMS records: %w", err)
	}
	for _, r := range smsRecords {
		report.SMSConfigs.Total++
		switch crypto.GetEncryptionStatus(r.ProviderConfig) {
		case "encrypted":
			report.SMSConfigs.Encrypted++
		case "plaintext":
			report.SMSConfigs.Plaintext++
		case "empty":
			report.SMSConfigs.Empty++
		}
	}

	// Check IMAP passwords
	var imapRecords []struct {
		Password string
	}
	err = db.Table("imap").Select("password").Scan(&imapRecords).Error
	if err != nil {
		return "", fmt.Errorf("failed to query IMAP records: %w", err)
	}
	for _, r := range imapRecords {
		report.IMAPPasswords.Total++
		switch crypto.GetEncryptionStatus(r.Password) {
		case "encrypted":
			report.IMAPPasswords.Encrypted++
		case "plaintext":
			report.IMAPPasswords.Plaintext++
		case "empty":
			report.IMAPPasswords.Empty++
		}
	}

	// Check Event details (captured credentials)
	var eventRecords []struct {
		Details string
	}
	err = db.Table("events").Select("details").Scan(&eventRecords).Error
	if err != nil {
		return "", fmt.Errorf("failed to query Event records: %w", err)
	}
	for _, r := range eventRecords {
		report.EventDetails.Total++
		switch crypto.GetEncryptionStatus(r.Details) {
		case "encrypted":
			report.EventDetails.Encrypted++
		case "plaintext":
			report.EventDetails.Plaintext++
		case "empty":
			report.EventDetails.Empty++
		}
	}

	return report.String(), nil
}

// MigrateToEncrypted encrypts all plaintext sensitive fields in the database
func MigrateToEncrypted() ([]string, error) {
	var results []string

	// Migrate SMTP passwords
	result, err := migrateSMTPToEncrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Migrate SMS provider configs
	result, err = migrateSMSToEncrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Migrate IMAP passwords
	result, err = migrateIMAPToEncrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Migrate Event details (captured credentials)
	result, err = migrateEventsToEncrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	return results, nil
}

// DryRunMigrateToEncrypted previews what would be encrypted without making changes
func DryRunMigrateToEncrypted() ([]string, error) {
	var results []string

	// Preview SMTP passwords
	result, err := dryRunSMTPEncrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview SMS provider configs
	result, err = dryRunSMSEncrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview IMAP passwords
	result, err = dryRunIMAPEncrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview Event details
	result, err = dryRunEventsEncrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	return results, nil
}

// MigrateToDecrypted decrypts all encrypted fields back to plaintext
func MigrateToDecrypted() ([]string, error) {
	var results []string

	// Decrypt SMTP passwords
	result, err := migrateSMTPToDecrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Decrypt SMS provider configs
	result, err = migrateSMSToDecrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Decrypt IMAP passwords
	result, err = migrateIMAPToDecrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Decrypt Event details (captured credentials)
	result, err = migrateEventsToDecrypted()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	return results, nil
}

func migrateSMTPToEncrypted() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("smtp").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Password == "" || crypto.IsEncrypted(r.Password) {
			skipped++
			continue
		}
		encrypted, err := crypto.Encrypt(r.Password)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt SMTP password (id=%d): %w", r.Id, err)
		}
		err = db.Table("smtp").Where("id = ?", r.Id).Update("password", encrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update SMTP password (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("SMTP passwords: %d encrypted, %d skipped", updated, skipped), nil
}

func migrateSMSToEncrypted() (string, error) {
	var records []struct {
		Id             int64
		ProviderConfig string `gorm:"column:provider_config"`
	}
	err := db.Table("sms_profiles").Select("id, provider_config").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMS records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.ProviderConfig == "" || crypto.IsEncrypted(r.ProviderConfig) {
			skipped++
			continue
		}
		encrypted, err := crypto.Encrypt(r.ProviderConfig)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt SMS config (id=%d): %w", r.Id, err)
		}
		err = db.Table("sms_profiles").Where("id = ?", r.Id).Update("provider_config", encrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update SMS config (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("SMS provider configs: %d encrypted, %d skipped", updated, skipped), nil
}

func migrateIMAPToEncrypted() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("imap").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query IMAP records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Password == "" || crypto.IsEncrypted(r.Password) {
			skipped++
			continue
		}
		encrypted, err := crypto.Encrypt(r.Password)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt IMAP password (id=%d): %w", r.Id, err)
		}
		err = db.Table("imap").Where("id = ?", r.Id).Update("password", encrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update IMAP password (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("IMAP passwords: %d encrypted, %d skipped", updated, skipped), nil
}

func migrateEventsToEncrypted() (string, error) {
	var records []struct {
		Id      int64
		Details string
	}
	err := db.Table("events").Select("id, details").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query Event records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Details == "" || crypto.IsEncrypted(r.Details) {
			skipped++
			continue
		}
		encrypted, err := crypto.Encrypt(r.Details)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt Event details (id=%d): %w", r.Id, err)
		}
		err = db.Table("events").Where("id = ?", r.Id).Update("details", encrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update Event details (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("Event details (captured data): %d encrypted, %d skipped", updated, skipped), nil
}

func migrateSMTPToDecrypted() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("smtp").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Password == "" || !crypto.IsEncrypted(r.Password) {
			skipped++
			continue
		}
		decrypted, err := crypto.Decrypt(r.Password)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt SMTP password (id=%d): %w", r.Id, err)
		}
		err = db.Table("smtp").Where("id = ?", r.Id).Update("password", decrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update SMTP password (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("SMTP passwords: %d decrypted, %d skipped", updated, skipped), nil
}

func migrateSMSToDecrypted() (string, error) {
	var records []struct {
		Id             int64
		ProviderConfig string `gorm:"column:provider_config"`
	}
	err := db.Table("sms_profiles").Select("id, provider_config").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMS records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.ProviderConfig == "" || !crypto.IsEncrypted(r.ProviderConfig) {
			skipped++
			continue
		}
		decrypted, err := crypto.Decrypt(r.ProviderConfig)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt SMS config (id=%d): %w", r.Id, err)
		}
		err = db.Table("sms_profiles").Where("id = ?", r.Id).Update("provider_config", decrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update SMS config (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("SMS provider configs: %d decrypted, %d skipped", updated, skipped), nil
}

func migrateIMAPToDecrypted() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("imap").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query IMAP records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Password == "" || !crypto.IsEncrypted(r.Password) {
			skipped++
			continue
		}
		decrypted, err := crypto.Decrypt(r.Password)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt IMAP password (id=%d): %w", r.Id, err)
		}
		err = db.Table("imap").Where("id = ?", r.Id).Update("password", decrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update IMAP password (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("IMAP passwords: %d decrypted, %d skipped", updated, skipped), nil
}

func migrateEventsToDecrypted() (string, error) {
	var records []struct {
		Id      int64
		Details string
	}
	err := db.Table("events").Select("id, details").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query Event records: %w", err)
	}

	updated := 0
	skipped := 0
	for _, r := range records {
		if r.Details == "" || !crypto.IsEncrypted(r.Details) {
			skipped++
			continue
		}
		decrypted, err := crypto.Decrypt(r.Details)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt Event details (id=%d): %w", r.Id, err)
		}
		err = db.Table("events").Where("id = ?", r.Id).Update("details", decrypted).Error
		if err != nil {
			return "", fmt.Errorf("failed to update Event details (id=%d): %w", r.Id, err)
		}
		updated++
	}

	return fmt.Sprintf("Event details (captured data): %d decrypted, %d skipped", updated, skipped), nil
}

// DryRunMigrateToDecrypted previews what would be decrypted without making changes
func DryRunMigrateToDecrypted() ([]string, error) {
	var results []string

	// Preview SMTP passwords
	result, err := dryRunSMTPDecrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview SMS provider configs
	result, err = dryRunSMSDecrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview IMAP passwords
	result, err = dryRunIMAPDecrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	// Preview Event details
	result, err = dryRunEventsDecrypt()
	if err != nil {
		return results, err
	}
	results = append(results, result)

	return results, nil
}

// Dry-run functions for encryption
func dryRunSMTPEncrypt() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("smtp").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP records: %w", err)
	}

	wouldEncrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Password == "" || crypto.IsEncrypted(r.Password) {
			wouldSkip++
		} else {
			wouldEncrypt++
		}
	}

	return fmt.Sprintf("SMTP passwords: %d would be encrypted, %d would be skipped", wouldEncrypt, wouldSkip), nil
}

func dryRunSMSEncrypt() (string, error) {
	var records []struct {
		Id             int64
		ProviderConfig string `gorm:"column:provider_config"`
	}
	err := db.Table("sms_profiles").Select("id, provider_config").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMS records: %w", err)
	}

	wouldEncrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.ProviderConfig == "" || crypto.IsEncrypted(r.ProviderConfig) {
			wouldSkip++
		} else {
			wouldEncrypt++
		}
	}

	return fmt.Sprintf("SMS provider configs: %d would be encrypted, %d would be skipped", wouldEncrypt, wouldSkip), nil
}

func dryRunIMAPEncrypt() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("imap").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query IMAP records: %w", err)
	}

	wouldEncrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Password == "" || crypto.IsEncrypted(r.Password) {
			wouldSkip++
		} else {
			wouldEncrypt++
		}
	}

	return fmt.Sprintf("IMAP passwords: %d would be encrypted, %d would be skipped", wouldEncrypt, wouldSkip), nil
}

func dryRunEventsEncrypt() (string, error) {
	var records []struct {
		Id      int64
		Details string
	}
	err := db.Table("events").Select("id, details").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query Event records: %w", err)
	}

	wouldEncrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Details == "" || crypto.IsEncrypted(r.Details) {
			wouldSkip++
		} else {
			wouldEncrypt++
		}
	}

	return fmt.Sprintf("Event details (captured data): %d would be encrypted, %d would be skipped", wouldEncrypt, wouldSkip), nil
}

// Dry-run functions for decryption
func dryRunSMTPDecrypt() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("smtp").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP records: %w", err)
	}

	wouldDecrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Password == "" || !crypto.IsEncrypted(r.Password) {
			wouldSkip++
		} else {
			wouldDecrypt++
		}
	}

	return fmt.Sprintf("SMTP passwords: %d would be decrypted, %d would be skipped", wouldDecrypt, wouldSkip), nil
}

func dryRunSMSDecrypt() (string, error) {
	var records []struct {
		Id             int64
		ProviderConfig string `gorm:"column:provider_config"`
	}
	err := db.Table("sms_profiles").Select("id, provider_config").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query SMS records: %w", err)
	}

	wouldDecrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.ProviderConfig == "" || !crypto.IsEncrypted(r.ProviderConfig) {
			wouldSkip++
		} else {
			wouldDecrypt++
		}
	}

	return fmt.Sprintf("SMS provider configs: %d would be decrypted, %d would be skipped", wouldDecrypt, wouldSkip), nil
}

func dryRunIMAPDecrypt() (string, error) {
	var records []struct {
		Id       int64
		Password string
	}
	err := db.Table("imap").Select("id, password").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query IMAP records: %w", err)
	}

	wouldDecrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Password == "" || !crypto.IsEncrypted(r.Password) {
			wouldSkip++
		} else {
			wouldDecrypt++
		}
	}

	return fmt.Sprintf("IMAP passwords: %d would be decrypted, %d would be skipped", wouldDecrypt, wouldSkip), nil
}

func dryRunEventsDecrypt() (string, error) {
	var records []struct {
		Id      int64
		Details string
	}
	err := db.Table("events").Select("id, details").Scan(&records).Error
	if err != nil {
		return "", fmt.Errorf("failed to query Event records: %w", err)
	}

	wouldDecrypt := 0
	wouldSkip := 0
	for _, r := range records {
		if r.Details == "" || !crypto.IsEncrypted(r.Details) {
			wouldSkip++
		} else {
			wouldDecrypt++
		}
	}

	return fmt.Sprintf("Event details (captured data): %d would be decrypted, %d would be skipped", wouldDecrypt, wouldSkip), nil
}
