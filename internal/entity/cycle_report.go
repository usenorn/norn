package entity

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	CycleCategoryUnknown StateCategory = ""

	CycleStaleAfterDays = 5
)

type CycleResult struct {
	CycleID    uuid.UUID
	IssueID    uuid.UUID
	TeamID     uuid.UUID
	Category   StateCategory
	StateName  string
	Decision   CycleRollover
	RecordedAt time.Time
}

func (r CycleResult) Finished() bool {
	return r.Category == StateCategoryComplete
}

type CycleStateChange struct {
	IssueID uuid.UUID
	From    StateCategory
	To      StateCategory
	At      time.Time
}

func (c CycleStateChange) Known() bool {
	return c.From != CycleCategoryUnknown && c.To != CycleCategoryUnknown
}

type CycleStaleIssue struct {
	IssueID uuid.UUID
	Days    int
}

func StaleInCycle(
	issueID uuid.UUID,
	lastStatusChange time.Time,
	today string,
	in *time.Location,
) (CycleStaleIssue, bool) {
	if lastStatusChange.IsZero() {
		return CycleStaleIssue{}, false
	}

	days, err := CalendarDaysSince(lastStatusChange, today, in)
	if err != nil || days < CycleStaleAfterDays {
		return CycleStaleIssue{}, false
	}

	return CycleStaleIssue{IssueID: issueID, Days: days}, true
}

type CycleSpell struct {
	From *time.Time
	To   *time.Time
}

func (s CycleSpell) runsBefore(cutoff time.Time) bool {
	if s.From != nil && !s.From.Before(cutoff) {
		return false
	}

	return s.To == nil || !s.To.Before(cutoff)
}

type CycleHistory struct {
	IssueID   uuid.UUID
	Category  StateCategory
	CreatedAt time.Time
	Spells    []CycleSpell
	Changes   []CycleStateChange
}

func (h CycleHistory) heldBefore(cutoff time.Time) bool {
	if !h.CreatedAt.IsZero() && !h.CreatedAt.Before(cutoff) {
		return false
	}

	return slices.ContainsFunc(h.Spells, func(spell CycleSpell) bool { return spell.runsBefore(cutoff) })
}

func (h CycleHistory) categoryBefore(cutoff time.Time) (StateCategory, bool) {
	category := h.Category

	ordered := slices.Clone(h.Changes)
	slices.SortFunc(ordered, func(first, second CycleStateChange) int {
		return second.At.Compare(first.At)
	})

	for _, change := range ordered {
		if change.At.Before(cutoff) {
			break
		}

		if change.From == CycleCategoryUnknown {
			return CycleCategoryUnknown, false
		}

		category = change.From
	}

	return category, category != CycleCategoryUnknown
}

func SpellsFrom(events []CycleScopeChange) []CycleSpell {
	ordered := slices.Clone(events)
	slices.SortFunc(ordered, func(first, second CycleScopeChange) int {
		return first.ChangedAt.Compare(second.ChangedAt)
	})

	spells := make([]CycleSpell, 0, 1)
	inside := len(ordered) == 0 || ordered[0].Change != CycleScopeChangeAdded

	var opened *time.Time

	for _, event := range ordered {
		at := event.ChangedAt

		if event.Change == CycleScopeChangeAdded {
			if inside {
				continue
			}

			opened = &at
			inside = true

			continue
		}

		if !inside {
			continue
		}

		spells = append(spells, CycleSpell{From: opened, To: &at})
		opened = nil
		inside = false
	}

	if inside {
		spells = append(spells, CycleSpell{From: opened})
	}

	return spells
}

type CycleBurndownPoint struct {
	On         string
	Scope      int
	Remaining  int
	Unknown    int
	Unrecorded bool
}

func (p CycleBurndownPoint) Known() bool {
	return !p.Unrecorded && p.Unknown == 0
}

type CycleBurndown struct {
	Points []CycleBurndownPoint
}

func (b CycleBurndown) Whole() bool {
	return !slices.ContainsFunc(b.Points, func(point CycleBurndownPoint) bool { return !point.Known() })
}

func (b CycleBurndown) Unreadable() int {
	unknown := 0

	for _, point := range b.Points {
		if !point.Known() {
			unknown++
		}
	}

	return unknown
}

func BurndownOver(
	days []string,
	histories []CycleHistory,
	timezone *time.Location,
	recordingFrom time.Time,
) CycleBurndown {
	burndown := CycleBurndown{Points: make([]CycleBurndownPoint, 0, len(days))}

	for _, day := range days {
		closing, err := StartOfNextCalendarDay(day, timezone)
		if err != nil {
			continue
		}

		opening, err := StartOfCalendarDay(day, timezone)
		if err != nil {
			continue
		}

		covered := recordingFrom.IsZero() || !opening.Before(recordingFrom)

		point := CycleBurndownPoint{On: day, Unrecorded: !covered}

		for _, history := range histories {
			if !history.heldBefore(closing) {
				continue
			}

			point.Scope++

			if point.Unrecorded {
				point.Unknown++

				continue
			}

			category, known := history.categoryBefore(closing)

			switch {
			case !known:
				point.Unknown++
			case OpenCategory(category):
				point.Remaining++
			}
		}

		burndown.Points = append(burndown.Points, point)
	}

	return burndown
}
