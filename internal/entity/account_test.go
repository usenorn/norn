package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAccountStatusTransitions(t *testing.T) {
	cases := []struct {
		from    entity.AccountStatus
		to      entity.AccountStatus
		allowed bool
	}{
		{entity.AccountStatusActive, entity.AccountStatusDeactivated, true},
		{entity.AccountStatusActive, entity.AccountStatusDeleted, true},
		{entity.AccountStatusActive, entity.AccountStatusActive, false},
		{entity.AccountStatusDeactivated, entity.AccountStatusActive, true},
		{entity.AccountStatusDeactivated, entity.AccountStatusDeleted, true},
		{entity.AccountStatusDeleted, entity.AccountStatusActive, false},
		{entity.AccountStatusDeleted, entity.AccountStatusDeactivated, false},
		{entity.AccountStatus("unknown"), entity.AccountStatusActive, false},
	}

	for _, c := range cases {
		t.Run(string(c.from)+"_to_"+string(c.to), func(t *testing.T) {
			if got := c.from.CanTransitionTo(c.to); got != c.allowed {
				t.Errorf("CanTransitionTo(%q) = %t, want %t", c.to, got, c.allowed)
			}
		})
	}
}

func TestAccountStatusValidity(t *testing.T) {
	valid := []entity.AccountStatus{
		entity.AccountStatusActive,
		entity.AccountStatusDeactivated,
		entity.AccountStatusDeleted,
	}

	for _, status := range valid {
		if !status.Valid() {
			t.Errorf("status %q should be valid", status)
		}
	}

	for _, status := range []entity.AccountStatus{"", "ACTIVE", "removed"} {
		if status.Valid() {
			t.Errorf("status %q should not be valid", status)
		}
	}
}

func TestAnAccountWithoutAPasswordCannotAuthenticate(t *testing.T) {
	account := entity.Account{Status: entity.AccountStatusActive}

	if account.HasPassword() {
		t.Fatal("account without a hash reports a password")
	}

	if account.CanAuthenticate() {
		t.Fatal("account without a password must not authenticate")
	}
}

func TestADeactivatedAccountCannotAuthenticate(t *testing.T) {
	account := entity.Account{Status: entity.AccountStatusDeactivated, PasswordHash: "$argon2id$"}

	if account.CanAuthenticate() {
		t.Fatal("deactivated account must not authenticate")
	}
}

func TestNormalizeEmailLowercasesAndTrims(t *testing.T) {
	if got := entity.NormalizeEmail("  Ada@Example.COM "); got != "ada@example.com" {
		t.Fatalf("NormalizeEmail = %q, want %q", got, "ada@example.com")
	}
}

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		name  string
		email string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"plain", "ada@example.com", ""},
		{"no at sign", "ada.example.com", entity.ValidationCodeMalformed},
		{"display name form", "Ada <ada@example.com>", entity.ValidationCodeMalformed},
		{"too long", strings.Repeat("a", 250) + "@example.com", entity.ValidationCodeTooLong},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateEmail("email", c.email).Code; got != c.code {
				t.Errorf("ValidateEmail(%q) code = %q, want %q", c.email, got, c.code)
			}
		})
	}
}

func TestValidateDisplayName(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"whitespace only", "   ", entity.ValidationCodeRequired},
		{"ordinary", "Ada Lovelace", ""},
		{"too long", strings.Repeat("a", entity.DisplayNameMaxLen+1), entity.ValidationCodeTooLong},
		{"at the limit", strings.Repeat("a", entity.DisplayNameMaxLen), ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateDisplayName("display_name", c.value).Code; got != c.code {
				t.Errorf("ValidateDisplayName(%q) code = %q, want %q", c.value, got, c.code)
			}
		})
	}
}

func TestValidateTimezone(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"utc", "UTC", ""},
		{"iana", "Europe/Berlin", ""},
		{"unknown", "Mars/Olympus", entity.ValidationCodeUnknownTimezone},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateTimezone("timezone", c.value).Code; got != c.code {
				t.Errorf("ValidateTimezone(%q) code = %q, want %q", c.value, got, c.code)
			}
		})
	}
}
