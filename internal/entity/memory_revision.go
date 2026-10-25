package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MemoryRevisionPageDefaultSize = 25
	MemoryRevisionPageMaxSize     = 100
)

var (
	ErrMemoryRevisionNotFound      = errors.New("memory revision not found")
	ErrMemoryRevisionCursorInvalid = errors.New("memory revision cursor is invalid")
)

type MemoryRevisionAction string

const (
	MemoryRevisionCreated    MemoryRevisionAction = "created"
	MemoryRevisionCorrected  MemoryRevisionAction = "corrected"
	MemoryRevisionReshared   MemoryRevisionAction = "reshared"
	MemoryRevisionAccepted   MemoryRevisionAction = "accepted"
	MemoryRevisionDisputed   MemoryRevisionAction = "disputed"
	MemoryRevisionSuperseded MemoryRevisionAction = "superseded"
	MemoryRevisionWithdrawn  MemoryRevisionAction = "withdrawn"
	MemoryRevisionRejected   MemoryRevisionAction = "rejected"
	MemoryRevisionReconciled MemoryRevisionAction = "reconciled"
)

func MemoryRevisionActions() []MemoryRevisionAction {
	return []MemoryRevisionAction{
		MemoryRevisionCreated,
		MemoryRevisionCorrected,
		MemoryRevisionReshared,
		MemoryRevisionAccepted,
		MemoryRevisionDisputed,
		MemoryRevisionSuperseded,
		MemoryRevisionWithdrawn,
		MemoryRevisionRejected,
		MemoryRevisionReconciled,
	}
}

func (a MemoryRevisionAction) Valid() bool {
	return slices.Contains(MemoryRevisionActions(), a)
}

type MemoryRevisionSummary struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	MemoryID    uuid.UUID
	Version     int
	Action      MemoryRevisionAction
	Status      MemoryStatus
	Statement   string
	Reason      string
	By          KnowledgeAttribution
	AccountName string
	CreatedAt   time.Time
}

type MemoryRevision struct {
	MemoryRevisionSummary
	Kind           MemoryKind
	Rationale      string
	Conditions     string
	Audience       KnowledgeAudience
	Applicability  KnowledgeApplicability
	Basis          MemoryBasis
	Evidence       []MemoryEvidence
	EffectiveFrom  time.Time
	EffectiveUntil *time.Time
}

func (m Memory) Revision(action MemoryRevisionAction, by KnowledgeAttribution) MemoryRevision {
	return MemoryRevision{
		MemoryRevisionSummary: MemoryRevisionSummary{
			ID:          uuid.New(),
			WorkspaceID: m.WorkspaceID,
			MemoryID:    m.ID,
			Version:     m.Version,
			Action:      action,
			Status:      m.Status,
			Statement:   m.Statement,
			By:          by,
			CreatedAt:   m.UpdatedAt,
		},
		Kind:           m.Kind,
		Rationale:      m.Rationale,
		Conditions:     m.Conditions,
		Audience:       m.Audience,
		Applicability:  m.Applicability,
		Basis:          m.Basis,
		Evidence:       m.Evidence,
		EffectiveFrom:  m.EffectiveFrom,
		EffectiveUntil: m.EffectiveUntil,
	}
}

func (m Memory) Rejection(reason string, by KnowledgeAttribution) MemoryRevision {
	revision := m.Revision(MemoryRevisionRejected, by)
	revision.Reason = strings.TrimSpace(reason)

	return revision
}

func (r MemoryRevisionSummary) Cursor() MemoryRevisionCursor {
	return MemoryRevisionCursor{Version: r.Version}
}

type MemoryRevisionCursor struct {
	Version int
}

func (c MemoryRevisionCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(c.Version)))
}

func DecodeMemoryRevisionCursor(raw string) (MemoryRevisionCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return MemoryRevisionCursor{}, ErrMemoryRevisionCursorInvalid
	}

	version, err := strconv.Atoi(string(decoded))
	if err != nil || version < 1 {
		return MemoryRevisionCursor{}, ErrMemoryRevisionCursorInvalid
	}

	return MemoryRevisionCursor{Version: version}, nil
}

type MemoryRevisionPage struct {
	Cursor *MemoryRevisionCursor
	Limit  int
}

func (p MemoryRevisionPage) Normalized() MemoryRevisionPage {
	if p.Limit <= 0 {
		p.Limit = MemoryRevisionPageDefaultSize
	}

	if p.Limit > MemoryRevisionPageMaxSize {
		p.Limit = MemoryRevisionPageMaxSize
	}

	return p
}

func (p MemoryRevisionPage) Lookahead() MemoryRevisionPage {
	p.Limit++

	return p
}
