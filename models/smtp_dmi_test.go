package models

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gophish/gophish/crypto"
	"github.com/gophish/gophish/dmi"
	check "gopkg.in/check.v1"
)

func msDMIProfile(name, method, secret string) SMTP {
	return SMTP{
		Name:              name,
		UserId:            1,
		Interface:         InterfaceMS365DMI,
		FromAddress:       "it@example.com",
		DMIAuthMethod:     method,
		DMITenantID:       "tenant",
		DMIClientID:       "client",
		DMISecretInput:    secret,
		DMIAllowedDomains: "example.com, contoso.com",
	}
}

func rawSMTPColumn(c *check.C, id int64, col string) string {
	var values []string
	c.Assert(db.Table("smtp").Where("id = ?", id).Pluck(col, &values).Error, check.IsNil)
	c.Assert(values, check.HasLen, 1)
	return values[0]
}

func parseCertAndKey(c *check.C, certPEM, keyPEM string) (*x509.Certificate, *rsa.PrivateKey) {
	cb, _ := pem.Decode([]byte(certPEM))
	c.Assert(cb, check.NotNil)
	cert, err := x509.ParseCertificate(cb.Bytes)
	c.Assert(err, check.IsNil)
	kb, _ := pem.Decode([]byte(keyPEM))
	c.Assert(kb, check.NotNil)
	k, err := x509.ParsePKCS8PrivateKey(kb.Bytes)
	c.Assert(err, check.IsNil)
	return cert, k.(*rsa.PrivateKey)
}

func testServiceAccountJSON(c *check.C) string {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	c.Assert(err, check.IsNil)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	c.Assert(err, check.IsNil)
	b, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "op@project.iam.gserviceaccount.com",
		"client_id":    "1234567890",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	})
	c.Assert(err, check.IsNil)
	return string(b)
}

func (s *ModelsSuite) TestPostDMIProfileMicrosoftSecret(c *check.C) {
	defer crypto.Unload()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	p := msDMIProfile("ms", dmi.AuthClientSecret, "s3cret")
	c.Assert(PostSMTP(&p), check.IsNil)

	raw := rawSMTPColumn(c, p.Id, "dmi_secret")
	c.Assert(raw, check.Not(check.Equals), "s3cret")
	c.Assert(raw, check.Matches, `^ENC:v1:.*`)

	got, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.DMISecret, check.Equals, "s3cret")
	b, err := json.Marshal(got)
	c.Assert(err, check.IsNil)
	c.Assert(strings.Contains(string(b), "s3cret"), check.Equals, false)
	c.Assert(strings.Contains(string(b), `"dmi_secret_set":true`), check.Equals, true)

	cfg, err := got.DMIConfig()
	c.Assert(err, check.IsNil)
	c.Assert(cfg.AllowedDomains, check.DeepEquals, []string{"example.com", "contoso.com"})
	c.Assert(cfg.Folder, check.Equals, dmi.FolderInbox)
	c.Assert(cfg.Provider, check.Equals, dmi.ProviderMicrosoft)
}

func (s *ModelsSuite) TestPutDMIKeepsSecretWhenEmpty(c *check.C) {
	p := msDMIProfile("ms", dmi.AuthClientSecret, "s3cret")
	c.Assert(PostSMTP(&p), check.IsNil)

	upd := msDMIProfile("ms renamed", dmi.AuthClientSecret, "")
	upd.Id = p.Id
	c.Assert(PutSMTP(&upd), check.IsNil)
	got, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Name, check.Equals, "ms renamed")
	c.Assert(got.DMISecret, check.Equals, "s3cret")

	// The stored secret only counts for its owner.
	other := msDMIProfile("ms renamed", dmi.AuthClientSecret, "")
	other.Id, other.UserId = p.Id, 2
	c.Assert(other.Validate(), check.ErrorMatches, "Client secret is required")

	upd = msDMIProfile("ms renamed", dmi.AuthClientSecret, "n3w")
	upd.Id = p.Id
	c.Assert(PutSMTP(&upd), check.IsNil)
	got, err = GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.DMISecret, check.Equals, "n3w")
}

