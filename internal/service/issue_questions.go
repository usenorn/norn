package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_questions.go -destination=issuequestion/mock_issue_questions.go -package=issuequestion -mock_names=IssueQuestions=MockIssueQuestions

type AskQuestionInput struct {
	Question      string
	Default       string
	Wait          time.Duration
	Kind          entity.QuestionKind
	Blocking      bool
	Options       []string
	AllowFreeText bool
	Context       entity.QuestionContext
	ExecutionID   string
	Ref           string
}

type AnswerQuestionInput struct {
	Answer string
}

type IssueQuestions interface {
	Ask(ctx context.Context, workspaceID, issueID uuid.UUID, input AskQuestionInput) (entity.IssueQuestion, error)
	List(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueQuestion, error)
	ListByExecution(
		ctx context.Context, workspaceID uuid.UUID, executionID string,
	) ([]entity.IssueQuestion, error)
	Answer(ctx context.Context, workspaceID, issueID, questionID uuid.UUID, input AnswerQuestionInput) (entity.IssueQuestion, error)
	Dismiss(ctx context.Context, workspaceID, issueID, questionID uuid.UUID) (entity.IssueQuestion, error)

	Asked(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	SweepExpired(ctx context.Context) error
}
