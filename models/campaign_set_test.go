package models

import (
	"math"
	"sort"
	"time"

	"gopkg.in/check.v1"
)

// campaignSendMinutes returns the scheduled send time of every result in a
// campaign, expressed as whole minutes after launch.
func campaignSendMinutes(c *check.C, cid int64, launch time.Time) []int {
	var results []Result
	c.Assert(db.Where("campaign_id = ?", cid).Find(&results).Error, check.Equals, nil)
	mins := make([]int, 0, len(results))
	for _, r := range results {
		mins = append(mins, int(math.Round(r.SendDate.Sub(launch).Minutes())))
	}
	return mins
}

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

// TestCampaignSetSharedScheduleStaggersAcrossSet verifies that when a set uses
// a shared send window, recipients across ALL campaigns are staggered as a
// single stream. Previously each campaign spread its own recipients over the
// whole window independently, so the campaigns synchronized and fired in
// bursts (multiple emails per minute) instead of an even drip.
//
// Two 4-recipient campaigns sharing an 8-minute window must produce 8 sends on
// 8 distinct minute offsets. The buggy per-campaign behavior collapses them
// onto 4 minutes (two sends each).
func (s *ModelsSuite) TestCampaignSetSharedScheduleStaggersAcrossSet(c *check.C) {
	first := s.createCampaignDependencies(c)
	first.Name = "First Campaign"
	second := s.createCampaignDependencies(c)
	second.Name = "Second Campaign"

	launch := time.Now().UTC().Add(time.Hour)
	sendBy := launch.Add(8 * time.Minute)

	cs := CampaignSet{Name: "Staggered Set", UserId: 1}
	cs.LaunchDate = launch
	cs.SendByDate = &sendBy
	cs.UseSharedSchedule = true
	cs.Campaigns = []Campaign{first, second}

	// Validate propagates the shared schedule onto each campaign, mirroring the
	// real launch path (LaunchDraftCampaignSet).
	c.Assert(cs.Validate(), check.Equals, nil)
	c.Assert(PostCampaignSet(&cs, 1), check.Equals, nil)

	minutes := map[int]int{}
	totalLogs := 0
	for _, campaign := range cs.Campaigns {
		ms, err := GetMailLogsByCampaign(campaign.Id)
		c.Assert(err, check.Equals, nil)
		for _, m := range ms {
			offset := int(math.Round(m.SendDate.Sub(cs.Campaigns[0].LaunchDate).Minutes()))
			minutes[offset]++
			totalLogs++
		}
	}

	c.Assert(totalLogs, check.Equals, 8)
	// No minute may hold more than one send across the whole set.
	for offset, count := range minutes {
		c.Assert(count, check.Equals, 1, check.Commentf("minute offset %d held %d sends", offset, count))
	}
	// The 8 sends fill 8 distinct minutes.
	c.Assert(len(minutes), check.Equals, 8)
}

// TestCampaignSetSequentialSendOrderFillsBlocks verifies that the default
// (sequential) send order schedules one campaign's recipients fully before the
// next campaign's, across the shared window.
func (s *ModelsSuite) TestCampaignSetSequentialSendOrderFillsBlocks(c *check.C) {
	first := s.createCampaignDependencies(c)
	first.Name = "First Campaign"
	second := s.createCampaignDependencies(c)
	second.Name = "Second Campaign"

	launch := time.Now().UTC().Add(time.Hour)
	sendBy := launch.Add(8 * time.Minute)

	cs := CampaignSet{Name: "Sequential Set", UserId: 1, SendOrder: SendOrderSequential}
	cs.LaunchDate = launch
	cs.SendByDate = &sendBy
	cs.UseSharedSchedule = true
	cs.Campaigns = []Campaign{first, second}

	c.Assert(cs.Validate(), check.Equals, nil)
	c.Assert(PostCampaignSet(&cs, 1), check.Equals, nil)

	firstMin := campaignSendMinutes(c, cs.Campaigns[0].Id, launch)
	secondMin := campaignSendMinutes(c, cs.Campaigns[1].Id, launch)
	sort.Ints(firstMin)
	sort.Ints(secondMin)

	// First campaign fills the first half of the window, second the latter.
	c.Assert(firstMin, check.DeepEquals, []int{0, 1, 2, 3})
	c.Assert(secondMin, check.DeepEquals, []int{4, 5, 6, 7})
}

