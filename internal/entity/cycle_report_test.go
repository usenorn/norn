package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func at(day int, hour int) time.Time {
	return time.Date(2026, time.January, day, hour, 0, 0, 0, time.UTC)
}

func window() []string {
	days, err := entity.CalendarDaysBetween("2026-01-01", "2026-01-05")
	if err != nil {
		panic(err)
	}

	return days
}

func remaining(burndown entity.CycleBurndown) []int {
	counts := make([]int, 0, len(burndown.Points))

	for _, point := range burndown.Points {
		counts = append(counts, point.Remaining)
	}

	return counts
}

func scopes(burndown entity.CycleBurndown) []int {
	counts := make([]int, 0, len(burndown.Points))

	for _, point := range burndown.Points {
		counts = append(counts, point.Scope)
	}

	return counts
}

func same(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}

	for index, value := range got {
		if want[index] != value {
			return false
		}
	}

	return true
}

func TestABurndownIsReadBackwardsFromWhereAnIssueStandsNow(t *testing.T) {
	for _, probe := range []struct {
		name      string
		history   entity.CycleHistory
		remaining []int
		scope     []int
		why       string
	}{
		{
			name: "finished on the third day and stayed finished",
			history: entity.CycleHistory{
				Category: entity.StateCategoryComplete,
				Spells:   []entity.CycleSpell{{}},
				Changes: []entity.CycleStateChange{
					{From: entity.StateCategoryActive, To: entity.StateCategoryComplete, At: at(3, 10)},
				},
			},
			remaining: []int{1, 1, 0, 0, 0},
			scope:     []int{1, 1, 1, 1, 1},
			why:       "the line falls on the day the work finished and stays down",
		},
		{
			name: "finished, reopened, finished again",
			history: entity.CycleHistory{
				Category: entity.StateCategoryComplete,
				Spells:   []entity.CycleSpell{{}},
				Changes: []entity.CycleStateChange{
					{From: entity.StateCategoryActive, To: entity.StateCategoryComplete, At: at(2, 9)},
					{From: entity.StateCategoryComplete, To: entity.StateCategoryActive, At: at(3, 9)},
					{From: entity.StateCategoryActive, To: entity.StateCategoryComplete, At: at(5, 9)},
				},
			},
			remaining: []int{1, 0, 1, 1, 0},
			scope:     []int{1, 1, 1, 1, 1},
			why: "a reopened issue climbs back onto the line; reading only the last completion " +
				"would draw it as finished from the second day onwards",
		},
		{
			name: "taken out of the cycle part way through",
			history: entity.CycleHistory{
				Category: entity.StateCategoryActive,
				Spells:   []entity.CycleSpell{{To: pointerTo(at(3, 12))}},
			},
			remaining: []int{1, 1, 0, 0, 0},
			scope:     []int{1, 1, 0, 0, 0},
			why:       "an issue removed from the cycle leaves the scope, it does not read as finished",
		},
		{
			name: "added part way through",
			history: entity.CycleHistory{
				Category: entity.StateCategoryActive,
				Spells:   []entity.CycleSpell{{From: pointerTo(at(3, 8))}},
			},
			remaining: []int{0, 0, 1, 1, 1},
			scope:     []int{0, 0, 1, 1, 1},
			why:       "work added mid-cycle counts from the day it arrived, not from the start",
		},
		{
			name: "edited after the window closed",
			history: entity.CycleHistory{
				Category: entity.StateCategoryActive,
				Spells:   []entity.CycleSpell{{}},
				Changes: []entity.CycleStateChange{
					{From: entity.StateCategoryComplete, To: entity.StateCategoryActive, At: at(20, 9)},
				},
			},
			remaining: []int{0, 0, 0, 0, 0},
			scope:     []int{1, 1, 1, 1, 1},
			why: "reopening an issue weeks later must not rewrite the days it was finished on, " +
				"which is the whole reason the series is read backwards through the ledger",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			probe.history.IssueID = uuid.New()

			burndown := entity.BurndownOver(window(), []entity.CycleHistory{probe.history}, time.UTC, time.Time{})

			if got := remaining(burndown); !same(got, probe.remaining) {
				t.Errorf("remaining = %v, want %v: %s", got, probe.remaining, probe.why)
			}

			if got := scopes(burndown); !same(got, probe.scope) {
				t.Errorf("scope = %v, want %v: %s", got, probe.scope, probe.why)
			}
		})
	}
}

