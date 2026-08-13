package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_questions.go -destination=issuequestion/mock_issue_questions.go -package=issuequestion -mock_names=IssueQuestions=MockIssueQuestions

type AskQuestionInput struct {
	Question string
	Default  string
	Options  []string
	Wait     time.Duration
}

type AnswerQuestionInput struct {
	Answer string
}

type IssueQuestions interface {
	Ask(ctx context.Context, workspaceID, issueID uuid.UUID, input AskQuestionInput) (entity.IssueQuestion, error)
	List(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueQuestion, error)
	Answer(ctx context.Context, workspaceID, issueID, questionID uuid.UUID, input AnswerQuestionInput) (entity.IssueQuestion, error)
}
