package csvfile_test

import (
	"slices"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

const labelledBacklog = "Title,Labels\n" +
	"Ship it,\"bug, ui\"\n" +
	"Fix it,ui\n" +
	"Land it,\n" +
	"Close it,bug\n"

func TestEveryRowNamesTheOneTeamAFileIsAbleToOfferIt(t *testing.T) {
	held := standing(t, labelledBacklog)
	source := held.source()

	if !slices.Contains(source.Resources(), entity.ImportTeam) {
		t.Fatal(
			"this source never offers a team. An issue resolves its team through the mapping " +
				"plan, a blank team key resolves to nothing, and a row with no team is skipped by " +
				"the apply pass: without a team of its own, every row of every CSV ever imported " +
				"would be left behind for having nowhere to land.",
		)
	}

	teams := fetched(t, source, asking(t, entity.ImportTeam, csvfile.Settings{}))

	if len(teams.Records) != 1 {
		t.Fatalf(
			"the team pass staged %d records, want exactly one. A file names no team anywhere in "+
				"it, so one stands in for the whole of it and the run maps that one onto a team "+
				"that exists.",
			len(teams.Records),
		)
	}

	if !teams.Done {
		t.Error("the team pass is never done, so staging asks for a second page of a file's one team")
	}

	stood := teams.Records[0]

	if stood.ExternalID == "" {
		t.Fatal("the team record has no identifier, so nothing can be mapped onto it")
	}

	issues := fetched(t, source, asking(t, entity.ImportIssue, csvfile.Settings{}))

	for external, payload := range issuesIn(t, issues) {
		if payload.Team != stood.ExternalID {
			t.Fatalf(
				"%s names team %q while the file's team was staged as %q. The team an issue names "+
					"is looked up in the mapping plan by exactly that key, and one that matches "+
					"nothing resolves to no team at all.",
				external, payload.Team, stood.ExternalID,
			)
		}
	}
}

func TestTheTeamAFileIsGivenIsTheOneTheRunNamed(t *testing.T) {
	held := standing(t, labelledBacklog)

	page := fetched(t, held.source(), asking(t, entity.ImportTeam, csvfile.Settings{
		TeamKey:  "ops",
		TeamName: "Operations",
	}))

	payload := payloadOf[service.ImportTeamPayload](t, page.Records[0])

	if payload.Key != "OPS" || payload.Name != "Operations" {
		t.Errorf(
			"the file's team was staged as %q / %q. A run that chooses to create the team it is "+
				"importing into gets the name it typed, and a key a team cannot be created with "+
				"loses every row behind it.",
			payload.Key, payload.Name,
		)
	}
}

func TestOneLabelArrivesForEachDistinctValueTheFileHolds(t *testing.T) {
	held := standing(t, labelledBacklog)
	source := held.source()

	page := fetched(t, source, asking(t, entity.ImportLabel, csvfile.Settings{}))

	named := make([]string, 0, len(page.Records))

	for _, record := range page.Records {
		named = append(named, payloadOf[service.ImportLabelPayload](t, record).Name)
	}

	slices.Sort(named)

	if !slices.Equal(named, []string{"bug", "ui"}) {
		t.Fatalf(
			"the label pass staged %v, want one record for each distinct value. A label staged "+
				"twice in one page is a statement naming the same record twice, and one never "+
				"staged at all is a label the run can neither create nor skip.",
			named,
		)
	}

	if !page.Done {
		t.Error("the label pass never says it is done, so staging walks the file again for nothing")
	}

	issues := fetched(t, source, asking(t, entity.ImportIssue, csvfile.Settings{}))
	staged := make(map[string]bool, len(page.Records))

	for _, record := range page.Records {
		staged[record.ExternalID] = true
	}

	for external, payload := range issuesIn(t, issues) {
		for _, label := range payload.Labels {
			if !staged[label] {
				t.Errorf(
					"%s names the label %q and no label was staged under that key. An issue's "+
						"labels are looked up by the identifier the label record was staged with, "+
						"so a key that matches nothing is a label quietly dropped from the issue.",
					external, label,
				)
			}
		}
	}
}

func TestAFileWithNothingInItIsAnEmptyImportRatherThanAFailedOne(t *testing.T) {
	held := standing(t, "")

	page := fetched(t, held.source(), asking(t, entity.ImportLabel, csvfile.Settings{}))

	if len(page.Records) != 0 || !page.Done {
		t.Fatalf(
			"an empty file staged %d records and reported done = %v",
			len(page.Records), page.Done,
		)
	}
}
