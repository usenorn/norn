package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrSSOIdentityNotFound = errors.New("this account is not linked to a provider identity")

type SSOIdentity struct {
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Subject     string
	LinkedAt    time.Time
}

func MatchLink(linked *SSOIdentity, subject, email string) error {
	if linked == nil {
		return nil
	}

	if strings.TrimSpace(subject) == linked.Subject {
		return nil
	}

	return NewSSOError(
		SSOStageMatching,
		"Your provider says you are "+NormalizeEmail(email)+", but a different provider "+
			"identity is already linked to that Norn account. An administrator can unlink it "+
			"in Settings, Members.",
	)
}
