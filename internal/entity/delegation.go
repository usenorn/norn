package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	IssueBriefMaxLen = 4000

	DelegationClaimTTLMin     = 30 * time.Second
	DelegationClaimTTLMax     = time.Hour
	DelegationClaimTTLDefault = 5 * time.Minute
)

var (
	ErrIssueDelegationNotFound      = errors.New("issue delegation not found")
	ErrIssueDelegationHeld          = errors.New("this issue is already delegated")
	ErrIssueDelegationAgentUnusable = errors.New("that agent cannot take work")
	ErrIssueDelegationNotYours      = errors.New("this issue is delegated to another agent")
	ErrDelegationQueueNotAgent      = errors.New("only an agent has a delegation queue")
	ErrDelegationClaimHeld          = errors.New("another runner is already working this issue")
	ErrDelegationClaimLost          = errors.New("this claim is no longer held")
)

type DelegationClaim struct {
	Runner    string
	Token     uuid.UUID
	ClaimedAt time.Time
	ExpiresAt time.Time
}

func (c DelegationClaim) Held() bool {
	return c.Token != uuid.Nil
}

func (c DelegationClaim) Live(now time.Time) bool {
	return c.Held() && now.Before(c.ExpiresAt)
}

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
	Claim                DelegationClaim
}

func (d IssueDelegation) Open() bool {
	return d.RecalledAt == nil
}

func (d IssueDelegation) Claimable(now time.Time) bool {
	return d.Open() && !d.Claim.Live(now)
}

func DelegationClaimTTL(requested time.Duration) time.Duration {
	if requested <= 0 {
		return DelegationClaimTTLDefault
	}

	return requested
}

func ValidateIssueBrief(field, brief string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(brief)) > IssueBriefMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateDelegationClaimTTL(field string, ttl time.Duration) FieldError {
	if ttl < DelegationClaimTTLMin || ttl > DelegationClaimTTLMax {
		return FieldError{Field: field, Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
}
