package models

import (
	"bytes"
	"crypto/rand"
	"html/template"
	"math/big"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
)

// MFAPageContext is the context used for templating the MFA page
type MFAPageContext struct {
	RId   string
	Error string
}

// RenderMFAPage renders the MFA verification page using the provided template HTML
// If customHTML is empty, it uses the default template
func RenderMFAPage(customHTML string, rid string, errorMessage string) string {
	// If no custom HTML, use the hardcoded version for backward compatibility
	if customHTML == "" {
		return GetMFAInjectedPage(rid, errorMessage)
	}

	// Parse and execute the custom template
	ctx := MFAPageContext{
		RId:   rid,
		Error: errorMessage,
	}

	tmpl, err := template.New("mfa_page").Parse(customHTML)
	if err != nil {
		log.Errorf("Error parsing custom MFA page template: %v", err)
		// Fallback to default
		return GetMFAInjectedPage(rid, errorMessage)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		log.Errorf("Error executing custom MFA page template: %v", err)
		// Fallback to default
		return GetMFAInjectedPage(rid, errorMessage)
	}

	return buf.String()
}

// MFACode represents a generated MFA code for a result
type MFACode struct {
	Id        int64     `json:"id" gorm:"column:id; primary_key:yes"`
	RId       string    `json:"r_id" gorm:"column:r_id;index"`
	Code      string    `json:"code" gorm:"column:code"`
	Phone     string    `json:"phone" gorm:"column:phone"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	Verified  bool      `json:"verified" gorm:"column:verified;default:false"`
}

// TableName specifies the database tablename for Gorm to use
func (m MFACode) TableName() string {
	return "mfa_codes"
}

// MFA Code Types
const (
	MFACodeTypeNumeric      = "numeric"
	MFACodeTypeAlphanumeric = "alphanumeric"
	MFACodeTypeAlpha        = "alpha"
)

// Default MFA Message
const DefaultMFAMessage = "Your verification code is: {{.Code}}"

// Character sets for code generation
const (
	numericChars      = "0123456789"
	alphaChars        = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	alphanumericChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// GenerateMFACode generates a random MFA code based on the specified length and type
func GenerateMFACode(length int, codeType string) (string, error) {
	if length <= 0 {
		length = 6 // Default to 6 characters
	}

	var charset string
	switch strings.ToLower(codeType) {
	case MFACodeTypeAlpha:
		charset = alphaChars
	case MFACodeTypeAlphanumeric:
		charset = alphanumericChars
	case MFACodeTypeNumeric:
		fallthrough
	default:
		charset = numericChars
	}

	code := make([]byte, length)
	for i := range code {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[idx.Int64()]
	}
	return string(code), nil
}

// SaveMFACode saves a new MFA code for the given result ID
// If a code already exists for this RID, it will be overwritten
func SaveMFACode(rid string, code string, phone string) (*MFACode, error) {
	// Delete any existing codes for this RID (we only keep the latest)
	err := db.Where("r_id = ?", rid).Delete(&MFACode{}).Error
	if err != nil {
		log.Warnf("Error deleting old MFA codes for rid %s: %v", rid, err)
		// Continue anyway - not a fatal error
	}

	mfaCode := &MFACode{
		RId:       rid,
		Code:      code,
		Phone:     phone,
		CreatedAt: time.Now().UTC(),
		Verified:  false,
	}

	err = db.Save(mfaCode).Error
	if err != nil {
		log.Errorf("Error saving MFA code for rid %s: %v", rid, err)
		return nil, err
	}

	log.Infof("MFA code saved for rid %s, phone %s", rid, phone)
	return mfaCode, nil
}

// GetLatestMFACode retrieves the most recent MFA code for the given result ID
func GetLatestMFACode(rid string) (*MFACode, error) {
	mfaCode := &MFACode{}
	err := db.Where("r_id = ?", rid).Order("created_at DESC").First(mfaCode).Error
	if err != nil {
		return nil, err
	}
	return mfaCode, nil
}

// VerifyMFACode checks if the submitted code matches the latest code for the RID
// Returns true if verified, false otherwise
func VerifyMFACode(rid string, submittedCode string) (bool, error) {
	mfaCode, err := GetLatestMFACode(rid)
	if err != nil {
		log.Warnf("No MFA code found for rid %s: %v", rid, err)
		return false, err
	}

	// Case-insensitive comparison for alpha/alphanumeric codes
	if strings.EqualFold(mfaCode.Code, submittedCode) {
		return true, nil
	}

	return false, nil
}

// MarkMFACodeVerified marks the MFA code as verified for the given RID
func MarkMFACodeVerified(rid string) error {
	err := db.Model(&MFACode{}).Where("r_id = ?", rid).Update("verified", true).Error
	if err != nil {
		log.Errorf("Error marking MFA code as verified for rid %s: %v", rid, err)
		return err
	}
	return nil
}

// GetMFACodeByRId retrieves any MFA code for the given result ID (for checking if MFA was initiated)
func GetMFACodeByRId(rid string) (*MFACode, error) {
	return GetLatestMFACode(rid)
}

// DeleteMFACode deletes all MFA codes for the given result ID
func DeleteMFACode(rid string) error {
	return db.Where("r_id = ?", rid).Delete(&MFACode{}).Error
}

// IsMFACodeVerified checks if an MFA code for the given RID has been verified
func IsMFACodeVerified(rid string) (bool, error) {
	mfaCode, err := GetLatestMFACode(rid)
	if err != nil {
		return false, err
	}
	return mfaCode.Verified, nil
}

// MFAMessageContext is the context used for templating the MFA message
type MFAMessageContext struct {
	Code string
}

// GenerateMFAMessage generates the MFA message by replacing {{.Code}} with the actual code
func GenerateMFAMessage(messageTemplate string, code string) string {
	if messageTemplate == "" {
		messageTemplate = DefaultMFAMessage
	}

	// Simple string replacement for the code placeholder
	result := strings.ReplaceAll(messageTemplate, "{{.Code}}", code)
	return result
}

// GetDefaultMFAPageTemplate returns the default MFA page template HTML
// This is used to populate the editor when customizing the MFA page
func GetDefaultMFAPageTemplate() string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification Required</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f5f5f5;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .mfa-container {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
            max-width: 400px;
            width: 100%;
            text-align: center;
        }
        .mfa-icon { font-size: 48px; margin-bottom: 20px; }
        h1 { color: #333; font-size: 24px; margin-bottom: 10px; }
        p { color: #666; margin-bottom: 25px; line-height: 1.5; }
        .mfa-error {
            color: #dc3545;
            margin-bottom: 15px;
            padding: 10px;
            background-color: #f8d7da;
            border: 1px solid #f5c6cb;
            border-radius: 4px;
        }
        .mfa-form { display: flex; flex-direction: column; gap: 15px; }
        .mfa-input {
            padding: 15px;
            font-size: 24px;
            text-align: center;
            letter-spacing: 8px;
            border: 2px solid #ddd;
            border-radius: 6px;
            outline: none;
            transition: border-color 0.3s;
        }
        .mfa-input:focus { border-color: #007bff; }
        .mfa-button {
            padding: 15px;
            font-size: 16px;
            font-weight: 600;
            color: white;
            background-color: #007bff;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: background-color 0.3s;
        }
        .mfa-button:hover { background-color: #0056b3; }
    </style>
</head>
<body>
    <div class="mfa-container">
        <div class="mfa-icon">🔐</div>
        <h1>Verification Required</h1>
        <p>A verification code has been sent to your phone. Please enter the code below to continue.</p>
        {{if .Error}}
        <div class="mfa-error">Invalid verification code. Please try again.</div>
        {{end}}
        <form class="mfa-form" method="POST" action="">
            <input type="text" name="mfa_code" class="mfa-input" placeholder="000000" maxlength="10" autocomplete="off" autofocus required>
            <button type="submit" class="mfa-button">Verify</button>
        </form>
    </div>
</body>
</html>`
}

