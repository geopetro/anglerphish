# Changelog

All notable changes to Anglerphish are documented here.

---

## [1.3.0] - 2026-07-27

### Added
- **Admin SSO (OIDC)** - Optional OIDC login for the admin UI, compatible with any standard provider (Keycloak, Microsoft Entra ID, and others). Users in a configured IdP group are mapped to pre-provisioned local accounts. The `admin` account retains password login as a break-glass fallback. *(contributed by [@audrey0042](https://github.com/audrey0042))*
- **Campaign Set Overview** - New Overview tab on launched campaign sets showing set-wide statistics rolled up two ways — cross-campaign totals and unique contacts — alongside a per-campaign breakdown that links to full results. Backed by a new `GET /api/campaign_sets/:id/summary` API endpoint.
- **Bulk complete campaigns** - The Campaigns page gains a "Complete Selected" button alongside the existing "Delete Selected", so multiple in-progress campaigns can be marked complete at once from the checkbox selection. The button appears only on the Active Campaigns tab (already-completed campaigns live on the Archived tab), and completes each selected campaign independently, reporting how many succeeded.
- **Message viewer in the IMAP Monitor** - Reported emails can be read in-app. Message bodies are not stored, they are fetched from the mailbox on demand by UID (with a Message-ID fallback) and rendered in a sandboxed iframe with an independent CSP, remote images off by default and links neutralized. Reports created before this release show a disabled View button.
- **Replies tab in the IMAP Monitor** - Replies to simulated phishing emails are listed with their captured content, filterable by campaign. Capture is controlled per IMAP configuration by the new "Store Reply Content" setting (on by default, reply tracking only). Body is capped at 256KB and headers at a separate 16KB, both encrypted at rest alongside the rest of the event details. Headers are captured at read time because reply events carry no IMAP identifier to re-fetch with, and the message may be deleted from the mailbox afterwards. Replies captured before this release have no headers stored.
- **New themes** 
  - A fun visual update introducing four new themes — **two light** and **two dark**. All are available under **Settings → Theme**.
    - **Goldphish theme** - A bright, warm light theme inspired by goldfish.
    - **Lagocephalus theme** - A cool, marine-inspired dark theme with a metallic feel.
    - **Sand theme (Light)** - A warm, earthy light theme built around soft natural tones.
    - **Matrix theme (Dark)** - A green take on Dark Crimson, inspired by digital rain.

### Improved
- **Campaign set performance:** viewing a launched campaign set now loads statistics in a single request instead of one per campaign, and indexes `results` and `events` by campaign for faster stats on larger datasets.
- **Campaign set editor UI:** the Campaigns split view now has a fixed height with the campaign list and details panels scrolling independently, so adding campaigns no longer stretches the modal past the viewport.

### Fixed
- Report anonymization no longer leaks submitted credentials. With "Anonymize emails" enabled, the captured payload (the email/password a target typed into the landing page) was still written into the Details column of the event tables in Excel and Word reports, defeating the masking applied to the Contact and IP columns. The captured values are now redacted (`field: "[REDACTED]"`) while the field names and IP/location context are preserved.
- Selecting a campaign with a long name in a campaign set no longer causes a slight panel resize.
- IMAP `RestrictDomain` never matched senders whose From header included a display name (`Jane Doe <jane@corp.com>`), silently discarding those reports and marking them read. Deployments using `RestrictDomain` will see report volume rise as a result. Mail discarded before this fix is unrecoverable, as it was already marked as seen.

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

### Removed
- Removed GoPhish transparency handler, `X-Server`, `X-Mailer`, and `X-Contact` headers *(contributed by [@mrnfrancesco](https://github.com/mrnfrancesco))*

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
- Default 404 landing page

---

*Anglerphish is a fork of [Gophish](https://github.com/gophish/gophish) by Jordan Wright, originally at v0.12.1.*
