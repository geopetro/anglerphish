// Package dmi writes campaign emails straight into mailboxes via Microsoft Graph and the Gmail API.
package dmi

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const (
	ProviderMicrosoft = "MS365_DMI"
	ProviderGoogle    = "GWS_DMI"
)

const (
	AuthClientSecret = "client_secret"
	AuthCertificate  = "certificate"
	AuthCustomerKey  = "customer_key"
	AuthOperatorKey  = "operator_key"
)

const (
	FolderInbox  = "inbox"
	FolderJunk   = "junk"
	FolderCustom = "custom"
)

// Endpoints overrides provider URLs, empty means production.
type Endpoints struct {
	MicrosoftLogin, Graph, GoogleToken, Gmail string
}

type Config struct {
	Provider, AuthMethod string
	TenantID, ClientID   string
	Secret               string // client secret | certificate private key PEM | customer service account JSON
	Certificate          string // public cert PEM (certificate method)
	OperatorKey          string // operator service account JSON (operator_key method)
	AllowedDomains       []string
	Folder, FolderName   string
	Endpoints            Endpoints
	HTTPClient           *http.Client // nil = default with 60s timeout
}

func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c Config) Validate() error {
	switch c.Provider {
	case ProviderMicrosoft:
		if c.AuthMethod != AuthClientSecret && c.AuthMethod != AuthCertificate {
			return errors.New("Auth method is not valid for Microsoft 365")
		}
		if strings.TrimSpace(c.TenantID) == "" {
			return errors.New("Tenant ID is required")
		}
		if strings.TrimSpace(c.ClientID) == "" {
			return errors.New("Client ID is required")
		}
		if c.Secret == "" {
			if c.AuthMethod == AuthCertificate {
				return errors.New("Certificate private key is required")
			}
			return errors.New("Client secret is required")
		}
		if c.AuthMethod == AuthCertificate && c.Certificate == "" {
			return errors.New("Certificate is required")
		}
	case ProviderGoogle:
		switch c.AuthMethod {
		case AuthCustomerKey:
			if c.Secret == "" {
				return errors.New("Customer service account key is required")
			}
		case AuthOperatorKey:
			if c.OperatorKey == "" {
				return errors.New("Set the operator service account in Sending Profiles > Service accounts first")
			}
		default:
			return errors.New("Auth method is not valid for Google Workspace")
		}
	default:
		return errors.New("Provider is not valid")
	}
	if len(c.domains()) == 0 {
		return errors.New("At least one allowed domain is required")
	}
	switch c.Folder {
	case "", FolderInbox, FolderJunk:
	case FolderCustom:
		if strings.TrimSpace(c.FolderName) == "" {
			return errors.New("Folder name is required for a custom folder")
		}
	default:
		return errors.New("Folder must be inbox, junk or custom")
	}
	return nil
}

func (c Config) domains() []string {
	var out []string
	for _, d := range c.AllowedDomains {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// CheckRecipient returns the normalised mailbox if its domain is allowed.
func (c Config) CheckRecipient(addr string) (string, error) {
	a, err := mail.ParseAddress(strings.TrimSpace(addr))
	if err != nil {
		return "", &Error{Err: errors.New("Recipient is not a valid email address")}
	}
	mailbox := strings.ToLower(a.Address)
	at := strings.LastIndex(mailbox, "@")
	domain := mailbox[at+1:]
	for _, d := range c.domains() {
		if d == domain {
			return mailbox, nil
		}
	}
	return "", &Error{Err: errors.New("Recipient domain " + domain + " is not in the allowed domains")}
}
