package issuecomment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type issueCommentRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueComment {
	return &issueCommentRepository{db: db}
}

const commentColumns = `
       c.id,
       c.workspace_id,
       c.issue_id,
       coalesce(c.parent_comment_id::text, ''),
       coalesce(c.author_account_id::text, ''),
       coalesce(a.display_name, ''),
       c.author_kind,
       c.body,
       c.edited_at,
       c.deleted_at,
       c.created_at,
       c.updated_at`

const commentJoins = `
FROM workspace_issue_comments c
LEFT JOIN accounts a ON a.id = c.author_account_id`

const createCommentQuery = `
WITH created AS (
    INSERT INTO workspace_issue_comments
        (workspace_id, issue_id, parent_comment_id, author_account_id, author_kind, body)
    SELECT $1, $2, nullif($3, '')::uuid, author.id, author.kind, $5
    FROM accounts author
    WHERE author.id = $4::uuid
    RETURNING *
)
SELECT` + commentColumns + `
FROM created c
LEFT JOIN accounts a ON a.id = c.author_account_id`

const commentByIDQuery = `SELECT` + commentColumns + commentJoins + `
WHERE c.id = $1 AND c.workspace_id = $2`

const lockCommentQuery = commentByIDQuery + `
FOR UPDATE OF c`

const threadRootsQuery = `SELECT` + commentColumns + commentJoins + `
WHERE c.issue_id = $1
  AND c.parent_comment_id IS NULL
  AND ($2::boolean IS NOT TRUE
       OR (c.created_at, c.id) > ($3::timestamptz, $4::uuid))
ORDER BY c.created_at, c.id
LIMIT $5`

const repliesQuery = `SELECT` + commentColumns + commentJoins + `
WHERE c.parent_comment_id = ANY($1::uuid[])
ORDER BY c.created_at, c.id`

const mentionsQuery = `
SELECT m.comment_id,
       m.kind,
       coalesce(m.account_id::text, ''),
       coalesce(m.team_id::text, ''),
       m.mentioned_name,
       m.visible
FROM workspace_issue_comment_mentions m
WHERE m.comment_id = ANY($1::uuid[])
ORDER BY m.created_at, m.mentioned_name`

const reactionsQuery = `
SELECT r.comment_id, r.reaction, r.account_id
FROM workspace_issue_comment_reactions r
WHERE r.comment_id = ANY($1::uuid[])
ORDER BY r.reaction, r.created_at`

const recordMentionQuery = `
INSERT INTO workspace_issue_comment_mentions
    (comment_id, workspace_id, kind, account_id, team_id, mentioned_name, visible)
SELECT c.id, c.workspace_id, $2, nullif($3, '')::uuid, nullif($4, '')::uuid, $5, $6
FROM workspace_issue_comments c
WHERE c.id = $1
ON CONFLICT DO NOTHING`

const editCommentQuery = `
UPDATE workspace_issue_comments
SET body = $2, edited_at = $3, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL`

const tombstoneCommentQuery = `
UPDATE workspace_issue_comments
SET body = '', edited_at = NULL, deleted_at = $2, updated_at = $2
WHERE id = $1 AND deleted_at IS NULL`

const reactQuery = `
INSERT INTO workspace_issue_comment_reactions (comment_id, account_id, reaction)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING`

const unreactQuery = `
DELETE FROM workspace_issue_comment_reactions
WHERE comment_id = $1 AND account_id = $2 AND reaction = $3`

const visibleAudienceQuery = `
SELECT m.account_id,
       a.display_name,
       (m.role = 'admin' OR t.visibility = 'public' OR tm.account_id IS NOT NULL)
FROM workspace_memberships m
JOIN accounts a ON a.id = m.account_id AND coalesce(a.display_name, '') <> ''
JOIN workspace_teams t ON t.id = $2::uuid AND t.workspace_id = m.workspace_id
LEFT JOIN workspace_team_members tm ON tm.team_id = t.id AND tm.account_id = m.account_id
WHERE m.workspace_id = $1
  AND m.account_id = ANY($3::uuid[])`

type scanner interface {
	Scan(dest ...any) error
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func identifiers(ids []uuid.UUID) []string {
	raw := make([]string, 0, len(ids))
	for _, id := range ids {
		raw = append(raw, id.String())
	}

	return raw
}

func scanComment(row scanner) (entity.IssueComment, error) {
	var (
		comment              entity.IssueComment
		id, workspace, issue string
		parent, author, kind string
		edited, deleted      sql.NullTime
	)

	if err := row.Scan(
		&id, &workspace, &issue, &parent, &author, &comment.AuthorName, &kind,
		&comment.Body, &edited, &deleted, &comment.CreatedAt, &comment.UpdatedAt,
	); err != nil {
		return entity.IssueComment{}, err
	}

	comment.AuthorKind = entity.AccountKind(kind)

	if edited.Valid {
		comment.EditedAt = &edited.Time
	}

	if deleted.Valid {
		comment.DeletedAt = &deleted.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueComment{}, fmt.Errorf("parse comment id: %w", err)
	}

	comment.ID = parsed

	if comment.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IssueComment{}, fmt.Errorf("parse comment workspace id: %w", err)
	}

	if comment.IssueID, err = uuid.Parse(issue); err != nil {
		return entity.IssueComment{}, fmt.Errorf("parse comment issue id: %w", err)
	}

	if parent != "" {
		if comment.ParentCommentID, err = uuid.Parse(parent); err != nil {
			return entity.IssueComment{}, fmt.Errorf("parse comment parent id: %w", err)
		}
	}

	if author != "" {
		if comment.AuthorAccountID, err = uuid.Parse(author); err != nil {
			return entity.IssueComment{}, fmt.Errorf("parse comment author id: %w", err)
		}
	}

	return comment, nil
}

