package cycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
)

type harness struct {
	cycles        *cyclerepo.MockCycle
	cadences      *cyclerepo.MockCycleCadence
	scope         *cyclerepo.MockCycleScopeChange
	results       *cyclerepo.MockCycleResult
	activity      *activityrepo.MockActivity
	states        *workflowstaterepo.MockWorkflowState
	members       *teammemberrepo.MockTeamMember
	frozen        []entity.CycleResult
	acquired      []string
	workspaceErr  error
	recordingFrom time.Time
	issues        *issuerepo.MockIssue
	teams         *teamrepo.MockTeam
	workspaces    *workspacerepo.MockWorkspace
	authorizer    *authorizersvc.MockAuthorizer
	transactor    *transactorrepo.MockTransactor
	service       service.Cycles

	workspaceID uuid.UUID
	teamID      uuid.UUID
	accountID   uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		cycles:      cyclerepo.NewMockCycle(ctrl),
		cadences:    cyclerepo.NewMockCycleCadence(ctrl),
		scope:       cyclerepo.NewMockCycleScopeChange(ctrl),
		results:     cyclerepo.NewMockCycleResult(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		states:      workflowstaterepo.NewMockWorkflowState(ctrl),
		members:     teammemberrepo.NewMockTeamMember(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		workspaces:  workspacerepo.NewMockWorkspace(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		teamID:      uuid.New(),
		accountID:   uuid.New(),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.results.EXPECT().
		RecordingFrom(gomock.Any()).
		DoAndReturn(func(_ context.Context) (time.Time, error) { return h.recordingFrom, nil }).
		AnyTimes()

	h.results.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, results []entity.CycleResult) error {
			h.frozen = append(h.frozen, results...)

			return nil
		}).
		AnyTimes()

	h.states.EXPECT().
		ShareByTeamID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, teamID uuid.UUID) ([]entity.WorkflowState, error) {
			h.acquired = append(h.acquired, "states "+teamID.String())

			return nil, nil
		}).
		AnyTimes()

	h.service = cyclesvc.New(
		h.cycles, h.cadences, h.scope, h.results, h.activity, h.states, h.members, h.issues,
		h.teams, h.workspaces, h.authorizer, silentEmitter(ctrl), h.transactor,
	)

	return h
}

func (h *harness) locks(cycles ...entity.Cycle) {
	for _, cycle := range cycles {
		h.cycles.EXPECT().
			GetVisible(gomock.Any(), h.workspaceID, cycle.ID, gomock.Any()).
			Return(cycle, nil).
			AnyTimes()
		h.cycles.EXPECT().
			LockByID(gomock.Any(), cycle.ID).
			DoAndReturn(func(_ context.Context, cycleID uuid.UUID) (entity.Cycle, error) {
				h.acquired = append(h.acquired, "cycle "+cycleID.String())

				return cycle, nil
			}).
			AnyTimes()
	}
}

func (h *harness) follows(previous, next entity.Cycle) {
	h.cycles.EXPECT().
		NextAfter(gomock.Any(), previous.TeamID, previous.EndsOn).
		Return(next, nil).
		AnyTimes()
}

func (h *harness) allowOnlyOwnTeam() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
			Workspace: entity.Workspace{ID: h.workspaceID, Timezone: "UTC"},
			Scope:     entity.TeamScope{WorkspaceID: h.workspaceID, TeamIDs: []uuid.UUID{h.teamID}},
		}, nil).
		AnyTimes()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), h.workspaceID).
		Return(entity.Workspace{ID: h.workspaceID, Timezone: "UTC"}, nil).
		AnyTimes()
}

func (h *harness) allowAnything() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
			Workspace: entity.Workspace{ID: h.workspaceID, Timezone: "UTC"},
			Scope:     entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), h.workspaceID).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) (entity.Workspace, error) {
			if h.workspaceErr != nil {
				return entity.Workspace{}, h.workspaceErr
			}

			return entity.Workspace{ID: h.workspaceID, Timezone: "UTC"}, nil
		}).
		AnyTimes()

	h.teams.EXPECT().
		GetByID(gomock.Any(), h.teamID).
		Return(entity.Team{
			ID:          h.teamID,
			WorkspaceID: h.workspaceID,
			Key:         "MOB",
			Status:      entity.TeamStatusActive,
		}, nil).
		AnyTimes()
}

func (h *harness) cycle(number int, startsOn, endsOn string) entity.Cycle {
	return entity.Cycle{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		TeamKey:     "MOB",
		Number:      number,
		StartsOn:    startsOn,
		EndsOn:      endsOn,
	}
}

