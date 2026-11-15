package models

import (
	"os"
	"path/filepath"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestReportSettingsDefaults(c *check.C) {
	rs, err := GetReportSettings()
	c.Assert(err, check.IsNil)
	c.Assert(rs.OnlineGeolocation, check.Equals, false)
	c.Assert(rs.JobTimeoutMinutes, check.Equals, DefaultReportJobTimeoutMinutes)
	c.Assert(rs.RetentionDays, check.Equals, DefaultReportRetentionDays)
}

func (s *ModelsSuite) TestReportSettingsSaveAndUpdate(c *check.C) {
	dbPath := filepath.Join(c.MkDir(), "GeoLite2-City.mmdb")
	c.Assert(os.WriteFile(dbPath, []byte("x"), 0600), check.IsNil)

	in := ReportSettings{OnlineGeolocation: true, GeoIPDatabasePath: " " + dbPath + " ", JobTimeoutMinutes: 10, RetentionDays: 7}
	c.Assert(SaveReportSettings(in), check.IsNil)
	in.RetentionDays = 90
	c.Assert(SaveReportSettings(in), check.IsNil)

	var count int
	c.Assert(db.Model(&ReportSettings{}).Count(&count).Error, check.IsNil)
	c.Assert(count, check.Equals, 1)

	rs, err := GetReportSettings()
	c.Assert(err, check.IsNil)
	c.Assert(rs.OnlineGeolocation, check.Equals, true)
	c.Assert(rs.GeoIPDatabasePath, check.Equals, dbPath)
	c.Assert(rs.JobTimeoutMinutes, check.Equals, 10)
	c.Assert(rs.RetentionDays, check.Equals, 90)
}

func (s *ModelsSuite) TestReportSettingsValidation(c *check.C) {
	cases := []ReportSettings{
		{JobTimeoutMinutes: 0, RetentionDays: 30},
		{JobTimeoutMinutes: MaxReportJobTimeoutMinutes + 1, RetentionDays: 30},
		{JobTimeoutMinutes: 30, RetentionDays: 0},
		{JobTimeoutMinutes: 30, RetentionDays: 30, GeoIPDatabasePath: filepath.Join(c.MkDir(), "missing.mmdb")},
		{JobTimeoutMinutes: 30, RetentionDays: 30, GeoIPDatabasePath: c.MkDir()},
	}
	for _, in := range cases {
		err := SaveReportSettings(in)
		c.Assert(err, check.NotNil)
		c.Assert(err, check.ErrorMatches, "invalid report settings: .*")
	}
}
