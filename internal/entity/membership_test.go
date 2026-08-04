package entity_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestMembershipRoleAcceptsOnlyKnownValues(t *testing.T) {
	cases := map[entity.MembershipRole]bool{
		entity.MembershipRoleAdmin:  true,
		entity.MembershipRoleMember: true,
		entity.MembershipRoleViewer: true,
		"":                          false,
		"owner":                     false,
		"guest":                     false,
		"ADMIN":                     false,
	}

	for role, want := range cases {
		if got := role.Valid(); got != want {
			t.Errorf("MembershipRole(%q).Valid() = %t, want %t", role, got, want)
		}
	}
}

func TestOnlyDirectoryMembershipsAreManaged(t *testing.T) {
	cases := map[entity.MembershipSource]bool{
		entity.MembershipSourceManual:    false,
		entity.MembershipSourceDirectory: true,
		"":                               false,
	}

	for source, want := range cases {
		if got := source.Managed(); got != want {
			t.Errorf("MembershipSource(%q).Managed() = %t, want %t", source, got, want)
		}
	}

	if entity.DefaultMembershipSource.Managed() {
		t.Fatal("a membership created by hand must not be reported as directory managed")
	}
}

func TestACursorRoundTripsThroughItsEncoding(t *testing.T) {
	names := []string{
		"rae okafor",
		"",
		"a/b+c=d",
		"zoë müller",
		"田中 太郎",
		strings.Repeat("long name ", 20),
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			want := entity.MembershipCursor{Name: name, AccountID: uuid.New()}

			got, err := entity.DecodeMembershipCursor(want.Encode())
			if err != nil {
				t.Fatalf("DecodeMembershipCursor: %v", err)
			}

			if got != want {
				t.Fatalf("cursor round trip = %+v, want %+v", got, want)
			}
		})
	}
}

func TestATamperedCursorIsRejected(t *testing.T) {
	cases := map[string]string{
		"not base64":        "!!!not-base64!!!",
		"too short":         base64.RawURLEncoding.EncodeToString([]byte("too-short")),
		"non uuid prefix":   base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("z", 36) + "rae")),
		"empty":             base64.RawURLEncoding.EncodeToString(nil),
		"uuid without dash": base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("a", 36))),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := entity.DecodeMembershipCursor(raw); !errors.Is(err, entity.ErrMembershipCursorInvalid) {
				t.Fatalf("DecodeMembershipCursor(%q) error = %v, want ErrMembershipCursorInvalid", raw, err)
			}
		})
	}
}

func TestAPageSizeIsClampedAndDefaulted(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		want  int
	}{
		{"unset", 0, entity.MembershipPageDefaultSize},
		{"negative", -10, entity.MembershipPageDefaultSize},
		{"inside the range", 25, 25},
		{"at the maximum", entity.MembershipPageMaxSize, entity.MembershipPageMaxSize},
		{"beyond the maximum", entity.MembershipPageMaxSize + 1, entity.MembershipPageMaxSize},
		{"absurd", 100000, entity.MembershipPageMaxSize},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := (entity.MembershipPage{Limit: testCase.limit}).Normalized().Limit; got != testCase.want {
				t.Fatalf("Normalized().Limit = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestANormalizedPageTrimsItsQuery(t *testing.T) {
	if got := (entity.MembershipPage{Query: "  rae  "}).Normalized().Query; got != "rae" {
		t.Fatalf("Normalized().Query = %q, want %q", got, "rae")
	}
}

func TestALookaheadAsksForOneMoreRowThanRequested(t *testing.T) {
	page := entity.MembershipPage{Limit: 25}.Normalized()

	if got := page.Lookahead().Limit; got != page.Limit+1 {
		t.Fatalf("Lookahead().Limit = %d, want %d so a next page can be detected without counting", got, page.Limit+1)
	}
}
