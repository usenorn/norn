package entity_test

import (
	"errors"
	"strings"
	"testing"

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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.ValidateWorkspaceSlug("slug", c.value).Code; got != c.code {
				t.Errorf("ValidateWorkspaceSlug(%q) code = %q, want %q", c.value, got, c.code)
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
