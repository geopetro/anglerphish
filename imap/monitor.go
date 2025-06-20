package imap

// This file implements the IMAP monitor functionality for Gophish
// It monitors IMAP accounts for reported phishing emails and processes them
import (
	"bytes"
	"context"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jordan-wright/email"

	"github.com/gophish/gophish/models"
)

// Map of parameter names to their regexes
var paramRegexes = make(map[string]*regexp.Regexp)
var lastRegexUpdate time.Time
var lastCampaignChangeTime time.Time
var regexUpdateMutex sync.Mutex

// Default regex for backward compatibility
var goPhishRegex = regexp.MustCompile("((\\?|%3F)rid(=|%3D)(3D)?([A-Za-z0-9]{7}))")

// updateParameterRegexes updates the map of parameter regexes based on active campaigns
func updateParameterRegexes() {
	// Clear any existing regexes to avoid outdated patterns
	paramRegexes = make(map[string]*regexp.Regexp)

	// Get all active campaigns
	campaigns, err := models.GetActiveCampaigns()
	if err != nil {
		log.Error(err)
		return
	}

	// If there are no active campaigns, don't add any regexes
	if len(campaigns) == 0 {
		log.Info("No active campaigns found. Using default regex pattern.")
		paramRegexes["rid"] = goPhishRegex
		return
	}

	// Create regex patterns for the "rid" parameter if it's used in any campaign
	// Use a pattern that looks for URL parameters in various formats
	paramRegexes["rid"] = regexp.MustCompile(`(?i)((\?|%3F|\s\?|\s%3F)rid(=|%3D|\s=|\s%3D)(3D)?([A-Za-z0-9]+))`)

	// Create a direct pattern for the default parameter too (used when URL encoding is broken)
	paramRegexes["rid_direct"] = regexp.MustCompile(`(?i)rid\s*?[=:]\s*?([A-Za-z0-9]+)`)

	// Track URL parameters we've seen to avoid duplicates
	seenParams := make(map[string]bool)
	seenParams["rid"] = true

	// Add regex for each unique parameter name
	for _, c := range campaigns {
		// Use URLParam instead of URL for the parameter name
		// If URLParam is empty, skip this campaign
		if c.URLParam == "" || seenParams[c.URLParam] {
			continue
		}

		seenParams[c.URLParam] = true

		// Create a pattern for this parameter - using the same format as the rid parameter
		pattern := fmt.Sprintf(`(?i)((\?|%%3F|\s\?|\s%%3F)%s(=|%%3D|\s=|\s%%3D)(3D)?([A-Za-z0-9]+))`, c.URLParam)
		paramRegexes[c.URLParam] = regexp.MustCompile(pattern)

		// Also add a direct pattern for this parameter
		directPattern := fmt.Sprintf(`(?i)%s\s*?[=:]\s*?([A-Za-z0-9]+)`, c.URLParam)
		paramRegexes[c.URLParam+"_direct"] = regexp.MustCompile(directPattern)

		log.Infof("Added regexes for campaign parameter: %s", c.URLParam)
	}

	// Log all the available parameter patterns for debugging
	if len(paramRegexes) > 0 {
		log.Infof("Active campaign detection parameters: %v", getMapKeys(paramRegexes))
	}
}

// getMapKeys returns a slice of keys from the given map
func getMapKeys(m map[string]*regexp.Regexp) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// shouldUpdateRegexes checks if we need to update the regex patterns
func shouldUpdateRegexes() bool {
	// Get all active campaigns
	campaigns, err := models.GetActiveCampaigns()
	if err != nil {
		// If we can't get campaigns, default to updating
		log.Debugf("Couldn't get active campaigns: %v - will update regexes", err)
		return true
	}

	// Note: We track the latest campaign creation date for detecting changes
	// This is more reliable than counting campaigns since parameters can change too

	// Look for the most recent campaign creation date
	var latestDate time.Time
	for _, campaign := range campaigns {
		if campaign.CreatedDate.After(latestDate) {
			latestDate = campaign.CreatedDate
		}
	}

	// If we found a campaign created after our last check, update
	if !latestDate.IsZero() && latestDate.After(lastCampaignChangeTime) {
		log.Debugf("New campaign detected - created at %v, last update at %v",
			latestDate, lastCampaignChangeTime)
		lastCampaignChangeTime = latestDate
		return true
	}

	// Also update if it's been more than 5 minutes since the last regex update
	if time.Since(lastRegexUpdate) > 5*time.Minute {
		log.Debugf("Regular 5-minute refresh of regexes")
		return true
	}

	return false
}

