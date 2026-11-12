package models

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gophish/gophish/auth"
	"github.com/gophish/gophish/crypto"
	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) newTestKey(c *check.C) string {
	k, err := crypto.GenerateEncryptionKey()
	c.Assert(err, check.IsNil)
	return k
}

func (s *ModelsSuite) TestCheckValueRoundTrip(c *check.C) {
	defer crypto.Unload()
	a, b := s.newTestKey(c), s.newTestKey(c)
	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	c.Assert(writeCheckValue(), check.IsNil)
	ok, err := verifyKey(a)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)
	ok, err = verifyKey(b)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, false)
	c.Assert(deleteCheckValue(), check.IsNil)
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, false)
}

func (s *ModelsSuite) TestVerifyKeyWithoutCheckValueUsesData(c *check.C) {
	defer crypto.Unload()
	a, b := s.newTestKey(c), s.newTestKey(c)
	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	c.Assert(PostSMTP(&SMTP{Name: "e", Host: "1.1.1.1:25", FromAddress: "foo@example.com", Password: "p", UserId: 1}), check.IsNil)
	crypto.Unload()
	ok, err := verifyKey(a)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)
	ok, err = verifyKey(b)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, false)
}

func (s *ModelsSuite) TestVerifyKeyEmptyDatabase(c *check.C) {
	ok, err := verifyKey(s.newTestKey(c))
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)
}

func (s *ModelsSuite) TestEncryptionReportCounts(c *check.C) {
	defer crypto.Unload()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	c.Assert(PostSMTP(&SMTP{Name: "e", Host: "1.1.1.1:25", FromAddress: "foo@example.com", Password: "p", UserId: 1}), check.IsNil)
	crypto.Unload()
	c.Assert(PostSMTP(&SMTP{Name: "p", Host: "1.1.1.1:25", FromAddress: "foo@example.com", Password: "q", UserId: 1}), check.IsNil)
	r, err := GetEncryptionReport()
	c.Assert(err, check.IsNil)
	c.Assert(r.SMTPPasswords.Encrypted, check.Equals, 1)
	c.Assert(r.SMTPPasswords.Plaintext, check.Equals, 1)
	n, err := countEncryptedValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 1)
	n, err = countPlaintextValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 1)
}

func (s *ModelsSuite) postTestSMTP(c *check.C, password string) int64 {
	smtp := SMTP{Name: "smtp-" + password, Host: "1.1.1.1:25", FromAddress: "foo@example.com", Password: password, UserId: 1}
	c.Assert(PostSMTP(&smtp), check.IsNil)
	return smtp.Id
}

func rawSMTPPassword(c *check.C, id int64) string {
	var values []string
	c.Assert(db.Table("smtp").Where("id = ?", id).Pluck("password", &values).Error, check.IsNil)
	c.Assert(values, check.HasLen, 1)
	return values[0]
}

func encryptionStatus(c *check.C) EncryptionStatus {
	st, err := GetEncryptionStatusInfo()
	c.Assert(err, check.IsNil)
	return st
}

// lockedWithDataUnder stores an SMTP password and a check value under key,
// then starts up with no env key so the state is Locked (missing_key).
func (s *ModelsSuite) lockedWithDataUnder(c *check.C, key, password string) int64 {
	c.Assert(crypto.LoadKey(key, crypto.SourceSession), check.IsNil)
	id := s.postTestSMTP(c, password)
	c.Assert(writeCheckValue(), check.IsNil)
	crypto.Unload()
	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	c.Assert(IsEncryptionLocked(), check.Equals, true)
	return id
}

func (s *ModelsSuite) TestStartupNoEnvEmptyDB(c *check.C) {
	defer resetEncryptionStateForTest()
	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOff)
	c.Assert(st.Source, check.Equals, crypto.SourceNone)
	c.Assert(IsEncryptionLocked(), check.Equals, false)
}

