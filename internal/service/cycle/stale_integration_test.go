package cycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
)

func (l *live) recorded(t *testing.T, issue entity.Issue, activity entity.Activity) {
	t.Helper()

	activity.WorkspaceID = l.workspace.ID
	activity.Subject = entity.IssueSubject(issue.ID)
	activity.Actor = entity.ActivityAttribution{Kind: entity.ActorKindUser, AccountID: l.account}

	if err := activityrepo.New(l.client).Record(context.Background(), activity); err != nil {
		t.Fatalf("record %s: %v", activity.Kind, err)
	}
}

func TestOnlyAStatusChangeStopsAnIssueLookingStale(t *testing.T) {
	client := scratchDatabase(t)
	cycles, world := liveCycles(t, client)

	now := time.Now().UTC()
	running := world.cycleOn(
		t, 1, entity.Today(now.AddDate(0, 0, -8), "UTC"), entity.Today(now.AddDate(0, 0, 5), "UTC"),
	)

	todo, doing := world.states[0], world.states[1]

	untouched := world.issueIn(t, running, todo)
	world.recorded(t, untouched, entity.Activity{
		Kind: entity.ActivityKindStateChanged, FromState: "Todo", ToState: "Doing",
		FromCategory: todo.Category, ToCategory: doing.Category,
		CreatedAt: now.AddDate(0, 0, -8),
	})
	world.recorded(t, untouched, entity.Activity{
		Kind: entity.ActivityKindCommented, CreatedAt: now.AddDate(0, 0, -1),
	})
	world.recorded(t, untouched, entity.Activity{
		Kind: entity.ActivityKindPropertyChanged, Field: "description",
		FromValue: "before", ToValue: "after", CreatedAt: now.AddDate(0, 0, -1),
	})
	world.recorded(t, untouched, entity.Activity{
		Kind: entity.ActivityKindStateReclassified, FromState: "Doing", ToState: "Doing",
		FromCategory: entity.StateCategoryNotStarted, ToCategory: doing.Category,
		CreatedAt: now.AddDate(0, 0, -1),
	})

	sideways := world.issueIn(t, running, doing)
	world.recorded(t, sideways, entity.Activity{
		Kind: entity.ActivityKindStateChanged, FromState: "Todo", ToState: "Doing",
		FromCategory: todo.Category, ToCategory: doing.Category,
		CreatedAt: now.AddDate(0, 0, -8),
	})
	world.recorded(t, sideways, entity.Activity{
		Kind: entity.ActivityKindStateChanged, FromState: "Doing", ToState: "In review",
		FromCategory: doing.Category, ToCategory: doing.Category,
		CreatedAt: now.AddDate(0, 0, -1),
	})

	report, err := cycles.Report(context.Background(), world.workspace.ID, running.ID)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	days := map[uuid.UUID]int{}
	for _, held := range report.Stale {
		days[held.IssueID] = held.Days
	}

	if days[untouched.ID] != 8 {
		t.Fatalf(
			"the issue nobody has moved for eight days is %d days stale in the report, want eight. "+
				"A comment, a description edit and an administrator reclassifying the state say "+
				"nothing about whether the work itself moved.",
			days[untouched.ID],
		)
	}

	if _, held := days[sideways.ID]; held {
		t.Fatal(
			"an issue moved from one active status to another yesterday is still called stale. " +
				"A move between two statuses of the same category is a move, and the recorded " +
				"category timestamp cannot see it.",
		)
	}
}
