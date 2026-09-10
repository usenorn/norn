package cycle_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal"
	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuedelegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	issuefollowerrepo "github.com/usenorn/norn/internal/repository/issuefollower"
	issuequestionrepo "github.com/usenorn/norn/internal/repository/issuequestion"
	issuerevisionrepo "github.com/usenorn/norn/internal/repository/issuerevision"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	requestkeyrepo "github.com/usenorn/norn/internal/repository/requestkey"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

type live struct {
	client    *postgres.Client
	workspace entity.Workspace
	team      entity.Team
	states    []entity.WorkflowState
	account   uuid.UUID
	numbering int
}

func scratchDatabase(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := os.Getenv("NORN_TEST_POSTGRES_DSN")
	if os.Getenv("NORN_TEST_INTEGRATION") != "true" || dsn == "" {
		t.Skip("set NORN_TEST_INTEGRATION=true and NORN_TEST_POSTGRES_DSN to run this")
	}

	name := fmt.Sprintf("norn_close_%d", time.Now().UnixNano())

	scratch, err := scratchDSN(dsn, name)
	if err != nil {
		t.Fatalf("%v", err)
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open the server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		_ = admin.Close()

		t.Fatalf("create the scratch database: %v", err)
	}

	t.Cleanup(func() {
		done, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()

		if _, err := admin.ExecContext(done, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("the scratch database %s was left behind: %v", name, err)
		}

		if err := admin.Close(); err != nil {
			t.Errorf("close the server connection: %v", err)
		}
	})

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             scratch,
		MaxConns:        8,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to the scratch database: %v", err)
	}

	t.Cleanup(cleanup)

	var reached string

	if err := client.DB.QueryRowContext(ctx, "SELECT current_database()").Scan(&reached); err != nil {
		t.Fatalf("read which database the connection reached: %v", err)
	}

	if reached != name {
		t.Fatalf(
			"the connection reached %q, want the scratch database %q. Migrating and seeding anything "+
				"other than the database this test created would write fixtures into somebody's data.",
			reached, name,
		)
	}

	migrator, err := internal.NewMigrator(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build the migrator: %v", err)
	}

	if err := migrator.Run(ctx); err != nil {
		t.Fatalf("migrate the scratch database: %v", err)
	}

	return client
}

func scratchDSN(dsn, name string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return "", errors.New(
			"NORN_TEST_POSTGRES_DSN must be a postgres:// URL so the database name can be replaced " +
				"safely; the value given cannot be redirected to a scratch database, and running " +
				"against the original would write fixtures into it",
		)
	}

	parsed.Path = "/" + name

	return parsed.String(), nil
}

func liveCycles(t *testing.T, client *postgres.Client) (service.Cycles, *live) {
	return liveCyclesEmitting(t, client, nil)
}