func (h *harness) issue(category entity.StateCategory) entity.Issue {
	return entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		TeamKey:     "MOB",
		State:       entity.IssueState{ID: uuid.New(), Name: "Anything", Category: category},
	}
}

func yesterday() string {
	return entity.Today(time.Now().UTC().AddDate(0, 0, -1), "UTC")
}

func lastMonth() string {
	return entity.Today(time.Now().UTC().AddDate(0, 0, -30), "UTC")
}

func TestGenerationStopsOnceThreeCyclesAreQueuedAhead(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	h.cadences.EXPECT().ListAll(gomock.Any()).Return([]repository.CadenceListing{{
		Cadence:  entity.CycleCadence{TeamID: h.teamID, WorkspaceID: h.workspaceID, LengthWeeks: 2, AnchorOn: entity.Today(time.Now().UTC(), "UTC")},
		Timezone: "UTC",
	}}, nil)

	h.cadences.EXPECT().Lock(gomock.Any(), h.teamID).Return(entity.CycleCadence{
		TeamID:      h.teamID,
		WorkspaceID: h.workspaceID,
		LengthWeeks: 2,
		AnchorOn:    entity.Today(time.Now().UTC(), "UTC"),
	}, nil)

	h.cycles.EXPECT().ListByTeamID(gomock.Any(), h.teamID).Return(nil, nil)
	h.cycles.EXPECT().HighestNumber(gomock.Any(), h.teamID).Return(0, nil)

	created := make([]entity.Cycle, 0)

	h.cycles.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cycle entity.Cycle) (entity.Cycle, error) {
			created = append(created, cycle)

			return cycle, nil
		}).
		AnyTimes()

	if err := h.service.Generate(context.Background()); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(created) != entity.CycleUpcomingCount+1 {
		t.Fatalf(
			"generation made %d cycles, want the one covering today plus %d queued ahead; "+
				"unbounded generation would run into the future forever",
			len(created),
			entity.CycleUpcomingCount,
		)
	}

	for i := 1; i < len(created); i++ {
		previous := created[i-1]
		next := created[i]

		expected, err := entity.CycleCadence{}.StartAfter(previous.EndsOn)
		if err != nil {
			t.Fatalf("start after: %v", err)
		}

		if next.StartsOn != expected {
			t.Errorf(
				"cycle %d starts %s but cycle %d ends %s; consecutive cycles must meet exactly "+
					"or the exclusion constraint refuses the insert",
				next.Number, next.StartsOn, previous.Number, previous.EndsOn,
			)
		}

		if next.Number != previous.Number+1 {
			t.Errorf("cycle numbers jumped from %d to %d", previous.Number, next.Number)
		}
	}
}

func TestGenerationMakesNothingNewWhenTheQueueIsAlreadyFull(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	today := entity.Today(time.Now().UTC(), "UTC")
	cadence := entity.CycleCadence{
		TeamID:      h.teamID,
		WorkspaceID: h.workspaceID,
		LengthWeeks: 1,
		AnchorOn:    today,
	}

	existing := []entity.Cycle{h.cycle(1, today, entity.Today(time.Now().UTC().AddDate(0, 0, 6), "UTC"))}
	next := existing[0].EndsOn

	for i := range entity.CycleUpcomingCount {
		start, err := cadence.StartAfter(next)
		if err != nil {
			t.Fatalf("start after: %v", err)
		}

		_, end, err := cadence.WindowFrom(start)
		if err != nil {
			t.Fatalf("window: %v", err)
		}

		existing = append(existing, h.cycle(i+2, start, end))
		next = end
	}

	h.cadences.EXPECT().ListAll(gomock.Any()).
		Return([]repository.CadenceListing{{Cadence: cadence, Timezone: "UTC"}}, nil)
	h.cadences.EXPECT().Lock(gomock.Any(), h.teamID).Return(cadence, nil)
	h.cycles.EXPECT().ListByTeamID(gomock.Any(), h.teamID).Return(existing, nil)
	h.cycles.EXPECT().HighestNumber(gomock.Any(), h.teamID).Return(len(existing), nil)

	h.cycles.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(0)

	if err := h.service.Generate(context.Background()); err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestClosingIsRefusedUntilTheUnfinishedIssuesHaveSomewhereToGo(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{h.issue(entity.StateCategoryActive)}, nil)

	_, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{})

	if !errors.Is(err, entity.ErrCycleRolloverRequired) {
		t.Fatalf(
			"closing with an open issue and no decision returned %v, want ErrCycleRolloverRequired; "+
				"the decision about unfinished work has to be deliberate",
			err,
		)
	}
}

