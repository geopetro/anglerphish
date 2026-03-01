package models

import (
	"encoding/json"

	"github.com/jinzhu/gorm"

	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestPostSMS(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)
	ss, err := GetSMSs(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ss), check.Equals, 1)
}

func (s *ModelsSuite) TestPostSMSNoFrom(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, ErrFromNotSpecified)
}

func (s *ModelsSuite) TestPostSMSNoProvider(c *check.C) {
	sms := SMS{
		Name: "Test SMS",
		From: "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, ErrProviderNotSpecified)
}

func (s *ModelsSuite) TestPostSMSNoProviderConfig(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		UserId:   1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, ErrProviderConfigNotSpecified)
}

func (s *ModelsSuite) TestPostSMSInvalidProviderConfig(c *check.C) {
	sms := SMS{
		Name:           "Test SMS",
		Provider:       "twilio",
		From:           "+15555555555",
		ProviderConfig: `invalid json`,
		UserId:         1,
	}
	err := PostSMS(&sms)
	_, ok := err.(*json.SyntaxError)
	c.Assert(ok, check.Equals, true)
}

func (s *ModelsSuite) TestPostSMSInvalidTwilioConfig(c *check.C) {
	sms := SMS{
		Name:           "Test SMS",
		Provider:       "twilio",
		From:           "+15555555555",
		ProviderConfig: `{}`,
		UserId:         1,
	}
	err := PostSMS(&sms)
	c.Assert(err.Error(), check.Equals, "Twilio Account SID and Auth Token are required")
}

func (s *ModelsSuite) TestPostSMSInvalidNexmoConfig(c *check.C) {
	sms := SMS{
		Name:           "Test SMS",
		Provider:       "nexmo",
		From:           "+15555555555",
		ProviderConfig: `{}`,
		UserId:         1,
	}
	err := PostSMS(&sms)
	c.Assert(err.Error(), check.Equals, "Nexmo API Key and API Secret are required")
}

func (s *ModelsSuite) TestPostSMSUnsupportedProvider(c *check.C) {
	sms := SMS{
		Name:           "Test SMS",
		Provider:       "unsupported",
		From:           "+15555555555",
		ProviderConfig: `{}`,
		UserId:         1,
	}
	err := PostSMS(&sms)
	c.Assert(err.Error(), check.Equals, "Unsupported SMS provider")
}

func (s *ModelsSuite) TestGetSMS(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	ss, err := GetSMS(sms.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(ss.Name, check.Equals, sms.Name)
	c.Assert(ss.Provider, check.Equals, sms.Provider)
	c.Assert(ss.From, check.Equals, sms.From)
}

func (s *ModelsSuite) TestGetSMSByName(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	ss, err := GetSMSByName(sms.Name, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(ss.Name, check.Equals, sms.Name)
	c.Assert(ss.Provider, check.Equals, sms.Provider)
	c.Assert(ss.From, check.Equals, sms.From)
}

func (s *ModelsSuite) TestGetInvalidSMS(c *check.C) {
	_, err := GetSMS(-1, 1)
	c.Assert(err, check.Not(check.Equals), nil)
}

func (s *ModelsSuite) TestPutSMS(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	sms.Name = "Updated SMS"
	err = PutSMS(&sms)
	c.Assert(err, check.Equals, nil)

	ss, err := GetSMS(sms.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(ss.Name, check.Equals, "Updated SMS")
}

func (s *ModelsSuite) TestDeleteSMS(c *check.C) {
	sms := SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := PostSMS(&sms)
	c.Assert(err, check.Equals, nil)

	err = DeleteSMS(sms.Id, 1)
	c.Assert(err, check.Equals, nil)

	_, err = GetSMS(sms.Id, 1)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}
