package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const IssueBriefMaxLen = 4000

var (
	ErrIssueDelegationNotFound      = errors.New("issue delegation not found")
	ErrIssueDelegationHeld          = errors.New("this issue is already delegated")
	ErrIssueDelegationAgentUnusable = errors.New("that agent cannot take work")
)

type IssueDelegation struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	IssueID              uuid.UUID
	AgentID              uuid.UUID
	AgentName            string
	AgentAccountID       uuid.UUID
	Brief                string
	DelegatedByAccountID uuid.UUID
	DelegatedAt          time.Time
	RecalledByAccountID  uuid.UUID
	RecalledAt           *time.Time
}

func (d IssueDelegation) Open() bool {
	return d.RecalledAt == nil
}

func ValidateIssueBrief(field, brief string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(brief)) > IssueBriefMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}
