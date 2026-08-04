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

func (h *harness) expectMayRequireSSO(workspaceID, actorID uuid.UUID) {
	h.expectActorMayActOn(
		workspaceID,
		actorID,
		entity.ResourceAuthPolicy,
		entity.ActionUpdate,
		workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive),
	)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
}

func blockerOf(t *testing.T, err error) entity.EnforcementBlocker {
	t.Helper()

	var refused entity.EnforcementRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error %v does not say which thing is missing", err)
	}

	return refused.Blocker
}

func TestRequiringSingleSignOnIsRefusedWithoutAProvider(t *testing.T) {
	h := newHarness(t)

	actorID, workspaceID := uuid.New(), uuid.New()
	h.expectMayRequireSSO(workspaceID, actorID)
	h.connections.EXPECT().
		Verified(gomock.Any(), workspaceID).
		Return(false, entity.ErrSSOConnectionNotFound)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err == nil {
		t.Fatal(
			"a workspace with no provider at all was allowed to require one, which locks out " +
				"every member immediately and leaves nothing to sign in with",
		)
	}

	if blocker := blockerOf(t, err); blocker != entity.EnforcementBlockerNoConnection {
		t.Fatalf("blocker %q, want %q", blocker, entity.EnforcementBlockerNoConnection)
	}
}

func TestRequiringSingleSignOnIsRefusedUntilTheProviderHasBeenTested(t *testing.T) {
	h := newHarness(t)

	actorID, workspaceID := uuid.New(), uuid.New()
	h.expectMayRequireSSO(workspaceID, actorID)
	h.connections.EXPECT().Verified(gomock.Any(), workspaceID).Return(false, nil)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err == nil {
		t.Fatal("an untested provider was allowed to become mandatory")
	}

	if blocker := blockerOf(t, err); blocker != entity.EnforcementBlockerNotVerified {
		t.Fatalf("blocker %q, want %q", blocker, entity.EnforcementBlockerNotVerified)
	}
}

func TestRequiringSingleSignOnIsRefusedWhileNoAdministratorCanUseIt(t *testing.T) {
	h := newHarness(t)

	actorID, workspaceID := uuid.New(), uuid.New()
	h.expectMayRequireSSO(workspaceID, actorID)
	h.connections.EXPECT().Verified(gomock.Any(), workspaceID).Return(true, nil)
	h.identities.EXPECT().AnyLinkedAdmin(gomock.Any(), workspaceID).Return(false, nil)

	_, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err == nil {
		t.Fatal(
			"single sign-on became mandatory with no administrator who has ever signed in " +
				"through the provider. A working provider that nobody has proved they can use " +
				"still leaves the workspace unadministrable.",
		)
	}

	if blocker := blockerOf(t, err); blocker != entity.EnforcementBlockerNoLinkedAdmin {
		t.Fatalf("blocker %q, want %q", blocker, entity.EnforcementBlockerNoLinkedAdmin)
	}
}

func TestRequiringSingleSignOnHandsBackRecoveryCodesExactlyOnce(t *testing.T) {
	h := newHarness(t)

	actorID, workspaceID := uuid.New(), uuid.New()
	h.expectMayRequireSSO(workspaceID, actorID)
	h.connections.EXPECT().Verified(gomock.Any(), workspaceID).Return(true, nil)
	h.identities.EXPECT().AnyLinkedAdmin(gomock.Any(), workspaceID).Return(true, nil)

	var stored [][]byte

	h.breakGlass.EXPECT().
		Replace(gomock.Any(), workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ *uuid.UUID, hashes [][]byte) error {
			stored = hashes

			return nil
		})

	h.authPolicies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
			return policy, nil
		})

	outcome, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementSSO)
	if err != nil {
		t.Fatalf("SetAuthPolicy: %v", err)
	}

	if len(outcome.RecoveryCodes) != entity.BreakGlassCodeCount {
		t.Fatalf("%d codes handed back, want %d", len(outcome.RecoveryCodes), entity.BreakGlassCodeCount)
	}

	if len(stored) != len(outcome.RecoveryCodes) {
		t.Fatalf("%d codes stored for %d handed back", len(stored), len(outcome.RecoveryCodes))
	}

	for i, code := range outcome.RecoveryCodes {
		if string(entity.HashBreakGlassCode(code)) != string(stored[i]) {
			t.Fatal("a code was handed back that does not match what was stored, so it could never be redeemed")
		}

		if len(stored[i]) != 32 {
			t.Fatal("the code was stored as something other than a digest")
		}
	}
}

