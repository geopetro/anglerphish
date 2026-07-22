package models

import (
	"errors"
	"math/rand"
	"net/url"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Campaign is a struct representing a created campaign
type Campaign struct {
	Id                 int64       `json:"id"`
	UserId             int64       `json:"-"`
	Name               string      `json:"name" sql:"not null"`
	CreatedDate        time.Time   `json:"created_date"`
	LaunchDate         time.Time   `json:"launch_date"`
	SendByDate         time.Time   `json:"send_by_date"`
	CompletedDate      time.Time   `json:"completed_date"`
	Type               string      `json:"type" sql:"default:'email'"`
	TemplateId         int64       `json:"-"`
	Template           Template    `json:"template"`
	SMSTemplateId      int64       `json:"-"`
	SMSTemplate        SMSTemplate `json:"sms_template,omitempty"`
	PageId             int64       `json:"-"`
	Page               Page        `json:"page"`
	Status             string      `json:"status"`
	Results            []Result    `json:"results,omitempty"`
	Groups             []Group     `json:"groups,omitempty"`
	Events             []Event     `json:"timeline,omitempty"`
	SMTPId             int64       `json:"-"`
	SMTP               SMTP        `json:"smtp"`
	SMSId              int64       `json:"-"`
	SMS                SMS         `json:"sms,omitempty"`
	URL                string      `json:"url"`
	URLParam           string      `json:"urlparam" sql:"column:url_param"`
	QRSize             string      `json:"qrsize" sql:"column:qr_size"`
	HTTPAuth           bool        `json:"basicauth" sql:"column:http_auth"`
	CampaignSetId      int64       `json:"campaign_set_id,omitempty"`
	RandomizeSendOrder bool        `json:"randomize_send_order" sql:"default:false"`
}

// CampaignResults is a struct representing the results from a campaign
type CampaignResults struct {
	Id       int64    `json:"id"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Type     string   `json:"type"`
	URL      string   `json:"url"`
	URLParam string   `json:"urlparam"`
	Results  []Result `json:"results,omitempty"`
	Events   []Event  `json:"timeline,omitempty"`
}

// CampaignSummaries is a struct representing the overview of campaigns
type CampaignSummaries struct {
	Total     int64             `json:"total"`
	Campaigns []CampaignSummary `json:"campaigns"`
}

// CampaignSummary is a struct representing the overview of a single camaign
type CampaignSummary struct {
	Id            int64         `json:"id"`
	CreatedDate   time.Time     `json:"created_date"`
	LaunchDate    time.Time     `json:"launch_date"`
	SendByDate    time.Time     `json:"send_by_date"`
	CompletedDate time.Time     `json:"completed_date"`
	Status        string        `json:"status"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Stats         CampaignStats `json:"stats"`
}

// CampaignStats is a struct representing the statistics for a single campaign
type CampaignStats struct {
	Total         int64 `json:"total"`
	EmailsSent    int64 `json:"sent"`
	OpenedEmail   int64 `json:"opened"`
	ClickedLink   int64 `json:"clicked"`
	SubmittedData int64 `json:"submitted_data"`
	Replied       int64 `json:"replied"`
	EmailReported int64 `json:"email_reported"`
	Error         int64 `json:"error"`
}

// Event contains the fields for an event
// that occurs during the campaign
type Event struct {
	Id         int64     `json:"-"`
	CampaignId int64     `json:"campaign_id"`
	Email      string    `json:"email"`
	Time       time.Time `json:"time"`
	Message    string    `json:"message"`
	Details    string    `json:"details"`
}

// BeforeSave encrypts sensitive Event fields before saving to database
func (e *Event) BeforeSave() error {
	// Only encrypt Details if it's not empty and not already encrypted
	if e.Details != "" && !isFieldEncrypted(e.Details) {
		encrypted, err := encryptField(e.Details)
		if err != nil {
			// Log but don't fail - encryption is optional
			log.Warnf("Failed to encrypt event details: %v", err)
		} else {
			e.Details = encrypted
		}
	}
	return nil
}

// AfterFind decrypts sensitive Event fields after loading from database
func (e *Event) AfterFind() error {
	// Only decrypt Details if it's encrypted
	if e.Details != "" && isFieldEncrypted(e.Details) {
		decrypted, err := decryptField(e.Details)
		if err != nil {
			// Log but don't fail - return encrypted data
			log.Warnf("Failed to decrypt event details: %v", err)
		} else {
			e.Details = decrypted
		}
	}
	return nil
}

// EventDetails is a struct that wraps common attributes we want to store
// in an event
type EventDetails struct {
	Payload url.Values        `json:"payload"`
	Browser map[string]string `json:"browser"`
}

// EventError is a struct that wraps an error that occurs when sending an
// email to a recipient
type EventError struct {
	Error string `json:"error"`
}

// ErrCampaignNameNotSpecified indicates there was no template given by the user
var ErrCampaignNameNotSpecified = errors.New("Campaign name not specified")

// ErrGroupNotSpecified indicates there was no template given by the user
var ErrGroupNotSpecified = errors.New("No groups specified")

// ErrTemplateNotSpecified indicates there was no template given by the user
var ErrTemplateNotSpecified = errors.New("No email template specified")

// ErrPageNotSpecified indicates a landing page was not provided for the campaign
var ErrPageNotSpecified = errors.New("No landing page specified")

// ErrSMTPNotSpecified indicates a sending profile was not provided for the campaign
var ErrSMTPNotSpecified = errors.New("No sending profile specified")

// ErrSMSNotSpecified indicates an SMS sending profile was not provided for the campaign
var ErrSMSNotSpecified = errors.New("No SMS sending profile specified")

// ErrSMSTemplateNotSpecified indicates there was no SMS template given by the user
var ErrSMSTemplateNotSpecified = errors.New("No SMS template specified")

// ErrTemplateNotFound indicates the template specified does not exist in the database
var ErrTemplateNotFound = errors.New("Template not found")

// ErrSMSTemplateNotFound indicates the SMS template specified does not exist in the database
var ErrSMSTemplateNotFound = errors.New("SMS template not found")

// ErrGroupNotFound indicates a group specified by the user does not exist in the database
var ErrGroupNotFound = errors.New("Group not found")

// ErrPageNotFound indicates a page specified by the user does not exist in the database
var ErrPageNotFound = errors.New("Page not found")

// ErrSMTPNotFound indicates a sending profile specified by the user does not exist in the database
var ErrSMTPNotFound = errors.New("Sending profile not found")

// ErrSMSNotFound indicates an SMS sending profile specified by the user does not exist in the database
var ErrSMSNotFound = errors.New("SMS sending profile not found")

// ErrInvalidSendByDate indicates that the user specified a send by date that occurs before the
// launch date
var ErrInvalidSendByDate = errors.New("The launch date must be before the \"send emails by\" date")

// RecipientParameter is the URL parameter that points to the result ID for a recipient.
var RecipientParameter = "rid"

// ErrInvalidTargetType indicates that the campaign contains targets that don't match the campaign type
var ErrInvalidTargetType = errors.New("Campaign contains targets that don't match the campaign type")

// ErrCampaignNotGeneric indicates that an operation specific to generic campaigns was attempted on a non-generic campaign
var ErrCampaignNotGeneric = errors.New("Campaign is not a generic campaign")

// ErrCampaignCompleted indicates that the campaign has already been completed
var ErrCampaignCompleted = errors.New("Campaign has already been completed")

// Validate checks to make sure there are no invalid fields in a submitted campaign
func (c *Campaign) Validate() error {
	// Common validation for all campaign types
	switch {
	case c.Name == "":
		return ErrCampaignNameNotSpecified
	case !c.SendByDate.IsZero() && !c.LaunchDate.IsZero() && c.SendByDate.Before(c.LaunchDate):
		return ErrInvalidSendByDate
	}

	// Type-specific validation
	switch c.Type {
	case "generic":
		// Generic campaigns don't require groups
		if err := c.validateGeneric(); err != nil {
			return err
		}
	case "sms":
		// SMS campaigns require groups
		if len(c.Groups) == 0 {
			return ErrGroupNotSpecified
		}
		if err := c.validateSMS(); err != nil {
			return err
		}
	default: // Default to email validation
		// Email campaigns require groups
		if len(c.Groups) == 0 {
			return ErrGroupNotSpecified
		}
		if err := c.validateEmail(); err != nil {
			return err
		}
	}

	// Validate that targets match campaign type (only for email/sms campaigns)
	if c.Type != "generic" {
		for _, g := range c.Groups {
			for _, t := range g.Targets {
				if c.Type == "sms" {
					// For SMS campaigns, targets should have a phone number
					if t.Phone == "" {
						return ErrInvalidTargetType
					}
				} else {
					// For email campaigns, targets should have an email address
					if t.Email == "" {
						return ErrInvalidTargetType
					}
				}
			}
		}
	}

	return nil
}

// validateEmail validates an email campaign
func (c *Campaign) validateEmail() error {
	switch {
	case c.Template.Name == "":
		return ErrTemplateNotSpecified
	case c.Page.Name == "":
		return ErrPageNotSpecified
	case c.SMTP.Name == "":
		return ErrSMTPNotSpecified
	}
	return nil
}

// validateSMS validates an SMS campaign
func (c *Campaign) validateSMS() error {
	switch {
	case c.SMSTemplate.Name == "":
		return ErrSMSTemplateNotSpecified
	case c.Page.Name == "":
		return ErrPageNotSpecified
	case c.SMS.Name == "":
		return ErrSMSNotSpecified
	}
	return nil
}

// validateGeneric validates a generic campaign (landing page only, no email/SMS)
func (c *Campaign) validateGeneric() error {
	if c.Page.Name == "" {
		return ErrPageNotSpecified
	}
	return nil
}

// GetActiveCampaigns returns all campaigns that are either in progress or queued.
// This is used by the IMAP monitor to check for custom URL parameters.
func GetActiveCampaigns() ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Where("status IN (?)", []string{CampaignInProgress}).Find(&cs).Error
	if err != nil {
		log.Error(err)
	}
	return cs, err
}

