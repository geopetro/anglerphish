package dmi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Error struct {
	Err        error
	Temp       bool
	RetryAfter time.Duration
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Retryable is matched by the mail worker via errors.As.
func (e *Error) Retryable() bool { return e.Temp }

// RetryDelay is the provider's Retry-After hint, zero when none was given.
func (e *Error) RetryDelay() time.Duration { return e.RetryAfter }

func classifyHTTP(status int, body []byte, retryAfter string) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 300 {
		snippet = snippet[:300]
	}
	e := &Error{}
	defer func() {
		if n, err := strconv.Atoi(strings.TrimSpace(retryAfter)); e.Temp && err == nil && n > 0 {
			e.RetryAfter = time.Duration(n) * time.Second
		}
	}()
	switch {
	case status == 429:
		e.Temp = true
		e.Err = fmt.Errorf("rate limited (HTTP 429): %s", snippet)
	case status >= 500:
		e.Temp = true
		e.Err = fmt.Errorf("provider error (HTTP %d): %s", status, snippet)
	case status == 401:
		e.Err = fmt.Errorf("authentication rejected (HTTP 401): %s", snippet)
	case status == 403:
		e.Err = fmt.Errorf("missing permission (HTTP 403): %s", snippet)
	case status == 404:
		e.Err = fmt.Errorf("mailbox or folder not found (HTTP 404): %s", snippet)
	default:
		e.Err = fmt.Errorf("request rejected (HTTP %d): %s", status, snippet)
	}
	return e
}
