package models

import (
	"time"

	"gopkg.in/check.v1"
)

// postPeopleCampaign creates and launches a campaign (targets test1..test4)
// for user 1 with the given name and launch date.
func (s *ModelsSuite) postPeopleCampaign(c *check.C, name string, launch time.Time) Campaign {
	campaign := s.createCampaignDependencies(c)
	campaign.Name = name
	campaign.LaunchDate = launch
	c.Assert(PostCampaign(&campaign, 1), check.Equals, nil)
	return campaign
}

// addPeopleEvents records one event per message for email in a campaign.
func addPeopleEvents(c *check.C, cid int64, email string, messages ...string) {
	for _, m := range messages {
		c.Assert(AddEvent(&Event{Email: email, Message: m}, cid), check.Equals, nil)
	}
}

// findPerson returns the row for key, failing the test if it is missing.
func findPerson(c *check.C, people []PersonStats, key string) PersonStats {
	for _, p := range people {
		if p.Key == key {
			return p
		}
	}
	c.Fatalf("person %q not found in %+v", key, people)
	return PersonStats{}
}

func personKeys(people []PersonStats) []string {
	keys := []string{}
	for _, p := range people {
		keys = append(keys, p.Key)
	}
	return keys
}

// twoPeopleCampaigns launches two campaigns a day apart. test1 is sent and
// clicks in both; test2 is sent only in the first.
func (s *ModelsSuite) twoPeopleCampaigns(c *check.C) (Campaign, Campaign) {
	now := time.Now().UTC()
	first := s.postPeopleCampaign(c, "First Campaign", now.Add(-48*time.Hour))
	second := s.postPeopleCampaign(c, "Second Campaign", now.Add(-24*time.Hour))
	addPeopleEvents(c, first.Id, "test1@example.com", EventSent, EventClicked)
	addPeopleEvents(c, second.Id, "test1@example.com", EventSent, EventClicked)
	addPeopleEvents(c, first.Id, "test2@example.com", EventSent)
	return first, second
}

func (s *ModelsSuite) TestPeopleStatsCountsAcrossCampaigns(c *check.C) {
	s.twoPeopleCampaigns(c)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(len(people), check.Equals, 2)
	c.Assert(people[0].Key, check.Equals, "test1@example.com")

	t1 := findPerson(c, people, "test1@example.com")
	c.Assert(t1.Sent, check.Equals, int64(2))
	c.Assert(t1.Clicked, check.Equals, int64(2))
	// Opened is implied by clicked.
	c.Assert(t1.Opened, check.Equals, int64(2))
	c.Assert(t1.Failures, check.Equals, int64(2))
	c.Assert(t1.PhishPronePct, check.Equals, 100.0)
	c.Assert(t1.FirstName, check.Equals, "First")
	c.Assert(t1.LastFailure.IsZero(), check.Equals, false)

	t2 := findPerson(c, people, "test2@example.com")
	c.Assert(t2.Sent, check.Equals, int64(1))
	c.Assert(t2.Failures, check.Equals, int64(0))
	c.Assert(t2.PhishPronePct, check.Equals, 0.0)
	c.Assert(t2.LastFailure.IsZero(), check.Equals, true)
}

// TestPeopleStatsListsCurrentGroups verifies each person carries the names of
// the user's groups they are currently in: deduplicated, sorted, matched
// case-insensitively, and never including another user's groups.
func (s *ModelsSuite) TestPeopleStatsListsCurrentGroups(c *check.C) {
	s.twoPeopleCampaigns(c)

	vip := Group{Name: "VIP", UserId: 1, Targets: []Target{
		{BaseRecipient: BaseRecipient{Email: "TEST1@example.com", FirstName: "First", LastName: "Example"}},
	}}
	c.Assert(PostGroup(&vip), check.Equals, nil)
	foreign := Group{Name: "Someone Else", UserId: 2, Targets: []Target{
		{BaseRecipient: BaseRecipient{Email: "test2@example.com", FirstName: "Second", LastName: "Example"}},
	}}
	c.Assert(PostGroup(&foreign), check.Equals, nil)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	// Both campaigns created their own "Test Group"; the name appears once.
	c.Assert(findPerson(c, people, "test1@example.com").Groups, check.DeepEquals, []string{"Test Group", "VIP"})
	c.Assert(findPerson(c, people, "test2@example.com").Groups, check.DeepEquals, []string{"Test Group"})
}

