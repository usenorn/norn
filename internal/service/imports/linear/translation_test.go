package linear_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestACycleEndsOnTheLastDayItCoversRatherThanOnTheDayTheNextOneBegins(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, staging(), held.source(), asking(entity.ImportCycle))

	opening := payloadOf[service.ImportCyclePayload](t, recordNamed(t, page, openingCycle))
	closing := payloadOf[service.ImportCyclePayload](t, recordNamed(t, page, closingCycle))

	if opening.StartsOn != "2026-01-05" || opening.EndsOn != "2026-01-18" {
		t.Errorf(
			"the cycle Linear runs from 2026-01-05 to 2026-01-19 arrives as %s to %s, want it to "+
				"end on 2026-01-18. Linear ends a cycle where the next one begins; a workspace cycle "+
				"here covers both its endpoints, so carrying the instant across unchanged claims a "+
				"day the cycle never held.",
			opening.StartsOn, opening.EndsOn,
		)
	}

	if opening.EndsOn >= closing.StartsOn {
		t.Fatalf(
			"cycle %d ends on %s and cycle %d starts on %s, so the two overlap. workspace_cycles "+
				"excludes overlaps on daterange(starts_on, ends_on, '[]'): the second of every "+
				"consecutive pair is refused by the database, which is half a backlog's cycles "+
				"failing one after another for a reason the report states as a constraint name.",
			opening.Number, opening.EndsOn, closing.Number, closing.StartsOn,
		)
	}

	if opening.ClosedOn != "2026-01-19" {
		t.Errorf(
			"the finished cycle closed on %q, want the day the source completed it. A cycle "+
				"imported as still open is one the team is asked to work in again.",
			opening.ClosedOn,
		)
	}

	if closing.ClosedOn != "" {
		t.Errorf(
			"the cycle the source has not completed claims to have closed on %q. A closing date "+
				"invented for an open cycle retires the one the team is working in right now.",
			closing.ClosedOn,
		)
	}

	if opening.Name != "Ingest" || opening.Number != 41 {
		t.Errorf(
			"the cycle carries name %q and number %d, want what Linear called it. A cycle here is "+
				"numbered by its team's own running count, so the source's name and number survive "+
				"only in the report and nowhere else.",
			opening.Name, opening.Number,
		)
	}
}

func TestAnIssueNobodyPrioritisedArrivesWithNoPriorityRatherThanTheLowestOne(t *testing.T) {
	levels := `{"issues":{"nodes":[
		{"id":"issue-p0","title":"Nobody prioritised this","priority":0,
		 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z",
		 "team":{"id":"` + engineeringTeam + `"}},
		{"id":"issue-p1","title":"Urgent","priority":1,
		 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z",
		 "team":{"id":"` + engineeringTeam + `"}},
		{"id":"issue-p2","title":"High","priority":2,
		 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z",
		 "team":{"id":"` + engineeringTeam + `"}},
		{"id":"issue-p3","title":"Medium","priority":3,
		 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z",
		 "team":{"id":"` + engineeringTeam + `"}},
		{"id":"issue-p4","title":"Low","priority":4,
		 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z",
		 "team":{"id":"` + engineeringTeam + `"}}
	],"pageInfo":{"hasNextPage":false}}}`

	held := standing(t).answering(map[string]string{"ImportIssues": levels})

	page := fetched(t, staging(), held.source(), asking(entity.ImportIssue))

	for _, level := range []struct {
		external string
		want     string
	}{
		{"issue-p0", ""},
		{"issue-p1", string(entity.IssuePriorityUrgent)},
		{"issue-p2", string(entity.IssuePriorityHigh)},
		{"issue-p3", string(entity.IssuePriorityMedium)},
		{"issue-p4", string(entity.IssuePriorityLow)},
	} {
		issue := payloadOf[service.ImportIssuePayload](t, recordNamed(t, page, level.external))

		if issue.Priority != level.want {
			t.Errorf(
				"%q arrives with priority %q, want %q. Linear numbers priority from one and keeps "+
					"zero for an issue nobody ranked, so a level read off zero puts a backlog nobody "+
					"triaged in front of the work somebody did.",
				level.external, issue.Priority, level.want,
			)
		}
	}
}

