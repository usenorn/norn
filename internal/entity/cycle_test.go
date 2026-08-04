package entity_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestACycleIsCurrentOnBothItsBoundaryDays(t *testing.T) {
	cycle := entity.Cycle{StartsOn: "2026-08-03", EndsOn: "2026-08-16"}

	for _, today := range []string{"2026-08-03", "2026-08-10", "2026-08-16"} {
		if phase := cycle.PhaseOn(today); phase != entity.CyclePhaseCurrent {
			t.Errorf("on %s the cycle is %q, want current", today, phase)
		}
	}

	if phase := cycle.PhaseOn("2026-08-02"); phase != entity.CyclePhaseUpcoming {
		t.Errorf("the day before it starts the cycle is %q, want upcoming", phase)
	}

	if phase := cycle.PhaseOn("2026-08-17"); phase != entity.CyclePhaseEnded {
		t.Errorf("the day after it ends the cycle is %q, want ended", phase)
	}
}

func TestAnEndedCycleStaysEndedUntilSomebodyClosesIt(t *testing.T) {
	closed := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	cycle := entity.Cycle{StartsOn: "2026-08-03", EndsOn: "2026-08-16"}

	if phase := cycle.PhaseOn("2026-09-01"); phase != entity.CyclePhaseEnded {
		t.Fatalf(
			"a long-past cycle reports %q; nothing closes a cycle on its own, because the "+
				"rollover decision has to be made deliberately",
			phase,
		)
	}

	cycle.ClosedAt = &closed

	if phase := cycle.PhaseOn("2026-08-10"); phase != entity.CyclePhaseClosed {
		t.Errorf("a closed cycle reports %q even mid-window, want closed", phase)
	}
}

func TestConsecutiveCyclesMeetWithoutOverlappingOrLeavingAGap(t *testing.T) {
	cadence := entity.CycleCadence{LengthWeeks: 2, AnchorOn: "2026-08-03"}

	starts, ends, err := cadence.WindowFrom(cadence.AnchorOn)
	if err != nil {
		t.Fatalf("first window: %v", err)
	}

	if starts != "2026-08-03" || ends != "2026-08-16" {
		t.Fatalf("first window is %s to %s, want 2026-08-03 to 2026-08-16", starts, ends)
	}

	next, err := cadence.StartAfter(ends)
	if err != nil {
		t.Fatalf("start after: %v", err)
	}

	if next != "2026-08-17" {
		t.Fatalf(
			"the next cycle starts %s, want 2026-08-17; the exclusion constraint treats the "+
				"window as inclusive, so a cycle starting on the previous one's end date is refused",
			next,
		)
	}
}

func TestTheAnchorLandsOnTheChosenWeekdayWithoutSkippingToday(t *testing.T) {
	for name, tc := range map[string]struct {
		from    string
		weekday time.Weekday
		want    string
	}{
		"today already is that weekday": {"2026-08-03", time.Monday, "2026-08-03"},
		"later the same week":           {"2026-08-03", time.Thursday, "2026-08-06"},
		"wraps into the next week":      {"2026-08-06", time.Monday, "2026-08-10"},
		"sunday from a monday":          {"2026-08-03", time.Sunday, "2026-08-09"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := entity.NextWeekdayOnOrAfter(tc.from, tc.weekday)
			if err != nil {
				t.Fatalf("next weekday: %v", err)
			}

			if got != tc.want {
				t.Errorf("from %s the next %s is %s, want %s", tc.from, tc.weekday, got, tc.want)
			}
		})
	}
}

func TestTodayIsTheWorkspacesDayNotTheServersDay(t *testing.T) {
	lateInLondon := time.Date(2026, 8, 3, 23, 30, 0, 0, time.UTC)

	if today := entity.Today(lateInLondon, "Pacific/Auckland"); today != "2026-08-04" {
		t.Errorf("Auckland reads %s, want 2026-08-04; it is already tomorrow there", today)
	}

	if today := entity.Today(lateInLondon, "America/Los_Angeles"); today != "2026-08-03" {
		t.Errorf("Los Angeles reads %s, want 2026-08-03", today)
	}

	if today := entity.Today(lateInLondon, "Not/AZone"); today != "2026-08-03" {
		t.Errorf("an unloadable zone reads %s, want the UTC day 2026-08-03", today)
	}
}

func TestOnlyARealDestinationCountsAsARolloverDecision(t *testing.T) {
	for _, rollover := range []entity.CycleRollover{
		entity.CycleRolloverNext,
		entity.CycleRolloverBacklog,
	} {
		if !rollover.Valid() {
			t.Errorf("%q is not accepted as a rollover decision", rollover)
		}
	}

	if entity.CycleRolloverNone.Valid() {
		t.Fatal(
			"the empty rollover is accepted as a decision, so a cycle could be closed with " +
				"open issues and no destination for them",
		)
	}
}

func TestACadenceRemembersTheWeekdayItWasAnchoredOn(t *testing.T) {
	cadence := entity.CycleCadence{LengthWeeks: 1, AnchorOn: "2026-08-06"}

	if cadence.Weekday() != time.Thursday {
		t.Errorf("the cadence reports %s, want Thursday", cadence.Weekday())
	}
}

func TestACycleLengthOutsideTheSupportedRangeIsRefused(t *testing.T) {
	for _, weeks := range []int{0, -1, 5, 52} {
		if err := entity.ValidateCycleLength(weeks); err == nil {
			t.Errorf("a %d week cycle was accepted", weeks)
		}
	}

	for weeks := entity.CycleMinLengthWeeks; weeks <= entity.CycleMaxLengthWeeks; weeks++ {
		if err := entity.ValidateCycleLength(weeks); err != nil {
			t.Errorf("a %d week cycle was refused: %v", weeks, err)
		}
	}
}