func liveCyclesEmitting(
	t *testing.T,
	client *postgres.Client,
	refusal error,
) (service.Cycles, *live) {
	t.Helper()

	ctrl := gomock.NewController(t)
	authorizer := authorizersvc.NewMockAuthorizer(ctrl)

	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(refusal).AnyTimes()

	world := &live{client: client, account: uuid.New()}

	accounts := accountrepo.New(client)
	workspaces := workspacerepo.New(client)
	teams := teamrepo.New(client)
	states := workflowstaterepo.New(client)

	ctx := context.Background()

	account, err := accounts.Create(ctx, entity.Account{
		ID: world.account, Status: entity.AccountStatusActive, Kind: entity.AccountKindPerson,
		Email: "closer-" + uuid.NewString()[:8] + "@northwind.co", DisplayName: "Closer",
		Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("create the account: %v", err)
	}

	world.account = account.ID

	workspace, err := workspaces.Create(ctx, entity.Workspace{
		ID: uuid.New(), Slug: "close-" + uuid.NewString()[:8], Name: "Closing",
		Status: entity.WorkspaceStatusActive, Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("create the workspace: %v", err)
	}

	team, err := teams.Create(ctx, entity.Team{
		ID: uuid.New(), WorkspaceID: workspace.ID, Key: "CLS", Name: "Closing",
		IconColor: entity.DefaultTeamColor, Estimation: entity.TeamEstimationPoints,
		Status: entity.TeamStatusActive, Visibility: entity.DefaultTeamVisibility,
	})
	if err != nil {
		t.Fatalf("create the team: %v", err)
	}

	for at, wanted := range []struct {
		name     string
		category entity.StateCategory
	}{
		{"Todo", entity.StateCategoryNotStarted},
		{"Doing", entity.StateCategoryActive},
		{"Done", entity.StateCategoryComplete},
	} {
		state, err := states.Create(ctx, entity.WorkflowState{
			ID: uuid.New(), WorkspaceID: workspace.ID, TeamID: team.ID,
			Name: wanted.name, Category: wanted.category,
			Position: at + 1, IsDefault: at == 0, IsCompletion: wanted.category == entity.StateCategoryComplete,
		})
		if err != nil {
			t.Fatalf("create the %s state: %v", wanted.name, err)
		}

		world.states = append(world.states, state)
	}

	world.workspace = workspace
	world.team = team

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: world.account},
				Workspace: workspace,
				Scope:     entity.TeamScope{WorkspaceID: workspace.ID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	cycles := cyclesvc.New(
		cyclerepo.New(client),
		cyclerepo.NewCadence(client),
		cyclerepo.NewScopeChange(client),
		cyclerepo.NewResult(client),
		activityrepo.New(client),
		states,
		teammemberrepo.New(client),
		issuerepo.New(client),
		teams,
		workspaces,
		authorizer,
		emitter,
		client,
	)

	return cycles, world
}

func (l *live) cycleOn(t *testing.T, number int, startsOn, endsOn string) entity.Cycle {
	t.Helper()

	cycle, err := cyclerepo.New(l.client).Create(context.Background(), entity.Cycle{
		ID: uuid.New(), WorkspaceID: l.workspace.ID, TeamID: l.team.ID, TeamKey: l.team.Key,
		Number: number, StartsOn: startsOn, EndsOn: endsOn,
	})
	if err != nil {
		t.Fatalf("create cycle %d: %v", number, err)
	}

	return cycle
}

func (l *live) issueIn(t *testing.T, cycle entity.Cycle, state entity.WorkflowState) entity.Issue {
	t.Helper()

	l.numbering++

	cycleID := cycle.ID
	issue, err := issuerepo.New(l.client).Create(context.Background(), entity.Issue{
		ID: uuid.New(), WorkspaceID: l.workspace.ID, TeamID: l.team.ID, TeamKey: l.team.Key,
		ReferenceKey: l.team.Key, Number: l.numbering, Title: "Work " + state.Name,
		Priority: entity.IssuePriorityNone, Status: entity.IssueStatusActive,
		State: entity.IssueState{ID: state.ID}, CycleID: cycleID, Rank: fmt.Sprintf("a%d", l.numbering),
		CreatedByAccountID: l.account, StateEnteredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create an issue: %v", err)
	}

	return issue
}

func TestClosingAnEndedCycleRecordsWhatEachIssueLookedLikeAndMovesTheRest(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)

	ended := world.cycleOn(t, 1, "2026-08-24", "2026-08-30")
	next := world.cycleOn(t, 2, "2026-08-31", "2026-09-13")

	finished := world.issueIn(t, ended, world.states[2])
	rolling := world.issueIn(t, ended, world.states[1])
	staying := world.issueIn(t, ended, world.states[0])

	ctx := context.Background()

	view, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover:  entity.CycleRolloverNext,
		Overrides: []service.RolloverOverride{{IssueID: staying.ID, Destination: entity.CycleRolloverKeep}},
		Reviewed:  []uuid.UUID{rolling.ID, staying.ID},
	})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	if view.Phase != entity.CyclePhaseClosed {
		t.Fatalf("phase after closing = %q, want closed", view.Phase)
	}

	results, err := cyclerepo.NewResult(client).ListByCycleID(ctx, ended.ID)
	if err != nil {
		t.Fatalf("read the recorded results: %v", err)
	}

	recorded := map[uuid.UUID]entity.CycleResult{}
	for _, result := range results {
		recorded[result.IssueID] = result
	}

	if len(recorded) != 3 {
		t.Fatalf("recorded %d results, want one for every issue the cycle held", len(recorded))
	}

	if recorded[finished.ID].Category != entity.StateCategoryComplete {
		t.Fatalf("the finished issue was recorded as %q, want complete", recorded[finished.ID].Category)
	}

	if recorded[rolling.ID].Decision != entity.CycleRolloverNext {
		t.Fatalf("the rolled issue was recorded as %q, want next", recorded[rolling.ID].Decision)
	}

	if recorded[staying.ID].Decision != entity.CycleRolloverKeep {
		t.Fatalf("the kept issue was recorded as %q, want keep", recorded[staying.ID].Decision)
	}

	held, err := issuerepo.New(client).CyclesOf(
		ctx, world.workspace.ID,
		[]uuid.UUID{finished.ID, rolling.ID, staying.ID},
		entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true},
	)
	if err != nil {
		t.Fatalf("read where the issues ended up: %v", err)
	}

	if held[rolling.ID] != next.ID {
		t.Fatalf("the rolled issue sits in %v, want the next cycle %v", held[rolling.ID], next.ID)
	}

	if held[staying.ID] != ended.ID {
		t.Fatalf("the kept issue sits in %v, want the cycle it was closed in %v", held[staying.ID], ended.ID)
	}

	if _, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverBacklog,
	}); !errors.Is(err, entity.ErrCycleClosed) {
		t.Fatalf("closing twice returned %v, want ErrCycleClosed", err)
	}
}

