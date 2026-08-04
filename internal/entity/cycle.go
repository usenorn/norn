package entity

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	CycleUpcomingCount  = 3
	CycleMinLengthWeeks = 1
	CycleMaxLengthWeeks = 4
	daysPerWeek         = 7
)

var (
	ErrCycleNotFound         = errors.New("cycle not found")
	ErrCycleClosed           = errors.New("cycle is closed")
	ErrCycleOverlaps         = errors.New("cycle overlaps another cycle in this team")
	ErrCycleTeamMismatch     = errors.New("cycle belongs to another team")
	ErrCycleRolloverRequired = errors.New("cycle has unfinished issues and no rollover decision")
	ErrCycleNotEnded         = errors.New("cycle has not ended yet")
	ErrCycleCadenceNotFound  = errors.New("team does not use cycles")
	ErrCycleNoNextCycle      = errors.New("team has no later cycle to move issues to")
)

type CyclePhase string

const (
	CyclePhaseUpcoming CyclePhase = "upcoming"
	CyclePhaseCurrent  CyclePhase = "current"
	CyclePhaseEnded    CyclePhase = "ended"
	CyclePhaseClosed   CyclePhase = "closed"
)

func CyclePhases() []CyclePhase {
	return []CyclePhase{CyclePhaseUpcoming, CyclePhaseCurrent, CyclePhaseEnded, CyclePhaseClosed}
}

func (p CyclePhase) Valid() bool {
	return slices.Contains(CyclePhases(), p)
}

type CycleRollover string

const (
	CycleRolloverNone    CycleRollover = ""
	CycleRolloverNext    CycleRollover = "next"
	CycleRolloverBacklog CycleRollover = "backlog"
)

func (r CycleRollover) Valid() bool {
	return r == CycleRolloverNext || r == CycleRolloverBacklog
}

type Cycle struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	TeamID            uuid.UUID
	TeamKey           string
	Number            int
	StartsOn          string
	EndsOn            string
	ClosedAt          *time.Time
	ClosedByAccountID uuid.UUID
	Rollover          CycleRollover
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (c Cycle) Closed() bool {
	return c.ClosedAt != nil
}

func (c Cycle) PhaseOn(today string) CyclePhase {
	switch {
	case c.Closed():
		return CyclePhaseClosed
	case today < c.StartsOn:
		return CyclePhaseUpcoming
	case today > c.EndsOn:
		return CyclePhaseEnded
	default:
		return CyclePhaseCurrent
	}
}

func (c Cycle) Started(today string) bool {
	return today >= c.StartsOn
}

type CycleCadence struct {
	TeamID      uuid.UUID
	WorkspaceID uuid.UUID
	LengthWeeks int
	AnchorOn    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c CycleCadence) Weekday() time.Weekday {
	anchor, err := ParseCalendarDate(c.AnchorOn)
	if err != nil {
		return time.Monday
	}

	return anchor.Weekday()
}

func (c CycleCadence) WindowFrom(startsOn string) (string, string, error) {
	start, err := ParseCalendarDate(startsOn)
	if err != nil {
		return "", "", err
	}

	return startsOn, FormatCalendarDate(start.AddDate(0, 0, c.LengthWeeks*daysPerWeek-1)), nil
}

func (c CycleCadence) StartAfter(endsOn string) (string, error) {
	end, err := ParseCalendarDate(endsOn)
	if err != nil {
		return "", err
	}

	return FormatCalendarDate(end.AddDate(0, 0, 1)), nil
}

func ValidateCycleLength(weeks int) error {
	if weeks < CycleMinLengthWeeks || weeks > CycleMaxLengthWeeks {
		return NewValidationError(FieldError{Field: "lengthWeeks", Code: ValidationCodeOutOfRange})
	}

	return nil
}

func ValidateCycleWeekday(weekday int) error {
	if weekday < int(time.Sunday) || weekday > int(time.Saturday) {
		return NewValidationError(FieldError{Field: "startsOn", Code: ValidationCodeUnsupportedValue})
	}

	return nil
}

func NextWeekdayOnOrAfter(date string, weekday time.Weekday) (string, error) {
	from, err := ParseCalendarDate(date)
	if err != nil {
		return "", err
	}

	shift := (int(weekday) - int(from.Weekday()) + daysPerWeek) % daysPerWeek

	return FormatCalendarDate(from.AddDate(0, 0, shift)), nil
}

func ParseCalendarDate(date string) (time.Time, error) {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return time.Time{}, NewValidationError(FieldError{Field: "date", Code: ValidationCodeMalformed})
	}

	return parsed, nil
}

func FormatCalendarDate(at time.Time) string {
	return at.Format(time.DateOnly)
}

func Today(now time.Time, timezone string) string {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return FormatCalendarDate(now.UTC())
	}

	return FormatCalendarDate(now.In(location))
}

type CycleScopeChangeKind string

const (
	CycleScopeChangeAdded      CycleScopeChangeKind = "added"
	CycleScopeChangeRemoved    CycleScopeChangeKind = "removed"
	CycleScopeChangeRolledOver CycleScopeChangeKind = "rolled_over"
	CycleScopeChangeReturned   CycleScopeChangeKind = "returned"
)

func CycleScopeChangeKinds() []CycleScopeChangeKind {
	return []CycleScopeChangeKind{
		CycleScopeChangeAdded,
		CycleScopeChangeRemoved,
		CycleScopeChangeRolledOver,
		CycleScopeChangeReturned,
	}
}

func (k CycleScopeChangeKind) Valid() bool {
	return slices.Contains(CycleScopeChangeKinds(), k)
}

type CycleScopeChange struct {
	ID             uuid.UUID
	CycleID        uuid.UUID
	IssueID        uuid.UUID
	IssueReference string
	IssueTitle     string
	Change         CycleScopeChangeKind
	ActorAccountID uuid.UUID
	ChangedAt      time.Time
}
