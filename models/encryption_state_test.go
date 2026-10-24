package models

import (
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
