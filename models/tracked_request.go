package models

import (
	"time"

	log "github.com/gophish/gophish/logger"
)

// TrackedRequest is a request received on the phishing server that carried the
// X-Tracked header but could not be attributed to a campaign result (it had no
// valid recipient id). These are macro/POST callbacks and similar out-of-band
// hits that are logged for the operator to inspect in the Monitor.
//
// This replaces the legacy tracked-data.csv file. With no recipient id there is
// no user/campaign to scope to, so the table is global and visible to any admin.
type TrackedRequest struct {
	Id        int64     `json:"id" gorm:"primary_key:yes"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	SourceIP  string    `json:"source_ip" gorm:"column:source_ip"`
	Method    string    `json:"method" gorm:"column:method"`
	Path      string    `json:"path" gorm:"column:path"`
	RawQuery  string    `json:"raw_query" gorm:"column:raw_query"`
	UserAgent string    `json:"user_agent" gorm:"column:user_agent"`
}

// TableName specifies the database tablename for Gorm to use.
func (t TrackedRequest) TableName() string {
	return "tracked_requests"
}

// RecordTrackedRequest stores a single X-Tracked request.
func RecordTrackedRequest(method, path, rawQuery, sourceIP, userAgent string) error {
	tr := &TrackedRequest{
		CreatedAt: time.Now().UTC(),
		SourceIP:  sourceIP,
		Method:    method,
		Path:      path,
		RawQuery:  rawQuery,
		UserAgent: userAgent,
	}
	if err := db.Create(tr).Error; err != nil {
		log.Errorf("Failed to record tracked request: %v", err)
		return err
	}
	return nil
}

// GetTrackedRequests returns up to limit tracked requests, most recent first.
func GetTrackedRequests(limit int) ([]TrackedRequest, error) {
	reqs := []TrackedRequest{}
	query := db.Order("created_at desc, id desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&reqs).Error
	if err != nil {
		log.Error(err)
	}
	return reqs, err
}

// DeleteTrackedRequestsByIds deletes the tracked requests with the given ids.
func DeleteTrackedRequestsByIds(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	err := db.Where("id IN (?)", ids).Delete(&TrackedRequest{}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// ClearTrackedRequests deletes every tracked request.
func ClearTrackedRequests() error {
	err := db.Delete(&TrackedRequest{}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}
