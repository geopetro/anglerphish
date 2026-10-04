package models

import (
	"fmt"
	"testing"

	"github.com/jinzhu/gorm"
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestPostGroup(c *check.C) {
	g := Group{Name: "Test Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "test@example.com"}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, nil)
	c.Assert(g.Name, check.Equals, "Test Group")
	c.Assert(g.Targets[0].Email, check.Equals, "test@example.com")
}

func (s *ModelsSuite) TestPostGroupWithPhone(c *check.C) {
	g := Group{Name: "Test Phone Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{
		Email: "test@example.com",
		Phone: "+15551234567",
	}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, nil)
	c.Assert(g.Name, check.Equals, "Test Phone Group")
	c.Assert(g.Targets[0].Email, check.Equals, "test@example.com")
	c.Assert(g.Targets[0].Phone, check.Equals, "+15551234567")
}

func (s *ModelsSuite) TestPostGroupPhoneOnly(c *check.C) {
	g := Group{Name: "Test Phone Only Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{
		Phone: "+15551234567",
	}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, nil)
	c.Assert(g.Name, check.Equals, "Test Phone Only Group")
	c.Assert(g.Targets[0].Phone, check.Equals, "+15551234567")
}

func (s *ModelsSuite) TestPostGroupNoName(c *check.C) {
	g := Group{Name: ""}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "test@example.com"}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, ErrGroupNameNotSpecified)
}

func (s *ModelsSuite) TestPostGroupNoTargets(c *check.C) {
	g := Group{Name: "No Target Group"}
	g.Targets = []Target{}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, ErrNoTargetsSpecified)
}

func (s *ModelsSuite) TestGetGroups(c *check.C) {
	// Add groups.
	PostGroup(&Group{
		Name: "Test Group 1",
		Targets: []Target{
			Target{
				BaseRecipient: BaseRecipient{Email: "test1@example.com"},
			},
		},
		UserId: 1,
	})
	PostGroup(&Group{
		Name: "Test Group 2",
		Targets: []Target{
			Target{
				BaseRecipient: BaseRecipient{Email: "test2@example.com"},
			},
		},
		UserId: 1,
	})

	// Get groups and test result.
	groups, err := GetGroups(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(groups), check.Equals, 2)
	c.Assert(len(groups[0].Targets), check.Equals, 1)
	c.Assert(len(groups[1].Targets), check.Equals, 1)
	c.Assert(groups[0].Name, check.Equals, "Test Group 1")
	c.Assert(groups[1].Name, check.Equals, "Test Group 2")
	c.Assert(groups[0].Targets[0].Email, check.Equals, "test1@example.com")
	c.Assert(groups[1].Targets[0].Email, check.Equals, "test2@example.com")
}

func (s *ModelsSuite) TestGetGroupsNoGroups(c *check.C) {
	groups, err := GetGroups(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(groups), check.Equals, 0)
}

func (s *ModelsSuite) TestGetGroup(c *check.C) {
	// Add group.
	originalGroup := &Group{
		Name: "Test Group",
		Targets: []Target{
			Target{
				BaseRecipient: BaseRecipient{Email: "test@example.com"},
			},
		},
		UserId: 1,
	}
	c.Assert(PostGroup(originalGroup), check.Equals, nil)

	// Get group and test result.
	group, err := GetGroup(originalGroup.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(group.Targets), check.Equals, 1)
	c.Assert(group.Name, check.Equals, "Test Group")
	c.Assert(group.Targets[0].Email, check.Equals, "test@example.com")
}

func (s *ModelsSuite) TestGetGroupNoGroups(c *check.C) {
	_, err := GetGroup(1, 1)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}

