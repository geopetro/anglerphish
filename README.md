![Anglerphish logo](https://raw.githubusercontent.com/geopetro/anglerphish/master/static/images/gophish_purple.png)

Anglerphish
=======

A feature-rich [Gophish](https://github.com/gophish/gophish) fork. Detailed presentation of all the Anglerphish features can be found in this article - *Coming Soon...*.

## 💖 Support This Project
Creating this project took a lot of time, effort — and money. If you find it useful, please consider supporting it by sponsoring or simply ⭐ starring this repository to show appreciation.

Additionally, if you want **access to the full feature set**, sponsoring unlocks the private version with the full-feature set.

---

## Version Comparison
Details of features can be found below the table:

| Feature                                   | Open Version | Full Version |
| ----------------------------------------- | ------------ | ------------ |
| **Per-Campaign URL Parameters**               | ✅            | ✅ |
| **`{{.Custom}}` Group Variable**              | ✅            | ✅ |
| **Campaign Summary Before Launching**         | ✅            | ✅ |
| **QR Code Generator**                         | ✅            | ✅ |
| **Group Export to CSV**                       | ✅            | ✅ |
| **Enhanced Reported Phishing Monitoring**     | ✅            | ✅ |
| **Non-Campaign Reports Page (IMAP)**          | ✅            | ✅ |
| **`X-Tracked` Header Support**                | ✅            | ✅ |
| **Default Landing Page**                          | ✅            | ✅ |
| **HTTP Basic Auth Landing Pages**             | ✅            | ✅ |
| **QR Email Embedding**                        | ✅            | ✅ |
| **Sneaky Gophish Tweaks**                     | ✅            | ✅ |
| **Campaign Sets**                         | ❌            | ✅ |
| **SMS Campaigns (Twilio/Vonage)**         | ❌            | ✅ |
| `{{.Phone}}` Group Variable for SMS       | ❌            | ✅ |
| **Multiple IMAP Profiles**                | ❌            | ✅ |
| **Email Replied Tracking**                  | ❌            | ✅ |
| **Export Reports (Word/Excel)**           | ❌            | ✅ |
| **Preview Templates (Email/SMS/Landing)** | ❌            | ✅ |
| **Dashboard Filtering by Campaign Type**  | ❌            | ✅ |

### This Version of Anglerphish includes:
- **Per-Campaign URL Parameters:** Allows unique URL parameters per campaign instead of a global `rid`.
- **Additional Group Variable:** Introduces `{{.Custom}}` for use in emails, landing pages, and attachments.
- **Campaign Summary Before Launching**: Provides a summarized overview of all configured parameters (targets, templates, landing pages, etc.) before launching the campaign to avoid misconfiguration.
- **Group Export:** Supports exporting user groups to `.csv` for easy backup and editing.
- **Reported Phishing Monitoring Enhancement:** Improved handling of reported phishing emails, now recognizing all variations of URL parameters across active campaigns.
- **Non-Campaign Reports Page:** Dedicated view for reported emails in the IMAP inbox that are unrelated to any Gophish campaign.
- **X-Tracked Header Handling:** Supports custom `POST` request containing the header `X-Tracked`.
  - When such a `POST` is made to the Anglerphish server, the system parses the parameters in the URL and generates a `.csv` log entry.
  - Example use cases: 
    - Macro-enabled `.doc` or `.xls` files that can’t be tracked directly through traditional campaigns..
    - Custom `POST` requests triggered by landing pages.
- **QR Code Generator:** Built-in tool to generate QR codes.
- **Default Landing Page:** A default Error 404 landing when visiting the domain based on [edermi/gophish_mods](https://github.com/edermi/gophish_mods/tree/master).
- **HTTP Basic Auth Landing Pages:** Enables basic authentication landing page campaigns based on [edermi/gophish_mods](https://github.com/edermi/gophish_mods/tree/master).
- **QR Code Embedding in Campaigns:** Integrates QR code campaigns, based on [Evil-Gophish](https://github.com/fin3ss3g0d/evilgophish.git).
- **Sneaky Tweaks:** Implements the sneaky gophish tweaks based on the [article](https://www.sprocketsecurity.com/resources/never-had-a-bad-day-phishing-how-to-set-up-gophish-to-evade-security-controls).

### Anglerphish Full Version Features include:
**All the above features plus,** 
- **Campaign Sets:** Introduced the Campaign Sets feature, enabling the creation and configuration of multiple campaigns simultaneously. Users can save these campaigns as drafts, make modifications as needed, and launch them all at once.
- **SMS Campaigns:** Added support for SMS-based campaigns alongside email. Includes dedicated SMS profiles (Twilio and Vonage) and SMS template creation.
- **Additional Group Variable:** Introduced `{{.Phone}}` as a group variable to support SMS messaging.
- **Multiple IMAP Configurations:** Supports adding and managing multiple IMAP server profiles instead of being limited to a single configuration.
  - Additionally two types of Configurations are supported Email Replied and Email Reported.
- **Email Replied:** Tracks when users reply to phishing emails (with additional chart in campaigns) — recognizing that replies can also result in sensitive data disclosure, not just clicks or form submissions.
- **Reports Page:** New reporting feature to export campaign results and metrics as Word or Excel files, with Privacy Options to anonymize results.
- **Preview Templates / Landing Pages**: Added the ability to preview Email, SMS, and Landing Page Templates directly—no need to open the editor.
- **Dashboard Filtering:** Allows filtering the campaign list on the dashboard to show only Email or only SMS campaigns.

---

### Potential New Additions (Not all guaranteed):
- **MS Teams Campaign Integration**
- **Dark Theme** 
- **Randomized Email Template Sending to Targets**
- **Simple MFA Campaigns Using SMS Functionality**
- **Evilginx Integration**

---

### Screenshots

![1](https://raw.githubusercontent.com/geopetro/anglerphish/master/static/images/1.jpg)
![2](https://raw.githubusercontent.com/geopetro/anglerphish/master/static/images/2.jpg)
![3](https://raw.githubusercontent.com/geopetro/anglerphish/master/static/images/3.jpg)


## A fork based on original Gophish v0.12.1:

![Build Status](https://github.com/gophish/gophish/workflows/CI/badge.svg) [![GoDoc](https://godoc.org/github.com/gophish/gophish?status.svg)](https://godoc.org/github.com/gophish/gophish)

### Gophish: Open-Source Phishing Toolkit

[Gophish](https://getgophish.com) is an open-source phishing toolkit designed for businesses and penetration testers. It provides the ability to quickly and easily setup and execute phishing engagements and security awareness training.

### Install

Installation of Gophish remains dead-simple - just download and extract the zip containing the [release for your system](https://github.com/geopetro/anglerfish-full/releases/), and run the binary. Gophish has binary releases for Windows, Mac, and Linux platforms.

### Building From Source

To build Anglerphish from source, simply run ```git clone https://github.com/geopetro/anglerphish-full.git``` and ```cd``` into the project source directory. Then, run ```go build```. After this, you should have a binary called ```gophish``` in the current directory.

### Setup
After running the Gophish binary, open an Internet browser to https://localhost:3333 and login with the default username and password listed in the log output.
e.g.
```
time="2020-07-29T01:24:08Z" level=info msg="Please login with the username admin and the password 4304d5255378177d"
```
### Documentation

Documentation of the original gophish can be found on the official [site](http://getgophish.com/documentation).

### Issues

Find a bug? Want more features? Find something missing in the documentation? Let us know! Please don't hesitate to [file an issue](https://github.com/gophish/gophish/issues/new) and we'll get right on it.

### License
```
Gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 - 2020 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software ("Gophish Community Edition") and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```
