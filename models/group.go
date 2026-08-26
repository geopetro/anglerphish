package models

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Group contains the fields needed for a user -> group mapping
// Groups contain 1..* Targets
type Group struct {
	Id           int64     `json:"id"`
	UserId       int64     `json:"-"`
	Name         string    `json:"name"`
	ModifiedDate time.Time `json:"modified_date"`
	Locked       bool      `json:"locked"`
	Targets      []Target  `json:"targets" sql:"-"`
}

// GroupSummaries is a struct representing the overview of Groups.
type GroupSummaries struct {
	Total  int64          `json:"total"`
	Groups []GroupSummary `json:"groups"`
}

// GroupSummary represents a summary of the Group model. The only
// difference is that, instead of listing the Targets (which could be expensive
// for large groups), it lists the target count.
type GroupSummary struct {
	Id           int64     `json:"id"`
	Name         string    `json:"name"`
	ModifiedDate time.Time `json:"modified_date"`
	Locked       bool      `json:"locked"`
	NumTargets   int64     `json:"num_targets"`
}

// GroupTarget is used for a many-to-many relationship between 1..* Groups and 1..* Targets
type GroupTarget struct {
	GroupId  int64 `json:"-"`
	TargetId int64 `json:"-"`
}

// Target contains the fields needed for individual targets specified by the user
// Groups contain 1..* Targets, but 1 Target may belong to 1..* Groups
type Target struct {
	Id int64 `json:"-"`
	BaseRecipient
}

// BaseRecipient contains the fields for a single recipient. This is the base
// struct used in members of groups and campaign results.
type BaseRecipient struct {
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Position  string `json:"position"`
	Custom    string `json:"custom"`
}

// FormatAddress returns the email address to use in the "To" header of the email
func (r *BaseRecipient) FormatAddress() string {
	addr := r.Email
	if r.FirstName != "" && r.LastName != "" {
		a := &mail.Address{
			Name:    fmt.Sprintf("%s %s", r.FirstName, r.LastName),
			Address: r.Email,
		}
		addr = a.String()
	}
	return addr
}

// FormatAddress returns the email address to use in the "To" header of the email
func (t *Target) FormatAddress() string {
	addr := t.Email
	if t.FirstName != "" && t.LastName != "" {
		a := &mail.Address{
			Name:    fmt.Sprintf("%s %s", t.FirstName, t.LastName),
			Address: t.Email,
		}
		addr = a.String()
	}
	return addr
}

// ErrEmailNotSpecified is thrown when no email is specified for the Target
var ErrEmailNotSpecified = errors.New("No email address specified")

// ErrGroupNameNotSpecified is thrown when a group name is not specified
var ErrGroupNameNotSpecified = errors.New("Group name not specified")

// ErrNoTargetsSpecified is thrown when no targets are specified by the user
var ErrNoTargetsSpecified = errors.New("No targets specified")

// Validate performs validation on a group given by the user
func (g *Group) Validate() error {
	switch {
	case g.Name == "":
		return ErrGroupNameNotSpecified
	case len(g.Targets) == 0:
		return ErrNoTargetsSpecified
	}
	return nil
}

// GetGroups returns the groups owned by the given user.
func GetGroups(uid int64) ([]Group, error) {
	gs := []Group{}
	err := db.Where("user_id=?", uid).Find(&gs).Error
	if err != nil {
		log.Error(err)
		return gs, err
	}
	for i := range gs {
		gs[i].Targets, err = GetTargets(gs[i].Id)
		if err != nil {
			log.Error(err)
		}
	}
	return gs, nil
}

// GetGroupSummaries returns the summaries for the groups
// created by the given uid.
func GetGroupSummaries(uid int64) (GroupSummaries, error) {
	gs := GroupSummaries{}
	query := db.Table("groups").Where("user_id=?", uid)
	err := query.Select("id, name, modified_date, locked").Scan(&gs.Groups).Error
	if err != nil {
		log.Error(err)
		return gs, err
	}

	log.WithFields(logrus.Fields{
		"user_id":      uid,
		"groups_count": len(gs.Groups),
	}).Info("Fetched groups for user")

	for i := range gs.Groups {
		// Get the count of targets for this group
		query = db.Table("group_targets").Where("group_id=?", gs.Groups[i].Id)
		err = query.Count(&gs.Groups[i].NumTargets).Error
		if err != nil {
			log.WithFields(logrus.Fields{
				"group_id": gs.Groups[i].Id,
				"error":    err,
			}).Error("Error counting targets for group")
			return gs, err
		}

		// Log the count for debugging
		log.WithFields(logrus.Fields{
			"group_id":    gs.Groups[i].Id,
			"group_name":  gs.Groups[i].Name,
			"num_targets": gs.Groups[i].NumTargets,
		}).Info("Group target count")

		// Double-check by getting the actual targets
		targets, err := GetTargets(gs.Groups[i].Id)
		if err != nil {
			log.WithFields(logrus.Fields{
				"group_id": gs.Groups[i].Id,
				"error":    err,
			}).Error("Error getting targets for group")
		} else {
			log.WithFields(logrus.Fields{
				"group_id":           gs.Groups[i].Id,
				"targets_count":      len(targets),
				"targets_with_email": countTargetsWithEmail(targets),
				"targets_with_phone": countTargetsWithPhone(targets),
			}).Info("Actual targets for group")
		}
	}

	gs.Total = int64(len(gs.Groups))
	return gs, nil
}

