package models

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
)

// PeopleFilter narrows the campaigns that count toward per-person stats.
type PeopleFilter struct {
	From          time.Time // zero = unbounded; compared to campaigns.launch_date
	To            time.Time // zero = unbounded; inclusive
	CampaignSetId int64     // 0 = any
	GroupId       int64     // 0 = any
}

// PersonStats rolls up one person's results across every campaign they were
// sent. Each count is the number of campaigns in which they took the action.
type PersonStats struct {
	Key              string    `json:"key"` // lowercased email or phone
	Email            string    `json:"email"`
	Phone            string    `json:"phone"`
	FirstName        string    `json:"first_name"`
	LastName         string    `json:"last_name"`
	Position         string    `json:"position"`
	Groups           []string  `json:"groups"` // names of the user's groups the person is currently in
	Sent             int64     `json:"sent"`
	Opened           int64     `json:"opened"`
	Clicked          int64     `json:"clicked"`
	AttachmentOpened int64     `json:"attachment_opened"`
	MacroExecuted    int64     `json:"macro_executed"`
	SubmittedData    int64     `json:"submitted_data"`
	Replied          int64     `json:"replied"`
	Reported         int64     `json:"reported"`
	Failures         int64     `json:"failures"`
	LastFailure      time.Time `json:"last_failure"` // launch_date of latest failed campaign; zero if none
	PhishPronePct    float64   `json:"phish_prone_pct"`
	ReportRatePct    float64   `json:"report_rate_pct"`
}

// PersonCampaign is one campaign a person was sent, with the actions they took.
type PersonCampaign struct {
	CampaignId   int64           `json:"campaign_id"`
	CampaignName string          `json:"campaign_name"`
	LaunchDate   time.Time       `json:"launch_date"`
	Flags        map[string]bool `json:"flags"` // recipientFlags keys
	Failed       bool            `json:"failed"`
}

// ErrPeopleFilterNotFound is returned when a filter names a campaign set or
// group that doesn't exist or belongs to another user.
var ErrPeopleFilterNotFound = errors.New("campaign set or group not found")

type personResultRow struct {
	CampaignId int64
	Email      string
	Phone      string
	FirstName  string
	LastName   string
	Position   string
	Status     string
	Reported   bool
	Replied    bool
	SMSTarget  bool
}

type personEventRow struct {
	CampaignId int64
	Email      string
	Message    string
}

type personGroupRow struct {
	Id   int64
	Name string
}

type personGroupTargetRow struct {
	GroupId int64
	Email   string
	Phone   string
}

type personCampaignRow struct {
	Id         int64
	Name       string
	LaunchDate time.Time
}

// personKey returns the key a recipient is grouped by: their lowercased
// email, falling back to phone number for SMS recipients.
func personKey(email, phone string) string {
	if k := strings.ToLower(strings.TrimSpace(email)); k != "" {
		return k
	}
	return strings.TrimSpace(phone)
}

// personFailed reports whether a person fell for a campaign. Opening alone is
// not a failure.
func personFailed(f recipientFlags) bool {
	return f["clicked"] || f["submitted_data"] || f["attachment_opened"] ||
		f["macro_executed"] || f["replied"]
}

// peoplePct returns n/d as a 0-100 percentage rounded to one decimal, or 0
// when d is 0.
func peoplePct(n, d int64) float64 {
	if d == 0 {
		return 0
	}
	return math.Round(float64(n)*1000/float64(d)) / 10
}

// peopleCampaignScope returns a subquery selecting the ids of the user's
// campaigns that match the filter. It is passed as a subquery rather than an
// expanded id list to stay under SQLite's bound variable limit.
func peopleCampaignScope(uid int64, f PeopleFilter) *gorm.SqlExpr {
	q := db.Table("campaigns").Select("id").Where("user_id = ?", uid)
	if !f.From.IsZero() {
		q = q.Where("launch_date >= ?", f.From.UTC())
	}
	if !f.To.IsZero() {
		q = q.Where("launch_date <= ?", f.To.UTC())
	}
	if f.CampaignSetId != 0 {
		q = q.Where("campaign_set_id = ?", f.CampaignSetId)
	}
	return q.SubQuery()
}

// peopleGroupKeys returns the keys of the group's current members, or nil if
// the filter has no group. It also checks that any campaign set or group in
// the filter belongs to the user.
func peopleGroupKeys(uid int64, f PeopleFilter) (map[string]bool, error) {
	if f.CampaignSetId != 0 {
		var n int64
		err := db.Table("campaign_sets").Where("id = ? AND user_id = ?", f.CampaignSetId, uid).Count(&n).Error
		if err != nil {
			log.Error(err)
			return nil, err
		}
		if n == 0 {
			return nil, ErrPeopleFilterNotFound
		}
	}
	if f.GroupId == 0 {
		return nil, nil
	}
	g, err := GetGroup(f.GroupId, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPeopleFilterNotFound
		}
		return nil, err
	}
	keys := make(map[string]bool, len(g.Targets))
	for _, t := range g.Targets {
		if k := personKey(t.Email, t.Phone); k != "" {
			keys[k] = true
		}
	}
	return keys, nil
}

