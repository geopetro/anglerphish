package imap

import (
	"errors"
	"testing"
)

func TestShouldUseUID(t *testing.T) {
	cases := []struct {
		name         string
		uid          uint32
		storedValid  uint32
		currentValid uint32
		want         bool
	}{
		{"matching uidvalidity", 42, 100, 100, true},
		{"mismatched uidvalidity", 42, 100, 101, false},
		{"no stored uid", 0, 100, 100, false},
		{"no stored uidvalidity", 42, 0, 100, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldUseUID(tc.uid, tc.storedValid, tc.currentValid); got != tc.want {
				t.Errorf("shouldUseUID(%d,%d,%d) = %v, want %v",
					tc.uid, tc.storedValid, tc.currentValid, got, tc.want)
			}
		})
	}
}

func TestResolveSearchResults(t *testing.T) {
	if _, err := resolveSearchResults(nil); !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("empty results: got %v, want ErrMessageNotFound", err)
	}
	got, err := resolveSearchResults([]uint32{7})
	if err != nil || got != 7 {
		t.Errorf("single result: got (%d, %v), want (7, nil)", got, err)
	}
	// A forged Message-ID can collide with another message. Refuse to guess.
	if _, err := resolveSearchResults([]uint32{7, 9}); !errors.Is(err, ErrMessageAmbiguous) {
		t.Errorf("multiple results: got %v, want ErrMessageAmbiguous", err)
	}
}
