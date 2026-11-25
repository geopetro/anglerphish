package models

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/gophish/gophish/crypto"
	"github.com/gophish/gophish/dmi"
	check "gopkg.in/check.v1"
)

func rawDMIGoogleKey(c *check.C) string {
	st, err := GetDMISettings()
	c.Assert(err, check.IsNil)
	return st.GoogleKey
}

func (s *ModelsSuite) saveDMIData(c *check.C) (int64, string) {
	p := msDMIProfile("ms", dmi.AuthClientSecret, "s3cret")
	c.Assert(PostSMTP(&p), check.IsNil)
	key := testServiceAccountJSON(c)
	c.Assert(SaveDMIGoogleKey(key), check.IsNil)
	return p.Id, key
}

// plaintextDMIData stores a DMI secret and an operator key with encryption off.
func (s *ModelsSuite) plaintextDMIData(c *check.C) (int64, string) {
	id, key := s.saveDMIData(c)
	c.Assert(rawSMTPColumn(c, id, "dmi_secret"), check.Equals, "s3cret")
	c.Assert(rawDMIGoogleKey(c), check.Equals, key)
	return id, key
}

func (s *ModelsSuite) TestDMIEncryptionReportCounts(c *check.C) {
	s.plaintextDMIData(c)
	r, err := GetEncryptionReport()
	c.Assert(err, check.IsNil)
	c.Assert(r.DMISecrets, check.Equals, crypto.FieldStatus{Total: 1, Plaintext: 1})
	c.Assert(r.DMIGoogleKeys, check.Equals, crypto.FieldStatus{Total: 1, Plaintext: 1})
	n, err := countPlaintextValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 2)
	c.Assert(encryptionStatus(c).PlaintextCount, check.Equals, 2)
	c.Assert(strings.Contains(r.String(), "Direct Injection"), check.Equals, true)
}

func (s *ModelsSuite) TestDMIEnableAndDisableMigrate(c *check.C) {
	defer resetEncryptionStateForTest()
	id, key := s.plaintextDMIData(c)
	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.IsNil)

	preview, err := RunEncryptionMigration(true, true)
	c.Assert(err, check.IsNil)
	c.Assert(strings.Join(preview, "\n"), check.Matches, `(?s).*Direct Injection secrets: 1 would be encrypted.*Direct Injection operator key: 1 would be encrypted.*`)

	_, err = RunEncryptionMigration(true, false)
	c.Assert(err, check.IsNil)
	c.Assert(rawSMTPColumn(c, id, "dmi_secret"), check.Matches, `^ENC:v1:.*`)
	c.Assert(rawDMIGoogleKey(c), check.Matches, `^ENC:v1:.*`)
	n, err := countEncryptedValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 2)

	preview, err = RunEncryptionMigration(false, true)
	c.Assert(err, check.IsNil)
	c.Assert(strings.Join(preview, "\n"), check.Matches, `(?s).*Direct Injection secrets: 1 would be decrypted.*Direct Injection operator key: 1 would be decrypted.*`)

	_, err = DisableEncryption()
	c.Assert(err, check.IsNil)
	c.Assert(rawSMTPColumn(c, id, "dmi_secret"), check.Equals, "s3cret")
	c.Assert(rawDMIGoogleKey(c), check.Equals, key)
}

func (s *ModelsSuite) TestDMIStartupLocksAndDiscard(c *check.C) {
	defer resetEncryptionStateForTest()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	id, _ := s.saveDMIData(c)
	c.Assert(rawSMTPColumn(c, id, "dmi_secret"), check.Matches, `^ENC:v1:.*`)
	c.Assert(rawDMIGoogleKey(c), check.Matches, `^ENC:v1:.*`)
	crypto.Unload()

	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	c.Assert(IsEncryptionLocked(), check.Equals, true)

	report, err := DiscardEncryptedData(DiscardConfirmationPhrase)
	c.Assert(err, check.IsNil)
	c.Assert(report.DMISecrets.Encrypted, check.Equals, 1)
	c.Assert(report.DMIGoogleKeys.Encrypted, check.Equals, 1)
	c.Assert(rawSMTPColumn(c, id, "dmi_secret"), check.Equals, "")
	c.Assert(rawDMIGoogleKey(c), check.Equals, "")

	got, err := GetSMTP(id, 1)
	c.Assert(err, check.IsNil)
	b, err := json.Marshal(got)
	c.Assert(err, check.IsNil)
	c.Assert(strings.Contains(string(b), `"dmi_secret_set":false`), check.Equals, true)
	set, _, _, err := DMIGoogleKeyInfo()
	c.Assert(err, check.IsNil)
	c.Assert(set, check.Equals, false)
}