// UpdateStatus changes the campaign status appropriately
func (c *Campaign) UpdateStatus(s string) error {
	// This could be made simpler, but I think there's a bug in gorm
	return db.Table("campaigns").Where("id=?", c.Id).Update("status", s).Error
}

// AddEvent creates a new campaign event in the database
func AddEvent(e *Event, campaignID int64) error {
	e.CampaignId = campaignID
	e.Time = time.Now().UTC()

	whs, err := GetActiveWebhooks()
	if err == nil {
		whEndPoints := []webhook.EndPoint{}
		for _, wh := range whs {
			whEndPoints = append(whEndPoints, webhook.EndPoint{
				URL:    wh.URL,
				Secret: wh.Secret,
			})
		}
		webhook.SendAll(whEndPoints, e)
	} else {
		log.Errorf("error getting active webhooks: %v", err)
	}

	return db.Save(e).Error
}

// getDetails retrieves the related attributes of the campaign
// from the database. If the Events and the Results are not available,
// an error is returned. Otherwise, the attribute name is set to [Deleted],
// indicating the user deleted the attribute (template, smtp, etc.)
func (c *Campaign) getDetails() error {
	err := db.Model(c).Related(&c.Results).Error
	if err != nil {
		log.Warnf("%s: results not found for campaign", err)
		return err
	}
	err = db.Model(c).Related(&c.Events).Error
	if err != nil {
		log.Warnf("%s: events not found for campaign", err)
		return err
	}

	// Generic campaigns don't have templates - skip template lookup
	if c.Type != "generic" {
		err = db.Table("templates").Where("id=?", c.TemplateId).Find(&c.Template).Error
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			c.Template = Template{Name: "[Deleted]"}
			log.Warnf("%s: template not found for campaign", err)
		}
		err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			log.Warn(err)
			return err
		}
	}

	err = db.Table("pages").Where("id=?", c.PageId).Find(&c.Page).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.Page = Page{Name: "[Deleted]"}
		log.Warnf("%s: page not found for campaign", err)
	}

	// Look for sending profiles based on campaign type
	switch c.Type {
	case "sms":
		// For SMS campaigns, look for SMS profile
		err = db.Table("sms_profiles").Where("id=?", c.SMSId).Find(&c.SMS).Error
		if err != nil {
			// Check if the SMS profile was deleted
			if err != gorm.ErrRecordNotFound {
				return err
			}
			c.SMS = SMS{Name: "[Deleted]"}
			log.Warnf("%s: SMS sending profile not found for campaign", err)
		}
	case "generic":
		// Generic campaigns don't have sending profiles - skip
	default:
		// For email campaigns, look for SMTP profile
		err = db.Table("smtp").Where("id=?", c.SMTPId).Find(&c.SMTP).Error
		if err != nil {
			// Check if the SMTP was deleted
			if err != gorm.ErrRecordNotFound {
				return err
			}
			c.SMTP = SMTP{Name: "[Deleted]"}
			log.Warnf("%s: sending profile not found for campaign", err)
		}
		err = db.Where("smtp_id=?", c.SMTP.Id).Find(&c.SMTP.Headers).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			log.Warn(err)
			return err
		}
	}
	return nil
}