// Monitor is a worker that monitors IMAP servers for reported campaign emails
type Monitor struct {
	cancel func()
}

// Monitor.start() checks for campaign emails
// As each account can have its own polling frequency set we need to run one Go routine for
// each, as well as keeping an eye on newly created user accounts.
func (im *Monitor) start(ctx context.Context) {
	usermap := make(map[int64]int) // Keep track of running go routines, one per user. We assume incrementing non-repeating UIDs (for the case where users are deleted and re-added).

	for {
		select {
		case <-ctx.Done():
			return
		default:
			dbusers, err := models.GetUsers() //Slice of all user ids. Each user gets their own IMAP monitor routine.
			if err != nil {
				log.Error(err)
				break
			}
			for _, dbuser := range dbusers {
				if _, ok := usermap[dbuser.Id]; !ok { // If we don't currently have a running Go routine for this user, start one.
					log.Info("Starting new IMAP monitor for user ", dbuser.Username)
					usermap[dbuser.Id] = 1
					go monitor(dbuser.Id, ctx)
				}
			}
			time.Sleep(10 * time.Second) // Every ten seconds we check if a new user has been created
		}
	}
}

// monitor will continuously login to the IMAP settings associated to the supplied user id (if the user account has IMAP settings, and they're enabled.)
// It also verifies the user account exists, and returns if not (for the case of a user being deleted).
// minTimeBetweenChecks is the minimum time to wait between IMAP checks in seconds
// This helps prevent overwhelming the IMAP server with too many connections
const minTimeBetweenChecks = 10

// Map to track last check time for each IMAP configuration
var lastCheckTimes = make(map[int64]time.Time)
var lastConfigUpdateTimes = make(map[int64]time.Time) // Track when config was last modified
var lastCheckMutex sync.Mutex

func monitor(uid int64, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 1. Check if user exists, if not, return.
			_, err := models.GetUser(uid)
			if err != nil { // Not sure if there's a better way to determine user existence via id.
				log.Info("User ", uid, " seems to have been deleted. Stopping IMAP monitor for this user.")
				return
			}
			// 2. Check if user has IMAP settings.
			imapSettings, err := models.GetIMAP(uid)
			if err != nil {
				log.Error(err)
				break
			}
			if len(imapSettings) > 0 {
				im := imapSettings[0]
				// 3. Check if IMAP is enabled
				if im.Enabled {
					monitorIMAPConfig(im, ctx)
				}
			}
			time.Sleep(10 * time.Second)
		}
	}
}

