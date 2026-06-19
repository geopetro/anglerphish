package models

import (
	"bytes"
	"errors"
	"math"
	"text/template"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
)

// SMSLog is a struct that holds information about an SMS that is to be
// sent out.
type SMSLog struct {
	Id          int64     `json:"-"`
	UserId      int64     `json:"-"`
	CampaignId  int64     `json:"campaign_id"`
	RId         string    `json:"id"`
	SendDate    time.Time `json:"send_date"`
	SendAttempt int       `json:"send_attempt"`
	Processing  bool      `json:"-"`
	Target      string    `json:"target"`

	cachedCampaign *Campaign
}

// TableName specifies the database tablename for Gorm to use
func (s SMSLog) TableName() string {
	return "sms_logs"
}

// Backoff sets the SMSLog SendDate to be the next entry in an exponential
// backoff. ErrMaxSendAttempts is thrown if this smslog has been retried
// too many times. Backoff also unlocks the smslog so that it can be processed
// again in the future.
func (s *SMSLog) Backoff(reason error) error {
	r, err := GetResult(s.RId)
	if err != nil {
		return err
	}
	if s.SendAttempt == MaxSendAttempts {
		r.HandleSMSError(ErrMaxSendAttempts)
		return ErrMaxSendAttempts
	}
	// Add an error, since we had to backoff because of a
	// temporary error of some sort during the SMS transaction
	s.SendAttempt++
	backoffDuration := math.Pow(2, float64(s.SendAttempt))
	s.SendDate = s.SendDate.Add(time.Minute * time.Duration(backoffDuration))
	err = db.Save(s).Error
	if err != nil {
		return err
	}
	err = r.HandleSMSBackoff(reason, s.SendDate)
	if err != nil {
		return err
	}
	err = s.Unlock()
	return err
}

// Unlock removes the processing flag so the smslog can be processed again
func (s *SMSLog) Unlock() error {
	s.Processing = false
	return db.Save(&s).Error
}

// Lock sets the processing flag so that other processes cannot modify the smslog
func (s *SMSLog) Lock() error {
	s.Processing = true
	return db.Save(&s).Error
}

// Error sets the error status on the models.Result that the
// smslog refers to. Since SMSLog errors are permanent,
// this action also deletes the smslog.
func (s *SMSLog) Error(e error) error {
	r, err := GetResult(s.RId)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = r.HandleSMSError(e)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = db.Delete(s).Error
	return err
}

// Success deletes the smslog from the database and updates the underlying
// campaign result.
func (s *SMSLog) Success() error {
	r, err := GetResult(s.RId)
	if err != nil {
		return err
	}
	err = r.HandleSMSSent()
	if err != nil {
		return err
	}
	err = db.Delete(s).Error
	return err
}

// CacheCampaign allows bulk-sms workers to cache the otherwise expensive
// campaign lookup operation by providing a pointer to the campaign here.
func (s *SMSLog) CacheCampaign(campaign *Campaign) error {
	if campaign.Id != s.CampaignId {
		return ErrInvalidCampaignID
	}
	s.cachedCampaign = campaign
	return nil
}

// Generate returns the generated SMS message as a string
func (s *SMSLog) Generate() (string, error) {
	campaign, err := s.getCampaign()
	if err != nil {
		return "", err
	}

	result, err := s.getResult()
	if err != nil {
		return "", err
	}

	recipientForTemplate := result.BaseRecipient
	if gv, gvErr := GetGlobalVariables(s.UserId); gvErr == nil {
		gv.ApplyTo(&recipientForTemplate)
	}
	// Create the SMS template context with proper URL formatting
	stc, err := NewSMSTemplateContext(&campaign, recipientForTemplate, s.RId)
	if err != nil {
		return "", err
	}

	// Parse the SMS template
	tmpl, err := template.New("sms_template").Parse(campaign.SMSTemplate.Text)
	if err != nil {
		return "", err
	}

	// Execute the template
	var msg bytes.Buffer
	err = tmpl.Execute(&msg, stc)
	if err != nil {
		return "", err
	}

	return msg.String(), nil
}

// GetDialer returns a mailer.SMSDialer based on the SMS profile
func (s *SMSLog) GetDialer() (mailer.SMSDialer, error) {
	campaign, err := s.getCampaign()
	if err != nil {
		return nil, err
	}

	dialer, err := mailer.GetSMSDialer(campaign.SMS.Provider, campaign.SMS.ProviderConfig)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return dialer, nil
}

// GetSMSFrom returns the SMS sender's phone number
func (s *SMSLog) GetSMSFrom() (string, error) {
	campaign, err := s.getCampaign()
	if err != nil {
		return "", err
	}

	// Check if SMS template has a From override
	if campaign.SMSTemplate.From != "" {
		return campaign.SMSTemplate.From, nil
	}

	// Fall back to SMS Profile's From
	return campaign.SMS.From, nil
}

// GetSMSTo returns the recipient's phone number
func (s *SMSLog) GetSMSTo() (string, error) {
	result, err := s.getResult()
	if err != nil {
		return "", err
	}

	// Always use the Phone field for phone numbers
	return result.Phone, nil
}