func (s *ModelsSuite) TestGetGroupByName(c *check.C) {
	// Add group.
	PostGroup(&Group{
		Name: "Test Group",
		Targets: []Target{
			Target{
				BaseRecipient: BaseRecipient{Email: "test@example.com"},
			},
		},
		UserId: 1,
	})

	// Get group and test result.
	group, err := GetGroupByName("Test Group", 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(group.Targets), check.Equals, 1)
	c.Assert(group.Name, check.Equals, "Test Group")
	c.Assert(group.Targets[0].Email, check.Equals, "test@example.com")
}

func (s *ModelsSuite) TestGetGroupByNameNoGroups(c *check.C) {
	_, err := GetGroupByName("Test Group", 1)
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}

func (s *ModelsSuite) TestPutGroup(c *check.C) {
	// Add test group.
	group := Group{Name: "Test Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "test1@example.com", FirstName: "First", LastName: "Example"}},
		Target{BaseRecipient: BaseRecipient{Email: "test2@example.com", FirstName: "Second", LastName: "Example"}},
	}
	group.UserId = 1
	PostGroup(&group)

	// Update one of group's targets.
	group.Targets[0].FirstName = "Updated"
	err := PutGroup(&group)
	c.Assert(err, check.Equals, nil)

	// Verify updated target information.
	targets, _ := GetTargets(group.Id)
	c.Assert(targets[0].Email, check.Equals, "test1@example.com")
	c.Assert(targets[0].FirstName, check.Equals, "Updated")
	c.Assert(targets[0].LastName, check.Equals, "Example")
	c.Assert(targets[1].Email, check.Equals, "test2@example.com")
	c.Assert(targets[1].FirstName, check.Equals, "Second")
	c.Assert(targets[1].LastName, check.Equals, "Example")
}

func (s *ModelsSuite) TestPutGroupEmptyAttribute(c *check.C) {
	// Add test group.
	group := Group{Name: "Test Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "test1@example.com", FirstName: "First", LastName: "Example"}},
		Target{BaseRecipient: BaseRecipient{Email: "test2@example.com", FirstName: "Second", LastName: "Example"}},
	}
	group.UserId = 1
	PostGroup(&group)

	// Update one of group's targets.
	group.Targets[0].FirstName = ""
	err := PutGroup(&group)
	c.Assert(err, check.Equals, nil)

	// Verify updated empty attribute was saved.
	targets, _ := GetTargets(group.Id)
	c.Assert(targets[0].Email, check.Equals, "test1@example.com")
	c.Assert(targets[0].FirstName, check.Equals, "")
	c.Assert(targets[0].LastName, check.Equals, "Example")
	c.Assert(targets[1].Email, check.Equals, "test2@example.com")
	c.Assert(targets[1].FirstName, check.Equals, "Second")
	c.Assert(targets[1].LastName, check.Equals, "Example")
}

func (s *ModelsSuite) TestPutGroupUpdatePhone(c *check.C) {
	// Add test group with phone numbers.
	group := Group{Name: "Test Phone Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{
			Email:     "test1@example.com",
			Phone:     "+15551234567",
			FirstName: "First",
			LastName:  "Example",
		}},
		Target{BaseRecipient: BaseRecipient{
			Email:     "test2@example.com",
			Phone:     "+15552345678",
			FirstName: "Second",
			LastName:  "Example",
		}},
	}
	group.UserId = 1
	PostGroup(&group)

	// Update one of group's target's phone number.
	group.Targets[0].Phone = "+15559876543"
	err := PutGroup(&group)
	c.Assert(err, check.Equals, nil)

	// Verify updated phone number was saved.
	targets, _ := GetTargets(group.Id)
	c.Assert(targets[0].Email, check.Equals, "test1@example.com")
	c.Assert(targets[0].Phone, check.Equals, "+15559876543")
	c.Assert(targets[0].FirstName, check.Equals, "First")
	c.Assert(targets[0].LastName, check.Equals, "Example")
	c.Assert(targets[1].Email, check.Equals, "test2@example.com")
	c.Assert(targets[1].Phone, check.Equals, "+15552345678")
	c.Assert(targets[1].FirstName, check.Equals, "Second")
	c.Assert(targets[1].LastName, check.Equals, "Example")
}

