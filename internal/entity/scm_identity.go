package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const SCMLoginMaxLen = 100

var (
	ErrSCMIdentityNotFound = errors.New("that platform account is not mapped to anybody here")
	ErrSCMIdentityExists   = errors.New("that platform account is already mapped to somebody")
)

type SCMIdentity struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Provider    SCMProvider
	Login       string
	CreatedAt   time.Time
}

type SCMIdentities []SCMIdentity

func (identities SCMIdentities) AccountFor(provider SCMProvider, login string) (uuid.UUID, bool) {
	for _, identity := range identities {
		if identity.Provider == provider && strings.EqualFold(identity.Login, login) {
			return identity.AccountID, true
		}
	}

	return uuid.Nil, false
}

func (identities SCMIdentities) LoginFor(provider SCMProvider, accountID uuid.UUID) (string, bool) {
	for _, identity := range identities {
		if identity.Provider == provider && identity.AccountID == accountID {
			return identity.Login, true
		}
	}

	return "", false
}

func NormalizeSCMLogin(login string) string {
	return strings.TrimPrefix(strings.TrimSpace(login), "@")
}

func ValidateSCMLogin(field, login string) FieldError {
	trimmed := NormalizeSCMLogin(login)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > SCMLoginMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case strings.ContainsAny(trimmed, " \t\n/"):
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}

type MirrorConflict struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	MirrorID    uuid.UUID
	IssueID     uuid.UUID
	Field       string
	Winner      MirrorWinner
	Discarded   string
	Kept        string
	OccurredAt  time.Time
}
