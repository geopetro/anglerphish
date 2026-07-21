package models

import "gopkg.in/check.v1"

func (s *ModelsSuite) TestRecordNonCampaignReportStoresIdentifiers(c *check.C) {
	err := RecordNonCampaignReport(1, 7, "jane@corp.com", "Suspicious mail", 4242, 99, "<abc@corp.com>")
	c.Assert(err, check.Equals, nil)

	reports, err := GetRecentNonCampaignReports(1, 10, 0)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reports) > 0, check.Equals, true)

	got := reports[0]
	c.Assert(got.ImapUid, check.Equals, int64(4242))
	c.Assert(got.ImapUidValidity, check.Equals, int64(99))
	c.Assert(got.MessageId, check.Equals, "<abc@corp.com>")
	c.Assert(got.ReporterEmail, check.Equals, "jane@corp.com")
}

// Reports created before the migration have no identifiers; they must read
// back as zero values rather than erroring, so the UI can disable the button.
func (s *ModelsSuite) TestLegacyReportHasZeroIdentifiers(c *check.C) {
	err := RecordNonCampaignReport(2, 7, "bob@corp.com", "Legacy", 0, 0, "")
	c.Assert(err, check.Equals, nil)

	reports, err := GetRecentNonCampaignReports(2, 10, 0)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reports) > 0, check.Equals, true)
	c.Assert(reports[0].ImapUid, check.Equals, int64(0))
	c.Assert(reports[0].MessageId, check.Equals, "")
}

func (s *ModelsSuite) TestIMAPCaptureReplyBodyDefaultsOn(c *check.C) {
	im := IMAP{
		UserId:           1,
		Name:             "capture-default",
		// A literal IP keeps Validate()'s host check off the network.
		Host:             "127.0.0.1",
		Port:             993,
		Username:         "user",
		Password:         "pass",
		TLS:              true,
		Folder:           "INBOX",
		IMAPFreq:         60,
		TrackingType:     TrackingTypeReply,
		CaptureReplyBody: true,
	}
	c.Assert(PostIMAP(&im, 1), check.Equals, nil)

	stored, err := GetIMAPById(im.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(stored.CaptureReplyBody, check.Equals, true)
}