func (s *ModelsSuite) TestPeopleStatsMergesEmailCase(c *check.C) {
	now := time.Now().UTC()
	first := s.postPeopleCampaign(c, "First Campaign", now.Add(-48*time.Hour))
	second := s.postPeopleCampaign(c, "Second Campaign", now.Add(-24*time.Hour))
	c.Assert(db.Model(&Result{}).Where("campaign_id = ? AND email = ?", second.Id, "test1@example.com").
		Update("email", "Test1@Example.com").Error, check.Equals, nil)
	addPeopleEvents(c, first.Id, "test1@example.com", EventSent)
	addPeopleEvents(c, second.Id, "Test1@Example.com", EventSent, EventClicked)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com"})
	c.Assert(people[0].Sent, check.Equals, int64(2))
	c.Assert(people[0].Failures, check.Equals, int64(1))
	c.Assert(people[0].PhishPronePct, check.Equals, 50.0)
}

func (s *ModelsSuite) TestPeopleStatsSkipsUnsent(c *check.C) {
	s.twoPeopleCampaigns(c)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	// test3 and test4 have result rows in both campaigns but were never sent.
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com", "test2@example.com"})
}

func (s *ModelsSuite) TestPeopleStatsEmptyIsNotNil(c *check.C) {
	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(people, check.NotNil)
	c.Assert(len(people), check.Equals, 0)
}

func (s *ModelsSuite) TestPeopleStatsReportRate(c *check.C) {
	now := time.Now().UTC()
	first := s.postPeopleCampaign(c, "First Campaign", now.Add(-48*time.Hour))
	second := s.postPeopleCampaign(c, "Second Campaign", now.Add(-24*time.Hour))
	addPeopleEvents(c, first.Id, "test1@example.com", EventSent, EventOpened, EventReported)
	addPeopleEvents(c, second.Id, "test1@example.com", EventSent, EventOpened)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	t1 := findPerson(c, people, "test1@example.com")
	c.Assert(t1.Sent, check.Equals, int64(2))
	c.Assert(t1.Reported, check.Equals, int64(1))
	c.Assert(t1.ReportRatePct, check.Equals, 50.0)
	c.Assert(t1.Opened, check.Equals, int64(2))
	// Opening alone is not a failure.
	c.Assert(t1.Failures, check.Equals, int64(0))
	c.Assert(t1.PhishPronePct, check.Equals, 0.0)
}

func (s *ModelsSuite) TestPeopleStatsRoundsPercentages(c *check.C) {
	now := time.Now().UTC()
	for i, name := range []string{"One", "Two", "Three"} {
		campaign := s.postPeopleCampaign(c, name, now.Add(-time.Duration(72-24*i)*time.Hour))
		addPeopleEvents(c, campaign.Id, "test1@example.com", EventSent)
		if i == 0 {
			addPeopleEvents(c, campaign.Id, "test1@example.com", EventReplied)
		}
	}

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	t1 := findPerson(c, people, "test1@example.com")
	// Replying counts as a failure: 1 of 3 campaigns.
	c.Assert(t1.Failures, check.Equals, int64(1))
	c.Assert(t1.PhishPronePct, check.Equals, 33.3)
}

func (s *ModelsSuite) TestPeopleStatsSMSKeyedByPhone(c *check.C) {
	campaign := s.postPeopleCampaign(c, "SMS Campaign", time.Now().UTC().Add(-time.Hour))
	c.Assert(db.Model(&Result{}).Where("campaign_id = ? AND email = ?", campaign.Id, "test1@example.com").
		Updates(map[string]interface{}{"email": "", "phone": "+15550001"}).Error, check.Equals, nil)
	addPeopleEvents(c, campaign.Id, "+15550001", EventSMSSent, EventClicked)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	p := findPerson(c, people, "+15550001")
	c.Assert(p.Sent, check.Equals, int64(1))
	c.Assert(p.Clicked, check.Equals, int64(1))
	c.Assert(p.Phone, check.Equals, "+15550001")
	c.Assert(p.Email, check.Equals, "")
	c.Assert(p.FirstName, check.Equals, "First")
}