func (s *ModelsSuite) TestPostGroupInvalidPhone(c *check.C) {
	g := Group{Name: "Test Invalid Phone Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{
		Phone: "invalid-phone",
	}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Invalid phone number format")
}

func (s *ModelsSuite) TestPostGroupWithCustomField(c *check.C) {
	g := Group{Name: "Test Custom Field Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{
		Email:     "test@example.com",
		FirstName: "John",
		LastName:  "Doe",
		Custom:    "Department: Engineering",
	}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, nil)
	c.Assert(g.Name, check.Equals, "Test Custom Field Group")
	c.Assert(g.Targets[0].Email, check.Equals, "test@example.com")
	c.Assert(g.Targets[0].Custom, check.Equals, "Department: Engineering")
}

func (s *ModelsSuite) TestPutGroupUpdateCustomField(c *check.C) {
	// Add test group with custom field.
	group := Group{Name: "Test Custom Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{
			Email:     "test1@example.com",
			FirstName: "First",
			LastName:  "Example",
			Custom:    "Original Custom Data",
		}},
		Target{BaseRecipient: BaseRecipient{
			Email:     "test2@example.com",
			FirstName: "Second",
			LastName:  "Example",
			Custom:    "Another Custom Field",
		}},
	}
	group.UserId = 1
	PostGroup(&group)

	// Update one of group's target's custom field.
	group.Targets[0].Custom = "Updated Custom Data"
	err := PutGroup(&group)
	c.Assert(err, check.Equals, nil)

	// Verify updated custom field was saved.
	targets, _ := GetTargets(group.Id)
	c.Assert(targets[0].Email, check.Equals, "test1@example.com")
	c.Assert(targets[0].Custom, check.Equals, "Updated Custom Data")
	c.Assert(targets[0].FirstName, check.Equals, "First")
	c.Assert(targets[0].LastName, check.Equals, "Example")
	c.Assert(targets[1].Email, check.Equals, "test2@example.com")
	c.Assert(targets[1].Custom, check.Equals, "Another Custom Field")
	c.Assert(targets[1].FirstName, check.Equals, "Second")
	c.Assert(targets[1].LastName, check.Equals, "Example")
}

func (s *ModelsSuite) TestPostGroupWithAllFields(c *check.C) {
	// Test creating a group with all possible target fields
	g := Group{Name: "Test All Fields Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{
		Email:     "complete@example.com",
		Phone:     "+15551234567",
		FirstName: "Complete",
		LastName:  "User",
		Position:  "Manager",
		Custom:    "Custom: All fields test",
	}}}
	g.UserId = 1
	err := PostGroup(&g)
	c.Assert(err, check.Equals, nil)

	// Verify all fields are saved correctly
	saved, err := GetGroup(g.Id, g.UserId)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(saved.Targets), check.Equals, 1)
	target := saved.Targets[0]
	c.Assert(target.Email, check.Equals, "complete@example.com")
	c.Assert(target.Phone, check.Equals, "+15551234567")
	c.Assert(target.FirstName, check.Equals, "Complete")
	c.Assert(target.LastName, check.Equals, "User")
	c.Assert(target.Position, check.Equals, "Manager")
	c.Assert(target.Custom, check.Equals, "Custom: All fields test")
}

func (s *ModelsSuite) TestDeleteGroupRemovesOrphanedTargets(c *check.C) {
	// Add a group with a single target that belongs to no other group.
	group := Group{Name: "Test Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "orphan@example.com"}},
	}
	group.UserId = 1
	c.Assert(PostGroup(&group), check.Equals, nil)

	c.Assert(DeleteGroup(&group), check.Equals, nil)

	// The target should no longer exist in the database, not just
	// unlinked from the group.
	var target Target
	err := db.Where("email=?", "orphan@example.com").Find(&target).Error
	c.Assert(err, check.Equals, gorm.ErrRecordNotFound)
}

