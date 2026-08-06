package entity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/usenorn/norn/internal/entity"
)

func TestMembershipRoleValidity(t *testing.T) {
	for _, role := range []entity.MembershipRole{entity.MembershipRoleAdmin, entity.MembershipRoleMember} {
		if !role.Valid() {
			t.Errorf("role %q should be valid", role)
		}
	}

	for _, role := range []entity.MembershipRole{"", "owner", "ADMIN"} {
		if role.Valid() {
			t.Errorf("role %q should not be valid", role)
		}
	}
}

func TestValidateWorkspaceSlug(t *testing.T) {
	cases := []struct {
		name  string
		value string
		code  string
	}{
		{"empty", "", entity.ValidationCodeRequired},
		{"too short", "a", entity.ValidationCodeTooShort},
		{"kebab case", "acme-labs", ""},
		{"uppercase", "Acme", entity.ValidationCodeMalformed},
		{"leading dash", "-acme", entity.ValidationCodeMalformed},
		{"trailing dash", "acme-", entity.ValidationCodeMalformed},
		{"double dash", "acme--labs", entity.ValidationCodeMalformed},
		{"underscore", "acme_labs", entity.ValidationCodeMalformed},
		{"too long", strings.Repeat("a", entity.WorkspaceSlugMaxLen+1), entity.ValidationCodeTooLong},
		{"a reserved slug is well formed", "v1", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateWorkspaceSlug("slug", c.value).Code; got != c.code {
				t.Errorf("ValidateWorkspaceSlug(%q) code = %q, want %q", c.value, got, c.code)
			}
		})
	}
}

func TestSlugsRoutedElsewhereAreReserved(t *testing.T) {
	cases := []struct {
		name     string
		slug     string
		reserved bool
	}{
		{"routed to the api", "v1", true},
		{"routed to the mcp edge", "oauth", true},
		{"shadowed by a sign-in route", "sign-in", true},
		{"shadowed by account settings", "settings", true},
		{"merely prefixed by a reserved word", "v1-team", false},
		{"ordinary slug", "acme-labs", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.WorkspaceSlugReserved(c.slug); got != c.reserved {
				t.Errorf("WorkspaceSlugReserved(%q) = %v, want %v", c.slug, got, c.reserved)
			}
		})
	}
}

func TestLastWorkspaceAdminErrorIsDetectableBySentinelAndCarriesTheWorkspaces(t *testing.T) {
	workspaceID := uuid.New()

	var err error = entity.LastWorkspaceAdminError{WorkspaceIDs: []uuid.UUID{workspaceID}}

	if !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatal("LastWorkspaceAdminError is not detectable as ErrAccountLastWorkspaceAdmin")
	}

	var detail entity.LastWorkspaceAdminError
	if !errors.As(err, &detail) {
		t.Fatal("LastWorkspaceAdminError is not recoverable with errors.As")
	}

	if len(detail.WorkspaceIDs) != 1 || detail.WorkspaceIDs[0] != workspaceID {
		t.Fatalf("WorkspaceIDs = %v, want [%v]", detail.WorkspaceIDs, workspaceID)
	}
}

func TestWorkspaceStatusAcceptsOnlyKnownValues(t *testing.T) {
	cases := map[entity.WorkspaceStatus]bool{
		entity.WorkspaceStatusActive:          true,
		entity.WorkspaceStatusPendingDeletion: true,
		"":                                    false,
		"purged":                              false,
		"deleted":                             false,
	}

	for status, want := range cases {
		if got := status.Valid(); got != want {
			t.Errorf("WorkspaceStatus(%q).Valid() = %t, want %t", status, got, want)
		}
	}
}

func TestWorkspaceDeletionIsReversibleButNotRepeatable(t *testing.T) {
	cases := []struct {
		name   string
		from   entity.WorkspaceStatus
		to     entity.WorkspaceStatus
		want   bool
		reason string
	}{
		{
			name:   "active workspace can be deleted",
			from:   entity.WorkspaceStatusActive,
			to:     entity.WorkspaceStatusPendingDeletion,
			want:   true,
			reason: "deletion must be possible",
		},
		{
			name:   "pending deletion can be restored",
			from:   entity.WorkspaceStatusPendingDeletion,
			to:     entity.WorkspaceStatusActive,
			want:   true,
			reason: "deletion must be reversible for a period",
		},
		{
			name:   "deleting twice is refused",
			from:   entity.WorkspaceStatusPendingDeletion,
			to:     entity.WorkspaceStatusPendingDeletion,
			want:   false,
			reason: "a second delete must not extend the recovery window",
		},
		{
			name:   "restoring a live workspace is refused",
			from:   entity.WorkspaceStatusActive,
			to:     entity.WorkspaceStatusActive,
			want:   false,
			reason: "restore only applies to a deleted workspace",
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

func TestWorkspacePurgeIsDueOnlyOnceTheWindowHasElapsed(t *testing.T) {
	now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	inside := now.Add(time.Second)
	elapsed := now.Add(-time.Second)

	cases := []struct {
		name      string
		workspace entity.Workspace
		want      bool
	}{
		{
			name:      "active workspace is never due",
			workspace: entity.Workspace{Status: entity.WorkspaceStatusActive, PurgeAfter: &elapsed},
			want:      false,
		},
		{
			name:      "pending deletion inside the window is not due",
			workspace: entity.Workspace{Status: entity.WorkspaceStatusPendingDeletion, PurgeAfter: &inside},
			want:      false,
		},
		{
			name:      "pending deletion exactly at the boundary is due",
			workspace: entity.Workspace{Status: entity.WorkspaceStatusPendingDeletion, PurgeAfter: &now},
			want:      true,
		},
		{
			name:      "pending deletion past the window is due",
			workspace: entity.Workspace{Status: entity.WorkspaceStatusPendingDeletion, PurgeAfter: &elapsed},
			want:      true,
		},
		{
			name:      "pending deletion with no purge date is never due",
			workspace: entity.Workspace{Status: entity.WorkspaceStatusPendingDeletion},
			want:      false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.workspace.PurgeDueAt(now); got != testCase.want {
				t.Fatalf("PurgeDueAt = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestWorkspaceDeletedErrorCarriesThePurgeDateAndUnwraps(t *testing.T) {
	purgeAfter := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	err := entity.WorkspaceDeletedError{PurgeAfter: &purgeAfter}

	if !errors.Is(err, entity.ErrWorkspaceDeleted) {
		t.Fatal("WorkspaceDeletedError must unwrap to ErrWorkspaceDeleted so callers detect it by identity")
	}

	if !strings.Contains(err.Error(), "2026-09-01") {
		t.Fatalf("error = %q, want it to name the date recovery ends", err.Error())
	}

	bare := entity.WorkspaceDeletedError{}
	if bare.Error() != entity.ErrWorkspaceDeleted.Error() {
		t.Fatalf("bare error = %q, want the plain sentinel message", bare.Error())
	}
}