// peopleCurrentGroups returns, per person key, the sorted names of the user's
// groups that person is currently a member of.
//
// "groups" is a reserved word in MySQL 8, so the table is only ever named
// through db.Table, which quotes it for the dialect, never in raw SQL.
func peopleCurrentGroups(uid int64) (map[string][]string, error) {
	var groupRows []personGroupRow
	err := db.Table("groups").Select("id, name").Where("user_id = ?", uid).Scan(&groupRows).Error
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if len(groupRows) == 0 {
		return map[string][]string{}, nil
	}
	names := make(map[int64]string, len(groupRows))
	ids := make([]int64, 0, len(groupRows))
	for _, g := range groupRows {
		names[g.Id] = g.Name
		ids = append(ids, g.Id)
	}
	var rows []personGroupTargetRow
	err = db.Table("targets").
		Select("group_targets.group_id, targets.email, targets.phone").
		Joins("JOIN group_targets ON group_targets.target_id = targets.id").
		Where("group_targets.group_id IN (?)", ids).
		Scan(&rows).Error
	if err != nil {
		log.Error(err)
		return nil, err
	}
	seen := make(map[string]map[string]bool)
	for _, row := range rows {
		k := personKey(row.Email, row.Phone)
		name, ok := names[row.GroupId]
		if k == "" || !ok {
			continue
		}
		if seen[k] == nil {
			seen[k] = make(map[string]bool)
		}
		seen[k][name] = true
	}
	groups := make(map[string][]string, len(seen))
	for k, names := range seen {
		list := make([]string, 0, len(names))
		for name := range names {
			list = append(list, name)
		}
		sort.Strings(list)
		groups[k] = list
	}
	return groups, nil
}

// loadPersonCampaigns returns, per person key, the campaigns they were sent
// (with flags derived the same way as buildRecipientStats), and the recipient
// details from their most recent campaign. An empty key loads everyone;
// otherwise only that person is loaded.
func loadPersonCampaigns(uid int64, f PeopleFilter, key string) (map[string][]PersonCampaign, map[string]BaseRecipient, error) {
	groupKeys, err := peopleGroupKeys(uid, f)
	if err != nil {
		return nil, nil, err
	}
	keep := func(k string) bool {
		if k == "" || (key != "" && k != key) {
			return false
		}
		return groupKeys == nil || groupKeys[k]
	}

	var campaignRows []personCampaignRow
	err = db.Table("campaigns").Select("id, name, launch_date").
		Where("id IN ?", peopleCampaignScope(uid, f)).Scan(&campaignRows).Error
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	campaigns := make(map[int64]personCampaignRow, len(campaignRows))
	for _, c := range campaignRows {
		campaigns[c.Id] = c
	}

	resultQuery := db.Table("results").
		Select("campaign_id, email, phone, first_name, last_name, position, status, reported, replied, sms_target").
		Where("campaign_id IN ?", peopleCampaignScope(uid, f))
	if key != "" {
		resultQuery = resultQuery.Where("LOWER(TRIM(email)) = ? OR phone = ?", key, key)
	}
	var results []personResultRow
	if err = resultQuery.Scan(&results).Error; err != nil {
		log.Error(err)
		return nil, nil, err
	}

	// SMS events are recorded against the phone number, while a target with
	// both an email and a phone is keyed by email. alias[campaign id][phone]
	// maps such phones back to the person's key.
	alias := make(map[int64]map[string]string)
	eventKeys := []string{key}
	for _, r := range results {
		phone := strings.TrimSpace(r.Phone)
		k := personKey(r.Email, r.Phone)
		if !r.SMSTarget || phone == "" || phone == k {
			continue
		}
		if alias[r.CampaignId] == nil {
			alias[r.CampaignId] = make(map[string]string)
		}
		alias[r.CampaignId][phone] = k
		if key != "" && k == key {
			eventKeys = append(eventKeys, phone)
		}
	}

	eventQuery := db.Table("events").
		Select("campaign_id, LOWER(email) AS email, message").
		Where("campaign_id IN ?", peopleCampaignScope(uid, f)).
		Where("email <> ''").
		Where("message IN (?)", trackedEventMessages())
	if key != "" {
		eventQuery = eventQuery.Where("LOWER(email) IN (?)", eventKeys)
	}
	var events []personEventRow
	err = eventQuery.Group("campaign_id, LOWER(email), message").Scan(&events).Error
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}

	// flags[key][campaign id]
	flags := make(map[string]map[int64]recipientFlags)
	flagsFor := func(k string, cid int64) recipientFlags {
		if flags[k] == nil {
			flags[k] = make(map[int64]recipientFlags)
		}
		if flags[k][cid] == nil {
			flags[k][cid] = newRecipientFlags()
		}
		return flags[k][cid]
	}

	recipients := make(map[string]BaseRecipient)
	latest := make(map[string]time.Time)
	for _, r := range results {
		k := personKey(r.Email, r.Phone)
		if !keep(k) {
			continue
		}
		fl := flagsFor(k, r.CampaignId)
		applyResultFlags(fl, r.Reported, r.Replied, r.Status)
		launch := campaigns[r.CampaignId].LaunchDate
		if prev, ok := latest[k]; !ok || launch.After(prev) {
			latest[k] = launch
			recipients[k] = BaseRecipient{
				Email:     strings.TrimSpace(r.Email),
				Phone:     r.Phone,
				FirstName: r.FirstName,
				LastName:  r.LastName,
				Position:  r.Position,
			}
		}
	}

	for _, e := range events {
		k := strings.TrimSpace(e.Email)
		if a, ok := alias[e.CampaignId][k]; ok {
			k = a
		}
		if !keep(k) {
			continue
		}
		fl := flagsFor(k, e.CampaignId)
		applyEventFlag(fl, e.Message)
	}

	people := make(map[string][]PersonCampaign)
	for k, byCampaign := range flags {
		for cid, fl := range byCampaign {
			// Only campaigns actually sent to the person count.
			if !fl["sent"] {
				continue
			}
			applyRecipientImplications(fl)
			c := campaigns[cid]
			people[k] = append(people[k], PersonCampaign{
				CampaignId:   cid,
				CampaignName: c.Name,
				LaunchDate:   c.LaunchDate,
				Flags:        fl,
				Failed:       personFailed(fl),
			})
		}
		if _, ok := people[k]; ok {
			if _, ok := recipients[k]; !ok {
				// Events with no matching result row (e.g. deleted results).
				if strings.Contains(k, "@") {
					recipients[k] = BaseRecipient{Email: k}
				} else {
					recipients[k] = BaseRecipient{Phone: k}
				}
			}
		}
	}
	for k := range recipients {
		if _, ok := people[k]; !ok {
			delete(recipients, k)
		}
	}
	return people, recipients, nil
}

