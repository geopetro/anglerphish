package imap

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/gophish/gophish/dialer"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"

	"github.com/jordan-wright/email"
)

// Client interface for IMAP interactions
type Client interface {
	Login(username, password string) (cmd *imap.Command, err error)
	Logout(timeout time.Duration) (cmd *imap.Command, err error)
	Select(name string, readOnly bool) (mbox *imap.MailboxStatus, err error)
	Store(seq *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) (err error)
	Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) (err error)
}

// Email represents an email.Email with an included IMAP Sequence Number
type Email struct {
	SeqNum uint32 `json:"seqnum"`
	// Uid is the server-assigned unique identifier, stable for the lifetime of
	// the mailbox's UidValidity. Requires imap.FetchUid to be requested.
	Uid         uint32 `json:"uid"`
	UidValidity uint32 `json:"uidvalidity"`
	MessageId   string `json:"message_id"`
	*email.Email
}

// Mailbox holds onto the credentials and other information
// needed for connecting to an IMAP server.
type Mailbox struct {
	Host             string
	TLS              bool
	IgnoreCertErrors bool
	User             string
	Pwd              string
	Folder           string
	// Read only mode, false (original logic) if not initialized
	ReadOnly bool
}

// Validate validates supplied IMAP model by connecting to the server
func Validate(s *models.IMAP) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}

	s.Host = s.Host + ":" + strconv.Itoa(int(s.Port)) // Append port
	mailServer := Mailbox{
		Host:             s.Host,
		TLS:              s.TLS,
		IgnoreCertErrors: s.IgnoreCertErrors,
		User:             s.Username,
		Pwd:              s.Password,
		Folder:           s.Folder}

	imapClient, err := mailServer.newClient()
	if err != nil {
		log.Error(err.Error())
	} else {
		imapClient.Logout()
	}
	return err
}

// MarkAsUnread will set the UNSEEN flag on a supplied slice of SeqNums
func (mbox *Mailbox) MarkAsUnread(seqs []uint32) error {
	// Implement a connection retry with backoff
	var imapClient *client.Client
	var err error

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Add exponential backoff between retries
			backoffTime := time.Duration(attempt*attempt) * time.Second
			log.Infof("IMAP connection attempt %d failed, retrying in %v", attempt, backoffTime)
			time.Sleep(backoffTime)
		}

		imapClient, err = mbox.newClient()
		if err == nil {
			break
		}
		log.Errorf("IMAP connection attempt %d failed: %v", attempt+1, err)
	}

	if err != nil {
		return fmt.Errorf("failed to create IMAP connection after %d attempts: %s", maxRetries, err)
	}

	// Make sure to close the connection even if there's a panic
	defer func() {
		// Add a small delay before logout to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
		if imapClient != nil {
			imapClient.Logout()
		}
	}()

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqs...)

	item := imap.FormatFlagsOp(imap.RemoveFlags, true)
	err = imapClient.Store(seqSet, item, imap.SeenFlag, nil)
	if err != nil {
		return err
	}

	return nil

}

// DeleteEmails will delete emails from the supplied slice of SeqNums
func (mbox *Mailbox) DeleteEmails(seqs []uint32) error {
	// Implement a connection retry with backoff
	var imapClient *client.Client
	var err error

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Add exponential backoff between retries
			backoffTime := time.Duration(attempt*attempt) * time.Second
			log.Infof("IMAP connection attempt %d failed, retrying in %v", attempt, backoffTime)
			time.Sleep(backoffTime)
		}

		imapClient, err = mbox.newClient()
		if err == nil {
			break
		}
		log.Errorf("IMAP connection attempt %d failed: %v", attempt+1, err)
	}

	if err != nil {
		return fmt.Errorf("failed to create IMAP connection after %d attempts: %s", maxRetries, err)
	}

	// Make sure to close the connection even if there's a panic
	defer func() {
		// Add a small delay before logout to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
		if imapClient != nil {
			imapClient.Logout()
		}
	}()

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqs...)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	err = imapClient.Store(seqSet, item, imap.DeletedFlag, nil)
	if err != nil {
		return err
	}

	return nil
}

// MarkAsRead will set the SEEN flag on a supplied slice of SeqNums
func (mbox *Mailbox) MarkAsRead(seqs []uint32) error {
	// Implement a connection retry with backoff
	var imapClient *client.Client
	var err error

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Add exponential backoff between retries
			backoffTime := time.Duration(attempt*attempt) * time.Second
			log.Infof("IMAP connection attempt %d failed, retrying in %v", attempt, backoffTime)
			time.Sleep(backoffTime)
		}

		imapClient, err = mbox.newClient()
		if err == nil {
			break
		}
		log.Errorf("IMAP connection attempt %d failed: %v", attempt+1, err)
	}

	if err != nil {
		return fmt.Errorf("failed to create IMAP connection after %d attempts: %s", maxRetries, err)
	}

	// Make sure to close the connection even if there's a panic
	defer func() {
		// Add a small delay before logout to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
		if imapClient != nil {
			imapClient.Logout()
		}
	}()

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqs...)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	err = imapClient.Store(seqSet, item, imap.SeenFlag, nil)
	if err != nil {
		return err
	}

	return nil
}