func (s *ModelsSuite) TestDeleteGroupKeepsTargetsStillInUse(c *check.C) {
	// Two groups sharing the same target.
	group1 := Group{Name: "Group One"}
	group1.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "shared@example.com", FirstName: "Shared"}},
	}
	group1.UserId = 1
	c.Assert(PostGroup(&group1), check.Equals, nil)

	group2 := Group{Name: "Group Two"}
	group2.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "shared@example.com", FirstName: "Shared"}},
	}
	group2.UserId = 1
	c.Assert(PostGroup(&group2), check.Equals, nil)

	// Deleting group1 must not remove the target, since group2 still uses it.
	c.Assert(DeleteGroup(&group1), check.Equals, nil)

	targets, err := GetTargets(group2.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(targets), check.Equals, 1)
	c.Assert(targets[0].Email, check.Equals, "shared@example.com")
}

func (s *ModelsSuite) TestPostGroupAfterDeleteUsesFreshTargetData(c *check.C) {
	// Reproduces the reported scenario: a group is imported from a CSV,
	// deleted, and then re-imported from an updated CSV with the same
	// recipient email but changed details. The re-import must reflect the
	// new data, not the stale data from before the deletion.
	original := Group{Name: "Imported Group"}
	original.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{
			Email:     "recreate@example.com",
			FirstName: "Old",
			Position:  "Old Position",
		}},
	}
	original.UserId = 1
	c.Assert(PostGroup(&original), check.Equals, nil)
	c.Assert(DeleteGroup(&original), check.Equals, nil)

	reimported := Group{Name: "Imported Group"}
	reimported.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{
			Email:     "recreate@example.com",
			FirstName: "New",
			Position:  "New Position",
		}},
	}
	reimported.UserId = 1
	c.Assert(PostGroup(&reimported), check.Equals, nil)

	targets, err := GetTargets(reimported.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(targets), check.Equals, 1)
	c.Assert(targets[0].Email, check.Equals, "recreate@example.com")
	c.Assert(targets[0].FirstName, check.Equals, "New")
	c.Assert(targets[0].Position, check.Equals, "New Position")
}

func (s *ModelsSuite) TestPostGroupRefreshesStaleTargetDataOnMatch(c *check.C) {
	// Even without a delete in between, adding a target to a new group
	// with an email that already exists must refresh that target's
	// details rather than silently keeping the old ones.
	group1 := Group{Name: "Group One"}
	group1.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "refresh@example.com", FirstName: "Old", Custom: "OldCustom"}},
	}
	group1.UserId = 1
	c.Assert(PostGroup(&group1), check.Equals, nil)

	group2 := Group{Name: "Group Two"}
	group2.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "refresh@example.com", FirstName: "New", Custom: "NewCustom"}},
	}
	group2.UserId = 1
	c.Assert(PostGroup(&group2), check.Equals, nil)

	targets, err := GetTargets(group2.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(targets), check.Equals, 1)
	c.Assert(targets[0].FirstName, check.Equals, "New")
	c.Assert(targets[0].Custom, check.Equals, "NewCustom")
}

func (s *ModelsSuite) TestToggleGroupLock(c *check.C) {
	g := Group{Name: "Test Lock Group"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "test@example.com"}}}
	g.UserId = 1
	c.Assert(PostGroup(&g), check.Equals, nil)
	c.Assert(g.Locked, check.Equals, false)

	// Lock the group
	locked, err := ToggleGroupLock(g.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(locked.Locked, check.Equals, true)

	// Unlock the group
	unlocked, err := ToggleGroupLock(g.Id, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(unlocked.Locked, check.Equals, false)
}

func (s *ModelsSuite) TestToggleGroupLockWrongUser(c *check.C) {
	g := Group{Name: "Test Lock Group Wrong User"}
	g.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "test@example.com"}}}
	g.UserId = 1
	c.Assert(PostGroup(&g), check.Equals, nil)

	// Attempt to lock with a different user ID
	_, err := ToggleGroupLock(g.Id, 2)
	c.Assert(err, check.NotNil)
}