func TestAClosePreparingToRollIntoNothingIsRefusedBeforeItWritesAnything(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)

	ended := world.cycleOn(t, 1, "2026-08-24", "2026-08-30")
	rolling := world.issueIn(t, ended, world.states[1])

	ctx := context.Background()

	if _, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
		Reviewed: []uuid.UUID{rolling.ID},
	}); !errors.Is(err, entity.ErrCycleNoNextCycle) {
		t.Fatalf("closing into a cycle that does not exist returned %v, want ErrCycleNoNextCycle", err)
	}

	results, err := cyclerepo.NewResult(client).ListByCycleID(ctx, ended.ID)
	if err != nil {
		t.Fatalf("read the recorded results: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf(
			"%d results survived a refused close. The whole close is one transaction, so a cycle "+
				"that was never closed must not carry a snapshot of what it held.",
			len(results),
		)
	}

	changes, err := cyclerepo.NewScopeChange(client).ListByCycleID(
		ctx, ended.ID, entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true},
	)
	if err != nil {
		t.Fatalf("read the scope ledger: %v", err)
	}

	if len(changes) != 0 {
		t.Fatalf("%d scope changes survived a refused close", len(changes))
	}

	held, err := cyclerepo.New(client).GetVisible(
		ctx, world.workspace.ID, ended.ID,
		entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true},
	)
	if err != nil {
		t.Fatalf("read the cycle back: %v", err)
	}

	if held.Closed() {
		t.Fatal("the cycle closed even though the work could not be moved")
	}
}

func TestWorkKeptInAClosedCycleCanMoveOnWithoutRewritingWhatWasRecorded(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)

	ended := world.cycleOn(t, 1, "2026-08-24", "2026-08-30")
	later := world.cycleOn(t, 2, "2026-08-31", "2026-09-13")
	staying := world.issueIn(t, ended, world.states[1])

	ctx := context.Background()

	if _, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverKeep,
		Reviewed: []uuid.UUID{staying.ID},
	}); err != nil {
		t.Fatalf("Close: %v", err)
	}

	issues := issuerepo.New(client)

	moved, err := issues.MoveIssuesToCycle(ctx, []uuid.UUID{staying.ID}, ended.ID, &later.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("move the kept issue on: %v", err)
	}

	if moved != 1 {
		t.Fatalf("moved %d issues out of the closed cycle, want the one that was kept there", moved)
	}

	results, err := cyclerepo.NewResult(client).ListByCycleID(ctx, ended.ID)
	if err != nil {
		t.Fatalf("read the recorded results: %v", err)
	}

	if len(results) != 1 || results[0].IssueID != staying.ID {
		t.Fatalf("results after the move = %+v, want the one recorded at close", results)
	}

	if results[0].Category != entity.StateCategoryActive || results[0].Decision != entity.CycleRolloverKeep {
		t.Fatalf(
			"the recorded result reads %q/%q after the issue moved on. What a closed cycle held is "+
				"history, and later work on the issue must not rewrite it.",
			results[0].Category, results[0].Decision,
		)
	}
}