// monitorIMAPConfig handles checking a single IMAP configuration
func monitorIMAPConfig(im models.IMAP, ctx context.Context) {
	// Get the last check time for this specific IMAP configuration
	lastCheckMutex.Lock()
	lastCheckTime, exists := lastCheckTimes[im.Id]
	if !exists {
		// If this is the first time we're checking this config, set it to 1 hour ago
		lastCheckTime = time.Now().Add(-1 * time.Hour)
	}

	// Check if this configuration was recently updated
	lastUpdateTime, exists := lastConfigUpdateTimes[im.Id]
	configUpdated := exists && im.ModifiedDate.After(lastUpdateTime)
	if configUpdated {
		log.Infof("IMAP config was updated. Applying new settings.")
		lastConfigUpdateTimes[im.Id] = im.ModifiedDate
	} else if !exists {
		// Initialize the update time tracking
		lastConfigUpdateTimes[im.Id] = im.ModifiedDate
	}
	lastCheckMutex.Unlock()

	// Ensure we respect the minimum time between checks
	timeSinceLastCheck := time.Since(lastCheckTime)
	minWaitTime := time.Duration(minTimeBetweenChecks) * time.Second

	if timeSinceLastCheck < minWaitTime {
		// If we checked too recently, wait until the minimum time has passed
		waitDuration := minWaitTime - timeSinceLastCheck
		log.Debugf("Waiting %v to avoid overloading IMAP server", waitDuration)
		time.Sleep(waitDuration)
	}

	startTime := time.Now()

	// Update the last check time for this configuration
	lastCheckMutex.Lock()
	lastCheckTimes[im.Id] = startTime
	lastCheckMutex.Unlock()

	log.Debug("Checking IMAP for user ", im.UserId, ": ", im.Username, " -> ", im.Host)
	checkForNewEmails(im)

	// Calculate the proper sleep time to maintain the desired frequency
	// This accounts for how long the check operation took
	elapsed := time.Since(startTime)
	sleepDuration := time.Duration(im.IMAPFreq)*time.Second - elapsed

	// Ensure we don't sleep for a negative duration
	if sleepDuration > 0 {
		log.Debugf("IMAP check took %v, sleeping for %v to maintain %d second frequency",
			elapsed, sleepDuration, im.IMAPFreq)
		time.Sleep(sleepDuration)
	} else {
		log.Infof("IMAP check took %v, which exceeds configured frequency of %d seconds",
			elapsed, im.IMAPFreq)
	}
}

// NewMonitor returns a new instance of imap.Monitor
func NewMonitor() *Monitor {
	im := &Monitor{}
	// Initialize parameter regexes
	updateParameterRegexes()
	lastRegexUpdate = time.Now()
	return im
}

// Start launches the IMAP campaign monitor
func (im *Monitor) Start() error {
	log.Info("Starting IMAP monitor manager")
	ctx, cancel := context.WithCancel(context.Background()) // ctx is the derivedContext
	im.cancel = cancel
	go im.start(ctx)
	return nil
}

// Shutdown attempts to gracefully shutdown the IMAP monitor.
func (im *Monitor) Shutdown() error {
	log.Info("Shutting down IMAP monitor manager")
	im.cancel()
	return nil
}