func (s *ModelsSuite) TestGetGroupSummariesLockedField(c *check.C) {
	// Create one unlocked and one locked group
	g1 := Group{Name: "Unlocked Group"}
	g1.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "a@example.com"}}}
	g1.UserId = 1
	c.Assert(PostGroup(&g1), check.Equals, nil)

	g2 := Group{Name: "Locked Group"}
	g2.Targets = []Target{Target{BaseRecipient: BaseRecipient{Email: "b@example.com"}}}
	g2.UserId = 1
	c.Assert(PostGroup(&g2), check.Equals, nil)
	_, err := ToggleGroupLock(g2.Id, 1)
	c.Assert(err, check.Equals, nil)

	summaries, err := GetGroupSummaries(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(summaries.Groups), check.Equals, 2)

	// Find each summary by name and check locked field
	var foundUnlocked, foundLocked bool
	for _, s := range summaries.Groups {
		if s.Name == "Unlocked Group" {
			c.Assert(s.Locked, check.Equals, false)
			foundUnlocked = true
		}
		if s.Name == "Locked Group" {
			c.Assert(s.Locked, check.Equals, true)
			foundLocked = true
		}
	}
	c.Assert(foundUnlocked, check.Equals, true)
	c.Assert(foundLocked, check.Equals, true)
}

func benchmarkPostGroup(b *testing.B, iter, size int) {
	b.StopTimer()
	g := &Group{
		Name: fmt.Sprintf("Group-%d", iter),
	}
	for i := 0; i < size; i++ {
		g.Targets = append(g.Targets, Target{
			BaseRecipient: BaseRecipient{
				FirstName: "User",
				LastName:  fmt.Sprintf("%d", i),
				Email:     fmt.Sprintf("test-%d@test.com", i),
			},
		})
	}
	b.StartTimer()
	err := PostGroup(g)
	if err != nil {
		b.Fatalf("error posting group: %v", err)
	}
}

// benchmarkPutGroup modifies half of the group to simulate a large change
func benchmarkPutGroup(b *testing.B, iter, size int) {
	b.StopTimer()
	// First, we need to create the group
	g := &Group{
		Name: fmt.Sprintf("Group-%d", iter),
	}
	for i := 0; i < size; i++ {
		g.Targets = append(g.Targets, Target{
			BaseRecipient: BaseRecipient{
				FirstName: "User",
				LastName:  fmt.Sprintf("%d", i),
				Email:     fmt.Sprintf("test-%d@test.com", i),
			},
		})
	}
	err := PostGroup(g)
	if err != nil {
		b.Fatalf("error posting group: %v", err)
	}
	// Now we need to change half of the group.
	for i := 0; i < size/2; i++ {
		g.Targets[i].Email = fmt.Sprintf("test-modified-%d@test.com", i)
	}
	b.StartTimer()
	err = PutGroup(g)
	if err != nil {
		b.Fatalf("error modifying group: %v", err)
	}
}

func BenchmarkPostGroup100(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPostGroup(b, i, 100)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}

func BenchmarkPostGroup1000(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPostGroup(b, i, 1000)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}

func BenchmarkPostGroup10000(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPostGroup(b, i, 10000)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}

func BenchmarkPutGroup100(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPutGroup(b, i, 100)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}

func BenchmarkPutGroup1000(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPutGroup(b, i, 1000)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}

func BenchmarkPutGroup10000(b *testing.B) {
	setupBenchmark(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPutGroup(b, i, 10000)
		b.StopTimer()
		resetBenchmark(b)
	}
	tearDownBenchmark(b)
}