func TestClosingACycleThatHasNotEndedIsRefused(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	today := entity.Today(time.Now().UTC(), "UTC")
	running := h.cycle(4, today, entity.Today(time.Now().UTC().AddDate(0, 0, 7), "UTC"))

	h.locks(running)

	_, err := h.service.Close(
		context.Background(),
		h.workspaceID,
		running.ID,
		service.CloseCycleInput{Rollover: entity.CycleRolloverBacklog},
	)

	if !errors.Is(err, entity.ErrCycleNotEnded) {
		t.Fatalf("closing a running cycle returned %v, want ErrCycleNotEnded", err)
	}
}

func TestAnOverrideSendsOneIssueToTheBacklogWhileTheRestRollForward(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	next := h.cycle(5, entity.Today(time.Now().UTC(), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))

	rolling := h.issue(entity.StateCategoryActive)
	staying := h.issue(entity.StateCategoryNotStarted)
	finished := h.issue(entity.StateCategoryComplete)

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{rolling, staying, finished}, nil)
	h.locks(next)
	h.follows(ended, next)

	var forward, backlog []uuid.UUID

	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uuid.UUID, _ uuid.UUID, cycleID *uuid.UUID, _ time.Time) (int, error) {
			if cycleID == nil {
				backlog = ids
			} else {
				forward = ids
			}

			return len(ids), nil
		}).
		Times(2)

	recorded := map[entity.CycleScopeChangeKind]int{}

	h.scope.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			recorded[change.Change]++

			return nil
		}).
		Times(3)

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverNext).
		Return(ended, nil)

	if _, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
		Overrides: []service.RolloverOverride{
			{IssueID: staying.ID, Destination: entity.CycleRolloverBacklog},
		},
	}); err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(forward) != 1 || forward[0] != rolling.ID {
		t.Errorf("issues moved to the next cycle are %v, want only %v", forward, rolling.ID)
	}

	if len(backlog) != 1 || backlog[0] != staying.ID {
		t.Errorf("issues returned to the backlog are %v, want only %v", backlog, staying.ID)
	}

	if recorded[entity.CycleScopeChangeRolledOver] != 1 || recorded[entity.CycleScopeChangeReturned] != 1 {
		t.Errorf(
			"the ledger recorded %d rolled over and %d returned, want one of each",
			recorded[entity.CycleScopeChangeRolledOver],
			recorded[entity.CycleScopeChangeReturned],
		)
	}
}

func TestAFinishedCycleClosesWithoutARolloverDecision(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{h.issue(entity.StateCategoryComplete), h.issue(entity.StateCategoryAbandoned)}, nil)
	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(ended, nil)

	if _, err := h.service.Close(
		context.Background(),
		h.workspaceID,
		ended.ID,
		service.CloseCycleInput{},
	); err != nil {
		t.Fatalf(
			"closing a cycle whose work is all finished or abandoned returned %v; there is "+
				"nothing left to decide about",
			err,
		)
	}
}

func TestScopeSeparatesWhatWasPlannedFromWhatArrivedLater(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	running := h.cycle(4, lastMonth(), entity.Today(time.Now().UTC().AddDate(0, 0, 7), "UTC"))
	planned := h.issue(entity.StateCategoryActive)
	late := h.issue(entity.StateCategoryNotStarted)

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, running.ID, gomock.Any()).
		Return(running, nil)
	h.scope.EXPECT().ListByCycleID(gomock.Any(), running.ID, gomock.Any()).Return([]entity.CycleScopeChange{{
		ID:      uuid.New(),
		CycleID: running.ID,
		IssueID: late.ID,
		Change:  entity.CycleScopeChangeAdded,
	}}, nil)
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.Issue{planned, late}, nil)

	scope, err := h.service.Scope(context.Background(), h.workspaceID, running.ID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if len(scope.Original) != 1 || scope.Original[0].ID != planned.ID {
		t.Errorf("original scope is %d issues, want only the one that was there at the start", len(scope.Original))
	}

	if len(scope.Added) != 1 || scope.Added[0].ID != late.ID {
		t.Errorf(
			"added-after-start is %d issues, want only the one with a ledger entry; without "+
				"this split a cycle that grew looks the same as one that was planned well",
			len(scope.Added),
		)
	}
}

func TestATeamWithoutACadenceHasNoCycleToShow(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	h.cadences.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID).Return(nil, nil)
	h.cycles.EXPECT().ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

	current, err := h.service.Current(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if len(current) != 0 {
		t.Fatalf("a workspace with no cadence reported %d current cycles, want none", len(current))
	}
}