func TestAnIssueWhoseHistoryCannotBeReadIsMarkedUnknownRatherThanCountedEitherWay(t *testing.T) {
	history := entity.CycleHistory{
		IssueID:  uuid.New(),
		Category: entity.StateCategoryComplete,
		Spells:   []entity.CycleSpell{{}},
		Changes: []entity.CycleStateChange{
			{From: entity.CycleCategoryUnknown, To: entity.StateCategoryComplete, At: at(3, 10)},
		},
	}

	burndown := entity.BurndownOver(window(), []entity.CycleHistory{history}, time.UTC, time.Time{})

	if burndown.Whole() || burndown.Unreadable() != 2 {
		t.Fatalf(
			"the series says Whole() = %v with %d unreadable days, want false and two. Activity "+
				"recorded before categories were kept says only that a state changed, never to "+
				"what kind of state, and a name cannot be turned back into a category.",
			burndown.Whole(), burndown.Unreadable(),
		)
	}

	if got := remaining(burndown); !same(got, []int{0, 0, 0, 0, 0}) {
		t.Fatalf(
			"remaining = %v, want zero throughout: a day nobody can vouch for is reported as "+
				"unknown, not quietly drawn as work still open. The chart breaks the line there.",
			got,
		)
	}

	if burndown.Points[0].Known() || burndown.Points[0].Unknown != 1 {
		t.Errorf(
			"the first day reports Known() = %v with %d unreadable issues, want false and one: "+
				"the uncertainty belongs on the day it applies to",
			burndown.Points[0].Known(), burndown.Points[0].Unknown,
		)
	}

	for _, point := range burndown.Points[2:] {
		if !point.Known() {
			t.Errorf(
				"day %s reads as unknown. Once the ledger reaches a change it can read, every "+
					"later day is certain again and must be drawn.",
				point.On,
			)
		}
	}
}

func TestAbandonedWorkLeavesTheLineRatherThanHoldingItUp(t *testing.T) {
	for _, category := range []entity.StateCategory{
		entity.StateCategoryComplete, entity.StateCategoryAbandoned,
	} {
		history := entity.CycleHistory{
			IssueID:  uuid.New(),
			Category: category,
			Spells:   []entity.CycleSpell{{}},
		}

		burndown := entity.BurndownOver(window(), []entity.CycleHistory{history}, time.UTC, time.Time{})

		if got := remaining(burndown); !same(got, []int{0, 0, 0, 0, 0}) {
			t.Errorf(
				"remaining = %v for a %s issue, want zero throughout. Remaining work is what is "+
					"still open — not started or active — and a cancelled issue is neither. A "+
					"cycle that cancels its last issue reaches zero.",
				got, category,
			)
		}
	}
}

func TestAnUnreadableChangeBeforeTheCycleDoesNotSpoilTheDaysInside(t *testing.T) {
	history := entity.CycleHistory{
		IssueID:  uuid.New(),
		Category: entity.StateCategoryActive,
		Spells:   []entity.CycleSpell{{From: pointerTo(at(3, 8))}},
		Changes: []entity.CycleStateChange{
			{From: entity.CycleCategoryUnknown, To: entity.StateCategoryActive, At: at(1, 9)},
		},
	}

	burndown := entity.BurndownOver(window(), []entity.CycleHistory{history}, time.UTC, time.Time{})

	if !burndown.Whole() {
		t.Fatalf(
			"the series reports %d unreadable days. The unreadable change happened before this "+
				"issue joined the cycle, so it says nothing about any day the cycle covers.",
			burndown.Unreadable(),
		)
	}

	if got := remaining(burndown); !same(got, []int{0, 0, 1, 1, 1}) {
		t.Fatalf("remaining = %v, want the issue counted only from the day it joined", got)
	}
}

func TestAnIssueTakenOutAndPutBackIsAbsentOnlyWhileItWasGone(t *testing.T) {
	history := entity.CycleHistory{
		IssueID:  uuid.New(),
		Category: entity.StateCategoryActive,
		Spells: []entity.CycleSpell{
			{To: pointerTo(at(2, 12))},
			{From: pointerTo(at(4, 8))},
		},
	}

	burndown := entity.BurndownOver(window(), []entity.CycleHistory{history}, time.UTC, time.Time{})

	if got := scopes(burndown); !same(got, []int{1, 0, 0, 1, 1}) {
		t.Fatalf(
			"scope = %v, want [1 0 0 1 1]. An issue can leave a cycle and come back to it, so "+
				"membership is a set of spells rather than one joined-and-left pair.",
			got,
		)
	}
}

func pointerTo[T any](value T) *T {
	return &value
}

