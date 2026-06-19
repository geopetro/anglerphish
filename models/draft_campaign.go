package models

import (
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
)

// DraftCampaign is a struct representing a draft campaign
type DraftCampaign struct {
	Id                 int64       `json:"id"`
	UserId             int64       `json:"-"`
	Name               string      `json:"name" sql:"not null"`
	CreatedDate        time.Time   `json:"created_date"`
	ModifiedDate       time.Time   `json:"modified_date"`
	LaunchDate         time.Time   `json:"launch_date"`
	SendByDate         *time.Time  `json:"send_by_date"`
	DraftCampaignSetId int64       `json:"-"`
	TemplateId         int64       `json:"template_id"`
	Template           Template    `json:"template" gorm:"-"`
	SMSTemplateId      int64       `json:"sms_template_id"`
	SMSTemplate        SMSTemplate `json:"sms_template" gorm:"-"`
	PageId             int64       `json:"page_id"`
	Page               Page        `json:"page" gorm:"-"`
	SMTPId             int64       `json:"smtp_id"`
	SMTP               SMTP        `json:"smtp" gorm:"-"`
	SMSId              int64       `json:"sms_id"`
	SMS                SMS         `json:"sms,omitempty" gorm:"-"`
	URL                string      `json:"url"`
	URLParam           string      `json:"urlparam" sql:"column:urlparam"`
	QRSize             string      `json:"qrsize" sql:"column:qrsize"`
	HTTPAuth           bool        `json:"basicauth" sql:"column:basicauth"`
	Type               string      `json:"type" sql:"not null"`
	Groups             []Group     `json:"groups,omitempty"`
}

// Validate performs validation on a draft campaign
func (dc *DraftCampaign) Validate() error {
	// For drafts, we're more lenient - just require a name
	if dc.Name == "" {
		return ErrCampaignNameNotSpecified
	}

	// Set default type if not specified
	if dc.Type == "" {
		dc.Type = "email"
	}

	return nil
}

// getDetails retrieves the related entities for a draft campaign
func (dc *DraftCampaign) getDetails() error {
	// Get the template
	if dc.Type == "email" && dc.TemplateId != 0 {
		t, err := GetTemplate(dc.TemplateId, dc.UserId)
		if err != nil {
			log.Error(err)
			return err
		}
		dc.Template = t
	}

	// Get the SMS template
	if dc.Type == "sms" && dc.SMSTemplateId != 0 {
		st, err := GetSMSTemplate(dc.SMSTemplateId, dc.UserId)
		if err != nil {
			log.Error(err)
			return err
		}
		dc.SMSTemplate = st
	}

	// Get the page
	if dc.PageId != 0 {
		p, err := GetPage(dc.PageId, dc.UserId)
		if err != nil {
			log.Error(err)
			return err
		}
		dc.Page = p
	}

	// Get the sending profile
	if dc.Type == "email" && dc.SMTPId != 0 {
		s, err := GetSMTP(dc.SMTPId, dc.UserId)
		if err != nil {
			log.Error(err)
			return err
		}
		dc.SMTP = s
	}

	// Get the SMS sending profile
	if dc.Type == "sms" && dc.SMSId != 0 {
		s, err := GetSMS(dc.SMSId, dc.UserId)
		if err != nil {
			log.Error(err)
			return err
		}
		dc.SMS = s
	}

	// Get the groups
	gs, err := GetGroupsByDraftCampaignId(dc.Id)
	if err != nil {
		log.Error(err)
		return err
	}
	dc.Groups = gs

	return nil
}

// getSummary loads only id+name for each related entity without fetching full
// objects or group targets. Used when displaying draft campaigns in the edit UI.
func (dc *DraftCampaign) getSummary() error {
	if dc.Type == "email" && dc.TemplateId != 0 {
		db.Select("id, name").Where("id = ? AND user_id = ?", dc.TemplateId, dc.UserId).First(&dc.Template)
	}
	if dc.Type == "sms" && dc.SMSTemplateId != 0 {
		db.Select("id, name").Where("id = ? AND user_id = ?", dc.SMSTemplateId, dc.UserId).First(&dc.SMSTemplate)
	}
	if dc.PageId != 0 {
		db.Select("id, name").Where("id = ? AND user_id = ?", dc.PageId, dc.UserId).First(&dc.Page)
	}
	if dc.Type == "email" && dc.SMTPId != 0 {
		db.Select("id, name").Where("id = ? AND user_id = ?", dc.SMTPId, dc.UserId).First(&dc.SMTP)
	}
	if dc.Type == "sms" && dc.SMSId != 0 {
		db.Select("id, name").Where("id = ? AND user_id = ?", dc.SMSId, dc.UserId).First(&dc.SMS)
	}
	gs, err := GetGroupsByDraftCampaignId(dc.Id)
	if err != nil {
		return err
	}
	dc.Groups = gs
	return nil
}

