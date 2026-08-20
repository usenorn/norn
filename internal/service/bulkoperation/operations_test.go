package bulkoperation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	bulkrepo "github.com/usenorn/norn/internal/repository/bulkaction"
	checkrepo "github.com/usenorn/norn/internal/repository/check"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	bulksvc "github.com/usenorn/norn/internal/service/bulkoperation"
	"github.com/usenorn/norn/internal/service/checkgate"
)

type harness struct {
	actions      *bulkrepo.MockBulkAction
	issues       *issuerepo.MockIssue
	states       *workflowstaterepo.MockWorkflowState
	labels       *labelrepo.MockLabel
	activity     *activityrepo.MockActivity
	members      *membershiprepo.MockMembership
	accounts     *accountrepo.MockAccount
	cycles       *cyclerepo.MockCycle
	scopeChanges *cyclerepo.MockCycleScopeChange
	jobs         *jobqueuerepo.MockJobProducer
	checks       *checkrepo.MockCheck
	evidence     *checkrepo.MockCheckEvidence
	codeLinks    *scmrepo.MockCodeLink
	authorizer   *authorizersvc.MockAuthorizer
	actor        entity.Actor
	blocking     map[uuid.UUID][]entity.Check
	service      service.BulkOperations
}

func newHarness(t *testing.T, scope entity.TeamScope) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	tx := transactorrepo.NewMockTransactor(ctrl)

	tx.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h := &harness{
		actions:      bulkrepo.NewMockBulkAction(ctrl),
		issues:       issuerepo.NewMockIssue(ctrl),
		states:       workflowstaterepo.NewMockWorkflowState(ctrl),
		labels:       labelrepo.NewMockLabel(ctrl),
		activity:     activityrepo.NewMockActivity(ctrl),
		members:      membershiprepo.NewMockMembership(ctrl),
		accounts:     accountrepo.NewMockAccount(ctrl),
		cycles:       cyclerepo.NewMockCycle(ctrl),
		scopeChanges: cyclerepo.NewMockCycleScopeChange(ctrl),
		jobs:         jobqueuerepo.NewMockJobProducer(ctrl),
		checks:       checkrepo.NewMockCheck(ctrl),
		evidence:     checkrepo.NewMockCheckEvidence(ctrl),
		codeLinks:    scmrepo.NewMockCodeLink(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
		actor:        entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
	}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{Actor: h.actor, Scope: scope}, nil
		}).
		AnyTimes()

	h.service = bulksvc.New(
		h.actions, h.issues, h.states, h.labels, h.activity, h.members, h.accounts,
		h.cycles, h.scopeChanges, h.jobs, checkgate.New(h.checks, h.evidence, h.codeLinks), h.authorizer, tx,
	)

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, issueID uuid.UUID) ([]entity.Check, error) {
			return h.blocking[issueID], nil
		}).
		AnyTimes()

	h.evidence.EXPECT().
		Digest(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	h.codeLinks.EXPECT().
		ListByIssue(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	return h
}

func (h *harness) expectAction(workspaceID uuid.UUID, expected *int) uuid.UUID {
	id := uuid.New()

	h.actions.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, a entity.BulkAction) (entity.BulkAction, error) {
			a.ID = id
			a.Status = entity.BulkActionQueued
			a.Expected = expected

			return a, nil
		})

	return id
}

func issueOn(teamID uuid.UUID, number int) entity.Issue {
	return entity.Issue{
		ID:           uuid.New(),
		TeamID:       teamID,
		ReferenceKey: "MOB",
		Number:       number,
		Version:      1,
		Status:       entity.IssueStatusActive,
		Priority:     entity.IssuePriorityNone,
		State:        entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
}

func TestABatchAppliesWhatItCanAndReportsEachFailureIndividually(t *testing.T) {
	workspaceID, mine, theirs := uuid.New(), uuid.New(), uuid.New()

	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{mine}})

	ok := issueOn(mine, 1)
	stale := issueOn(mine, 2)
	hidden := issueOn(theirs, 3)
	missing := uuid.New()

	h.expectAction(workspaceID, ptr(4))

	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, ok.ID, gomock.Any()).Return(ok, nil)
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, stale.ID, gomock.Any()).Return(stale, nil)
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, hidden.ID, gomock.Any()).Return(hidden, nil)
	h.issues.EXPECT().
		LockByID(gomock.Any(), workspaceID, missing, gomock.Any()).
		Return(entity.Issue{}, entity.ErrIssueNotFound)

	h.issues.EXPECT().Update(gomock.Any(), ok.ID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.issues.EXPECT().
		Update(gomock.Any(), stale.ID, 1, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.ErrIssueStale)

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), entity.BulkActionComplete, gomock.Any()).Return(nil)

	urgent := entity.IssuePriorityUrgent

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{Priority: &urgent},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{ok.ID, stale.ID, hidden.ID, missing}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := map[uuid.UUID]entity.BulkOutcome{}
	for _, outcome := range result.Outcomes {
		got[outcome.IssueID] = outcome.Outcome
	}

	want := map[uuid.UUID]entity.BulkOutcome{
		ok.ID:     entity.BulkOutcomeApplied,
		stale.ID:  entity.BulkOutcomeConflict,
		hidden.ID: entity.BulkOutcomeForbidden,
		missing:   entity.BulkOutcomeNotFound,
	}

	for id, expected := range want {
		if got[id] != expected {
			t.Errorf("issue %s reported %q, want %q", id, got[id], expected)
		}
	}

	if len(result.Outcomes) != 4 {
		t.Fatalf(
			"%d outcomes for a batch of four. Every issue must be accounted for, or the caller "+
				"cannot tell which ones silently did nothing.",
			len(result.Outcomes),
		)
	}
}

