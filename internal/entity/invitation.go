package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	InvitationTokenTTL   = 7 * 24 * time.Hour
	InvitationTokenBytes = 32
)

var (
	ErrInvitationNotFound        = errors.New("invitation not found")
	ErrInvitationExpired         = errors.New("invitation link expired")
	ErrInvitationRevoked         = errors.New("invitation was revoked")
	ErrInvitationAccepted        = errors.New("invitation was already accepted")
	ErrInvitationTokenInvalid    = errors.New("invitation token is invalid")
	ErrInvitationAddressMismatch = errors.New("invitation was issued to a different address")
)

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

func (s InvitationStatus) Valid() bool {
	switch s {
	case InvitationStatusPending, InvitationStatusAccepted, InvitationStatusRevoked:
		return true
	default:
		return false
	}
}

type InvitationDelivery string

const (
	InvitationDeliveryPending InvitationDelivery = "pending"
	InvitationDeliverySent    InvitationDelivery = "sent"
	InvitationDeliveryFailed  InvitationDelivery = "failed"
)

func (d InvitationDelivery) Valid() bool {
	switch d {
	case InvitationDeliveryPending, InvitationDeliverySent, InvitationDeliveryFailed:
		return true
	default:
		return false
	}
}

type InvitationOutcome string

const (
	InvitationOutcomeCreated       InvitationOutcome = "created"
	InvitationOutcomeInvalidEmail  InvitationOutcome = "invalid_email"
	InvitationOutcomeAlreadyMember InvitationOutcome = "already_member"
)

func (o InvitationOutcome) Valid() bool {
	switch o {
	case InvitationOutcomeCreated, InvitationOutcomeInvalidEmail, InvitationOutcomeAlreadyMember:
		return true
	default:
		return false
	}
}

type Invitation struct {
	ID                  uuid.UUID
	WorkspaceID         uuid.UUID
	Email               string
	Role                MembershipRole
	TeamIDs             []uuid.UUID
	Status              InvitationStatus
	Delivery            InvitationDelivery
	TokenHash           []byte
	InvitedByAccountID  *uuid.UUID
	InvitedAt           time.Time
	ExpiresAt           time.Time
	AcceptedAt          *time.Time
	AcceptedByAccountID *uuid.UUID
	RevokedAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (i Invitation) Accepted() bool {
	return i.Status == InvitationStatusAccepted
}

func (i Invitation) Revoked() bool {
	return i.Status == InvitationStatusRevoked
}

func (i Invitation) ExpiredAt(now time.Time) bool {
	return !now.Before(i.ExpiresAt)
}

func (i Invitation) UsableAt(now time.Time) error {
	switch {
	case i.Revoked():
		return ErrInvitationRevoked
	case i.Accepted():
		return ErrInvitationAccepted
	case i.ExpiredAt(now):
		return ErrInvitationExpired
	default:
		return nil
	}
}

func NewInvitationToken() (string, []byte, error) {
	raw := make([]byte, InvitationTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate invitation token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, HashInvitationToken(token), nil
}

func HashInvitationToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}