func (s *ModelsSuite) TestPeopleStatsFilters(c *check.C) {
	now := time.Now().UTC()
	solo := s.postPeopleCampaign(c, "Solo Campaign", now.Add(-72*time.Hour))
	addPeopleEvents(c, solo.Id, "test3@example.com", EventSent)

	first := s.createCampaignDependencies(c)
	first.Name = "Set Campaign A"
	second := s.createCampaignDependencies(c)
	second.Name = "Set Campaign B"
	cs := CampaignSet{Name: "Test Set", UserId: 1, LaunchDate: now.Add(-24 * time.Hour)}
	cs.Campaigns = []Campaign{first, second}
	c.Assert(PostCampaignSet(&cs, 1), check.Equals, nil)
	for _, campaign := range cs.Campaigns {
		addPeopleEvents(c, campaign.Id, "test1@example.com", EventSent, EventClicked)
	}
	addPeopleEvents(c, cs.Campaigns[0].Id, "test2@example.com", EventSent)

	// Campaign set filter only counts the set's campaigns.
	people, err := GetPeopleStats(1, PeopleFilter{CampaignSetId: cs.Id})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com", "test2@example.com"})
	c.Assert(findPerson(c, people, "test1@example.com").Sent, check.Equals, int64(2))

	// From after the solo launch excludes it.
	people, err = GetPeopleStats(1, PeopleFilter{From: now.Add(-48 * time.Hour)})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com", "test2@example.com"})

	// To before the set launch keeps only the solo campaign.
	people, err = GetPeopleStats(1, PeopleFilter{To: now.Add(-48 * time.Hour)})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test3@example.com"})

	// Group filter keeps only the group's current members.
	g := Group{Name: "Only Test2", UserId: 1}
	g.Targets = []Target{{BaseRecipient: BaseRecipient{Email: "TEST2@example.com"}}}
	c.Assert(PostGroup(&g), check.Equals, nil)
	people, err = GetPeopleStats(1, PeopleFilter{GroupId: g.Id})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test2@example.com"})

	// Another user's campaign set or group is not found.
	foreignSet := CampaignSet{Name: "Foreign Set", UserId: 2}
	c.Assert(db.Save(&foreignSet).Error, check.Equals, nil)
	_, err = GetPeopleStats(1, PeopleFilter{CampaignSetId: foreignSet.Id})
	c.Assert(err, check.Equals, ErrPeopleFilterNotFound)
	_, err = GetPeopleStats(2, PeopleFilter{GroupId: g.Id})
	c.Assert(err, check.Equals, ErrPeopleFilterNotFound)
	_, err = GetPersonTimeline(1, "test1@example.com", PeopleFilter{CampaignSetId: foreignSet.Id})
	c.Assert(err, check.Equals, ErrPeopleFilterNotFound)
}