// Helper function to count targets with email
func countTargetsWithEmail(targets []Target) int {
	count := 0
	for _, t := range targets {
		if t.Email != "" {
			count++
		}
	}
	return count
}

// Helper function to count targets with phone
func countTargetsWithPhone(targets []Target) int {
	count := 0
	for _, t := range targets {
		if t.Phone != "" {
			count++
		}
	}
	return count
}

// GetGroup returns the group, if it exists, specified by the given id and user_id.
func GetGroup(id int64, uid int64) (Group, error) {
	g := Group{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&g).Error
	if err != nil {
		log.Error(err)
		return g, err
	}
	g.Targets, err = GetTargets(g.Id)
	if err != nil {
		log.Error(err)
	}
	return g, nil
}

// GetGroupSummary returns the summary for the requested group
func GetGroupSummary(id int64, uid int64) (GroupSummary, error) {
	g := GroupSummary{}
	query := db.Table("groups").Where("user_id=? and id=?", uid, id)
	err := query.Select("id, name, modified_date, locked").Scan(&g).Error
	if err != nil {
		log.Error(err)
		return g, err
	}
	query = db.Table("group_targets").Where("group_id=?", id)
	err = query.Count(&g.NumTargets).Error
	if err != nil {
		return g, err
	}
	return g, nil
}

// GetGroupByName returns the group, if it exists, specified by the given name and user_id.
func GetGroupByName(n string, uid int64) (Group, error) {
	g := Group{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&g).Error
	if err != nil {
		log.Error(err)
		return g, err
	}
	g.Targets, err = GetTargets(g.Id)
	if err != nil {
		log.Error(err)
	}
	return g, err
}

// PostGroup creates a new group in the database.
func PostGroup(g *Group) error {
	if err := g.Validate(); err != nil {
		return err
	}
	// Insert the group into the DB
	tx := db.Begin()
	err := tx.Save(g).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}
	for _, t := range g.Targets {
		err = insertTargetIntoGroup(tx, t, g.Id)
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	return nil
}

