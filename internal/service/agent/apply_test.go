package agent_test

import (
	"context"
	"errors"
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

	if _, err := h.service.Approve(context.Background(), h.workspaceID, held.ID, nil); err != nil {
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

func TestApprovingAHeldCreationFilesTheIssueTheAgentAsked(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	title := "Retries drop the idempotency key"
	description := "The second attempt sends a fresh key."
	priority := entity.IssuePriorityHigh
	labelID, teamID, agentID := uuid.New(), uuid.New(), uuid.New()

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     agentID,
		TeamID:      teamID,
		Action:      entity.AgentActionIssueCreate,
		Status:      entity.AgentProposalPending,
		Change: entity.AgentChange{
			Title:       &title,
			Description: &description,
			Priority:    &priority,
			LabelIDs:    []uuid.UUID{labelID},
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

	var filed service.CreateIssueInput

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
			filed = input

			return entity.Issue{ID: uuid.New()}, nil
		})

	if _, err := h.service.Approve(context.Background(), h.workspaceID, held.ID, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if filed.TeamID != teamID {
		t.Errorf("filed on team %v, want %v", filed.TeamID, teamID)
	}

	if filed.Title != title || filed.Description != description {
		t.Errorf(
			"approving filed %q / %q, want what the approver was shown",
			filed.Title, filed.Description,
		)
	}

	if filed.Priority != priority {
		t.Errorf("priority = %q, want %q", filed.Priority, priority)
	}

	if len(filed.LabelIDs) != 1 || filed.LabelIDs[0] != labelID {
		t.Errorf("labels = %v, want the one the agent chose", filed.LabelIDs)
	}
}

func TestApprovingAHeldDocEditWritesTheWholeDocAgainstTheVersionTheAgentRead(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID, docID := uuid.New(), uuid.New()

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     agentID,
		DocID:       docID,
		Action:      entity.AgentActionDocEdit,
		Status:      entity.AgentProposalPending,
		Change: entity.AgentChange{
			ExpectedVersion: 3,
			Doc:             &entity.AgentDocChange{Title: "Runbook", Markdown: "## Deploy\n\nPin the sha tag."},
		},
	}

	h.proposals.EXPECT().GetByID(gomock.Any(), h.workspaceID, held.ID).Return(held, nil).AnyTimes()
	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, WorkspaceID: h.workspaceID, AccountID: uuid.New(), Status: entity.AgentStatusActive}, nil)
	h.proposals.EXPECT().
		Settle(gomock.Any(), held.ID, entity.AgentProposalApplied, h.adminID, gomock.Any(), "").
		Return(nil)

	var applied service.UpdateDocInput

	h.docs.EXPECT().Update(gomock.Any(), h.workspaceID, docID, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID, input service.UpdateDocInput) (entity.Doc, error) {
			applied = input

			actor, _ := identity.Actor(ctx)
			if _, approved := identity.Approver(ctx); !approved || actor.Kind != entity.ActorKindAgent {
				t.Error("the replayed doc edit is not the agent's write carrying an approval")
			}

			return entity.Doc{}, nil
		})

	if _, err := h.service.Approve(context.Background(), h.workspaceID, held.ID, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if applied.ExpectedVersion != 3 || applied.Markdown == nil || *applied.Markdown != held.Change.Doc.Markdown ||
		applied.Title == nil || *applied.Title != "Runbook" || applied.Section != nil {
		t.Fatalf("the replay is not a full replace against version 3: %+v", applied)
	}
}

