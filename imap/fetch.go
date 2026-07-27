package imap

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/charset"
	log "github.com/gophish/gophish/logger"
	"github.com/jordan-wright/email"
)

// ErrMessageNotFound indicates the message is no longer in the mailbox.
var ErrMessageNotFound = errors.New("message not found in mailbox")

// ErrMessageAmbiguous indicates the Message-ID fallback matched more than one
// message. Message-ID is attacker-controlled and can be forged to collide, so
// we refuse to guess which message was meant.
var ErrMessageAmbiguous = errors.New("could not uniquely identify message")

// ErrMailboxRecreated indicates UIDVALIDITY changed, invalidating stored UIDs.
var ErrMailboxRecreated = errors.New("mailbox was recreated; stored identifier is no longer valid")

// HeaderPair is a single raw header, preserved in order.
type HeaderPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FetchedMessage is a single message retrieved on demand.
type FetchedMessage struct {
	Subject string       `json:"subject"`
	From    string       `json:"from"`
	Date    string       `json:"date"`
	Text    string       `json:"text"`
	HTML    string       `json:"html"`
	Headers []HeaderPair `json:"headers"`
}

// shouldUseUID reports whether the stored UID is still a valid handle.
// A UID is only meaningful within the UIDVALIDITY it was recorded under.
func shouldUseUID(uid, storedValidity, currentValidity uint32) bool {
	if uid == 0 || storedValidity == 0 {
		return false
	}
	return storedValidity == currentValidity
}

// resolveSearchResults turns a Message-ID search result into a single UID.
func resolveSearchResults(uids []uint32) (uint32, error) {
	switch len(uids) {
	case 0:
		return 0, ErrMessageNotFound
	case 1:
		return uids[0], nil
	default:
		return 0, ErrMessageAmbiguous
	}
}

// FetchMessage retrieves a single message by UID, falling back to a Message-ID
// search when the UID is unusable.
//
// The connection is opened read-only so that viewing a report never sets the
// \Seen flag on unread mail the monitor has not processed yet.
func (mbox *Mailbox) FetchMessage(uid uint32, uidValidity uint32, messageID string) (*FetchedMessage, error) {
	imap.CharsetReader = charset.Reader

	readOnlyBox := *mbox
	readOnlyBox.ReadOnly = true

	imapClient, status, err := readOnlyBox.newClientWithStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %s", err)
	}
	defer func() {
		time.Sleep(100 * time.Millisecond)
		if imapClient != nil {
			imapClient.Logout()
		}
	}()

	targetUID := uid
	if !shouldUseUID(uid, uidValidity, status.UidValidity) {
		if messageID == "" {
			if uid != 0 && uidValidity != status.UidValidity {
				return nil, ErrMailboxRecreated
			}
			return nil, ErrMessageNotFound
		}
		criteria := imap.NewSearchCriteria()
		criteria.Header.Add("Message-Id", messageID)
		found, err := imapClient.UidSearch(criteria)
		if err != nil {
			return nil, fmt.Errorf("message-id search failed: %s", err)
		}
		targetUID, err = resolveSearchResults(found)
		if err != nil {
			return nil, err
		}
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(targetUID)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, section.FetchItem()}
	messages := make(chan *imap.Message, 1)

	go func() {
		if ferr := imapClient.UidFetch(seqset, items, messages); ferr != nil {
			log.Errorf("Error fetching message by UID: %v", ferr)
		}
	}()

	var parsed *email.Email
	for msg := range messages {
		var buf []byte
		for _, value := range msg.Body {
			buf = make([]byte, value.Len())
			value.Read(buf)
			break
		}
		if len(buf) == 0 {
			continue
		}
		// Remove CR characters, see jordan-wright/email#106
		cleaned := regexp.MustCompile(`\r`).ReplaceAllString(string(buf), "")
		parsed, err = email.NewEmailFromReader(bytes.NewReader([]byte(cleaned)))
		if err != nil {
			return nil, fmt.Errorf("failed to parse message: %s", err)
		}
	}

	if parsed == nil {
		return nil, ErrMessageNotFound
	}

	headers := make([]HeaderPair, 0, len(parsed.Headers))
	for name, values := range parsed.Headers {
		for _, v := range values {
			headers = append(headers, HeaderPair{Name: name, Value: v})
		}
	}

	return &FetchedMessage{
		Subject: parsed.Subject,
		From:    parsed.From,
		Date:    parsed.Headers.Get("Date"),
		Text:    string(parsed.Text),
		HTML:    string(parsed.HTML),
		Headers: headers,
	}, nil
}
