package triage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	triagesvc "github.com/usenorn/norn/internal/service/triage"
)

type harness struct {
	triage     *triagerepo.MockTriage
	issues     *issuerepo.MockIssue
	states     *workflowstaterepo.MockWorkflowState
	activity   *activityrepo.MockActivity
	teams      *teamrepo.MockTeam
	relations  *issuerelationsvc.MockIssueRelations
	issueMoves *issuesvc.MockIssues
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.Triages

	workspaceID uuid.UUID
	teamID      uuid.UUID
	issueID     uuid.UUID
	actorID     uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		triage:      triagerepo.NewMockTriage(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		states:      workflowstaterepo.NewMockWorkflowState(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		relations:   issuerelationsvc.NewMockIssueRelations(ctrl),
		issueMoves:  issuesvc.NewMockIssues(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		teamID:      uuid.New(),
		issueID:     uuid.New(),
		actorID:     uuid.New(),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.actorID},
			Role:  entity.MembershipRoleAdmin,
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()

	h.service = triagesvc.New(
		h.triage, h.issues, h.states, h.activity, h.teams,
		h.relations, h.issueMoves, h.authorizer, h.transactor,
	)

	return h
}

func (h *harness) waiting() entity.Issue {
	return entity.Issue{
		ID:           h.issueID,
		WorkspaceID:  h.workspaceID,
		TeamID:       h.teamID,
		ReferenceKey: "MOB",
		Number:       14,
		Version:      1,
		TriageState:  entity.TriageStateWaiting,
		TriageSource: entity.ActorKindToken,
		State:        entity.IssueState{ID: uuid.New(), Name: "Todo", Category: entity.StateCategoryNotStarted},
	}
}

func (h *harness) holds(issue entity.Issue) {
	h.issues.EXPECT().LockByID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(issue, nil).AnyTimes()
	h.issues.EXPECT().GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(issue, nil).AnyTimes()
}

func (h *harness) abandonedState() entity.WorkflowState {
	return entity.WorkflowState{
		ID: uuid.New(), TeamID: h.teamID, Name: "Canceled",
		Category: entity.StateCategoryAbandoned, Position: 6,
	}
}

func (h *harness) captureDecision() *entity.Activity {
	recorded := &entity.Activity{}

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.Activity) error {
			if entry.Kind == entity.ActivityKindTriaged {
				*recorded = entry
			}

			return nil
		}).
		AnyTimes()

	return recorded
}

func (h *harness) expectDecided(state entity.TriageState) {
	h.triage.EXPECT().
		Decide(gomock.Any(), h.issueID, state, h.actorID, gomock.Any()).
		Return(nil)
}

func TestAcceptingLetsTheIssueThroughAndSaysWhoDecided(t *testing.T) {
	h := newHarness(t)
	h.holds(h.waiting())
	h.expectDecided(entity.TriageStateAccepted)

	recorded := h.captureDecision()

	if _, err := h.service.Accept(context.Background(), h.workspaceID, h.issueID); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if recorded.Kind != entity.ActivityKindTriaged {
		t.Fatal(
			"accepting an issue recorded no triage activity. The reporter may ask what happened " +
				"to it, and the answer has to be written down at the moment it is decided.",
		)
	}

	if recorded.Field != string(entity.TriageStateAccepted) {
		t.Errorf("the activity names the decision as %q, want %q", recorded.Field, entity.TriageStateAccepted)
	}

	if recorded.Actor.AccountID != h.actorID {
		t.Error("the activity does not name who decided")
	}
}

func TestDecliningClosesTheIssueWithoutDeletingIt(t *testing.T) {
	h := newHarness(t)
	issue := h.waiting()
	abandoned := h.abandonedState()

	h.holds(issue)
	h.states.EXPECT().
		ListByTeamID(gomock.Any(), h.teamID).
		Return([]entity.WorkflowState{abandoned}, nil)
	h.expectDecided(entity.TriageStateDeclined)

	var moved *uuid.UUID

	h.issues.EXPECT().
		Update(gomock.Any(), h.issueID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, _ int, change entity.IssueChange,
			_ *entity.StateTimestamps, _ time.Time,
		) error {
			moved = change.StateID

			return nil
		})

	recorded := h.captureDecision()

	if _, err := h.service.Decline(context.Background(), h.workspaceID, h.issueID); err != nil {
		t.Fatalf("Decline: %v", err)
	}

	if moved == nil || *moved != abandoned.ID {
		t.Fatalf(
			"declining did not move the issue to the team's abandoned state (moved to %v). "+
				"Declining is not deletion: the issue stays, keeps its reference, and reads as "+
				"closed so whoever reported it can still be told what happened.",
			moved,
		)
	}

	if recorded.Field != string(entity.TriageStateDeclined) {
		t.Errorf("the activity names the decision as %q, want declined", recorded.Field)
	}
}

func TestDecliningIsRefusedWhenTheTeamHasNowhereToPutIt(t *testing.T) {
	h := newHarness(t)
	h.holds(h.waiting())
	h.states.EXPECT().
		ListByTeamID(gomock.Any(), h.teamID).
		Return([]entity.WorkflowState{{
			ID: uuid.New(), Name: "Todo", Category: entity.StateCategoryNotStarted,
		}}, nil)

	if _, err := h.service.Decline(
		context.Background(), h.workspaceID, h.issueID,
	); !errors.Is(err, entity.ErrIssueDestinationIncapable) {
		t.Fatalf(
			"declining into a team with no abandoned state returned %v. Silently leaving it open "+
				"would mean the queue reports it decided while the board still shows it as work.",
			err,
		)
	}
}

