package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func samplePreview() entity.ImportPreview {
	return entity.ImportPreview{
		Created: []entity.ImportPreviewLine{
			{Resource: entity.ImportIssue, ExternalID: "PROJ-1", Subject: "Payments time out", Outcome: entity.ImportOutcomeCreated},
			{Resource: entity.ImportIssue, ExternalID: "PROJ-2", Subject: "Retry the webhook", Outcome: entity.ImportOutcomeCreated},
			{Resource: entity.ImportLabel, ExternalID: "bug", Subject: "Bug", Outcome: entity.ImportOutcomeCreated},
		},
		Skipped: []entity.ImportPreviewLine{
			{Resource: entity.ImportIssue, ExternalID: "PROJ-3", Outcome: entity.ImportOutcomeSkipped, Detail: "team not visible"},
		},
		Unattributed: []entity.ImportPreviewLine{
			{Resource: entity.ImportIssue, ExternalID: "PROJ-1", Subject: "sam@elsewhere.example", Outcome: entity.ImportOutcomeUnattributed},
		},
	}
}

func TestTheSamePreviewAlwaysFingerprintsTheSame(t *testing.T) {
	first, second := samplePreview().Digest(), samplePreview().Digest()

	if first != second {
		t.Fatalf(
			"the same preview fingerprints as %s then %s. Execute refuses a run whose digest "+
				"has moved, so an unstable fingerprint would refuse every import.",
			first, second,
		)
	}

	if first == "" {
		t.Error("a preview with content fingerprints as nothing")
	}
}

func TestTheFingerprintDoesNotDependOnTheOrderLinesArriveIn(t *testing.T) {
	shuffled := samplePreview()
	shuffled.Created[0], shuffled.Created[2] = shuffled.Created[2], shuffled.Created[0]

	if shuffled.Digest() != samplePreview().Digest() {
		t.Fatal(
			"reordering the same lines changes the fingerprint. Rows come back from a keyset " +
				"walk and their order is not a promise, so an order-sensitive digest would " +
				"refuse imports at random.",
		)
	}
}

func TestSomethingDisappearingBetweenPreviewAndExecuteChangesTheFingerprint(t *testing.T) {
	before := samplePreview()

	moved := samplePreview()
	moved.Created = moved.Created[:len(moved.Created)-1]
	moved.Skipped = append(moved.Skipped, entity.ImportPreviewLine{
		Resource:   entity.ImportLabel,
		ExternalID: "bug",
		Outcome:    entity.ImportOutcomeSkipped,
		Detail:     "the label it would have joined has been deleted",
	})

	if before.Digest() == moved.Digest() {
		t.Fatal(
			"a line moving from created to skipped leaves the fingerprint alone. That is the " +
				"whole point of the digest: the operator approved a preview, and if the " +
				"workspace has changed underneath it they have not approved what would happen now.",
		)
	}
}

func TestARenamedSubjectIsANewPreview(t *testing.T) {
	renamed := samplePreview()
	renamed.Created[0].Subject = "Payments time out under load"

	if renamed.Digest() == samplePreview().Digest() {
		t.Error("changing what an issue would be called leaves the fingerprint alone")
	}
}

func TestATeamThatWouldTriageIsPartOfWhatWasApproved(t *testing.T) {
	quiet := samplePreview()

	triaging := samplePreview()
	triaging.TriageTeams = []string{"Engineering"}

	if quiet.Digest() == triaging.Digest() {
		t.Fatal(
			"turning triage on for a target team leaves the fingerprint alone. Where imported " +
				"issues land is the single thing an operator most needs to have agreed to.",
		)
	}

	if !triaging.WouldTriage() {
		t.Error("a preview naming a triaging team does not report that it would triage")
	}

	if quiet.WouldTriage() {
		t.Error("a preview naming no triaging team reports that it would triage")
	}
}

func TestAPreviewThatWouldCreateNothingSaysSo(t *testing.T) {
	if !(entity.ImportPreview{}).Empty() {
		t.Error("a preview with no lines at all does not report itself empty")
	}

	onlySkips := entity.ImportPreview{
		Skipped: []entity.ImportPreviewLine{{Resource: entity.ImportIssue, ExternalID: "PROJ-3"}},
	}

	if !onlySkips.Empty() {
		t.Error(
			"a preview that would skip everything and create nothing does not report itself " +
				"empty, so an operator would be invited to run an import that does nothing",
		)
	}

	if samplePreview().Empty() {
		t.Error("a preview that would create three things reports itself empty")
	}
}
