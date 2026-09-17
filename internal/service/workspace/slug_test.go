package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestRenamingTheIdentifierKeepsTheOldAddressForThirtyDays(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()
	slug := "northwind-labs"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	var redirectExpiry time.Time

	gomock.InOrder(
		h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil),
		h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil),
		h.connections.EXPECT().Protocol(gomock.Any(), workspaceID).Return(entity.SSOProtocol(""), entity.ErrSSOConnectionNotFound),
		h.workspaces.EXPECT().ReserveSlug(gomock.Any(), slug, workspaceID, gomock.Any()).Return(nil),
		h.workspaces.EXPECT().
			RecordSlugRedirect(gomock.Any(), "northwind", workspaceID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, _ uuid.UUID, expiresAt time.Time) error {
				redirectExpiry = expiresAt

				return nil
			}),
		h.workspaces.EXPECT().
			UpdateSettings(gomock.Any(), workspaceID, gomock.Any()).
			DoAndReturn(func(_ context.Context, id uuid.UUID, settings repository.WorkspaceSettings) (entity.Workspace, error) {
				if settings.Slug != slug {
					t.Errorf("wrote slug %q, want %q", settings.Slug, slug)
				}

				renamed := activeWorkspace(id)
				renamed.Slug = settings.Slug

				return renamed, nil
			}),
	)

	before := time.Now().UTC()

	if _, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{Slug: &slug}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	lowest := before.Add(entity.WorkspaceSlugRedirectTTL)
	if redirectExpiry.Before(lowest) || redirectExpiry.After(lowest.Add(time.Minute)) {
		t.Errorf("redirect expires at %v, want thirty days after %v", redirectExpiry, before)
	}
}

func TestAnIdentifierHeldByAnotherWorkspaceIsRefused(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()
	slug := "lakeside"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), gomock.Any()).Return(nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)
	h.connections.EXPECT().Protocol(gomock.Any(), workspaceID).Return(entity.SSOProtocol(""), entity.ErrSSOConnectionNotFound)
	h.workspaces.EXPECT().ReserveSlug(gomock.Any(), slug, workspaceID, gomock.Any()).Return(entity.ErrWorkspaceSlugTaken)

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{Slug: &slug})
	if !errors.Is(err, entity.ErrWorkspaceSlugTaken) {
		t.Fatalf("Update error = %v, want %v", err, entity.ErrWorkspaceSlugTaken)
	}
}

func TestAWorkspaceWithSingleSignOnCannotChangeItsIdentifier(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()
	slug := "northwind-labs"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), gomock.Any()).Return(nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)
	h.connections.EXPECT().Protocol(gomock.Any(), workspaceID).Return(entity.SSOProtocolSAML, nil)

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{Slug: &slug})
	if !errors.Is(err, entity.ErrWorkspaceSlugPinned) {
		t.Fatalf("Update error = %v, want %v", err, entity.ErrWorkspaceSlugPinned)
	}
}

func TestAnIdentifierTheRouterOwnsIsRefusedBeforeAnyWrite(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()
	slug := "settings"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	_, err := h.service.Update(actingAs(actorID), workspaceID, service.UpdateWorkspaceInput{Slug: &slug})
	if !errors.Is(err, entity.ErrWorkspaceSlugTaken) {
		t.Fatalf("Update error = %v, want %v", err, entity.ErrWorkspaceSlugTaken)
	}
}

func TestAFormerAddressResolvesOnlyForAMember(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()

	h.workspaces.EXPECT().ResolveSlugRedirect(gomock.Any(), "northwind", gomock.Any()).Return(workspaceID, nil)
	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, entity.ResourceWorkspace, entity.ActionRead)).
		Return(entity.Decision{}, entity.AccessDeniedError{Reason: entity.DenyReasonNotAMember, WorkspaceID: workspaceID})

	_, err := h.service.ResolveSlugRedirect(actingAs(uuid.New()), "northwind")
	if !errors.Is(err, entity.ErrWorkspaceNotFound) {
		t.Fatalf("ResolveSlugRedirect error = %v, want %v", err, entity.ErrWorkspaceNotFound)
	}
}