// GetPeopleStats returns per-person stats across the user's campaigns that
// match the filter, sorted by failures (most first), then key. People who
// were never sent a matching campaign are omitted.
func GetPeopleStats(uid int64, f PeopleFilter) ([]PersonStats, error) {
	people, recipients, err := loadPersonCampaigns(uid, f, "")
	if err != nil {
		return nil, err
	}
	groups, err := peopleCurrentGroups(uid)
	if err != nil {
		return nil, err
	}
	stats := make([]PersonStats, 0, len(people))
	for k, campaigns := range people {
		r := recipients[k]
		ps := PersonStats{
			Key:       k,
			Email:     r.Email,
			Phone:     r.Phone,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Position:  r.Position,
			Groups:    groups[k],
		}
		if ps.Groups == nil {
			ps.Groups = []string{}
		}
		for _, pc := range campaigns {
			ps.Sent++
			count := func(flag string, n *int64) {
				if pc.Flags[flag] {
					*n++
				}
			}
			count("opened", &ps.Opened)
			count("clicked", &ps.Clicked)
			count("attachment_opened", &ps.AttachmentOpened)
			count("macro_executed", &ps.MacroExecuted)
			count("submitted_data", &ps.SubmittedData)
			count("replied", &ps.Replied)
			count("reported", &ps.Reported)
			if pc.Failed {
				ps.Failures++
				if pc.LaunchDate.After(ps.LastFailure) {
					ps.LastFailure = pc.LaunchDate
				}
			}
		}
		ps.PhishPronePct = peoplePct(ps.Failures, ps.Sent)
		ps.ReportRatePct = peoplePct(ps.Reported, ps.Sent)
		stats = append(stats, ps)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Failures != stats[j].Failures {
			return stats[i].Failures > stats[j].Failures
		}
		return stats[i].Key < stats[j].Key
	})
	return stats, nil
}

// GetPersonTimeline returns the matching campaigns sent to the person with
// the given key (email or phone, case-insensitive), newest first.
func GetPersonTimeline(uid int64, key string, f PeopleFilter) ([]PersonCampaign, error) {
	key = personKey(key, key)
	timeline := []PersonCampaign{}
	if key == "" {
		return timeline, nil
	}
	people, _, err := loadPersonCampaigns(uid, f, key)
	if err != nil {
		return nil, err
	}
	timeline = append(timeline, people[key]...)
	sort.Slice(timeline, func(i, j int) bool {
		if !timeline[i].LaunchDate.Equal(timeline[j].LaunchDate) {
			return timeline[i].LaunchDate.After(timeline[j].LaunchDate)
		}
		return timeline[i].CampaignId > timeline[j].CampaignId
	})
	return timeline, nil
}