func TestMidnightBelongsToTheDayItStarts(t *testing.T) {
	midnight := at(4, 0)

	for _, probe := range []struct {
		name      string
		history   entity.CycleHistory
		scope     []int
		remaining []int
		why       string
	}{
		{
			name: "joined exactly at midnight",
			history: entity.CycleHistory{
				Category: entity.StateCategoryActive,
				Spells:   []entity.CycleSpell{{From: pointerTo(midnight)}},
			},
			scope:     []int{0, 0, 0, 1, 1},
			remaining: []int{0, 0, 0, 1, 1},
			why:       "arriving as the fourth begins is the fourth, not the third",
		},
		{
			name: "left exactly at midnight",
			history: entity.CycleHistory{
				Category: entity.StateCategoryActive,
				Spells:   []entity.CycleSpell{{To: pointerTo(midnight)}},
			},
			scope:     []int{1, 1, 1, 0, 0},
			remaining: []int{1, 1, 1, 0, 0},
			why:       "leaving as the fourth begins still counts for the whole of the third",
		},
		{
			name: "finished exactly at midnight",
			history: entity.CycleHistory{
				Category: entity.StateCategoryComplete,
				Spells:   []entity.CycleSpell{{}},
				Changes: []entity.CycleStateChange{
					{From: entity.StateCategoryActive, To: entity.StateCategoryComplete, At: midnight},
				},
			},
			scope:     []int{1, 1, 1, 1, 1},
			remaining: []int{1, 1, 1, 0, 0},
			why:       "work finished as the fourth begins was still open on the third",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			probe.history.IssueID = uuid.New()

			burndown := entity.BurndownOver(window(), []entity.CycleHistory{probe.history}, time.UTC, time.Time{})

			if got := scopes(burndown); !same(got, probe.scope) {
				t.Errorf("scope = %v, want %v: %s", got, probe.scope, probe.why)
			}

			if got := remaining(burndown); !same(got, probe.remaining) {
				t.Errorf("remaining = %v, want %v: %s", got, probe.remaining, probe.why)
			}
		})
	}
}

func TestADayIsClosedByItsOwnZoneEvenWhenTheClocksMove(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin is not available: %v", err)
	}

	days, err := entity.CalendarDaysBetween("2026-03-28", "2026-03-30")
	if err != nil {
		t.Fatalf("days: %v", err)
	}

	finished := time.Date(2026, time.March, 30, 0, 30, 0, 0, berlin)

	history := entity.CycleHistory{
		IssueID:  uuid.New(),
		Category: entity.StateCategoryComplete,
		Spells:   []entity.CycleSpell{{}},
		Changes: []entity.CycleStateChange{
			{From: entity.StateCategoryActive, To: entity.StateCategoryComplete, At: finished},
		},
	}

	if got := remaining(entity.BurndownOver(days, []entity.CycleHistory{history}, berlin, time.Time{})); !same(got, []int{1, 1, 0}) {
		t.Fatalf(
			"remaining = %v in Berlin, want [1 1 0]. Half past midnight on the thirtieth is the "+
				"thirtieth for the team living there, and the twenty-ninth is twenty-three hours "+
				"long that week, so the day has to close at its own local midnight.",
			got,
		)
	}

	if got := remaining(entity.BurndownOver(days, []entity.CycleHistory{history}, time.UTC, time.Time{})); !same(got, []int{1, 0, 0}) {
		t.Fatalf(
			"remaining = %v in UTC, want [1 0 0]. The same instant is half past ten the evening "+
				"before in UTC: reading a team's cycle in the wrong zone moves work to the wrong "+
				"day, which is why the zone is passed in rather than assumed.",
			got,
		)
	}
}