func (r *issueCommentRepository) query(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.IssueComment, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read comments: %w", err)
	}

	defer func() { _ = rows.Close() }()

	comments := make([]entity.IssueComment, 0)

	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}

	return comments, nil
}

func (r *issueCommentRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.IssueComment, error) {
	comment, err := scanComment(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueComment{}, entity.ErrIssueCommentNotFound
		}

		return entity.IssueComment{}, fmt.Errorf("find comment: %w", err)
	}

	return comment, nil
}

func (r *issueCommentRepository) Create(
	ctx context.Context,
	comment entity.IssueComment,
) (entity.IssueComment, error) {
	created, err := scanComment(r.db.Querier(ctx).QueryRowContext(
		ctx, createCommentQuery,
		comment.WorkspaceID.String(),
		comment.IssueID.String(),
		text(comment.ParentCommentID),
		comment.AuthorAccountID.String(),
		comment.Body,
	))
	if err != nil {
		return entity.IssueComment{}, fmt.Errorf("create comment: %w", err)
	}

	return created, nil
}

func (r *issueCommentRepository) GetByID(
	ctx context.Context,
	workspaceID, commentID uuid.UUID,
) (entity.IssueComment, error) {
	return r.hydrated(ctx, commentByIDQuery, commentID, workspaceID)
}

func (r *issueCommentRepository) LockByID(
	ctx context.Context,
	workspaceID, commentID uuid.UUID,
) (entity.IssueComment, error) {
	return r.hydrated(ctx, lockCommentQuery, commentID, workspaceID)
}

func (r *issueCommentRepository) hydrated(
	ctx context.Context,
	query string,
	commentID, workspaceID uuid.UUID,
) (entity.IssueComment, error) {
	comment, err := r.find(ctx, query, commentID.String(), workspaceID.String())
	if err != nil {
		return entity.IssueComment{}, err
	}

	found := []entity.IssueComment{comment}
	if err := r.hydrate(ctx, found); err != nil {
		return entity.IssueComment{}, err
	}

	return found[0], nil
}

func (r *issueCommentRepository) ListThread(
	ctx context.Context,
	issueID uuid.UUID,
	page entity.CommentPage,
) ([]entity.IssueComment, error) {
	cursorCreatedAt := time.Time{}
	cursorID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorCreatedAt = page.Cursor.CreatedAt
		cursorID = page.Cursor.CommentID.String()
	}

	roots, err := r.query(
		ctx, threadRootsQuery,
		issueID.String(), page.Cursor != nil, cursorCreatedAt, cursorID, page.Limit,
	)
	if err != nil {
		return nil, err
	}

	if len(roots) == 0 {
		return roots, nil
	}

	rootIDs := make([]uuid.UUID, 0, len(roots))
	for _, root := range roots {
		rootIDs = append(rootIDs, root.ID)
	}

	replies, err := r.query(ctx, repliesQuery, identifiers(rootIDs))
	if err != nil {
		return nil, err
	}

	thread := make([]entity.IssueComment, 0, len(roots)+len(replies))
	thread = append(thread, roots...)
	thread = append(thread, replies...)

	if err := r.hydrate(ctx, thread); err != nil {
		return nil, err
	}

	at := make(map[uuid.UUID]int, len(roots))
	for i := range roots {
		at[thread[i].ID] = i
	}

	for _, reply := range thread[len(roots):] {
		index, ok := at[reply.ParentCommentID]
		if !ok {
			continue
		}

		thread[index].Replies = append(thread[index].Replies, reply)
	}

	return thread[:len(roots)], nil
}

