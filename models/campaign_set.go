package models

import (
	"errors"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// CampaignSet is a struct representing a set of campaigns to be launched together
type CampaignSet struct {
	Id                int64      `json:"id"`
	UserId            int64      `json:"-"`
	Name              string     `json:"name" sql:"not null"`
	CreatedDate       time.Time  `json:"created_date"`
	LaunchDate        time.Time  `json:"launch_date"`
	SendByDate        *time.Time `json:"send_by_date"`
	CompletedDate     time.Time  `json:"completed_date"`
	Status            string     `json:"status"`
	URL               string     `json:"url"`
	URLParam          string     `json:"urlparam" sql:"column:urlparam"`
	QRSize            string     `json:"qrsize" sql:"column:qrsize"`
	HTTPAuth          bool       `json:"basicauth" sql:"column:basicauth"`
	UseSharedSettings bool       `json:"use_shared_settings" sql:"column:use_shared_settings"`
	UseSharedPage     bool       `json:"use_shared_page" sql:"column:use_shared_page"`
	UseSharedURL      bool       `json:"use_shared_url" sql:"column:use_shared_url"`
	UseSharedURLParam bool       `json:"use_shared_urlparam" sql:"column:use_shared_urlparam"`
	UseSharedQRSize   bool       `json:"use_shared_qrsize" sql:"column:use_shared_qrsize"`
	UseSharedHTTPAuth bool       `json:"use_shared_httpauth" sql:"column:use_shared_httpauth"`
	UseSharedSchedule bool       `json:"use_shared_schedule" sql:"column:use_shared_schedule"`
	PageId            int64      `json:"-"`
	Page              Page       `json:"page"`
	SMTPId            int64      `json:"-"`
	SMTP              SMTP       `json:"smtp"`
	SMSId             int64      `json:"-"`
	SMS               SMS        `json:"sms,omitempty"`
	Campaigns         []Campaign `json:"campaigns,omitempty"`
}

// CampaignSetSummary is a struct representing the overview of a campaign set
type CampaignSetSummary struct {
	Id            int64             `json:"id"`
	Name          string            `json:"name"`
	CreatedDate   time.Time         `json:"created_date"`
	LaunchDate    time.Time         `json:"launch_date"`
	SendByDate    time.Time         `json:"send_by_date"`
	CompletedDate time.Time         `json:"completed_date"`
	Status        string            `json:"status"`
	CampaignCount int               `json:"campaign_count"`
	Campaigns     []CampaignSummary `json:"campaigns,omitempty"`
}

// CampaignSetSummaries is a struct representing the overview of campaign sets
type CampaignSetSummaries struct {
	Total        int64                `json:"total"`
	CampaignSets []CampaignSetSummary `json:"campaign_sets"`
}

// ErrCampaignSetNameNotSpecified indicates there was no name given by the user
var ErrCampaignSetNameNotSpecified = errors.New("Campaign set name not specified")

// ErrNoCampaignsSpecified indicates there were no campaigns provided for the set
var ErrNoCampaignsSpecified = errors.New("No campaigns specified for the set")

// ErrInvalidCampaignSetSendByDate indicates that the user specified a send by date that occurs before the
// launch date
var ErrInvalidCampaignSetSendByDate = errors.New("The launch date must be before the \"send emails by\" date")

// Validate checks to make sure there are no invalid fields in a submitted campaign set
func (cs *CampaignSet) Validate() error {
	switch {
	case cs.Name == "":
		return ErrCampaignSetNameNotSpecified
	case len(cs.Campaigns) == 0:
		return ErrNoCampaignsSpecified
	case cs.SendByDate != nil && !cs.LaunchDate.IsZero() && cs.SendByDate.Before(cs.LaunchDate):
		return ErrInvalidCampaignSetSendByDate
	}

	// Validate each campaign in the set
	for i := range cs.Campaigns {
		// Apply shared schedule if enabled
		if cs.UseSharedSettings || cs.UseSharedSchedule {
			// Set campaign launch date to match the set
			cs.Campaigns[i].LaunchDate = cs.LaunchDate

			// Only set send by date if it's not nil - otherwise keep it empty
			if cs.SendByDate != nil {
				cs.Campaigns[i].SendByDate = *cs.SendByDate
			}
		}

		// Apply other shared settings based on individual flags or the global flag

		// If shared URL is enabled and set, use it for all campaigns
		if (cs.UseSharedSettings || cs.UseSharedURL) && cs.URL != "" {
			cs.Campaigns[i].URL = cs.URL
		}

		// If shared URL parameter is enabled, use it for all campaigns
		// If it's empty, set it to "rid" by default
		if cs.UseSharedSettings || cs.UseSharedURLParam {
			if cs.URLParam != "" {
				cs.Campaigns[i].URLParam = cs.URLParam
			} else {
				cs.Campaigns[i].URLParam = "rid"
				log.Infof("Applied default URL parameter 'rid' to campaign %s", cs.Campaigns[i].Name)
			}
		}

		// Final safety check - ensure no campaign has an empty URL parameter
		if cs.Campaigns[i].URLParam == "" {
			cs.Campaigns[i].URLParam = "rid"
			log.Infof("Applied default URL parameter 'rid' to campaign %s (safety check)", cs.Campaigns[i].Name)
		}

		// If shared QR size is enabled, use it for all campaigns
		// This allows empty QR size to be passed through to campaigns
		if cs.UseSharedSettings || cs.UseSharedQRSize {
			cs.Campaigns[i].QRSize = cs.QRSize
		}

		// If shared HTTP auth is enabled, use it for all campaigns
		if cs.UseSharedSettings || cs.UseSharedHTTPAuth {
			cs.Campaigns[i].HTTPAuth = cs.HTTPAuth
		}

		// If shared page is enabled and set, use it for all campaigns
		if (cs.UseSharedSettings || cs.UseSharedPage) && cs.PageId != 0 {
			cs.Campaigns[i].PageId = cs.PageId
			cs.Campaigns[i].Page = cs.Page
		}

		// If shared SMTP profile is set and campaign is email type, use it
		// Note: SMTP profile sharing is not controlled by granular settings yet
		if cs.UseSharedSettings && cs.SMTPId != 0 && (cs.Campaigns[i].Type == "" || cs.Campaigns[i].Type == "email") {
			cs.Campaigns[i].SMTPId = cs.SMTPId
			cs.Campaigns[i].SMTP = cs.SMTP
			// Ensure campaign type is set to email
			cs.Campaigns[i].Type = "email"
		}

		// If shared SMS profile is set and campaign is SMS type, use it
		// Note: SMS profile sharing is not controlled by granular settings yet
		if cs.UseSharedSettings && cs.SMSId != 0 && cs.Campaigns[i].Type == "sms" {
			cs.Campaigns[i].SMSId = cs.SMSId
			cs.Campaigns[i].SMS = cs.SMS
		}

		// Validate the campaign
		err := cs.Campaigns[i].Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateStatus changes the campaign set status appropriately
func (cs *CampaignSet) UpdateStatus(s string) error {
	return db.Table("campaign_sets").Where("id=?", cs.Id).Update("status", s).Error
}

// GetCampaignSets returns the campaign sets owned by the given user.
func GetCampaignSets(uid int64) ([]CampaignSet, error) {
	css := []CampaignSet{}
	err := db.Model(&User{Id: uid}).Related(&css).Error
	if err != nil {
		log.Error(err)
		return css, err
	}

	for i := range css {
		// Get the campaigns for this set
		err = db.Model(&css[i]).Related(&css[i].Campaigns).Error
		if err != nil {
			log.Error(err)
			return css, err
		}

	}

	return css, nil
}

// GetCampaignSet returns the campaign set, if it exists, specified by the given id and user_id.
func GetCampaignSet(id int64, uid int64) (CampaignSet, error) {
	cs := CampaignSet{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&cs).Error
	if err != nil {
		log.Errorf("%s: campaign set not found", err)
		return cs, err
	}

	// Get the campaigns for this set
	err = db.Model(&cs).Related(&cs.Campaigns).Error
	if err != nil {
		log.Error(err)
		return cs, err
	}

	// Load summary (id+name only) for each campaign's related entities
	for i := range cs.Campaigns {
		if err = cs.Campaigns[i].getCampaignSummary(); err != nil {
			log.Error(err)
		}
	}

	return cs, nil
}

// GetCampaignSetSummaries gets the summary objects for all the campaign sets
// owned by the current user
func GetCampaignSetSummaries(uid int64) (CampaignSetSummaries, error) {
	overview := CampaignSetSummaries{}
	css := []CampaignSetSummary{}

	// Get the basic campaign set information
	query := db.Table("campaign_sets").Where("user_id = ?", uid)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
	err := query.Scan(&css).Error
	if err != nil {
		log.Error(err)
		return overview, err
	}

	for i := range css {
		// Get the campaigns for this set
		var campaigns []CampaignSummary
		campaignQuery := db.Table("campaigns").Where("campaign_set_id = ?", css[i].Id)
		campaignQuery = campaignQuery.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
		err := campaignQuery.Scan(&campaigns).Error
		if err != nil {
			log.Error(err)
			return overview, err
		}

		// Get stats for each campaign
		for j := range campaigns {
			s, err := getCampaignStats(campaigns[j].Id)
			if err != nil {
				log.Error(err)
				return overview, err
			}
			campaigns[j].Stats = s
		}

		css[i].CampaignCount = len(campaigns)
		css[i].Campaigns = campaigns
	}

	overview.Total = int64(len(css))
	overview.CampaignSets = css
	return overview, nil
}

// PostCampaignSet inserts a campaign set and all associated campaigns into the database.
func PostCampaignSet(cs *CampaignSet, uid int64) error {
	var err error

	// Validate basic campaign set properties, but not the campaigns yet
	// since their groups haven't been loaded with targets
	if cs.Name == "" {
		return ErrCampaignSetNameNotSpecified
	}
	if len(cs.Campaigns) == 0 {
		return ErrNoCampaignsSpecified
	}
	if cs.SendByDate != nil && !cs.LaunchDate.IsZero() && cs.SendByDate.Before(cs.LaunchDate) {
		return ErrInvalidCampaignSetSendByDate
	}

	// Fill in the details
	cs.UserId = uid
	cs.CreatedDate = time.Now().UTC()
	cs.CompletedDate = time.Time{}
	cs.Status = CampaignQueued
	if cs.LaunchDate.IsZero() {
		cs.LaunchDate = cs.CreatedDate
	} else {
		cs.LaunchDate = cs.LaunchDate.UTC()
	}
	if cs.SendByDate != nil {
		utcTime := cs.SendByDate.UTC()
		cs.SendByDate = &utcTime
	}
	if cs.LaunchDate.Before(cs.CreatedDate) || cs.LaunchDate.Equal(cs.CreatedDate) {
		cs.Status = CampaignInProgress
	}

	// Start a transaction
	tx := db.Begin()

	// Insert the campaign set
	log.Infof("Saving campaign set with ID: %d, Name: %s", cs.Id, cs.Name)
	err = tx.Save(cs).Error
	if err != nil {
		tx.Rollback()
		log.Errorf("Error saving campaign set: %v", err)
		return err
	}
	log.Infof("Campaign set saved successfully with ID: %d", cs.Id)

	// Create each campaign in the set
	for i := range cs.Campaigns {
		// Set the campaign set ID
		cs.Campaigns[i].CampaignSetId = cs.Id
		log.Infof("Creating campaign %d of %d: %s with campaign_set_id: %d", i+1, len(cs.Campaigns), cs.Campaigns[i].Name, cs.Id)

		// Create the campaign
		err = PostCampaignTx(&cs.Campaigns[i], uid, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("Error creating campaign %s: %v", cs.Campaigns[i].Name, err)
			return err
		}
		log.Infof("Campaign %s created successfully with ID: %d", cs.Campaigns[i].Name, cs.Campaigns[i].Id)
	}

	// Commit the transaction
	return tx.Commit().Error
}

// PostCampaignTx creates a campaign within a transaction
// This is a modified version of PostCampaign that works within a transaction
func PostCampaignTx(c *Campaign, uid int64, tx *gorm.DB) error {
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

	// Check to make sure all the groups already exist
	// Also, later we'll need to know the total number of recipients (counting
	// duplicates is ok for now), so we'll do that here to save a loop.
	totalRecipients := 0

	// Create a new slice to hold the resolved groups
	resolvedGroups := make([]Group, 0, len(c.Groups))

	for _, g := range c.Groups {
		log.Infof("Looking up group: %s for user ID: %d", g.Name, uid)

		// First, get the group by name and user ID
		var group Group
		err := tx.Where("name = ? AND user_id = ?", g.Name, uid).First(&group).Error
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"group": g.Name,
			}).Error("Group does not exist")
			return ErrGroupNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}

		log.Infof("Found group %s with ID: %d", group.Name, group.Id)

		// Get the targets for this group using the transaction
		// This is similar to GetTargets but uses the transaction
		var targets []Target
		err = tx.Table("targets").
			Select("targets.id, targets.email, targets.phone, targets.first_name, targets.last_name, targets.position, targets.custom").
			Joins("left join group_targets gt ON targets.id = gt.target_id").
			Where("gt.group_id=?", group.Id).
			Scan(&targets).Error
		if err != nil {
			log.Errorf("Error getting targets for group %s: %v", group.Name, err)
			return err
		}

		log.Infof("Found %d targets for group %s", len(targets), group.Name)

		// Assign the targets to the group
		group.Targets = targets

		// Add the group to our resolved groups
		resolvedGroups = append(resolvedGroups, group)
		totalRecipients += len(targets)
	}

	// Replace the original groups with the resolved ones
	c.Groups = resolvedGroups

	// Now that we have loaded all the groups with their targets,
	// validate the campaign to ensure targets match the campaign type
	err = c.Validate()
	if err != nil {
		return err
	}

	// Check to make sure the page exists
	p := Page{}
	err = tx.Where("name = ?", c.Page.Name).Where("user_id = ?", uid).First(&p).Error
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
		st := SMSTemplate{}
		err = tx.Where("name = ?", c.SMSTemplate.Name).Where("user_id = ?", uid).First(&st).Error
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
		s := SMS{}
		err = tx.Where("name = ?", c.SMS.Name).Where("user_id = ?", uid).First(&s).Error
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
		t := Template{}
		err = tx.Where("name = ?", c.Template.Name).Where("user_id = ?", uid).First(&t).Error
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
		s := SMTP{}
		err = tx.Where("name = ?", c.SMTP.Name).Where("user_id = ?", uid).First(&s).Error
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
	err = tx.Save(c).Error
	if err != nil {
		log.Error(err)
		return err
	}

	// Create an event for the campaign
	e := Event{
		Message:    "Campaign Created",
		CampaignId: c.Id,
		Time:       time.Now().UTC(),
	}
	err = tx.Save(&e).Error
	if err != nil {
		log.Error(err)
		return err
	}

	// Insert all the results
	resultMap := make(map[string]bool)
	recipientIndex := 0

	for _, g := range c.Groups {
		// Insert a result for each target in the group
		for _, t := range g.Targets {
			// Remove duplicate results - we should only
			// send messages to unique addresses.
			if _, ok := resultMap[t.Email]; ok {
				continue
			}
			resultMap[t.Email] = true
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

			// For SMS campaigns, ensure the Phone field is set
			// since we're using the Email field to store the phone number
			if c.Type == "sms" && r.Phone == "" {
				r.Phone = r.Email
			}
			err = r.GenerateId(tx)
			if err != nil {
				log.Error(err)
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
					return err
				}
			}
			recipientIndex++
		}
	}

	return nil
}

