package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

// URLTemplate represents a URL template for campaigns
type URLTemplate struct {
	Id        int64     `json:"id"`
	UserId    int64     `json:"-" gorm:"column:user_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url" gorm:"column:url"`
	Category  string    `json:"category"`
	IsPreset  bool      `json:"is_preset" gorm:"column:is_preset"`
	CreatedAt time.Time `json:"created_date"`
	UpdatedAt time.Time `json:"modified_date"`
}

// GetURLTemplates returns the URL templates for a given user (including presets)
func GetURLTemplates(uid int64) ([]URLTemplate, error) {
	templates := []URLTemplate{}
	err := db.Where("user_id = ? OR is_preset = ?", uid, true).
		Order("is_preset DESC, category ASC, name ASC").
		Find(&templates).Error
	return templates, err
}

// GetURLTemplate returns the URL template with the given ID
func GetURLTemplate(id int64, uid int64) (URLTemplate, error) {
	template := URLTemplate{}
	err := db.Where("id = ? AND (user_id = ? OR is_preset = ?)", id, uid, true).
		First(&template).Error
	return template, err
}

// PostURLTemplate creates a new custom URL template for a user
func PostURLTemplate(template *URLTemplate) error {
	// Ensure it's not marked as preset (only system can create presets)
	template.IsPreset = false
	template.CreatedAt = time.Now().UTC()
	template.UpdatedAt = time.Now().UTC()
	err := db.Save(template).Error
	return err
}

// PutURLTemplate updates a custom URL template
func PutURLTemplate(template *URLTemplate, uid int64) error {
	// Ensure user can only update their own non-preset templates
	err := db.Where("id = ? AND user_id = ? AND is_preset = ?", template.Id, uid, false).
		First(&URLTemplate{}).Error
	if err != nil {
		return err
	}
	template.UpdatedAt = time.Now().UTC()
	err = db.Save(template).Error
	return err
}

// DeleteURLTemplate deletes a custom URL template
func DeleteURLTemplate(id int64, uid int64) error {
	// Ensure user can only delete their own non-preset templates
	err := db.Where("id = ? AND user_id = ? AND is_preset = ?", id, uid, false).
		Delete(&URLTemplate{}).Error
	return err
}