// getCampaign returns the campaign associated with the SMSLog
func (s *SMSLog) getCampaign() (Campaign, error) {
	if s.cachedCampaign != nil {
		return *s.cachedCampaign, nil
	}

	campaign, err := GetCampaignSMSContext(s.CampaignId, s.UserId)
	if err != nil {
		return campaign, err
	}

	s.cachedCampaign = &campaign
	return campaign, nil
}

// getResult returns the Result associated with the SMSLog
func (s *SMSLog) getResult() (Result, error) {
	r := Result{}
	err := db.Where("r_id = ?", s.RId).First(&r).Error
	return r, err
}

// GetQueuedSMSLogs returns the SMS logs that are queued up for the given minute.
func GetQueuedSMSLogs(t time.Time) ([]*SMSLog, error) {
	ss := []*SMSLog{}
	err := db.Where("send_date <= ? AND processing = ?", t, false).
		Find(&ss).Error
	if err != nil {
		log.Warn(err)
	}
	return ss, err
}

// GetSMSLogsByCampaign returns all of the SMS logs for a given campaign.
func GetSMSLogsByCampaign(cid int64) ([]*SMSLog, error) {
	ss := []*SMSLog{}
	err := db.Where("campaign_id = ?", cid).Find(&ss).Error
	return ss, err
}

// LockSMSLogs locks or unlocks a slice of smslogs for processing.
func LockSMSLogs(ss []*SMSLog, lock bool) error {
	tx := db.Begin()
	for i := range ss {
		ss[i].Processing = lock
		err := tx.Save(ss[i]).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

// UnlockAllSMSLogs removes the processing lock for all smslogs
// in the database. This is intended to be called when Gophish is started
// so that any previously locked smslogs can resume processing.
func UnlockAllSMSLogs() error {
	return db.Model(&SMSLog{}).Update("processing", false).Error
}

// ResendFailedSMSInCampaign re-queues all results in Error or Retrying status
// for the given SMS campaign by replacing existing SMSLog rows and resetting
// their status to StatusSending. Returns the number of messages re-queued.
func ResendFailedSMSInCampaign(cid, uid int64) (int, error) {
	c, err := GetCampaign(cid, uid)
	if err != nil {
		return 0, err
	}
	if c.Status != CampaignInProgress {
		return 0, errors.New("campaign must be In Progress to resend messages")
	}
	if c.Type != "sms" {
		return 0, errors.New("resend is only available for SMS campaigns")
	}

	var results []Result
	if err := db.Where("campaign_id = ? AND (status = ? OR status = ?)", cid, Error, StatusRetry).Find(&results).Error; err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}

	tx := db.Begin()
	count := 0
	for _, r := range results {
		// If an SMSLog already exists (Retrying case), delete it so we can re-queue immediately.
		var existing SMSLog
		if err := tx.Where("r_id = ?", r.RId).First(&existing).Error; err == nil {
			if err := tx.Delete(&existing).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}

		s := SMSLog{
			UserId:      uid,
			CampaignId:  cid,
			RId:         r.RId,
			SendDate:    time.Now().UTC(),
			SendAttempt: 0,
			Processing:  false,
		}
		if err := tx.Save(&s).Error; err != nil {
			tx.Rollback()
			return 0, err
		}

		r.Status = StatusSending
		r.ModifiedDate = time.Now().UTC()
		if err := tx.Save(&r).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		count++
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	if err := AddEvent(&Event{Message: "Failed SMS Re-queued"}, cid); err != nil {
		log.Error(err)
	}

	return count, nil
}

// ResendFailedSMS re-queues a single failed SMS result (identified by rid)
// within the given campaign for re-sending. Works for both Error and Retrying results.
func ResendFailedSMS(cid int64, rid string, uid int64) error {
	c, err := GetCampaign(cid, uid)
	if err != nil {
		return err
	}
	if c.Status != CampaignInProgress {
		return errors.New("campaign must be In Progress to resend messages")
	}
	if c.Type != "sms" {
		return errors.New("resend is only available for SMS campaigns")
	}

	var r Result
	if err := db.Where("r_id = ? AND campaign_id = ?", rid, cid).First(&r).Error; err != nil {
		return err
	}
	if r.Status != Error && r.Status != StatusRetry {
		return errors.New("result is not in a failed state")
	}

	tx := db.Begin()

	// If an SMSLog already exists (Retrying case), delete it so we can re-queue immediately.
	var existing SMSLog
	if err := tx.Where("r_id = ?", rid).First(&existing).Error; err == nil {
		if err := tx.Delete(&existing).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	s := SMSLog{
		UserId:      uid,
		CampaignId:  cid,
		RId:         r.RId,
		SendDate:    time.Now().UTC(),
		SendAttempt: 0,
		Processing:  false,
	}
	if err := tx.Save(&s).Error; err != nil {
		tx.Rollback()
		return err
	}

	r.Status = StatusSending
	r.ModifiedDate = time.Now().UTC()
	if err := tx.Save(&r).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	if err := AddEvent(&Event{Message: "SMS Re-queued", Email: r.Phone}, cid); err != nil {
		log.Error(err)
	}

	return nil
}
