package models

import (
	"encoding/json"

	"github.com/gophish/gophish/crypto"
	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestSMTPGetDialerRefusesCiphertext(c *check.C) {
	smtp := SMTP{Host: "1.1.1.1:25", FromAddress: "foo@example.com", Password: crypto.EncryptedPrefix + "x"}
	_, err := smtp.GetDialer()
	c.Assert(err, check.Equals, ErrEncryptionKeyUnavailable)

	smtp.Password = "plain"
	d, err := smtp.GetDialer()
	c.Assert(err, check.IsNil)
	c.Assert(d, check.NotNil)
}

func (s *ModelsSuite) TestSMSRequestGetDialerRefusesCiphertext(c *check.C) {
	req := SMSRequest{SMS: SMS{Provider: "twilio", ProviderConfig: crypto.EncryptedPrefix + "x"}}
	_, err := req.GetDialer()
	c.Assert(err, check.Equals, ErrEncryptionKeyUnavailable)

	req.SMS.ProviderConfig = `{"account_sid":"a","auth_token":"b"}`
	d, err := req.GetDialer()
	c.Assert(err, check.IsNil)
	c.Assert(d, check.NotNil)
}

func (s *ModelsSuite) TestSMTPSaveWhileLockedKeepsCiphertext(c *check.C) {
	defer crypto.Unload()
	a := s.newTestKey(c)
	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	id := s.postTestSMTP(c, "secret")
	original := rawSMTPPassword(c, id)
	c.Assert(original, check.Matches, `^ENC:v1:.*`)
	crypto.Unload()

	smtp, err := GetSMTP(id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(smtp.Password, check.Equals, original)
	smtp.Name = "renamed"
	c.Assert(PutSMTP(&smtp), check.IsNil)
	c.Assert(rawSMTPPassword(c, id), check.Equals, original)

	c.Assert(crypto.LoadKey(a, crypto.SourceSession), check.IsNil)
	smtp, err = GetSMTP(id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(smtp.Password, check.Equals, "secret")
}

func (s *ModelsSuite) TestEventDetailsLockedFlag(c *check.C) {
	defer crypto.Unload()
	campaign := s.createCampaign(c)
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	c.Assert(AddEvent(&Event{Email: "enc@example.com", Message: "Enc", Details: "{}"}, campaign.Id), check.IsNil)
	crypto.Unload()
	c.Assert(AddEvent(&Event{Email: "plain@example.com", Message: "Plain", Details: "{}"}, campaign.Id), check.IsNil)

	var enc, plain Event
	c.Assert(db.Where("campaign_id = ? AND message = ?", campaign.Id, "Enc").First(&enc).Error, check.IsNil)
	c.Assert(enc.Details, check.Equals, "")
	c.Assert(enc.DetailsLocked, check.Equals, true)

	c.Assert(db.Where("campaign_id = ? AND message = ?", campaign.Id, "Plain").First(&plain).Error, check.IsNil)
	c.Assert(plain.Details, check.Equals, "{}")
	c.Assert(plain.DetailsLocked, check.Equals, false)
	b, err := json.Marshal(plain)
	c.Assert(err, check.IsNil)
	var m map[string]interface{}
	c.Assert(json.Unmarshal(b, &m), check.IsNil)
	_, has := m["details_locked"]
	c.Assert(has, check.Equals, false)
}
