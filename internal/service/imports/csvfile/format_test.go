package csvfile_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

func TestAMarkTheEditorLeftInFrontOfTheFileIsNotPartOfTheFirstColumnsName(t *testing.T) {
	held := standing(t, "\ufeffTitle,Status\nShip the thing,Open\n")

	catalogue := probed(t, held.source(), csvfile.Settings{})

	if len(catalogue.Columns) == 0 {
		t.Fatal("the probe read no columns at all")
	}

	if catalogue.Columns[0].Header != "Title" {
		t.Errorf(
			"the first column is named %q. A byte order mark is invisible in every editor that "+
				"writes one, so left on the front of the header it turns Title into a name nothing "+
				"matches and the column that holds every issue's name proposes nothing.",
			catalogue.Columns[0].Header,
		)
	}

	if catalogue.Columns[0].Proposed != "title" {
		t.Errorf(
			"the first column proposes %q rather than the title. The header reads Title once the "+
				"mark is gone.",
			catalogue.Columns[0].Proposed,
		)
	}

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))

	if titles := titlesIn(t, page); !slices.Equal(titles, []string{"Ship the thing"}) {
		t.Errorf("the rows staged as %v, want the one row the file holds", titles)
	}
}

func TestAFileSavedAsUTF16IsRefusedWithTheOneInstructionThatFixesIt(t *testing.T) {
	body := "\xff\xfeT\x00i\x00t\x00l\x00e\x00\n\x00"

	_, err := standing(t, body).source().Probe(staging(), configured(t, csvfile.Settings{}))
	if err == nil {
		t.Fatal(
			"a UTF-16 file was accepted. Every one of its rows reads as a single column of " +
				"interleaved nulls, so the run would stage a backlog of unreadable titles rather " +
				"than stop at the one thing somebody can fix.",
		)
	}

	saying(t, refusal(t, err).Reason, "UTF-8")
}

func TestAnExportSeparatedBySemicolonsIsReadAsColumnsRatherThanOneLongField(t *testing.T) {
	held := standing(t, "Title;Status;Labels\nRegler la facture;Ouvert;compta\n")

	catalogue := probed(t, held.source(), csvfile.Settings{})

	if len(catalogue.Columns) != 3 {
		t.Fatalf(
			"the probe read %d columns, want 3. A spreadsheet in a locale where the comma is the "+
				"decimal point writes semicolons, and read as commas the whole row is one field: "+
				"nothing matches a header, every column proposes ignore, and the import is empty.",
			len(catalogue.Columns),
		)
	}

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))
	issues := issuesIn(t, page)

	if len(issues) != 1 {
		t.Fatalf("the file staged %d rows, want 1", len(issues))
	}

	for _, payload := range issues {
		if payload.Title != "Regler la facture" || payload.State != "Ouvert" {
			t.Errorf(
				"the row staged as title %q and state %q, want the three cells the file separated "+
					"with semicolons",
				payload.Title, payload.State,
			)
		}
	}

	note := strings.Join(catalogue.Notes, " ")

	if !strings.Contains(note, "semicolon") {
		t.Errorf(
			"the catalogue says %q and never names the separator it guessed. The guess is the one "+
				"thing a person has to overrule when it is wrong, and they cannot overrule what they "+
				"were never told.",
			note,
		)
	}
}

func TestASettingsChoiceOfSeparatorOverrulesWhatTheFirstLineLooksLike(t *testing.T) {
	held := standing(t, "Title|Status\nShip it|Open\n")

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{Delimiter: "|"}))

	if titles := titlesIn(t, page); !slices.Equal(titles, []string{"Ship it"}) {
		t.Errorf(
			"the rows staged as %v with a pipe named in settings. A guess read off one line is a "+
				"guess, and the run has to be able to say otherwise.",
			titles,
		)
	}
}

