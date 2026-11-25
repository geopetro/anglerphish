package dmi

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClassifyHTTP(t *testing.T) {
	var de *Error
	err := classifyHTTP(429, []byte("slow"), "7")
	if !errors.As(err, &de) || !de.Temp || de.RetryAfter != 7*time.Second || !de.Retryable() {
		t.Fatalf("429: %+v", de)
	}
	for _, s := range []int{503, 504} {
		if !errors.As(classifyHTTP(s, nil, "5"), &de) || !de.Temp || de.RetryAfter != 5*time.Second {
			t.Errorf("%d with Retry-After 5: %+v", s, de)
		}
	}
	for _, s := range []int{500, 503, 504} {
		if !errors.As(classifyHTTP(s, nil, ""), &de) || !de.Temp {
			t.Errorf("%d should be temporary", s)
		}
	}
	for _, s := range []int{400, 403, 404} {
		err := classifyHTTP(s, []byte("nope"), "")
		if !errors.As(err, &de) || de.Temp {
			t.Errorf("%d should be permanent", s)
		}
	}
	if err := classifyHTTP(403, nil, ""); !strings.Contains(err.Error(), "permission") {
		t.Errorf("403 message: %v", err)
	}
}
