package cycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type reported struct {
	harness *harness
	cycle   entity.Cycle
	asked   []uuid.UUID
	scope   entity.TeamScope
}

func (r *reported) wasAsked(t *testing.T, issueIDs ...uuid.UUID) {
	t.Helper()

	held := map[uuid.UUID]bool{}
	for _, issueID := range r.asked {
		held[issueID] = true
	}

	for _, issueID := range issueIDs {
		if !held[issueID] {
			t.Fatalf(
				"the history was read for %v, which leaves out %v. Every issue the ledgers or "+
					"the snapshot name is a candidate, whatever cycle it sits in now.",
				r.asked, issueID,
			)
		}
	}
}

func (r *reported) expect(
	members []entity.Issue,
	results []entity.CycleResult,
	changes []entity.CycleScopeChange,
	visible []entity.Issue,
) {
	h := r.harness

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, r.cycle.ID, gomock.Any()).
		Return(r.cycle, nil)
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(members, nil)
	h.scope.EXPECT().
		ListByCycleID(gomock.Any(), r.cycle.ID, gomock.Any()).
		Return(changes, nil)
	h.results.EXPECT().
		ListByCycleID(gomock.Any(), r.cycle.ID).
		Return(results, nil)
	allowed := map[uuid.UUID]entity.Issue{}
	for _, issue := range visible {
		allowed[issue.ID] = issue
	}

	h.issues.EXPECT().
		ListVisibleByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, scope entity.TeamScope, issueIDs []uuid.UUID,
		) ([]entity.Issue, error) {
			r.asked = issueIDs
			r.scope = scope

			found := make([]entity.Issue, 0, len(issueIDs))

			for _, issueID := range issueIDs {
				if issue, ok := allowed[issueID]; ok {
					found = append(found, issue)
				}
			}

			return found, nil
		})
	h.activity.EXPECT().
		ListStateChanges(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
}

func (r *reported) read(t *testing.T) service.CycleReport {
	t.Helper()

	report, err := r.harness.service.Report(context.Background(), r.harness.workspaceID, r.cycle.ID)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	return report
}

func yesterdayCycle(h *harness) (string, string) {
	start := time.Now().UTC().AddDate(0, 0, -10)

	return entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 6), "UTC")
}

func closedResult(cycleID uuid.UUID, issue entity.Issue, decision entity.CycleRollover) entity.CycleResult {
	return entity.CycleResult{
		CycleID:   cycleID,
		IssueID:   issue.ID,
		TeamID:    issue.TeamID,
		Category:  issue.State.Category,
		StateName: issue.State.Name,
		Decision:  decision,
	}
}

func TestAClosedCycleStillReportsTheWorkItSentElsewhere(t *testing.T) {
	for _, decision := range []entity.CycleRollover{
		entity.CycleRolloverNext, entity.CycleRolloverBacklog, entity.CycleRolloverKeep,
	} {
		t.Run(string(decision), func(t *testing.T) {
			h := newHarness(t)
			h.allowAnything()

			closedAt := time.Now().UTC()
			start := time.Now().UTC().AddDate(0, 0, -10)
			ended := h.cycle(9, entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 6), "UTC"))
			ended.ClosedAt = &closedAt
			ended.ResultsRecordedAt = &closedAt

			moved := h.issue(entity.StateCategoryActive)
			moved.CreatedAt = start

			r := &reported{harness: h, cycle: ended}
			r.expect(
				[]entity.Issue{},
				[]entity.CycleResult{closedResult(ended.ID, moved, decision)},
				nil,
				[]entity.Issue{moved},
			)

			report := r.read(t)

			r.wasAsked(t, moved.ID)

			if len(report.Results) != 1 || report.Results[0].IssueID != moved.ID {
				t.Fatalf(
					"the closed cycle reports %d results, want the one it held. An issue that "+
						"went to the next cycle or the backlog is no longer a member of this "+
						"one, so reading the report off current membership erases the very work "+
						"the snapshot exists to remember.",
					len(report.Results),
				)
			}

			if len(report.Issues) != 1 || report.Issues[0].ID != moved.ID {
				t.Fatalf("the closed cycle shows %d issues, want the one its snapshot names", len(report.Issues))
			}
		})
	}
}

