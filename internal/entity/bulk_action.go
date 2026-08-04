package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	BulkSyncLimit = 100
	BulkChunkSize = 25
)

var (
	ErrBulkActionNotFound    = errors.New("bulk action not found")
	ErrBulkChangeEmpty       = errors.New("a bulk change must alter something")
	ErrBulkChangeConflicting = errors.New("a bulk change cannot both alter properties and remove the issue")
	ErrBulkSetEmpty          = errors.New("a bulk action must name issues or a filter")
	ErrBulkSetAmbiguous      = errors.New("a bulk action takes either issue ids or a filter, not both")
)

type BulkOutcome string

const (
	BulkOutcomeApplied   BulkOutcome = "applied"
	BulkOutcomeUnchanged BulkOutcome = "unchanged"
	BulkOutcomeForbidden BulkOutcome = "forbidden"
	BulkOutcomeNotFound  BulkOutcome = "not_found"
	BulkOutcomeConflict  BulkOutcome = "conflict"
	BulkOutcomeInvalid   BulkOutcome = "invalid"
)

func BulkOutcomes() []BulkOutcome {
	return []BulkOutcome{
		BulkOutcomeApplied,
		BulkOutcomeUnchanged,
		BulkOutcomeForbidden,
		BulkOutcomeNotFound,
		BulkOutcomeConflict,
		BulkOutcomeInvalid,
	}
}

func (o BulkOutcome) Valid() bool {
	return slices.Contains(BulkOutcomes(), o)
}

func (o BulkOutcome) Applied() bool {
	return o == BulkOutcomeApplied || o == BulkOutcomeUnchanged
}

func OutcomeFor(err error) BulkOutcome {
	var validation ValidationError

	switch {
	case err == nil:
		return BulkOutcomeApplied

	case errors.Is(err, ErrIssueNotFound),
		errors.Is(err, ErrWorkflowStateNotFound),
		errors.Is(err, ErrLabelNotFound),
		errors.Is(err, ErrTeamNotFound):
		return BulkOutcomeNotFound

	case errors.Is(err, ErrAccountForbidden):
		return BulkOutcomeForbidden

	case errors.Is(err, ErrIssueStale),
		errors.Is(err, ErrIssueStatusTransition),
		errors.Is(err, ErrIssueChildrenOpen):
		return BulkOutcomeConflict

	case errors.As(err, &validation),
		errors.Is(err, ErrLabelOutOfScope),
		errors.Is(err, ErrIssueDestinationIncapable):
		return BulkOutcomeInvalid

	default:
		return BulkOutcomeInvalid
	}
}

type BulkActionStatus string

const (
	BulkActionQueued   BulkActionStatus = "queued"
	BulkActionRunning  BulkActionStatus = "running"
	BulkActionComplete BulkActionStatus = "complete"
	BulkActionFailed   BulkActionStatus = "failed"
)

func BulkActionStatuses() []BulkActionStatus {
	return []BulkActionStatus{BulkActionQueued, BulkActionRunning, BulkActionComplete, BulkActionFailed}
}

func (s BulkActionStatus) Valid() bool {
	return slices.Contains(BulkActionStatuses(), s)
}

func (s BulkActionStatus) Settled() bool {
	return s == BulkActionComplete || s == BulkActionFailed
}

func (s BulkActionStatus) CanTransitionTo(target BulkActionStatus) bool {
	switch s {
	case BulkActionQueued:
		return target == BulkActionRunning || target == BulkActionFailed
	case BulkActionRunning:
		return target == BulkActionComplete || target == BulkActionFailed
	default:
		return false
	}
}

type BulkChange struct {
	StateID    *uuid.UUID
	AssigneeID *uuid.UUID
	Priority   *IssuePriority
	AddLabelID *uuid.UUID
	Status     *IssueStatus

	ClearAssignee bool
}

func (c BulkChange) Touched() []string {
	fields := make([]string, 0, 5)

	if c.StateID != nil {
		fields = append(fields, IssueFieldState)
	}

	if c.AssigneeID != nil || c.ClearAssignee {
		fields = append(fields, IssueFieldAssignee)
	}

	if c.Priority != nil {
		fields = append(fields, IssueFieldPriority)
	}

	if c.AddLabelID != nil {
		fields = append(fields, IssueFieldLabels)
	}

	if c.Status != nil {
		fields = append(fields, IssueFieldStatus)
	}

	return fields
}

func (c BulkChange) Lifecycle() bool {
	return c.Status != nil
}

func (c BulkChange) Validate() error {
	touched := c.Touched()

	if len(touched) == 0 {
		return ErrBulkChangeEmpty
	}

	if c.Lifecycle() && len(touched) > 1 {
		return ErrBulkChangeConflicting
	}

	if c.Priority != nil && !c.Priority.Valid() {
		return NewValidationError(FieldError{Field: "priority", Code: ValidationCodeUnsupportedValue})
	}

	if c.Status != nil && !c.Status.Valid() {
		return NewValidationError(FieldError{Field: "status", Code: ValidationCodeUnsupportedValue})
	}

	if c.AssigneeID != nil && c.ClearAssignee {
		return ErrBulkChangeConflicting
	}

	return nil
}

type BulkFilter struct {
	TeamID   *uuid.UUID
	Statuses []IssueStatus
}

type BulkSet struct {
	IssueIDs []uuid.UUID
	Filter   *BulkFilter
}

func (s BulkSet) Validate() error {
	switch {
	case len(s.IssueIDs) == 0 && s.Filter == nil:
		return ErrBulkSetEmpty
	case len(s.IssueIDs) > 0 && s.Filter != nil:
		return ErrBulkSetAmbiguous
	default:
		return nil
	}
}

func (s BulkSet) RunsInline() bool {
	return s.Filter == nil && len(s.IssueIDs) <= BulkSyncLimit
}

func (s BulkSet) Expected() *int {
	if s.Filter != nil {
		return nil
	}

	expected := len(s.IssueIDs)

	return &expected
}

type BulkAction struct {
	ID                  uuid.UUID
	WorkspaceID         uuid.UUID
	RequestedByAccount  uuid.UUID
	RequestedActorKind  ActorKind
	RequestedAuthMethod SessionAuthMethod
	Change              BulkChange
	Set                 BulkSet
	Status              BulkActionStatus
	Expected            *int
	Processed           int
	StartedAt           *time.Time
	FinishedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (a BulkAction) Requester() Actor {
	kind := a.RequestedActorKind
	if kind == "" {
		kind = ActorKindUser
	}

	return Actor{
		Kind:       kind,
		AccountID:  a.RequestedByAccount,
		AuthMethod: a.RequestedAuthMethod,
	}
}

type BulkActionOutcome struct {
	IssueID   uuid.UUID
	Reference string
	Outcome   BulkOutcome
}

func Chunks[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = BulkChunkSize
	}

	chunks := make([][]T, 0, (len(items)+size-1)/size)

	for start := 0; start < len(items); start += size {
		end := min(start+size, len(items))
		chunks = append(chunks, items[start:end])
	}

	return chunks
}