func (r *issueCommentRepository) hydrate(
	ctx context.Context,
	comments []entity.IssueComment,
) error {
	if len(comments) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(comments))
	at := make(map[uuid.UUID]int, len(comments))

	for i, comment := range comments {
		ids = append(ids, comment.ID)
		at[comment.ID] = i
	}

	raw := identifiers(ids)

	mentions, err := r.db.Querier(ctx).QueryContext(ctx, mentionsQuery, raw)
	if err != nil {
		return fmt.Errorf("read comment mentions: %w", err)
	}

	defer func() { _ = mentions.Close() }()

	for mentions.Next() {
		var (
			mention       entity.CommentMention
			comment, kind string
			account, team string
		)

		if err := mentions.Scan(
			&comment, &kind, &account, &team, &mention.Name, &mention.Visible,
		); err != nil {
			return fmt.Errorf("scan comment mention: %w", err)
		}

		mention.Kind = entity.MentionKind(kind)

		if account != "" {
			if mention.AccountID, err = uuid.Parse(account); err != nil {
				return fmt.Errorf("parse mentioned account id: %w", err)
			}
		}

		if team != "" {
			if mention.TeamID, err = uuid.Parse(team); err != nil {
				return fmt.Errorf("parse mentioned team id: %w", err)
			}
		}

		commentID, err := uuid.Parse(comment)
		if err != nil {
			return fmt.Errorf("parse mention comment id: %w", err)
		}

		if index, ok := at[commentID]; ok {
			comments[index].Mentions = append(comments[index].Mentions, mention)
		}
	}

	if err := mentions.Err(); err != nil {
		return fmt.Errorf("iterate comment mentions: %w", err)
	}

	return r.hydrateReactions(ctx, comments, at, raw)
}

func (r *issueCommentRepository) hydrateReactions(
	ctx context.Context,
	comments []entity.IssueComment,
	at map[uuid.UUID]int,
	ids []string,
) error {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, reactionsQuery, ids)
	if err != nil {
		return fmt.Errorf("read comment reactions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var comment, reaction, account string

		if err := rows.Scan(&comment, &reaction, &account); err != nil {
			return fmt.Errorf("scan comment reaction: %w", err)
		}

		commentID, err := uuid.Parse(comment)
		if err != nil {
			return fmt.Errorf("parse reaction comment id: %w", err)
		}

		accountID, err := uuid.Parse(account)
		if err != nil {
			return fmt.Errorf("parse reaction account id: %w", err)
		}

		index, ok := at[commentID]
		if !ok {
			continue
		}

		comments[index].Reactions = tally(
			comments[index].Reactions, entity.CommentReaction(reaction), accountID,
		)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate comment reactions: %w", err)
	}

	return nil
}

func tally(
	tallies []entity.CommentReactionTally,
	reaction entity.CommentReaction,
	accountID uuid.UUID,
) []entity.CommentReactionTally {
	for i := range tallies {
		if tallies[i].Reaction == reaction {
			tallies[i].Accounts = append(tallies[i].Accounts, accountID)

			return tallies
		}
	}

	return append(tallies, entity.CommentReactionTally{
		Reaction: reaction,
		Accounts: []uuid.UUID{accountID},
	})
}

func (r *issueCommentRepository) Edit(
	ctx context.Context,
	commentID uuid.UUID,
	body string,
	at time.Time,
) error {
	return r.write(ctx, "edit comment", editCommentQuery, commentID.String(), body, at)
}

func (r *issueCommentRepository) Tombstone(
	ctx context.Context,
	commentID uuid.UUID,
	at time.Time,
) error {
	return r.write(ctx, "delete comment", tombstoneCommentQuery, commentID.String(), at)
}

func (r *issueCommentRepository) write(
	ctx context.Context,
	action, query string,
	args ...any,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	if affected == 0 {
		return entity.ErrIssueCommentNotFound
	}

	return nil
}

func (r *issueCommentRepository) RecordMentions(
	ctx context.Context,
	commentID uuid.UUID,
	mentions []entity.CommentMention,
) error {
	querier := r.db.Querier(ctx)

	for _, mention := range mentions {
		if _, err := querier.ExecContext(
			ctx, recordMentionQuery,
			commentID.String(),
			string(mention.Kind),
			text(mention.AccountID),
			text(mention.TeamID),
			mention.Name,
			mention.Visible,
		); err != nil {
			return fmt.Errorf("record comment mention: %w", err)
		}
	}

	return nil
}

func (r *issueCommentRepository) Audience(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	accountIDs []uuid.UUID,
) ([]repository.CommentAudience, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, visibleAudienceQuery,
		workspaceID.String(), teamID.String(), identifiers(accountIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("read comment audience: %w", err)
	}

	defer func() { _ = rows.Close() }()

	audience := make([]repository.CommentAudience, 0, len(accountIDs))

	for rows.Next() {
		var (
			member  repository.CommentAudience
			account string
		)

		if err := rows.Scan(&account, &member.Name, &member.Visible); err != nil {
			return nil, fmt.Errorf("scan comment audience: %w", err)
		}

		if member.AccountID, err = uuid.Parse(account); err != nil {
			return nil, fmt.Errorf("parse audience account id: %w", err)
		}

		audience = append(audience, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment audience: %w", err)
	}

	return audience, nil
}

func (r *issueCommentRepository) React(
	ctx context.Context,
	commentID, accountID uuid.UUID,
	reaction entity.CommentReaction,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, reactQuery, commentID.String(), accountID.String(), string(reaction),
	); err != nil {
		return fmt.Errorf("react to comment: %w", err)
	}

	return nil
}

func (r *issueCommentRepository) Unreact(
	ctx context.Context,
	commentID, accountID uuid.UUID,
	reaction entity.CommentReaction,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, unreactQuery, commentID.String(), accountID.String(), string(reaction),
	); err != nil {
		return fmt.Errorf("remove comment reaction: %w", err)
	}

	return nil
}
