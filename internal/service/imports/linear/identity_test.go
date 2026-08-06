package linear_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestEveryRecordTheSourceStagesIsAddressedByTheSourcesOwnIdentifier(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())
	source := held.source()

	for _, resource := range source.Resources() {
		page := fetched(t, staging(), source, asking(resource))

		if len(page.Records) == 0 {
			t.Fatalf(
				"%s came back empty from a workspace holding one of everything. A phase that stages "+
					"nothing reads as a source with nothing to give, and the import finishes without "+
					"it rather than reporting it missing.",
				resource,
			)
		}

		addressed := make(map[string]bool, len(page.Records))

		for _, record := range page.Records {
			if strings.TrimSpace(record.ExternalID) == "" {
				t.Fatalf(
					"a %s record arrived with no external id. Every ledger entry, every mapping "+
						"decision and every report line is keyed on it, so one blank row does not "+
						"cost itself — it stops the whole staging pass.",
					resource,
				)
			}

			if addressed[record.ExternalID] {
				t.Errorf(
					"%s staged %q twice in one page. The ledger holds one row per source identifier, "+
						"so the second arrival either overwrites the first or is refused, and which "+
						"of the two happened is not visible from the report.",
					resource, record.ExternalID,
				)
			}

			addressed[record.ExternalID] = true
		}
	}
}

func TestARecordIsAddressedByTheSourcesNodeIdRatherThanByWhatPeopleCallIt(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())
	source := held.source()

	teams := fetched(t, staging(), source, asking(entity.ImportTeam))
	team := recordNamed(t, teams, engineeringTeam)

	if named := payloadOf[service.ImportTeamPayload](t, team); named.Key != "ENG" {
		t.Fatalf(
			"the team staged as %q carries key %q, want the human key beside the node id. A key "+
				"people type is the one thing a mapping screen can be read by, and a node id in "+
				"its place makes every suggestion unrecognisable.",
			team.ExternalID, named.Key,
		)
	}

	for _, taken := range []string{"ENG", "Engineering"} {
		for _, record := range teams.Records {
			if record.ExternalID == taken {
				t.Fatalf(
					"a team is addressed by %q. A key and a name are both renamed by whoever owns "+
						"the team, and a ledger keyed on either points at the wrong row the first "+
						"time somebody renames one — or collides with a second team that took it.",
					taken,
				)
			}
		}
	}

	projects := fetched(t, staging(), source, asking(entity.ImportProject))
	project := recordNamed(t, projects, atlasProject)

	if slug := payloadOf[service.ImportProjectPayload](t, project).Slug; slug != "atlas-9f2" {
		t.Errorf("the project carries slug %q, want the source's own slug beside its node id", slug)
	}
}

func TestAnIssueCarriesItsParentSoThePhaseDerivedFromItHasSomethingToRead(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, staging(), held.source(), asking(entity.ImportIssue))
	child := recordNamed(t, page, childIssue)

	if child.ParentExternalID != hubIssue {
		t.Fatalf(
			"the sub-issue names parent %q, want %q. Nothing is fetched for the parent phase: it is "+
				"derived from this field alone, so an issue that arrives without it is imported as a "+
				"top-level row and the hierarchy is quietly flattened.",
			child.ParentExternalID, hubIssue,
		)
	}

	if parent := recordNamed(t, page, hubIssue); parent.ParentExternalID != "" {
		t.Errorf("a top-level issue claims parent %q", parent.ParentExternalID)
	}
}