func TestMembershipIsReadAsSpellsFromTheScopeLedger(t *testing.T) {
	issueID := uuid.New()

	event := func(kind entity.CycleScopeChangeKind, day int) entity.CycleScopeChange {
		return entity.CycleScopeChange{IssueID: issueID, Change: kind, ChangedAt: at(day, 9)}
	}

	for _, probe := range []struct {
		name   string
		events []entity.CycleScopeChange
		want   []entity.CycleSpell
		why    string
	}{
		{
			name: "nothing recorded",
			want: []entity.CycleSpell{{}},
			why:  "an issue the ledger says nothing about was there for the whole cycle",
		},
		{
			name:   "added after the cycle began",
			events: []entity.CycleScopeChange{event(entity.CycleScopeChangeAdded, 3)},
			want:   []entity.CycleSpell{{From: pointerTo(at(3, 9))}},
			why:    "the first event being an arrival means it was not there before",
		},
		{
			name:   "removed part way through",
			events: []entity.CycleScopeChange{event(entity.CycleScopeChangeRemoved, 3)},
			want:   []entity.CycleSpell{{To: pointerTo(at(3, 9))}},
			why:    "leaving without an arrival means it was there from the beginning",
		},
		{
			name: "removed and added back",
			events: []entity.CycleScopeChange{
				event(entity.CycleScopeChangeRemoved, 2),
				event(entity.CycleScopeChangeAdded, 4),
			},
			want: []entity.CycleSpell{{To: pointerTo(at(2, 9))}, {From: pointerTo(at(4, 9))}},
			why:  "two spells, with the gap between them",
		},
		{
			name: "rolled into the next cycle at the end",
			events: []entity.CycleScopeChange{
				event(entity.CycleScopeChangeAdded, 2),
				event(entity.CycleScopeChangeRolledOver, 5),
			},
			want: []entity.CycleSpell{{From: pointerTo(at(2, 9)), To: pointerTo(at(5, 9))}},
			why:  "rolling over and returning both end a spell, the same as a removal",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			got := entity.SpellsFrom(probe.events)

			if len(got) != len(probe.want) {
				t.Fatalf("read %d spells, want %d: %s", len(got), len(probe.want), probe.why)
			}

			for index, spell := range got {
				want := probe.want[index]

				if (spell.From == nil) != (want.From == nil) ||
					(spell.From != nil && !spell.From.Equal(*want.From)) {
					t.Errorf("spell %d starts at %v, want %v: %s", index, spell.From, want.From, probe.why)
				}

				if (spell.To == nil) != (want.To == nil) ||
					(spell.To != nil && !spell.To.Equal(*want.To)) {
					t.Errorf("spell %d ends at %v, want %v: %s", index, spell.To, want.To, probe.why)
				}
			}
		})
	}
}

func TestDaysBeforeTheRecordersExistedAreUnknownWhateverTheIssueLooksLikeNow(t *testing.T) {
	recordingFrom := at(3, 12)

	history := entity.CycleHistory{
		IssueID:   uuid.New(),
		Category:  entity.StateCategoryActive,
		CreatedAt: at(1, 0).AddDate(0, -3, 0),
		Spells:    []entity.CycleSpell{{}},
	}

	burndown := entity.BurndownOver(window(), []entity.CycleHistory{history}, time.UTC, recordingFrom)

	if got := remaining(burndown); !same(got, []int{0, 0, 0, 1, 1}) {
		t.Fatalf(
			"remaining = %v, want [0 0 0 1 1]. Nothing was written down before the recorders "+
				"existed, so no reading of those days can be defended — not from the issue's "+
				"creation date, which says when it existed rather than when it joined this "+
				"cycle, and not from an empty ledger, which is silence rather than evidence.",
			got,
		)
	}

	for _, point := range burndown.Points[:3] {
		if point.Known() || point.Unknown != 1 {
			t.Errorf("day %s reports Known() = %v, want the uncertainty carried openly", point.On, point.Known())
		}
	}

	for _, point := range burndown.Points[3:] {
		if !point.Known() {
			t.Errorf("day %s reads as unknown, but the recorders were running by then", point.On)
		}
	}
}

func TestACycleThatBeganAfterTheRecordersIsDrawnInFull(t *testing.T) {
	recordingFrom := at(1, 0).AddDate(0, 0, -1)

	imported := entity.CycleHistory{
		IssueID:   uuid.New(),
		Category:  entity.StateCategoryActive,
		CreatedAt: at(1, 0).AddDate(0, -3, 0),
		Spells:    []entity.CycleSpell{{}},
	}

	burndown := entity.BurndownOver(window(), []entity.CycleHistory{imported}, time.UTC, recordingFrom)

	if !burndown.Whole() {
		t.Fatalf(
			"a cycle that ran entirely after the recorders started reports %d unreadable days. "+
				"Once every join and every state change is being written down, an empty ledger "+
				"for an issue that is a member now does mean it was a member throughout.",
			burndown.Unreadable(),
		)
	}
}

func TestADayNobodyRecordedIsUnavailableEvenWithNothingToShow(t *testing.T) {
	recordingFrom := at(3, 0)

	burndown := entity.BurndownOver(window(), nil, time.UTC, recordingFrom)

	if burndown.Whole() {
		t.Fatal(
			"a cycle with no candidates at all reports every day as certain. Whether anything " +
				"was in scope on a day nobody was recording is itself unknown: an empty answer " +
				"is not the same as an answer of none.",
		)
	}

	for _, point := range burndown.Points[:2] {
		if point.Known() || !point.Unrecorded {
			t.Errorf("day %s reads as certain, but nothing was being written down then", point.On)
		}
	}

	for _, point := range burndown.Points[2:] {
		if !point.Known() {
			t.Errorf("day %s reads as unknown although the records cover it and hold nothing", point.On)
		}
	}
}