// getCampaignSummary loads only id+name for template/page/smtp without fetching
// Results or Events. Used when displaying campaigns inside a campaign set view.
func (c *Campaign) getCampaignSummary() error {
	if c.Type != "generic" && c.TemplateId != 0 {
		if err := db.Table("templates").Select("id, name").Where("id = ?", c.TemplateId).First(&c.Template).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	if c.SMSTemplateId != 0 {
		db.Table("sms_templates").Select("id, name").Where("id = ?", c.SMSTemplateId).First(&c.SMSTemplate)
	}
	if c.PageId != 0 {
		if err := db.Table("pages").Select("id, name").Where("id = ?", c.PageId).First(&c.Page).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	switch c.Type {
	case "sms":
		if c.SMSId != 0 {
			db.Table("sms_profiles").Select("id, name").Where("id = ?", c.SMSId).First(&c.SMS)
		}
	case "generic":
		// no sending profile
	default:
		if c.SMTPId != 0 {
			db.Table("smtp").Select("id, name").Where("id = ?", c.SMTPId).First(&c.SMTP)
		}
	}
	return nil
}

// getBaseURL returns the Campaign's configured URL.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getBaseURL() string {
	return c.URL
}

// getFromAddress returns the Campaign's configured SMTP "From" address.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getFromAddress() string {
	return c.SMTP.FromAddress
}