func (s *ModelsSuite) TestStartupEnvKeyFresh(c *check.C) {
	defer resetEncryptionStateForTest()
	os.Setenv(crypto.EncryptionKeyEnvVar, s.newTestKey(c))
	defer os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOn)
	c.Assert(st.Source, check.Equals, crypto.SourceEnv)
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, true)
	c.Assert(IsSessionKeyActive(), check.Equals, false)
}

func (s *ModelsSuite) TestStartupMissingKeyWithEncryptedData(c *check.C) {
	defer resetEncryptionStateForTest()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	s.postTestSMTP(c, "secret")
	crypto.Unload()
	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "missing_key")
	c.Assert(IsEncryptionLocked(), check.Equals, true)
}

func (s *ModelsSuite) TestStartupWrongEnvKey(c *check.C) {
	defer resetEncryptionStateForTest()
	a, b := s.newTestKey(c), s.newTestKey(c)
	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	s.postTestSMTP(c, "secret")
	c.Assert(writeCheckValue(), check.IsNil)
	crypto.Unload()
	os.Setenv(crypto.EncryptionKeyEnvVar, b)
	defer os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "wrong_key")
	c.Assert(crypto.HasKey(), check.Equals, false)
}

func (s *ModelsSuite) TestStartupMalformedEnvKey(c *check.C) {
	defer resetEncryptionStateForTest()
	os.Setenv(crypto.EncryptionKeyEnvVar, "bad")
	defer os.Unsetenv(crypto.EncryptionKeyEnvVar)

	// Empty DB: Off with a warning.
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOff)
	c.Assert(st.StartupWarning, check.Not(check.Equals), "")
	c.Assert(crypto.HasKey(), check.Equals, false)

	// With encrypted data: Locked (invalid_key).
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	s.postTestSMTP(c, "secret")
	crypto.Unload()
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st = encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "invalid_key")
	c.Assert(crypto.HasKey(), check.Equals, false)
}

func (s *ModelsSuite) TestStartupLegacyNoCheckValue(c *check.C) {
	defer resetEncryptionStateForTest()
	defer os.Unsetenv(crypto.EncryptionKeyEnvVar)
	a, b := s.newTestKey(c), s.newTestKey(c)
	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	s.postTestSMTP(c, "secret")
	crypto.Unload()

	// Wrong key without a check value: verified against the data, Locked.
	os.Setenv(crypto.EncryptionKeyEnvVar, b)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "wrong_key")
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, false)

	// Right key: On, and a check value is written.
	os.Setenv(crypto.EncryptionKeyEnvVar, a)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st = encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOn)
	has, err = hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, true)

	// The written check value now rejects the wrong key.
	os.Setenv(crypto.EncryptionKeyEnvVar, b)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	st = encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "wrong_key")
}

func (s *ModelsSuite) TestEnableFromOff(c *check.C) {
	defer resetEncryptionStateForTest()
	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	plainID := s.postTestSMTP(c, "before")

	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.IsNil)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOn)
	c.Assert(st.Source, check.Equals, crypto.SourceSession)
	c.Assert(st.Report.SMTPPasswords.Plaintext, check.Equals, 1)
	c.Assert(st.PlaintextCount >= 1, check.Equals, true)
	c.Assert(IsSessionKeyActive(), check.Equals, true)
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, true)

	c.Assert(rawSMTPPassword(c, plainID), check.Equals, "before")
	newID := s.postTestSMTP(c, "after")
	c.Assert(crypto.IsEncrypted(rawSMTPPassword(c, newID)), check.Equals, true)

	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.Equals, ErrEncryptionAlreadyOn)
}

func (s *ModelsSuite) TestEnableRejectsMalformedKey(c *check.C) {
	defer resetEncryptionStateForTest()
	c.Assert(EnableEncryptionWithKey("bad"), check.Equals, crypto.ErrInvalidKeyLength)
	c.Assert(crypto.HasKey(), check.Equals, false)
}

