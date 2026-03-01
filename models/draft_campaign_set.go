package models

import (
	"time"

	log "github.com/gophish/gophish/logger"
)

// DraftCampaignSet is a struct representing a draft set of campaigns
type DraftCampaignSet struct {
	Id                int64           `json:"id"`
	UserId            int64           `json:"-"`
	Name              string          `json:"name" sql:"not null"`
	CreatedDate       time.Time       `json:"created_date"`
	ModifiedDate      time.Time       `json:"modified_date"`
	LaunchDate        time.Time       `json:"launch_date"`
	SendByDate        *time.Time      `json:"send_by_date"`
	URL               string          `json:"url"`
	URLParam          string          `json:"urlparam" sql:"column:urlparam"`
	QRSize            string          `json:"qrsize" sql:"column:qrsize"`
	HTTPAuth          bool            `json:"basicauth" sql:"column:basicauth"`
	UseSharedSettings bool            `json:"use_shared_settings" sql:"column:use_shared_settings"`
	UseSharedPage     bool            `json:"use_shared_page" sql:"column:use_shared_page"`
	UseSharedURL      bool            `json:"use_shared_url" sql:"column:use_shared_url"`
	UseSharedURLParam bool            `json:"use_shared_urlparam" sql:"column:use_shared_urlparam"`
	UseSharedQRSize   bool            `json:"use_shared_qrsize" sql:"column:use_shared_qrsize"`
	UseSharedHTTPAuth bool            `json:"use_shared_httpauth" sql:"column:use_shared_httpauth"`
	UseSharedSchedule bool            `json:"use_shared_schedule" sql:"column:use_shared_schedule"`
	PageId            int64           `json:"page_id"`
	Page              Page            `json:"page" gorm:"-"`
	SMTPId            int64           `json:"smtp_id"`
	SMTP              SMTP            `json:"smtp" gorm:"-"`
	SMSId             int64           `json:"sms_id"`
	SMS               SMS             `json:"sms,omitempty" gorm:"-"`
	DraftCampaigns    []DraftCampaign `json:"campaigns,omitempty" gorm:"-"`
}

// Validate checks to make sure there are no invalid fields in a submitted draft campaign set
func (dcs *DraftCampaignSet) Validate() error {
	if dcs.Name == "" {
		return ErrCampaignSetNameNotSpecified
	}

	// For drafts, we don't require campaigns to be specified
	// This allows saving a draft with just basic info

	// Check send by date if specified
	if dcs.SendByDate != nil && !dcs.LaunchDate.IsZero() && dcs.SendByDate.Before(dcs.LaunchDate) {
		return ErrInvalidCampaignSetSendByDate
	}

	return nil
}

// GetDraftCampaignSets returns the draft campaign sets owned by the given user.
func GetDraftCampaignSets(uid int64) ([]DraftCampaignSet, error) {
	dcs := []DraftCampaignSet{}
	err := db.Where("user_id = ?", uid).Find(&dcs).Error
	if err != nil {
		log.Error(err)
		return dcs, err
	}

	for i := range dcs {
		// Get the draft campaigns for this set
		err = db.Where("draft_campaign_set_id = ?", dcs[i].Id).Find(&dcs[i].DraftCampaigns).Error
		if err != nil {
			log.Error(err)
			return dcs, err
		}

		// Get details for each draft campaign
		for j := range dcs[i].DraftCampaigns {
			err = dcs[i].DraftCampaigns[j].getDetails()
			if err != nil {
				log.Error(err)
			}
		}
	}

	return dcs, nil
}