func TestApprovingAHeldDocCreationFilesTheDocWhereTheAgentAsked(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID, parentID, projectID := uuid.New(), uuid.New(), uuid.New()

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     agentID,
		Action:      entity.AgentActionDocCreate,
		Status:      entity.AgentProposalPending,
		Change: entity.AgentChange{Doc: &entity.AgentDocChange{
			Title:      "Postmortem",
			Markdown:   "What happened.",
			ParentID:   &parentID,
			Visibility: entity.KnowledgeVisibilityProject,
			ProjectID:  &projectID,
		}},
	}

	h.proposals.EXPECT().GetByID(gomock.Any(), h.workspaceID, held.ID).Return(held, nil).AnyTimes()
	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, WorkspaceID: h.workspaceID, AccountID: uuid.New(), Status: entity.AgentStatusActive}, nil)
	h.proposals.EXPECT().
		Settle(gomock.Any(), held.ID, entity.AgentProposalApplied, h.adminID, gomock.Any(), "").
		Return(nil)

	var created service.CreateDocInput

	h.docs.EXPECT().Create(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.CreateDocInput) (entity.Doc, error) {
			created = input

			return entity.Doc{}, nil
		})

	if _, err := h.service.Approve(context.Background(), h.workspaceID, held.ID, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if created.Title != "Postmortem" || created.Markdown != "What happened." ||
		created.ParentID == nil || *created.ParentID != parentID ||
		created.Audience != entity.ProjectAudience(projectID) {
		t.Fatalf("the approved doc is not the one the agent asked for: %+v", created)
	}
}

func TestAHeldDocWriteIsTakenWholeOrNotAtAll(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     uuid.New(),
		DocID:       uuid.New(),
		Action:      entity.AgentActionDocEdit,
		Status:      entity.AgentProposalPending,
		Change:      entity.AgentChange{ExpectedVersion: 1, Doc: &entity.AgentDocChange{Title: "Runbook"}},
	}

	h.proposals.EXPECT().GetByID(gomock.Any(), h.workspaceID, held.ID).Return(held, nil).AnyTimes()

	var invalid entity.ValidationError
	if _, err := h.service.Approve(
		context.Background(), h.workspaceID, held.ID, []entity.ChangePart{entity.ChangePartTitle},
	); !errors.As(err, &invalid) {
		t.Fatalf("taking part of a doc write got %v, want it refused", err)
	}
}

func TestADocProposalWaitsBesideTheDocAsItIsNow(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	docID := uuid.New()

	held := entity.AgentProposal{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     uuid.New(),
		DocID:       docID,
		Action:      entity.AgentActionDocEdit,
		Status:      entity.AgentProposalPending,
		Change:      entity.AgentChange{ExpectedVersion: 1, Doc: &entity.AgentDocChange{Title: "Runbook", Markdown: "New."}},
	}

	h.proposals.EXPECT().ListWaiting(gomock.Any(), h.workspaceID, gomock.Any()).Return([]entity.AgentProposal{held}, nil)
	h.docs.EXPECT().Get(gomock.Any(), h.workspaceID, docID).
		Return(entity.Doc{DocSummary: entity.DocSummary{ID: docID, Title: "Runbook"}, Markdown: "Old."}, nil)

	waiting, err := h.service.Waiting(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("waiting: %v", err)
	}

	if len(waiting) != 1 || waiting[0].Doc.Markdown != "Old." || waiting[0].Team.ID != uuid.Nil {
		t.Fatalf("a doc proposal without a team was not shown beside its doc: %+v", waiting)
	}
}

func TestAWorkspaceSaysWhetherItHoldsDocWritesOnlyInWordsItKnows(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	var invalid entity.ValidationError
	if _, err := h.service.ConfigureWorkspace(context.Background(), service.ConfigureWorkspaceAgentsInput{
		WorkspaceID:   h.workspaceID,
		HoldDocWrites: "sometimes",
	}); !errors.As(err, &invalid) {
		t.Fatalf("an unknown hold got %v", err)
	}

	h.workspace.EXPECT().
		Upsert(gomock.Any(), entity.WorkspaceAgentSettings{WorkspaceID: h.workspaceID, HoldDocWrites: entity.AgentHoldAlways}).
		DoAndReturn(func(_ context.Context, settings entity.WorkspaceAgentSettings) (entity.WorkspaceAgentSettings, error) {
			return settings, nil
		})

	saved, err := h.service.ConfigureWorkspace(context.Background(), service.ConfigureWorkspaceAgentsInput{
		WorkspaceID:   h.workspaceID,
		HoldDocWrites: entity.AgentHoldAlways,
	})
	if err != nil || saved.HoldDocWrites != entity.AgentHoldAlways {
		t.Fatalf("configure: %+v, %v", saved, err)
	}
}
