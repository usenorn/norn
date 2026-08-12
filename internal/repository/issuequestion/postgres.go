package issuequestion

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

const questionColumns = `
    q.id, q.workspace_id, q.issue_id, q.question, q.default_answer, q.deadline, q.answer,
    coalesce(q.asked_by_account_id::text, ''), coalesce(asked.display_name, ''), q.actor_kind,
    coalesce(q.answered_by_account_id::text, ''), coalesce(answered.display_name, ''),
    q.answered_at, q.created_at
FROM workspace_issue_questions q
LEFT JOIN accounts asked ON asked.id = q.asked_by_account_id
LEFT JOIN accounts answered ON answered.id = q.answered_by_account_id`

const insertQuestionQuery = `
INSERT INTO workspace_issue_questions (
    id, workspace_id, issue_id, question, default_answer, deadline,
    asked_by_account_id, actor_kind, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

const questionByIDQuery = `
SELECT` + questionColumns + `
WHERE q.workspace_id = $1 AND q.id = $2`

const questionsByIssueQuery = `
SELECT` + questionColumns + `
WHERE q.workspace_id = $1 AND q.issue_id = $2
ORDER BY q.created_at, q.id`

const answerQuestionQuery = `
UPDATE workspace_issue_questions
SET answer = $3, answered_by_account_id = $4, answered_at = $5
WHERE workspace_id = $1 AND id = $2 AND answered_at IS NULL
RETURNING id`

type questionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueQuestion {
	return &questionRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row scanner) (entity.IssueQuestion, error) {
	var (
		question    entity.IssueQuestion
		id          string
		workspaceID string
		issueID     string
		askedBy     string
		actorKind   string
		answeredBy  string
		answeredAt  sql.NullTime
	)

	if err := row.Scan(
		&id,
		&workspaceID,
		&issueID,
		&question.Question,
		&question.DefaultAnswer,
		&question.Deadline,
		&question.Answer,
		&askedBy,
		&question.AskedByName,
		&actorKind,
		&answeredBy,
		&question.AnsweredByName,
		&answeredAt,
		&question.CreatedAt,
	); err != nil {
		return entity.IssueQuestion{}, err
	}

	question.ActorKind = entity.ActorKind(actorKind)

	if answeredAt.Valid {
		question.AnsweredAt = &answeredAt.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question id: %w", err)
	}

	question.ID = parsed

	if question.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question workspace id: %w", err)
	}

	if question.IssueID, err = uuid.Parse(issueID); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question issue id: %w", err)
	}

	if question.AskedByAccountID, err = optionalID(askedBy); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question asker id: %w", err)
	}

	if question.AnsweredBy, err = optionalID(answeredBy); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question answerer id: %w", err)
	}

	return question, nil
}

func optionalID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value)
}

func idOrNil(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func (r *questionRepository) Ask(
	ctx context.Context,
	question entity.IssueQuestion,
) (entity.IssueQuestion, error) {
	if question.ID == uuid.Nil {
		question.ID = uuid.New()
	}

	if question.CreatedAt.IsZero() {
		question.CreatedAt = time.Now().UTC()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertQuestionQuery,
		question.ID.String(),
		question.WorkspaceID.String(),
		question.IssueID.String(),
		question.Question,
		question.DefaultAnswer,
		question.Deadline,
		idOrNil(question.AskedByAccountID),
		string(question.ActorKind),
		question.CreatedAt,
	); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("insert question: %w", err)
	}

	return r.GetByID(ctx, question.WorkspaceID, question.ID)
}

func (r *questionRepository) GetByID(
	ctx context.Context,
	workspaceID, questionID uuid.UUID,
) (entity.IssueQuestion, error) {
	question, err := scanQuestion(r.db.Querier(ctx).QueryRowContext(
		ctx, questionByIDQuery, workspaceID.String(), questionID.String(),
	))

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
	case err != nil:
		return entity.IssueQuestion{}, fmt.Errorf("read question: %w", err)
	default:
		return question, nil
	}
}

func (r *questionRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueQuestion, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, questionsByIssueQuery, workspaceID.String(), issueID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("read questions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	questions := make([]entity.IssueQuestion, 0)

	for rows.Next() {
		question, err := scanQuestion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}

		questions = append(questions, question)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read questions: %w", err)
	}

	return questions, nil
}

func (r *questionRepository) Answer(
	ctx context.Context,
	workspaceID uuid.UUID,
	answer repository.QuestionAnswer,
) (entity.IssueQuestion, error) {
	var answered string

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		answerQuestionQuery,
		workspaceID.String(),
		answer.QuestionID.String(),
		answer.Answer,
		idOrNil(answer.AccountID),
		answer.AnsweredAt,
	).Scan(&answered)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return entity.IssueQuestion{}, entity.ErrIssueQuestionAnswered
	case err != nil:
		return entity.IssueQuestion{}, fmt.Errorf("answer question: %w", err)
	}

	return r.GetByID(ctx, workspaceID, answer.QuestionID)
}
