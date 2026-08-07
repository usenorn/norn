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

// SCMIdentity is who a person is on a forge. It is stated rather than guessed: a display
// name that happens to match is not evidence, and acting on one puts somebody else's work on
// a stranger.
type SCMIdentity struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Provider    SCMProvider
	Login       string
	CreatedAt   time.Time
}

type SCMIdentities []SCMIdentity

// AccountFor answers who a login belongs to. An unmapped login is not an error and not a
// guess — it is simply somebody this workspace has not said anything about, and every caller
// has to carry on without them.
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

// MirrorConflict is the edit arbitration discarded. Keeping it is what makes the rule
// something a person can live with: they can see what they wrote and put it back, rather
// than discovering that a machine chose and their work is gone.
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