func (s *ModelsSuite) TestEnableRefusedWhenEncryptedDataExists(c *check.C) {
	defer resetEncryptionStateForTest()
	a, b := s.newTestKey(c), s.newTestKey(c)
	id := s.lockedWithDataUnder(c, a, "secret")
	before := rawSMTPPassword(c, id)

	c.Assert(EnableEncryptionWithKey(b), check.Equals, ErrEncryptedDataExists)
	c.Assert(crypto.HasKey(), check.Equals, false)
	c.Assert(IsEncryptionLocked(), check.Equals, true)
	c.Assert(rawSMTPPassword(c, id), check.Equals, before)
}

func (s *ModelsSuite) TestUnlock(c *check.C) {
	defer resetEncryptionStateForTest()
	a, b := s.newTestKey(c), s.newTestKey(c)

	c.Assert(UnlockEncryption(a), check.Equals, ErrNotLocked)

	id := s.lockedWithDataUnder(c, a, "secret")
	c.Assert(UnlockEncryption(b), check.Equals, ErrWrongKey)
	c.Assert(IsEncryptionLocked(), check.Equals, true)
	c.Assert(crypto.HasKey(), check.Equals, false)

	c.Assert(UnlockEncryption(a), check.IsNil)
	c.Assert(IsEncryptionLocked(), check.Equals, false)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOn)
	c.Assert(st.Source, check.Equals, crypto.SourceSession)
	smtp, err := GetSMTP(id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(smtp.Password, check.Equals, "secret")
}

func (s *ModelsSuite) TestMigrationSingleFlight(c *check.C) {
	defer resetEncryptionStateForTest()
	_, err := RunEncryptionMigration(true, false)
	c.Assert(err, check.Equals, ErrNoKeyLoaded)

	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	migrationMu.Lock()
	_, err = RunEncryptionMigration(true, false)
	c.Assert(err, check.Equals, ErrMigrationRunning)
	_, err = DisableEncryption()
	c.Assert(err, check.Equals, ErrMigrationRunning)
	migrationMu.Unlock()
}

func (s *ModelsSuite) TestMigrateDryRunChangesNothing(c *check.C) {
	defer resetEncryptionStateForTest()
	id := s.postTestSMTP(c, "plain")
	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.IsNil)
	before := encryptionStatus(c)
	c.Assert(before.Report.SMTPPasswords.Plaintext, check.Equals, 1)

	results, err := RunEncryptionMigration(true, true)
	c.Assert(err, check.IsNil)
	c.Assert(len(results) > 0, check.Equals, true)
	st := encryptionStatus(c)
	c.Assert(st.Report.SMTPPasswords.Plaintext, check.Equals, 1)
	c.Assert(st.PlaintextCount, check.Equals, before.PlaintextCount)
	c.Assert(rawSMTPPassword(c, id), check.Equals, "plain")

	_, err = RunEncryptionMigration(true, false)
	c.Assert(err, check.IsNil)
	c.Assert(crypto.IsEncrypted(rawSMTPPassword(c, id)), check.Equals, true)
}

func (s *ModelsSuite) TestDisableWithConcurrentWrites(c *check.C) {
	defer resetEncryptionStateForTest()
	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.IsNil)
	id := s.postTestSMTP(c, "secret")
	for i := 0; i < 5; i++ {
		c.Assert(db.Save(&Event{CampaignId: 1, Email: "a@example.com", Details: fmt.Sprintf("before-%d", i)}).Error, check.IsNil)
	}
	c.Assert(crypto.IsEncrypted(rawSMTPPassword(c, id)), check.Equals, true)

	var wg sync.WaitGroup
	started := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			db.Save(&Event{CampaignId: 1, Email: "b@example.com", Details: fmt.Sprintf("during-%d", i)})
			if i == 0 {
				close(started)
			}
		}
	}()
	<-started
	_, err := DisableEncryption()
	wg.Wait()
	c.Assert(err, check.IsNil)

	n, err := countEncryptedValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 0)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOff)
	c.Assert(crypto.HasKey(), check.Equals, false)
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, false)

	var events []Event
	c.Assert(db.Find(&events).Error, check.IsNil)
	c.Assert(events, check.HasLen, 25)
	for _, e := range events {
		c.Assert(e.Details, check.Not(check.Equals), "")
	}
	c.Assert(rawSMTPPassword(c, id), check.Equals, "secret")
}

