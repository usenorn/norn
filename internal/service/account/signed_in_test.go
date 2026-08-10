package account_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func signedInAs(sessions ...entity.Session) context.Context {
	ctx := identity.WithSignedIn(context.Background(), sessions)

	if len(sessions) > 0 {
		ctx = identity.WithSession(ctx, sessions[0])
	}

	return ctx
}

func sessionFor(accountID uuid.UUID, slot string) entity.Session {
	return entity.Session{
		ID:         uuid.New(),
		AccountID:  accountID,
		Slot:       slot,
		AuthMethod: entity.SessionAuthMethodPassword,
	}
}

func TestASessionWhoseAccountIsGoneLeavesTheOtherAccountsSignedIn(t *testing.T) {
	h := newHarness(t)

	live := uuid.New()
	gone := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), live).Return(activeAccount(live), nil)
	h.accounts.EXPECT().GetByID(gomock.Any(), gone).Return(entity.Account{}, entity.ErrAccountNotFound)
	h.workspaces.EXPECT().ListByAccountID(gomock.Any(), live).Return(nil, nil)

	accounts, err := h.service.SignedIn(signedInAs(sessionFor(live, "live"), sessionFor(gone, "gone")))
	if err != nil {
		t.Fatalf("SignedIn returned %v, want the surviving account", err)
	}

	if len(accounts) != 1 {
		t.Fatalf("SignedIn returned %d accounts, want 1", len(accounts))
	}

	if accounts[0].Account.ID != live {
		t.Errorf("SignedIn returned account %s, want %s", accounts[0].Account.ID, live)
	}
}

func TestEverySessionPointingAtAMissingAccountIsRefused(t *testing.T) {
	h := newHarness(t)

	gone := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), gone).Return(entity.Account{}, entity.ErrAccountNotFound)

	if _, err := h.service.SignedIn(signedInAs(sessionFor(gone, "gone"))); err == nil {
		t.Fatal("SignedIn succeeded with no resolvable account, want a refusal")
	}
}

func TestAWorkspaceReachedByTwoSessionsIsReadOnce(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	workspaceID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.workspaces.EXPECT().
		ListByAccountID(gomock.Any(), accountID).
		Return([]entity.Workspace{{ID: workspaceID, Slug: "alpha"}}, nil)
	h.authPolicies.EXPECT().
		Get(gomock.Any(), workspaceID).
		Return(entity.WorkspaceAuthPolicy{WorkspaceID: workspaceID}, nil).
		Times(1)

	ctx := signedInAs(sessionFor(accountID, "first"), sessionFor(accountID, "second"))

	if _, err := h.service.SignedIn(ctx); err != nil {
		t.Fatalf("SignedIn returned %v", err)
	}
}
