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
	hoursPerDay         = 24
)

var (
	ErrCycleNotFound             = errors.New("cycle not found")
	ErrCycleClosed               = errors.New("cycle is closed")
	ErrCycleCreateRequiresOrigin = errors.New("cycle creation is reserved for an import")
	ErrCycleOverlaps             = errors.New("cycle overlaps another cycle in this team")
	ErrCycleTeamMismatch         = errors.New("cycle belongs to another team")
	ErrCycleRolloverRequired     = errors.New("cycle has unfinished issues and no rollover decision")
	ErrCycleOwnerNotOnTeam       = errors.New("cycle owner must be a member of the cycle's team")
	ErrCycleStale                = errors.New("cycle membership changed while it was being closed")
	ErrCycleNotEnded             = errors.New("cycle has not ended yet")
	ErrCycleCadenceNotFound      = errors.New("team does not use cycles")
	ErrCycleNoNextCycle          = errors.New("team has no later cycle to move issues to")
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
	CycleRolloverKeep    CycleRollover = "keep"
)

func CycleRollovers() []CycleRollover {
	return []CycleRollover{CycleRolloverNext, CycleRolloverBacklog, CycleRolloverKeep}
}

func (r CycleRollover) Valid() bool {
	return slices.Contains(CycleRollovers(), r)
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
	OwnerAccountID    uuid.UUID
	ResultsRecordedAt *time.Time
	Rollover          CycleRollover
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Origin            *ImportOrigin
}

func (c Cycle) Closed() bool {
	return c.ClosedAt != nil
}

func (c Cycle) Frozen() bool {
	return c.ResultsRecordedAt != nil
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

func ValidateCycleDate(field, date string) FieldError {
	if date == "" {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func ValidateCycleWindow(startsOn, endsOn string) FieldError {
	start, err := time.Parse(time.DateOnly, startsOn)
	if err != nil {
		return FieldError{}
	}

	end, err := time.Parse(time.DateOnly, endsOn)
	if err != nil {
		return FieldError{}
	}

	if end.Before(start) {
		return FieldError{Field: "endsOn", Code: ValidationCodeOutOfRange}
	}

	return FieldError{}
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

func StartOfCalendarDay(date string, in *time.Location) (time.Time, error) {
	day, err := ParseCalendarDate(date)
	if err != nil {
		return time.Time{}, err
	}

	if in == nil {
		in = time.UTC
	}

	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, in), nil
}

func StartOfNextCalendarDay(date string, in *time.Location) (time.Time, error) {
	day, err := ParseCalendarDate(date)
	if err != nil {
		return time.Time{}, err
	}

	if in == nil {
		in = time.UTC
	}

	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, in).AddDate(0, 0, 1), nil
}

func CalendarDaysBetween(from, to string) ([]string, error) {
	start, err := ParseCalendarDate(from)
	if err != nil {
		return nil, err
	}

	end, err := ParseCalendarDate(to)
	if err != nil {
		return nil, err
	}

	days := make([]string, 0)

	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		days = append(days, FormatCalendarDate(day))
	}

	return days, nil
}

func CalendarDaysSince(at time.Time, today string, in *time.Location) (int, error) {
	if in == nil {
		in = time.UTC
	}

	changed, err := ParseCalendarDate(FormatCalendarDate(at.In(in)))
	if err != nil {
		return 0, err
	}

	day, err := ParseCalendarDate(today)
	if err != nil {
		return 0, err
	}

	days := int(day.Sub(changed).Hours()) / hoursPerDay
	if days < 0 {
		return 0, nil
	}

	return days, nil
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
