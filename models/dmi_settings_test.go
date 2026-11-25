package models

import (
	"github.com/gophish/gophish/crypto"
	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestSaveDMIGoogleKeyRejectsGarbage(c *check.C) {
	for _, bad := range []string{
		"not json",
		`{"client_email":"a@b.c"}`,
		`{"client_email":"a@b.c","private_key":"garbage"}`,
		`{"private_key":"-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"}`,
	} {
		c.Assert(SaveDMIGoogleKey(bad), check.NotNil)
	}
	set, _, _, err := DMIGoogleKeyInfo()
	c.Assert(err, check.IsNil)
	c.Assert(set, check.Equals, false)
}

func (s *ModelsSuite) TestDMIGoogleKeyInfo(c *check.C) {
	defer crypto.Unload()
	c.Assert(crypto.LoadKey(s.newTestKey(c), crypto.SourceSession), check.IsNil)
	key := testServiceAccountJSON(c)
	c.Assert(SaveDMIGoogleKey(key), check.IsNil)

	st, err := GetDMISettings()
	c.Assert(err, check.IsNil)
	c.Assert(st.GoogleKey, check.Matches, `^ENC:v1:.*`)

	set, clientID, email, err := DMIGoogleKeyInfo()
	c.Assert(err, check.IsNil)
	c.Assert(set, check.Equals, true)
	c.Assert(clientID, check.Equals, "1234567890")
	c.Assert(email, check.Equals, "op@project.iam.gserviceaccount.com")

	// Saving again keeps a single row.
	c.Assert(SaveDMIGoogleKey(key), check.IsNil)
	var n int
	c.Assert(db.Model(&DMISettings{}).Count(&n).Error, check.IsNil)
	c.Assert(n, check.Equals, 1)

	crypto.Unload()
	set, _, _, err = DMIGoogleKeyInfo()
	c.Assert(set, check.Equals, true)
	c.Assert(err, check.Equals, ErrEncryptionKeyUnavailable)

	c.Assert(SaveDMIGoogleKey(""), check.IsNil)
	set, _, _, err = DMIGoogleKeyInfo()
	c.Assert(err, check.IsNil)
	c.Assert(set, check.Equals, false)
}