func liveIssues(t *testing.T, client *postgres.Client, world *live) service.Issues {
	t.Helper()

	ctrl := gomock.NewController(t)
	authorizer := authorizersvc.NewMockAuthorizer(ctrl)

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: world.account},
				Workspace: world.workspace,
				Scope:     entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	events := eventsvc.NewMockEvents(ctrl)
	events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	jobs := jobqueuerepo.NewMockJobProducer(ctrl)
	jobs.EXPECT().EnqueueBulkApply(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return issuesvc.New(
		issuerepo.New(client),
		issuerevisionrepo.New(client),
		requestkeyrepo.New(client),
		workflowstaterepo.New(client),
		activityrepo.New(client),
		labelrepo.New(client),
		accountrepo.New(client),
		membershiprepo.New(client),
		cyclerepo.New(client),
		cyclerepo.NewScopeChange(client),
		projectrepo.New(client),
		teamrepo.New(client),
		triagerepo.New(client),
		notificationeventrepo.New(client),
		events,
		silentEmitter(ctrl),
		issuefollowerrepo.New(client),
		jobs,
		agenthold.New(
			agentsettingrepo.New(client),
			agentproposalrepo.New(client),
			agentrepo.New(client),
			workflowstaterepo.New(client),
			issuedelegationrepo.New(client),
			issuequestionrepo.New(client),
			notificationeventrepo.New(client),
		),
		authorizer,
		client,
	)
}

func TestAKeptIssueWorkedOnLaterLeavesTheClosedCycleReportAlone(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)
	issues := liveIssues(t, client, world)

	ended := world.cycleOn(t, 1, "2026-08-24", "2026-08-30")
	later := world.cycleOn(t, 2, "2026-08-31", "2026-09-13")
	staying := world.issueIn(t, ended, world.states[1])

	ctx := context.Background()

	if _, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverKeep,
		Reviewed: []uuid.UUID{staying.ID},
	}); err != nil {
		t.Fatalf("Close: %v", err)
	}

	finished := world.states[2].ID

	read, err := issues.Get(ctx, world.workspace.ID, staying.ID)
	if err != nil {
		t.Fatalf("read the kept issue back: %v", err)
	}

	moved, err := issues.Update(ctx, world.workspace.ID, staying.ID, service.UpdateIssueInput{
		StateID: &finished, ExpectedVersion: read.Version,
	})
	if err != nil {
		t.Fatalf("finish the kept issue through the issue service: %v", err)
	}

	if _, err := issues.Update(ctx, world.workspace.ID, staying.ID, service.UpdateIssueInput{
		CycleID: &later.ID, ExpectedVersion: moved.Version,
	}); err != nil {
		t.Fatalf("move the kept issue on through the issue service: %v", err)
	}

	report, err := cycles.Report(ctx, world.workspace.ID, ended.ID)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	if !report.Frozen {
		t.Fatal("the closed cycle reports itself as unrecorded even though it was closed with a snapshot")
	}

	if len(report.Results) != 1 || len(report.Issues) != 1 {
		t.Fatalf(
			"the report shows %d results and %d issues, want the one issue that was in the cycle "+
				"when it closed. Work done afterwards must not add to or remove from it.",
			len(report.Results), len(report.Issues),
		)
	}

	if report.Results[0].IssueID != staying.ID {
		t.Fatalf("the report names %v, want %v", report.Results[0].IssueID, staying.ID)
	}

	if report.Results[0].Category != entity.StateCategoryActive {
		t.Fatalf(
			"the recorded category reads %q after the issue was finished and moved on, want active. "+
				"A closed cycle records what it held, not what happened to the work later.",
			report.Results[0].Category,
		)
	}

	if report.Results[0].Decision != entity.CycleRolloverKeep {
		t.Fatalf("the recorded decision reads %q, want keep", report.Results[0].Decision)
	}

	held, err := issuerepo.New(client).CyclesOf(
		ctx, world.workspace.ID, []uuid.UUID{staying.ID},
		entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true},
	)
	if err != nil {
		t.Fatalf("read where the issue ended up: %v", err)
	}

	if held[staying.ID] != later.ID {
		t.Fatalf("the issue sits in %v, want the later cycle %v", held[staying.ID], later.ID)
	}
}

