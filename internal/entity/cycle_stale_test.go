package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestFiveCalendarDaysWithoutAStatusChangeAreWhatMakeAnIssueStale(t *testing.T) {
	issueID := uuid.New()
	today := "2026-08-12"

	probes := []struct {
		name    string
		changed time.Time
		days    int
		stale   bool
	}{
		{"moved today", time.Date(2026, time.August, 12, 8, 0, 0, 0, time.UTC), 0, false},
		{"moved four days ago", time.Date(2026, time.August, 8, 23, 59, 0, 0, time.UTC), 4, false},
		{"moved five days ago", time.Date(2026, time.August, 7, 0, 1, 0, 0, time.UTC), 5, true},
		{"moved nine days ago", time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC), 9, true},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			held, stale := entity.StaleInCycle(issueID, probe.changed, today, time.UTC)

			if stale != probe.stale {
				t.Fatalf("stale = %v, want %v", stale, probe.stale)
			}

			if !stale {
				return
			}

			if held.IssueID != issueID || held.Days != probe.days {
				t.Fatalf("held = %+v, want issue %v after %d days", held, issueID, probe.days)
			}
		})
	}
}

func TestTheDaysWithoutAStatusChangeAreCountedInTheWorkspaceZone(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("Asia/Tokyo is not available: %v", err)
	}

	changed := time.Date(2026, time.August, 6, 23, 30, 0, 0, time.UTC)

	inTokyo, err := entity.CalendarDaysSince(changed, "2026-08-12", tokyo)
	if err != nil {
		t.Fatalf("in Tokyo: %v", err)
	}

	inUTC, err := entity.CalendarDaysSince(changed, "2026-08-12", time.UTC)
	if err != nil {
		t.Fatalf("in UTC: %v", err)
	}

	if inTokyo != 5 || inUTC != 6 {
		t.Fatalf(
			"days = %d in Tokyo and %d in UTC, want 5 and 6. Half past eleven at night in London "+
				"is already the next morning in Tokyo, so the team there has waited one day less.",
			inTokyo, inUTC,
		)
	}
}

func TestAShorterOrLongerDayDoesNotChangeTheCount(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin is not available: %v", err)
	}

	changed := time.Date(2026, time.March, 25, 9, 0, 0, 0, berlin)

	days, err := entity.CalendarDaysSince(changed, "2026-03-30", berlin)
	if err != nil {
		t.Fatalf("days: %v", err)
	}

	if days != 5 {
		t.Fatalf(
			"days = %d, want 5. The twenty-ninth is twenty-three hours long in Berlin that week, "+
				"and a calendar day is still a day.",
			days,
		)
	}
}

func TestWorkThatHasNotMovedYetIsNotCountedFromNothing(t *testing.T) {
	if _, stale := entity.StaleInCycle(uuid.New(), time.Time{}, "2026-08-12", time.UTC); stale {
		t.Fatal("an issue with no recorded movement was called stale, which invents a history it does not have")
	}

	ahead := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)

	if _, stale := entity.StaleInCycle(uuid.New(), ahead, "2026-08-12", time.UTC); stale {
		t.Fatal("an issue that moved after today was called stale")
	}
}
