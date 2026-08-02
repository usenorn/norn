package account_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestDeactivateKeepsEveryIdentifyingFieldIntact(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	before := activeAccount(accountID)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)
	h.expectNoAdminWorkspaces(accountID)
	h.expectSessionsRevoked(accountID)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	if _, err := h.service.Deactivate(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	if captured.Status != entity.AccountStatusDeactivated {
		t.Fatalf("status = %q, want deactivated", captured.Status)
	}

	if captured.DeactivatedAt == nil {
		t.Fatal("DeactivatedAt was not stamped")
	}

	if captured.ID != before.ID || captured.Email != before.Email || captured.DisplayName != before.DisplayName {
		t.Fatal("deactivation altered the fields that attribute authored content")
	}
}

func TestDeactivateIsRefusedWhenTheAccountIsTheLastAdminOfAWorkspace(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	workspaceID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectSoleAdminOf(accountID, workspaceID)

	_, err := h.service.Deactivate(actingAs(accountID), accountID)
	if !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatalf("Deactivate error = %v, want ErrAccountLastWorkspaceAdmin", err)
	}

	var detail entity.LastWorkspaceAdminError
	if !errors.As(err, &detail) || len(detail.WorkspaceIDs) != 1 || detail.WorkspaceIDs[0] != workspaceID {
		t.Fatalf("error does not name the blocking workspace: %v", err)
	}
}

func TestDeactivateSucceedsWhenAnotherAdminRemains(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	workspaceID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectAdminWithPeers(accountID, workspaceID)
	h.expectSessionsRevoked(accountID)
	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	if _, err := h.service.Deactivate(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
}

func TestDeactivateSkipsTheWorkspaceLockForANonAdmin(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectNoAdminWorkspaces(accountID)
	h.expectSessionsRevoked(accountID)
	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	if _, err := h.service.Deactivate(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
}

func TestDeactivateIsRefusedForAnAlreadyDeletedAccount(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	deleted := activeAccount(accountID)
	deleted.Status = entity.AccountStatusDeleted

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(deleted, nil)

	_, err := h.service.Deactivate(actingAs(accountID), accountID)
	if !errors.Is(err, entity.ErrAccountStatusTransition) {
		t.Fatalf("Deactivate error = %v, want ErrAccountStatusTransition", err)
	}
}

func TestDeleteAnonymizesInPlaceAndKeepsTheAccountIdentifier(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	before := activeAccount(accountID)
	before.PasswordHash = "$argon2id$stored"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)
	h.expectNoAdminWorkspaces(accountID)
	h.expectSessionsRevoked(accountID)
	h.emailChanges.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordHistory.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)
	h.memberships.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	if err := h.service.Delete(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if captured.ID != accountID {
		t.Fatal("deletion changed the identifier authored content references")
	}

	if captured.Status != entity.AccountStatusDeleted || captured.DeletedAt == nil {
		t.Fatalf("status = %q with DeletedAt %v, want deleted and stamped", captured.Status, captured.DeletedAt)
	}

	if captured.Email != "" || captured.DisplayName != "" || captured.Timezone != "" || captured.PasswordHash != "" {
		t.Fatalf("deletion left identifying data behind: %+v", captured)
	}
}

func TestDeleteRemovesTheAvatarObjectAfterTheAccountIsAnonymized(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	before := activeAccount(accountID)
	before.AvatarObjectKey = "avatars/ada/portrait.png"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)
	h.expectNoAdminWorkspaces(accountID)
	h.expectSessionsRevoked(accountID)
	h.emailChanges.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordHistory.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)
	h.memberships.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)

	updated := h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	h.blobs.EXPECT().Delete(gomock.Any(), before.AvatarObjectKey).Return(nil).After(updated)

	if err := h.service.Delete(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestDeleteSucceedsWhenTheAvatarObjectDeleteFails(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	before := activeAccount(accountID)
	before.AvatarObjectKey = "avatars/ada/portrait.png"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)
	h.expectNoAdminWorkspaces(accountID)
	h.expectSessionsRevoked(accountID)
	h.emailChanges.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	h.passwordHistory.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)
	h.memberships.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil)
	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	h.blobs.EXPECT().Delete(gomock.Any(), before.AvatarObjectKey).Return(errors.New("object store unreachable"))

	if err := h.service.Delete(actingAs(accountID), accountID); err != nil {
		t.Fatalf("Delete must survive a failed object cleanup, got %v", err)
	}
}

func TestDeleteIsRefusedWhenTheAccountIsTheLastAdminOfAWorkspace(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	workspaceID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectSoleAdminOf(accountID, workspaceID)

	err := h.service.Delete(actingAs(accountID), accountID)
	if !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatalf("Delete error = %v, want ErrAccountLastWorkspaceAdmin", err)
	}
}

func TestOperationsOnAnotherAccountAreRefusedWithoutTouchingTheStore(t *testing.T) {
	accountID := uuid.New()
	intruder := uuid.New()

	operations := map[string]func(h *harness, ctx context.Context) error{
		"Get": func(h *harness, ctx context.Context) error {
			_, err := h.service.Get(ctx, accountID)

			return err
		},
		"Deactivate": func(h *harness, ctx context.Context) error {
			_, err := h.service.Deactivate(ctx, accountID)

			return err
		},
		"Delete": func(h *harness, ctx context.Context) error {
			return h.service.Delete(ctx, accountID)
		},
		"RequestEmailChange": func(h *harness, ctx context.Context) error {
			_, err := h.service.RequestEmailChange(ctx, accountID, "grace@example.com")

			return err
		},
		"ChangePassword": func(h *harness, ctx context.Context) error {
			_, err := h.service.ChangePassword(ctx, accountID, "old-password-value", "new-password-value")

			return err
		},
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			if err := operation(h, actingAs(intruder)); !errors.Is(err, entity.ErrAccountForbidden) {
				t.Fatalf("%s error = %v, want ErrAccountForbidden", name, err)
			}

			if err := operation(h, context.Background()); !errors.Is(err, entity.ErrAccountForbidden) {
				t.Fatalf("%s without an identity error = %v, want ErrAccountForbidden", name, err)
			}
		})
	}
}