func (s *ModelsSuite) TestPutDMIMethodChangeClearsSecret(c *check.C) {
	p := msDMIProfile("ms", dmi.AuthClientSecret, "s3cret")
	c.Assert(PostSMTP(&p), check.IsNil)

	upd := msDMIProfile("ms", dmi.AuthCertificate, "")
	upd.Id = p.Id
	c.Assert(PutSMTP(&upd), check.IsNil)
	got, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.DMISecret, check.Not(check.Equals), "s3cret")
	cert, key := parseCertAndKey(c, got.DMICertificate, got.DMISecret)
	c.Assert(cert.PublicKey.(*rsa.PublicKey).Equal(&key.PublicKey), check.Equals, true)

	// Back to client_secret with no input: the certificate key must not survive.
	upd = msDMIProfile("ms", dmi.AuthClientSecret, "")
	upd.Id = p.Id
	c.Assert(upd.Validate(), check.ErrorMatches, "Client secret is required")
}

func (s *ModelsSuite) TestPostDMICertificateGenerated(c *check.C) {
	p := msDMIProfile("ms cert", dmi.AuthCertificate, "ignored")
	c.Assert(PostSMTP(&p), check.IsNil)
	got, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.DMISecret, check.Not(check.Equals), "ignored")
	cert, key := parseCertAndKey(c, got.DMICertificate, got.DMISecret)
	c.Assert(cert.Subject.CommonName, check.Equals, "ms cert")
	c.Assert(cert.PublicKey.(*rsa.PublicKey).Equal(&key.PublicKey), check.Equals, true)
	c.Assert(got.DMISecretSet, check.Equals, true)

	// A PUT keeps the stored key and certificate.
	upd := msDMIProfile("ms cert", dmi.AuthCertificate, "")
	upd.Id = p.Id
	c.Assert(PutSMTP(&upd), check.IsNil)
	again, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(again.DMICertificate, check.Equals, got.DMICertificate)
	c.Assert(again.DMISecret, check.Equals, got.DMISecret)
}