// getQRSize returns the Campaign's configured SMTP "From" address.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getQRSize() string {
	return c.QRSize
}

// generateSendDate creates a sendDate
func (c *Campaign) generateSendDate(idx int, totalRecipients int) time.Time {
	// If no send date is specified, just return the launch date
	if c.SendByDate.IsZero() || c.SendByDate.Equal(c.LaunchDate) {
		return c.LaunchDate
	}
	// Otherwise, we can calculate the range of minutes to send emails
	// (since we only poll once per minute)
	totalMinutes := c.SendByDate.Sub(c.LaunchDate).Minutes()

	// Next, we can determine how many minutes should elapse between emails
	minutesPerEmail := totalMinutes / float64(totalRecipients)

	// Then, we can calculate the offset for this particular email
	offset := int(minutesPerEmail * float64(idx))

	// Finally, we can just add this offset to the launch date to determine
	// when the email should be sent
	return c.LaunchDate.Add(time.Duration(offset) * time.Minute)
}

// getCampaignStats returns a CampaignStats object for the campaign with the given campaign ID.
// It also backfills numbers as appropriate with a running total, so that the values are aggregated.
func getCampaignStats(cid int64) (CampaignStats, error) {
	s := CampaignStats{}

	// Get all results for this campaign
	var results []Result
	err := db.Where("campaign_id = ?", cid).Find(&results).Error
	if err != nil {
		return s, err
	}

	s.Total = int64(len(results))

	// Get all events for this campaign to reconstruct complete timeline
	var events []Event
	err = db.Where("campaign_id = ?", cid).Order("time ASC").Find(&events).Error
	if err != nil {
		return s, err
	}

	// Track each recipient's actions
	recipientStats := make(map[string]map[string]bool)

	// Initialize recipient tracking from results
	for _, result := range results {
		email := result.Email
		if email == "" {
			email = result.Phone // For SMS campaigns
		}

		if recipientStats[email] == nil {
			recipientStats[email] = map[string]bool{
				"sent":           false,
				"opened":         false,
				"clicked":        false,
				"submitted_data": false,
				"replied":        false,
				"reported":       false,
				"error":          false,
			}
		}

		// Handle flags from results table
		if result.Reported {
			recipientStats[email]["reported"] = true
		}
		if result.Replied {
			recipientStats[email]["replied"] = true
		}
	}

	// Process events to get complete action history
	for _, event := range events {
		email := event.Email
		if recipientStats[email] == nil {
			recipientStats[email] = map[string]bool{
				"sent":           false,
				"opened":         false,
				"clicked":        false,
				"submitted_data": false,
				"replied":        false,
				"reported":       false,
				"error":          false,
			}
		}

		// Map events to actions
		switch event.Message {
		case EventSent, EventSMSSent:
			recipientStats[email]["sent"] = true
		case EventOpened:
			recipientStats[email]["opened"] = true
		case EventClicked:
			recipientStats[email]["clicked"] = true
		case EventDataSubmit:
			recipientStats[email]["submitted_data"] = true
		case EventReplied:
			recipientStats[email]["replied"] = true
		case EventReported:
			recipientStats[email]["reported"] = true
		case EventSendingError, EventSMSError, Error:
			recipientStats[email]["error"] = true
		}
	}

	// Apply logical backfilling and count totals
	for _, stats := range recipientStats {
		// Apply logical implications
		if stats["submitted_data"] {
			stats["clicked"] = true
			stats["opened"] = true
		}
		if stats["clicked"] {
			stats["opened"] = true
		}
		if stats["replied"] {
			stats["opened"] = true
		}
		// Note: "reported" remains standalone - no implications

		// Count totals
		if stats["sent"] {
			s.EmailsSent++
		}
		if stats["opened"] {
			s.OpenedEmail++
		}
		if stats["clicked"] {
			s.ClickedLink++
		}
		if stats["submitted_data"] {
			s.SubmittedData++
		}
		if stats["replied"] {
			s.Replied++
		}
		if stats["reported"] {
			s.EmailReported++
		}
		if stats["error"] {
			s.Error++
		}
	}

	return s, nil
}

