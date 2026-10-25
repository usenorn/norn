package knowledge

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
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errKnowledgeRollback = errors.New("roll the knowledge fixture back")

const knowledgeFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Knowledge fixture')
    RETURNING id
), author AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC')
    RETURNING id
), outsider AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($5, 'active', 'person', $6, 'Sol', 'UTC')
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'platform', 'Platform' FROM workspace
    RETURNING id
), other AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'billing', 'Billing' FROM workspace
    RETURNING id
)
SELECT project.id, other.id FROM project, other, author, outsider`

const insertMemoryQuery = `
INSERT INTO workspace_memories (
    id, workspace_id, kind, statement, rationale, conditions, visibility, project_id, applies_project_id, component,
    status, basis, author_account_id, source, effective_from, effective_until, superseded_by_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, '', $6, $7, $8, $9, $10, 'observation', $11, 'person', $12, $13, $14, $15, $15)`

const insertDocQuery = `
INSERT INTO workspace_docs (id, workspace_id, title, content, markdown, visibility, project_id, author_account_id)
VALUES ($1, $2, $3, '{"type":"doc"}', $4, $5, $6, $7)`

const insertPassageQuery = `
INSERT INTO workspace_doc_passages (workspace_id, doc_id, doc_version, position, heading, body)
VALUES ($1, $2, 1, $3, $4, $5)`

type knowledgeFixture struct {
	workspaceID, authorID, outsiderID, projectID, otherProjectID uuid.UUID
}

type fixtureMemory struct {
	id, projectID, appliesProjectID, supersededByID uuid.UUID
	kind, statement, rationale, visibility          string
	component, status                               string
	effectiveFrom                                   time.Time
	effectiveUntil                                  *time.Time
	authorID                                        uuid.UUID
	at                                              time.Time
}

func withKnowledgeFixture(
	t *testing.T,
	run func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error,
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
		fixture := knowledgeFixture{workspaceID: uuid.New(), authorID: uuid.New(), outsiderID: uuid.New()}

		if err := client.Querier(ctx).QueryRowContext(
			ctx,
			knowledgeFixtureQuery,
			fixture.workspaceID,
			"knowledge-"+fixture.workspaceID.String()[:8],
			fixture.authorID,
			fixture.authorID.String()+"@example.test",
			fixture.outsiderID,
			fixture.outsiderID.String()+"@example.test",
		).Scan(&fixture.projectID, &fixture.otherProjectID); err != nil {
			failure = fmt.Errorf("lay knowledge fixture: %w", err)

			return errKnowledgeRollback
		}

		failure = run(ctx, client, fixture)

		return errKnowledgeRollback
	})
	if !errors.Is(err, errKnowledgeRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func (f knowledgeFixture) memory(ctx context.Context, client *postgres.Client, memory fixtureMemory) (uuid.UUID, error) {
	if memory.id == uuid.Nil {
		memory.id = uuid.New()
	}

	if memory.kind == "" {
		memory.kind = "constraint"
	}

	if memory.visibility == "" {
		memory.visibility = "workspace"
	}

	if memory.status == "" {
		memory.status = "accepted"
	}

	if memory.authorID == uuid.Nil {
		memory.authorID = f.authorID
	}

	if memory.at.IsZero() {
		memory.at = time.Now().UTC()
	}

	if memory.effectiveFrom.IsZero() {
		memory.effectiveFrom = memory.at
	}

	_, err := client.Querier(ctx).ExecContext(
		ctx, insertMemoryQuery,
		memory.id, f.workspaceID, memory.kind, memory.statement, memory.rationale, memory.visibility,
		nullable(memory.projectID), nullable(memory.appliesProjectID), memory.component, memory.status,
		memory.authorID, memory.effectiveFrom, memory.effectiveUntil, nullable(memory.supersededByID), memory.at,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert memory %q: %w", memory.statement, err)
	}

	return memory.id, nil
}

func (f knowledgeFixture) doc(
	ctx context.Context,
	client *postgres.Client,
	title, visibility string,
	projectID, authorID uuid.UUID,
	passages ...[2]string,
) (uuid.UUID, error) {
	docID := uuid.New()

	var markdown strings.Builder
	for _, passage := range passages {
		markdown.WriteString(passage[1] + "\n\n")
	}

	if _, err := client.Querier(ctx).ExecContext(
		ctx, insertDocQuery, docID, f.workspaceID, title, markdown.String(), visibility, nullable(projectID), authorID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert doc %q: %w", title, err)
	}

	for position, passage := range passages {
		if _, err := client.Querier(ctx).ExecContext(
			ctx, insertPassageQuery, f.workspaceID, docID, position, passage[0], passage[1],
		); err != nil {
			return uuid.Nil, fmt.Errorf("insert passage %q: %w", passage[0], err)
		}
	}

	return docID, nil
}