// DeleteCampaignSet deletes the specified campaign set and all its campaigns
func DeleteCampaignSet(id int64) error {
	log.WithFields(logrus.Fields{
		"campaign_set_id": id,
	}).Info("Deleting campaign set")

	// Start a transaction
	tx := db.Begin()

	// Get all campaigns in this set
	var campaigns []Campaign
	err := tx.Where("campaign_set_id = ?", id).Find(&campaigns).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Delete each campaign
	for _, campaign := range campaigns {
		// Delete all the campaign results
		err := tx.Where("campaign_id = ?", campaign.Id).Delete(&Result{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		err = tx.Where("campaign_id = ?", campaign.Id).Delete(&Event{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		err = tx.Where("campaign_id = ?", campaign.Id).Delete(&MailLog{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		err = tx.Where("campaign_id = ?", campaign.Id).Delete(&SMSLog{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
	}

	// Delete all campaigns in this set
	err = tx.Where("campaign_set_id = ?", id).Delete(&Campaign{}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Delete the campaign set
	err = tx.Delete(&CampaignSet{Id: id}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Commit the transaction
	return tx.Commit().Error
}

// CompleteCampaignSet effectively "ends" a campaign set and all its campaigns.
func CompleteCampaignSet(id int64, uid int64) error {
	log.WithFields(logrus.Fields{
		"campaign_set_id": id,
	}).Info("Marking campaign set as complete")

	// Get the campaign set
	cs, err := GetCampaignSet(id, uid)
	if err != nil {
		return err
	}

	// Don't overwrite original completed time
	if cs.Status == CampaignComplete {
		return nil
	}

	// Start a transaction
	tx := db.Begin()

	// Complete each campaign in the set
	for _, campaign := range cs.Campaigns {
		// Delete any maillogs still set to be sent out, preventing future emails
		err = tx.Where("campaign_id = ?", campaign.Id).Delete(&MailLog{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		// Delete any smslogs still set to be sent out, preventing future SMS messages
		err = tx.Where("campaign_id = ?", campaign.Id).Delete(&SMSLog{}).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		// Mark the campaign as complete
		campaign.CompletedDate = time.Now().UTC()
		campaign.Status = CampaignComplete
		err = tx.Model(&Campaign{}).Where("id = ? and user_id = ?", campaign.Id, uid).
			Select([]string{"completed_date", "status"}).UpdateColumns(&campaign).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
	}

	// Mark the campaign set as complete
	cs.CompletedDate = time.Now().UTC()
	cs.Status = CampaignComplete
	err = tx.Model(&CampaignSet{}).Where("id = ? and user_id = ?", id, uid).
		Select([]string{"completed_date", "status"}).UpdateColumns(&cs).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Commit the transaction
	return tx.Commit().Error
}
