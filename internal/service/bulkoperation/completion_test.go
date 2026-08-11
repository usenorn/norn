package bulkoperation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) completingBulk(
	t *testing.T,
	workspaceID uuid.UUID,
	issues []entity.Issue,
	done entity.WorkflowState,
) {
	t.Helper()

	h.actions.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, action entity.BulkAction) (entity.BulkAction, error) {
			action.ID = uuid.New()

			return action, nil
		})

	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().GetByID(gomock.Any(), workspaceID, gomock.Any()).
		Return(entity.BulkAction{WorkspaceID: workspaceID}, nil).AnyTimes()

	h.states.EXPECT().
		ListByTeamID(gomock.Any(), gomock.Any()).
		Return([]entity.WorkflowState{done}, nil).
		AnyTimes()

	for _, issue := range issues {
		h.issues.EXPECT().
			LockByID(gomock.Any(), workspaceID, issue.ID, gomock.Any()).
			Return(issue, nil).
			AnyTimes()
	}

	h.issues.EXPECT().
		ListChildren(gomock.Any(), workspaceID, gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	h.issues.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

func TestABulkCloseIsRefusedPerIssueAndTheRestStillApply(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})
	h.actor = entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()}

	done := entity.WorkflowState{
		ID:       uuid.New(),
		TeamID:   teamID,
		Name:     "Done",
		Category: entity.StateCategoryComplete,
	}

	blocked := entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     1,
		Status:      entity.IssueStatusActive,
		State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}

	clear := entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Version:     1,
		Status:      entity.IssueStatusActive,
		State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}

	h.completingBulk(t, workspaceID, []entity.Issue{blocked, clear}, done)

	h.blocking = map[uuid.UUID][]entity.Check{
		blocked.ID: {{
			ID:        uuid.New(),
			Statement: "payments retry without duplicating a charge",
			Approval:  entity.CheckApprovalApproved,
		}},
	}

	outcomes := map[uuid.UUID]entity.BulkOutcome{}

	h.actions.EXPECT().
		RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, recorded []entity.BulkActionOutcome) error {
			for _, outcome := range recorded {
				outcomes[outcome.IssueID] = outcome.Outcome
			}

			return nil
		}).
		AnyTimes()

	if _, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{blocked.ID, clear.ID}},
		Change: entity.BulkChange{StateID: &done.ID},
	}); err != nil {
		t.Fatalf("bulk apply returned %v, want it to run and report per issue", err)
	}

	if outcomes[blocked.ID] != entity.BulkOutcomeConflict {
		t.Fatalf(
			"the issue with an unproven check was recorded as %q, want a conflict",
			outcomes[blocked.ID],
		)
	}

	if outcomes[clear.ID] != entity.BulkOutcomeApplied {
		t.Fatalf(
			"the issue with nothing in the way was recorded as %q; one blocked issue must not "+
				"take the rest of the batch down with it",
			outcomes[clear.ID],
		)
	}
}
