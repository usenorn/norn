package issue_test

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
	"github.com/usenorn/norn/internal/repository"
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
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

type world struct {
	client    *postgres.Client
	workspace entity.Workspace
	team      entity.Team
	state     entity.WorkflowState
	account   uuid.UUID
}

func documentDatabase(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := os.Getenv("NORN_TEST_POSTGRES_DSN")
	if os.Getenv("NORN_TEST_INTEGRATION") != "true" || dsn == "" {
		t.Skip("set NORN_TEST_INTEGRATION=true and NORN_TEST_POSTGRES_DSN to run this")
	}

	name := fmt.Sprintf("norn_documents_%d", time.Now().UnixNano())

	scratch, err := documentDSN(dsn, name)
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
			"the connection reached %q, want the scratch database %q. Migrating anything other than "+
				"the database this test created would write fixtures into somebody's data.",
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

func documentDSN(dsn, name string) (string, error) {
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

func writingIssues(t *testing.T, client *postgres.Client) (service.Issues, repository.IssueRevision, *world) {
	t.Helper()

	ctrl := gomock.NewController(t)

	place := &world{client: client}

	ctx := context.Background()

	account, err := accountrepo.New(client).Create(ctx, entity.Account{
		ID: uuid.New(), Status: entity.AccountStatusActive, Kind: entity.AccountKindPerson,
		Email: "writer-" + uuid.NewString()[:8] + "@northwind.co", DisplayName: "Writer",
		Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("create the account: %v", err)
	}

	place.account = account.ID

	workspace, err := workspacerepo.New(client).Create(ctx, entity.Workspace{
		ID: uuid.New(), Slug: "docs-" + uuid.NewString()[:8], Name: "Writing",
		Status: entity.WorkspaceStatusActive, Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("create the workspace: %v", err)
	}

	team, err := teamrepo.New(client).Create(ctx, entity.Team{
		ID: uuid.New(), WorkspaceID: workspace.ID, Key: "WRT", Name: "Writing",
		IconColor: entity.DefaultTeamColor, Estimation: entity.TeamEstimationPoints,
		Status: entity.TeamStatusActive, Visibility: entity.DefaultTeamVisibility,
	})
	if err != nil {
		t.Fatalf("create the team: %v", err)
	}

	state, err := workflowstaterepo.New(client).Create(ctx, entity.WorkflowState{
		ID: uuid.New(), WorkspaceID: workspace.ID, TeamID: team.ID, Name: "Todo",
		Category: entity.StateCategoryNotStarted, Position: 1, IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create the state: %v", err)
	}

	if _, err := membershiprepo.New(client).Create(ctx, entity.Membership{
		WorkspaceID: workspace.ID, AccountID: account.ID,
		Role: entity.MembershipRoleAdmin, Source: entity.DefaultMembershipSource,
	}); err != nil {
		t.Fatalf("join the workspace: %v", err)
	}

	place.workspace, place.team, place.state = workspace, team, state

	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: place.account},
				Workspace: workspace,
				Scope:     entity.TeamScope{WorkspaceID: workspace.ID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	events := eventsvc.NewMockEvents(ctrl)
	events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	jobs := jobqueuerepo.NewMockJobProducer(ctrl)
	jobs.EXPECT().EnqueueBulkApply(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	revisions := issuerevisionrepo.New(client)

	return issuesvc.New(
		issuerepo.New(client),
		revisions,
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
		emitter,
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
	), revisions, place
}

const writtenDescription = `## What is wrong

The importer drops **inline** images when the message is ` + "`multipart/related`" + `.

- [x] reproduce it
- [ ] fix it

> reported by a customer

| field | value |
| --- | --- |
| messages | 3 |
`

func TestADescriptionWrittenAsMarkdownIsStoredAndReadBackAsADocument(t *testing.T) {
	client := documentDatabase(t)
	issues, _, place := writingIssues(t, client)

	ctx := context.Background()

	created, err := issues.Create(ctx, service.CreateIssueInput{
		WorkspaceID: place.workspace.ID,
		TeamID:      place.team.ID,
		Title:       "Inline images are dropped",
		Description: writtenDescription,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	read, err := issues.Get(ctx, place.workspace.ID, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if read.DescriptionDoc.Type != entity.DocumentType {
		t.Fatalf("the description came back as %q, want a document", read.DescriptionDoc.Type)
	}

	if got := read.DescriptionDoc.Markdown(); got != read.Description {
		t.Fatalf("the stored document renders to\n%q\nbut the description reads\n%q", got, read.Description)
	}

	if read.Description != strings.TrimRight(writtenDescription, "\n") {
		t.Fatalf("the description came back as\n%q\nwant\n%q", read.Description, writtenDescription)
	}
}

func TestADescriptionWrittenBeforeDocumentsIsReadAsOne(t *testing.T) {
	client := documentDatabase(t)
	issues, _, place := writingIssues(t, client)

	ctx := context.Background()

	created, err := issues.Create(ctx, service.CreateIssueInput{
		WorkspaceID: place.workspace.ID,
		TeamID:      place.team.ID,
		Title:       "Written before documents",
		Description: "A plain **line**.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := client.ExecContext(
		ctx, "UPDATE workspace_issues SET description_doc = NULL WHERE id = $1", created.ID,
	); err != nil {
		t.Fatalf("empty the document column: %v", err)
	}

	read, err := issues.Get(ctx, place.workspace.ID, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if read.DescriptionDoc.Type != entity.DocumentType {
		t.Fatalf("a row without a document came back as %q", read.DescriptionDoc.Type)
	}

	if got := read.DescriptionDoc.Markdown(); got != "A plain **line**." {
		t.Fatalf("the row was read as %q", got)
	}
}

func TestEditingADescriptionKeepsWhatItReplaced(t *testing.T) {
	client := documentDatabase(t)
	issues, revisions, place := writingIssues(t, client)

	ctx := context.Background()

	created, err := issues.Create(ctx, service.CreateIssueInput{
		WorkspaceID: place.workspace.ID,
		TeamID:      place.team.ID,
		Title:       "Requirements move",
		Description: "Ship the importer.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	second := "Ship the importer **and** the attachments."

	edited, err := issues.Update(ctx, place.workspace.ID, created.ID, service.UpdateIssueInput{
		ExpectedVersion: created.Version,
		Description:     &second,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	kept, err := revisions.List(ctx, place.workspace.ID, created.ID, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 2 {
		t.Fatalf("kept %d revisions, want the one it was created with and the one it was edited to", len(kept))
	}

	if kept[0].Markdown != second || kept[0].IssueVersion != edited.Version {
		t.Fatalf(
			"the newest revision reads %q at version %d, want %q at version %d",
			kept[0].Markdown, kept[0].IssueVersion, second, edited.Version,
		)
	}

	if kept[1].Markdown != "Ship the importer." {
		t.Fatalf("the replaced text reads %q", kept[1].Markdown)
	}

	if kept[0].AuthorAccountID != place.account || kept[0].Source != entity.RevisionSourcePerson {
		t.Fatalf("the revision was filed under %s as %q", kept[0].AuthorAccountID, kept[0].Source)
	}

	if kept[0].Doc.Markdown() != second {
		t.Fatalf("the stored revision document renders to %q", kept[0].Doc.Markdown())
	}
}

func TestASecondWriterOnTheSameVersionIsToldRatherThanOverwriting(t *testing.T) {
	client := documentDatabase(t)
	issues, revisions, place := writingIssues(t, client)

	ctx := context.Background()

	created, err := issues.Create(ctx, service.CreateIssueInput{
		WorkspaceID: place.workspace.ID,
		TeamID:      place.team.ID,
		Title:       "Two writers",
		Description: "The first sentence.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mine, theirs := "Mine.", "Theirs."

	if _, err := issues.Update(ctx, place.workspace.ID, created.ID, service.UpdateIssueInput{
		ExpectedVersion: created.Version,
		Description:     &mine,
	}); err != nil {
		t.Fatalf("the first write: %v", err)
	}

	_, err = issues.Update(ctx, place.workspace.ID, created.ID, service.UpdateIssueInput{
		ExpectedVersion: created.Version,
		Description:     &theirs,
	})

	var stale entity.IssueStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("the second write returned %v, want the reader to be told the issue moved on", err)
	}

	read, err := issues.Get(ctx, place.workspace.ID, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if read.Description != mine {
		t.Fatalf("the stored description reads %q, want the first write to have stood", read.Description)
	}

	kept, err := revisions.List(ctx, place.workspace.ID, created.ID, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 2 {
		t.Fatalf("kept %d revisions, want the refused write to have left none", len(kept))
	}
}