func TestNavigationFallsBackToTheNextCycleWhenNoneIsRunning(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	tomorrow := entity.Today(time.Now().UTC().AddDate(0, 0, 1), "UTC")
	upcoming := h.cycle(9, tomorrow, entity.Today(time.Now().UTC().AddDate(0, 0, 14), "UTC"))

	h.cadences.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID).
		Return([]entity.CycleCadence{{TeamID: h.teamID, WorkspaceID: h.workspaceID, LengthWeeks: 2, AnchorOn: tomorrow}}, nil)
	h.cycles.EXPECT().ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.Cycle{upcoming}, nil)

	current, err := h.service.Current(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if len(current) != 1 || current[0].View.Cycle.ID != upcoming.ID {
		t.Fatalf("navigation showed %d cycles, want the upcoming one so the team can still reach it", len(current))
	}

	if current[0].View.Phase != entity.CyclePhaseUpcoming {
		t.Errorf("the cycle is labelled %q, want upcoming", current[0].View.Phase)
	}
}

func importOrigin() entity.ImportOrigin {
	created := time.Date(2023, time.March, 6, 8, 0, 0, 0, time.UTC)

	return entity.NewImportOrigin(created, created.AddDate(0, 0, 20), uuid.New())
}

func TestCreatingACycleWithoutAnAttributedOriginIsRefused(t *testing.T) {
	decoded := entity.ImportOrigin{
		CreatedAt:       time.Date(2023, time.March, 6, 8, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2023, time.March, 26, 8, 0, 0, 0, time.UTC),
		AuthorAccountID: uuid.New(),
	}

	cases := map[string]*entity.ImportOrigin{
		"no origin":                          nil,
		"an origin filled in from a request": &decoded,
	}

	for name, origin := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.allowAnything()

			h.cycles.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cycle entity.Cycle) (entity.Cycle, error) {
					return cycle, nil
				}).
				AnyTimes()

			_, err := h.service.Create(context.Background(), service.CreateCycleInput{
				WorkspaceID: h.workspaceID,
				TeamID:      h.teamID,
				Number:      1,
				StartsOn:    "2023-03-06",
				EndsOn:      "2023-03-19",
				Origin:      origin,
			})

			if !errors.Is(err, entity.ErrCycleCreateRequiresOrigin) {
				t.Fatalf(
					"creating a cycle with %s returned %v, want ErrCycleCreateRequiresOrigin; cycles "+
						"are generated from a cadence, and this method exists only so an import can "+
						"carry finished ones across",
					name, err,
				)
			}
		})
	}
}

func TestAnImportedCycleReachesTheStoreClosedAndDatedFromItsSource(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	origin := importOrigin()
	closedAt := time.Date(2023, time.March, 20, 17, 0, 0, 0, time.UTC)

	var captured entity.Cycle

	h.cycles.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cycle entity.Cycle) (entity.Cycle, error) {
			captured = cycle
			cycle.ID = uuid.New()

			return cycle, nil
		})

	view, err := h.service.Create(context.Background(), service.CreateCycleInput{
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		Number:      7,
		StartsOn:    "2023-03-06",
		EndsOn:      "2023-03-19",
		ClosedAt:    &closedAt,
		Origin:      &origin,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Number != 7 || captured.StartsOn != "2023-03-06" || captured.EndsOn != "2023-03-19" {
		t.Errorf(
			"cycle %d ran %s to %s, want cycle 7 running 2023-03-06 to 2023-03-19",
			captured.Number, captured.StartsOn, captured.EndsOn,
		)
	}

	if captured.ClosedAt == nil || !captured.ClosedAt.Equal(closedAt) {
		t.Fatalf(
			"the cycle reached the store closed at %v, want %v; an imported cycle left open becomes "+
				"the team's current cycle on every dashboard",
			captured.ClosedAt, closedAt,
		)
	}

	gotCreated, gotUpdated := entity.OriginStamp(captured.Origin, time.Now().UTC())

	if !gotCreated.Equal(origin.CreatedAt) || !gotUpdated.Equal(origin.UpdatedAt) {
		t.Errorf(
			"stamp = (%v, %v), want (%v, %v); without the origin the row is dated the moment the "+
				"import ran",
			gotCreated, gotUpdated, origin.CreatedAt, origin.UpdatedAt,
		)
	}

	if view.Phase != entity.CyclePhaseClosed {
		t.Errorf("the imported cycle is shown as %q, want closed", view.Phase)
	}
}

func TestAnImportedCycleWithoutANumberContinuesTheTeamsNumbering(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	origin := importOrigin()

	h.cycles.EXPECT().HighestNumber(gomock.Any(), h.teamID).Return(12, nil)

	var captured entity.Cycle

	h.cycles.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cycle entity.Cycle) (entity.Cycle, error) {
			captured = cycle

			return cycle, nil
		})

	if _, err := h.service.Create(context.Background(), service.CreateCycleInput{
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		StartsOn:    "2023-03-06",
		EndsOn:      "2023-03-19",
		Origin:      &origin,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if captured.Number != 13 {
		t.Fatalf(
			"the imported cycle was numbered %d, want 13; a team that already generates cycles has "+
				"the source's 1..N taken, and the unique (team_id, number) index refuses the second one",
			captured.Number,
		)
	}
}

func TestACycleEndingBeforeItStartsIsRefused(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	origin := importOrigin()

	h.cycles.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cycle entity.Cycle) (entity.Cycle, error) {
			return cycle, nil
		}).
		AnyTimes()

	_, err := h.service.Create(context.Background(), service.CreateCycleInput{
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		Number:      1,
		StartsOn:    "2023-03-19",
		EndsOn:      "2023-03-06",
		Origin:      &origin,
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf(
			"an inverted window returned %v, want a ValidationError naming endsOn; the table's "+
				"window check would otherwise refuse it as an opaque write failure",
			err,
		)
	}
}