var errWebhookRefused = errors.New("the webhook could not be recorded")

func TestACloseThatFailsAfterItsWritesLeavesNothingBehind(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCyclesEmitting(t, client, errWebhookRefused)

	ended := world.cycleOn(t, 1, "2026-08-24", "2026-08-30")
	next := world.cycleOn(t, 2, "2026-08-31", "2026-09-13")

	finished := world.issueIn(t, ended, world.states[2])
	rolling := world.issueIn(t, ended, world.states[1])

	ctx := context.Background()
	scope := entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true}
	wanted := []uuid.UUID{finished.ID, rolling.ID}

	before, err := issuerepo.New(client).ListVisibleByIDs(ctx, scope, wanted)
	if err != nil {
		t.Fatalf("read the issues before the close: %v", err)
	}

	if _, err := cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
		Rollover: entity.CycleRolloverNext,
		Reviewed: []uuid.UUID{rolling.ID},
	}); !errors.Is(err, errWebhookRefused) {
		t.Fatalf("Close returned %v, want the refusal raised after everything had been written", err)
	}

	results, err := cyclerepo.NewResult(client).ListByCycleID(ctx, ended.ID)
	if err != nil {
		t.Fatalf("read the recorded results: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf(
			"%d snapshot rows survived a close that failed at its last step, after the moves, the "+
				"scope rows and the closed marker had all been written",
			len(results),
		)
	}

	changes, err := cyclerepo.NewScopeChange(client).ListByCycleID(ctx, ended.ID, scope)
	if err != nil {
		t.Fatalf("read the scope ledger: %v", err)
	}

	if len(changes) != 0 {
		t.Fatalf("%d scope rows survived the failed close", len(changes))
	}

	held, err := cyclerepo.New(client).GetVisible(ctx, world.workspace.ID, ended.ID, scope)
	if err != nil {
		t.Fatalf("read the cycle back: %v", err)
	}

	if held.Closed() || held.ResultsRecordedAt != nil {
		t.Fatalf("the cycle reads closed at %v with results stamped %v after the failed close",
			held.ClosedAt, held.ResultsRecordedAt)
	}

	after, err := issuerepo.New(client).ListVisibleByIDs(ctx, scope, wanted)
	if err != nil {
		t.Fatalf("read the issues after the close: %v", err)
	}

	was := map[uuid.UUID]entity.Issue{}
	for _, issue := range before {
		was[issue.ID] = issue
	}

	if len(after) != len(before) {
		t.Fatalf("%d issues came back, want the %d that were there", len(after), len(before))
	}

	for _, issue := range after {
		if issue.CycleID != was[issue.ID].CycleID || issue.Version != was[issue.ID].Version {
			t.Fatalf(
				"%s reads cycle %v version %d after the failed close, was cycle %v version %d. The "+
					"whole close is one transaction, so a rolled-over issue must not keep the move "+
					"when the close itself did not happen.",
				issue.Reference(), issue.CycleID, issue.Version,
				was[issue.ID].CycleID, was[issue.ID].Version,
			)
		}
	}

	landed, err := issuerepo.New(client).CyclesOf(ctx, world.workspace.ID, wanted, scope)
	if err != nil {
		t.Fatalf("read where the issues sit: %v", err)
	}

	if landed[rolling.ID] == next.ID {
		t.Fatalf("the rolled issue stayed in the next cycle %v after the close was rolled back", next.ID)
	}
}