// GetGroupsByDraftCampaignId returns the groups associated with the draft campaign
func GetGroupsByDraftCampaignId(id int64) ([]Group, error) {
	gs := []Group{}
	err := db.Table("groups").
		Select("groups.id, groups.name, groups.modified_date, groups.user_id").
		Joins("left join draft_campaign_groups dcg ON groups.id = dcg.group_id").
		Where("dcg.draft_campaign_id=?", id).
		Scan(&gs).Error
	if err != nil {
		log.Error(err)
		return gs, err
	}

	return gs, nil
}

// PostDraftCampaignTx creates a draft campaign within a transaction
func PostDraftCampaignTx(dc *DraftCampaign, uid int64, tx *gorm.DB) error {
	// Set the campaign type if not specified
	if dc.Type == "" {
		dc.Type = "email"
	}

	// Validate the draft campaign
	err := dc.Validate()
	if err != nil {
		return err
	}

	// Fill in the details
	dc.UserId = uid
	dc.CreatedDate = time.Now().UTC()
	dc.ModifiedDate = dc.CreatedDate

	// Look up and validate template if provided, but only store the ID
	if dc.Template.Id != 0 {
		t, err := GetTemplate(dc.Template.Id, uid)
		if err != nil {
			log.Errorf("Template ID %d not found or doesn't belong to user: %v", dc.Template.Id, err)
			return err
		}
		// Only store the ID
		dc.TemplateId = t.Id
	}

	// Look up and validate SMS template if provided, but only store the ID
	if dc.SMSTemplate.Id != 0 {
		st, err := GetSMSTemplate(dc.SMSTemplate.Id, uid)
		if err != nil {
			log.Errorf("SMS template ID %d not found or doesn't belong to user: %v", dc.SMSTemplate.Id, err)
			return err
		}
		// Only store the ID
		dc.SMSTemplateId = st.Id
	}

	// Look up and validate page if provided, but only store the ID
	if dc.Page.Id != 0 {
		p, err := GetPage(dc.Page.Id, uid)
		if err != nil {
			log.Errorf("Page ID %d not found or doesn't belong to user: %v", dc.Page.Id, err)
			return err
		}
		// Only store the ID
		dc.PageId = p.Id
	}

	// Look up and validate SMTP profile if provided, but only store the ID
	if dc.SMTP.Id != 0 {
		s, err := GetSMTP(dc.SMTP.Id, uid)
		if err != nil {
			log.Errorf("SMTP profile ID %d not found or doesn't belong to user: %v", dc.SMTP.Id, err)
			return err
		}
		// Only store the ID
		dc.SMTPId = s.Id
	}

	// Look up and validate SMS profile if provided, but only store the ID
	if dc.SMS.Id != 0 {
		s, err := GetSMS(dc.SMS.Id, uid)
		if err != nil {
			log.Errorf("SMS profile ID %d not found or doesn't belong to user: %v", dc.SMS.Id, err)
			return err
		}
		// Only store the ID
		dc.SMSId = s.Id
	}

	// Clear all nested objects to prevent GORM from trying to save them
	// This is the key difference from the previous approach - we're not storing
	// any objects at all, just the IDs
	dc.Template = Template{}
	dc.SMSTemplate = SMSTemplate{}
	dc.Page = Page{}
	dc.SMTP = SMTP{}
	dc.SMS = SMS{}

	// Disable GORM's automatic association saving
	tx = tx.Set("gorm:save_associations", false)
	tx = tx.Set("gorm:association_save_reference", false)

	// Enable debug logging to see the SQL queries
	debugTx := tx.Debug()

	// Insert into the DB and get the ID
	result := debugTx.Save(dc)
	if result.Error != nil {
		log.Error(result.Error)
		return result.Error
	}

	// Get the ID from the result
	var id int64
	row := tx.Raw("SELECT last_insert_rowid()").Row()
	err = row.Scan(&id)
	if err != nil {
		log.Error(err)
		return err
	}

	// Set the ID in the struct
	dc.Id = id

	// Log the ID for debugging
	log.Infof("Saved draft campaign with ID: %d", dc.Id)

	// Save the group associations using direct SQL to avoid ID issues
	for _, g := range dc.Groups {
		// Create the association using the ID we just got
		log.Infof("Creating group association for draft campaign ID: %d, group ID: %d", dc.Id, g.Id)
		err = tx.Exec("INSERT INTO draft_campaign_groups (draft_campaign_id, group_id) VALUES (?, ?)",
			dc.Id, g.Id).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}
