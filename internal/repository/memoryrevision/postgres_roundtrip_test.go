package memoryrevision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
)

var errMemoryRollback = errors.New("roll the memory fixture back")

const memoryFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Memory fixture')
    RETURNING id
), author AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC')
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'platform', 'Platform' FROM workspace
    RETURNING id
), doc AS (
    INSERT INTO workspace_docs (id, workspace_id, title, content, visibility, author_account_id)
    SELECT $5, workspace.id, 'Storage notes', '{"type":"doc"}', 'personal', author.id FROM workspace, author
    RETURNING id, workspace_id
), revision AS (
    INSERT INTO workspace_doc_revisions (id, workspace_id, doc_id, version, action, title, content, visibility, source)
    SELECT $6, workspace_id, id, 1, 'created', 'Storage notes', '{"type":"doc"}', 'personal', 'person' FROM doc
    RETURNING id
)
SELECT project.id FROM project, revision`

const tokenFixtureQuery = `
INSERT INTO api_tokens (id, account_id, name, token_hash, scopes)
VALUES ($1, $2, 'Memory agent', $3, '{knowledge:manage}')`

func layToken(ctx context.Context, client *postgres.Client, accountID uuid.UUID) (*uuid.UUID, error) {
	tokenID := uuid.New()
	if _, err := client.Querier(ctx).ExecContext(ctx, tokenFixtureQuery, tokenID, accountID, tokenID[:]); err != nil {
		return nil, fmt.Errorf("lay token fixture: %w", err)
	}

	return &tokenID, nil
}

type memoryFixture struct {
	workspaceID, authorID, projectID, docID, docRevisionID uuid.UUID
}

func withMemoryFixture(
	t *testing.T,
	run func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error,
) {
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
	defer cleanup()

	var failure error

	err = client.WithTx(context.Background(), func(ctx context.Context) error {
		fixture := memoryFixture{
			workspaceID:   uuid.New(),
			authorID:      uuid.New(),
			docID:         uuid.New(),
			docRevisionID: uuid.New(),
		}

		if err := client.Querier(ctx).QueryRowContext(
			ctx,
			memoryFixtureQuery,
			fixture.workspaceID,
			"memories-"+fixture.workspaceID.String()[:8],
			fixture.authorID,
			fixture.authorID.String()+"@example.test",
			fixture.docID,
			fixture.docRevisionID,
		).Scan(&fixture.projectID); err != nil {
			failure = fmt.Errorf("lay memory fixture: %w", err)

			return errMemoryRollback
		}

		failure = run(ctx, client, fixture)

		return errMemoryRollback
	})
	if !errors.Is(err, errMemoryRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func draftMemory(fixture memoryFixture, statement string, audience entity.KnowledgeAudience, at time.Time) entity.Memory {
	return entity.Memory{
		ID:            uuid.New(),
		WorkspaceID:   fixture.workspaceID,
		Kind:          entity.MemoryKindConstraint,
		Statement:     statement,
		Conditions:    "local prototypes only",
		Audience:      audience,
		Status:        entity.MemoryStatusAccepted,
		Basis:         entity.MemoryBasisObservation,
		AuthorID:      fixture.authorID,
		By:            entity.KnowledgeAttribution{AccountID: fixture.authorID, Source: entity.RevisionSourcePerson},
		EffectiveFrom: entity.MemoryDay(at),
		Version:       1,
		CreatedAt:     at,
		UpdatedAt:     at,
	}
}

func TestAMemoryRevisionKeepsTheEvidenceItWasRecordedWith(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := memoryrepo.New(client)
		revisions := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
		connectionID := uuid.New()

		memory := draftMemory(fixture, "Use SQLite", entity.WorkspaceAudience(), at)
		memory.Applicability = entity.KnowledgeApplicability{ProjectID: &fixture.projectID, Component: "storage"}
		memory.Evidence = []entity.MemoryEvidence{
			{Kind: entity.MemoryEvidenceDocRevision, DocID: &fixture.docID, DocRevisionID: &fixture.docRevisionID},
			{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "the build broke on SQLite"},
		}

		if err := memories.Create(ctx, memory); err != nil {
			return fmt.Errorf("create memory: %w", err)
		}

		tokenID, err := layToken(ctx, client, fixture.authorID)
		if err != nil {
			return err
		}

		by := entity.KnowledgeAttribution{
			AccountID: fixture.authorID, ConnectionID: &connectionID, TokenID: tokenID, Source: entity.RevisionSourceAgent,
		}
		first := memory.Revision(entity.MemoryRevisionCreated, by)

		memory.Version, memory.Status, memory.Evidence = 2, entity.MemoryStatusWithdrawn, nil
		memory.UpdatedAt = at.Add(time.Hour)
		second := memory.Revision(entity.MemoryRevisionWithdrawn, by)

		for _, revision := range []entity.MemoryRevision{first, second} {
			if err := revisions.Record(ctx, revision); err != nil {
				return fmt.Errorf("record version %d: %w", revision.Version, err)
			}
		}

		history, err := revisions.List(ctx, fixture.workspaceID, memory.ID, entity.MemoryRevisionPage{Limit: 10})
		if err != nil {
			return fmt.Errorf("list revisions: %w", err)
		}

		if len(history) != 2 || history[0].Action != entity.MemoryRevisionWithdrawn || history[1].Action != entity.MemoryRevisionCreated {
			return fmt.Errorf("history read back as %+v", history)
		}

		if history[0].AccountName != "Rae" || history[0].By.ConnectionID == nil || *history[0].By.ConnectionID != connectionID ||
			history[0].By.TokenID == nil || *history[0].By.TokenID != *tokenID {
			return fmt.Errorf("history lost its attribution: %+v", history[0])
		}

		read, err := revisions.GetByID(ctx, fixture.workspaceID, memory.ID, first.ID)
		if err != nil {
			return fmt.Errorf("read first revision: %w", err)
		}

		if read.Status != entity.MemoryStatusAccepted || read.Applicability.Component != "storage" || len(read.Evidence) != 2 {
			return fmt.Errorf("the first revision read back as %+v", read)
		}

		cited := read.Evidence[0]
		if cited.DocRevisionID == nil || *cited.DocRevisionID != fixture.docRevisionID ||
			cited.DocAudience.Visibility != entity.KnowledgeVisibilityPersonal || cited.DocAuthorID != fixture.authorID {
			return fmt.Errorf("the cited doc revision read back as %+v", cited)
		}

		if read.Evidence[1].Excerpt != "the build broke on SQLite" {
			return fmt.Errorf("the excerpt read back as %+v", read.Evidence[1])
		}

		if _, err := revisions.GetByID(ctx, fixture.workspaceID, memory.ID, uuid.New()); !errors.Is(err, entity.ErrMemoryRevisionNotFound) {
			return fmt.Errorf("an unknown revision got %v, want ErrMemoryRevisionNotFound", err)
		}

		return nil
	})
}