// GetUnread will find all unread emails in the folder and return them as a list.
func (mbox *Mailbox) GetUnread(markAsRead, delete bool) ([]Email, error) {
	imap.CharsetReader = charset.Reader
	var emails []Email

	// Implement a connection retry with backoff
	var imapClient *client.Client
	var mboxStatus *imap.MailboxStatus
	var err error

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoffTime := time.Duration(attempt*attempt) * time.Second
			log.Infof("IMAP connection attempt %d failed, retrying in %v", attempt, backoffTime)
			time.Sleep(backoffTime)
		}

		imapClient, mboxStatus, err = mbox.newClientWithStatus()
		if err == nil {
			break
		}
		log.Errorf("IMAP connection attempt %d failed: %v", attempt+1, err)
	}

	if err != nil {
		return emails, fmt.Errorf("failed to create IMAP connection after %d attempts: %s", maxRetries, err)
	}

	// Make sure to close the connection even if there's a panic
	defer func() {
		// Add a small delay before logout to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
		if imapClient != nil {
			imapClient.Logout()
		}
	}()

	// Search for unread emails
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	seqs, err := imapClient.Search(criteria)
	if err != nil {
		return emails, err
	}

	if len(seqs) == 0 {
		return emails, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(seqs...)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, imap.FetchUid, section.FetchItem()}
	messages := make(chan *imap.Message)

	go func() {
		if err := imapClient.Fetch(seqset, items, messages); err != nil {
			log.Error("Error fetching emails: ", err.Error()) // TODO: How to handle this, need to propogate error out
		}
	}()

	// Step through each email
	for msg := range messages {
		// Extract raw message body. I can't find a better way to do this with the emersion library
		var em *email.Email
		var buf []byte
		for _, value := range msg.Body {
			buf = make([]byte, value.Len())
			value.Read(buf)
			break // There should only ever be one item in this map, but I'm not 100% sure
		}

		//Remove CR characters, see https://github.com/jordan-wright/email/issues/106
		tmp := string(buf)
		re := regexp.MustCompile(`\r`)
		tmp = re.ReplaceAllString(tmp, "")
		buf = []byte(tmp)

		rawBodyStream := bytes.NewReader(buf)
		em, err = email.NewEmailFromReader(rawBodyStream) // Parse with @jordanwright's library
		if err != nil {
			return emails, err
		}

		var uidValidity uint32
		if mboxStatus != nil {
			uidValidity = mboxStatus.UidValidity
		}
		messageID := ""
		if msg.Envelope != nil {
			messageID = msg.Envelope.MessageId
		}
		emtmp := Email{
			Email:       em,
			SeqNum:      msg.SeqNum,
			Uid:         msg.Uid,
			UidValidity: uidValidity,
			MessageId:   messageID,
		}
		emails = append(emails, emtmp)

	}
	return emails, nil
}

// newClient will initiate a new IMAP connection with the given creds.
func (mbox *Mailbox) newClient() (*client.Client, error) {
	c, _, err := mbox.newClientWithStatus()
	return c, err
}

// newClientWithStatus is newClient, additionally returning the SELECTed
// mailbox status. UIDVALIDITY is only available here — it must be captured at
// SELECT time to be meaningful.
func (mbox *Mailbox) newClientWithStatus() (*client.Client, *imap.MailboxStatus, error) {
	var imapClient *client.Client
	var err error
	restrictedDialer := dialer.Dialer()
	if mbox.TLS {
		config := new(tls.Config)
		config.InsecureSkipVerify = mbox.IgnoreCertErrors
		imapClient, err = client.DialWithDialerTLS(restrictedDialer, mbox.Host, config)
	} else {
		imapClient, err = client.DialWithDialer(restrictedDialer, mbox.Host)
	}
	if err != nil {
		return imapClient, nil, err
	}

	err = imapClient.Login(mbox.User, mbox.Pwd)
	if err != nil {
		return imapClient, nil, err
	}

	status, err := imapClient.Select(mbox.Folder, mbox.ReadOnly)
	if err != nil {
		return imapClient, nil, err
	}

	return imapClient, status, nil
}