// GetCampaigns returns the campaigns owned by the given user.
func GetCampaigns(uid int64) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Model(&User{Id: uid}).Related(&cs).Error
	if err != nil {
		log.Error(err)
	}
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// GetCampaignSummaries gets the summary objects for all the campaigns
// owned by the current user
func GetCampaignSummaries(uid int64) (CampaignSummaries, error) {
	overview := CampaignSummaries{}
	cs := []CampaignSummary{}
	// Get the basic campaign information
	query := db.Table("campaigns").Where("user_id = ?", uid)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status, type")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return overview, err
	}
	for i := range cs {
		s, err := getCampaignStats(cs[i].Id)
		if err != nil {
			log.Error(err)
			return overview, err
		}
		cs[i].Stats = s
	}
	overview.Total = int64(len(cs))
	overview.Campaigns = cs
	return overview, nil
}

// GetCampaignSummary gets the summary object for a campaign specified by the campaign ID
func GetCampaignSummary(id int64, uid int64) (CampaignSummary, error) {
	cs := CampaignSummary{}
	query := db.Table("campaigns").Where("user_id = ? AND id = ?", uid, id)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status, type")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return cs, err
	}
	s, err := getCampaignStats(cs.Id)
	if err != nil {
		log.Error(err)
		return cs, err
	}
	cs.Stats = s
	return cs, nil
}

// GetCampaignMailContext returns a campaign object with just the relevant
// data needed to generate and send emails. This includes the top-level
// metadata, the template, and the sending profile.
//
// This should only ever be used if you specifically want this lightweight
// context, since it returns a non-standard campaign object.
// ref: #1726
func GetCampaignMailContext(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&c).Error
	if err != nil {
		return c, err
	}

	// Verify this is an email campaign
	if c.Type == "sms" {
		return c, errors.New("attempted to get email context for an SMS campaign")
	}

	err = db.Table("smtp").Where("id=?", c.SMTPId).Find(&c.SMTP).Error
	if err != nil {
		return c, err
	}
	err = db.Where("smtp_id=?", c.SMTP.Id).Find(&c.SMTP.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return c, err
	}
	err = db.Table("templates").Where("id=?", c.TemplateId).Find(&c.Template).Error
	if err != nil {
		return c, err
	}
	err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return c, err
	}
	return c, nil
}

// GetCampaignSMSContext returns a campaign object with just the relevant
// data needed to generate and send SMS messages. This includes the top-level
// metadata, the SMS template, and the SMS sending profile.
//
// This should only ever be used if you specifically want this lightweight
// context, since it returns a non-standard campaign object.
func GetCampaignSMSContext(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&c).Error
	if err != nil {
		return c, err
	}

	// Verify this is an SMS campaign
	if c.Type != "sms" {
		return c, errors.New("attempted to get SMS context for a non-SMS campaign")
	}

	err = db.Table("sms_profiles").Where("id=?", c.SMSId).Find(&c.SMS).Error
	if err != nil {
		return c, err
	}
	err = db.Table("sms_templates").Where("id=?", c.SMSTemplateId).Find(&c.SMSTemplate).Error
	if err != nil {
		return c, err
	}
	return c, nil
}

// GetCampaign returns the campaign, if it exists, specified by the given id and user_id.
func GetCampaign(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&c).Error
	if err != nil {
		log.Errorf("%s: campaign not found", err)
		return c, err
	}
	err = c.getDetails()
	return c, err
}

// GetCampaignResults returns just the campaign results for the given campaign
func GetCampaignResults(id int64, uid int64) (CampaignResults, error) {
	cr := CampaignResults{}
	err := db.Table("campaigns").Where("id=? and user_id=?", id, uid).Find(&cr).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"campaign_id": id,
			"error":       err,
		}).Error(err)
		return cr, err
	}
	err = db.Table("results").Where("campaign_id=? and user_id=?", cr.Id, uid).Find(&cr.Results).Error
	if err != nil {
		log.Errorf("%s: results not found for campaign", err)
		return cr, err
	}
	err = db.Table("events").Where("campaign_id=?", cr.Id).Find(&cr.Events).Error
	if err != nil {
		log.Errorf("%s: events not found for campaign", err)
		return cr, err
	}
	return cr, err
}