func TestACellThatWouldBeAFormulaSomewhereElseIsStoredExactlyAsItArrived(t *testing.T) {
	held := standing(t, "Title,Description\n-1,=SUM(A1:A2)\n")

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))

	if len(page.Records) != 1 {
		t.Fatalf("the file staged %d rows, want 1", len(page.Records))
	}

	payload := payloadOf[service.ImportIssuePayload](t, page.Records[0])

	if payload.Title != "-1" || payload.Description != "=SUM(A1:A2)" {
		t.Errorf(
			"the row staged as title %q and description %q. Norn never writes a CSV back out and "+
				"renders bodies through DOMPurify, so nothing here executes a formula; sanitising "+
				"the cell would corrupt an issue actually named -1 to answer a risk this "+
				"application does not create.",
			payload.Title, payload.Description,
		)
	}
}

func TestAFileWhoseFirstRowIsAlreadyDataSaysSoRatherThanEatingIt(t *testing.T) {
	held := standing(t, "1,Ship it,Open\n2,Fix it,Done\n")

	catalogue := probed(t, held.source(), csvfile.Settings{})

	said := strings.Join(catalogue.Notes, " ")

	if !strings.Contains(said, "does not read as the names of the columns") {
		t.Errorf(
			"the catalogue says %q. A file exported without a header row loses its first issue to "+
				"whoever assumed there was one, and the count and the column names read off that "+
				"row are both fiction.",
			said,
		)
	}

	for _, column := range catalogue.Columns {
		if column.Proposed != "ignore" {
			t.Errorf(
				"column %d proposes %q from a row that is data. A proposal made from an issue's own "+
					"title reads as a mapping somebody can accept, and accepting it drops that issue.",
				column.Index, column.Proposed,
			)
		}
	}

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{
		Columns: []csvfile.Column{{Index: 1, Target: "title"}, {Index: 2, Target: "state"}},
	}))

	if titles := titlesIn(t, page); !slices.Equal(titles, []string{"Ship it", "Fix it"}) {
		t.Errorf(
			"the rows staged as %v, want both rows of a file that never had a header. The first "+
				"row of a headerless file is an issue, not a set of names.",
			titles,
		)
	}
}

func TestTheFileIsReadInBitesLargeEnoughThatStorageIsNotAskedPerLine(t *testing.T) {
	const rows = 1000

	body := strings.Builder{}
	body.WriteString("Title,Status\n")

	for row := range rows {
		body.WriteString("Issue number " + strconv.Itoa(row) + " of a real backlog,Open\n")
	}

	held := standing(t, body.String())

	request := asking(t, entity.ImportIssue, csvfile.Settings{})
	request.PageHint = rows

	page := fetched(t, held.source(), request)

	if len(page.Records) != rows {
		t.Fatalf("the page staged %d rows, want %d", len(page.Records), rows)
	}

	if reads := held.reads(); reads > 4 {
		t.Errorf(
			"reading %d bytes took %d calls to storage. The reader underneath is a range reader "+
				"that issues one GET per Read, so encoding/csv's own 4 KB buffer turns a 25 MB "+
				"upload into thousands of HTTP requests: the file is wrapped in a megabyte before "+
				"csv ever sees it.",
			body.Len(), reads,
		)
	}

	if open := held.leftOpen(); open != 0 {
		t.Errorf("%d objects were left open after the page was staged", open)
	}
}

func TestAFileOutsideTheStorageThisRunOwnsIsNeverOpened(t *testing.T) {
	held := standing(t, "Title\nShip it\n")

	_, err := held.source().Fetch(staging(), asking(t, entity.ImportIssue, csvfile.Settings{
		ObjectKey: "attachments/" + stagingWorkspace.String() + "/somebody-elses-file",
	}))
	if err == nil {
		t.Fatal(
			"a key naming an object outside this run's own prefix was opened. The key arrives in " +
				"settings from whoever configured the run, and handed to storage unchecked it " +
				"addresses every object in the bucket.",
		)
	}

	saying(t, refusal(t, err).Reason, "outside the storage")
}