func (s *ModelsSuite) TestDMICertificateKeyLostBlocksSave(c *check.C) {
	p := msDMIProfile("ms cert lost", dmi.AuthCertificate, "")
	c.Assert(PostSMTP(&p), check.IsNil)
	// Discarding encrypted data blanks the key but keeps the certificate.
	c.Assert(db.Table("smtp").Where("id=?", p.Id).Update("dmi_secret", "").Error, check.IsNil)

	upd := msDMIProfile("ms cert lost renamed", dmi.AuthCertificate, "")
	upd.Id, upd.UserId = p.Id, 1
	c.Assert(upd.Validate(), check.Equals, ErrDMICertificateKeyLost)

	_, err := RegenerateDMICertificate(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(upd.Validate(), check.IsNil)
	c.Assert(PutSMTP(&upd), check.IsNil)
}

func (s *ModelsSuite) TestRegenerateDMICertificate(c *check.C) {
	p := msDMIProfile("ms cert", dmi.AuthCertificate, "")
	c.Assert(PostSMTP(&p), check.IsNil)
	before, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)

	after, err := RegenerateDMICertificate(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(after.DMICertificate, check.Not(check.Equals), before.DMICertificate)
	stored, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(stored.DMICertificate, check.Equals, after.DMICertificate)
	cert, key := parseCertAndKey(c, stored.DMICertificate, stored.DMISecret)
	c.Assert(cert.PublicKey.(*rsa.PublicKey).Equal(&key.PublicKey), check.Equals, true)

	sec := msDMIProfile("ms secret", dmi.AuthClientSecret, "x")
	c.Assert(PostSMTP(&sec), check.IsNil)
	_, err = RegenerateDMICertificate(sec.Id, 1)
	c.Assert(err, check.NotNil)
}

func (s *ModelsSuite) TestDMIValidateSkipsHost(c *check.C) {
	p := msDMIProfile("ms", dmi.AuthClientSecret, "x")
	c.Assert(p.Host, check.Equals, "")
	c.Assert(p.Validate(), check.IsNil)

	p.FromAddress = ""
	c.Assert(p.Validate(), check.Equals, ErrFromAddressNotSpecified)
	p.FromAddress = "not-an-email"
	c.Assert(p.Validate(), check.Equals, ErrInvalidFromAddress)
	p.FromAddress = "it@example.com"
	p.DMIAllowedDomains = " , "
	c.Assert(p.Validate(), check.ErrorMatches, "At least one allowed domain is required")
}

func (s *ModelsSuite) TestDMIGetDialerEncryptionLocked(c *check.C) {
	p := SMTP{Interface: InterfaceMS365DMI, FromAddress: "it@example.com", DMIAuthMethod: dmi.AuthClientSecret,
		DMITenantID: "t", DMIClientID: "c", DMISecret: crypto.EncryptedPrefix + "x", DMIAllowedDomains: "example.com"}
	_, err := p.GetDialer()
	c.Assert(err, check.Equals, ErrEncryptionKeyUnavailable)

	p.DMISecret = "plain"
	d, err := p.GetDialer()
	c.Assert(err, check.IsNil)
	c.Assert(d, check.NotNil)

	// The operator key is checked too.
	c.Assert(db.Create(&DMISettings{GoogleKey: crypto.EncryptedPrefix + "x"}).Error, check.IsNil)
	g := SMTP{Interface: InterfaceGWSDMI, FromAddress: "it@example.com", DMIAuthMethod: dmi.AuthOperatorKey, DMIAllowedDomains: "example.com"}
	_, err = g.GetDialer()
	c.Assert(err, check.Equals, ErrEncryptionKeyUnavailable)
}

func (s *ModelsSuite) TestDMIOperatorKeyMissing(c *check.C) {
	p := SMTP{Name: "gws", UserId: 1, Interface: InterfaceGWSDMI, FromAddress: "it@example.com",
		DMIAuthMethod: dmi.AuthOperatorKey, DMIAllowedDomains: "example.com"}
	c.Assert(p.Validate(), check.ErrorMatches, "Set the operator service account in Sending Profiles > Service accounts first")
	c.Assert(PostSMTP(&p), check.ErrorMatches, "Set the operator service account in Sending Profiles > Service accounts first")

	c.Assert(SaveDMIGoogleKey(testServiceAccountJSON(c)), check.IsNil)
	c.Assert(PostSMTP(&p), check.IsNil)
	got, err := GetSMTP(p.Id, 1)
	c.Assert(err, check.IsNil)
	d, err := got.GetDialer()
	c.Assert(err, check.IsNil)
	c.Assert(d, check.NotNil)
}

func (s *ModelsSuite) TestDMIConnectionRejectsNonDMI(c *check.C) {
	id := s.postTestSMTP(c, "pw")
	c.Assert(TestDMIConnection(id, 1, "a@example.com"), check.NotNil)
}

func (s *ModelsSuite) TestLegacySMTPUnchanged(c *check.C) {
	id := s.postTestSMTP(c, "pw")
	got, err := GetSMTP(id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.IsDMI(), check.Equals, false)
	d, err := got.GetDialer()
	c.Assert(err, check.IsNil)
	_, ok := d.(*Dialer)
	c.Assert(ok, check.Equals, true)

	got.Interface = InterfaceSMTP
	c.Assert(got.Validate(), check.IsNil)
	got.Host = ""
	c.Assert(got.Validate(), check.Equals, ErrHostNotSpecified)
}

func (s *ModelsSuite) TestPostDMICustomerKeyValidated(c *check.C) {
	p := SMTP{
		Name: "gws bad key", UserId: 1, Interface: InterfaceGWSDMI, FromAddress: "it@example.com",
		DMIAuthMethod: dmi.AuthCustomerKey, DMISecretInput: `{"client_email":"a@b.c","private_key":"garbage"}`,
		DMIAllowedDomains: "example.com",
	}
	c.Assert(PostSMTP(&p), check.ErrorMatches, "The service account key is invalid")
	p.DMISecretInput = "not json"
	c.Assert(PostSMTP(&p), check.ErrorMatches, ".*client_email.*")
	p.DMISecretInput = testServiceAccountJSON(c)
	c.Assert(PostSMTP(&p), check.IsNil)
	// Editing without a new key keeps the stored one and skips the check.
	p.DMISecretInput = ""
	c.Assert(PutSMTP(&p), check.IsNil)
}

func (s *ModelsSuite) TestMailLogBackoffHonoursRetryAfter(c *check.C) {
	campaign := s.createCampaign(c)
	m := &MailLog{}
	c.Assert(db.Where("r_id=? AND campaign_id=?", campaign.Results[0].RId, campaign.Id).Find(m).Error, check.IsNil)

	before := time.Now().UTC()
	c.Assert(m.Backoff(&dmi.Error{Err: errors.New("throttled"), Temp: true, RetryAfter: 2 * time.Hour}), check.IsNil)
	c.Assert(m.SendDate.Before(before.Add(2*time.Hour)), check.Equals, false)
	c.Assert(m.SendDate.After(time.Now().UTC().Add(2*time.Hour)), check.Equals, false)

	// A short hint never shortens the normal schedule.
	normal := m.SendDate.Add(4 * time.Minute)
	c.Assert(m.Backoff(fmt.Errorf("wrapped: %w", &dmi.Error{Err: errors.New("throttled"), Temp: true, RetryAfter: time.Second})), check.IsNil)
	c.Assert(m.SendDate, check.Equals, normal)
}
