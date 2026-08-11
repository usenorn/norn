package scm_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestAMergedChangeBlockedByUnprovenChecksKeepsItsTransitionForLater(t *testing.T) {
	h := newAdvanceHarness(t)
	merged := h.mergedChange(t)

	h.issueWriter.EXPECT().
		Update(gomock.Any(), merged.workspaceID, merged.issueID, gomock.Any()).
		Return(entity.Issue{}, entity.IssueChecksUnprovenError{
			Checks: []entity.Check{{Statement: "payments retry without duplicating a charge"}},
		})

	var deferred entity.CodeTransitionBlock

	h.links.EXPECT().
		DeferTransition(gomock.Any(), merged.linkID, entity.CodeChangeMerged, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			_ entity.CodeChangeState,
			blockedBy entity.CodeTransitionBlock,
			_ time.Time,
		) error {
			deferred = blockedBy

			return nil
		})

	h.deliveries.EXPECT().
		Settle(gomock.Any(), merged.deliveryID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	if err := h.sync.Apply(context.Background(), merged.deliveryID); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if deferred != entity.CodeTransitionChecksUnproven {
		t.Fatalf(
			"a blocked merge recorded %q; without a deferred record the transition is burned and "+
				"the issue never advances even after the checks pass",
			deferred,
		)
	}
}

func TestABlockedTransitionAdvancesOnceTheWayIsClear(t *testing.T) {
	h := newAdvanceHarness(t)

	workspaceID := uuid.New()
	issueID := uuid.New()
	linkID := uuid.New()
	doneID := uuid.New()

	issue := entity.Issue{
		ID:          issueID,
		WorkspaceID: workspaceID,
		TeamID:      uuid.New(),
		Version:     4,
		State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}

	h.links.EXPECT().
		ListDeferredTransitions(gomock.Any(), issueID).
		Return([]entity.CodeTransition{{
			LinkID:     linkID,
			IssueID:    issueID,
			Transition: entity.CodeChangeMerged,
			StateID:    doneID,
			Status:     entity.CodeTransitionDeferred,
			BlockedBy:  entity.CodeTransitionChecksUnproven,
		}}, nil)

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()

	repositoryID := uuid.New()
	connectionID := uuid.New()
	ownerID := uuid.New()

	h.links.EXPECT().
		ListByIssue(gomock.Any(), workspaceID, issueID).
		Return([]entity.CodeLink{{
			ID:           linkID,
			WorkspaceID:  workspaceID,
			IssueID:      issueID,
			RepositoryID: repositoryID,
			Kind:         entity.CodeLinkChange,
			State:        entity.CodeChangeMerged,
			Resolving:    true,
		}}, nil)

	h.repositories.EXPECT().
		GetForDelivery(gomock.Any(), repositoryID).
		Return(entity.SCMRepository{
			ID:           repositoryID,
			ConnectionID: connectionID,
			WorkspaceID:  workspaceID,
			Provider:     entity.SCMProviderGitHub,
			FullName:     "acme/api",
		}, nil)

	h.connections.EXPECT().
		GetForDelivery(gomock.Any(), connectionID).
		Return(entity.SCMConnection{
			ID:             connectionID,
			WorkspaceID:    workspaceID,
			Provider:       entity.SCMProviderGitHub,
			OwnerAccountID: ownerID,
			Status:         entity.SCMConnectionConnected,
		}, nil)

	h.connections.EXPECT().Token(gomock.Any(), connectionID).Return("token", nil).AnyTimes()

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, ownerID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   ownerID,
			Role:        entity.MembershipRoleAdmin,
		}, nil)

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true, IncludePrivate: true},
			Role:  entity.MembershipRoleAdmin,
		}, nil)

	h.settings.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.AgentSettings{}, nil).
		AnyTimes()

	moved := false

	h.issueWriter.EXPECT().
		Update(gomock.Any(), workspaceID, issueID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.UpdateIssueInput,
		) (entity.Issue, error) {
			if input.StateID == nil || *input.StateID != doneID {
				t.Errorf("the resume moved the issue to %v, want the state the merge routed to", input.StateID)
			}

			moved = true

			return issue, nil
		})

	settled := false

	h.links.EXPECT().
		SettleTransition(gomock.Any(), linkID, entity.CodeChangeMerged).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ entity.CodeChangeState) error {
			settled = true

			return nil
		})

	if err := h.sync.Resume(context.Background(), workspaceID, issueID); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if !moved || !settled {
		t.Fatalf("resume moved=%v settled=%v, want the deferred transition applied and closed out", moved, settled)
	}
}

func TestResumingAnIssueThatAlreadyReachedTheStateJustClosesTheTransitionOut(t *testing.T) {
	h := newAdvanceHarness(t)

	workspaceID := uuid.New()
	issueID := uuid.New()
	linkID := uuid.New()
	doneID := uuid.New()

	h.links.EXPECT().
		ListDeferredTransitions(gomock.Any(), issueID).
		Return([]entity.CodeTransition{{
			LinkID:     linkID,
			IssueID:    issueID,
			Transition: entity.CodeChangeMerged,
			StateID:    doneID,
			Status:     entity.CodeTransitionDeferred,
		}}, nil)

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{
			ID:          issueID,
			WorkspaceID: workspaceID,
			State:       entity.IssueState{ID: doneID, Category: entity.StateCategoryComplete},
		}, nil)

	h.links.EXPECT().ListByIssue(gomock.Any(), workspaceID, issueID).Return(nil, nil)
	h.links.EXPECT().SettleTransition(gomock.Any(), linkID, entity.CodeChangeMerged).Return(nil)

	if err := h.sync.Resume(context.Background(), workspaceID, issueID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
}

func TestResumingAnIssueWithNothingWaitingDoesNothing(t *testing.T) {
	h := newAdvanceHarness(t)

	issueID := uuid.New()

	h.links.EXPECT().ListDeferredTransitions(gomock.Any(), issueID).Return(nil, nil)

	if err := h.sync.Resume(context.Background(), uuid.New(), issueID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
}
