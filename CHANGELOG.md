# Changelog

All notable changes to Anglerphish are documented here.

---

## [Unreleased]

### Added
- **Admin SSO (OIDC)** - Optional OIDC login for the admin UI. Users in a configured IdP group are mapped to pre-provisioned local accounts. The `admin` account retains password login as a break-glass fallback. *(contributed by [@audrey0042](https://github.com/audrey0042))*

---

## [1.2.0] - 2026-06-19

### Added
- **Global Variables** - Define system-wide variables reusable across email/SMS templates and landing pages.
- **Group Locking** - Lock groups to prevent accidental inclusion to live campaigns.
- **Resend Failed Messages** - Re-queue all failed/errored emails or SMS messages in a campaign with one click, or resend to a single recipient from the results page.

### Improved
- Campaign set performance improvements.

### Fixed
- Removed automatic SMS retry backoff since manual resend was introduced.

---

## [1.1.0] - 2026-04-17

### Added
- **Default 404 Page Editor** - Fully editable default 404 landing page from the Settings UI.
- **Database Encryption** - AES-256-GCM encryption for sensitive database fields (SMTP passwords, SMS credentials, IMAP passwords, captured data).

### Improved
- MFA injection cleanup, editor tips, and code length label improvements.

### Fixed
- Linting and code quality improvements.
- More verbose error messages on template errors. *(contributed by [@mrnfrancesco](https://github.com/mrnfrancesco))*

---

## [1.0.0] - 2026-03-01

### Added
Initial Anglerphish release - a feature-rich fork of [Gophish v0.12.1](https://github.com/gophish/gophish).

**Campaign Management**
- Campaign Sets for multi-campaign creation and launch
- Per-campaign URL parameters
- Campaign summary before launching
- Dashboard filtering by campaign type (Email / SMS / Generic)
- Generic Campaigns for non-email/SMS delivery (QR codes, social media, etc.)
- QR Code Generator

**New Campaign Vectors**
- SMS Campaigns (Twilio and Vonage)
- MFA Simulation on landing pages
- HTTP Basic Auth landing pages
- QR Code embedding in campaigns
- Email Replied tracking

**Reporting & Tracking**
- Reports page - export results as Word or Excel with privacy/anonymization options
- Improved reported phishing monitoring across all URL parameter variations
- IMAP Monitor for non-campaign inbox emails
- X-Tracked header handling for macro/POST-based tracking
- Multiple IMAP configurations

**Templates & Groups**
- New template variables: `{{.Custom}}`, `{{.Phone}}`, `{{.CurrentDateTime}}`, `{{.CurrentDate}}`, `{{.CurrentTime}}`, `{{.CurrentTime24}}`
- URL Templates
- Template and landing page previews
- Group Export to CSV

**UI & UX**
- Dark Theme
- In-App API Documentation page

**Stealth**
- Removed GoPhish transparency handler, `X-Server`, `X-Mailer`, and `X-Contact` headers *(contributed by [@mrnfrancesco](https://github.com/mrnfrancesco))*
- Default 404 landing page

---

*Anglerphish is a fork of [Gophish](https://github.com/gophish/gophish) by Jordan Wright, originally at v0.12.1.*
