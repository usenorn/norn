package agent_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func TestAnApprovedProposalAppliesEveryFieldTheAgentAsked(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	title := "Retries keep the idempotency key"
	description := "The retry path rebuilds the request and loses the header."
	estimate := 3
	stateID := uuid.New()
	agentID := uuid.New()
	issueID := uuid.New()

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     agentID,
		IssueID:     issueID,
		TeamID:      uuid.New(),
		Action:      entity.AgentActionIssueEdit,
		Status:      entity.AgentProposalPending,
		Change: entity.AgentChange{
			ExpectedVersion: 4,
			StateID:         &stateID,
			Title:           &title,
			Description:     &description,
			Estimate:        &estimate,
		},
	}

	h.proposals.EXPECT().GetByID(gomock.Any(), h.workspaceID, held.ID).Return(held, nil).AnyTimes()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:          agentID,
			WorkspaceID: h.workspaceID,
			AccountID:   uuid.New(),
			Status:      entity.AgentStatusActive,
		}, nil)

	h.proposals.EXPECT().
		Settle(gomock.Any(), held.ID, entity.AgentProposalApplied, h.adminID, gomock.Any(), "").
		Return(nil)

	var applied service.UpdateIssueInput

	h.issues.EXPECT().
		Update(gomock.Any(), h.workspaceID, issueID, gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, _, _ uuid.UUID, input service.UpdateIssueInput,
		) (entity.Issue, error) {
			applied = input

			approver, approved := identity.Approver(ctx)
			if !approved {
				t.Error("the replayed write carried no approval, so the gate would refuse it")
			}

			if approver != h.adminID {
				t.Errorf("the approval named %s, want the person who approved it", approver)
			}

			return entity.Issue{}, nil
		})

	if _, err := h.service.Approve(context.Background(), h.workspaceID, held.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if applied.Description == nil || *applied.Description != description {
		t.Fatal(
			"approving dropped the description, so the issue ends up different from what the " +
				"agent asked for and what the approver was shown",
		)
	}

	if applied.Title == nil || *applied.Title != title {
		t.Fatal("approving dropped the title")
	}

	if applied.Estimate == nil || *applied.Estimate != estimate {
		t.Fatal("approving dropped the estimate")
	}

	if applied.StateID == nil || *applied.StateID != stateID {
		t.Fatal("approving dropped the state change")
	}

	if applied.ExpectedVersion != held.Change.ExpectedVersion {
		t.Fatalf(
			"approving offered version %d, want the %d the agent read: without it a person "+
				"editing since is overwritten rather than left alone",
			applied.ExpectedVersion, held.Change.ExpectedVersion,
		)
	}
}

func TestAnApprovedStateChangeCarriesThePermissionTheWriteChecks(t *testing.T) {
	held := entity.AgentProposal{Action: entity.AgentActionStateChange}

	acting := entity.Actor{Kind: entity.ActorKindAgent, Scopes: held.Action.Scopes()}

	if !acting.Holds(entity.NewPermission(entity.ResourceIssue, entity.ActionManage)) {
		t.Fatal(
			"an approved state change does not carry issue:manage, which is what the write " +
				"authorises against, so approving one is refused",
		)
	}
}
