package entity

import (
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const RequestKeyMaxLen = 128

var (
	ErrRequestKeyTaken    = errors.New("a request with this key is already being handled")
	ErrRequestKeyNotFound = errors.New("request key not found")
)

type RequestScope string

const RequestScopeIssue RequestScope = "issue"

func RequestScopes() []RequestScope {
	return []RequestScope{RequestScopeIssue}
}

func (s RequestScope) Valid() bool {
	return slices.Contains(RequestScopes(), s)
}

// RequestKey is what a caller sends so that asking twice cannot mean twice. A network that drops
// the answer looks exactly like one that dropped the question, and without this the only way to
// find out which happened is to look — by which time a second issue exists.
type RequestKey struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Scope       RequestScope
	Key         string
	IssueID     uuid.UUID
	CreatedAt   time.Time
}

func ValidateRequestKey(field, key string) FieldError {
	if utf8.RuneCountInString(key) > RequestKeyMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}