func TestOnlyOneLinkSurvivesBetweenAPairAndTheRestAreNamedRatherThanDropped(t *testing.T) {
	ctx, said := noted()
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, ctx, held.source(), asking(entity.ImportIssueRelation))

	if ids := externalIDs(page); len(ids) != 1 || ids[0] != duplicateLink {
		t.Fatalf(
			"the pair holding a related, a blocks and a duplicate link staged %v, want only %q. A "+
				"workspace here holds one link per pair: choosing here is what makes the preview "+
				"count what apply will create, and choosing nowhere leaves apply taking whichever "+
				"arrived first and refusing the rest.",
			ids, duplicateLink,
		)
	}

	kept := payloadOf[service.ImportIssueRelationPayload](t, page.Records[0])

	if kept.Kind != string(entity.IssueRelationDuplicates) {
		t.Fatalf(
			"the surviving link reads %q, want %q. Duplicate outranks blocks and blocks outranks "+
				"related because a duplicate is the strongest thing two issues can say about each "+
				"other: losing it leaves two open issues nobody is told are the same work.",
			kept.Kind, entity.IssueRelationDuplicates,
		)
	}

	if kept.Issue == "" || kept.Related == "" {
		t.Fatalf("the link joins %q to %q, and a link missing an end joins nothing", kept.Issue, kept.Related)
	}

	for _, lost := range []string{relatedLink, blockingLink} {
		if !strings.Contains(said.String(), lost) {
			t.Errorf(
				"the link %q was dropped without being named in the report. Somebody who set that "+
					"link up is the only person who can decide whether it mattered, and an import "+
					"that quietly loses it never gives them the chance.",
				lost,
			)
		}
	}
}

func TestALinkKindThisWorkspaceHasNoEquivalentForIsReportedRatherThanInvented(t *testing.T) {
	ctx, said := noted()
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, ctx, held.source(), asking(entity.ImportIssueRelation))

	for _, record := range page.Records {
		if record.ExternalID == unknownLink {
			t.Fatalf(
				"the link Linear calls \"similar\" was staged anyway as %s. Nothing here means "+
					"similar, so the row arrives claiming a kind the source never said or an empty "+
					"one the apply pass has to guess at.",
				record.Payload,
			)
		}

		if payloadOf[service.ImportIssueRelationPayload](t, record).Kind == "" {
			t.Fatalf(
				"link %q was staged with no kind at all. A relation with no kind is not a weaker "+
					"relation, it is a row the apply pass cannot create and reports as a fault of "+
					"the import.",
				record.ExternalID,
			)
		}
	}

	if !strings.Contains(said.String(), unknownLink) {
		t.Errorf(
			"the link kind this workspace cannot hold was left out of the report. A kind Linear " +
				"adds later disappears the same silent way, and the first sign of it is somebody " +
				"noticing a link they remember making is not there.",
		)
	}
}

func TestAWorkflowStateArrivesInTheCategoryItsBehaviourMatches(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, staging(), held.source(), asking(entity.ImportWorkflowState))

	for _, state := range []struct {
		external string
		want     entity.StateCategory
	}{
		{inflightState, entity.StateCategoryActive},
		{shippedState, entity.StateCategoryComplete},
	} {
		got := payloadOf[service.ImportWorkflowStatePayload](t, recordNamed(t, page, state.external))

		if got.Category != string(state.want) {
			t.Errorf(
				"the %q state arrives in category %q, want %q. Every count a workspace draws — what "+
					"is open, what shipped this cycle — is drawn on the category rather than the "+
					"name, so a state in the wrong one moves work that never moved.",
				got.Name, got.Category, state.want,
			)
		}

		if got.Team != engineeringTeam {
			t.Errorf("the %q state names team %q, want the team that owns it", got.Name, got.Team)
		}
	}
}
