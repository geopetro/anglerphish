package dmi

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckRecipient(t *testing.T) {
	c := Config{AllowedDomains: []string{" Contoso.com "}}
	got, err := c.CheckRecipient(" Alice@Contoso.COM ")
	if err != nil || got != "alice@contoso.com" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"alice@sub.contoso.com", "not-an-address", "a@other.com"} {
		_, err := c.CheckRecipient(bad)
		var de *Error
		if !errors.As(err, &de) || de.Retryable() {
			t.Fatalf("%q: want permanent *Error, got %v", bad, err)
		}
	}
}

func TestValidate(t *testing.T) {
	ms := Config{Provider: ProviderMicrosoft, AuthMethod: AuthClientSecret, TenantID: "t", ClientID: "c", Secret: "s", AllowedDomains: []string{"a.com"}, Folder: FolderInbox}
	cert := ms
	cert.AuthMethod, cert.Certificate = AuthCertificate, "CERT"
	cust := Config{Provider: ProviderGoogle, AuthMethod: AuthCustomerKey, Secret: "{}", AllowedDomains: []string{"a.com"}, Folder: FolderJunk}
	op := Config{Provider: ProviderGoogle, AuthMethod: AuthOperatorKey, OperatorKey: "{}", AllowedDomains: []string{"a.com"}}
	custom := ms
	custom.Folder, custom.FolderName = FolderCustom, "Reports"

	for name, c := range map[string]Config{"ms": ms, "cert": cert, "cust": cust, "op": op, "custom": custom} {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: unexpected %v", name, err)
		}
	}

	mut := func(base Config, f func(*Config)) Config { f(&base); return base }
	cases := []struct {
		name string
		c    Config
		msg  string
	}{
		{"provider", mut(ms, func(c *Config) { c.Provider = "x" }), "Provider"},
		{"tenant", mut(ms, func(c *Config) { c.TenantID = "" }), "Tenant ID is required"},
		{"client", mut(ms, func(c *Config) { c.ClientID = "" }), "Client ID is required"},
		{"secret", mut(ms, func(c *Config) { c.Secret = "" }), "secret"},
		{"cert", mut(cert, func(c *Config) { c.Certificate = "" }), "Certificate"},
		{"domains", mut(ms, func(c *Config) { c.AllowedDomains = nil }), "At least one allowed domain is required"},
		{"folder", mut(ms, func(c *Config) { c.Folder = "weird" }), "Folder"},
		{"foldername", mut(ms, func(c *Config) { c.Folder = FolderCustom }), "Folder name is required for a custom folder"},
		{"wrongauth", mut(ms, func(c *Config) { c.AuthMethod = AuthCustomerKey }), "Auth"},
		{"wrongauth2", mut(cust, func(c *Config) { c.AuthMethod = AuthClientSecret }), "Auth"},
		{"custkey", mut(cust, func(c *Config) { c.Secret = "" }), "service account"},
		{"opkey", mut(op, func(c *Config) { c.OperatorKey = "" }), "Set the operator service account in Sending Profiles > Service accounts first"},
	}
	for _, tc := range cases {
		err := tc.c.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%s: want %q, got %v", tc.name, tc.msg, err)
		}
	}
}