func TestLiftingTheRequirementThrowsTheOldRecoveryCodesAway(t *testing.T) {
	h := newHarness(t)

	actorID, workspaceID := uuid.New(), uuid.New()
	h.expectMayRequireSSO(workspaceID, actorID)
	h.breakGlass.EXPECT().Discard(gomock.Any(), workspaceID).Return(nil)
	h.authPolicies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
			return policy, nil
		})

	outcome, err := h.service.SetAuthPolicy(actingAs(actorID), workspaceID, entity.AuthEnforcementAny)
	if err != nil {
		t.Fatalf("SetAuthPolicy: %v", err)
	}

	if len(outcome.RecoveryCodes) != 0 {
		t.Fatal("codes were handed out for a workspace that no longer requires single sign-on")
	}
}

func TestARecoveryCodeLiftsTheRequirementAndIsSpent(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	workspace := workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive)
	workspace.Slug = "northwind"

	h.workspaces.EXPECT().GetBySlug(gomock.Any(), "northwind").Return(workspace, nil)
	h.authPolicies.EXPECT().
		Get(gomock.Any(), workspaceID).
		Return(entity.WorkspaceAuthPolicy{WorkspaceID: workspaceID, Enforcement: entity.AuthEnforcementSSO}, nil)

	var redeemed []byte

	h.breakGlass.EXPECT().
		Redeem(gomock.Any(), workspaceID, gomock.Any(), "10.0.0.1").
		DoAndReturn(func(_ context.Context, _ uuid.UUID, hash []byte, _ string) error {
			redeemed = hash

			return nil
		})

	var saved entity.WorkspaceAuthPolicy

	h.authPolicies.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
			saved = policy

			return policy, nil
		})

	if err := h.service.RedeemRecoveryCode(context.Background(), service.RedeemRecoveryCodeInput{
		WorkspaceSlug: "northwind",
		Code:          "abcd-efgh-jklm",
		From:          "10.0.0.1",
	}); err != nil {
		t.Fatalf("RedeemRecoveryCode: %v", err)
	}

	if saved.Enforcement != entity.AuthEnforcementAny {
		t.Fatalf(
			"enforcement is %q after redeeming a recovery code. The whole point is that a "+
				"workspace whose provider has died becomes reachable with a password again.",
			saved.Enforcement,
		)
	}

	if string(redeemed) != string(entity.HashBreakGlassCode("ABCDEFGHJKLM")) {
		t.Fatal("the code was not normalised before being looked up, so typing it differently would fail")
	}
}

func TestARecoveryCodeIsRefusedWhereNothingIsBeingEnforced(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	workspace := workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive)
	workspace.Slug = "northwind"

	h.workspaces.EXPECT().GetBySlug(gomock.Any(), "northwind").Return(workspace, nil)
	h.authPolicies.EXPECT().
		Get(gomock.Any(), workspaceID).
		Return(entity.WorkspaceAuthPolicy{WorkspaceID: workspaceID, Enforcement: entity.AuthEnforcementAny}, nil)

	err := h.service.RedeemRecoveryCode(context.Background(), service.RedeemRecoveryCodeInput{
		WorkspaceSlug: "northwind",
		Code:          "abcd-efgh-jklm",
	})

	if !errors.Is(err, entity.ErrBreakGlassNotEnforcing) {
		t.Fatalf(
			"redeeming against a workspace that requires nothing gave %v. Spending a code for "+
				"no reason would burn one of a small, finite set.",
			err,
		)
	}
}

func TestAnUnknownWorkspaceLooksTheSameAsAWrongCode(t *testing.T) {
	h := newHarness(t)

	h.workspaces.EXPECT().
		GetBySlug(gomock.Any(), "nowhere").
		Return(entity.Workspace{}, entity.ErrWorkspaceNotFound)

	err := h.service.RedeemRecoveryCode(context.Background(), service.RedeemRecoveryCodeInput{
		WorkspaceSlug: "nowhere",
		Code:          "abcd-efgh-jklm",
	})

	if !errors.Is(err, entity.ErrBreakGlassCodeInvalid) {
		t.Fatalf(
			"a missing workspace gave %v rather than the same answer as a wrong code. This "+
				"endpoint is unauthenticated, so it must not confirm which workspaces exist.",
			err,
		)
	}
}