// GetQueuedCampaigns returns the campaigns that are queued up for this given minute
func GetQueuedCampaigns(t time.Time) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Where("launch_date <= ?", t).
		Where("status = ?", CampaignQueued).Find(&cs).Error
	if err != nil {
		log.Error(err)
	}
	log.Infof("Found %d Campaigns to run\n", len(cs))
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// PostCampaign inserts a campaign and all associated records into the database.
func PostCampaign(c *Campaign, uid int64) error {
	var err error

	// Set the campaign type if not specified
	if c.Type == "" {
		c.Type = "email"
	}

	// Set the custom parameter provided for the URL
	if c.URLParam == "" {
		RecipientParameter = "rid" // Default to "rid" if empty
		log.Infof("Using default URL parameter 'rid' for campaign %s", c.Name)
	} else {
		RecipientParameter = c.URLParam
	}

	// Fill in the details
	c.UserId = uid
	c.CreatedDate = time.Now().UTC()
	c.CompletedDate = time.Time{}
	c.Status = CampaignQueued
	if c.LaunchDate.IsZero() {
		c.LaunchDate = c.CreatedDate
	} else {
		c.LaunchDate = c.LaunchDate.UTC()
	}
	if !c.SendByDate.IsZero() {
		c.SendByDate = c.SendByDate.UTC()
	}
	if c.LaunchDate.Before(c.CreatedDate) || c.LaunchDate.Equal(c.CreatedDate) {
		c.Status = CampaignInProgress
	}

	// Handle generic campaigns separately - they don't need groups
	if c.Type == "generic" {
		return postGenericCampaign(c, uid)
	}

	// Check to make sure all the groups already exist
	// Also, later we'll need to know the total number of recipients (counting
	// duplicates is ok for now), so we'll do that here to save a loop.
	totalRecipients := 0
	for i, g := range c.Groups {
		c.Groups[i], err = GetGroupByName(g.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"group": g.Name,
			}).Error("Group does not exist")
			return ErrGroupNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		totalRecipients += len(c.Groups[i].Targets)
	}

	// Now that we have loaded all the groups with their targets,
	// validate the campaign to ensure targets match the campaign type
	err = c.Validate()
	if err != nil {
		return err
	}

	// Check to make sure the page exists
	p, err := GetPageByName(c.Page.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"page": c.Page.Name,
		}).Error("Page does not exist")
		return ErrPageNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Page = p
	c.PageId = p.Id

	// Type-specific setup
	switch c.Type {
	case "sms":
		// Check to make sure the SMS template exists
		st, err := GetSMSTemplateByName(c.SMSTemplate.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"sms_template": c.SMSTemplate.Name,
			}).Error("SMS template does not exist")
			return ErrSMSTemplateNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.SMSTemplate = st
		c.SMSTemplateId = st.Id

		// Check to make sure the SMS sending profile exists
		s, err := GetSMSByName(c.SMS.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"sms": c.SMS.Name,
			}).Error("SMS sending profile does not exist")
			return ErrSMSNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.SMS = s
		c.SMSId = s.Id
	default: // Default to email campaign setup
		// Check to make sure the template exists
		t, err := GetTemplateByName(c.Template.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"template": c.Template.Name,
			}).Error("Template does not exist")
			return ErrTemplateNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.Template = t
		c.TemplateId = t.Id

		// Check to make sure the sending profile exists
		s, err := GetSMTPByName(c.SMTP.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"smtp": c.SMTP.Name,
			}).Error("Sending profile does not exist")
			return ErrSMTPNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		c.SMTP = s
		c.SMTPId = s.Id
	}

	// Insert into the DB
	err = db.Save(c).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = AddEvent(&Event{Message: "Campaign Created"}, c.Id)
	if err != nil {
		log.Error(err)
	}

	// Collect all the unique targets across the campaign's groups. We
	// deduplicate here so that the recipient index (and therefore the
	// send date) is only assigned once per unique recipient.
	resultMap := make(map[string]bool)
	targets := []Target{}
	for _, g := range c.Groups {
		for _, t := range g.Targets {
			// Remove duplicate results - we should only
			// send messages to unique addresses.
			// For SMS campaigns, use phone number as the deduplication key
			// For email campaigns, use email address
			dedupKey := t.Email
			if c.Type == "sms" {
				dedupKey = t.Phone
			}

			if _, ok := resultMap[dedupKey]; ok {
				continue
			}
			resultMap[dedupKey] = true
			targets = append(targets, t)
		}
	}

	// If requested, randomize the order in which recipients are sent
	// messages, rather than the default order derived from the groups.
	if c.RandomizeSendOrder {
		rand.Shuffle(len(targets), func(i, j int) {
			targets[i], targets[j] = targets[j], targets[i]
		})
	}

	// Insert all the results
	recipientIndex := 0
	tx := db.Begin()
	for _, t := range targets {
		sendDate := c.generateSendDate(recipientIndex, totalRecipients)
		r := &Result{
			BaseRecipient: BaseRecipient{
				Email:     t.Email,
				Phone:     t.Phone,
				Position:  t.Position,
				FirstName: t.FirstName,
				LastName:  t.LastName,
				Custom:    t.Custom,
			},
			Status:       StatusScheduled,
			CampaignId:   c.Id,
			UserId:       c.UserId,
			SendDate:     sendDate,
			Reported:     false,
			ModifiedDate: c.CreatedDate,
			SMSTarget:    c.Type == "sms",
		}

		// For SMS campaigns, we require the Phone field to be set
		// We no longer use the Email field to store phone numbers
		err = r.GenerateId(tx)
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
		processing := false
		if r.SendDate.Before(c.CreatedDate) || r.SendDate.Equal(c.CreatedDate) {
			r.Status = StatusSending
			processing = true
		}
		err = tx.Save(r).Error
		if err != nil {
			log.WithFields(logrus.Fields{
				"email": t.Email,
			}).Errorf("error creating result: %v", err)
			tx.Rollback()
			return err
		}
		c.Results = append(c.Results, *r)

		// Create the appropriate log entry based on campaign type
		if c.Type == "sms" {
			// Create SMS log entry
			s := &SMSLog{
				UserId:     c.UserId,
				CampaignId: c.Id,
				RId:        r.RId,
				SendDate:   sendDate,
				Processing: processing,
			}
			err = tx.Save(s).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"phone": t.Email, // Phone number is stored in Email field
				}).Errorf("error creating smslog entry: %v", err)
				tx.Rollback()
				return err
			}
		} else {
			// Create mail log entry
			m := &MailLog{
				UserId:     c.UserId,
				CampaignId: c.Id,
				RId:        r.RId,
				SendDate:   sendDate,
				Processing: processing,
			}
			err = tx.Save(m).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"email": t.Email,
				}).Errorf("error creating maillog entry: %v", err)
				tx.Rollback()
				return err
			}
		}
		recipientIndex++
	}
	return tx.Commit().Error
}

