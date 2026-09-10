package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func checklist(items ...entity.Node) entity.Document {
	return entity.NewDocument(entity.Node{Type: entity.NodeTaskList, Content: items})
}

func criterion(id, text string, checked bool) entity.Node {
	return entity.Node{
		Type:  entity.NodeTaskItem,
		Attrs: map[string]any{"id": id, "checked": checked},
		Content: []entity.Node{{
			Type:    entity.NodeParagraph,
			Content: []entity.Node{{Type: entity.NodeText, Text: text}},
		}},
	}
}

func TestAcceptanceCriteriaAreTheChecklistTheDescriptionCarries(t *testing.T) {
	found := entity.AcceptanceCriteria(checklist(
		criterion("one", "The export writes a file", true),
		criterion("two", "The file opens in a spreadsheet", false),
	))

	if len(found) != 2 {
		t.Fatalf("read %d criteria from a checklist of two", len(found))
	}

	if found[0].Text != "The export writes a file" || !found[0].Checked {
		t.Fatalf("the first criterion came back as %+v", found[0])
	}

	if found[1].Checked {
		t.Fatalf("an unticked criterion came back ticked")
	}
}

func TestAChecklistItemWithNoIdentifierIsNotACriterion(t *testing.T) {
	unnamed := entity.Node{
		Type:  entity.NodeTaskItem,
		Attrs: map[string]any{"checked": false},
		Content: []entity.Node{{
			Type:    entity.NodeParagraph,
			Content: []entity.Node{{Type: entity.NodeText, Text: "Written before criteria had names"}},
		}},
	}

	if found := entity.AcceptanceCriteria(checklist(unnamed)); len(found) != 0 {
		t.Fatalf(
			"an item with no identifier was read as a criterion. Evidence would be filed against "+
				"something nothing can point back to: %+v",
			found,
		)
	}
}

func TestEvidenceGoesStaleWhenTheCriterionIsRewritten(t *testing.T) {
	proof := entity.CriterionEvidence{
		CriterionID:   "one",
		CriterionText: "The export writes a file",
		Kind:          entity.EvidenceTest,
		Label:         "TestExportWritesAFile",
	}

	if proof.Stale("The export writes a file") {
		t.Fatalf("evidence answering the criterion as it reads was called stale")
	}

	if !proof.Stale("The export writes a valid file") {
		t.Fatalf(
			"evidence filed against older wording was called current. What it proves is the " +
				"sentence it was filed against, not the one that replaced it.",
		)
	}
}

func TestACriterionIsProvenOnlyByEvidenceThatStillAnswersIt(t *testing.T) {
	held := entity.AcceptanceCriterion{
		ID:   "one",
		Text: "The export writes a valid file",
		Evidence: []entity.CriterionEvidence{
			{CriterionText: "The export writes a file", Kind: entity.EvidenceTest},
		},
	}

	if held.Proven() {
		t.Fatalf("stale evidence was counted as proof")
	}

	held.Evidence = append(held.Evidence, entity.CriterionEvidence{
		CriterionText: "The export writes a valid file",
		Kind:          entity.EvidencePerson,
	})

	if !held.Proven() {
		t.Fatalf("evidence answering the criterion as it reads was not counted")
	}
}

func TestEvidenceThatPointsNowhereIsRefusedUnlessAPersonVouched(t *testing.T) {
	if failures := entity.ValidateEvidence(entity.EvidenceTest, "TestExport", ""); len(failures) == 0 {
		t.Fatalf("a test with no address was accepted, so nobody can go and look at it")
	}

	if failures := entity.ValidateEvidence(entity.EvidencePerson, "Rae checked it", ""); len(failures) != 0 {
		t.Fatalf("a person vouching was refused for having no address: %+v", failures)
	}

	if failures := entity.ValidateEvidence(entity.EvidencePerson, "  ", ""); len(failures) == 0 {
		t.Fatalf("evidence with nothing written on it was accepted")
	}
}