func TestClosingWhileSomebodyMovesAnIssueOutSettlesOneWayOrTheOther(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)
	issues := liveIssues(t, client, world)

	scope := entity.TeamScope{WorkspaceID: world.workspace.ID, AllTeams: true}
	ctx := context.Background()

	for attempt := range 8 {
		opening := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, attempt*14)
		ended := world.cycleOn(
			t, attempt*2+1,
			opening.Format(time.DateOnly), opening.AddDate(0, 0, 6).Format(time.DateOnly),
		)
		elsewhere := world.cycleOn(
			t, attempt*2+2,
			opening.AddDate(0, 0, 7).Format(time.DateOnly),
			opening.AddDate(0, 0, 13).Format(time.DateOnly),
		)
		leaving := world.issueIn(t, ended, world.states[1])

		read, err := issues.Get(ctx, world.workspace.ID, leaving.ID)
		if err != nil {
			t.Fatalf("read the issue: %v", err)
		}

		var (
			closed, moved error
			ready         = make(chan struct{})
			done          = make(chan struct{}, 2)
		)

		go func() {
			<-ready

			_, closed = cycles.Close(ctx, world.workspace.ID, ended.ID, service.CloseCycleInput{
				Rollover: entity.CycleRolloverKeep,
				Reviewed: []uuid.UUID{leaving.ID},
			})

			done <- struct{}{}
		}()

		go func() {
			<-ready

			_, moved = issues.Update(ctx, world.workspace.ID, leaving.ID, service.UpdateIssueInput{
				CycleID: &elsewhere.ID, ExpectedVersion: read.Version,
			})

			done <- struct{}{}
		}()

		close(ready)
		<-done
		<-done

		for _, err := range []error{closed, moved} {
			if err != nil && strings.Contains(err.Error(), "deadlock") {
				t.Fatalf("attempt %d deadlocked: %v", attempt, err)
			}
		}

		results, err := cyclerepo.NewResult(client).ListByCycleID(ctx, ended.ID)
		if err != nil {
			t.Fatalf("read the recorded results: %v", err)
		}

		held, err := issuerepo.New(client).CyclesOf(ctx, world.workspace.ID, []uuid.UUID{leaving.ID}, scope)
		if err != nil {
			t.Fatalf("read where the issue sits: %v", err)
		}

		shut, err := cyclerepo.New(client).GetVisible(ctx, world.workspace.ID, ended.ID, scope)
		if err != nil {
			t.Fatalf("read the cycle back: %v", err)
		}

		snapshotted := len(results) == 1 && results[0].IssueID == leaving.ID
		left := held[leaving.ID] == elsewhere.ID

		switch {
		case closed == nil:
			if !shut.Closed() {
				t.Fatalf("attempt %d: closing reported success and the cycle is still open", attempt)
			}

			if !snapshotted {
				t.Fatalf(
					"attempt %d: the close accepted a reviewed set naming this issue, so it was a "+
						"member when the snapshot was taken, yet the cycle recorded %d results. "+
						"Leaving afterwards is allowed; being left out of the record is not.",
					attempt, len(results),
				)
			}

			if results[0].Category != entity.StateCategoryActive {
				t.Fatalf(
					"attempt %d: the snapshot recorded %q, want the category the issue actually had "+
						"when the cycle closed",
					attempt, results[0].Category,
				)
			}

			if results[0].Decision != entity.CycleRolloverKeep {
				t.Fatalf(
					"attempt %d: the snapshot recorded the decision as %q, want keep",
					attempt, results[0].Decision,
				)
			}

		case errors.Is(closed, entity.ErrCycleStale):
			if !left {
				t.Fatalf(
					"attempt %d: the close was refused for changed membership, but the issue never "+
						"left the cycle",
					attempt,
				)
			}

			if shut.Closed() || len(results) != 0 {
				t.Fatalf(
					"attempt %d: a refused close left the cycle at closed=%v with %d recorded results",
					attempt, shut.Closed(), len(results),
				)
			}

		default:
			t.Fatalf("attempt %d: closing failed with %v", attempt, closed)
		}

		after, err := issues.Get(ctx, world.workspace.ID, leaving.ID)
		if err != nil {
			t.Fatalf("read the issue back: %v", err)
		}

		if moved != nil {
			t.Fatalf(
				"attempt %d: moving the issue on failed with %v, and it now reads cycle %v version %d "+
					"against cycle %v version %d before. Work kept in a cycle stays movable whether or "+
					"not the cycle closed underneath the move, so a refusal here traps the issue.",
				attempt, moved, after.CycleID, after.Version, read.CycleID, read.Version,
			)
		}

		if !left {
			t.Fatalf("attempt %d: the move reported success but the issue is in %v", attempt, held[leaving.ID])
		}
	}
}
