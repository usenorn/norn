package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnUnknownReferenceIsSkippedUnlessSomebodySaidOtherwise(t *testing.T) {
	if entity.ImportUnknownPolicy("").Or(entity.ImportUnknownSkip) != entity.ImportUnknownSkip {
		t.Fatal(
			"a run that was never told what to do with a reference it cannot resolve does not " +
				"default to skipping it. Creating a stand-in for every unknown assignee fills a " +
				"workspace with accounts nobody asked for, and failing the run over one stale " +
				"reference throws away an import that was otherwise fine.",
		)
	}

	if entity.ImportUnknownFail.Or(entity.ImportUnknownSkip) != entity.ImportUnknownFail {
		t.Error("a policy that was chosen was overridden by the default")
	}

	for _, policy := range entity.ImportUnknownPolicies() {
		if !policy.Valid() {
			t.Errorf("the policy %q is offered but refused as invalid", policy)
		}
	}

	for _, refused := range []entity.ImportUnknownPolicy{"", "invent", "CREATE"} {
		if entity.ImportUnknownPolicy(refused).Valid() {
			t.Errorf(
				"the policy %q is accepted. The column constrains itself to three values, so a "+
					"fourth reaching the database fails the save rather than the request.",
				refused,
			)
		}
	}
}

func TestASourceStaysConfigurableOnlyWhileItsRowsStillMatchIt(t *testing.T) {
	configurable := map[entity.ImportStatus]bool{
		entity.ImportDraft:   true,
		entity.ImportStaging: true,
		entity.ImportStaged:  true,
	}

	for _, status := range entity.ImportStatuses() {
		if status.Configurable() != configurable[status] {
			t.Errorf(
				"a %s run reports Configurable() as %t. Mapping decisions are made against rows "+
					"that were staged with one key and one selection; letting either change after "+
					"that leaves the decisions describing rows the run would no longer read.",
				status, status.Configurable(),
			)
		}
	}
}