func TestAnIssueRemovedMidCycleStillStandsOnTheDaysItWasThere(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	start := time.Now().UTC().AddDate(0, 0, -6)
	running := h.cycle(9, entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 13), "UTC"))

	dropped := h.issue(entity.StateCategoryActive)
	dropped.CreatedAt = start.AddDate(0, 0, -1)

	left := start.AddDate(0, 0, 3)

	r := &reported{harness: h, cycle: running}
	r.expect(
		[]entity.Issue{},
		nil,
		[]entity.CycleScopeChange{{
			CycleID:   running.ID,
			IssueID:   dropped.ID,
			Change:    entity.CycleScopeChangeRemoved,
			ChangedAt: left,
		}},
		[]entity.Issue{dropped},
	)

	report := r.read(t)

	r.wasAsked(t, dropped.ID)

	if len(report.Burndown.Points) == 0 {
		t.Fatal("the chart has no days at all")
	}

	if report.Burndown.Points[0].Scope != 1 {
		t.Fatalf(
			"the first day counts %d issues, want one. Work taken out of a cycle on the fourth "+
				"day was genuinely in it for the first three, and a chart drawn only from what "+
				"is a member now cannot say that.",
			report.Burndown.Points[0].Scope,
		)
	}

	last := report.Burndown.Points[len(report.Burndown.Points)-1]

	if last.Scope != 0 {
		t.Fatalf("the last day counts %d issues, want none: it had left by then", last.Scope)
	}

	if len(report.Issues) != 0 {
		t.Fatalf(
			"the running cycle lists %d issues, want none. What it holds today is its membership; "+
				"the chart is what remembers the rest.",
			len(report.Issues),
		)
	}
}

func TestWorkOnATeamTheReaderCannotSeeIsAbsentFromEveryPartOfTheReport(t *testing.T) {
	h := newHarness(t)
	h.allowOnlyOwnTeam()

	closedAt := time.Now().UTC()
	start := time.Now().UTC().AddDate(0, 0, -10)
	ended := h.cycle(9, entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 6), "UTC"))
	ended.ClosedAt = &closedAt
	ended.ResultsRecordedAt = &closedAt

	hidden := h.issue(entity.StateCategoryComplete)
	hidden.TeamID = uuid.New()

	r := &reported{harness: h, cycle: ended}
	r.expect(
		[]entity.Issue{},
		[]entity.CycleResult{closedResult(ended.ID, hidden, entity.CycleRolloverNone)},
		nil,
		[]entity.Issue{},
	)

	report := r.read(t)

	r.wasAsked(t, hidden.ID)

	if r.scope.AllTeams || len(r.scope.TeamIDs) != 1 || r.scope.TeamIDs[0] != h.teamID {
		t.Fatalf(
			"the history was read with the scope %+v, want only the reader's own team. Reading "+
				"the candidates back is the one place the reader's reach is applied, so widening "+
				"it here would hand over exactly what the rest of the report withholds.",
			r.scope,
		)
	}

	if len(report.Results) != 0 || len(report.Issues) != 0 {
		t.Fatalf(
			"the report carries %d results and %d issues about work the reader may not see. A "+
				"frozen row is still a row about an issue, and an issue that has moved to a "+
				"private team since is refused everywhere else.",
			len(report.Results), len(report.Issues),
		)
	}

	for _, point := range report.Burndown.Points {
		if point.Scope != 0 {
			t.Fatalf("the chart counts %d on %s, want nothing the reader may not see", point.Scope, point.On)
		}
	}
}

