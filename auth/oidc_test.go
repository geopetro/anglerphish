package auth

import (
	"testing"
)

func TestUsernameFromEmailLocalPart(t *testing.T) {
	username, err := UsernameFromEmail("firstlast@example.com", UsernameFromEmailLocalPart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "firstlast" {
		t.Fatalf("expected firstlast, got %s", username)
	}
}

func TestUsernameFromEmailFull(t *testing.T) {
	username, err := UsernameFromEmail("firstlast@example.com", UsernameFromEmailFull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "firstlast@example.com" {
		t.Fatalf("expected full email, got %s", username)
	}
}

func TestUsernameFromEmailInvalid(t *testing.T) {
	_, err := UsernameFromEmail("invalid-email", UsernameFromEmailLocalPart)
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestHasRequiredGroup(t *testing.T) {
	groups := []string{"other-group", "anglerphish-admins"}
	if !HasRequiredGroup(groups, "anglerphish-admins") {
		t.Fatal("expected required group to match")
	}
	if HasRequiredGroup(groups, "missing-group") {
		t.Fatal("expected missing group to fail")
	}
	if HasRequiredGroup(nil, "anglerphish-admins") {
		t.Fatal("expected nil groups to fail")
	}
}

func TestHasRequiredGroupWithLeadingSlash(t *testing.T) {
	groups := []string{"/anglerphish-admins", "other-group"}
	if !HasRequiredGroup(groups, "anglerphish-admins") {
		t.Fatal("expected slash-prefixed token group to match")
	}
	if !HasRequiredGroup(groups, "/anglerphish-admins") {
		t.Fatal("expected slash-prefixed required group to match")
	}
	if HasRequiredGroup(groups, "anglerphish") {
		t.Fatal("expected partial group name to fail")
	}
}

func TestClaimsFromMap(t *testing.T) {
	claims, err := claimsFromMap(map[string]interface{}{
		"email":         "firstlast@example.com",
		"custom_groups": []interface{}{"anglerphish-admins"},
	}, "custom_groups")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Email != "firstlast@example.com" {
		t.Fatalf("unexpected email: %s", claims.Email)
	}
	if len(claims.Groups) != 1 || claims.Groups[0] != "anglerphish-admins" {
		t.Fatalf("unexpected groups: %v", claims.Groups)
	}
}

func TestValidateOIDCConfigDisabled(t *testing.T) {
	if err := ValidateOIDCConfig(OIDCConfig{}); err != nil {
		t.Fatalf("expected no error when disabled: %v", err)
	}
}

func TestValidateOIDCConfigEnabledMissingFields(t *testing.T) {
	err := ValidateOIDCConfig(OIDCConfig{Enabled: true})
	if err == nil {
		t.Fatal("expected validation error for incomplete config")
	}
}

func TestExtractGroups(t *testing.T) {
	groups := extractGroups([]interface{}{"group-a", "group-b"})
	if len(groups) != 2 || groups[0] != "group-a" {
		t.Fatalf("unexpected groups: %v", groups)
	}
}