func (s *ModelsSuite) TestDisableRequiresKey(c *check.C) {
	defer resetEncryptionStateForTest()
	_, err := DisableEncryption()
	c.Assert(err, check.Equals, ErrNoKeyLoaded)
}

func (s *ModelsSuite) TestDiscard(c *check.C) {
	defer resetEncryptionStateForTest()
	a := s.newTestKey(c)

	_, err := DiscardEncryptedData(DiscardConfirmationPhrase)
	c.Assert(err, check.Equals, ErrNotLocked)

	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	c.Assert(db.Save(&Event{CampaignId: 1, Email: "a@example.com", Details: "captured"}).Error, check.IsNil)
	crypto.Unload()
	plainID := s.postTestSMTP(c, "plain")
	encID := s.lockedWithDataUnder(c, a, "secret")
	before := rawSMTPPassword(c, encID)

	_, err = DiscardEncryptedData("discard encrypted data")
	c.Assert(err, check.Equals, ErrBadConfirmation)
	c.Assert(IsEncryptionLocked(), check.Equals, true)
	c.Assert(rawSMTPPassword(c, encID), check.Equals, before)

	report, err := DiscardEncryptedData(DiscardConfirmationPhrase)
	c.Assert(err, check.IsNil)
	c.Assert(report.SMTPPasswords.Encrypted, check.Equals, 1)
	c.Assert(report.SMTPPasswords.Plaintext, check.Equals, 1)
	c.Assert(report.EventDetails.Encrypted, check.Equals, 1)

	n, err := countEncryptedValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 0)
	c.Assert(rawSMTPPassword(c, encID), check.Equals, "")
	c.Assert(rawSMTPPassword(c, plainID), check.Equals, "plain")
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, false)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateOff)
	c.Assert(IsEncryptionLocked(), check.Equals, false)
}

func (s *ModelsSuite) TestSetEncryptionLockedForTest(c *check.C) {
	defer resetEncryptionStateForTest()
	SetEncryptionLockedForTest(true)
	st := encryptionStatus(c)
	c.Assert(st.State, check.Equals, EncryptionStateLocked)
	c.Assert(st.LockedReason, check.Equals, "missing_key")
	SetEncryptionLockedForTest(false)
	c.Assert(IsEncryptionLocked(), check.Equals, false)
}

func (s *ModelsSuite) TestDisableWithUndecryptableValue(c *check.C) {
	defer resetEncryptionStateForTest()
	key := s.newTestKey(c)
	c.Assert(EnableEncryptionWithKey(key), check.IsNil)
	id := s.postTestSMTP(c, "secret")
	c.Assert(crypto.IsEncrypted(rawSMTPPassword(c, id)), check.Equals, true)

	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	other, err := crypto.Encrypt("captured")
	c.Assert(err, check.IsNil)
	c.Assert(crypto.LoadKey(key, crypto.SourceSession), check.IsNil)
	bad := &Event{CampaignId: 1, Email: "a@example.com", Details: other}
	c.Assert(db.Save(bad).Error, check.IsNil)

	results, err := DisableEncryption()
	c.Assert(err, check.Equals, ErrDecryptIncomplete)
	c.Assert(len(results) > 0, check.Equals, true)
	c.Assert(encryptionStatus(c).State, check.Equals, EncryptionStateDisabling)
	c.Assert(rawSMTPPassword(c, id), check.Equals, "secret")
	var raw []string
	c.Assert(db.Table("events").Where("id = ?", bad.Id).Pluck("details", &raw).Error, check.IsNil)
	c.Assert(raw, check.DeepEquals, []string{other})
}

