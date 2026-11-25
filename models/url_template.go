package models

import (
	"sort"
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

// GetURLTemplates returns the user's own templates by name, then the presets
// in the order of presetURLTemplates
func GetURLTemplates(uid int64) ([]URLTemplate, error) {
	templates := []URLTemplate{}
	err := db.Where("user_id = ? OR is_preset = ?", uid, true).
		Order("is_preset ASC, name ASC").
		Find(&templates).Error
	if err != nil {
		return templates, err
	}
	position := make(map[string]int, len(presetURLTemplates))
	for i, p := range presetURLTemplates {
		position[p.Name] = i
	}
	sort.SliceStable(templates, func(i, j int) bool {
		a, b := templates[i], templates[j]
		if a.IsPreset != b.IsPreset {
			return !a.IsPreset
		}
		return a.IsPreset && position[a.Name] < position[b.Name]
	})
	return templates, nil
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

// presetURLTemplates is the built-in list, in the order the picker shows it.
// Every host is the [DOMAIN] placeholder, to be replaced with the phishing domain.
var presetURLTemplates = []URLTemplate{
	// Microsoft 365
	{Name: "Microsoft 365 Sign-in", URL: "https://[DOMAIN]/common/oauth2/v2.0/authorize?client_id=4765445b-32c6-49b0-83e6-1d93765276ca&response_type=code&scope=openid", Category: "Microsoft 365", IsPreset: true},
	{Name: "Outlook Mail", URL: "https://[DOMAIN]/mail/inbox/id/AAQkAGI2", Category: "Microsoft 365", IsPreset: true},
	{Name: "Microsoft Password Reset", URL: "https://[DOMAIN]/passwordreset?ru=%2Fportal", Category: "Microsoft 365", IsPreset: true},
	{Name: "Microsoft MFA Security Info", URL: "https://[DOMAIN]/security-info?tenant=common", Category: "Microsoft 365", IsPreset: true},
	{Name: "SharePoint Document", URL: "https://[DOMAIN]/:w:/r/sites/Finance/_layouts/15/Doc.aspx?sourcedoc=%7B8A9B2C3D%7D", Category: "Microsoft 365", IsPreset: true},
	{Name: "OneDrive Shared File", URL: "https://[DOMAIN]/:x:/g/personal/finance/EaB3kL9mQ2x", Category: "Microsoft 365", IsPreset: true},
	{Name: "Teams Meeting", URL: "https://[DOMAIN]/l/meetup-join/19%3ameeting_NjM5Yz/0?context=%7b%22Tid%22%7d", Category: "Microsoft 365", IsPreset: true},

	// Google Workspace
	{Name: "Google Sign-in", URL: "https://[DOMAIN]/v3/signin/identifier?flowName=GlifWebSignIn", Category: "Google Workspace", IsPreset: true},
	{Name: "Gmail", URL: "https://[DOMAIN]/mail/u/0/#inbox", Category: "Google Workspace", IsPreset: true},
	{Name: "Google Drive File", URL: "https://[DOMAIN]/file/d/1a2B3c4D5e6F/view?usp=sharing", Category: "Google Workspace", IsPreset: true},
	{Name: "Google Docs Document", URL: "https://[DOMAIN]/document/d/1BxiMVs0XRA5nFMd/edit?usp=sharing", Category: "Google Workspace", IsPreset: true},
	{Name: "Google Security Alert", URL: "https://[DOMAIN]/notifications/security?hl=en", Category: "Google Workspace", IsPreset: true},
	{Name: "Google 2-Step Challenge", URL: "https://[DOMAIN]/signin/challenge/totp/2?TL=AO-Mg3x8Y7w6V", Category: "Google Workspace", IsPreset: true},

	// File Sharing & E-Signature
	{Name: "DocuSign Envelope", URL: "https://[DOMAIN]/Signing/EmailStart.aspx?a=7f3c9e21&acct=4b8d", Category: "File Sharing & E-Signature", IsPreset: true},
	{Name: "Adobe Acrobat Sign", URL: "https://[DOMAIN]/public/esign/verify?tx=CBJCHBCAABAA", Category: "File Sharing & E-Signature", IsPreset: true},
	{Name: "Dropbox File", URL: "https://[DOMAIN]/scl/fi/8h4k9m2p/Q4_Report.pdf?rlkey=9n2m4k7j&dl=0", Category: "File Sharing & E-Signature", IsPreset: true},
	{Name: "Box File", URL: "https://[DOMAIN]/s/7h3k9m4n2p5q8r6t", Category: "File Sharing & E-Signature", IsPreset: true},
	{Name: "WeTransfer Download", URL: "https://[DOMAIN]/downloads/4f7h2k9m3n6p/9d2f5h7j", Category: "File Sharing & E-Signature", IsPreset: true},
	{Name: "Shared Document", URL: "https://[DOMAIN]/share/doc/view?id=DOC-48213", Category: "File Sharing & E-Signature", IsPreset: true},

	// Meetings & Voicemail
	{Name: "Zoom Meeting", URL: "https://[DOMAIN]/j/84127356021?pwd=bXk3dFZ", Category: "Meetings & Voicemail", IsPreset: true},
	{Name: "Webex Meeting", URL: "https://[DOMAIN]/meet/j.php?MTID=m8f2c4a", Category: "Meetings & Voicemail", IsPreset: true},
	{Name: "Voicemail Message", URL: "https://[DOMAIN]/voicemail/play?msg=VM-20931&dur=0:42", Category: "Meetings & Voicemail", IsPreset: true},
	{Name: "Fax Received", URL: "https://[DOMAIN]/fax/view?id=FX-55120&pages=3", Category: "Meetings & Voicemail", IsPreset: true},

	// HR & Payroll
	{Name: "Workday", URL: "https://[DOMAIN]/wday/authgwy/company/login.htmld", Category: "HR & Payroll", IsPreset: true},
	{Name: "ADP Portal", URL: "https://[DOMAIN]/portal/login.html", Category: "HR & Payroll", IsPreset: true},
	{Name: "BambooHR", URL: "https://[DOMAIN]/login.php?r=%2Fhome%2F", Category: "HR & Payroll", IsPreset: true},
	{Name: "Payslip", URL: "https://[DOMAIN]/payroll/payslip/view?period=current", Category: "HR & Payroll", IsPreset: true},
	{Name: "Benefits Enrollment", URL: "https://[DOMAIN]/benefits/open-enrollment/start", Category: "HR & Payroll", IsPreset: true},
	{Name: "Policy Acknowledgement", URL: "https://[DOMAIN]/policies/acknowledge?doc=POL-114", Category: "HR & Payroll", IsPreset: true},
	{Name: "Performance Review", URL: "https://[DOMAIN]/reviews/self-assessment?cycle=annual", Category: "HR & Payroll", IsPreset: true},

	// IT & Remote Access
	{Name: "Okta Sign-in", URL: "https://[DOMAIN]/login/login.htm?fromURI=%2Fapp%2FUserHome", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Help Desk Ticket", URL: "https://[DOMAIN]/ticket/view?req=INC0098765", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Identity Verification", URL: "https://[DOMAIN]/verify-identity?token=a1b2c3d4e5f6", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Password Expiry", URL: "https://[DOMAIN]/account/password/expiring?days=1", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Mailbox Storage Full", URL: "https://[DOMAIN]/mailbox/storage?usage=98", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Device Compliance", URL: "https://[DOMAIN]/device/compliance?status=noncompliant", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Cisco VPN", URL: "https://[DOMAIN]/+CSCOE+/logon.html", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Citrix Gateway", URL: "https://[DOMAIN]/Citrix/StoreWeb/", Category: "IT & Remote Access", IsPreset: true},
	{Name: "Remote Desktop Web", URL: "https://[DOMAIN]/RDWeb/Pages/en-US/login.aspx", Category: "IT & Remote Access", IsPreset: true},

	// Finance & Payments
	{Name: "Invoice Portal", URL: "https://[DOMAIN]/invoices/view?inv=INV-30481", Category: "Finance & Payments", IsPreset: true},
	{Name: "Payment Confirmation", URL: "https://[DOMAIN]/payments/confirm?ref=PMT-77210", Category: "Finance & Payments", IsPreset: true},
	{Name: "Online Banking", URL: "https://[DOMAIN]/online-banking/login?ref=security_alert", Category: "Finance & Payments", IsPreset: true},
	{Name: "PayPal", URL: "https://[DOMAIN]/signin?returnUri=%2Fmyaccount%2Ftransactions%2F", Category: "Finance & Payments", IsPreset: true},
	{Name: "Stripe", URL: "https://[DOMAIN]/login?redirect=%2Fpayments", Category: "Finance & Payments", IsPreset: true},
	{Name: "Amazon Order", URL: "https://[DOMAIN]/ap/signin?openid.return_to=%2Forders", Category: "Finance & Payments", IsPreset: true},

	// Delivery & SMS Short Links
	{Name: "Parcel Tracking", URL: "https://[DOMAIN]/track?id=JD014600006281", Category: "Delivery & SMS Short Links", IsPreset: true},
	{Name: "Delivery Reschedule", URL: "https://[DOMAIN]/delivery/reschedule?awb=7810394", Category: "Delivery & SMS Short Links", IsPreset: true},
	{Name: "Customs Fee", URL: "https://[DOMAIN]/customs/pay?ref=CF-2209", Category: "Delivery & SMS Short Links", IsPreset: true},
	{Name: "Short Link", URL: "https://[DOMAIN]/d/8Kx3", Category: "Delivery & SMS Short Links", IsPreset: true},
	{Name: "Short Link with Code", URL: "https://[DOMAIN]/s/Qm7v?c=41", Category: "Delivery & SMS Short Links", IsPreset: true},

	// Social & SaaS
	{Name: "LinkedIn", URL: "https://[DOMAIN]/login?trk=guest_homepage-basic_nav-header-signin", Category: "Social & SaaS", IsPreset: true},
	{Name: "Facebook", URL: "https://[DOMAIN]/login/?next=%2F", Category: "Social & SaaS", IsPreset: true},
	{Name: "Instagram", URL: "https://[DOMAIN]/accounts/login/?next=%2Fdirect%2Finbox%2F", Category: "Social & SaaS", IsPreset: true},
	{Name: "X (Twitter)", URL: "https://[DOMAIN]/i/flow/login?redirect_after_login=%2Fhome", Category: "Social & SaaS", IsPreset: true},
	{Name: "Slack", URL: "https://[DOMAIN]/ssb/signin_redirect?team=T04A2", Category: "Social & SaaS", IsPreset: true},
	{Name: "Salesforce", URL: "https://[DOMAIN]/?startURL=%2Fhome%2Fhome.jsp", Category: "Social & SaaS", IsPreset: true},

	// Cloud & Developer
	{Name: "AWS Sign-in", URL: "https://[DOMAIN]/signin?Action=login&Destination=%2Fconsole", Category: "Cloud & Developer", IsPreset: true},
	{Name: "Azure Portal", URL: "https://[DOMAIN]/#view/Microsoft_AAD_IAM/ActiveDirectoryMenuBlade", Category: "Cloud & Developer", IsPreset: true},
	{Name: "Google Cloud Console", URL: "https://[DOMAIN]/home/dashboard?project=prod-387214", Category: "Cloud & Developer", IsPreset: true},
	{Name: "GitHub Device Verification", URL: "https://[DOMAIN]/sessions/verified-device", Category: "Cloud & Developer", IsPreset: true},

	// Crypto
	{Name: "Coinbase", URL: "https://[DOMAIN]/signin?return_to=%2Fhome", Category: "Crypto", IsPreset: true},
	{Name: "Binance", URL: "https://[DOMAIN]/en/login?return_to=aHR0cHM6Ly93d3cu", Category: "Crypto", IsPreset: true},
	{Name: "Kraken", URL: "https://[DOMAIN]/sign-in?redirect_url=%2Faccount", Category: "Crypto", IsPreset: true},
	{Name: "Ledger Update", URL: "https://[DOMAIN]/ledger-live/update?utm_source=email", Category: "Crypto", IsPreset: true},
	{Name: "Wallet Security Alert", URL: "https://[DOMAIN]/wallet/security/activity?alert=8h7g6f", Category: "Crypto", IsPreset: true},

	// Generic Patterns
	{Name: "Generic Login", URL: "https://[DOMAIN]/login", Category: "Generic Patterns", IsPreset: true},
	{Name: "Generic Sign-In", URL: "https://[DOMAIN]/signin", Category: "Generic Patterns", IsPreset: true},
	{Name: "Generic Auth Login", URL: "https://[DOMAIN]/auth/login", Category: "Generic Patterns", IsPreset: true},
	{Name: "Generic App Login", URL: "https://[DOMAIN]/#/login", Category: "Generic Patterns", IsPreset: true},
	{Name: "Generic Secure Login", URL: "https://[DOMAIN]/auth?session=8f7e6d5c4b3a2190", Category: "Generic Patterns", IsPreset: true},
	{Name: "Account Verification", URL: "https://[DOMAIN]/confirm?token=9k8j7h6g5f4d3s2a1z0y", Category: "Generic Patterns", IsPreset: true},
	{Name: "Update Information", URL: "https://[DOMAIN]/user/verify?code=a1b2c3d4e5f6g7h8", Category: "Generic Patterns", IsPreset: true},
	{Name: "Action Required", URL: "https://[DOMAIN]/alert/action-required?ref=n_5d4c3b2a", Category: "Generic Patterns", IsPreset: true},
}

// EnsurePresetURLTemplates syncs the stored presets with presetURLTemplates
func EnsurePresetURLTemplates(db *gorm.DB) error {
	presets := presetURLTemplates
	names := make([]string, len(presets))
	for i, preset := range presets {
		names[i] = preset.Name
	}
	// Presets dropped from this list would otherwise keep their old URLs
	err := db.Where("is_preset = ? AND name NOT IN (?)", true, names).Delete(&URLTemplate{}).Error
	if err != nil {
		return err
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
		} else if err != nil {
			return err
		} else if existing.URL != preset.URL || existing.Category != preset.Category {
			// Older installs seeded presets that pointed at the real services
			err = db.Model(&existing).Updates(map[string]interface{}{
				"url":        preset.URL,
				"category":   preset.Category,
				"updated_at": time.Now().UTC(),
			}).Error
			if err != nil {
				return err
			}
		}
	}

	return nil
}
