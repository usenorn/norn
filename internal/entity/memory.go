package entity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MemoryStatementMaxLen  = 500
	MemoryRationaleMaxLen  = 4000
	MemoryConditionsMaxLen = 4000
	MemoryComponentMaxLen  = 100
	MemoryExcerptMaxLen    = 2000
	MemoryReasonMaxLen     = 1000
	MemoryEvidenceMax      = 20
	MemoryLineageMax       = 50

	MemoryPageDefaultSize = 50
	MemoryPageMaxSize     = 200

	memoryCursorIDLen = 36
)

const (
	MemoryFieldKind           = "kind"
	MemoryFieldStatement      = "statement"
	MemoryFieldRationale      = "rationale"
	MemoryFieldConditions     = "conditions"
	MemoryFieldBasis          = "basis"
	MemoryFieldComponent      = "component"
	MemoryFieldAppliesTo      = "appliesToProjectId"
	MemoryFieldProjectID      = "projectId"
	MemoryFieldEvidence       = "evidence"
	MemoryFieldEffectiveFrom  = "effectiveFrom"
	MemoryFieldEffectiveUntil = "effectiveUntil"
	MemoryFieldReason         = "reason"
)

var (
	ErrMemoryNotFound          = errors.New("memory not found")
	ErrMemoryStale             = errors.New("memory changed since it was read")
	ErrMemoryClosed            = errors.New("memory is superseded, withdrawn, or rejected")
	ErrMemoryTransitionInvalid = errors.New("memory cannot move to that status")
	ErrMemoryCursorInvalid     = errors.New("memory cursor is invalid")
	ErrMemoryReviewForbidden   = errors.New("memory review needs a workspace admin or the project lead")
)

type MemoryStaleError struct {
	Version int
}

func (e MemoryStaleError) Error() string {
	return fmt.Sprintf("%s: now at version %d", ErrMemoryStale, e.Version)
}

func (e MemoryStaleError) Unwrap() error {
	return ErrMemoryStale
}

type MemoryTransitionError struct {
	From MemoryStatus
	To   MemoryStatus
}

func (e MemoryTransitionError) Error() string {
	return fmt.Sprintf("%s: %s to %s", ErrMemoryTransitionInvalid, e.From, e.To)
}

func (e MemoryTransitionError) Unwrap() error {
	return ErrMemoryTransitionInvalid
}

type MemoryKind string

const (
	MemoryKindDecision   MemoryKind = "decision"
	MemoryKindConstraint MemoryKind = "constraint"
	MemoryKindLesson     MemoryKind = "lesson"
	MemoryKindPreference MemoryKind = "preference"
)

func MemoryKinds() []MemoryKind {
	return []MemoryKind{MemoryKindDecision, MemoryKindConstraint, MemoryKindLesson, MemoryKindPreference}
}

func (k MemoryKind) Valid() bool {
	return slices.Contains(MemoryKinds(), k)
}

type MemoryBasis string

const (
	MemoryBasisObservation MemoryBasis = "observation"
	MemoryBasisInference   MemoryBasis = "inference"
)

func MemoryBases() []MemoryBasis {
	return []MemoryBasis{MemoryBasisObservation, MemoryBasisInference}
}

func (b MemoryBasis) Valid() bool {
	return slices.Contains(MemoryBases(), b)
}

type MemoryStatus string

const (
	MemoryStatusProposed   MemoryStatus = "proposed"
	MemoryStatusAccepted   MemoryStatus = "accepted"
	MemoryStatusDisputed   MemoryStatus = "disputed"
	MemoryStatusSuperseded MemoryStatus = "superseded"
	MemoryStatusWithdrawn  MemoryStatus = "withdrawn"
	MemoryStatusRejected   MemoryStatus = "rejected"
)

func MemoryStatuses() []MemoryStatus {
	return []MemoryStatus{
		MemoryStatusProposed,
		MemoryStatusAccepted,
		MemoryStatusDisputed,
		MemoryStatusSuperseded,
		MemoryStatusWithdrawn,
		MemoryStatusRejected,
	}
}