// GetDraftCampaignSet returns the draft campaign set, if it exists, specified by the given id and user_id.
func GetDraftCampaignSet(id int64, uid int64) (DraftCampaignSet, error) {
	dcs := DraftCampaignSet{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).Find(&dcs).Error
	if err != nil {
		log.Errorf("%s: draft campaign set not found", err)
		return dcs, err
	}

	// Get the draft campaigns for this set
	err = db.Where("draft_campaign_set_id = ?", dcs.Id).Find(&dcs.DraftCampaigns).Error
	if err != nil {
		log.Error(err)
		return dcs, err
	}

	// Get details for each draft campaign
	for i := range dcs.DraftCampaigns {
		err = dcs.DraftCampaigns[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}

	return dcs, nil
}

// PostDraftCampaignSet inserts a draft campaign set and all associated draft campaigns into the database.
func PostDraftCampaignSet(dcs *DraftCampaignSet, uid int64) error {
	// Validate the draft campaign set
	err := dcs.Validate()
	if err != nil {
		return err
	}

	// Fill in the details
	dcs.UserId = uid
	dcs.CreatedDate = time.Now().UTC()
	dcs.ModifiedDate = dcs.CreatedDate

	// Look up and validate page if provided, but only store the ID
	if dcs.Page.Id != 0 {
		p, err := GetPage(dcs.Page.Id, uid)
		if err != nil {
			log.Errorf("Page ID %d not found or doesn't belong to user: %v", dcs.Page.Id, err)
			return err
		}
		// Only store the ID
		dcs.PageId = p.Id
	}

	// Look up and validate SMTP profile if provided, but only store the ID
	if dcs.SMTP.Id != 0 {
		s, err := GetSMTP(dcs.SMTP.Id, uid)
		if err != nil {
			log.Errorf("SMTP profile ID %d not found or doesn't belong to user: %v", dcs.SMTP.Id, err)
			return err
		}
		// Only store the ID
		dcs.SMTPId = s.Id
	}

	// Look up and validate SMS profile if provided, but only store the ID
	if dcs.SMS.Id != 0 {
		s, err := GetSMS(dcs.SMS.Id, uid)
		if err != nil {
			log.Errorf("SMS profile ID %d not found or doesn't belong to user: %v", dcs.SMS.Id, err)
			return err
		}
		// Only store the ID
		dcs.SMSId = s.Id
	}

	// Clear all nested objects to prevent GORM from trying to save them
	// This is the key difference from the previous approach - we're not storing
	// any objects at all, just the IDs
	dcs.Page = Page{}
	dcs.SMTP = SMTP{}
	dcs.SMS = SMS{}

	// Start a transaction
	tx := db.Begin()

	// Disable GORM's automatic association saving
	tx = tx.Set("gorm:save_associations", false)
	tx = tx.Set("gorm:association_save_reference", false)

	// Enable debug logging to see the SQL queries
	debugTx := tx.Debug()

	// Insert the draft campaign set
	// Use the Save method with the struct to ensure all fields are included
	err = debugTx.Save(dcs).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Get the ID from the result if it's not set
	if dcs.Id == 0 {
		var id int64
		row := tx.Raw("SELECT last_insert_rowid()").Row()
		err = row.Scan(&id)
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
		dcs.Id = id
	}

	log.Infof("Saved draft campaign set with ID: %d", dcs.Id)

	// Create each draft campaign in the set
	for i := range dcs.DraftCampaigns {
		// Set the draft campaign set ID
		dcs.DraftCampaigns[i].DraftCampaignSetId = dcs.Id

		// Save the draft campaign first without groups
		campaign := &dcs.DraftCampaigns[i]
		campaign.UserId = uid
		campaign.CreatedDate = time.Now().UTC()
		campaign.ModifiedDate = campaign.CreatedDate

		// Extract IDs from nested objects before clearing them
		if campaign.Template.Id != 0 {
			campaign.TemplateId = campaign.Template.Id
			log.Infof("Extracted template ID %d from nested object", campaign.TemplateId)
		}
		if campaign.SMSTemplate.Id != 0 {
			campaign.SMSTemplateId = campaign.SMSTemplate.Id
			log.Infof("Extracted SMS template ID %d from nested object", campaign.SMSTemplateId)
		}
		if campaign.Page.Id != 0 {
			campaign.PageId = campaign.Page.Id
			log.Infof("Extracted page ID %d from nested object", campaign.PageId)
		}
		if campaign.SMTP.Id != 0 {
			campaign.SMTPId = campaign.SMTP.Id
			log.Infof("Extracted SMTP ID %d from nested object", campaign.SMTPId)
		}
		if campaign.SMS.Id != 0 {
			campaign.SMSId = campaign.SMS.Id
			log.Infof("Extracted SMS ID %d from nested object", campaign.SMSId)
		}

		// Clear all nested objects to prevent GORM from trying to save them
		campaign.Template = Template{}
		campaign.SMSTemplate = SMSTemplate{}
		campaign.Page = Page{}
		campaign.SMTP = SMTP{}
		campaign.SMS = SMS{}

		// Disable GORM's automatic association saving for this campaign
		campaignTx := tx.Set("gorm:save_associations", false)
		campaignTx = campaignTx.Set("gorm:association_save_reference", false)

		// Enable debug logging for campaign saving
		debugCampaignTx := campaignTx.Debug()

		// Save the campaign
		err = debugCampaignTx.Save(campaign).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		// Get the campaign ID if it's not set
		if campaign.Id == 0 {
			var id int64
			row := tx.Raw("SELECT last_insert_rowid()").Row()
			err = row.Scan(&id)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				return err
			}
			campaign.Id = id
		}

		log.Infof("Saved draft campaign with ID: %d", campaign.Id)

		// Save the group associations directly
		for _, g := range campaign.Groups {
			log.Infof("Creating group association for draft campaign ID: %d, group ID: %d", campaign.Id, g.Id)
			err = tx.Exec("INSERT INTO draft_campaign_groups (draft_campaign_id, group_id) VALUES (?, ?)",
				campaign.Id, g.Id).Error
			if err != nil {
				tx.Rollback()
				log.Error(err)
				return err
			}
		}
	}

	// Commit the transaction
	return tx.Commit().Error
}