func TestOnlyAClosingDateKeepsAnImportedCycleOutOfTheCurrentSlot(t *testing.T) {
	historicalStart, historicalEnd := "2023-03-06", "2023-03-19"

	current := func(t *testing.T, historical entity.Cycle) []service.TeamCycle {
		t.Helper()

		h := newHarness(t)
		h.allowAnything()

		historical.ID = uuid.New()
		historical.WorkspaceID = h.workspaceID
		historical.TeamID = h.teamID
		running := h.cycle(40, yesterday(), entity.Today(time.Now().UTC().AddDate(0, 0, 12), "UTC"))

		h.cadences.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID).
			Return([]entity.CycleCadence{{
				TeamID:      h.teamID,
				WorkspaceID: h.workspaceID,
				LengthWeeks: 2,
				AnchorOn:    running.StartsOn,
			}}, nil)
		h.cycles.EXPECT().ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entity.Cycle{historical, running}, nil)

		views, err := h.service.Current(context.Background(), h.workspaceID)
		if err != nil {
			t.Fatalf("Current: %v", err)
		}

		if len(views) != 1 {
			t.Fatalf("the team showed %d cycles, want exactly one", len(views))
		}

		return views
	}

	t.Run("closed, so the running cycle is the one on show", func(t *testing.T) {
		closedAt := time.Date(2023, time.March, 20, 17, 0, 0, 0, time.UTC)
		views := current(t, entity.Cycle{
			Number:   1,
			StartsOn: historicalStart,
			EndsOn:   historicalEnd,
			ClosedAt: &closedAt,
		})

		if views[0].View.Cycle.StartsOn == historicalStart {
			t.Fatal(
				"the 2023 cycle is on show even though it is closed; Current must reach the cycle " +
					"the team is actually working in",
			)
		}
	})

	t.Run("left open, so it takes the slot from the running cycle", func(t *testing.T) {
		views := current(t, entity.Cycle{
			Number:   1,
			StartsOn: historicalStart,
			EndsOn:   historicalEnd,
		})

		if views[0].View.Cycle.StartsOn != historicalStart {
			t.Fatalf(
				"an imported cycle from %s with no closed_at was not the one on show; this test "+
					"pins why the import must write closed_at — such a cycle is phase %q, which is "+
					"what Current picks first, so it would sit on the team's dashboard forever",
				historicalStart, entity.CyclePhaseEnded,
			)
		}
	})
}

func TestTheCyclesHistoryIsAskedForOnlyWhatTheReaderMaySee(t *testing.T) {
	h := newHarness(t)

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
			Workspace: entity.Workspace{ID: h.workspaceID, Timezone: "UTC"},
			Scope:     entity.TeamScope{WorkspaceID: h.workspaceID, TeamIDs: []uuid.UUID{h.teamID}},
		}, nil).
		AnyTimes()

	running := h.cycle(4, lastMonth(), entity.Today(time.Now().UTC().AddDate(0, 0, 7), "UTC"))

	var asked entity.TeamScope

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, running.ID, gomock.Any()).
		Return(running, nil)

	h.scope.EXPECT().
		ListByCycleID(gomock.Any(), running.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			narrowed entity.TeamScope,
		) ([]entity.CycleScopeChange, error) {
			asked = narrowed

			return nil, nil
		})

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	if _, err := h.service.Scope(context.Background(), h.workspaceID, running.ID); err != nil {
		t.Fatalf("scope: %v", err)
	}

	if asked.AllTeams {
		t.Fatal(
			"the history was asked for across every team. Being entitled to a cycle is not " +
				"being entitled to the title of every issue that ever passed through it.",
		)
	}

	if len(asked.TeamIDs) != 1 || asked.TeamIDs[0] != h.teamID {
		t.Fatalf("the history was scoped to %v, want the reader's own team %v", asked.TeamIDs, h.teamID)
	}
}

