package executionreview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const commentColumns = `
    c.id, c.execution_id, c.workspace_id, coalesce(c.review_id::text, ''),
    coalesce(c.parent_id::text, ''), c.repository, c.path, c.side, c.line, c.head_sha, c.hunk,
    c.body, coalesce(c.author_account_id::text, ''), coalesce(author.display_name, ''),
    c.created_at, c.edited_at, c.resolved_at, coalesce(c.resolved_by::text, ''),
    coalesce(resolver.display_name, '')`

const commentNames = `
LEFT JOIN accounts author ON author.id = c.author_account_id
LEFT JOIN accounts resolver ON resolver.id = c.resolved_by`

const insertCommentQuery = `
WITH inserted AS (
    INSERT INTO workspace_execution_review_comments
        (execution_id, workspace_id, review_id, parent_id, repository, path, side, line,
         head_sha, hunk, body, author_account_id, created_at)
    VALUES ($1, $2, nullif($3, '')::uuid, nullif($4, '')::uuid, $5, $6, $7, $8, $9, $10, $11,
            nullif($12, '')::uuid, $13)
    RETURNING *
)
SELECT` + commentColumns + `
FROM inserted c` + commentNames

const commentByIDQuery = `
SELECT` + commentColumns + `
FROM workspace_execution_review_comments c` + commentNames + `
WHERE c.execution_id = $1 AND c.id = $2`

const commentsByExecutionQuery = `
SELECT` + commentColumns + `
FROM workspace_execution_review_comments c` + commentNames + `
WHERE c.execution_id = $1
ORDER BY c.created_at, c.id`

const countCommentsQuery = `
SELECT count(*) FROM workspace_execution_review_comments WHERE execution_id = $1`

const editCommentQuery = `
WITH edited AS (
    UPDATE workspace_execution_review_comments
    SET body = $3, edited_at = $4
    WHERE execution_id = $1 AND id = $2
    RETURNING *
)
SELECT` + commentColumns + `
FROM edited c` + commentNames

const deleteCommentQuery = `
DELETE FROM workspace_execution_review_comments WHERE execution_id = $1 AND id = $2`

const resolveCommentQuery = `
WITH resolved AS (
    UPDATE workspace_execution_review_comments
    SET resolved_at = CASE WHEN $3 THEN $5::timestamptz END,
        resolved_by = CASE WHEN $3 THEN nullif($4, '')::uuid END
    WHERE execution_id = $1 AND id = $2
    RETURNING *
)
SELECT` + commentColumns + `
FROM resolved c` + commentNames

const insertReviewQuery = `
WITH inserted AS (
    INSERT INTO workspace_execution_reviews
        (execution_id, workspace_id, verdict, summary, heads, author_account_id, submitted_at)
    VALUES ($1, $2, $3, $4, $5::jsonb, nullif($6, '')::uuid, $7)
    RETURNING *
)
SELECT r.id, r.execution_id, r.workspace_id, r.verdict, r.summary, r.heads,
       coalesce(r.author_account_id::text, ''), coalesce(author.display_name, ''), r.submitted_at
FROM inserted r
LEFT JOIN accounts author ON author.id = r.author_account_id`

const attachPendingCommentsQuery = `
UPDATE workspace_execution_review_comments
SET review_id = $3
WHERE execution_id = $1 AND author_account_id = $2 AND review_id IS NULL`

const reviewsByExecutionQuery = `
SELECT r.id, r.execution_id, r.workspace_id, r.verdict, r.summary, r.heads,
       coalesce(r.author_account_id::text, ''), coalesce(author.display_name, ''), r.submitted_at
FROM workspace_execution_reviews r
LEFT JOIN accounts author ON author.id = r.author_account_id
WHERE r.execution_id = $1
ORDER BY r.submitted_at, r.id`

type reviewRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionReview {
	return &reviewRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanComment(row scanner) (entity.ExecutionReviewComment, error) {
	var (
		comment     entity.ExecutionReviewComment
		id          string
		workspaceID string
		reviewID    string
		parentID    string
		side        string
		authorID    string
		resolvedBy  string
		editedAt    sql.NullTime
		resolvedAt  sql.NullTime
	)

	if err := row.Scan(
		&id,
		&comment.ExecutionID,
		&workspaceID,
		&reviewID,
		&parentID,
		&comment.Anchor.Repository,
		&comment.Anchor.Path,
		&side,
		&comment.Anchor.Line,
		&comment.Anchor.HeadSHA,
		&comment.Anchor.Hunk,
		&comment.Body,
		&authorID,
		&comment.AuthorName,
		&comment.CreatedAt,
		&editedAt,
		&resolvedAt,
		&resolvedBy,
		&comment.ResolvedByName,
	); err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	comment.Anchor.Side = entity.ReviewSide(side)

	if editedAt.Valid {
		comment.EditedAt = &editedAt.Time
	}

	if resolvedAt.Valid {
		comment.ResolvedAt = &resolvedAt.Time
	}

	var err error

	if comment.ID, err = uuid.Parse(id); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment id: %w", err)
	}

	if comment.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment workspace id: %w", err)
	}

	if comment.ReviewID, err = optionalID(reviewID); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment review id: %w", err)
	}

	if comment.ParentID, err = optionalID(parentID); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment parent id: %w", err)
	}

	if comment.AuthorAccountID, err = optionalID(authorID); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment author id: %w", err)
	}

	if comment.ResolvedByID, err = optionalID(resolvedBy); err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("parse review comment resolver id: %w", err)
	}

	return comment, nil
}