// checkForNewEmails logs into an IMAP account and checks unread emails
// for the rid campaign identifier.
func checkForNewEmails(im models.IMAP) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic in IMAP monitor: %v", r)
		}
	}()

	// Check if we need to update the regexes based on new campaigns
	regexUpdateMutex.Lock()
	if shouldUpdateRegexes() {
		log.Debugf("New campaigns detected, updating parameter regexes")
		updateParameterRegexes()
		lastRegexUpdate = time.Now()
	}
	regexUpdateMutex.Unlock()

	im.Host = im.Host + ":" + strconv.Itoa(int(im.Port)) // Append port
	mailServer := Mailbox{
		Host:             im.Host,
		TLS:              im.TLS,
		IgnoreCertErrors: im.IgnoreCertErrors,
		User:             im.Username,
		Pwd:              im.Password,
		Folder:           im.Folder,
	}

	msgs, err := mailServer.GetUnread(true, false)
	if err != nil {
		log.Error(err)
		// Record the login failure
		if err := im.RecordLoginFailure(); err != nil {
			log.Errorf("Failed to record login failure: %v", err)
		}

		// Calculate backoff time - exponential with maximum of 30 minutes
		backoffSeconds := 60 // Default 1 minute backoff for first failure
		if im.LoginFailures > 1 {
			// Use exponential backoff: 2^failures seconds, up to 30 minutes
			maxBackoff := 1800.0 // 30 minutes in seconds
			backoffSeconds = int(math.Min(maxBackoff, math.Pow(2, float64(im.LoginFailures))))
		}

		log.Infof("IMAP login failed %d times. Backing off for %d seconds before next attempt",
			im.LoginFailures, backoffSeconds)
		return
	}

	// Update last_successful_login here via im.Host
	if err := models.SuccessfulLogin(&im); err != nil {
		log.Error("Failed to update last successful login: ", err)
	}

	if len(msgs) > 0 {
		log.Debugf("%d new emails for %s", len(msgs), im.Username)
		var reportingFailed []uint32 // SeqNums of emails that were unable to be reported to phishing server, mark as unread
		var deleteEmails []uint32    // SeqNums of campaign emails. If DeleteReportedCampaignEmail is true, we will delete these
		var processedEmails []uint32 // SeqNums of all emails we processed but don't need to delete

		// Process all emails regardless of whether there are active campaigns
		regexUpdateMutex.Lock()
		regexUpdateMutex.Unlock()

		for _, m := range msgs {
			// Track this email as processed by default unless we decide to delete it
			processedEmails = append(processedEmails, m.SeqNum)

			// Check if sender is from company's domain, if enabled
			if im.RestrictDomain != "" { // e.g domainResitct = widgets.com
				splitEmail := strings.Split(m.Email.From, "@")
				if len(splitEmail) < 2 {
					log.Debugf("Invalid email format from '%s', skipping domain check", m.Email.From)
					continue
				}

				senderDomain := splitEmail[len(splitEmail)-1]
				if senderDomain != im.RestrictDomain {
					log.Debug("Ignoring email as not from company domain: ", senderDomain)
					continue
				}
			}

			rids, err := matchEmail(m.Email) // Search email Text, HTML, and each attachment for rid parameters

			if err != nil {
				log.Errorf("Error searching email for rids from user '%s': %s", m.Email.From, err.Error())
				continue
			}

			if len(rids) < 1 {
				// This is a non-campaign email report - record it in the database
				log.Infof("User '%s' reported email with subject '%s'. This is not a GoPhish campaign; you should investigate it.", m.Email.From, m.Email.Subject)

				// Record the non-campaign report
				err := models.RecordNonCampaignReport(
					im.UserId,
					im.Id,
					m.Email.From,
					m.Email.Subject,
				)

				if err != nil {
					log.Errorf("Failed to record non-campaign report: %v", err)
				}

				// Never delete non-campaign reports - these should always be preserved for investigation
				continue
			}

			// Process campaign emails
			validRIDFound := false
			for rid := range rids {
				log.Infof("User '%s' reported email with rid %s", m.Email.From, rid)
				result, err := models.GetResult(rid)
				if err != nil {
					log.Error("Error processing email with rid ", rid, ": ", err.Error())
					reportingFailed = append(reportingFailed, m.SeqNum)
					continue
				}

				// Successfully found a campaign - mark email for deletion if configured
				validRIDFound = true

				err = result.HandleEmailReport(models.EventDetails{})
				if err != nil {
					log.Error("Error updating email status with rid ", rid, ": ", err.Error())
					continue
				}
			}

			// If any valid RID was found and delete is enabled, mark for deletion
			if validRIDFound && im.DeleteReportedCampaignEmail {
				// Remove from processed list if we're going to delete it
				for i, seq := range processedEmails {
					if seq == m.SeqNum {
						processedEmails = append(processedEmails[:i], processedEmails[i+1:]...)
						break
					}
				}
				deleteEmails = append(deleteEmails, m.SeqNum)
			}
		}

		// Process emails that failed to report
		if len(reportingFailed) > 0 {
			log.Debugf("Marking %d emails as unread as failed to report", len(reportingFailed))
			err := mailServer.MarkAsUnread(reportingFailed) // Set emails as unread that we failed to report to GoPhish
			if err != nil {
				log.Error("Unable to mark emails as unread: ", err.Error())
			}

			// Remove failed emails from processed list
			for _, failedSeq := range reportingFailed {
				for i, seq := range processedEmails {
					if seq == failedSeq {
						processedEmails = append(processedEmails[:i], processedEmails[i+1:]...)
						break
					}
				}
			}
		}

		// Process emails to delete
		if len(deleteEmails) > 0 {
			log.Debugf("Deleting %d campaign emails", len(deleteEmails))
			err := mailServer.DeleteEmails(deleteEmails) // Delete GoPhish campaign emails.
			if err != nil {
				log.Error("Failed to delete emails: ", err.Error())
			}
		}

		// Mark remaining emails as read to prevent reprocessing
		if len(processedEmails) > 0 {
			log.Debugf("Marking %d emails as read", len(processedEmails))
			err := mailServer.MarkAsRead(processedEmails)
			if err != nil {
				log.Error("Failed to mark emails as read: ", err.Error())
			}
		}
	} else {
		log.Debug("No new emails for ", im.Username)
	}
}