func TestAnIssueKeptWhereItIsStaysInTheCycleItWasClosedIn(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	next := h.cycle(5, entity.Today(time.Now().UTC(), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))

	rolling := h.issue(entity.StateCategoryActive)
	kept := h.issue(entity.StateCategoryActive)

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{rolling, kept}, nil)
	h.locks(next)
	h.follows(ended, next)

	var moved []uuid.UUID

	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uuid.UUID, _ uuid.UUID, _ *uuid.UUID, _ time.Time) (int, error) {
			moved = append(moved, ids...)

			return len(ids), nil
		})

	arriving := map[uuid.UUID][]uuid.UUID{}

	h.scope.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			if change.Change == entity.CycleScopeChangeAdded {
				arriving[change.CycleID] = append(arriving[change.CycleID], change.IssueID)
			}

			return nil
		}).
		Times(2)

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverNext).
		Return(ended, nil)

	if _, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
		Overrides: []service.RolloverOverride{
			{IssueID: kept.ID, Destination: entity.CycleRolloverKeep},
		},
	}); err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(arriving[next.ID]) != 1 || arriving[next.ID][0] != rolling.ID {
		t.Fatalf(
			"the next cycle recorded %v arriving, want only %v. A cycle already under way has to "+
				"say when work landed in it, or the issue reads as having been there from the "+
				"first day and its carried-in count is a guess.",
			arriving[next.ID], rolling.ID,
		)
	}

	if len(moved) != 1 || moved[0] != rolling.ID {
		t.Fatalf(
			"closing moved %v. Keeping an issue here means it is not moved at all: it stays "+
				"attached to the cycle it was closed in, which is what the report of that cycle "+
				"then has to account for.",
			moved,
		)
	}
}

func TestKeepingEveryUnfinishedIssueClosesTheCycleWithoutMovingAnything(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	open := h.issue(entity.StateCategoryActive)

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{open}, nil)

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverKeep).
		Return(ended, nil)

	if _, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverKeep,
	}); err != nil {
		t.Fatalf(
			"close: %v. Keep is a decision like the other two, so a cycle every issue is kept in "+
				"still closes — it neither moves an issue nor asks for a next cycle.",
			err,
		)
	}
}

func TestWorkRolledIntoACycleThatHasNotStartedNeedsNoArrivalEvent(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	next := h.cycle(5, entity.Today(time.Now().UTC().AddDate(0, 0, 7), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 20), "UTC"))

	rolling := h.issue(entity.StateCategoryActive)

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{rolling}, nil)
	h.locks(next)
	h.follows(ended, next)
	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uuid.UUID, _ uuid.UUID, _ *uuid.UUID, _ time.Time) (int, error) {
			return len(ids), nil
		})

	kinds := map[entity.CycleScopeChangeKind]int{}

	h.scope.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.CycleScopeChange) error {
			kinds[change.Change]++

			return nil
		}).
		Times(1)

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverNext).
		Return(ended, nil)

	if _, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
	}); err != nil {
		t.Fatalf("close: %v", err)
	}

	if kinds[entity.CycleScopeChangeAdded] != 0 {
		t.Fatalf(
			"an arrival was recorded on a cycle that has not begun. Work that lands before the " +
				"first day was there from the first day, and the ledger only carries what " +
				"departs from that.",
		)
	}
}