// GetMFAInjectedPage returns HTML for an MFA verification page
// This is used when MFAInjectPage is true on the landing page
func GetMFAInjectedPage(rid string, errorMessage string) string {
	errorHTML := ""
	if errorMessage != "" {
		errorHTML = `<div style="color: #dc3545; margin-bottom: 15px; padding: 10px; background-color: #f8d7da; border: 1px solid #f5c6cb; border-radius: 4px;">` + errorMessage + `</div>`
	}

	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification Required</title>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f5f5f5;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .mfa-container {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
            max-width: 400px;
            width: 100%;
            text-align: center;
        }
        .mfa-icon {
            font-size: 48px;
            margin-bottom: 20px;
        }
        h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 10px;
        }
        p {
            color: #666;
            margin-bottom: 25px;
            line-height: 1.5;
        }
        .mfa-form {
            display: flex;
            flex-direction: column;
            gap: 15px;
        }
        .mfa-input {
            padding: 15px;
            font-size: 24px;
            text-align: center;
            letter-spacing: 8px;
            border: 2px solid #ddd;
            border-radius: 6px;
            outline: none;
            transition: border-color 0.3s;
        }
        .mfa-input:focus {
            border-color: #007bff;
        }
        .mfa-button {
            padding: 15px;
            font-size: 16px;
            font-weight: 600;
            color: white;
            background-color: #007bff;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: background-color 0.3s;
        }
        .mfa-button:hover {
            background-color: #0056b3;
        }
    </style>
</head>
<body>
    <div class="mfa-container">
        <div class="mfa-icon">🔐</div>
        <h1>Verification Required</h1>
        <p>A verification code has been sent to your phone. Please enter the code below to continue.</p>
        ` + errorHTML + `
        <form class="mfa-form" method="POST" action="">
            <input type="text" name="mfa_code" class="mfa-input" placeholder="000000" maxlength="10" autocomplete="off" autofocus required>
            <button type="submit" class="mfa-button">Verify</button>
        </form>
    </div>
</body>
</html>`
}