// UpdateDraftCampaignSet updates a draft campaign set in the database
func UpdateDraftCampaignSet(dcs *DraftCampaignSet, uid int64) error {
	// Validate the draft campaign set
	err := dcs.Validate()
	if err != nil {
		return err
	}

	// Update the modified date
	dcs.ModifiedDate = time.Now().UTC()

	// Look up and validate page if provided, but only store the ID
	if dcs.Page.Id != 0 {
		p, err := GetPage(dcs.Page.Id, uid)
		if err != nil {
			log.Errorf("Page ID %d not found or doesn't belong to user: %v", dcs.Page.Id, err)
			return err
		}
		// Only store the ID
		dcs.PageId = p.Id
	}

	// Look up and validate SMTP profile if provided, but only store the ID
	if dcs.SMTP.Id != 0 {
		s, err := GetSMTP(dcs.SMTP.Id, uid)
		if err != nil {
			log.Errorf("SMTP profile ID %d not found or doesn't belong to user: %v", dcs.SMTP.Id, err)
			return err
		}
		// Only store the ID
		dcs.SMTPId = s.Id
	}

	// Look up and validate SMS profile if provided, but only store the ID
	if dcs.SMS.Id != 0 {
		s, err := GetSMS(dcs.SMS.Id, uid)
		if err != nil {
			log.Errorf("SMS profile ID %d not found or doesn't belong to user: %v", dcs.SMS.Id, err)
			return err
		}
		// Only store the ID
		dcs.SMSId = s.Id
	}

	// Clear all nested objects to prevent GORM from trying to save them
	// This is the key difference from the previous approach - we're not storing
	// any objects at all, just the IDs
	dcs.Page = Page{}
	dcs.SMTP = SMTP{}
	dcs.SMS = SMS{}

	// Start a transaction
	tx := db.Begin()

	// First, delete all existing draft campaigns for this set
	err = tx.Where("draft_campaign_set_id = ?", dcs.Id).Delete(&DraftCampaign{}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Disable GORM's automatic association saving
	tx = tx.Set("gorm:save_associations", false)
	tx = tx.Set("gorm:association_save_reference", false)

	// Enable debug logging to see the SQL queries
	debugTx := tx.Debug()

	// Update the draft campaign set
	// Explicitly list all fields to ensure all shared settings are included
	err = debugTx.Model(&DraftCampaignSet{}).Where("id = ? AND user_id = ?", dcs.Id, uid).
		Updates(map[string]interface{}{
			"name":                dcs.Name,
			"modified_date":       dcs.ModifiedDate,
			"launch_date":         dcs.LaunchDate,
			"send_by_date":        dcs.SendByDate,
			"url":                 dcs.URL,
			"urlparam":            dcs.URLParam,
			"qrsize":              dcs.QRSize,
			"basicauth":           dcs.HTTPAuth,
			"use_shared_settings": dcs.UseSharedSettings,
			"use_shared_page":     dcs.UseSharedPage,
			"use_shared_url":      dcs.UseSharedURL,
			"use_shared_urlparam": dcs.UseSharedURLParam,
			"use_shared_qrsize":   dcs.UseSharedQRSize,
			"use_shared_httpauth": dcs.UseSharedHTTPAuth,
			"use_shared_schedule": dcs.UseSharedSchedule,
			"page_id":             dcs.PageId,
			"smtp_id":             dcs.SMTPId,
			"sms_id":              dcs.SMSId,
		}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Create each draft campaign in the set
	for i := range dcs.DraftCampaigns {
		// Set the draft campaign set ID
		dcs.DraftCampaigns[i].DraftCampaignSetId = dcs.Id

		// Save the draft campaign first without groups
		campaign := &dcs.DraftCampaigns[i]
		campaign.UserId = uid
		campaign.CreatedDate = time.Now().UTC()
		campaign.ModifiedDate = campaign.CreatedDate

		// Extract IDs from nested objects before clearing them
		if campaign.Template.Id != 0 {
			campaign.TemplateId = campaign.Template.Id
			log.Infof("Extracted template ID %d from nested object", campaign.TemplateId)
		}
		if campaign.SMSTemplate.Id != 0 {
			campaign.SMSTemplateId = campaign.SMSTemplate.Id
			log.Infof("Extracted SMS template ID %d from nested object", campaign.SMSTemplateId)
		}
		if campaign.Page.Id != 0 {
			campaign.PageId = campaign.Page.Id
			log.Infof("Extracted page ID %d from nested object", campaign.PageId)
		}
		if campaign.SMTP.Id != 0 {
			campaign.SMTPId = campaign.SMTP.Id
			log.Infof("Extracted SMTP ID %d from nested object", campaign.SMTPId)
		}
		if campaign.SMS.Id != 0 {
			campaign.SMSId = campaign.SMS.Id
			log.Infof("Extracted SMS ID %d from nested object", campaign.SMSId)
		}

		// Clear all nested objects to prevent GORM from trying to save them
		campaign.Template = Template{}
		campaign.SMSTemplate = SMSTemplate{}
		campaign.Page = Page{}
		campaign.SMTP = SMTP{}
		campaign.SMS = SMS{}

		// Disable GORM's automatic association saving for this campaign
		campaignTx := tx.Set("gorm:save_associations", false)
		campaignTx = campaignTx.Set("gorm:association_save_reference", false)

		// Enable debug logging for campaign saving
		debugCampaignTx := campaignTx.Debug()

		// Save the campaign
		err = debugCampaignTx.Save(campaign).Error
		if err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}

		// Get the campaign ID if it's not set
		if campaign.Id == 0 {
			var id int64
			row := tx.Raw("SELECT last_insert_rowid()").Row()
			err = row.Scan(&id)
			if err != nil {
				tx.Rollback()
				log.Error(err)
				return err
			}
			campaign.Id = id
		}

		log.Infof("Updated draft campaign with ID: %d", campaign.Id)

		// Save the group associations directly
		for _, g := range campaign.Groups {
			log.Infof("Creating group association for draft campaign ID: %d, group ID: %d", campaign.Id, g.Id)
			err = tx.Exec("INSERT INTO draft_campaign_groups (draft_campaign_id, group_id) VALUES (?, ?)",
				campaign.Id, g.Id).Error
			if err != nil {
				tx.Rollback()
				log.Error(err)
				return err
			}
		}
	}

	// Commit the transaction
	return tx.Commit().Error
}

// DeleteDraftCampaignSet deletes the specified draft campaign set and all its draft campaigns
func DeleteDraftCampaignSet(id int64, uid int64) error {
	// Start a transaction
	tx := db.Begin()

	// Delete all draft campaigns in this set
	err := tx.Where("draft_campaign_set_id = ?", id).Delete(&DraftCampaign{}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Delete the draft campaign set
	err = tx.Where("id = ? AND user_id = ?", id, uid).Delete(&DraftCampaignSet{}).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}

	// Commit the transaction
	return tx.Commit().Error
}

// LaunchDraftCampaignSet converts a draft campaign set to a real campaign set and launches it
func LaunchDraftCampaignSet(id int64, uid int64) (CampaignSet, error) {
	// Get the draft campaign set
	dcs, err := GetDraftCampaignSet(id, uid)
	if err != nil {
		return CampaignSet{}, err
	}

	// Create a new campaign set from the draft
	cs := CampaignSet{
		UserId:     uid,
		Name:       dcs.Name,
		LaunchDate: dcs.LaunchDate,
		SendByDate: dcs.SendByDate,
		URL:        dcs.URL,
		URLParam:   dcs.URLParam,
		QRSize:     dcs.QRSize,
		HTTPAuth:   dcs.HTTPAuth,
		// Copy all granular shared settings flags
		UseSharedSettings: dcs.UseSharedSettings,
		UseSharedPage:     dcs.UseSharedPage,
		UseSharedURL:      dcs.UseSharedURL,
		UseSharedURLParam: dcs.UseSharedURLParam,
		UseSharedQRSize:   dcs.UseSharedQRSize,
		UseSharedHTTPAuth: dcs.UseSharedHTTPAuth,
		UseSharedSchedule: dcs.UseSharedSchedule,
	}

	// Set default URL parameter to "rid" if it's empty
	if cs.URLParam == "" {
		cs.URLParam = "rid"
		log.Infof("Setting default URL parameter 'rid' for campaign set")
	}

	// Look up and validate page if provided
	if dcs.PageId != 0 {
		p, err := GetPage(dcs.PageId, uid)
		if err != nil {
			log.Errorf("Page ID %d not found or doesn't belong to user: %v", dcs.PageId, err)
			return cs, err
		}
		cs.Page = p
		cs.PageId = p.Id
		log.Infof("Set shared page ID %d for campaign set", cs.PageId)
	}

	// Look up and validate SMTP profile if provided
	if dcs.SMTPId != 0 {
		s, err := GetSMTP(dcs.SMTPId, uid)
		if err != nil {
			log.Errorf("SMTP profile ID %d not found or doesn't belong to user: %v", dcs.SMTPId, err)
			return cs, err
		}
		cs.SMTP = s
		cs.SMTPId = s.Id
		log.Infof("Set shared SMTP profile ID %d for campaign set", cs.SMTPId)
	}

	// Look up and validate SMS profile if provided
	if dcs.SMSId != 0 {
		s, err := GetSMS(dcs.SMSId, uid)
		if err != nil {
			log.Errorf("SMS profile ID %d not found or doesn't belong to user: %v", dcs.SMSId, err)
			return cs, err
		}
		cs.SMS = s
		cs.SMSId = s.Id
		log.Infof("Set shared SMS profile ID %d for campaign set", cs.SMSId)
	}

	// Convert draft campaigns to real campaigns
	cs.Campaigns = make([]Campaign, len(dcs.DraftCampaigns))
	for i, dc := range dcs.DraftCampaigns {
		// Create the basic campaign structure
		campaign := Campaign{
			UserId:     uid,
			Name:       dc.Name,
			LaunchDate: dc.LaunchDate,
			URL:        dc.URL,
			URLParam:   dc.URLParam,
			QRSize:     dc.QRSize,
			HTTPAuth:   dc.HTTPAuth,
			Type:       dc.Type,
			Groups:     dc.Groups,
		}

		// Set default URL parameter to "rid" if it's empty
		if campaign.URLParam == "" {
			campaign.URLParam = "rid"
			log.Infof("Setting default URL parameter 'rid' for campaign %s", campaign.Name)
		}

		// Apply shared schedule if enabled
		if cs.UseSharedSettings || cs.UseSharedSchedule {
			// Set campaign launch date to match the set
			campaign.LaunchDate = cs.LaunchDate
			log.Infof("Applied shared launch date to campaign %s", campaign.Name)

			// Only set send by date if it's not nil - otherwise keep it empty
			if cs.SendByDate != nil {
				campaign.SendByDate = *cs.SendByDate
				log.Infof("Applied shared send by date to campaign %s", campaign.Name)
			}
		} else {
			// Set SendByDate if not nil and not using shared schedule
			if dc.SendByDate != nil {
				campaign.SendByDate = *dc.SendByDate
			}
		}

		// Apply other shared settings based on individual flags or the global flag

		// If shared URL is enabled and set, use it for all campaigns
		if (cs.UseSharedSettings || cs.UseSharedURL) && cs.URL != "" {
			campaign.URL = cs.URL
			log.Infof("Applied shared URL '%s' to campaign %s", cs.URL, campaign.Name)
		}

		// If shared URL parameter is enabled, use it for all campaigns
		if cs.UseSharedSettings || cs.UseSharedURLParam {
			if cs.URLParam != "" {
				campaign.URLParam = cs.URLParam
				log.Infof("Applied shared URL parameter '%s' to campaign %s", cs.URLParam, campaign.Name)
			} else {
				campaign.URLParam = "rid"
				log.Infof("Applied default URL parameter 'rid' to campaign %s", campaign.Name)
			}
		}

		// Final safety check - ensure no campaign has an empty URL parameter
		if campaign.URLParam == "" {
			campaign.URLParam = "rid"
			log.Infof("Applied default URL parameter 'rid' to campaign %s (safety check)", campaign.Name)
		}

		// If shared QR size is enabled, use it for all campaigns
		if cs.UseSharedSettings || cs.UseSharedQRSize {
			campaign.QRSize = cs.QRSize
			log.Infof("Applied shared QR size '%s' to campaign %s", cs.QRSize, campaign.Name)
		}

		// If shared HTTP auth is enabled, use it for all campaigns
		if cs.UseSharedSettings || cs.UseSharedHTTPAuth {
			campaign.HTTPAuth = cs.HTTPAuth
			log.Infof("Applied shared HTTP auth setting to campaign %s", campaign.Name)
		}

		// If shared page is enabled and set, use it for all campaigns
		if (cs.UseSharedSettings || cs.UseSharedPage) && cs.PageId != 0 {
			campaign.PageId = cs.PageId
			campaign.Page = cs.Page
			log.Infof("Applied shared page ID %d to campaign %s", cs.PageId, campaign.Name)
		} else if dc.PageId != 0 {
			// Look up and validate individual page if provided and not using shared page
			p, err := GetPage(dc.PageId, uid)
			if err != nil {
				log.Errorf("Page ID %d not found or doesn't belong to user: %v", dc.PageId, err)
				return cs, err
			}
			campaign.Page = p
			campaign.PageId = p.Id
			log.Infof("Set individual page ID %d for campaign %s", campaign.PageId, campaign.Name)
		}

		// If shared SMTP profile is set and campaign is email type, use it
		if cs.UseSharedSettings && cs.SMTPId != 0 && (campaign.Type == "" || campaign.Type == "email") {
			campaign.SMTPId = cs.SMTPId
			campaign.SMTP = cs.SMTP
			// Ensure campaign type is set to email
			campaign.Type = "email"
			log.Infof("Applied shared SMTP profile ID %d to campaign %s", cs.SMTPId, campaign.Name)
		} else if dc.SMTPId != 0 && (campaign.Type == "" || campaign.Type == "email") {
			// Look up and validate individual SMTP profile if provided and not using shared profile
			s, err := GetSMTP(dc.SMTPId, uid)
			if err != nil {
				log.Errorf("SMTP profile ID %d not found or doesn't belong to user: %v", dc.SMTPId, err)
				return cs, err
			}
			campaign.SMTP = s
			campaign.SMTPId = s.Id
			// Ensure campaign type is set to email
			campaign.Type = "email"
			log.Infof("Set individual SMTP profile ID %d for campaign %s", campaign.SMTPId, campaign.Name)
		}

		// If shared SMS profile is set and campaign is SMS type, use it
		if cs.UseSharedSettings && cs.SMSId != 0 && campaign.Type == "sms" {
			campaign.SMSId = cs.SMSId
			campaign.SMS = cs.SMS
			log.Infof("Applied shared SMS profile ID %d to campaign %s", cs.SMSId, campaign.Name)
		} else if dc.SMSId != 0 && campaign.Type == "sms" {
			// Look up and validate individual SMS profile if provided and not using shared profile
			s, err := GetSMS(dc.SMSId, uid)
			if err != nil {
				log.Errorf("SMS profile ID %d not found or doesn't belong to user: %v", dc.SMSId, err)
				return cs, err
			}
			campaign.SMS = s
			campaign.SMSId = s.Id
			log.Infof("Set individual SMS profile ID %d for campaign %s", campaign.SMSId, campaign.Name)
		}

		// Look up and validate template if provided for email campaigns
		if campaign.Type == "email" || campaign.Type == "" {
			if dc.TemplateId != 0 {
				t, err := GetTemplate(dc.TemplateId, uid)
				if err != nil {
					log.Errorf("Template ID %d not found or doesn't belong to user: %v", dc.TemplateId, err)
					return cs, err
				}
				campaign.Template = t
				campaign.TemplateId = t.Id
				log.Infof("Set template ID %d for campaign %s", campaign.TemplateId, campaign.Name)
			}
		}

		// Look up and validate SMS template if provided for SMS campaigns
		if campaign.Type == "sms" {
			if dc.SMSTemplateId != 0 {
				st, err := GetSMSTemplate(dc.SMSTemplateId, uid)
				if err != nil {
					log.Errorf("SMS template ID %d not found or doesn't belong to user: %v", dc.SMSTemplateId, err)
					return cs, err
				}
				campaign.SMSTemplate = st
				campaign.SMSTemplateId = st.Id
				log.Infof("Set SMS template ID %d for campaign %s", campaign.SMSTemplateId, campaign.Name)
			}
		}

		// Add the campaign to the set
		cs.Campaigns[i] = campaign
	}

	// Pre-launch validation to catch any missing required fields
	for i, campaign := range cs.Campaigns {
		// Check for missing page
		if campaign.PageId == 0 {
			log.Errorf("Campaign %s is missing a landing page", campaign.Name)
			return cs, ErrPageNotSpecified
		}

		// Check for missing template for email campaigns
		if (campaign.Type == "email" || campaign.Type == "") && campaign.TemplateId == 0 {
			log.Errorf("Email campaign %s is missing a template", campaign.Name)
			return cs, ErrTemplateNotSpecified
		}

		// Check for missing SMS template for SMS campaigns
		if campaign.Type == "sms" && campaign.SMSTemplateId == 0 {
			log.Errorf("SMS campaign %s is missing an SMS template", campaign.Name)
			return cs, ErrSMSTemplateNotSpecified
		}

		// Check for missing SMTP profile for email campaigns
		if (campaign.Type == "email" || campaign.Type == "") && campaign.SMTPId == 0 {
			log.Errorf("Email campaign %s is missing a sending profile", campaign.Name)
			return cs, ErrSMTPNotSpecified
		}

		// Check for missing SMS profile for SMS campaigns
		if campaign.Type == "sms" && campaign.SMSId == 0 {
			log.Errorf("SMS campaign %s is missing an SMS sending profile", campaign.Name)
			return cs, ErrSMSNotSpecified
		}

		// Check for missing groups
		if len(campaign.Groups) == 0 {
			log.Errorf("Campaign %s has no groups specified", campaign.Name)
			return cs, ErrGroupNotSpecified
		}

		log.Infof("Campaign %s validation passed", campaign.Name)
		cs.Campaigns[i] = campaign
	}

	// Validate the campaign set before posting
	err = cs.Validate()
	if err != nil {
		log.Errorf("Campaign set validation failed: %v", err)
		return cs, err
	}

	// Post the campaign set (this will create all campaigns and results)
	err = PostCampaignSet(&cs, uid)
	if err != nil {
		log.Errorf("Error posting campaign set: %v", err)
		return cs, err
	}

	// Delete the draft campaign set
	err = DeleteDraftCampaignSet(id, uid)
	if err != nil {
		log.Error(err)
		// Don't return this error, as the campaign set was successfully created
	}

	log.Infof("Successfully launched campaign set %s with ID %d", cs.Name, cs.Id)
	return cs, nil
}