func scanReview(row scanner) (entity.ExecutionReview, error) {
	var (
		review      entity.ExecutionReview
		id          string
		workspaceID string
		verdict     string
		heads       []byte
		authorID    string
	)

	if err := row.Scan(
		&id,
		&review.ExecutionID,
		&workspaceID,
		&verdict,
		&review.Summary,
		&heads,
		&authorID,
		&review.AuthorName,
		&review.SubmittedAt,
	); err != nil {
		return entity.ExecutionReview{}, err
	}

	review.Verdict = entity.ExecutionReviewVerdict(verdict)

	if err := json.Unmarshal(heads, &review.Heads); err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("read review heads: %w", err)
	}

	var err error

	if review.ID, err = uuid.Parse(id); err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("parse review id: %w", err)
	}

	if review.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("parse review workspace id: %w", err)
	}

	if review.AuthorAccountID, err = optionalID(authorID); err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("parse review author id: %w", err)
	}

	return review, nil
}

func optionalID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value)
}

func idOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *reviewRepository) AddComment(
	ctx context.Context,
	comment entity.ExecutionReviewComment,
) (entity.ExecutionReviewComment, error) {
	added, err := scanComment(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertCommentQuery,
		comment.ExecutionID,
		comment.WorkspaceID.String(),
		idOrEmpty(comment.ReviewID),
		idOrEmpty(comment.ParentID),
		comment.Anchor.Repository,
		comment.Anchor.Path,
		string(comment.Anchor.Side),
		comment.Anchor.Line,
		comment.Anchor.HeadSHA,
		comment.Anchor.Hunk,
		comment.Body,
		idOrEmpty(comment.AuthorAccountID),
		comment.CreatedAt,
	))
	if err != nil {
		return entity.ExecutionReviewComment{}, fmt.Errorf("add review comment: %w", err)
	}

	return added, nil
}

func (r *reviewRepository) GetComment(
	ctx context.Context,
	executionID string,
	commentID uuid.UUID,
) (entity.ExecutionReviewComment, error) {
	return r.one(ctx, "get review comment", commentByIDQuery, executionID, commentID.String())
}

func (r *reviewRepository) ListComments(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionReviewComment, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, commentsByExecutionQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list review comments: %w", err)
	}

	defer func() { _ = rows.Close() }()

	comments := make([]entity.ExecutionReviewComment, 0)

	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review comment: %w", err)
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review comments: %w", err)
	}

	return comments, nil
}

func (r *reviewRepository) CountComments(ctx context.Context, executionID string) (int, error) {
	var count int

	if err := r.db.Querier(ctx).QueryRowContext(ctx, countCommentsQuery, executionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count review comments: %w", err)
	}

	return count, nil
}

func (r *reviewRepository) EditComment(
	ctx context.Context,
	edit repository.ReviewCommentEdit,
) (entity.ExecutionReviewComment, error) {
	return r.one(ctx, "edit review comment", editCommentQuery,
		edit.ExecutionID, edit.CommentID.String(), edit.Body, edit.At,
	)
}

func (r *reviewRepository) DeleteComment(
	ctx context.Context,
	executionID string,
	commentID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteCommentQuery, executionID, commentID.String())
	if err != nil {
		return fmt.Errorf("delete review comment: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted review comments: %w", err)
	}

	if removed == 0 {
		return entity.ErrReviewCommentNotFound
	}

	return nil
}

func (r *reviewRepository) ResolveComment(
	ctx context.Context,
	resolution repository.ReviewResolution,
) (entity.ExecutionReviewComment, error) {
	return r.one(ctx, "resolve review comment", resolveCommentQuery,
		resolution.ExecutionID, resolution.CommentID.String(), resolution.Resolved,
		idOrEmpty(resolution.AccountID), resolution.At,
	)
}

func (r *reviewRepository) CreateReview(
	ctx context.Context,
	review entity.ExecutionReview,
) (entity.ExecutionReview, error) {
	heads, err := json.Marshal(review.Heads)
	if err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("encode review heads: %w", err)
	}

	created, err := scanReview(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertReviewQuery,
		review.ExecutionID,
		review.WorkspaceID.String(),
		string(review.Verdict),
		review.Summary,
		heads,
		idOrEmpty(review.AuthorAccountID),
		review.SubmittedAt,
	))
	if err != nil {
		return entity.ExecutionReview{}, fmt.Errorf("create review: %w", err)
	}

	return created, nil
}

func (r *reviewRepository) AttachPending(
	ctx context.Context,
	executionID string,
	authorID, reviewID uuid.UUID,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		attachPendingCommentsQuery,
		executionID,
		authorID.String(),
		reviewID.String(),
	); err != nil {
		return fmt.Errorf("attach pending review comments: %w", err)
	}

	return nil
}

func (r *reviewRepository) ListReviews(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionReview, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, reviewsByExecutionQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	defer func() { _ = rows.Close() }()

	reviews := make([]entity.ExecutionReview, 0)

	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}

		reviews = append(reviews, review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}

	return reviews, nil
}

func (r *reviewRepository) one(
	ctx context.Context,
	action string,
	query string,
	args ...any,
) (entity.ExecutionReviewComment, error) {
	comment, err := scanComment(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionReviewComment{}, entity.ErrReviewCommentNotFound
		}

		return entity.ExecutionReviewComment{}, fmt.Errorf("%s: %w", action, err)
	}

	return comment, nil
}
