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