// TestCampaignSetInterleavedSendOrderDeclusters verifies that interleaved send
// order weaves the campaigns together AND keeps a recipient who appears in both
// campaigns from being emailed in back-to-back slots. Both campaigns target the
// same four addresses, so every recipient is a cross-campaign overlap.
func (s *ModelsSuite) TestCampaignSetInterleavedSendOrderDeclusters(c *check.C) {
	first := s.createCampaignDependencies(c)
	first.Name = "First Campaign"
	second := s.createCampaignDependencies(c)
	second.Name = "Second Campaign"

	launch := time.Now().UTC().Add(time.Hour)
	sendBy := launch.Add(8 * time.Minute)

	cs := CampaignSet{Name: "Interleaved Set", UserId: 1, SendOrder: SendOrderInterleaved}
	cs.LaunchDate = launch
	cs.SendByDate = &sendBy
	cs.UseSharedSchedule = true
	cs.Campaigns = []Campaign{first, second}

	c.Assert(cs.Validate(), check.Equals, nil)
	c.Assert(PostCampaignSet(&cs, 1), check.Equals, nil)

	byEmail := map[string][]int{}
	distinct := map[int]bool{}
	totalLogs := 0
	for _, campaign := range cs.Campaigns {
		var results []Result
		c.Assert(db.Where("campaign_id = ?", campaign.Id).Find(&results).Error, check.Equals, nil)
		for _, r := range results {
			m := int(math.Round(r.SendDate.Sub(launch).Minutes()))
			byEmail[r.Email] = append(byEmail[r.Email], m)
			distinct[m] = true
			totalLogs++
		}
	}

	// 8 sends on 8 distinct minutes: the set is woven into one even stream.
	c.Assert(totalLogs, check.Equals, 8)
	c.Assert(len(distinct), check.Equals, 8)

	// Each address appears in both campaigns; its two sends must be at least
	// two minutes apart (minGap = number of campaigns) so the recipient is not
	// hit twice in a row.
	c.Assert(len(byEmail), check.Equals, 4)
	for email, mins := range byEmail {
		c.Assert(len(mins), check.Equals, 2, check.Commentf("address %s", email))
		sort.Ints(mins)
		c.Assert(mins[1]-mins[0] >= 2, check.Equals, true, check.Commentf("address %s sent at minutes %v", email, mins))
	}
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

func (s *ModelsSuite) TestGetCampaignSetSummaryCountsAttachmentOpened(c *check.C) {
	cs := s.buildTwoCampaignSet(c)
	// test1 opens a tracked attachment in BOTH campaigns.
	for _, campaign := range cs.Campaigns {
		c.Assert(AddEvent(&Event{Email: "test1@example.com", Message: EventAttachmentOpened}, campaign.Id), check.Equals, nil)
	}

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)

	// Totals double-count across the two campaigns.
	c.Assert(summary.Stats.OpenedAttachment, check.Equals, int64(2))
	// Unique dedups test1 to a single contact.
	c.Assert(summary.UniqueStats.OpenedAttachment, check.Equals, int64(1))
}

func (s *ModelsSuite) TestGetCampaignSetSummaryCountsMacroExecuted(c *check.C) {
	cs := s.buildTwoCampaignSet(c)
	// test1 runs the macro in BOTH campaigns.
	for _, campaign := range cs.Campaigns {
		c.Assert(AddEvent(&Event{Email: "test1@example.com", Message: EventMacroExecuted}, campaign.Id), check.Equals, nil)
	}

	summary, err := GetCampaignSetSummary(cs.Id, 1)
	c.Assert(err, check.Equals, nil)

	c.Assert(summary.Stats.MacroExecuted, check.Equals, int64(2))
	c.Assert(summary.UniqueStats.MacroExecuted, check.Equals, int64(1))
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
