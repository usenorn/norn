package csvfile_test

import (
	"slices"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

const fiveRows = "Title,Status\n" +
	"First,Open\n" +
	"Second,Open\n" +
	"Third,\"a title\nover two lines\"\n" +
	"Fourth,Open\n" +
	"Fifth,Done\n"

func TestAResumedWalkStartsOnTheRowAfterTheOneTheLastPageStoppedOn(t *testing.T) {
	held := standing(t, fiveRows)
	source := held.source()

	request := asking(t, entity.ImportIssue, csvfile.Settings{})
	request.PageHint = 2

	walked := make([]string, 0, 5)

	for turn := 1; turn <= 4; turn++ {
		page := fetched(t, source, request)

		walked = append(walked, titlesIn(t, page)...)

		if page.Done {
			break
		}

		if page.NextCursor == "" {
			t.Fatalf(
				"page %d is not done and hands back nowhere to resume from. A staging pass is cut "+
					"into slices by the lease it keeps renewing, so a walk with no cursor restages "+
					"the first rows forever and never reaches the end of the file.",
				turn,
			)
		}

		request.Cursor = page.NextCursor
	}

	want := []string{"First", "Second", "Third", "Fourth", "Fifth"}

	if !slices.Equal(walked, want) {
		t.Fatalf(
			"the walk staged %v, want %v. The offset a page ends on is the reader's own position "+
				"after a completed record, so resuming lands between two rows rather than inside "+
				"the quoted title that spans two lines.",
			walked, want,
		)
	}
}

func TestARowIsStagedUnderTheSameNameWhicheverPageItArrivesOn(t *testing.T) {
	held := standing(t, fiveRows)
	source := held.source()

	whole := asking(t, entity.ImportIssue, csvfile.Settings{})
	all := externalIDs(fetched(t, source, whole))

	paged := asking(t, entity.ImportIssue, csvfile.Settings{})
	paged.PageHint = 2

	first := fetched(t, source, paged)
	paged.Cursor = first.NextCursor

	second := fetched(t, source, paged)

	carried := append(externalIDs(first), externalIDs(second)...)

	if !slices.Equal(carried, all[:len(carried)]) {
		t.Fatalf(
			"the same rows are named %v when they arrive two at a time and %v when they arrive at "+
				"once. A record is addressed by its identifier, so a row that changes name between "+
				"two attempts of the same run is staged twice and imported twice.",
			carried, all[:len(carried)],
		)
	}
}

func TestACursorFromOnePassIsNeverReadAsAPositionInAnother(t *testing.T) {
	held := standing(t, fiveRows)
	source := held.source()

	labels := asking(t, entity.ImportLabel, csvfile.Settings{})
	labels.PageHint = 2

	page := fetched(t, source, labels)

	if page.NextCursor == "" {
		t.Fatal("the label pass exhausted a five row file in two rows")
	}

	issues := asking(t, entity.ImportIssue, csvfile.Settings{})
	issues.Cursor = page.NextCursor

	if _, err := source.Fetch(staging(), issues); err == nil {
		t.Fatal(
			"a cursor written by the label pass was accepted as a position in the issue pass. The " +
				"two passes are the same file walked twice and their offsets have nothing to do with " +
				"each other, so one read as the other silently skips or repeats rows.",
		)
	}
}
