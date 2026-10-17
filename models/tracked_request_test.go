package models

import (
	check "gopkg.in/check.v1"
)

func (s *ModelsSuite) TestRecordAndGetTrackedRequest(c *check.C) {
	err := RecordTrackedRequest("GET", "/track", "m=1&rid=abc", "203.0.113.5", "Office/16.0")
	c.Assert(err, check.Equals, nil)

	reqs, err := GetTrackedRequests(10)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reqs), check.Equals, 1)
	c.Assert(reqs[0].Method, check.Equals, "GET")
	c.Assert(reqs[0].Path, check.Equals, "/track")
	c.Assert(reqs[0].RawQuery, check.Equals, "m=1&rid=abc")
	c.Assert(reqs[0].SourceIP, check.Equals, "203.0.113.5")
	c.Assert(reqs[0].UserAgent, check.Equals, "Office/16.0")
	c.Assert(reqs[0].CreatedAt.IsZero(), check.Equals, false)
}

func (s *ModelsSuite) TestGetTrackedRequestsNewestFirst(c *check.C) {
	c.Assert(RecordTrackedRequest("GET", "/a", "", "", ""), check.Equals, nil)
	c.Assert(RecordTrackedRequest("GET", "/b", "", "", ""), check.Equals, nil)

	reqs, err := GetTrackedRequests(10)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reqs), check.Equals, 2)
	// Most recent record is returned first.
	c.Assert(reqs[0].Path, check.Equals, "/b")
	c.Assert(reqs[1].Path, check.Equals, "/a")
}

func (s *ModelsSuite) TestDeleteTrackedRequestsByIds(c *check.C) {
	c.Assert(RecordTrackedRequest("GET", "/keep", "", "", ""), check.Equals, nil)
	c.Assert(RecordTrackedRequest("GET", "/drop", "", "", ""), check.Equals, nil)

	reqs, err := GetTrackedRequests(10)
	c.Assert(err, check.Equals, nil)
	var dropId int64
	for _, r := range reqs {
		if r.Path == "/drop" {
			dropId = r.Id
		}
	}
	c.Assert(DeleteTrackedRequestsByIds([]int64{dropId}), check.Equals, nil)

	reqs, err = GetTrackedRequests(10)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reqs), check.Equals, 1)
	c.Assert(reqs[0].Path, check.Equals, "/keep")
}

func (s *ModelsSuite) TestClearTrackedRequests(c *check.C) {
	c.Assert(RecordTrackedRequest("GET", "/a", "", "", ""), check.Equals, nil)
	c.Assert(RecordTrackedRequest("GET", "/b", "", "", ""), check.Equals, nil)

	c.Assert(ClearTrackedRequests(), check.Equals, nil)

	reqs, err := GetTrackedRequests(10)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(reqs), check.Equals, 0)
}