func TestMergingRecordsTheDuplicateInTheDirectionThatSaysWhichSurvives(t *testing.T) {
	h := newHarness(t)
	survivorID := uuid.New()

	h.holds(h.waiting())
	h.expectDecided(entity.TriageStateMerged)

	var asked service.AddIssueRelationInput

	var onIssue uuid.UUID

	h.relations.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, issueID uuid.UUID, input service.AddIssueRelationInput,
		) (entity.IssueRelation, error) {
			onIssue, asked = issueID, input

			return entity.IssueRelation{}, nil
		})

	recorded := h.captureDecision()

	if _, err := h.service.Merge(
		context.Background(), h.workspaceID, h.issueID, survivorID,
	); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if onIssue != h.issueID || asked.CounterpartID != survivorID {
		t.Fatalf(
			"the relation was recorded from %s to %s, want the waiting issue duplicating the "+
				"survivor. The direction is what says which one lives on.",
			onIssue, asked.CounterpartID,
		)
	}

	if asked.Kind != entity.IssueRelationViewDuplicates {
		t.Errorf("the relation kind is %q, want duplicates", asked.Kind)
	}

	if !asked.CloseDuplicate {
		t.Error(
			"the duplicate was left open. Merging that leaves the duplicate on the board is not a " +
				"merge, it is two issues saying the same thing.",
		)
	}

	if recorded.ToValue == "" {
		t.Error("the activity does not name what it was merged into, so the trail stops here")
	}
}

func TestADecisionIsRefusedOnAnythingThatIsNotWaiting(t *testing.T) {
	for name, state := range map[string]entity.TriageState{
		"already accepted": entity.TriageStateAccepted,
		"already declined": entity.TriageStateDeclined,
		"never triaged":    "",
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			issue := h.waiting()
			issue.TriageState = state
			h.holds(issue)

			if _, err := h.service.Accept(
				context.Background(), h.workspaceID, h.issueID,
			); !errors.Is(err, entity.ErrIssueNotWaiting) {
				t.Fatalf(
					"deciding on an issue that is %s returned %v. Two people working the queue "+
						"would otherwise both report success and the second decision would "+
						"quietly overwrite the first.",
					name, err,
				)
			}
		})
	}
}

func TestReassigningToATeamWithoutTriageAcceptsItThere(t *testing.T) {
	h := newHarness(t)
	destination := uuid.New()

	h.holds(h.waiting())
	h.issueMoves.EXPECT().
		MoveToTeam(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.waiting(), nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), destination).
		Return(entity.Team{ID: destination, WorkspaceID: h.workspaceID, Name: "Platform"}, nil)
	h.triage.EXPECT().
		Settings(gomock.Any(), h.workspaceID, destination).
		Return(entity.TriageSettings{}, entity.ErrTriageDisabled)
	h.expectDecided(entity.TriageStateAccepted)

	h.captureDecision()

	if _, err := h.service.Reassign(
		context.Background(), h.workspaceID, h.issueID,
		service.ReassignTriageInput{TeamID: destination, ExpectedVersion: 1},
	); err != nil {
		t.Fatalf(
			"reassigning to a team that does not triage failed: %v\nOtherwise the issue waits "+
				"forever in a queue nobody looks at, because that team has no queue.",
			err,
		)
	}
}

func TestReassigningToATeamThatTriagesLeavesItWaitingThere(t *testing.T) {
	h := newHarness(t)
	destination := uuid.New()

	h.holds(h.waiting())
	h.issueMoves.EXPECT().
		MoveToTeam(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.waiting(), nil)
	h.teams.EXPECT().
		GetByID(gomock.Any(), destination).
		Return(entity.Team{ID: destination, WorkspaceID: h.workspaceID, Name: "Platform"}, nil)
	h.triage.EXPECT().
		Settings(gomock.Any(), h.workspaceID, destination).
		Return(entity.TriageSettings{TeamID: destination}, nil)

	recorded := h.captureDecision()

	if _, err := h.service.Reassign(
		context.Background(), h.workspaceID, h.issueID,
		service.ReassignTriageInput{TeamID: destination, ExpectedVersion: 1},
	); err != nil {
		t.Fatalf("Reassign: %v", err)
	}

	if recorded.Kind != entity.ActivityKindTriaged || recorded.ToValue != "Platform" {
		t.Fatalf(
			"handing the issue to another team recorded %+v. Passing something on is a decision "+
				"someone made and the next team will want to know who sent it.",
			recorded,
		)
	}
}

func TestTheQueueOnlyEverAsksForWaitingWork(t *testing.T) {
	h := newHarness(t)

	var asked entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			asked = page

			return nil, nil
		})

	h.issues.EXPECT().
		TallyByGroup(gomock.Any(), gomock.Any(), gomock.Any(), entity.IssueGroupByTeam).
		Return(nil, nil)

	if _, err := h.service.Queue(
		context.Background(), h.workspaceID, service.TriageQueueInput{},
	); err != nil {
		t.Fatalf("Queue: %v", err)
	}

	if !asked.Waiting {
		t.Fatal(
			"the queue asked for the ordinary backlog. The queue and the backlog are the same " +
				"engine asking opposite questions, and without the flag it shows the wrong one.",
		)
	}
}
