package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func workspaceMembers(workspaceID uuid.UUID, count int) []entity.WorkspaceMember {
	members := make([]entity.WorkspaceMember, count)

	for i := range members {
		accountID := uuid.New()

		members[i] = entity.WorkspaceMember{
			Membership: entity.Membership{
				WorkspaceID: workspaceID,
				AccountID:   accountID,
				Role:        entity.MembershipRoleMember,
				Source:      entity.MembershipSourceManual,
			},
			DisplayName: "Member",
			Email:       "member@meridian.co",
			SortName:    "member",
		}
	}

	return members
}

func TestAFullPageCarriesACursorToTheNextOne(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	limit := 3

	h.expectActorMayReadMembers(workspaceID, actorID)
	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		Return(workspaceMembers(workspaceID, limit+1), nil)

	page, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{Limit: limit})
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}

	if len(page.Members) != limit {
		t.Fatalf("page size = %d, want the lookahead row trimmed to %d", len(page.Members), limit)
	}

	if page.NextCursor == "" {
		t.Fatal("a full page must carry a cursor to the next one")
	}

	cursor, err := entity.DecodeMembershipCursor(page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeMembershipCursor: %v", err)
	}

	last := page.Members[len(page.Members)-1]
	if cursor.AccountID != last.Membership.AccountID || cursor.Name != last.SortName {
		t.Fatalf("cursor = %+v, want the last returned row's keyset position", cursor)
	}
}

func TestAPartialPageCarriesNoCursor(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayReadMembers(workspaceID, actorID)
	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		Return(workspaceMembers(workspaceID, 2), nil)

	page, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{Limit: 10})
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}

	if page.NextCursor != "" {
		t.Fatal("the absence of a cursor is the only end-of-list signal; a short page must not carry one")
	}
}

func TestACursorIsForwardedAsAKeysetPositionNotAnOffset(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()
	limit := 25

	position := entity.MembershipCursor{Name: "rae okafor", AccountID: uuid.New()}

	h.expectActorMayReadMembers(workspaceID, actorID)

	var captured entity.MembershipPage

	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.MembershipPage) ([]entity.WorkspaceMember, error) {
			captured = page

			return nil, nil
		})

	if _, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{
		Cursor: position.Encode(),
		Limit:  limit,
	}); err != nil {
		t.Fatalf("ListMembers: %v", err)
	}

	if captured.Cursor == nil || *captured.Cursor != position {
		t.Fatalf("forwarded cursor = %+v, want the decoded keyset position %+v", captured.Cursor, position)
	}

	if captured.Limit != limit+1 {
		t.Fatalf("forwarded limit = %d, want %d so the next page is detectable without counting", captured.Limit, limit+1)
	}
}

func TestSearchIsForwardedTrimmedToTheQuery(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayReadMembers(workspaceID, actorID)

	var captured entity.MembershipPage

	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.MembershipPage) ([]entity.WorkspaceMember, error) {
			captured = page

			return nil, nil
		})

	if _, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{
		Query: "  rae  ",
	}); err != nil {
		t.Fatalf("ListMembers: %v", err)
	}

	if captured.Query != "rae" {
		t.Fatalf("forwarded query = %q, want it trimmed", captured.Query)
	}
}

func TestAnOversizedPageRequestIsClampedNotRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayReadMembers(workspaceID, actorID)

	var captured entity.MembershipPage

	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.MembershipPage) ([]entity.WorkspaceMember, error) {
			captured = page

			return nil, nil
		})

	if _, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{
		Limit: 5000,
	}); err != nil {
		t.Fatalf("an oversized page must be clamped, not refused: %v", err)
	}

	if captured.Limit != entity.MembershipPageMaxSize+1 {
		t.Fatalf("forwarded limit = %d, want the maximum plus the lookahead row", captured.Limit)
	}
}

func TestAMalformedCursorIsRefusedAsAValidationError(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayReadMembers(workspaceID, actorID)

	_, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{
		Cursor: "!!!not-a-cursor!!!",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("ListMembers error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "cursor" {
		t.Fatalf("field = %q, want the error attributed to cursor", validation.Fields[0].Field)
	}

	if errors.Is(err, entity.ErrMembershipCursorInvalid) {
		t.Fatal("the store-level cursor error must not escape the service")
	}
}

func TestAnAbsentCursorStartsAtTheFirstPage(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayReadMembers(workspaceID, actorID)

	var captured entity.MembershipPage

	h.memberships.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, page entity.MembershipPage) ([]entity.WorkspaceMember, error) {
			captured = page

			return nil, nil
		})

	if _, err := h.service.ListMembers(actingAs(actorID), workspaceID, service.ListMembersInput{}); err != nil {
		t.Fatalf("an empty cursor means the first page, not a decode error: %v", err)
	}

	if captured.Cursor != nil {
		t.Fatalf("forwarded cursor = %+v, want none", captured.Cursor)
	}

	if captured.Limit != entity.MembershipPageDefaultSize+1 {
		t.Fatalf("forwarded limit = %d, want the default plus the lookahead row", captured.Limit)
	}
}