// extractIPFromEmail attempts to extract the sender's originating IP address from email headers.
// It parses "Received" headers to find the IP address of the sending mail server.
// Supports both IPv6 and IPv4 addresses. Returns empty string if extraction fails.
func extractIPFromEmail(em *email.Email) string {
	if em == nil {
		return ""
	}

	// Get all "Received" headers - they trace the email's path
	receivedHeaders := em.Headers["Received"]
	if len(receivedHeaders) == 0 {
		log.Debug("No Received headers found in email")
		return ""
	}

	// The first "Received" header typically contains the sender's IP
	// Modern email providers often use IPv6, so we prioritize that
	// Look for patterns like:
	// - "from hostname ([2a02:587:b902:7500:e94d:e65b:4ac5:cf62])"
	// - "from hostname [2a02:587:b902:7500:e94d:e65b:4ac5:cf62]"
	// - "from hostname [1.2.3.4]"
	// - "from hostname (1.2.3.4)"

	// Regex to match IPv6 addresses in square brackets or parentheses
	// IPv6 format: 8 groups of 1-4 hex digits separated by colons
	ipv6Pattern := regexp.MustCompile(`(?:from\s+[^\s]+\s+)?[\[\(]([0-9a-fA-F:]+:+[0-9a-fA-F:]+)[\]\)]`)

	// Regex to match IPv4 addresses in square brackets or parentheses
	ipv4Pattern := regexp.MustCompile(`(?:from\s+[^\s]+\s+)?[\[\(](\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})[\]\)]`)

	// Try to find IPv6 first (many modern providers use IPv6)
	for _, header := range receivedHeaders {
		matches := ipv6Pattern.FindStringSubmatch(header)
		if len(matches) > 1 {
			ip := matches[1]

			// Validate IPv6 format
			if !isValidIPv6Format(ip) {
				continue
			}

			// Skip private/link-local IPv6 addresses
			if isPrivateIPv6(ip) {
				log.Debugf("Skipping private/link-local IPv6 address: %s", ip)
				continue
			}

			log.Debugf("Extracted public IPv6 from email Received header: %s", ip)
			return ip
		}
	}

	// Fall back to IPv4 if no IPv6 found
	for _, header := range receivedHeaders {
		matches := ipv4Pattern.FindStringSubmatch(header)
		if len(matches) > 1 {
			ip := matches[1]

			// Basic validation - check if it's a valid IP format
			if !isValidIPFormat(ip) {
				continue
			}

			// Skip private/internal IP addresses
			if isPrivateIP(ip) {
				log.Debugf("Skipping private IPv4 address: %s", ip)
				continue
			}

			log.Debugf("Extracted public IPv4 from email Received header: %s", ip)
			return ip
		}
	}

	log.Debug("No valid public IP found in Received headers")
	return ""
}

// isValidIPFormat checks if a string is in valid IPv4 format
func isValidIPFormat(ip string) bool {
	parts := regexp.MustCompile(`\.`).Split(ip, -1)
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip string) bool {
	// Check for common private ranges
	if regexp.MustCompile(`^10\.`).MatchString(ip) {
		return true
	}
	if regexp.MustCompile(`^192\.168\.`).MatchString(ip) {
		return true
	}
	if regexp.MustCompile(`^172\.(1[6-9]|2[0-9]|3[0-1])\.`).MatchString(ip) {
		return true
	}
	if regexp.MustCompile(`^127\.`).MatchString(ip) {
		return true
	}
	if regexp.MustCompile(`^169\.254\.`).MatchString(ip) {
		return true
	}

	return false
}

// isValidIPv6Format checks if a string is in valid IPv6 format
func isValidIPv6Format(ip string) bool {
	// IPv6 addresses contain colons and hex digits (0-9, a-f, A-F)
	// Must have at least 2 colons and valid hex digits
	if !regexp.MustCompile(`^[0-9a-fA-F:]+$`).MatchString(ip) {
		return false
	}

	// Count colons - should have at least 2, at most 7
	colonCount := 0
	for _, c := range ip {
		if c == ':' {
			colonCount++
		}
	}

	if colonCount < 2 || colonCount > 7 {
		return false
	}

	// Check for valid double colon :: usage (can only appear once)
	doubleColonCount := 0
	if regexp.MustCompile(`::`).MatchString(ip) {
		doubleColonCount = len(regexp.MustCompile(`::`).FindAllString(ip, -1))
		if doubleColonCount > 1 {
			return false
		}
	}

	// Split by colon and validate each segment
	segments := regexp.MustCompile(`:`).Split(ip, -1)
	for _, segment := range segments {
		// Empty segments are ok (from ::)
		if segment == "" {
			continue
		}

		// Each segment should be 1-4 hex digits
		if len(segment) > 4 {
			return false
		}

		// Verify it's all hex digits
		if !regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(segment) {
			return false
		}
	}

	return true
}

// isPrivateIPv6 checks if an IPv6 address is in a private/link-local range
func isPrivateIPv6(ip string) bool {
	// Convert to lowercase for easier matching
	ipLower := regexp.MustCompile(`[A-F]`).ReplaceAllStringFunc(ip, func(s string) string {
		return regexp.MustCompile(`[A-F]`).ReplaceAllString(s, string(s[0]+32))
	})

	// Link-local addresses: fe80::/10
	if regexp.MustCompile(`^fe[89ab][0-9a-f]:`).MatchString(ipLower) {
		return true
	}

	// Unique local addresses (ULA): fc00::/7 and fd00::/8
	if regexp.MustCompile(`^f[cd][0-9a-f]{2}:`).MatchString(ipLower) {
		return true
	}

	// Loopback: ::1
	if ipLower == "::1" || ipLower == "0:0:0:0:0:0:0:1" {
		return true
	}

	// Unspecified address: ::
	if ipLower == "::" || ipLower == "0:0:0:0:0:0:0:0" {
		return true
	}

	return false
}