func TestTheReportRefusesRatherThanGuessingTheDayWhenTheWorkspaceCannotBeRead(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	starts, ends := yesterdayCycle(h)
	running := h.cycle(9, starts, ends)

	unreachable := errors.New("workspace store is unavailable")

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, running.ID, gomock.Any()).
		Return(running, nil)

	h.workspaceErr = unreachable

	_, err := h.service.Report(context.Background(), h.workspaceID, running.ID)

	if !errors.Is(err, unreachable) {
		t.Fatalf(
			"Report error = %v, want the store's failure. The team's timezone decides where each "+
				"day of the chart ends, so falling back to UTC would answer confidently with "+
				"work on the wrong days.",
			err,
		)
	}
}

func TestDaysBeforeTheRecordersAreUnknownAndLaterDaysAreDrawn(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	start := time.Now().UTC().AddDate(0, 0, -6)
	running := h.cycle(9, entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 13), "UTC"))

	h.recordingFrom = start.AddDate(0, 0, 3)

	imported := h.issue(entity.StateCategoryActive)
	imported.CreatedAt = start.AddDate(0, 0, -90)

	r := &reported{harness: h, cycle: running}
	r.expect([]entity.Issue{imported}, nil, nil, []entity.Issue{imported})

	report := r.read(t)

	if report.Burndown.Whole() {
		t.Fatal(
			"the chart claims every day. This cycle was already running before anything was " +
				"written down: an issue imported into it on the fourth day and one planned in " +
				"before the first look identical in an empty ledger, and the issue's creation " +
				"date says only when it existed, never when it joined this cycle.",
		)
	}

	if report.Burndown.Points[0].Known() {
		t.Error("the first day is drawn as certain, but nothing was recording then")
	}

	last := report.Burndown.Points[len(report.Burndown.Points)-1]

	if !last.Known() || last.Remaining != 1 {
		t.Fatalf(
			"the last day reports Known() = %v with %d remaining, want a drawn day counting the "+
				"issue. Once the recorders are running, an empty ledger for a current member is "+
				"evidence that it stayed a member.",
			last.Known(), last.Remaining,
		)
	}
}

func TestAnArrivalKeepsImportedWorkOffTheDaysBeforeItWasSeen(t *testing.T) {
	h := newHarness(t)
	h.allowAnything()

	start := time.Now().UTC().AddDate(0, 0, -6)
	running := h.cycle(9, entity.Today(start, "UTC"), entity.Today(start.AddDate(0, 0, 13), "UTC"))

	h.recordingFrom = start.AddDate(0, 0, -1)

	imported := h.issue(entity.StateCategoryActive)
	imported.CreatedAt = start.AddDate(0, -3, 0)

	seen := start.AddDate(0, 0, 3)

	r := &reported{harness: h, cycle: running}
	r.expect(
		[]entity.Issue{imported},
		nil,
		[]entity.CycleScopeChange{{
			CycleID:   running.ID,
			IssueID:   imported.ID,
			Change:    entity.CycleScopeChangeAdded,
			ChangedAt: seen,
		}},
		[]entity.Issue{imported},
	)

	report := r.read(t)

	if !report.Burndown.Whole() {
		t.Fatalf("the chart reports %d unreadable days, want none inside the records", report.Burndown.Unreadable())
	}

	if report.Burndown.Points[0].Scope != 0 || report.Burndown.Points[1].Scope != 0 {
		t.Fatalf(
			"the first two days count %d and %d issues, want none. The issue was created three "+
				"months earlier somewhere else, and the day Norn saw it in this cycle is the "+
				"only day it can honestly be counted from.",
			report.Burndown.Points[0].Scope, report.Burndown.Points[1].Scope,
		)
	}

	last := report.Burndown.Points[len(report.Burndown.Points)-1]

	if last.Scope != 1 || last.Remaining != 1 {
		t.Fatalf("the last day counts %d in scope and %d remaining, want one of each", last.Scope, last.Remaining)
	}
}