func TestAnIssueOnATeamOutsideTheScopeIsRefusedIndividuallyNotForTheWholeBatch(t *testing.T) {
	workspaceID, mine, theirs := uuid.New(), uuid.New(), uuid.New()

	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{mine}})

	ok := issueOn(mine, 1)
	hidden := issueOn(theirs, 2)

	h.expectAction(workspaceID, ptr(2))
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, ok.ID, gomock.Any()).Return(ok, nil)
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, hidden.ID, gomock.Any()).Return(hidden, nil)
	h.issues.EXPECT().Update(gomock.Any(), ok.ID, 1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	low := entity.IssuePriorityLow

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{Priority: &low},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{ok.ID, hidden.ID}},
	})
	if err != nil {
		t.Fatalf(
			"the whole batch failed with %v. One issue the actor cannot touch must not stop the "+
				"rest from being applied.",
			err,
		)
	}

	for _, outcome := range result.Outcomes {
		if outcome.IssueID == hidden.ID && outcome.Outcome != entity.BulkOutcomeForbidden {
			t.Errorf("an out-of-scope issue reported %q, want forbidden", outcome.Outcome)
		}

		if outcome.IssueID == ok.ID && !outcome.Outcome.Applied() {
			t.Errorf("the in-scope issue reported %q, want it applied", outcome.Outcome)
		}
	}
}

func TestABulkAssignmentToAnAgentIsRefused(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()

	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	target := issueOn(teamID, 1)
	agentID := uuid.New()

	h.expectAction(workspaceID, ptr(1))
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, target.ID, gomock.Any()).Return(target, nil)
	h.members.EXPECT().
		Get(gomock.Any(), workspaceID, agentID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: agentID}, nil)
	h.accounts.EXPECT().
		GetByID(gomock.Any(), agentID).
		Return(entity.Account{ID: agentID, Kind: entity.AccountKindAgent}, nil)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{AssigneeID: &agentID},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{target.ID}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(result.Outcomes) != 1 || result.Outcomes[0].Outcome != entity.BulkOutcomeInvalid {
		t.Fatalf("outcomes = %+v, want the agent assignment reported invalid", result.Outcomes)
	}
}

func TestASetBeyondTheInlineLimitBecomesAJobInsteadOfRunningNow(t *testing.T) {
	workspaceID := uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	ids := make([]uuid.UUID, entity.BulkSyncLimit+1)
	for i := range ids {
		ids[i] = uuid.New()
	}

	h.expectAction(workspaceID, ptr(len(ids)))

	var queued entity.BulkApplyPayload

	h.jobs.EXPECT().
		EnqueueBulkApply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p entity.BulkApplyPayload) error {
			queued = p

			return nil
		})

	urgent := entity.IssuePriorityUrgent

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{Priority: &urgent},
		Set:    entity.BulkSet{IssueIDs: ids},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(queued.IssueIDs) != len(ids) {
		t.Fatalf("the job was queued with %d ids, want %d", len(queued.IssueIDs), len(ids))
	}

	if len(result.Outcomes) != 0 {
		t.Fatal("a queued action returned outcomes; nothing has been applied yet")
	}

	if result.Action.Status != entity.BulkActionQueued {
		t.Fatalf("queued action reports status %q", result.Action.Status)
	}
}

func TestAFilterSetIsAlwaysAJobEvenWhenItWouldFitInline(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	h.expectAction(workspaceID, nil)

	var queued entity.BulkApplyPayload

	h.jobs.EXPECT().
		EnqueueBulkApply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p entity.BulkApplyPayload) error {
			queued = p

			return nil
		})

	urgent := entity.IssuePriorityUrgent

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{Priority: &urgent},
		Set:    entity.BulkSet{Filter: &entity.BulkFilter{TeamID: &teamID}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if queued.Filter == nil {
		t.Fatal(
			"a filter set ran inline. Its size is unknown until it is walked, and discovering it " +
				"first would mean counting the issues that match.",
		)
	}

	if result.Action.Expected != nil {
		t.Fatal("a filter action claims an expected size, so its progress would render as a percentage of a guess")
	}
}

