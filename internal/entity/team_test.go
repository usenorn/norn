package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestTeamStatusAcceptsOnlyKnownValues(t *testing.T) {
	cases := map[entity.TeamStatus]bool{
		entity.TeamStatusActive:   true,
		entity.TeamStatusArchived: true,
		"":                        false,
		"deleted":                 false,
		"ACTIVE":                  false,
	}

	for status, want := range cases {
		if got := status.Valid(); got != want {
			t.Errorf("TeamStatus(%q).Valid() = %t, want %t", status, got, want)
		}
	}
}

func TestTeamVisibilityAcceptsOnlyKnownValues(t *testing.T) {
	cases := map[entity.TeamVisibility]bool{
		entity.TeamVisibilityPublic:  true,
		entity.TeamVisibilityPrivate: true,
		"":                           false,
		"secret":                     false,
		"workspace":                  false,
	}

	for visibility, want := range cases {
		if got := visibility.Valid(); got != want {
			t.Errorf("TeamVisibility(%q).Valid() = %t, want %t", visibility, got, want)
		}
	}
}

func TestTeamArchivingIsReversibleButNotRepeatable(t *testing.T) {
	cases := []struct {
		name   string
		from   entity.TeamStatus
		to     entity.TeamStatus
		want   bool
		reason string
	}{
		{
			name:   "active team can be archived",
			from:   entity.TeamStatusActive,
			to:     entity.TeamStatusArchived,
			want:   true,
			reason: "teams dissolve without destroying their issues",
		},
		{
			name:   "archived team can be brought back",
			from:   entity.TeamStatusArchived,
			to:     entity.TeamStatusActive,
			want:   true,
			reason: "archiving is not a terminal decision",
		},
		{
			name:   "archiving twice is refused",
			from:   entity.TeamStatusArchived,
			to:     entity.TeamStatusArchived,
			want:   false,
			reason: "a second archive must not restamp the archive date",
		},
		{
			name:   "unarchiving a live team is refused",
			from:   entity.TeamStatusActive,
			to:     entity.TeamStatusActive,
			want:   false,
			reason: "unarchive only applies to an archived team",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.from.CanTransitionTo(testCase.to); got != testCase.want {
				t.Fatalf("%s -> %s = %t, want %t (%s)", testCase.from, testCase.to, got, testCase.want, testCase.reason)
			}
		})
	}
}

func TestValidateTeamKey(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"single letter", "M", entity.ValidationCodeTooShort},
		{"two letters", "AB", ""},
		{"five letters", "MOBIL", ""},
		{"six letters", "MOBILE", entity.ValidationCodeTooLong},
		{"lowercase", "mob", entity.ValidationCodeMalformed},
		{"digits", "M0B", entity.ValidationCodeMalformed},
		{"space", "M B", entity.ValidationCodeMalformed},
		{"hyphen", "M-B", entity.ValidationCodeMalformed},
		{"too long", strings.Repeat("A", entity.TeamKeyMaxLen+1), entity.ValidationCodeTooLong},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateTeamKey("key", c.value).Code; got != c.code {
				t.Errorf("ValidateTeamKey(%q) code = %q, want %q", c.value, got, c.code)
			}
		})
	}
}

func TestValidateTeamName(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"whitespace only", "   ", entity.ValidationCodeRequired},
		{"ordinary", "Mobile", ""},
		{"too long", strings.Repeat("a", entity.TeamNameMaxLen+1), entity.ValidationCodeTooLong},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateTeamName("name", c.value).Code; got != c.code {
				t.Errorf("ValidateTeamName(%q) code = %q, want %q", c.value, got, c.code)
			}
		})
	}
}

func TestNormalizeTeamKeyUppercasesAndTrims(t *testing.T) {
	cases := map[string]string{
		" mob ": "MOB",
		"mob":   "MOB",
		"MOB":   "MOB",
		"MoB":   "MOB",
		"":      "",
	}

	for input, want := range cases {
		if got := entity.NormalizeTeamKey(input); got != want {
			t.Errorf("NormalizeTeamKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTeamReportsItsOwnPrivacyAndArchival(t *testing.T) {
	private := entity.Team{Visibility: entity.TeamVisibilityPrivate, Status: entity.TeamStatusActive}
	if !private.Private() || private.Archived() {
		t.Fatalf("private active team = %+v, want private and not archived", private)
	}

	archived := entity.Team{Visibility: entity.TeamVisibilityPublic, Status: entity.TeamStatusArchived}
	if archived.Private() || !archived.Archived() {
		t.Fatalf("public archived team = %+v, want archived and not private", archived)
	}
}
