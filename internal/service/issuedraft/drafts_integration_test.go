package issuedraft_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal"
	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	issuedraftrepo "github.com/usenorn/norn/internal/repository/issuedraft"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuedraftsvc "github.com/usenorn/norn/internal/service/issuedraft"
)

type desk struct {
	workspace entity.Workspace
	team      entity.Team
	writer    uuid.UUID
	other     uuid.UUID
}

func draftDatabase(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := os.Getenv("NORN_TEST_POSTGRES_DSN")
	if os.Getenv("NORN_TEST_INTEGRATION") != "true" || dsn == "" {
		t.Skip("set NORN_TEST_INTEGRATION=true and NORN_TEST_POSTGRES_DSN to run this")
	}

	name := fmt.Sprintf("norn_drafts_%d", time.Now().UnixNano())

	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatalf(
			"NORN_TEST_POSTGRES_DSN must be a postgres:// URL so the database name can be replaced " +
				"safely; running against the original would write fixtures into it",
		)
	}

	parsed.Path = "/" + name

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
		DSN:             parsed.String(),
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

	if err := client.QueryRowContext(ctx, "SELECT current_database()").Scan(&reached); err != nil {
		t.Fatalf("read which database the connection reached: %v", err)
	}

	if reached != name {
		t.Fatalf("the connection reached %q, want the scratch database %q", reached, name)
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

func writingDrafts(t *testing.T, client *postgres.Client) (func(uuid.UUID) service.IssueDrafts, *desk) {
	t.Helper()

	ctx := context.Background()

	place := &desk{}

	accounts := accountrepo.New(client)

	for _, target := range []*uuid.UUID{&place.writer, &place.other} {
		account, err := accounts.Create(ctx, entity.Account{
			ID: uuid.New(), Status: entity.AccountStatusActive, Kind: entity.AccountKindPerson,
			Email: "drafter-" + uuid.NewString()[:8] + "@northwind.co", DisplayName: "Drafter",
			Timezone: "UTC",
		})
		if err != nil {
			t.Fatalf("create the account: %v", err)
		}

		*target = account.ID
	}

	workspace, err := workspacerepo.New(client).Create(ctx, entity.Workspace{
		ID: uuid.New(), Slug: "drafts-" + uuid.NewString()[:8], Name: "Drafting",
		Status: entity.WorkspaceStatusActive, Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("create the workspace: %v", err)
	}

	team, err := teamrepo.New(client).Create(ctx, entity.Team{
		ID: uuid.New(), WorkspaceID: workspace.ID, Key: "DFT", Name: "Drafting",
		IconColor: entity.DefaultTeamColor, Estimation: entity.TeamEstimationPoints,
		Status: entity.TeamStatusActive, Visibility: entity.DefaultTeamVisibility,
	})
	if err != nil {
		t.Fatalf("create the team: %v", err)
	}

	place.workspace, place.team = workspace, team

	drafts := issuedraftrepo.New(client)

	return func(accountID uuid.UUID) service.IssueDrafts {
		ctrl := gomock.NewController(t)
		authorizer := authorizersvc.NewMockAuthorizer(ctrl)

		authorizer.EXPECT().
			Decide(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
				return entity.Decision{
					Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID},
					Workspace: workspace,
					Scope:     entity.TeamScope{WorkspaceID: workspace.ID, AllTeams: true},
				}, nil
			}).
			AnyTimes()

		return issuedraftsvc.New(drafts, authorizer)
	}, place
}

func TestADraftKeptOnTheServerComesBackWithEverythingChosen(t *testing.T) {
	client := draftDatabase(t)
	drafting, place := writingDrafts(t, client)

	ctx := context.Background()
	writer := drafting(place.writer)

	saved, err := writer.Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		TeamID:      place.team.ID,
		Title:       "Half a thought",
		Description: "- [ ] finish this\n- [ ] then raise it",
		Priority:    entity.IssuePriorityHigh,
		Estimate:    3,
		DueOn:       "2026-10-01",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	kept, err := writer.List(ctx, place.workspace.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 1 {
		t.Fatalf("the desk holds %d drafts, want the one", len(kept))
	}

	held := kept[0]

	switch {
	case held.ID != saved.ID:
		t.Fatalf("the draft came back as %s, want %s", held.ID, saved.ID)
	case held.Title != "Half a thought":
		t.Fatalf("the title came back as %q", held.Title)
	case held.TeamID != place.team.ID:
		t.Fatalf("the team came back as %s", held.TeamID)
	case held.Priority != entity.IssuePriorityHigh:
		t.Fatalf("the priority came back as %q", held.Priority)
	case held.Estimate != 3:
		t.Fatalf("the estimate came back as %d", held.Estimate)
	case held.DueOn != "2026-10-01":
		t.Fatalf("the due date came back as %q", held.DueOn)
	case held.DescriptionDoc.Type != entity.DocumentType:
		t.Fatalf("the description came back as %q, not a document", held.DescriptionDoc.Type)
	}

	if content := held.DescriptionDoc.Content; len(content) != 1 || content[0].Type != entity.NodeTaskList {
		t.Fatalf("the description came back as %+v, want the checklist it was written as", content)
	}
}

func TestWritingMoreIntoADraftKeepsOneDraft(t *testing.T) {
	client := draftDatabase(t)
	drafting, place := writingDrafts(t, client)

	ctx := context.Background()
	writer := drafting(place.writer)

	started, err := writer.Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		TeamID: place.team.ID, Title: "First words",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := writer.Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		DraftID: started.ID, TeamID: place.team.ID, Title: "First words, then more",
	}); err != nil {
		t.Fatalf("Save again: %v", err)
	}

	kept, err := writer.List(ctx, place.workspace.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 1 || kept[0].Title != "First words, then more" {
		t.Fatalf("the desk holds %d drafts and reads %q", len(kept), kept[0].Title)
	}
}

func TestADraftIsNeverReadOrThrownAwayByAnybodyButItsWriter(t *testing.T) {
	client := draftDatabase(t)
	drafting, place := writingDrafts(t, client)

	ctx := context.Background()

	mine, err := drafting(place.writer).Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		TeamID: place.team.ID, Title: "Nobody else's business",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	stranger := drafting(place.other)

	theirs, err := stranger.List(ctx, place.workspace.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(theirs) != 0 {
		t.Fatalf(
			"somebody else's desk shows %d drafts. A draft is unfinished writing, and reading it "+
				"is not something a workspace membership grants.",
			len(theirs),
		)
	}

	if err := stranger.Remove(ctx, place.workspace.ID, mine.ID); !errors.Is(err, entity.ErrIssueDraftNotFound) {
		t.Fatalf("throwing away somebody else's draft returned %v, want a refusal", err)
	}

	kept, err := drafting(place.writer).List(ctx, place.workspace.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 1 {
		t.Fatalf("the writer's draft is gone")
	}
}

func TestThrowingADraftAwayLeavesTheRest(t *testing.T) {
	client := draftDatabase(t)
	drafting, place := writingDrafts(t, client)

	ctx := context.Background()
	writer := drafting(place.writer)

	first, err := writer.Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		TeamID: place.team.ID, Title: "One",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := writer.Save(ctx, place.workspace.ID, service.SaveIssueDraftInput{
		TeamID: place.team.ID, Title: "Two",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := writer.Remove(ctx, place.workspace.ID, first.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	kept, err := writer.List(ctx, place.workspace.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(kept) != 1 || kept[0].Title != "Two" {
		t.Fatalf("the desk holds %d drafts and reads %q", len(kept), kept[0].Title)
	}
}
