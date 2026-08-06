package csvfile_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

const exportedBacklog = "ID,Summary,Body,Owner,Status,Tags,Priority,Points,Due Date,Parent,Created At\n" +
	"ABC-1,Ship the thing,It has to go out,ann@northwind.co,In Progress,\"bug, ui\",High,3,2026-09-01,,2026-08-01\n" +
	"ABC-2,Fix the other thing,,bob@northwind.co,Todo,ui,Low,1,,ABC-1,2026-08-02\n"

func proposalFor(t *testing.T, catalogue entity.ImportCatalogue, header string) entity.ImportColumn {
	t.Helper()

	for _, column := range catalogue.Columns {
		if strings.EqualFold(column.Header, header) {
			return column
		}
	}

	t.Fatalf("the catalogue holds no column named %q", header)

	return entity.ImportColumn{}
}

func TestTheCatalogueProposesWhatAnOrdinaryExportsHeadersMean(t *testing.T) {
	catalogue := probed(t, standing(t, exportedBacklog).source(), csvfile.Settings{})

	wanted := map[string]string{
		"ID":         "id",
		"Summary":    "title",
		"Body":       "description",
		"Owner":      "assignee",
		"Status":     "state",
		"Tags":       "labels",
		"Priority":   "priority",
		"Points":     "estimate",
		"Due Date":   "due",
		"Parent":     "parent",
		"Created At": "created",
	}

	for header, target := range wanted {
		column := proposalFor(t, catalogue, header)

		if column.Proposed != target {
			t.Errorf(
				"%q proposes %q, want %q. The proposal is what somebody accepts or changes in one "+
					"screen; a column left proposing nothing is a field of every issue in the file "+
					"silently dropped by whoever accepted the page as it was.",
				header, column.Proposed, target,
			)
		}

		if column.Confidence == "" {
			t.Errorf("%q proposes a target and says nothing about how sure it is", header)
		}
	}

	if said := strings.Join(catalogue.Notes, " "); !strings.Contains(said, "team") {
		t.Errorf(
			"the catalogue says %q and never mentions the team. A file names no team, one stands "+
				"in for all of it, and a run that never maps it imports nothing at all.",
			said,
		)
	}
}

func TestTheCatalogueSaysHowManyRowsItWasAbleToSee(t *testing.T) {
	catalogue := probed(t, standing(t, exportedBacklog).source(), csvfile.Settings{})

	if said := strings.Join(catalogue.Notes, " "); !strings.Contains(said, "2 rows") {
		t.Errorf(
			"the catalogue says %q. Two data rows sit under the header, and a count read back "+
				"before anything is staged is how somebody notices they uploaded the wrong export.",
			said,
		)
	}
}

func TestAColumnNothingRecognisesProposesToBeLeftAlone(t *testing.T) {
	catalogue := probed(t, standing(t, "Title,Sprint velocity\nShip it,4\n").source(), csvfile.Settings{})

	if column := proposalFor(t, catalogue, "Sprint velocity"); column.Proposed != "ignore" {
		t.Errorf(
			"a column nothing here recognises proposes %q. Guessing at a header nobody matched "+
				"puts a column of unrelated numbers into an issue's estimate.",
			column.Proposed,
		)
	}
}

func TestARunMayReadColumnsItWasToldAboutRatherThanTheOnesTheHeaderNames(t *testing.T) {
	held := standing(t, exportedBacklog)

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{
		Columns: []csvfile.Column{{Index: 2, Target: "title"}},
	}))

	issues := issuesIn(t, page)

	first, staged := issues["row:2"]
	if !staged {
		t.Fatalf("nothing was staged for the first row; the page holds %v", externalIDs(page))
	}

	if first.Title != "It has to go out" {
		t.Errorf(
			"the first row was titled %q. A header match is a proposal, and the run's own answer "+
				"to it is what the rows are read with.",
			first.Title,
		)
	}
}

func TestAnExportsFieldsArriveOnTheIssueTheyBelongTo(t *testing.T) {
	held := standing(t, exportedBacklog)

	page := fetched(t, held.source(), asking(t, entity.ImportIssue, csvfile.Settings{}))
	issues := issuesIn(t, page)

	first, staged := issues["ABC-1"]
	if !staged {
		t.Fatalf(
			"the first row was not staged under the identifier its own file gave it; the page "+
				"holds %v. The parent column names rows by that identifier, so a row addressed by "+
				"anything else leaves every hierarchy in the file pointing at nothing.",
			externalIDs(page),
		)
	}

	if first.Title != "Ship the thing" || first.State != "In Progress" || first.Priority != "High" {
		t.Errorf("the first row staged as %+v", first)
	}

	if first.Estimate != 3 {
		t.Errorf("the first row's estimate is %d, want the 3 points its own column holds", first.Estimate)
	}

	if first.DueOn != "2026-09-01" {
		t.Errorf(
			"the first row is due %q. An issue's due date is stored as a calendar day, and "+
				"anything else is refused when the row is applied.",
			first.DueOn,
		)
	}

	if first.Assignee.Email != "ann@northwind.co" {
		t.Errorf(
			"the first row's assignee arrived as %+v. The address is what suggests a member of "+
				"this workspace to map the person onto.",
			first.Assignee,
		)
	}

	if record := recordNamed(t, page, "ABC-2"); record.ParentExternalID != "ABC-1" {
		t.Errorf(
			"the second row names parent %q, want ABC-1. The hierarchy is derived from the parent "+
				"a row names, so a parent left off the record is a sub-issue imported as a "+
				"top-level one.",
			record.ParentExternalID,
		)
	}

	if record := recordNamed(t, page, "ABC-1"); record.SourceCreatedAt == nil {
		t.Error(
			"the first row arrived with no creation date. An import that keeps the day a row was " +
				"opened is the difference between a backlog and a list of things created today.",
		)
	}
}

func recordNamed(
	t *testing.T,
	page service.ImportFetchPage,
	externalID string,
) entity.ImportRecord {
	t.Helper()

	for _, record := range page.Records {
		if record.ExternalID == externalID {
			return record
		}
	}

	t.Fatalf("no record for %q; the page holds %v", externalID, externalIDs(page))

	return entity.ImportRecord{}
}
