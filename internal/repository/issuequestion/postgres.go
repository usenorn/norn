package issuequestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const questionColumns = `
    q.id, q.workspace_id, q.issue_id, coalesce(q.execution_id, ''), q.runner_ref, q.kind,
    q.blocking, q.options, q.allow_free_text, q.context, q.state,
    q.question, q.default_answer, q.deadline, q.answer,
    coalesce(q.asked_by_account_id::text, ''), coalesce(asked.display_name, ''), q.actor_kind,
    coalesce(q.answered_by_account_id::text, ''), coalesce(answered.display_name, ''),
    coalesce(q.settled_by_account_id::text, ''), coalesce(settled.display_name, ''),
    q.answered_at, q.settled_at, q.created_at
FROM workspace_issue_questions q
LEFT JOIN accounts asked ON asked.id = q.asked_by_account_id
LEFT JOIN accounts answered ON answered.id = q.answered_by_account_id
LEFT JOIN accounts settled ON settled.id = q.settled_by_account_id`

const insertQuestionQuery = `
INSERT INTO workspace_issue_questions (
    id, workspace_id, issue_id, execution_id, runner_ref, kind, blocking, options,
    allow_free_text, context, question, default_answer, deadline,
    asked_by_account_id, actor_kind, created_at
)
VALUES ($1, $2, $3, nullif($4, ''), $5, $6, $7, $8::jsonb, $9, $10::jsonb, $11, $12, $13,
        nullif($14, '')::uuid, $15, $16)
ON CONFLICT (execution_id, runner_ref) WHERE runner_ref <> '' DO NOTHING
RETURNING id`

const questionByIDQuery = `
SELECT` + questionColumns + `
WHERE q.workspace_id = $1 AND q.id = $2`

const questionsByIssueQuery = `
SELECT` + questionColumns + `
WHERE q.workspace_id = $1 AND q.issue_id = $2
ORDER BY q.created_at, q.id`

const questionsByExecutionQuery = `
SELECT` + questionColumns + `
WHERE q.workspace_id = $1 AND q.execution_id = $2
ORDER BY q.created_at, q.id`

const lapsedQuestionsQuery = `
SELECT` + questionColumns + `
WHERE q.state = 'asked' AND q.deadline < $1
ORDER BY q.deadline, q.id
LIMIT $2`

const answerQuestionQuery = `
UPDATE workspace_issue_questions
SET answer = $3, answered_by_account_id = nullif($4, '')::uuid, answered_at = $5,
    state = 'answered', settled_at = $5, settled_by_account_id = nullif($4, '')::uuid
WHERE workspace_id = $1 AND id = $2 AND state = 'asked'
RETURNING id`

const settleQuestionQuery = `
UPDATE workspace_issue_questions
SET state = $3, settled_at = $4, settled_by_account_id = nullif($5, '')::uuid
WHERE workspace_id = $1 AND id = $2 AND state = 'asked'
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
		kind        string
		state       string
		options     []byte
		holds       []byte
		askedBy     string
		actorKind   string
		answeredBy  string
		settledBy   string
		answeredAt  sql.NullTime
		settledAt   sql.NullTime
	)

	if err := row.Scan(
		&id,
		&workspaceID,
		&issueID,
		&question.ExecutionID,
		&question.Ref,
		&kind,
		&question.Blocking,
		&options,
		&question.AllowFreeText,
		&holds,
		&state,
		&question.Question,
		&question.DefaultAnswer,
		&question.Deadline,
		&question.Answer,
		&askedBy,
		&question.AskedByName,
		&actorKind,
		&answeredBy,
		&question.AnsweredByName,
		&settledBy,
		&question.SettledByName,
		&answeredAt,
		&settledAt,
		&question.CreatedAt,
	); err != nil {
		return entity.IssueQuestion{}, err
	}

	question.Kind = entity.QuestionKind(kind)
	question.State = entity.QuestionState(state)
	question.ActorKind = entity.ActorKind(actorKind)

	if err := json.Unmarshal(options, &question.Options); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("read question options: %w", err)
	}

	if err := json.Unmarshal(holds, &question.Context); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("read question context: %w", err)
	}

	if answeredAt.Valid {
		question.AnsweredAt = &answeredAt.Time
	}

	if settledAt.Valid {
		question.SettledAt = &settledAt.Time
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

	if question.SettledBy, err = optionalID(settledBy); err != nil {
		return entity.IssueQuestion{}, fmt.Errorf("parse question settler id: %w", err)
	}

	return question, nil
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

func encode(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode question detail: %w", err)
	}

	return body, nil
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

	if question.Options == nil {
		question.Options = []string{}
	}

	options, err := encode(question.Options)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	holds, err := encode(question.Context)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	var recorded string

	err = r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertQuestionQuery,
		question.ID.String(),
		question.WorkspaceID.String(),
		question.IssueID.String(),
		question.ExecutionID,
		question.Ref,
		string(question.Kind),
		question.Blocking,
		options,
		question.AllowFreeText,
		holds,
		question.Question,
		question.DefaultAnswer,
		question.Deadline,
		idOrEmpty(question.AskedByAccountID),
		string(question.ActorKind),
		question.CreatedAt,
	).Scan(&recorded)

	// A ref this run has already used is the question it already asked: the machine replayed the
	// message after a reconnect, and the caller drops it rather than putting a second copy of the
	// same question in front of a person.
	if errors.Is(err, sql.ErrNoRows) {
		return entity.IssueQuestion{}, entity.ErrIssueQuestionRecorded
	}

	if err != nil {
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
	return r.list(ctx, questionsByIssueQuery, workspaceID.String(), issueID.String())
}

func (r *questionRepository) ListByExecution(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]entity.IssueQuestion, error) {
	return r.list(ctx, questionsByExecutionQuery, workspaceID.String(), executionID)
}

func (r *questionRepository) Lapsed(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]entity.IssueQuestion, error) {
	return r.list(ctx, lapsedQuestionsQuery, now, limit)
}

func (r *questionRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.IssueQuestion, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
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
		idOrEmpty(answer.AccountID),
		answer.AnsweredAt,
	).Scan(&answered)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return entity.IssueQuestion{}, r.whyNot(ctx, workspaceID, answer.QuestionID)
	case err != nil:
		return entity.IssueQuestion{}, fmt.Errorf("answer question: %w", err)
	}

	return r.GetByID(ctx, workspaceID, answer.QuestionID)
}

func (r *questionRepository) Settle(
	ctx context.Context,
	workspaceID uuid.UUID,
	settlement repository.QuestionSettlement,
) (entity.IssueQuestion, error) {
	var settled string

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		settleQuestionQuery,
		workspaceID.String(),
		settlement.QuestionID.String(),
		string(settlement.State),
		settlement.SettledAt,
		idOrEmpty(settlement.AccountID),
	).Scan(&settled)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return entity.IssueQuestion{}, r.whyNot(ctx, workspaceID, settlement.QuestionID)
	case err != nil:
		return entity.IssueQuestion{}, fmt.Errorf("settle question: %w", err)
	}

	return r.GetByID(ctx, workspaceID, settlement.QuestionID)
}

// whyNot separates a question that is not there from one somebody already dealt with, because the
// conditional update declines both the same way and a caller told "already answered" about a
// question id that never existed cannot tell what went wrong.
func (r *questionRepository) whyNot(
	ctx context.Context,
	workspaceID, questionID uuid.UUID,
) error {
	held, err := r.GetByID(ctx, workspaceID, questionID)
	if err != nil {
		return err
	}

	if held.Answered() {
		return entity.ErrIssueQuestionAnswered
	}

	return entity.ErrIssueQuestionSettled
}