func (s *ModelsSuite) TestStartupDeletesStaleCheckValue(c *check.C) {
	defer resetEncryptionStateForTest()
	c.Assert(EnableEncryptionWithKey(s.newTestKey(c)), check.IsNil)
	has, err := hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, true)

	crypto.Unload()
	os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	has, err = hasCheckValue()
	c.Assert(err, check.IsNil)
	c.Assert(has, check.Equals, false)

	os.Setenv(crypto.EncryptionKeyEnvVar, s.newTestKey(c))
	defer os.Unsetenv(crypto.EncryptionKeyEnvVar)
	c.Assert(InitEncryptionAtStartup(), check.IsNil)
	c.Assert(encryptionStatus(c).State, check.Equals, EncryptionStateOn)
}

func (s *ModelsSuite) TestEncryptionReportIncludesOIDCSecret(c *check.C) {
	defer crypto.Unload()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	c.Assert(SaveOIDCSettings(validOIDCInput()), check.IsNil)
	report, err := GetEncryptionReport()
	c.Assert(err, check.IsNil)
	c.Assert(report.OIDCSecrets.Encrypted, check.Equals, 1)
	n, err := countEncryptedValues()
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 1)
}

func (s *ModelsSuite) TestMigrateOIDCSecretRoundTrip(c *check.C) {
	defer crypto.Unload()
	c.Assert(SaveOIDCSettings(validOIDCInput()), check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, "s")
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	_, err := MigrateToEncrypted()
	c.Assert(err, check.IsNil)
	c.Assert(strings.HasPrefix(rawOIDCSecret(c), crypto.EncryptedPrefix), check.Equals, true)
	_, err = MigrateToDecrypted()
	c.Assert(err, check.IsNil)
	c.Assert(rawOIDCSecret(c), check.Equals, "s")
}

// lockedOIDCSetup stores an encrypted, enabled SSO secret, then puts
// encryption in the Locked state. It returns the key.
func (s *ModelsSuite) lockedOIDCSetup(c *check.C) string {
	key := s.newTestKey(c)
	c.Assert(crypto.LoadKey(key, crypto.SourceSession), check.IsNil)
	c.Assert(writeCheckValue(), check.IsNil)
	c.Assert(SaveOIDCSettings(validOIDCInput()), check.IsNil)
	crypto.Unload()
	SetEncryptionLockedForTest(true)
	return key
}

func (s *ModelsSuite) TestOIDCUnavailableWhileLockedThenUnlock(c *check.C) {
	defer resetEncryptionStateForTest()
	key := s.lockedOIDCSetup(c)
	c.Assert(ReloadOIDC(), check.IsNil)
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusUnavailable)

	c.Assert(UnlockEncryption(key), check.IsNil)
	c.Assert(auth.ActiveOIDCClient() != nil, check.Equals, true)
	c.Assert(CurrentOIDCStatus(), check.Equals, OIDCStatusEnabled)
}

func (s *ModelsSuite) TestDiscardDisablesOIDC(c *check.C) {
	defer resetEncryptionStateForTest()
	s.lockedOIDCSetup(c)
	c.Assert(ReloadOIDC(), check.IsNil)

	report, err := DiscardEncryptedData(DiscardConfirmationPhrase)
	c.Assert(err, check.IsNil)
	c.Assert(report.OIDCSecrets.Encrypted, check.Equals, 1)
	got, err := GetOIDCSettings()
	c.Assert(err, check.IsNil)
	c.Assert(got.Enabled, check.Equals, false)
	c.Assert(got.ClientSecret, check.Equals, "")
	c.Assert(auth.ActiveOIDCClient() == nil, check.Equals, true)
}

func (s *ModelsSuite) TestVerifyKeyUsesOIDCSecret(c *check.C) {
	defer crypto.Unload()
	right, wrong := s.newTestKey(c), s.newTestKey(c)
	c.Assert(crypto.LoadKey(right, crypto.SourceSession), check.IsNil)
	c.Assert(SaveOIDCSettings(validOIDCInput()), check.IsNil)
	crypto.Unload()
	ok, err := verifyKey(right)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)
	ok, err = verifyKey(wrong)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, false)
}
