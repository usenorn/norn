package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSSOIdentityNotFound = errors.New("this account is not linked to a provider identity")

	ErrSSOSubjectLinked = errors.New(
		"that provider identity is already linked to a different account in this workspace",
	)
)

type SSOIdentity struct {
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Issuer      string
	Subject     string
	LinkedAt    time.Time
}

func MatchLink(linked *SSOIdentity, issuer, subject, email string) error {
	if linked == nil || linked.Issuer != strings.TrimSpace(issuer) {
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