func TestEveryChangeAnIssueReceivesIsAttributedToTheOneBulkAction(t *testing.T) {
	workspaceID, teamID := uuid.New(), uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	first, second := issueOn(teamID, 1), issueOn(teamID, 2)
	actionID := h.expectAction(workspaceID, ptr(2))

	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, first.ID, gomock.Any()).Return(first, nil)
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, second.ID, gomock.Any()).Return(second, nil)
	h.issues.EXPECT().Update(gomock.Any(), gomock.Any(), 1, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)
	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	attributed := map[uuid.UUID]uuid.UUID{}

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.Activity) error {
			attributed[entry.Subject.ID] = entry.BulkActionID

			return nil
		}).
		Times(2)

	urgent := entity.IssuePriorityUrgent

	if _, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{Priority: &urgent},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{first.ID, second.ID}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, issue := range []entity.Issue{first, second} {
		if attributed[issue.ID] != actionID {
			t.Errorf(
				"issue %s attributes its change to %s, want the one action %s. Without it a history "+
					"shows an unexplained change rather than part of a bulk edit.",
				issue.ID, attributed[issue.ID], actionID,
			)
		}
	}
}

func TestMovingABatchIntoACycleCountsAsScopeAddedToAStartedCycle(t *testing.T) {
	workspaceID, teamID, cycleID := uuid.New(), uuid.New(), uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	issue := issueOn(teamID, 1)

	h.expectAction(workspaceID, ptr(1))
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issue.ID, gomock.Any()).Return(issue, nil)
	h.cycles.EXPECT().
		GetVisible(gomock.Any(), workspaceID, cycleID, gomock.Any()).
		Return(entity.Cycle{
			ID: cycleID, WorkspaceID: workspaceID, TeamID: teamID, Number: 24, StartsOn: "2020-01-01",
		}, nil)

	var joined *uuid.UUID

	h.issues.EXPECT().
		Update(gomock.Any(), issue.ID, 1, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ int, change entity.IssueChange, _ *entity.StateTimestamps, _ time.Time) error {
			joined = change.CycleID

			return nil
		})

	var recorded entity.CycleScopeChange

	h.scopeChanges.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			recorded = change

			return nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{CycleID: &cycleID},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{issue.ID}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if joined == nil || *joined != cycleID {
		t.Fatalf("the issue joined %v, want cycle %v", joined, cycleID)
	}

	if recorded.CycleID != cycleID || recorded.Change != entity.CycleScopeChangeAdded {
		t.Fatalf(
			"scope change = %+v, want work added to cycle %v. A cycle already under way has to "+
				"show what arrived after it started.",
			recorded, cycleID,
		)
	}
}

func TestAnIssueIsRefusedACycleBelongingToAnotherTeamWithoutFailingTheBatch(t *testing.T) {
	workspaceID, mine, theirs, cycleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	issue := issueOn(mine, 1)

	h.expectAction(workspaceID, ptr(1))
	h.issues.EXPECT().LockByID(gomock.Any(), workspaceID, issue.ID, gomock.Any()).Return(issue, nil)
	h.cycles.EXPECT().
		GetVisible(gomock.Any(), workspaceID, cycleID, gomock.Any()).
		Return(entity.Cycle{ID: cycleID, WorkspaceID: workspaceID, TeamID: theirs, Number: 24}, nil)

	h.actions.EXPECT().RecordOutcomes(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Advance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.actions.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.actions.EXPECT().Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	result, err := h.service.Apply(context.Background(), workspaceID, service.ApplyBulkInput{
		Change: entity.BulkChange{CycleID: &cycleID},
		Set:    entity.BulkSet{IssueIDs: []uuid.UUID{issue.ID}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(result.Outcomes) != 1 || result.Outcomes[0].Outcome == entity.BulkOutcomeApplied {
		t.Fatalf("outcomes = %+v, want the one issue reported as not applied", result.Outcomes)
	}
}

func TestAnAlreadySettledActionIsNotRunAgainWhenTheTaskIsRedelivered(t *testing.T) {
	workspaceID, actionID := uuid.New(), uuid.New()
	h := newHarness(t, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	h.actions.EXPECT().
		GetByID(gomock.Any(), workspaceID, actionID).
		Return(entity.BulkAction{
			ID:          actionID,
			WorkspaceID: workspaceID,
			Status:      entity.BulkActionComplete,
			FinishedAt:  ptrTime(time.Now()),
		}, nil)

	if err := h.service.Run(context.Background(), entity.BulkApplyPayload{
		BulkActionID: actionID,
		WorkspaceID:  workspaceID,
		IssueIDs:     []uuid.UUID{uuid.New()},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func ptr(n int) *int { return &n }

func ptrTime(t time.Time) *time.Time { return &t }