// PutGroup updates the given group if found in the database.
func PutGroup(g *Group) error {
	if err := g.Validate(); err != nil {
		return err
	}
	// Fetch group's existing targets from database.
	ts, err := GetTargets(g.Id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"group_id": g.Id,
		}).Error("Error getting targets from group")
		return err
	}

	// Log the existing and new targets for debugging
	log.WithFields(logrus.Fields{
		"group_id":         g.Id,
		"existing_targets": len(ts),
		"new_targets":      len(g.Targets),
	}).Info("PutGroup called with targets")

	// Preload the caches - we need to handle both email and phone identifiers
	emailCacheNew := make(map[string]int64, len(g.Targets))
	phoneCacheNew := make(map[string]int64, len(g.Targets))
	for _, t := range g.Targets {
		if t.Email != "" {
			emailCacheNew[t.Email] = t.Id
		}
		if t.Phone != "" {
			phoneCacheNew[t.Phone] = t.Id
		}
	}

	emailCacheExisting := make(map[string]int64, len(ts))
	phoneCacheExisting := make(map[string]int64, len(ts))
	for _, t := range ts {
		if t.Email != "" {
			emailCacheExisting[t.Email] = t.Id
		}
		if t.Phone != "" {
			phoneCacheExisting[t.Phone] = t.Id
		}
	}

	tx := db.Begin()

	// IMPORTANT: We're removing this code that deletes targets not in the new list
	// This was causing the issue where adding a new target would remove all others

	// Check existing targets, removing any that are no longer in the group.
	for _, existing := range ts {
		existsInNew := false

		// Normalize
		existingEmail := strings.ToLower(strings.TrimSpace(existing.Email))
		existingPhone := regexp.MustCompile(`\D`).ReplaceAllString(existing.Phone, "")

		for _, incoming := range g.Targets {
			newEmail := strings.ToLower(strings.TrimSpace(incoming.Email))
			newPhone := regexp.MustCompile(`\D`).ReplaceAllString(incoming.Phone, "")

			if existingEmail != "" && existingEmail == newEmail {
				existsInNew = true
				break
			}
			if existingPhone != "" && existingPhone == newPhone {
				existsInNew = true
				break
			}
		}

		if !existsInNew {
			log.WithFields(logrus.Fields{
				"group_id":  g.Id,
				"target_id": existing.Id,
				"email":     existing.Email,
				"phone":     existing.Phone,
			}).Info("Removing target from group")

			err := tx.Where("group_id = ? AND target_id = ?", g.Id, existing.Id).Delete(&GroupTarget{}).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"group_id": g.Id,
					"error":    err,
				}).Error("Failed to remove GroupTarget")
				tx.Rollback()
				return err
			}
		}
	}

	// Add any targets that are not in the database yet.
	for _, nt := range g.Targets {
		// If the target already exists in the database, we should just update
		// the record with the latest information.
		existingId := int64(0)

		// Check by email if available
		if nt.Email != "" && emailCacheExisting[nt.Email] != 0 {
			existingId = emailCacheExisting[nt.Email]
			log.WithFields(logrus.Fields{
				"email":       nt.Email,
				"existing_id": existingId,
			}).Debug("Found existing target by email")
		}

		// Check by phone if available and we haven't found by email
		if existingId == 0 && nt.Phone != "" && phoneCacheExisting[nt.Phone] != 0 {
			existingId = phoneCacheExisting[nt.Phone]
			log.WithFields(logrus.Fields{
				"phone":       nt.Phone,
				"existing_id": existingId,
			}).Debug("Found existing target by phone")
		}

		if existingId != 0 {
			nt.Id = existingId
			err = UpdateTarget(tx, nt)
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
			continue
		}
		// Otherwise, add target if not in database
		err = insertTargetIntoGroup(tx, nt, g.Id)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
	}
	err = tx.Save(g).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// ToggleGroupLock flips the locked flag for the given group.
func ToggleGroupLock(id int64, uid int64) (Group, error) {
	g := Group{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&g).Error
	if err != nil {
		return g, err
	}
	g.Locked = !g.Locked
	err = db.Model(&g).Update("locked", g.Locked).Error
	return g, err
}