func (s *ModelsSuite) TestPeopleStatsUserScoped(c *check.C) {
	mine := s.postPeopleCampaign(c, "Mine", time.Now().UTC().Add(-time.Hour))
	addPeopleEvents(c, mine.Id, "test1@example.com", EventSent)

	// Hand a fresh set of dependencies to user 2 and launch a campaign there.
	theirs := s.createCampaignDependencies(c)
	theirs.Name = "Theirs"
	for _, table := range []string{"groups", "templates", "pages", "smtp"} {
		var id int64
		switch table {
		case "groups":
			id = theirs.Groups[0].Id
		case "templates":
			id = theirs.Template.Id
		case "pages":
			id = theirs.Page.Id
		case "smtp":
			id = theirs.SMTP.Id
		}
		c.Assert(db.Table(table).Where("id = ?", id).Update("user_id", 2).Error, check.Equals, nil)
	}
	c.Assert(PostCampaign(&theirs, 2), check.Equals, nil)
	addPeopleEvents(c, theirs.Id, "test1@example.com", EventSent, EventClicked)
	addPeopleEvents(c, theirs.Id, "test2@example.com", EventSent)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com"})
	c.Assert(people[0].Sent, check.Equals, int64(1))
	c.Assert(people[0].Clicked, check.Equals, int64(0))

	people, err = GetPeopleStats(2, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com", "test2@example.com"})
}

func (s *ModelsSuite) TestPersonTimeline(c *check.C) {
	first, second := s.twoPeopleCampaigns(c)

	timeline, err := GetPersonTimeline(1, "Test1@Example.com", PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(len(timeline), check.Equals, 2)
	c.Assert(timeline[0].CampaignId, check.Equals, second.Id)
	c.Assert(timeline[0].CampaignName, check.Equals, "Second Campaign")
	c.Assert(timeline[1].CampaignId, check.Equals, first.Id)
	c.Assert(timeline[0].LaunchDate.After(timeline[1].LaunchDate), check.Equals, true)
	for _, entry := range timeline {
		c.Assert(entry.Failed, check.Equals, true)
		c.Assert(entry.Flags["clicked"], check.Equals, true)
		c.Assert(entry.Flags["opened"], check.Equals, true)
		c.Assert(entry.Flags["sent"], check.Equals, true)
	}

	timeline, err = GetPersonTimeline(1, "nobody@example.com", PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(timeline, check.NotNil)
	c.Assert(len(timeline), check.Equals, 0)

	// test3 has a result row but was never sent, so has no timeline.
	timeline, err = GetPersonTimeline(1, "test3@example.com", PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(len(timeline), check.Equals, 0)
}

// postPeopleSMSCampaign creates and launches an SMS campaign for user 1 whose
// targets (test1..test4) have both an email and a phone number.
func (s *ModelsSuite) postPeopleSMSCampaign(c *check.C, name string, launch time.Time) Campaign {
	campaign := s.createSMSCampaignDependencies(c)
	campaign.Name = name
	campaign.LaunchDate = launch
	c.Assert(PostCampaign(&campaign, 1), check.Equals, nil)
	return campaign
}

func (s *ModelsSuite) TestPeopleStatsSMSWithEmailAndPhone(c *check.C) {
	now := time.Now().UTC()
	// Targets are shared by email, so post the SMS campaign first to keep
	// their phone numbers.
	sms := s.postPeopleSMSCampaign(c, "SMS Campaign", now.Add(-24*time.Hour))
	email := s.postPeopleCampaign(c, "Email Campaign", now.Add(-48*time.Hour))
	addPeopleEvents(c, email.Id, "test1@example.com", EventSent)
	// SMS events are recorded against the phone number, the result row
	// against the email.
	addPeopleEvents(c, sms.Id, "+15551234567", EventSMSSent, EventClicked)
	c.Assert(db.Model(&Result{}).Where("campaign_id = ? AND email = ?", sms.Id, "test1@example.com").
		Update("reported", true).Error, check.Equals, nil)

	check1 := func(people []PersonStats) {
		c.Assert(personKeys(people), check.DeepEquals, []string{"test1@example.com"})
		p := people[0]
		c.Assert(p.Sent, check.Equals, int64(2))
		c.Assert(p.Clicked, check.Equals, int64(1))
		c.Assert(p.Reported, check.Equals, int64(1))
		c.Assert(p.Failures, check.Equals, int64(1))
		c.Assert(p.Email, check.Equals, "test1@example.com")
		c.Assert(p.Phone, check.Equals, "+15551234567")
	}

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	check1(people)

	g := Group{Name: "Only Test1", UserId: 1}
	g.Targets = []Target{{BaseRecipient: BaseRecipient{Email: "test1@example.com", Phone: "+15551234567"}}}
	c.Assert(PostGroup(&g), check.Equals, nil)
	people, err = GetPeopleStats(1, PeopleFilter{GroupId: g.Id})
	c.Assert(err, check.Equals, nil)
	check1(people)

	timeline, err := GetPersonTimeline(1, "test1@example.com", PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	c.Assert(len(timeline), check.Equals, 2)
	c.Assert(timeline[0].CampaignId, check.Equals, sms.Id)
	c.Assert(timeline[0].Flags["clicked"], check.Equals, true)
	c.Assert(timeline[0].Flags["reported"], check.Equals, true)
	c.Assert(timeline[0].Failed, check.Equals, true)
	c.Assert(timeline[1].CampaignId, check.Equals, email.Id)
}

func (s *ModelsSuite) TestPeopleStatsOrphanPhoneEvents(c *check.C) {
	campaign := s.postPeopleCampaign(c, "Orphan Campaign", time.Now().UTC().Add(-time.Hour))
	// Events for a recipient with no result row (e.g. a deleted result).
	addPeopleEvents(c, campaign.Id, "+15559999", EventSMSSent)
	addPeopleEvents(c, campaign.Id, "gone@example.com", EventSent)

	people, err := GetPeopleStats(1, PeopleFilter{})
	c.Assert(err, check.Equals, nil)
	p := findPerson(c, people, "+15559999")
	c.Assert(p.Phone, check.Equals, "+15559999")
	c.Assert(p.Email, check.Equals, "")
	p = findPerson(c, people, "gone@example.com")
	c.Assert(p.Email, check.Equals, "gone@example.com")
	c.Assert(p.Phone, check.Equals, "")
}