func checkRIDs(em *email.Email, rids map[string]bool) {
	// Check Text and HTML
	emailContent := string(em.Text) + string(em.HTML)

	// Only proceed with pattern matching if we have active campaign parameters to look for
	regexUpdateMutex.Lock()
	paramCount := len(paramRegexes)
	regexUpdateMutex.Unlock()

	if paramCount == 0 {
		log.Debug("No active campaign parameters to check. Using default regex.")
		// Use the default regex as fallback
		for _, r := range goPhishRegex.FindAllStringSubmatch(emailContent, -1) {
			newrid := r[len(r)-1]
			if !rids[newrid] {
				rids[newrid] = true
			}
		}
		return
	}

	// Process each potential campaign ID match
	regexUpdateMutex.Lock()
	for paramName, regex := range paramRegexes {
		// Skip "_direct" variants for logging clarity
		if strings.HasSuffix(paramName, "_direct") {
			continue
		}

		// Try standard pattern first
		for _, r := range regex.FindAllStringSubmatch(emailContent, -1) {
			newrid := r[len(r)-1]

			// Skip if we've already found this RID
			if rids[newrid] {
				continue
			}

			// Verify this RID exists in the database before adding it
			_, err := models.GetResult(newrid)
			if err != nil {
				log.Debugf("Potential campaign ID %s not found in database: %v", newrid, err)
				continue
			}

			// It's a valid campaign ID
			rids[newrid] = true
			log.Infof("Found valid campaign ID %s using parameter %s", newrid, paramName)
		}

		// Now try the direct pattern variant
		directPattern := paramName + "_direct"
		directRegex, exists := paramRegexes[directPattern]
		if exists {
			for _, r := range directRegex.FindAllStringSubmatch(emailContent, -1) {
				newrid := r[1] // The direct regex only has one capture group

				// Skip if we've already found this RID
				if rids[newrid] {
					continue
				}

				// Verify this RID exists in the database before adding it
				_, err := models.GetResult(newrid)
				if err != nil {
					log.Debugf("Potential campaign ID %s not found in database: %v", newrid, err)
					continue
				}

				// It's a valid campaign ID
				rids[newrid] = true
				log.Infof("Found valid campaign ID %s using direct pattern %s", newrid, paramName)
			}
		}
	}
	regexUpdateMutex.Unlock()
}

// returns a slice of gophish rid paramters found in the email HTML, Text, and attachments
func matchEmail(em *email.Email) (map[string]bool, error) {
	rids := make(map[string]bool)
	checkRIDs(em, rids)

	// Next check each attachment
	for _, a := range em.Attachments {
		ext := filepath.Ext(a.Filename)
		if a.Header.Get("Content-Type") == "message/rfc822" || ext == ".eml" {

			// Let's decode the email
			rawBodyStream := bytes.NewReader(a.Content)
			attachmentEmail, err := email.NewEmailFromReader(rawBodyStream)
			if err != nil {
				return rids, err
			}

			checkRIDs(attachmentEmail, rids)
		}
	}

	return rids, nil
}