func TestClosingWritesWhereEveryIssueStoodAndWhatWasDecidedAboutIt(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	next := h.cycle(5, entity.Today(time.Now().UTC(), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))

	rolling := h.issue(entity.StateCategoryActive)
	kept := h.issue(entity.StateCategoryNotStarted)
	finished := h.issue(entity.StateCategoryComplete)

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{rolling, kept, finished}, nil)
	h.locks(next)
	h.follows(ended, next)
	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uuid.UUID, _ uuid.UUID, _ *uuid.UUID, _ time.Time) (int, error) {
			return len(ids), nil
		})
	h.scope.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverNext).
		Return(ended, nil)

	if _, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
		Overrides: []service.RolloverOverride{
			{IssueID: kept.ID, Destination: entity.CycleRolloverKeep},
		},
	}); err != nil {
		t.Fatalf("close: %v", err)
	}

	frozen := h.frozen

	if len(frozen) != 3 {
		t.Fatalf("the cycle froze %d rows, want one for every issue it held", len(frozen))
	}

	held := map[uuid.UUID]entity.CycleResult{}
	for _, result := range frozen {
		held[result.IssueID] = result
	}

	if held[finished.ID].Category != entity.StateCategoryComplete || !held[finished.ID].Finished() {
		t.Errorf(
			"the finished issue froze as %q. What a cycle achieved is decided when it closes, "+
				"not by where its issues drift to afterwards.",
			held[finished.ID].Category,
		)
	}

	if held[finished.ID].Decision != entity.CycleRolloverNone {
		t.Errorf(
			"the finished issue froze carrying the decision %q, want none: nothing was decided "+
				"about work that was already done",
			held[finished.ID].Decision,
		)
	}

	if held[kept.ID].Decision != entity.CycleRolloverKeep {
		t.Errorf("the kept issue froze carrying %q, want keep", held[kept.ID].Decision)
	}

	if held[rolling.ID].Decision != entity.CycleRolloverNext {
		t.Errorf("the rolled issue froze carrying %q, want next", held[rolling.ID].Decision)
	}
}

func TestACycleTakesAnOwnerFromItsOwnTeamAndCanBeCleared(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	running := h.cycle(6, entity.Today(time.Now().UTC(), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))
	owner := uuid.New()

	h.cycles.EXPECT().LockByID(gomock.Any(), running.ID).Return(running, nil).Times(2)
	h.members.EXPECT().
		Get(gomock.Any(), h.teamID, owner).
		Return(entity.TeamMembership{TeamID: h.teamID, AccountID: owner}, nil)

	var given []*uuid.UUID

	h.cycles.EXPECT().
		SetOwner(gomock.Any(), running.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, account *uuid.UUID) (entity.Cycle, error) {
			given = append(given, account)

			return running, nil
		}).
		Times(2)

	if _, err := h.service.SetOwner(context.Background(), h.workspaceID, running.ID, &owner); err != nil {
		t.Fatalf("setting an owner from the team: %v", err)
	}

	if _, err := h.service.SetOwner(context.Background(), h.workspaceID, running.ID, nil); err != nil {
		t.Fatalf("clearing the owner: %v", err)
	}

	if len(given) != 2 || given[0] == nil || *given[0] != owner || given[1] != nil {
		t.Fatalf(
			"the store was given %v, want the chosen account and then nothing. Unassigned is a "+
				"real value here, not the absence of a request.",
			given,
		)
	}
}

func TestSomebodyOutsideTheTeamCannotOwnItsCycle(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	running := h.cycle(6, entity.Today(time.Now().UTC(), "UTC"), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))
	stranger := uuid.New()

	h.locks(running)
	h.members.EXPECT().
		Get(gomock.Any(), h.teamID, stranger).
		Return(entity.TeamMembership{}, entity.ErrTeamMembershipNotFound)

	_, err := h.service.SetOwner(context.Background(), h.workspaceID, running.ID, &stranger)

	if !errors.Is(err, entity.ErrCycleOwnerNotOnTeam) {
		t.Fatalf(
			"SetOwner error = %v, want ErrCycleOwnerNotOnTeam. The owner is picked from the "+
				"team that runs the cycle, and choosing one grants nothing, so the list it is "+
				"picked from is the whole of the rule.",
			err,
		)
	}
}

func TestAClosedCycleWillNotTakeANewOwner(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	closedAt := time.Now().UTC()
	ended := h.cycle(4, lastMonth(), yesterday())
	ended.ClosedAt = &closedAt

	owner := uuid.New()

	h.locks(ended)

	_, err := h.service.SetOwner(context.Background(), h.workspaceID, ended.ID, &owner)

	if !errors.Is(err, entity.ErrCycleClosed) {
		t.Fatalf(
			"SetOwner error = %v, want ErrCycleClosed. A finished cycle reads as a record of "+
				"what happened, and the row is locked while that is decided so a close cannot "+
				"land between the check and the write.",
			err,
		)
	}
}

func TestClosingACycleThatHeldNothingStillRecordsThatItWasCounted(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())

	h.locks(ended)
	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{}, nil)

	var stamped bool

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), entity.CycleRolloverBacklog).
		DoAndReturn(func(
			_ context.Context, cycleID uuid.UUID, closedAt time.Time,
			_ *uuid.UUID, rollover entity.CycleRollover,
		) (entity.Cycle, error) {
			stamped = true
			closed := ended
			closed.ClosedAt = &closedAt
			closed.ResultsRecordedAt = &closedAt
			closed.Rollover = rollover

			return closed, nil
		})

	closed, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{})
	if err != nil {
		t.Fatalf("closing a cycle that held nothing: %v", err)
	}

	if !stamped || !closed.Cycle.Frozen() {
		t.Fatal(
			"closing an empty cycle left no record that its results were taken. Zero issues is " +
				"an answer; having never been asked is not, and the screen has to tell them apart.",
		)
	}

	if len(h.frozen) != 0 {
		t.Fatalf("the empty cycle froze %d rows, want none", len(h.frozen))
	}
}

