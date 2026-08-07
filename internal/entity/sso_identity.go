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

func EmailClaimRefusal(account Account, elsewhere bool) error {
	if account.HasPassword() {
		return NewSSOError(
			SSOStageMatching,
			"The Norn account for "+NormalizeEmail(account.Email)+" has a password of its own, so "+
				"this workspace's provider cannot claim it by address. Sign in with that password "+
				"and connect your provider from account settings.",
		)
	}

	if elsewhere {
		return NewSSOError(
			SSOStageMatching,
			"The Norn account for "+NormalizeEmail(account.Email)+" is a member of other "+
				"workspaces, so this workspace's provider cannot claim it by address. Sign in as "+
				"you normally do and connect your provider from account settings.",
		)
	}

	return nil
}
