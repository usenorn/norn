package csvfile_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

const brokenBacklog = "Title,Assignee,Labels\n" +
	"Ship the thing,ann@northwind.co,\"bug, ui\"\n" +
	"Short one,two\n" +
	"Fix the other thing,bob@northwind.co,ui\n" +
	"Bro\"ken,x,y\n" +
	"Land the last one,cara@northwind.co,ui\n"

func TestARowTheFileBrokeIsStagedCarryingItsOwnExplanationRatherThanEndingTheRun(t *testing.T) {
	held := standing(t, brokenBacklog)

	page, err := held.source().Fetch(staging(), asking(t, entity.ImportIssue, csvfile.Settings{}))
	if err != nil {
		t.Fatalf(
			"one unreadable row out of five ended the fetch with %v. An error out of Fetch fails "+
				"the whole run, so a backlog of ten thousand rows is lost to the one line somebody's "+
				"export quoted wrongly. A row that cannot be read is staged carrying its own "+
				"explanation and costs nothing but itself.",
			err,
		)
	}

	issues := issuesIn(t, page)

	for _, wanted := range []string{"Ship the thing", "Fix the other thing", "Land the last one"} {
		if !slices.Contains(titlesIn(t, page), wanted) {
			t.Errorf(
				"%q never arrived. The rows on either side of a broken one are readable and have "+
					"nothing to do with it.",
				wanted,
			)
		}
	}

	short, staged := issues["row:3"]
	if !staged {
		t.Fatalf("nothing was staged for the short row; the page holds %v", externalIDs(page))
	}

	if short.Defect == "" {
		t.Error(
			"the short row was staged as an ordinary issue. Without a defect the preview shows it " +
				"as an issue somebody is about to create, and what lands is a row of cells shifted " +
				"one column left.",
		)
	}

	if !strings.Contains(short.Defect, "row 3 has 2 fields, the header has 3") {
		t.Errorf(
			"the short row explains itself as %q. The count and the row number are the whole "+
				"repair instruction: a report that says only \"wrong number of fields\" sends "+
				"somebody looking through the file by hand.",
			short.Defect,
		)
	}

	quoted, staged := issues["row:5"]
	if !staged {
		t.Fatalf("nothing was staged for the row with the broken quote; the page holds %v", externalIDs(page))
	}

	if quoted.Defect == "" {
		t.Error(
			"the row with a bare quote in it was staged as an ordinary issue. A quote in the wrong " +
				"place changes where every field after it ends, so the issue created from it is " +
				"cells from two columns run together.",
		)
	}
}

func TestAQuoteLeftOpenToTheEndOfTheFileCostsOneRowAndEndsTheWalk(t *testing.T) {
	held := standing(t, "Title,Status\nShip it,Open\n\"never closed,Open\n")

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))

	if !page.Done {
		t.Error(
			"the page is not done at the end of a file that has no more readable rows in it. A " +
				"walk that never finishes asks for the same page again on every slice of the run.",
		)
	}

	if len(page.Records) != 2 {
		t.Fatalf(
			"the page staged %d records, want the readable row and the unreadable one. A quote "+
				"nothing ever closes swallows the rest of the file, and the rows it swallowed are "+
				"worth a line in the report rather than silence.",
			len(page.Records),
		)
	}

	if payloadOf[service.ImportIssuePayload](t, page.Records[1]).Defect == "" {
		t.Error("the row with the unclosed quote was staged as though it had been read")
	}
}

func TestEveryRowOfAFileWithNoTitleColumnIsRefusedOnceRatherThanSkippedOneByOne(t *testing.T) {
	held := standing(t, "Reference,Owner\nABC-1,ann@northwind.co\n")

	_, err := held.source().Fetch(staging(), asking(t, entity.ImportIssue, csvfile.Settings{}))
	if err == nil {
		t.Fatal(
			"a file with nothing that reads as a title was staged anyway. Every row of it would " +
				"reach the apply pass nameless, fail validation one at a time and land in a report " +
				"of thousands of identical refusals, when the column to map is the single thing " +
				"that was missing.",
		)
	}

	saying(t, refusal(t, err).Reason, "title")
}

func TestTwoRowsCarryingTheSameIdentifierAreBothStaged(t *testing.T) {
	held := standing(t, "Id,Title\nABC-1,Ship it\nABC-1,Ship it again\n")

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))

	if len(page.Records) != 2 {
		t.Fatalf("the page staged %d rows, want both", len(page.Records))
	}

	if page.Records[0].ExternalID == page.Records[1].ExternalID {
		t.Fatalf(
			"both rows were staged as %q. A page is written in one statement keyed on the "+
				"identifier, and Postgres refuses a statement that names the same key twice: the "+
				"page would fail outright rather than lose one row.",
			page.Records[0].ExternalID,
		)
	}
}

func TestADefectIsAllThatIsStagedForARowNothingCouldBeReadFrom(t *testing.T) {
	held := standing(t, "Title,Status\nShip it,Open\nonly one\n")

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))

	for _, record := range page.Records {
		payload := payloadOf[service.ImportIssuePayload](t, record)

		if payload.Defect == "" {
			continue
		}

		if payload.Title != "" || payload.State != "" {
			t.Errorf(
				"the record staged for %q carries a defect and a title of %q. Half a row read out "+
					"of a row that could not be read is a guess, and the guess is what the report "+
					"would name the issue by.",
				record.ExternalID, payload.Title,
			)
		}
	}
}
