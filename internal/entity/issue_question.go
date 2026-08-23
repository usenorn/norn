package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	QuestionTextMaxLen      = 1000
	QuestionAnswerMaxLen    = 2000
	QuestionRefMaxLen       = 64
	QuestionOptionMaxLen    = 200
	QuestionOptionsMax      = 8
	QuestionContextFilesMax = 20
	QuestionWaitMin         = time.Minute
	QuestionWaitMax         = 7 * 24 * time.Hour
	QuestionWaitDefault     = 24 * time.Hour
	QuestionParkedWait      = 72 * time.Hour
)

var (
	ErrIssueQuestionNotFound     = errors.New("question not found")
	ErrIssueQuestionAnswered     = errors.New("question is already answered")
	ErrIssueQuestionNotAgent     = errors.New("only an agent asks a question this way")
	ErrIssueQuestionSettled      = errors.New("nobody is waiting on this question any more")
	ErrIssueQuestionRecorded     = errors.New("this run has already asked that question")
	ErrIssueQuestionUnanswerable = errors.New("that is not one of the answers this question takes")
)

type QuestionKind string

const (
	QuestionDecision      QuestionKind = "decision"
	QuestionClarification QuestionKind = "clarification"
	QuestionApproval      QuestionKind = "approval"
)

func QuestionKinds() []QuestionKind {
	return []QuestionKind{QuestionDecision, QuestionClarification, QuestionApproval}
}

func (k QuestionKind) Valid() bool {
	return slices.Contains(QuestionKinds(), k)
}

type QuestionState string

const (
	QuestionAsked     QuestionState = "asked"
	QuestionAnswered  QuestionState = "answered"
	QuestionDismissed QuestionState = "dismissed"
	QuestionExpired   QuestionState = "expired"
)

func QuestionStates() []QuestionState {
	return []QuestionState{QuestionAsked, QuestionAnswered, QuestionDismissed, QuestionExpired}
}

func (s QuestionState) Valid() bool {
	return slices.Contains(QuestionStates(), s)
}

func (s QuestionState) Settled() bool {
	return s != QuestionAsked
}

type QuestionContext struct {
	Preview   string   `json:"preview,omitempty"`
	Files     []string `json:"files,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type IssueQuestion struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	IssueID          uuid.UUID
	ExecutionID      string
	Ref              string
	Kind             QuestionKind
	Blocking         bool
	Options          []string
	AllowFreeText    bool
	Context          QuestionContext
	State            QuestionState
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
	SettledBy        uuid.UUID
	SettledByName    string
	SettledAt        *time.Time
	CreatedAt        time.Time
}

func (q IssueQuestion) Answered() bool {
	return q.AnsweredAt != nil
}

func (q IssueQuestion) Settled() bool {
	return q.State.Settled()
}

// Expired reads the clock as well as the stored state, because the sweep that settles a lapsed
// question runs on a schedule and a person reading the issue in between should not be told the
// agent is still waiting.
func (q IssueQuestion) Expired(now time.Time) bool {
	if q.State == QuestionExpired {
		return true
	}

	return !q.Settled() && !now.Before(q.Deadline)
}

func (q IssueQuestion) Standing() string {
	if q.Answered() {
		return q.Answer
	}

	return q.DefaultAnswer
}

// Parked says the run that asked this stopped to wait for it, so settling the question one way or
// another is the only thing that gets that run moving again.
func (q IssueQuestion) Parked() bool {
	return q.Blocking && q.ExecutionID != "" && !q.Settled()
}

func (q IssueQuestion) Acceptable(answer string) bool {
	trimmed := strings.TrimSpace(answer)

	if len(q.Options) == 0 || q.AllowFreeText {
		return true
	}

	return slices.Contains(q.Options, trimmed)
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

func ValidateQuestionKind(field string, kind QuestionKind) FieldError {
	if !kind.Valid() {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ValidateQuestionRef(field, ref string) FieldError {
	trimmed := strings.TrimSpace(ref)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > QuestionRefMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateQuestionOptions(field string, options []string) FieldError {
	if len(options) > QuestionOptionsMax {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	for _, option := range options {
		trimmed := strings.TrimSpace(option)

		if trimmed == "" {
			return FieldError{Field: field, Code: ValidationCodeRequired}
		}

		if utf8.RuneCountInString(trimmed) > QuestionOptionMaxLen {
			return FieldError{Field: field, Code: ValidationCodeTooLong}
		}
	}

	return FieldError{}
}

// ValidateQuestionReachable refuses a question nobody could answer: no options to pick from and no
// free text to type means the agent has stopped for something a person cannot give it.
func ValidateQuestionReachable(field string, options []string, allowFreeText bool) FieldError {
	if len(options) == 0 && !allowFreeText {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	return FieldError{}
}

func ValidateQuestionContext(field string, held QuestionContext) FieldError {
	if len(held.Files) > QuestionContextFilesMax ||
		len(held.Artifacts) > QuestionContextFilesMax {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}