// postGenericCampaign handles the creation of a generic campaign.
// Generic campaigns don't require groups, templates, or sending profiles.
// They only need a landing page and create a single anonymous Result for tracking.
func postGenericCampaign(c *Campaign, uid int64) error {
	// Validate the campaign
	err := c.Validate()
	if err != nil {
		return err
	}

	// Check to make sure the page exists
	p, err := GetPageByName(c.Page.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"page": c.Page.Name,
		}).Error("Page does not exist")
		return ErrPageNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Page = p
	c.PageId = p.Id

	// Insert the campaign into the DB
	err = db.Save(c).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = AddEvent(&Event{Message: "Campaign Created"}, c.Id)
	if err != nil {
		log.Error(err)
	}

	// Create a single anonymous Result for tracking
	tx := db.Begin()
	r := &Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Link 1",
			LastName:  "",
			Email:     "",
			Phone:     "",
		},
		Status:       StatusSending,
		CampaignId:   c.Id,
		UserId:       c.UserId,
		SendDate:     c.CreatedDate,
		Reported:     false,
		ModifiedDate: c.CreatedDate,
		SMSTarget:    false,
	}

	err = r.GenerateId(tx)
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}

	err = tx.Save(r).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"campaign_id": c.Id,
		}).Errorf("error creating result for generic campaign: %v", err)
		tx.Rollback()
		return err
	}
	c.Results = append(c.Results, *r)

	// Note: We do NOT create MailLog or SMSLog entries for generic campaigns
	// because there is nothing to send - users distribute the links manually

	log.WithFields(logrus.Fields{
		"campaign_id": c.Id,
		"rid":         r.RId,
	}).Info("Created generic campaign with tracking link")

	return tx.Commit().Error
}