func TestACycleClosedBeforeAnyOfThisHasNoResultsToReport(t *testing.T) {
	h := newHarness(t)

	closedAt := time.Now().UTC().AddDate(0, 0, -30)
	legacy := h.cycle(3, lastMonth(), yesterday())
	legacy.ClosedAt = &closedAt

	if !legacy.Closed() {
		t.Fatal("the fixture is not closed, so it cannot stand for a cycle closed long ago")
	}

	if legacy.Frozen() {
		t.Fatal(
			"a cycle closed before results were ever recorded reads as frozen. It has no " +
				"snapshot at all, which is a different answer from a cycle that closed holding " +
				"nothing, and reporting zero for it would be an invention.",
		)
	}
}

func TestClosingIsRefusedWhenTheNextCycleClosedWhileTheFormWasOpen(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())
	next := h.cycle(5, yesterday(), entity.Today(time.Now().UTC().AddDate(0, 0, 13), "UTC"))
	shut := time.Now().UTC()
	next.ClosedAt = &shut

	h.locks(ended, next)
	h.follows(ended, next)

	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{h.issue(entity.StateCategoryActive)}, nil)
	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)
	h.cycles.EXPECT().
		Close(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	_, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
	})
	if !errors.Is(err, entity.ErrCycleStale) {
		t.Fatalf(
			"closing into a cycle that had itself closed returned %v, want a conflict. Unfinished "+
				"work would otherwise be moved into a cycle whose numbers are already final.",
			err,
		)
	}
}

func TestClosingIsRefusedWhenTheUnfinishedWorkIsNotWhatTheFormShowed(t *testing.T) {
	for name, reviewed := range map[string][]uuid.UUID{
		"the form showed nothing":       {},
		"the form showed another issue": {uuid.New()},
	} {
		t.Run(name, func(t *testing.T) {
			closingIsRefused(t, reviewed)
		})
	}
}

func closingIsRefused(t *testing.T, reviewed []uuid.UUID) {
	t.Helper()

	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())

	h.locks(ended)

	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), gomock.Any()).
		Return([]entity.Issue{h.issue(entity.StateCategoryActive)}, nil)
	h.issues.EXPECT().
		MoveIssuesToCycle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)
	h.cycles.EXPECT().
		Close(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	_, err := h.service.Close(context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverBacklog,
		Reviewed: reviewed,
	})
	if !errors.Is(err, entity.ErrCycleStale) {
		t.Fatalf(
			"closing a cycle whose unfinished work changed returned %v, want a conflict. An issue "+
				"nobody saw in the form would otherwise be sent wherever the default pointed.",
			err,
		)
	}
}

func TestClosingHoldsTheTeamsWorkflowStatesBeforeItReadsWhatTheCycleHeld(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	ended := h.cycle(4, lastMonth(), yesterday())

	h.locks(ended)

	h.issues.EXPECT().
		LockByCycleID(gomock.Any(), ended.ID).
		DoAndReturn(func(_ context.Context, cycleID uuid.UUID) ([]entity.Issue, error) {
			h.acquired = append(h.acquired, "issues "+cycleID.String())

			return []entity.Issue{h.issue(entity.StateCategoryComplete)}, nil
		})

	h.cycles.EXPECT().
		Close(gomock.Any(), ended.ID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(ended, nil)

	if _, err := h.service.Close(
		context.Background(), h.workspaceID, ended.ID, service.CloseCycleInput{},
	); err != nil {
		t.Fatalf("Close: %v", err)
	}

	want := []string{
		"states " + h.teamID.String(),
		"cycle " + ended.ID.String(),
		"issues " + ended.ID.String(),
	}

	if len(h.acquired) != len(want) {
		t.Fatalf("acquired %v, want %v", h.acquired, want)
	}

	for at, step := range want {
		if h.acquired[at] != step {
			t.Fatalf(
				"acquired %v, want %v. The query that reads what a cycle held joins the workflow "+
					"states and locks only the issue rows, so a reclassification committing while it "+
					"waits leaves the joined category at the statement's older snapshot.",
				h.acquired, want,
			)
		}
	}
}
