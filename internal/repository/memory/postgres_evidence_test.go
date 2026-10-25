package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

const citedIssueFixtureQuery = `
WITH member AS (
    INSERT INTO workspace_memberships (workspace_id, account_id, role)
    VALUES ($1, $4, 'member')
    RETURNING account_id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    VALUES ($1, 'MEM', 'Memory')
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (id, workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT $2, team.workspace_id, team.id, 1, 'Pick a database', state.id, 'MEM', 'n'
    FROM team, state
    RETURNING id, workspace_id
)
INSERT INTO workspace_issue_comments (id, workspace_id, issue_id, author_account_id, author_kind, body)
SELECT $3, issue.workspace_id, issue.id, member.account_id, 'person', 'SQLite broke the build'
FROM issue, member`

func evidenceStates(ctx context.Context, memories *memoryRepository, fixture memoryFixture, memoryID uuid.UUID) ([]entity.MemoryEvidenceState, error) {
	memory, err := memories.GetByID(ctx, fixture.workspaceID, memoryID)
	if err != nil {
		return nil, err
	}

	states := make([]entity.MemoryEvidenceState, 0, len(memory.Evidence))
	for _, evidence := range memory.Evidence {
		states = append(states, evidence.State)
	}

	return states, nil
}

func TestEvidenceReadsBackChangedOrUnavailableOnceItsSourceMoves(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := &memoryRepository{db: client}
		issueID, commentID, missingRevision := uuid.New(), uuid.New(), uuid.New()
		one := 1

		if _, err := client.Querier(ctx).ExecContext(ctx, citedIssueFixtureQuery,
			fixture.workspaceID, issueID, commentID, fixture.authorID); err != nil {
			return fmt.Errorf("lay cited issue fixture: %w", err)
		}

		draft := draftMemory(fixture, "Use SQLite", entity.PersonalAudience(), time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
		draft.Evidence = []entity.MemoryEvidence{
			{Kind: entity.MemoryEvidenceDocRevision, DocID: &fixture.docID, DocRevisionID: &fixture.docRevisionID},
			{Kind: entity.MemoryEvidenceIssue, IssueID: &issueID, IssueVersion: &one},
			{Kind: entity.MemoryEvidenceComment, IssueID: &issueID, CommentID: &commentID, CommentVersion: &one},
			{Kind: entity.MemoryEvidenceDocRevision, DocID: &fixture.docID, DocRevisionID: &missingRevision},
			{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "the build broke on SQLite"},
		}

		if err := memories.Create(ctx, draft); err != nil {
			return fmt.Errorf("create memory: %w", err)
		}

		run := func(query string, args ...any) error {
			_, err := client.Querier(ctx).ExecContext(ctx, query, args...)

			return err
		}

		revise := func(version int, action, title string) error {
			return run(`
INSERT INTO workspace_doc_revisions (workspace_id, doc_id, version, action, title, content, visibility, source)
VALUES ($1, $2, $3, $4, $5, '{"type":"doc"}', 'personal', 'person')`,
				fixture.workspaceID, fixture.docID, version, action, title)
		}

		steps := []struct {
			name  string
			apply func() error
			want  string
		}{
			{
				name:  "as cited",
				apply: func() error { return nil },
				want:  "[current current current unavailable current]",
			},
			{
				name: "after the doc moves and the issue changes state",
				apply: func() error {
					if err := revise(2, "moved", "Storage notes"); err != nil {
						return err
					}

					return run(`UPDATE workspace_issues SET version = 2, field_versions = '{"state": 2}' WHERE id = $1`, issueID)
				},
				want: "[current current current unavailable current]",
			},
			{
				name: "after the doc, the issue description, and the comment are edited",
				apply: func() error {
					if err := revise(3, "edited", "Storage decisions"); err != nil {
						return err
					}

					if err := run(`UPDATE workspace_issues SET version = 3, field_versions = '{"state": 2, "description": 3}'
WHERE id = $1`, issueID); err != nil {
						return err
					}

					return run(`UPDATE workspace_issue_comments SET version = 2 WHERE id = $1`, commentID)
				},
				want: "[changed changed changed unavailable current]",
			},
			{
				name: "after the comment and the issue are deleted",
				apply: func() error {
					if err := run(`DELETE FROM workspace_issue_comments WHERE id = $1`, commentID); err != nil {
						return err
					}

					return run(`DELETE FROM workspace_issues WHERE id = $1`, issueID)
				},
				want: "[changed unavailable unavailable unavailable current]",
			},
		}

		for _, step := range steps {
			if err := step.apply(); err != nil {
				return fmt.Errorf("%s: %w", step.name, err)
			}

			states, err := evidenceStates(ctx, memories, fixture, draft.ID)
			if err != nil {
				return fmt.Errorf("%s: %w", step.name, err)
			}

			if fmt.Sprint(states) != step.want {
				return fmt.Errorf("%s: evidence reads %v, want %s", step.name, states, step.want)
			}
		}

		memory, err := memories.GetByID(ctx, fixture.workspaceID, draft.ID)
		if err != nil {
			return err
		}

		if memory.Evidence[0].DocTitle != "Storage notes" {
			return fmt.Errorf("a cited revision reads under the doc's current title %q", memory.Evidence[0].DocTitle)
		}

		return nil
	})
}
