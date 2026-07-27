package models

import (
	"gopkg.in/check.v1"
)

// buildTwoCampaignSet creates a campaign set containing two campaigns, where
// test1@example.com appears in BOTH and clicks in both. That overlap is what
// distinguishes Stats (counts twice) from UniqueStats (counts once).
func (s *ModelsSuite) buildTwoCampaignSet(c *check.C) CampaignSet {
	first := s.createCampaignDependencies(c)
	first.Name = "First Campaign"
	second := s.createCampaignDependencies(c)
	second.Name = "Second Campaign"

	cs := CampaignSet{Name: "Test Set", UserId: 1}
	cs.Campaigns = []Campaign{first, second}
	c.Assert(PostCampaignSet(&cs, 1), check.Equals, nil)

	for _, campaign := range cs.Campaigns {
		c.Assert(AddEvent(&Event{Email: "test1@example.com", Message: EventSent}, campaign.Id), check.Equals, nil)
		c.Assert(AddEvent(&Event{Email: "test1@example.com", Message: EventClicked}, campaign.Id), check.Equals, nil)
	}
	// test2 only exists in the first campaign.
	c.Assert(AddEvent(&Event{Email: "test2@example.com", Message: EventSent}, cs.Campaigns[0].Id), check.Equals, nil)

	return cs
}

func (s *ModelsSuite) TestGetCampaignSetSummaryTotalsDoubleCountOverlap(c *check.C) {
	cs := s.buildTwoCampaignSet(c)

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(summary.CampaignCount, check.Equals, 2)

	// test1 sent+clicked in both campaigns, test2 sent in one.
	c.Assert(summary.Stats.EmailsSent, check.Equals, int64(3))
	c.Assert(summary.Stats.ClickedLink, check.Equals, int64(2))
	// Each campaign has 4 targets (test1..test4), so Stats.Total sums to 8
	// rather than deduping like UniqueStats.Total does.
	c.Assert(summary.Stats.Total, check.Equals, int64(8))
}

func (s *ModelsSuite) TestGetCampaignSetSummaryUniqueDedupsOverlap(c *check.C) {
	cs := s.buildTwoCampaignSet(c)

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)

	// test1 counts once despite appearing in both campaigns.
	c.Assert(summary.UniqueStats.EmailsSent, check.Equals, int64(2))
	c.Assert(summary.UniqueStats.ClickedLink, check.Equals, int64(1))
	// Opened is backfilled from clicked.
	c.Assert(summary.UniqueStats.OpenedEmail, check.Equals, int64(1))
	// Four distinct contacts across both campaigns: test1..test4. Blank-email
	// system events (e.g. "Campaign Created") must not inflate this count.
	c.Assert(summary.UniqueStats.Total, check.Equals, int64(4))
}

// TestGetCampaignSetSummaryIgnoresBlankEmailSystemEvents pins the bug where a
// blank-email system event (e.g. "Campaign Created", which every campaign
// emits automatically) was merged into the unique-recipient keyspace as a
// phantom "" recipient, inflating UniqueStats.Total by one. This test adds an
// explicit extra blank-email event on top of the four real recipients and
// asserts it does not change UniqueStats.Total.
func (s *ModelsSuite) TestGetCampaignSetSummaryIgnoresBlankEmailSystemEvents(c *check.C) {
	cs := s.buildTwoCampaignSet(c)

	// Simulate additional system events that carry a blank email, as seen in
	// production ("Campaign Created", "Failed Emails Re-queued", "Failed SMS
	// Re-queued").
	c.Assert(AddEvent(&Event{Message: "Campaign Created"}, cs.Campaigns[0].Id), check.Equals, nil)
	c.Assert(AddEvent(&Event{Message: "Failed Emails Re-queued"}, cs.Campaigns[1].Id), check.Equals, nil)

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)

	// Still only four distinct contacts: test1..test4. A blank-email system
	// event must not be counted as a unique contact.
	c.Assert(summary.UniqueStats.Total, check.Equals, int64(4))
}

func (s *ModelsSuite) TestGetCampaignSetSummaryTypeIsPopulated(c *check.C) {
	cs := s.buildTwoCampaignSet(c)

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(summary.Campaigns), check.Equals, 2)
	// The frontend branches on this to pick email vs SMS stat badges.
	c.Assert(summary.Campaigns[0].Type, check.Equals, "email")
}

func (s *ModelsSuite) TestGetCampaignSetSummaryRejectsOtherUser(c *check.C) {
	cs := s.buildTwoCampaignSet(c)

	// User 2 must not be able to read user 1's set.
	_, err := GetCampaignSetSummary(cs.Id, 2)
	c.Assert(err, check.NotNil)
}

func (s *ModelsSuite) TestGetCampaignSetSummaryMissingSet(c *check.C) {
	_, err := GetCampaignSetSummary(999999, 1)
	c.Assert(err, check.NotNil)
}
