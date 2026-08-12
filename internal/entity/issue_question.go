package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	QuestionTextMaxLen   = 1000
	QuestionAnswerMaxLen = 2000
	QuestionWaitMin      = time.Minute
	QuestionWaitMax      = 7 * 24 * time.Hour
	QuestionWaitDefault  = 24 * time.Hour
)

var (
	ErrIssueQuestionNotFound = errors.New("question not found")
	ErrIssueQuestionAnswered = errors.New("question is already answered")
	ErrIssueQuestionNotAgent = errors.New("only an agent asks a question this way")
)

type IssueQuestion struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	IssueID          uuid.UUID
	Question         string
	DefaultAnswer    string
	Deadline         time.Time
	Answer           string
	AskedByAccountID uuid.UUID
	AskedByName      string
	ActorKind        ActorKind
	AnsweredBy       uuid.UUID
	AnsweredByName   string
	AnsweredAt       *time.Time
	CreatedAt        time.Time
}

func (q IssueQuestion) Answered() bool {
	return q.AnsweredAt != nil
}

func (q IssueQuestion) Expired(now time.Time) bool {
	return !q.Answered() && !now.Before(q.Deadline)
}

func (q IssueQuestion) Standing() string {
	if q.Answered() {
		return q.Answer
	}

	return q.DefaultAnswer
}

func UnansweredQuestions(questions []IssueQuestion) []IssueQuestion {
	unanswered := make([]IssueQuestion, 0, len(questions))

	for _, question := range questions {
		if !question.Answered() {
			unanswered = append(unanswered, question)
		}
	}

	return unanswered
}

func QuestionSummaries(questions []IssueQuestion) string {
	summaries := make([]string, 0, len(questions))

	for _, question := range questions {
		summaries = append(summaries, question.Question)
	}

	return strings.Join(summaries, "; ")
}

func ValidateQuestionText(field, question string) FieldError {
	trimmed := strings.TrimSpace(question)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > QuestionTextMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateQuestionAnswer(field, answer string) FieldError {
	trimmed := strings.TrimSpace(answer)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > QuestionAnswerMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateQuestionWait(field string, wait time.Duration) FieldError {
	if wait < QuestionWaitMin || wait > QuestionWaitMax {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}
