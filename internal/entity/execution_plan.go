package entity

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	ExecutionPlanBodyMaxLen = 100000
	ExecutionPlanRefMaxLen  = 64
)

type ExecutionPlan struct {
	ID                      uuid.UUID
	ExecutionID             string
	WorkspaceID             uuid.UUID
	Revision                int
	Ref                     string
	Body                    string
	ProposedAt              time.Time
	ApprovedByAccountID     uuid.UUID
	ApprovedByName          string
	ApprovedAt              *time.Time
	RevisionFeedback        string
	RevisionRequestedByID   uuid.UUID
	RevisionRequestedByName string
	RevisionRequestedAt     *time.Time
}

func (p ExecutionPlan) Approved() bool {
	return p.ApprovedAt != nil
}

func (p ExecutionPlan) RevisionRequested() bool {
	return p.RevisionRequestedAt != nil
}

func (p ExecutionPlan) Undecided() bool {
	return !p.Approved() && !p.RevisionRequested()
}

func LatestPlan(plans []ExecutionPlan) (ExecutionPlan, bool) {
	var latest ExecutionPlan

	for _, plan := range plans {
		if plan.Revision > latest.Revision {
			latest = plan
		}
	}

	return latest, latest.Revision > 0
}

func ValidateExecutionPlanBody(field, body string) FieldError {
	trimmed := strings.TrimSpace(body)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > ExecutionPlanBodyMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateExecutionPlanRef(field, ref string) FieldError {
	trimmed := strings.TrimSpace(ref)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > ExecutionPlanRefMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func BlockingQuestionsOpen(questions []IssueQuestion) []IssueQuestion {
	open := make([]IssueQuestion, 0, len(questions))

	for _, question := range questions {
		if question.Blocking && !question.Settled() {
			open = append(open, question)
		}
	}

	return open
}