// EnsurePresetURLTemplates creates preset URL templates if they don't exist
func EnsurePresetURLTemplates(db *gorm.DB) error {
	presets := []URLTemplate{
		// Authentication & Login
		{UserId: 0, Name: "Microsoft 365 Login", URL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=4345a7b9-9a63-4910-a426-35363201d503", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Outlook Web Access", URL: "https://outlook.office365.com/mail/inbox/id/AAMkAGI1AA=", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Microsoft Account Security", URL: "https://account.microsoft.com/password/reset?mkt=en-US", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Google Sign-In", URL: "https://accounts.google.com/signin/v2/identifier?flowName=GlifWebSignIn", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Gmail Login", URL: "https://accounts.google.com/ServiceLogin?continue=https://mail.google.com", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Google Account Security", URL: "https://myaccount.google.com/security/signinchecker/9f3a827b", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Apple ID Login", URL: "https://appleid.apple.com/account/manage", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Facebook Login", URL: "https://www.facebook.com/login/?next=https%3A%2F%2Fwww.facebook.com%2F", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "LinkedIn Login", URL: "https://www.linkedin.com/uas/login?session_redirect=%2Ffeed%2F", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Twitter Login", URL: "https://twitter.com/i/flow/login?redirect_after_login=%2Fhome", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Instagram Login", URL: "https://www.instagram.com/accounts/login/?next=%2Fdirect%2Finbox%2F", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "Okta SSO", URL: "https://[COMPANY].okta.com/login/login.htm", Category: "Authentication & Login", IsPreset: true},
		{UserId: 0, Name: "OneLogin SSO", URL: "https://[COMPANY].onelogin.com/login", Category: "Authentication & Login", IsPreset: true},

		// Document & File Access
		{UserId: 0, Name: "Office Online Document", URL: "https://view.officeapps.live.com/op/view.aspx?src=https://[DOMAIN]/document.docx", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "Google Docs", URL: "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "Google Drive File", URL: "https://drive.google.com/file/d/1a2B3c4D5e6F7g8H9i0J/view?usp=sharing", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "Dropbox Shared File", URL: "https://www.dropbox.com/s/3k9m2n1o5p7q8r4s/Annual_Report_2024.pdf?dl=0", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "Dropbox Share Link", URL: "https://www.dropbox.com/scl/fi/8h4k9m2p5q7r3s6t/document.pdf?rlkey=9n2m4k7j&dl=0", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "SharePoint Document", URL: "https://[TENANT].sharepoint.com/:w:/r/sites/[SITE]/_layouts/15/Doc.aspx?sourcedoc=%7B8A9B2C3D%7D", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "OneDrive Personal File", URL: "https://[TENANT]-my.sharepoint.com/personal/john_doe_company_com/Documents/Q4_Financials.xlsx", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "OneDrive Share Link", URL: "https://1drv.ms/w/s!Ak5F6B7C8D9E0F1G2H3I4J5K6L7M", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "Box Shared File", URL: "https://app.box.com/s/7h3k9m4n2p5q8r6t", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "WeTransfer Download", URL: "https://wetransfer.com/downloads/4f7h2k9m3n6p8q5r/9d2f5h7j4k8m3n6p", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "MEGA File Share", URL: "https://mega.nz/file/8K4J2M5N#6H9k4m7p2q5r8t3v", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "MediaFire File", URL: "https://www.mediafire.com/file/5h8k2m9n3p6q4r7t/document.pdf/file", Category: "Document & File Access", IsPreset: true},
		{UserId: 0, Name: "iCloud Drive", URL: "https://www.icloud.com/iclouddrive/7k3m9n2p5q8r4t6v", Category: "Document & File Access", IsPreset: true},

		// Security & Verification
		{UserId: 0, Name: "Microsoft Security Alert", URL: "https://account.microsoft.com/security/suspicious-activity?ref=alert_5d3f2a1b", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Google Security Alert", URL: "https://myaccount.google.com/notifications/suspiciousactivity?token=8h7g6f5d4s3a2", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Microsoft Password Reset", URL: "https://passwordreset.microsoftonline.com/?ru=https%3A%2F%2Fportal.office.com", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Google Password Reset", URL: "https://accounts.google.com/signin/v2/challenge/pwd?TL=AH-1Ng2X7k5Y4z3W2v1U0t9S", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Apple Password Reset", URL: "https://iforgot.apple.com/password/verify/appleid", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Microsoft MFA Verification", URL: "https://mysignins.microsoft.com/security-info/mfa/verify?code=892341", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Google 2FA Challenge", URL: "https://account.google.com/signin/v2/challenge/selection?TL=AO-Mg3x8Y7w6V", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "PayPal Security Alert", URL: "https://secure.paypal.com/us/webapps/mpp/security/unusual-activity?id=9k8j7h6g5f4d3s2a1", Category: "Security & Verification", IsPreset: true},
		{UserId: 0, Name: "Amazon Security Alert", URL: "https://www.amazon.com/ap/signin?openid.return_to=%2Fyour-account%2Fsecurity-alert", Category: "Security & Verification", IsPreset: true},

		// Internal Systems
		{UserId: 0, Name: "ADP Payroll Portal", URL: "https://[COMPANY].adp.com/portal/login.html", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "Workday Login", URL: "https://[COMPANY].workday.com/signin", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "BambooHR Portal", URL: "https://[COMPANY].bamboohr.com/login.php?r=%2Fhome%2F", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "HR Benefits Portal", URL: "https://hr.[COMPANY].com/benefits/enrollment/2024?emp_id=12345", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "Employee W-2 Access", URL: "https://portal.[COMPANY].com/employee/payroll/w2/view?year=2024", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "Corporate Intranet", URL: "https://intranet.[COMPANY].com/news/announcement?id=5678", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "Company Portal Login", URL: "https://portal.[COMPANY].com/login?returnUrl=%2Fdashboard", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "IT Help Desk Ticket", URL: "https://helpdesk.[COMPANY].com/ticket/reset-password?req=9876543", Category: "Internal Systems", IsPreset: true},
		{UserId: 0, Name: "IT Identity Verification", URL: "https://support.[COMPANY].com/verify-identity?token=a1b2c3d4e5f6", Category: "Internal Systems", IsPreset: true},

		// VPN & Remote Access
		{UserId: 0, Name: "Cisco AnyConnect VPN", URL: "https://vpn.[COMPANY].com/+CSCOE+/logon.html", Category: "VPN & Remote Access", IsPreset: true},
		{UserId: 0, Name: "Citrix Gateway", URL: "https://remote.[COMPANY].com/Citrix/StoreWeb/", Category: "VPN & Remote Access", IsPreset: true},
		{UserId: 0, Name: "Remote Desktop Web", URL: "https://rdp.[COMPANY].com/RDWeb/Pages/en-US/login.aspx", Category: "VPN & Remote Access", IsPreset: true},

		// Financial Services
		{UserId: 0, Name: "Chase Online Banking", URL: "https://secure.chase.com/web/auth/#/logon/logon/chaseOnline", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "Wells Fargo Login", URL: "https://www.wellsfargo.com/verify-identity?token=8h7g6f5d4s", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "Bank of America", URL: "https://onlinebanking.bankofamerica.com/login?ref=security_alert", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "PayPal Login", URL: "https://www.paypal.com/signin?returnUri=%2Fmyaccount%2Ftransactions%2F", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "Stripe Dashboard", URL: "https://dashboard.stripe.com/login?redirect=%2Fpayments", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "Venmo Verification", URL: "https://secure.venmo.com/account/verify-transaction?id=v1_8h7g6f5d", Category: "Financial Services", IsPreset: true},
		{UserId: 0, Name: "Amazon Account", URL: "https://www.amazon.com/ap/signin?_encoding=UTF8&openid.return_to=%2Forders", Category: "Financial Services", IsPreset: true},

		// Password Managers
		{UserId: 0, Name: "Bitwarden Web Vault", URL: "https://vault.bitwarden.com/#/login", Category: "Password Managers", IsPreset: true},
		{UserId: 0, Name: "Vaultwarden Login", URL: "https://vault.[COMPANY].com/#/login", Category: "Password Managers", IsPreset: true},
		{UserId: 0, Name: "1Password Sign-In", URL: "https://my.1password.com/signin", Category: "Password Managers", IsPreset: true},
		{UserId: 0, Name: "LastPass Login", URL: "https://lastpass.com/?ac=1&lpnorefresh=1", Category: "Password Managers", IsPreset: true},
		{UserId: 0, Name: "Dashlane Web App", URL: "https://app.dashlane.com/login", Category: "Password Managers", IsPreset: true},
		{UserId: 0, Name: "Keeper Security", URL: "https://keepersecurity.com/vault/", Category: "Password Managers", IsPreset: true},

		// Cloud Services
		{UserId: 0, Name: "AWS Console", URL: "https://console.aws.amazon.com/console/home?region=us-east-1", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "AWS Sign-In", URL: "https://signin.aws.amazon.com/console?account_id=123456789012", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "Azure Portal", URL: "https://portal.azure.com/#@[TENANT]/resource/subscriptions/1a2b3c4d", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "Azure AD Portal", URL: "https://aad.portal.azure.com/#blade/Microsoft_AAD_IAM", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "Google Cloud Console", URL: "https://console.cloud.google.com/home/dashboard?project=[PROJECT_ID]", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "GCP IAM & Admin", URL: "https://console.cloud.google.com/iam-admin/iam?project=[PROJECT_ID]", Category: "Cloud Services", IsPreset: true},
		{UserId: 0, Name: "GCP Compute Engine", URL: "https://console.cloud.google.com/compute/instances?project=[PROJECT_ID]", Category: "Cloud Services", IsPreset: true},

		// Cryptocurrency Services
		{UserId: 0, Name: "Coinbase Login", URL: "https://login.coinbase.com/?return_to=%2Fdashboard", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Coinbase Security Alert", URL: "https://www.coinbase.com/settings/security-activity?alert_id=8h7g6f5d", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Binance Login", URL: "https://accounts.binance.com/en/login?return_to=aHR0cHM6Ly93d3cuYmluYW5jZS5jb20v", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Binance Verification", URL: "https://www.binance.com/en/my/security/verify-identity?ref=9k8j7h6g", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Kraken Sign-In", URL: "https://www.kraken.com/sign-in?redirect_url=%2Faccount", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Kraken Security Notice", URL: "https://www.kraken.com/u/security/activity?notice_id=5d4c3b2a", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Crypto.com Login", URL: "https://crypto.com/exchange/signin?redirect=%2Fexchange", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Gemini Login", URL: "https://exchange.gemini.com/signin", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Gemini 2FA Verification", URL: "https://exchange.gemini.com/verify-device?token=a1b2c3d4", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Blockchain Wallet", URL: "https://login.blockchain.com/en/#/login", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Blockchain Security Alert", URL: "https://www.blockchain.com/wallet/security-center?alert=unusual_activity", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "MetaMask Verification", URL: "https://metamask.io/verify-transaction?tx=0x8h7g6f5d4s3a2z1y", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "KuCoin Login", URL: "https://www.kucoin.com/login?redirect_url=%2Faccount", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Bitfinex Sign-In", URL: "https://www.bitfinex.com/login?return_to=%2Ft%2FBTC%3AUSD", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Bittrex Login", URL: "https://bittrex.com/Account/Login?ReturnUrl=%2FMarket%2FIndex", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Bitstamp Account", URL: "https://www.bitstamp.net/account/login/?next=%2Faccount%2Fbalance%2F", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Ledger Live App", URL: "https://www.ledger.com/ledger-live/download?utm_source=email&utm_medium=security", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Trezor Wallet", URL: "https://suite.trezor.io/web/", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Trust Wallet Security", URL: "https://trustwallet.com/security-alert?ref=8k7j6h5g", Category: "Cryptocurrency Services", IsPreset: true},
		{UserId: 0, Name: "Exodus Wallet", URL: "https://www.exodus.com/download/?utm_source=notification", Category: "Cryptocurrency Services", IsPreset: true},

		// Generic Patterns
		{UserId: 0, Name: "Generic Secure Login", URL: "https://secure-login-verify.com/auth?session=8f7e6d5c4b3a2190", Category: "Generic Patterns", IsPreset: true},
		{UserId: 0, Name: "Account Verification", URL: "https://verify-account.[DOMAIN]/confirm?token=9k8j7h6g5f4d3s2a1z0y", Category: "Generic Patterns", IsPreset: true},
		{UserId: 0, Name: "Update Information", URL: "https://update-info.[DOMAIN]/user/verify?code=a1b2c3d4e5f6g7h8", Category: "Generic Patterns", IsPreset: true},
		{UserId: 0, Name: "Action Required", URL: "https://notification.[DOMAIN]/alert/action-required?ref=n_5d4c3b2a", Category: "Generic Patterns", IsPreset: true},
	}

	for _, preset := range presets {
		// Check if preset already exists
		var existing URLTemplate
		err := db.Where("name = ? AND is_preset = ?", preset.Name, true).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			// Create the preset
			preset.CreatedAt = time.Now().UTC()
			preset.UpdatedAt = time.Now().UTC()
			if err := db.Create(&preset).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
