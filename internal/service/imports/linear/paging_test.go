package linear_test

import (
	"slices"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func teamPage(id, cursor string, more bool) string {
	carry := "false"
	if more {
		carry = "true"
	}

	return `{"teams":{
		"nodes":[{"id":"` + id + `","key":"K","name":"` + id + `",
		          "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-06T09:00:00.000Z"}],
		"pageInfo":{"hasNextPage":` + carry + `,"endCursor":"` + cursor + `"}
	}}`
}

func TestAWalkFollowsTheSourcesOwnCursorAndStopsOnlyWhenTheSourceSaysThereIsNoMore(t *testing.T) {
	pages := map[string]string{
		"":           teamPage("team-one", "cursor-one", true),
		"cursor-one": teamPage("team-two", "cursor-two", true),
		"cursor-two": teamPage("team-three", "cursor-three", false),
	}

	held := standing(t)

	held.replying(func(call graphCall) string {
		after, _ := call.Variables["after"].(string)

		data, known := pages[after]
		if !known {
			t.Errorf(
				"the adapter resumed from %q, which is not a cursor this source ever handed out. A "+
					"cursor the source did not write addresses nothing, and the page it answers with "+
					"is either the start again or an error.",
				after,
			)

			return `{"data":{}}`
		}

		return `{"data":` + data + `}`
	})

	source := held.source()
	request := asking(entity.ImportTeam)
	walked := make([]string, 0, 3)

	for turn := 1; turn <= 4; turn++ {
		page := fetched(t, staging(), source, request)

		walked = append(walked, externalIDs(page)...)

		if page.Done != (turn == 3) {
			t.Fatalf(
				"page %d reports done = %v. The framework stops staging a resource the moment a page "+
					"says it is done, so calling the last page early leaves the rest of the backlog "+
					"unimported with nothing recorded as missing.",
				turn, page.Done,
			)
		}

		if page.Done {
			break
		}

		if page.NextCursor == "" {
			t.Fatalf(
				"page %d carries no next cursor. A resource that is not done and has nowhere to "+
					"resume from restarts at the first page on the next slice and never advances.",
				turn,
			)
		}

		request.Cursor = page.NextCursor
	}

	if want := []string{"team-one", "team-two", "team-three"}; !slices.Equal(walked, want) {
		t.Fatalf("the walk staged %v, want %v", walked, want)
	}

	calls := held.seen()

	if len(calls) != 3 {
		t.Fatalf("the walk made %d calls, want one per page", len(calls))
	}

	if _, sent := calls[0].Variables["after"]; sent {
		t.Errorf(
			"the first page was asked for with after = %v. A run that has not started holds no "+
				"cursor, and an empty one handed to the source is a value it has to interpret rather "+
				"than the absence the schema expects.",
			calls[0].Variables["after"],
		)
	}

	for index, want := range []string{"cursor-one", "cursor-two"} {
		if got := calls[index+1].Variables["after"]; got != want {
			t.Errorf(
				"page %d was asked for from %v, want the %q the previous page ended on. A cursor the "+
					"adapter invents rather than carries skips or repeats whatever sits between them.",
				index+2, got, want,
			)
		}
	}
}

func TestAResumedWalkAsksTheSourceForExactlyTheCursorItWasHandedBack(t *testing.T) {
	held := standing(t).answering(map[string]string{
		"ImportIssues": `{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}`,
	})

	request := asking(entity.ImportIssue)
	request.Cursor = "cursor-two"

	fetched(t, staging(), held.source(), request)

	if after := held.only().Variables["after"]; after != "cursor-two" {
		t.Fatalf(
			"the resumed slice asked the source for after = %v, want %q. A staging pass is broken "+
				"into slices by a lease it keeps renewing, so nearly every page of a real import is "+
				"a resumption: an adapter that drops the cursor re-stages page one forever.",
			after, "cursor-two",
		)
	}
}

func TestThePageSizeTheFrameworkAsksForIsWhatTheSourceIsAsked(t *testing.T) {
	for _, asked := range []struct {
		name string
		hint int
		want int
	}{
		{"the framework names a size", 7, 7},
		{"the framework names none", 0, defaultPageSize},
		{"the framework asks for more than the source will answer", 5000, 250},
	} {
		t.Run(asked.name, func(t *testing.T) {
			held := standing(t).paging(defaultPageSize).answering(map[string]string{
				"ImportComments": `{"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}}}`,
			})

			request := asking(entity.ImportComment)
			request.PageHint = asked.hint

			fetched(t, staging(), held.source(), request)

			if first := numberIn(t, held.only(), "first"); first != asked.want {
				t.Fatalf(
					"the source was asked for %d rows where the framework hinted %d and wanted %d. "+
						"The hint is how a run whose slices keep expiring asks for smaller pages, and "+
						"a source deaf to it stalls on the same oversized page every attempt.",
					first, asked.hint, asked.want,
				)
			}
		})
	}
}