func MemoryPendingStatuses() []MemoryStatus {
	return []MemoryStatus{MemoryStatusProposed, MemoryStatusDisputed}
}

func (s MemoryStatus) Valid() bool {
	return slices.Contains(MemoryStatuses(), s)
}

func (s MemoryStatus) Terminal() bool {
	return s == MemoryStatusSuperseded || s == MemoryStatusWithdrawn || s == MemoryStatusRejected
}

func (s MemoryStatus) CanBecome(to MemoryStatus) bool {
	switch s {
	case MemoryStatusProposed:
		return to == MemoryStatusAccepted || to == MemoryStatusDisputed || to == MemoryStatusWithdrawn ||
			to == MemoryStatusRejected
	case MemoryStatusAccepted:
		return to == MemoryStatusProposed || to == MemoryStatusDisputed || to == MemoryStatusSuperseded ||
			to == MemoryStatusWithdrawn
	case MemoryStatusDisputed:
		return to == MemoryStatusAccepted || to == MemoryStatusSuperseded || to == MemoryStatusWithdrawn ||
			to == MemoryStatusRejected
	default:
		return false
	}
}

type MemoryViewer struct {
	Edits    bool
	Reshares bool
	Reviews  bool
}

type Memory struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	Kind           MemoryKind
	Statement      string
	Rationale      string
	Conditions     string
	Audience       KnowledgeAudience
	Applicability  KnowledgeApplicability
	Status         MemoryStatus
	Basis          MemoryBasis
	Evidence       []MemoryEvidence
	AuthorID       uuid.UUID
	AuthorName     string
	By             KnowledgeAttribution
	EffectiveFrom  time.Time
	EffectiveUntil *time.Time
	SupersedesID   *uuid.UUID
	SupersededByID *uuid.UUID
	Version        int
	ContributorIDs []uuid.UUID
	Viewer         MemoryViewer
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (m Memory) Access() KnowledgeAccess {
	return KnowledgeAccess{
		WorkspaceID:    m.WorkspaceID,
		AuthorID:       m.AuthorID,
		Audience:       m.Audience,
		ContributorIDs: m.ContributorIDs,
		Applicability:  m.Applicability,
	}
}

func (m Memory) ViewedBy(reader KnowledgeReader) Memory {
	access := m.Access()
	m.Viewer = MemoryViewer{
		Edits:    reader.Edits(access),
		Reshares: reader.Reshares(access),
		Reviews:  reader.Reviews(access),
	}

	return m
}

func (m Memory) SettledBy(reader KnowledgeReader) bool {
	return m.Audience.Visibility == KnowledgeVisibilityPersonal || reader.Reviews(m.Access())
}

func (m Memory) Review(reader KnowledgeReader) error {
	if !reader.Reviews(m.Access()) {
		return ErrMemoryReviewForbidden
	}

	if !m.Pending() {
		return ErrMemoryNotPending
	}

	return nil
}

func ValidateMemoryReason(reason string) error {
	return NewValidationError(validateMemoryText(MemoryFieldReason, reason, MemoryReasonMaxLen, true))
}

func (m Memory) Expect(version int) error {
	if m.Version != version {
		return MemoryStaleError{Version: m.Version}
	}

	if m.Status.Terminal() {
		return ErrMemoryClosed
	}

	return nil
}

func (m *Memory) Become(to MemoryStatus) error {
	if !m.Status.CanBecome(to) {
		return MemoryTransitionError{From: m.Status, To: to}
	}

	m.Status = to

	return nil
}

func (m *Memory) SupersededBy(replacementID uuid.UUID) error {
	if err := m.Become(MemoryStatusSuperseded); err != nil {
		return err
	}

	m.SupersededByID = &replacementID

	return nil
}

