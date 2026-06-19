package models

import (
	"fmt"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestResendFailedSMSInCampaign(c *check.C) {
	campaign := s.createSMSCampaign(c)
	result := campaign.Results[0]

	sl := &SMSLog{}
	err := db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(sl).Error
	c.Assert(err, check.Equals, nil)
	err = sl.Error(fmt.Errorf("test sms error"))
	c.Assert(err, check.Equals, nil)

	count, err := ResendFailedSMSInCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(count, check.Equals, 1)

	result, err = GetResult(result.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(result.Status, check.Equals, StatusSending)

	newLog := &SMSLog{}
	err = db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(newLog).Error
	c.Assert(err, check.Equals, nil)
	c.Assert(newLog.SendAttempt, check.Equals, 0)
	c.Assert(newLog.Processing, check.Equals, false)
}

func (s *ModelsSuite) TestResendFailedSMSInCampaign_Retrying(c *check.C) {
	campaign := s.createSMSCampaign(c)
	result := campaign.Results[0]

	sl := &SMSLog{}
	err := db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(sl).Error
	c.Assert(err, check.Equals, nil)
	err = sl.Backoff(fmt.Errorf("temporary sms error"))
	c.Assert(err, check.Equals, nil)

	result, err = GetResult(result.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(result.Status, check.Equals, StatusRetry)

	count, err := ResendFailedSMSInCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(count, check.Equals, 1)

	// Exactly one SMSLog should exist — old one replaced by new one
	logs := []*SMSLog{}
	err = db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(&logs).Error
	c.Assert(err, check.Equals, nil)
	c.Assert(len(logs), check.Equals, 1)
	c.Assert(logs[0].SendAttempt, check.Equals, 0)

	result, err = GetResult(result.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(result.Status, check.Equals, StatusSending)
}

func (s *ModelsSuite) TestResendFailedSMSInCampaign_NotInProgress(c *check.C) {
	campaign := s.createSMSCampaign(c)
	err := CompleteCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	_, err = ResendFailedSMSInCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(err.Error(), check.Equals, "campaign must be In Progress to resend messages")
}

func (s *ModelsSuite) TestResendFailedSMSInCampaign_NoFailed(c *check.C) {
	campaign := s.createSMSCampaign(c)

	count, err := ResendFailedSMSInCampaign(campaign.Id, campaign.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(count, check.Equals, 0)
}

func (s *ModelsSuite) TestResendFailedSMS(c *check.C) {
	campaign := s.createSMSCampaign(c)
	result := campaign.Results[0]

	sl := &SMSLog{}
	err := db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(sl).Error
	c.Assert(err, check.Equals, nil)
	err = sl.Error(fmt.Errorf("test sms error"))
	c.Assert(err, check.Equals, nil)

	err = ResendFailedSMS(campaign.Id, result.RId, campaign.UserId)
	c.Assert(err, check.Equals, nil)

	result, err = GetResult(result.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(result.Status, check.Equals, StatusSending)

	newLog := &SMSLog{}
	err = db.Where("r_id=? AND campaign_id=?", result.RId, campaign.Id).Find(newLog).Error
	c.Assert(err, check.Equals, nil)
	c.Assert(newLog.SendAttempt, check.Equals, 0)
	c.Assert(newLog.Processing, check.Equals, false)
}

func (s *ModelsSuite) TestResendFailedSMS_NotFailed(c *check.C) {
	campaign := s.createSMSCampaign(c)
	result := campaign.Results[0]

	// Result is in "Sending" state — not a failed state
	err := ResendFailedSMS(campaign.Id, result.RId, campaign.UserId)
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(err.Error(), check.Equals, "result is not in a failed state")
}
