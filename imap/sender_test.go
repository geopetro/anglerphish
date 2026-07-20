package imap

import "testing"

func TestParseSenderAddress(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"bare address", "jane@corp.com", "jane@corp.com", false},
		{"display name", "Jane Doe <jane@corp.com>", "jane@corp.com", false},
		{"quoted display name with comma", `"Doe, Jane" <jane@corp.com>`, "jane@corp.com", false},
		{"display name containing an at sign", `"jane@other.com" <jane@corp.com>`, "jane@corp.com", false},
		{"unclosed bracket falls back", "Jane <jane@corp.com", "jane@corp.com", false},
		{"empty", "", "", true},
		{"no at sign", "not-an-email", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSenderAddress(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseSenderAddress(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSenderDomain(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"jane@corp.com", "corp.com"},
		{"Jane Doe <jane@corp.com>", "corp.com"},
		{"Jane Doe <jane@CORP.COM>", "corp.com"},
		{"Jane <jane@corp.com", "corp.com"},
	}
	for _, tc := range cases {
		got, err := SenderDomain(tc.in)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("SenderDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// This is the regression test for the bug: a display-name From header
// previously produced the domain "corp.com>" and never matched.
func TestDomainMatches(t *testing.T) {
	cases := []struct {
		name     string
		from     string
		restrict string
		want     bool
	}{
		{"display name in domain", "Jane Doe <jane@corp.com>", "corp.com", true},
		{"bare address in domain", "jane@corp.com", "corp.com", true},
		{"case insensitive", "Jane <jane@CORP.com>", "corp.com", true},
		{"restrict value case insensitive", "jane@corp.com", "CORP.com", true},
		{"different domain", "Jane Doe <jane@evil.com>", "corp.com", false},
		{"subdomain is not a match", "jane@mail.corp.com", "corp.com", false},
		{"unparseable sender", "garbage", "corp.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DomainMatches(tc.from, tc.restrict); got != tc.want {
				t.Errorf("DomainMatches(%q, %q) = %v, want %v", tc.from, tc.restrict, got, tc.want)
			}
		})
	}
}