func (m Memory) Restates(other Memory) bool {
	return m.Kind == other.Kind &&
		m.Statement == other.Statement &&
		m.Rationale == other.Rationale &&
		m.Conditions == other.Conditions &&
		sameID(m.Applicability.ProjectID, other.Applicability.ProjectID) &&
		m.Applicability.Component == other.Applicability.Component &&
		m.Basis == other.Basis &&
		m.EffectiveFrom.Equal(other.EffectiveFrom) &&
		sameDay(m.EffectiveUntil, other.EffectiveUntil) &&
		slices.EqualFunc(m.Evidence, other.Evidence, MemoryEvidence.Cites)
}

func sameDay(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equal(*b)
}

func (m Memory) Cursor() MemoryCursor {
	return MemoryCursor{UpdatedAt: m.UpdatedAt, MemoryID: m.ID}
}

func (m Memory) Validate() error {
	fields := []FieldError{
		validateMemoryText(MemoryFieldStatement, m.Statement, MemoryStatementMaxLen, true),
		validateMemoryText(MemoryFieldRationale, m.Rationale, MemoryRationaleMaxLen, false),
		validateMemoryText(MemoryFieldConditions, m.Conditions, MemoryConditionsMaxLen, false),
		validateMemoryText(MemoryFieldComponent, m.Applicability.Component, MemoryComponentMaxLen, false),
		ValidateMemoryDates(m.EffectiveFrom, m.EffectiveUntil),
	}

	if !m.Kind.Valid() {
		fields = append(fields, FieldError{Field: MemoryFieldKind, Code: ValidationCodeUnsupportedValue})
	}

	if !m.Basis.Valid() {
		fields = append(fields, FieldError{Field: MemoryFieldBasis, Code: ValidationCodeUnsupportedValue})
	}

	if len(m.Evidence) > MemoryEvidenceMax {
		fields = append(fields, FieldError{Field: MemoryFieldEvidence, Code: ValidationCodeTooLong})
	}

	for i, evidence := range m.Evidence {
		fields = append(fields, evidence.Validate(MemoryEvidenceField(i)))
	}

	return NewValidationError(fields...)
}

func validateMemoryText(field, text string, maxLen int, required bool) FieldError {
	trimmed := strings.TrimSpace(text)

	switch {
	case required && trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case strings.ContainsRune(text, 0):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	case utf8.RuneCountInString(trimmed) > maxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateMemoryDates(from time.Time, until *time.Time) FieldError {
	switch {
	case from.IsZero():
		return FieldError{Field: MemoryFieldEffectiveFrom, Code: ValidationCodeRequired}
	case until != nil && until.Before(from):
		return FieldError{Field: MemoryFieldEffectiveUntil, Code: ValidationCodeOutOfRange}
	default:
		return FieldError{}
	}
}

func MemoryDay(at time.Time) time.Time {
	year, month, day := at.UTC().Date()

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func MemoryEvidenceField(index int) string {
	return MemoryFieldEvidence + "[" + strconv.Itoa(index) + "]"
}

type MemoryCursor struct {
	UpdatedAt time.Time
	MemoryID  uuid.UUID
}

func (c MemoryCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.MemoryID.String() + c.UpdatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeMemoryCursor(raw string) (MemoryCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < memoryCursorIDLen {
		return MemoryCursor{}, ErrMemoryCursorInvalid
	}

	memoryID, err := uuid.Parse(string(decoded[:memoryCursorIDLen]))
	if err != nil {
		return MemoryCursor{}, ErrMemoryCursorInvalid
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, string(decoded[memoryCursorIDLen:]))
	if err != nil {
		return MemoryCursor{}, ErrMemoryCursorInvalid
	}

	return MemoryCursor{UpdatedAt: updatedAt, MemoryID: memoryID}, nil
}

type MemoryPage struct {
	Cursor *MemoryCursor
	Limit  int
}

func (p MemoryPage) Normalized() MemoryPage {
	if p.Limit <= 0 {
		p.Limit = MemoryPageDefaultSize
	}

	if p.Limit > MemoryPageMaxSize {
		p.Limit = MemoryPageMaxSize
	}

	return p
}

func (p MemoryPage) Lookahead() MemoryPage {
	p.Limit++

	return p
}