// GenerateCampaignLink creates a new tracking link (Result) for a generic campaign.
// This allows users to generate additional unique URLs on demand.
// If customName is provided, it will be used as the link name; otherwise auto-generates (Link 1, Link 2, etc.)
func GenerateCampaignLink(campaignId int64, uid int64, customName string) (*Result, error) {
	// Get the campaign
	c, err := GetCampaign(campaignId, uid)
	if err != nil {
		return nil, err
	}

	// Verify this is a generic campaign
	if c.Type != "generic" {
		return nil, ErrCampaignNotGeneric
	}

	// Verify the campaign is not completed
	if c.Status == CampaignComplete {
		return nil, ErrCampaignCompleted
	}

	// Count existing results to determine the link number
	var count int64
	err = db.Model(&Result{}).Where("campaign_id = ?", campaignId).Count(&count).Error
	if err != nil {
		return nil, err
	}

	// Use custom name if provided, otherwise auto-generate
	var linkName string
	if customName != "" {
		linkName = customName
	} else {
		// Create link name using simple string concatenation
		linkNumber := count + 1
		linkName = "Link "
		// Convert number to string manually for simple cases
		if linkNumber < 10 {
			linkName += string(rune('0' + linkNumber))
		} else if linkNumber < 100 {
			linkName += string(rune('0'+linkNumber/10)) + string(rune('0'+linkNumber%10))
		} else {
			// For larger numbers, just use the count
			linkName += string(rune('0'+linkNumber/100)) + string(rune('0'+(linkNumber/10)%10)) + string(rune('0'+linkNumber%10))
		}
	}

	// Create a new Result
	tx := db.Begin()
	r := &Result{
		BaseRecipient: BaseRecipient{
			FirstName: linkName,
			LastName:  "",
			Email:     "",
			Phone:     "",
		},
		Status:       StatusSending,
		CampaignId:   campaignId,
		UserId:       uid,
		SendDate:     time.Now().UTC(),
		Reported:     false,
		ModifiedDate: time.Now().UTC(),
		SMSTarget:    false,
	}

	err = r.GenerateId(tx)
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return nil, err
	}

	err = tx.Save(r).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"campaign_id": campaignId,
		}).Errorf("error creating new link for generic campaign: %v", err)
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}

	// Add a "Link Created" event with the RId as the email field for timeline matching
	err = AddEvent(&Event{Message: "Link Created", Email: r.RId}, campaignId)
	if err != nil {
		log.Warnf("error adding Link Created event: %v", err)
	}

	log.WithFields(logrus.Fields{
		"campaign_id": campaignId,
		"rid":         r.RId,
		"link_number": count + 1,
	}).Info("Generated new tracking link for generic campaign")

	return r, nil
}

// DeleteCampaign deletes the specified campaign
func DeleteCampaign(id int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Deleting campaign")
	// Delete all the campaign results
	err := db.Where("campaign_id=?", id).Delete(&Result{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&Event{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&SMSLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Delete the campaign
	err = db.Delete(&Campaign{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// DeleteCampaigns deletes multiple campaigns by their IDs for a specific user.
// It verifies ownership of each campaign before deletion.
func DeleteCampaigns(ids []int64, uid int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	log.WithFields(logrus.Fields{
		"campaign_ids": ids,
		"user_id":      uid,
	}).Info("Bulk deleting campaigns")

	// Verify that all campaigns belong to the user
	var campaigns []Campaign
	err := db.Where("id IN (?) AND user_id = ?", ids, uid).Find(&campaigns).Error
	if err != nil {
		log.Error(err)
		return 0, err
	}

	// Get the IDs of campaigns that actually belong to this user
	validIds := make([]int64, 0, len(campaigns))
	for _, c := range campaigns {
		validIds = append(validIds, c.Id)
	}

	if len(validIds) == 0 {
		return 0, nil
	}

	// Start a transaction
	tx := db.Begin()

	// Delete all related records for the valid campaigns
	err = tx.Where("campaign_id IN (?)", validIds).Delete(&Result{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return 0, err
	}

	err = tx.Where("campaign_id IN (?)", validIds).Delete(&Event{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return 0, err
	}

	err = tx.Where("campaign_id IN (?)", validIds).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return 0, err
	}

	err = tx.Where("campaign_id IN (?)", validIds).Delete(&SMSLog{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return 0, err
	}

	// Delete the campaigns
	err = tx.Where("id IN (?)", validIds).Delete(&Campaign{}).Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return 0, err
	}

	err = tx.Commit().Error
	if err != nil {
		log.Error(err)
		return 0, err
	}

	return len(validIds), nil
}

// CompleteCampaign effectively "ends" a campaign.
// Any future emails clicked will return a simple "404" page.
func CompleteCampaign(id int64, uid int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Marking campaign as complete")
	c, err := GetCampaign(id, uid)
	if err != nil {
		return err
	}
	// Delete any maillogs still set to be sent out, preventing future emails
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}

	// Delete any smslogs still set to be sent out, preventing future SMS messages
	err = db.Where("campaign_id=?", id).Delete(&SMSLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Don't overwrite original completed time
	if c.Status == CampaignComplete {
		return nil
	}
	// Mark the campaign as complete
	c.CompletedDate = time.Now().UTC()
	c.Status = CampaignComplete
	err = db.Model(&Campaign{}).Where("id=? and user_id=?", id, uid).
		Select([]string{"completed_date", "status"}).UpdateColumns(&c).Error
	if err != nil {
		log.Error(err)
	}
	return err
}
