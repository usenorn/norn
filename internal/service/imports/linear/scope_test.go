package linear_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestEveryResourceThisSourceOffersIsNarrowedToTheTeamsTheRunChose(t *testing.T) {
	for _, resource := range standing(t).source().Resources() {
		t.Run(string(resource), func(t *testing.T) {
			held := standing(t).answering(wholeWorkspace())

			fetched(t, staging(), held.source(), asking(resource))

			call := held.only()

			if !strings.Contains(call.Query, "in: $teams") {
				t.Fatalf(
					"the %s query never filters on the chosen teams:\n%s\nA run imports the teams it "+
						"was told to and nothing else. A query that reads the whole workspace copies "+
						"another team's backlog into this one, and the teams variable travelling "+
						"beside it changes nothing if the query ignores it.",
					resource, call.Query,
				)
			}

			want := []string{engineeringTeam, operationsTeam}

			if sent := teamsIn(t, call); !slices.Equal(sent, want) {
				t.Fatalf(
					"the %s query was given teams %v, want the %v the run chose. Narrowing on a "+
						"different set than the run selected either leaks the teams it excluded or "+
						"silently drops the ones it asked for.",
					resource, sent, want,
				)
			}
		})
	}
}

func TestARunThatNamedNoTeamIsRefusedRatherThanLeftToReadTheWholeWorkspace(t *testing.T) {
	for _, chosen := range []struct {
		name     string
		settings string
	}{
		{"nothing was configured at all", ``},
		{"the selection is empty", `{"teamIds":[]}`},
		{"the selection holds only blanks", `{"teamIds":["","   "]}`},
	} {
		t.Run(chosen.name, func(t *testing.T) {
			held := standing(t).answering(wholeWorkspace())
			source := held.source()

			for _, resource := range source.Resources() {
				request := asking(resource)
				request.Config.Settings = json.RawMessage(chosen.settings)

				_, err := source.Fetch(staging(), request)

				var refused entity.ImportSourceRefusedError

				if !errors.As(err, &refused) {
					t.Fatalf(
						"fetching %s with no team chosen returned %v, want a refusal. An empty "+
							"selection is the one thing that cannot be guessed: read as everything it "+
							"copies a whole workspace nobody approved, read as nothing it reports a "+
							"successful import of zero rows.",
						resource, err,
					)
				}

				if !strings.Contains(refused.Reason, "team") {
					t.Errorf(
						"the refusal of %s reads %q and never says teams are the thing missing. The "+
							"person holding this run can add a team to it in a minute if told; the "+
							"message is the only place they find out.",
						resource, refused.Reason,
					)
				}
			}

			if calls := held.seen(); len(calls) != 0 {
				t.Fatalf(
					"the adapter called the source %d times before refusing. A run that cannot be "+
						"narrowed must not touch the source at all: the first unnarrowed page is "+
						"already somebody else's data.",
					len(calls),
				)
			}
		})
	}
}

func TestARunWithNoKeyIsRefusedBeforeTheSourceIsCalled(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())
	source := held.source()

	request := asking(entity.ImportIssue)
	request.Config.Secret = "   "

	_, err := source.Fetch(staging(), request)

	var refused entity.ImportSourceRefusedError

	if !errors.As(err, &refused) {
		t.Fatalf(
			"fetching without a key returned %v, want a refusal. An anonymous call is rejected by "+
				"the source as unauthenticated, which the framework would read as a key to re-check "+
				"rather than as a run nobody finished configuring.",
			err,
		)
	}

	if len(held.seen()) != 0 {
		t.Errorf("the adapter called the source with no key rather than refusing on the spot")
	}

	if _, err := source.Probe(context.Background(), service.ImportSourceConfig{}); err == nil {
		t.Errorf("a probe with no key was allowed to call the source")
	}
}

func TestProbeAnswersWithTheTeamsAKeyCanSeeAndStagesNothing(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())

	catalogue, err := held.source().Probe(context.Background(), service.ImportSourceConfig{
		Secret: sourceKey,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	keys := make([]string, 0, len(catalogue.Scopes))

	for _, scope := range catalogue.Scopes {
		keys = append(keys, scope.Key)
	}

	if want := []string{engineeringTeam, operationsTeam}; !slices.Equal(keys, want) {
		t.Fatalf(
			"the probe offered %v to choose from, want %v. This list is what the requester picks "+
				"teams out of, and a key missing from it is a team that cannot be imported at all.",
			keys, want,
		)
	}

	if catalogue.Scopes[0].Name != "Engineering" || catalogue.Scopes[0].Detail != "ENG" {
		t.Errorf(
			"the first team offers name %q and detail %q, want the workspace's own name and key. "+
				"Nobody recognises a team by its node id, which is what they would be choosing "+
				"between otherwise.",
			catalogue.Scopes[0].Name, catalogue.Scopes[0].Detail,
		)
	}

	if len(catalogue.Notes) == 0 {
		t.Errorf(
			"the catalogue carries no notes. What Linear holds and what this workspace can hold " +
				"differ in ways nobody discovers from the import itself — one link per pair, most " +
				"attachments being links — and this is the only screen that says so beforehand.",
		)
	}

	call := held.only()

	if call.Operation != "ImportScopes" {
		t.Fatalf("the probe ran %q, want the bounded scopes query", call.Operation)
	}

	for _, staged := range []string{"issues", "comments", "attachments", "after"} {
		if strings.Contains(call.Query, staged) {
			t.Errorf(
				"the probe's query reads %s. Probe is the one call made outside the staging job, "+
					"from a request a person is waiting on: it answers which teams exist and nothing "+
					"that has to be walked.",
				staged,
			)
		}
	}
}

func TestThePhasesDerivedFromRowsAlreadyStagedAreNeverFetched(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())
	source := held.source()
	offered := source.Resources()

	for _, derived := range []entity.ImportResource{entity.ImportIssueParent, entity.ImportEmbed} {
		if slices.Contains(offered, derived) {
			t.Fatalf(
				"the source offers %s as something to fetch. It is derived from a row the source "+
					"already sent, so fetching it stages a page of nothing over the records the "+
					"staging pass had just derived itself.",
				derived,
			)
		}

		_, err := source.Fetch(staging(), asking(derived))

		var refused entity.ImportSourceRefusedError

		if !errors.As(err, &refused) {
			t.Errorf("fetching %s returned %v, want a refusal naming the resource", derived, err)
		}
	}

	if len(offered) != len(entity.ImportPhases())-2 {
		t.Errorf(
			"the source offers %d of the %d phases. Every phase except the two derived ones has to "+
				"be fetched from somewhere, and one left out is a resource the import silently "+
				"never carries.",
			len(offered), len(entity.ImportPhases()),
		)
	}
}
