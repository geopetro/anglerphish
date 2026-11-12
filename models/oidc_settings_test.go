package models

import (
	"errors"
	"strings"

	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/crypto"
	check "gopkg.in/check.v1"
)

func validOIDCInput() OIDCSettingsInput {
	return OIDCSettingsInput{
		Enabled:       true,
		Provider:      OIDCProviderGeneric,
		Issuer:        "https://idp.example.com",
		ClientID:      "c",
		ClientSecret:  "s",
		RedirectURL:   "https://a/auth/oidc/callback",
		RequiredGroup: "g",
	}
}

func rawOIDCSecret(c *check.C) string {
	var secrets []string
	c.Assert(db.Table("oidc_settings").Order("id asc").Limit(1).Pluck("client_secret", &secrets).Error, check.IsNil)
	c.Assert(secrets, check.HasLen, 1)
	return secrets[0]
}

func (s *ModelsSuite) TestGetOIDCSettingsDefaults(c *check.C) {
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.Provider, check.Equals, "generic")
	c.Assert(got.GroupsClaim, check.Equals, "")
	c.Assert(got.UsernameFromEmail, check.Equals, "local_part")
	c.Assert(got.Enabled, check.Equals, false)
}

func (s *ModelsSuite) TestSaveOIDCKeepsEmptyGroupsClaim(c *check.C) {
	in := validOIDCInput()
	in.GroupsClaim = ""
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.GroupsClaim, check.Equals, "")
	c.Assert(auth.ActiveOIDCClient(), check.NotNil)
}

func (s *ModelsSuite) TestSaveOIDCRoundTrip(c *check.C) {
	in := validOIDCInput()
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.Enabled, check.Equals, true)
	c.Assert(got.Provider, check.Equals, in.Provider)
	c.Assert(got.Issuer, check.Equals, in.Issuer)
	c.Assert(got.ClientID, check.Equals, in.ClientID)
	c.Assert(got.RedirectURL, check.Equals, in.RedirectURL)
	c.Assert(got.RequiredGroup, check.Equals, in.RequiredGroup)
	c.Assert(OIDCSecretSet(got), check.Equals, true)
	c.Assert(auth.ActiveOIDCClient() != nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusEnabled)
}

func (s *ModelsSuite) TestSaveOIDCTrimsFields(c *check.C) {
	in := validOIDCInput()
	in.Issuer = "  https://idp.example.com \t"
	in.RequiredGroup = " g "
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.Issuer, check.Equals, "https://idp.example.com")
	c.Assert(got.RequiredGroup, check.Equals, "g")
}

func (s *ModelsSuite) TestSaveOIDCKeepsSecretWhenEmpty(c *check.C) {
	in := validOIDCInput()
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	before := rawOIDCSecret(c)
	in.ClientSecret = ""
	in.RequiredGroup = "g2"
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, before)
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.RequiredGroup, check.Equals, "g2")
}

func (s *ModelsSuite) TestSaveOIDCDisabledAllowsIncomplete(c *check.C) {
	c.Assert(SaveOIDCSettings(OIDCSettingsInput{Enabled: false, Issuer: "x"}), check.IsNil)
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusDisabled)
}

func (s *ModelsSuite) TestSaveOIDCEnabledRequiresFields(c *check.C) {
	in := validOIDCInput()
	in.Issuer = ""
	err := SaveOIDCSettings(in)
	c.Assert(errors.Is(err, ErrOIDCInvalid), check.Equals, true)

	in = validOIDCInput()
	in.ClientSecret = ""
	err = SaveOIDCSettings(in)
	c.Assert(errors.Is(err, ErrOIDCInvalid), check.Equals, true)
}

func (s *ModelsSuite) TestSaveOIDCRejectsUnknownProvider(c *check.C) {
	in := validOIDCInput()
	in.Provider = "saml"
	err := SaveOIDCSettings(in)
	c.Assert(errors.Is(err, ErrOIDCInvalid), check.Equals, true)
}

func (s *ModelsSuite) TestSaveOIDCRejectsUnknownUsernameMode(c *check.C) {
	in := validOIDCInput()
	in.UsernameFromEmail = "nickname"
	err := SaveOIDCSettings(in)
	c.Assert(errors.Is(err, ErrOIDCInvalid), check.Equals, true)
}

func (s *ModelsSuite) TestSaveOIDCSecretEncryptedWhenKeyLoaded(c *check.C) {
	defer crypto.Unload()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	in := validOIDCInput()
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	raw := rawOIDCSecret(c)
	c.Assert(crypto.IsEncrypted(raw), check.Equals, true)

	crypto.Unload()
	in.ClientSecret = ""
	in.RequiredGroup = "g2"
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, raw)
	// The secret is locked, so SSO must not come up with the ciphertext.
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusUnavailable)
}

func (s *ModelsSuite) TestReloadOIDCInvalidSettingsLeavesOff(c *check.C) {
	c.Assert(db.Exec("INSERT INTO oidc_settings (enabled, issuer, updated_at) VALUES (1, '', CURRENT_TIMESTAMP)").Error, check.IsNil)
	c.Assert(ReloadOIDC(), check.NotNil)
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusUnavailable)
}

func (s *ModelsSuite) TestDisableOIDCClearsClient(c *check.C) {
	in := validOIDCInput()
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(auth.ActiveOIDCClient() != nil, check.Equals, true)
	in.Enabled = false
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusDisabled)
}

func (s *ModelsSuite) TestSaveOIDCTrimsSecret(c *check.C) {
	in := validOIDCInput()
	in.ClientSecret = "  s\n"
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, "s")
}

func (s *ModelsSuite) TestSaveOIDCKeepSecretDoesNotRewriteColumn(c *check.C) {
	in := validOIDCInput()
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(db.Table("oidc_settings").Update("client_secret", "other").Error, check.IsNil)
	in.ClientSecret = ""
	in.RequiredGroup = "g2"
	c.Assert(SaveOIDCSettings(in), check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, "other")
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.RequiredGroup, check.Equals, "g2")
}

func (s *ModelsSuite) TestSaveOIDCRejectsLongFields(c *check.C) {
	in := validOIDCInput()
	in.Issuer = "https://" + strings.Repeat("a", 248)
	c.Assert(len(in.Issuer), check.Equals, 256)
	err := SaveOIDCSettings(in)
	c.Assert(errors.Is(err, ErrOIDCInvalid), check.Equals, true)
	c.Assert(err.Error(), check.Matches, ".*issuer must be 255 characters or fewer")
}
