package imap

import (
	"errors"
	"net/mail"
	"strings"
)

// ErrNoSenderDomain indicates a From header we could not extract a domain from.
var ErrNoSenderDomain = errors.New("could not determine sender domain")

// ParseSenderAddress extracts the bare address from a raw From header.
//
// The email library assigns the *unparsed* header to Email.From, so this value
// is commonly of the form `Jane Doe <jane@corp.com>` rather than a bare
// address. Splitting such a value on "@" yields a domain with a trailing ">".
func ParseSenderAddress(from string) (string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", ErrNoSenderDomain
	}
	if addr, err := mail.ParseAddress(from); err == nil {
		return addr.Address, nil
	}
	// Malformed headers (e.g. an unclosed angle bracket) still carry a usable
	// address often enough to be worth recovering rather than discarding.
	at := strings.LastIndex(from, "@")
	if at < 0 || at == len(from)-1 {
		return "", ErrNoSenderDomain
	}
	local := strings.Trim(from[:at], "<>() \t\"'")
	if i := strings.LastIndexAny(local, " \t"); i >= 0 {
		local = local[i+1:]
	}
	local = strings.Trim(local, "<>() \t\"'")
	domain := strings.Trim(from[at+1:], "<>() \t\"'")
	if local == "" || domain == "" {
		return "", ErrNoSenderDomain
	}
	return local + "@" + domain, nil
}

// SenderDomain returns the lowercased domain portion of a raw From header.
func SenderDomain(from string) (string, error) {
	addr, err := ParseSenderAddress(from)
	if err != nil {
		return "", err
	}
	at := strings.LastIndex(addr, "@")
	if at < 0 || at == len(addr)-1 {
		return "", ErrNoSenderDomain
	}
	return strings.ToLower(addr[at+1:]), nil
}

// DomainMatches reports whether the sender is within restrictDomain.
// Comparison is case-insensitive because domain names are.
func DomainMatches(from, restrictDomain string) bool {
	domain, err := SenderDomain(from)
	if err != nil {
		return false
	}
	return domain == strings.ToLower(strings.TrimSpace(restrictDomain))
}
