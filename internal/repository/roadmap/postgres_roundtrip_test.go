package roadmap

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errRoadmapRollback = errors.New("roll the roadmap fixture back")

const roadmapFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Roadmap fixture')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'RDM', 'Roadmap' FROM workspace
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name, description)
    SELECT id, 'internal-project', 'Internal project name', 'Internal description' FROM workspace
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (workspace_id, team_id, number, title, state_id, reference_key, rank, project_id)
    SELECT team.workspace_id, team.id, 1, 'Internal issue title', state.id, 'RDM', 'n', project.id
    FROM team, state, project
    RETURNING id, team_id, project_id
)
SELECT issue.id, issue.team_id, issue.project_id FROM issue`

type roadmapFixture struct {
	workspaceID, issueID, teamID, projectID uuid.UUID
	slug                                    string
}

func liveRoadmapRepository(t *testing.T) (*postgres.Client, func()) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to test")
	}

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        2,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}

	return client, cleanup
}

func seedRoadmapFixture(ctx context.Context, client *postgres.Client) (roadmapFixture, error) {
	fixture := roadmapFixture{workspaceID: uuid.New()}
	fixture.slug = "roadmap-" + fixture.workspaceID.String()[:8]

	err := client.Querier(ctx).QueryRowContext(ctx, roadmapFixtureQuery, fixture.workspaceID, fixture.slug).
		Scan(&fixture.issueID, &fixture.teamID, &fixture.projectID)

	return fixture, err
}

func withRoadmapFixture(t *testing.T, run func(ctx context.Context, client *postgres.Client, fixture roadmapFixture) error) {
	t.Helper()

	client, cleanup := liveRoadmapRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		fixture, err := seedRoadmapFixture(ctx, client)
		if err != nil {
			failure = err

			return errRoadmapRollback
		}

		failure = run(ctx, client, fixture)

		return errRoadmapRollback
	})
	if !errors.Is(err, errRoadmapRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func draftEntry(roadmap entity.Roadmap, kind entity.RoadmapEntryKind, title string) (entity.RoadmapEntry, error) {
	publicID, err := entity.NewRoadmapPublicID()

	return entity.RoadmapEntry{
		RoadmapID:    roadmap.ID,
		WorkspaceID:  roadmap.WorkspaceID,
		PublicID:     publicID,
		Kind:         kind,
		PublicTitle:  title,
		Status:       entity.PublicStatusPlanned,
		ProgressMode: entity.RoadmapProgressHidden,
	}, err
}

func TestAPublishedRoadmapReadsItsSourcesLiveAndFailsClosed(t *testing.T) {
	withRoadmapFixture(t, func(ctx context.Context, client *postgres.Client, fixture roadmapFixture) error {
		roadmaps := New(client)

		roadmap, err := roadmaps.Ensure(ctx, fixture.workspaceID)
		if err != nil {
			return err
		}

		if again, err := roadmaps.Ensure(ctx, fixture.workspaceID); err != nil || again.ID != roadmap.ID {
			return errors.New("a second Ensure did not return the same roadmap")
		}

		if _, err := roadmaps.PublishedBySlug(ctx, fixture.slug); !errors.Is(err, entity.ErrRoadmapNotFound) {
			return errors.New("a roadmap that was never published was readable")
		}

		issueEntry, err := draftEntry(roadmap, entity.RoadmapEntryIssue, "Public issue")
		if err != nil {
			return err
		}

		issueEntry.SourceIssueID = fixture.issueID
		issueEntry.Timeline = entity.RoadmapTimeline{StartOn: "2026-11-01", EndOn: "2026-12-01"}

		created, err := roadmaps.CreateEntry(ctx, issueEntry)
		if err != nil {
			return err
		}

		if created.Timeline != issueEntry.Timeline {
			return errors.New("the timeline did not survive a round trip")
		}

		duplicate, err := draftEntry(roadmap, entity.RoadmapEntryIssue, "Again")
		if err != nil {
			return err
		}

		duplicate.SourceIssueID = fixture.issueID

		err = client.WithSavepoint(ctx, func(ctx context.Context) error {
			_, err := roadmaps.CreateEntry(ctx, duplicate)

			return err
		})
		if !errors.Is(err, entity.ErrRoadmapEntryDuplicate) {
			return errors.New("the same issue was added to the roadmap twice")
		}

		sources, err := roadmaps.EntrySources(ctx, roadmap.ID)
		if err != nil {
			return err
		}

		if source := sources[created.ID]; !source.Present || source.TeamID != fixture.teamID || source.ProjectID != fixture.projectID {
			return errors.New("the draft entry's source did not resolve")
		}

		published := entity.PublishedRoadmapEntry{
			RoadmapEntry: created,
			Approval:     entity.RoadmapApproval{TeamID: fixture.teamID, ProjectID: fixture.projectID},
		}

		if err := roadmaps.Publish(ctx, roadmap.ID, entity.RoadmapPublication{
			Title:       "Our roadmap",
			Version:     roadmap.Version,
			PublishedAt: time.Now().UTC(),
		}, []entity.PublishedRoadmapEntry{published}); err != nil {
			return err
		}

		read, err := roadmaps.PublishedBySlug(ctx, fixture.slug)
		if err != nil {
			return err
		}

		if len(read.Entries) != 1 || !read.Entries[0].Intact() || read.Entries[0].PublicTitle != "Public issue" {
			return errors.New("the published entry did not read back intact")
		}

		if read.Publication.PublishedAt.Location() != time.UTC {
			return errors.New("the publication time did not read back in UTC")
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx, "UPDATE workspace_teams SET visibility = 'private' WHERE id = $1", fixture.teamID,
		); err != nil {
			return err
		}

		read, err = roadmaps.PublishedBySlug(ctx, fixture.slug)
		if err != nil {
			return err
		}

		if read.Entries[0].Intact() {
			return errors.New("an issue whose team turned private stayed public")
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx, "DELETE FROM workspace_issues WHERE id = $1", fixture.issueID,
		); err != nil {
			return err
		}

		read, err = roadmaps.PublishedBySlug(ctx, fixture.slug)
		if err != nil {
			return err
		}

		if read.Entries[0].Source.Present {
			return errors.New("a deleted issue still read as present")
		}

		if err := roadmaps.DeletePublishedEntry(ctx, roadmap.ID, created.PublicID); err != nil {
			return err
		}

		read, err = roadmaps.PublishedBySlug(ctx, fixture.slug)
		if err != nil {
			return err
		}

		if len(read.Entries) != 0 || read.Publication.Title != "Our roadmap" {
			return errors.New("a removed entry stayed published or the empty roadmap lost its header")
		}

		if err := roadmaps.Unpublish(ctx, roadmap.ID); err != nil {
			return err
		}

		if _, err := roadmaps.PublishedBySlug(ctx, fixture.slug); !errors.Is(err, entity.ErrRoadmapNotFound) {
			return errors.New("an unpublished roadmap was still readable")
		}

		return nil
	})
}

func TestADraftEntryRefusesABlankPublicTitle(t *testing.T) {
	withRoadmapFixture(t, func(ctx context.Context, client *postgres.Client, fixture roadmapFixture) error {
		roadmaps := New(client)

		roadmap, err := roadmaps.Ensure(ctx, fixture.workspaceID)
		if err != nil {
			return err
		}

		entry, err := draftEntry(roadmap, entity.RoadmapEntryProject, "   ")
		if err != nil {
			return err
		}

		entry.SourceProjectID = fixture.projectID

		var validation entity.ValidationError
		if _, err := roadmaps.CreateEntry(ctx, entry); !errors.As(err, &validation) {
			return errors.New("a blank public title was stored")
		}

		return nil
	})
}

func TestADraftEntryKeepsTheFieldsItHides(t *testing.T) {
	withRoadmapFixture(t, func(ctx context.Context, client *postgres.Client, fixture roadmapFixture) error {
		roadmaps := New(client)

		roadmap, err := roadmaps.Ensure(ctx, fixture.workspaceID)
		if err != nil {
			return err
		}

		entry, err := draftEntry(roadmap, entity.RoadmapEntryProject, "Public project")
		if err != nil {
			return err
		}

		entry.SourceProjectID = fixture.projectID
		entry.PublicDescription = "Shown only when descriptions are on"

		created, err := roadmaps.CreateEntry(ctx, entry)
		if err != nil {
			return err
		}

		if created.Visibility != (entity.RoadmapVisibility{}) {
			return errors.New("an entry created with every field hidden came back showing some")
		}

		entries, err := roadmaps.ListEntries(ctx, roadmap.ID)
		if err != nil {
			return err
		}

		if len(entries) != 1 || entries[0].Visibility != (entity.RoadmapVisibility{}) {
			return errors.New("an entry created with every field hidden read back showing some")
		}

		return nil
	})
}
