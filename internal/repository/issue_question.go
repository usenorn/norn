package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_question.go -destination=issuequestion/mock_issue_question.go -package=issuequestion -mock_names=IssueQuestion=MockIssueQuestion

type QuestionAnswer struct {
	QuestionID uuid.UUID
	Answer     string
	AccountID  uuid.UUID
	AnsweredAt time.Time
}

type IssueQuestion interface {
	Ask(ctx context.Context, question entity.IssueQuestion) (entity.IssueQuestion, error)
	GetByID(ctx context.Context, workspaceID, questionID uuid.UUID) (entity.IssueQuestion, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueQuestion, error)
	Answer(ctx context.Context, workspaceID uuid.UUID, answer QuestionAnswer) (entity.IssueQuestion, error)
}