// DeleteGroup deletes a given group by group ID and user ID
func DeleteGroup(g *Group) error {
	tx := db.Begin()

	// Remember which targets were linked to this group so we can check
	// whether they become orphaned once the links are removed.
	var targetIds []int64
	err := tx.Model(&GroupTarget{}).Where("group_id=?", g.Id).Pluck("target_id", &targetIds).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}

	// Delete all the group_targets entries for this group
	err = tx.Where("group_id=?", g.Id).Delete(&GroupTarget{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	// Delete the group itself
	err = tx.Delete(g).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}

	// Clean up targets that no longer belong to any group. Without this,
	// deleted targets keep existing in the database and get silently
	// reattached (with their old, stale data) the next time a target with
	// the same email or phone is added to a new or re-imported group.
	for _, tid := range targetIds {
		var count int
		err = tx.Model(&GroupTarget{}).Where("target_id=?", tid).Count(&count).Error
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
		if count == 0 {
			err = tx.Where("id=?", tid).Delete(&Target{}).Error
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit().Error
}

// ErrNoContactInfoSpecified is thrown when neither email nor phone is provided for a target
var ErrNoContactInfoSpecified = errors.New("Either email or phone must be specified")

func insertTargetIntoGroup(tx *gorm.DB, t Target, gid int64) error {
	// Check if either email or phone is provided
	if t.Email == "" && t.Phone == "" {
		return ErrNoContactInfoSpecified
	}

	// If email is provided, validate it
	if t.Email != "" {
		if _, err := mail.ParseAddress(t.Email); err != nil {
			log.WithFields(logrus.Fields{
				"email": t.Email,
			}).Error("Invalid email")
			return err
		}
	}

	// If phone is provided, validate it (simple check for now)
	if t.Phone != "" {
		isValidPhone := false
		for _, c := range t.Phone {
			if !((c >= '0' && c <= '9') || c == '+' || c == '-' || c == '(' || c == ')' || c == ' ') {
				log.WithFields(logrus.Fields{
					"phone": t.Phone,
				}).Error("Invalid phone number")
				return errors.New("Invalid phone number format")
			}
			if c >= '0' && c <= '9' {
				isValidPhone = true
			}
		}
		if !isValidPhone {
			log.WithFields(logrus.Fields{
				"phone": t.Phone,
			}).Error("Phone number must contain at least one digit")
			return errors.New("Phone number must contain at least one digit")
		}
	}
	// Create a query that can find targets by either email or phone
	query := tx.Model(&Target{})

	// Build a query that can find targets by either email or phone or both
	if t.Email != "" && t.Phone != "" {
		// If both email and phone are provided, check for either match
		query = query.Where("email = ? OR phone = ?", t.Email, t.Phone)
		log.WithFields(logrus.Fields{
			"email": t.Email,
			"phone": t.Phone,
		}).Debug("Searching for target by both email and phone")
	} else if t.Email != "" {
		// If only email is provided
		query = query.Where("email = ?", t.Email)
		log.WithFields(logrus.Fields{
			"email": t.Email,
		}).Debug("Searching for target by email")
	} else if t.Phone != "" {
		// If only phone is provided
		query = query.Where("phone = ?", t.Phone)
		log.WithFields(logrus.Fields{
			"phone": t.Phone,
		}).Debug("Searching for target by phone")
	}

	// Try to find the target first
	var existingTarget Target
	err := query.First(&existingTarget).Error

	// If the target doesn't exist, create it
	if err == gorm.ErrRecordNotFound {
		err = tx.Create(&t).Error
		if err != nil {
			log.WithFields(logrus.Fields{
				"email": t.Email,
				"phone": t.Phone,
			}).Error(err)
			return err
		}
	} else if err != nil {
		log.WithFields(logrus.Fields{
			"email": t.Email,
			"phone": t.Phone,
		}).Error(err)
		return err
	} else {
		// Target exists. Reuse its ID, but refresh its details with the
		// incoming data so re-adding a target (e.g. after its group was
		// deleted and re-imported from an updated CSV) doesn't leave the
		// old, stale values in place.
		t.Id = existingTarget.Id
		if err := UpdateTarget(tx, t); err != nil {
			return err
		}
	}
	// Check if the group-target relationship already exists
	var existingRelation GroupTarget
	err = tx.Where("group_id = ? AND target_id = ?", gid, t.Id).First(&existingRelation).Error

	// If the relationship doesn't exist, create it
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"group_id":  gid,
			"target_id": t.Id,
			"email":     t.Email,
			"phone":     t.Phone,
		}).Debug("Adding new target to group")

		err = tx.Create(&GroupTarget{GroupId: gid, TargetId: t.Id}).Error
		if err != nil {
			log.WithFields(logrus.Fields{
				"group_id":  gid,
				"target_id": t.Id,
				"email":     t.Email,
				"phone":     t.Phone,
				"error":     err,
			}).Error("Error creating group-target relationship")
			return err
		}
	} else if err != nil {
		log.WithFields(logrus.Fields{
			"group_id":  gid,
			"target_id": t.Id,
			"email":     t.Email,
			"phone":     t.Phone,
			"error":     err,
		}).Error("Error checking for existing group-target relationship")
		return err
	} else {
		log.WithFields(logrus.Fields{
			"group_id":  gid,
			"target_id": t.Id,
			"email":     t.Email,
			"phone":     t.Phone,
		}).Debug("Target already in group, skipping")
	}
	return nil
}

// UpdateTarget updates the given target information in the database.
func UpdateTarget(tx *gorm.DB, target Target) error {
	targetInfo := map[string]interface{}{
		"email":      target.Email,
		"phone":      target.Phone,
		"first_name": target.FirstName,
		"last_name":  target.LastName,
		"position":   target.Position,
		"custom":     target.Custom,
	}
	err := tx.Model(&target).Where("id = ?", target.Id).Updates(targetInfo).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"email": target.Email,
			"phone": target.Phone,
		}).Error("Error updating target information")
	}
	return err
}

// GetTargets performs a many-to-many select to get all the Targets for a Group
func GetTargets(gid int64) ([]Target, error) {
	ts := []Target{}
	err := db.Table("targets").Select("targets.id, targets.email, targets.phone, targets.first_name, targets.last_name, targets.position, targets.custom").Joins("left join group_targets gt ON targets.id = gt.target_id").Where("gt.group_id=?", gid).Scan(&ts).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"group_id": gid,
			"error":    err,
		}).Error("Error getting targets for group")
	}
	return ts, err
}
