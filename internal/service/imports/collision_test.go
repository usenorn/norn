package imports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func threeLabels(t *testing.T) []entity.ImportRecord {
	t.Helper()

	return []entity.ImportRecord{
		fetched(t, sourceLabelBug, "", service.ImportLabelPayload{Name: "Bug", Team: sourceTeam}),
		fetched(t, sourceLabelWork, "", service.ImportLabelPayload{Name: "Chore", Team: sourceTeam}),
		fetched(t, sourceLabelOps, "", service.ImportLabelPayload{Name: "Ops", Team: sourceTeam}),
	}
}

func TestANameTheWorkspaceAlreadyHoldsCostsThatRowAndNotItsChunk(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")
	workspace := newWorkspace()

	standing := entity.Label{ID: uuid.New(), TeamID: team.ID, Name: "Bug"}
	workspace.standingLabels[standing.ID] = standing

	h.scopedTo(team.ID)
	h.stage(entity.ImportLabel, threeLabels(t)...)
	onlyTheTeamIsDecided(h, team.ID)
	applyingInto(h, team, workspace)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf(
			"a workspace that already held a label named Bug ended the run: %v. The unique "+
				"violation aborted the transaction the chunk runs in, so the two labels behind it, "+
				"the ledger, the settlement and the counter were all refused with 25P02 and the "+
				"whole chunk was handed back to be retried into the same collision. Isolating each "+
				"record in a savepoint is what leaves the transaction usable after the row that "+
				"failed.",
			err,
		)
	}

	if made := len(h.ledgerFor(entity.ImportLabel)); made != 2 {
		t.Fatalf(
			"the ledger holds %d labels after one of three collided, want the other two. Importing "+
				"into a workspace that already contains anything is the ordinary case, not an edge "+
				"one.",
			made,
		)
	}

	if state := h.recordState(entity.ImportLabel, sourceLabelBug); state != entity.ImportRecordSkipped {
		t.Errorf(
			"the colliding label reads as %q, want skipped. A name the workspace already holds is "+
				"a row the source legitimately produced, and settling it failed carries the run "+
				"toward its attempt limit for something nothing is wrong with.",
			state,
		)
	}

	for _, external := range []string{sourceLabelWork, sourceLabelOps} {
		if state := h.recordState(entity.ImportLabel, external); state != entity.ImportRecordApplied {
			t.Errorf("%s reads as %q after the row before it collided, want applied", external, state)
		}
	}

	line, found := h.lineOf(entity.ImportLabel, sourceLabelBug)

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Errorf("the report says %q about the colliding label, want skipped", line.Outcome)
	}

	if line.Detail["reason"] != entity.ErrLabelNameTaken.Error() {
		t.Errorf(
			"the report gives %q as the reason, want the collision itself. The reason is the only "+
				"place the requester learns which of their labels was already here.",
			line.Detail["reason"],
		)
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q, want imported with the collision recorded rather than raised", h.run().Status)
	}
}

func TestEveryRecordIsIsolatedInItsOwnSavepointAndReleasedWhenItStands(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportLabel, threeLabels(t)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if h.savepoints != 3 {
		t.Fatalf(
			"applying three records asked the transactor for %d savepoints, want one each. Whether "+
				"a refused row costs the row or the chunk is decided entirely by whether there is a "+
				"point to roll back to, so this has to be asked for per record rather than per chunk.",
			h.savepoints,
		)
	}

	if h.released != 3 || h.unwound != 0 {
		t.Errorf(
			"three records that all stood released %d savepoints and unwound %d, want three "+
				"released and none unwound. A savepoint left open holds its point for the rest of "+
				"the transaction.",
			h.released, h.unwound,
		)
	}

	if h.transactions != 1 {
		t.Errorf(
			"the chunk took %d transactions, want one. A savepoint is a point inside the chunk's "+
				"transaction, never a transaction of its own: split the chunk in two and a worker "+
				"dying between them leaves rows created that nothing in the ledger admits to.",
			h.transactions,
		)
	}
}

func TestASavepointIsUnwoundOnlyForTheRecordThatFailedInIt(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")
	workspace := newWorkspace()

	standing := entity.Label{ID: uuid.New(), TeamID: team.ID, Name: "Chore"}
	workspace.standingLabels[standing.ID] = standing

	h.scopedTo(team.ID)
	h.stage(entity.ImportLabel, threeLabels(t)...)
	onlyTheTeamIsDecided(h, team.ID)
	applyingInto(h, team, workspace)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if h.savepoints != 3 || h.unwound != 1 || h.released != 2 {
		t.Errorf(
			"three records with one collision among them took %d savepoints, unwound %d and "+
				"released %d, want three taken, one unwound and two released.",
			h.savepoints, h.unwound, h.released,
		)
	}
}

func TestASourceImportedTwiceSkipsWhatItAlreadyMadeAndFinishes(t *testing.T) {
	team := teamNamed("Core")
	workspace := newWorkspace()

	first := importingLabels(t, team, workspace)

	if made := len(first.ledgerFor(entity.ImportLabel)); made != 3 {
		t.Fatalf("the first run made %d labels, want all three", made)
	}

	second := importingLabels(t, team, workspace)

	if made := len(second.ledgerFor(entity.ImportLabel)); made != 0 {
		t.Errorf(
			"the second run made %d labels the first had already made. Re-running an import is "+
				"something the framework supports by design, and a row it made last time is one to "+
				"recognise rather than one to make again.",
			made,
		)
	}

	for _, external := range []string{sourceLabelBug, sourceLabelWork, sourceLabelOps} {
		state := second.recordState(entity.ImportLabel, external)

		if state != entity.ImportRecordSkipped {
			t.Errorf(
				"%s reads as %q on the second run, want skipped. The workspace already holds it, "+
					"which is the database agreeing with the first run rather than anything having "+
					"gone wrong.",
				external, state,
			)
		}
	}

	if second.run().Status != entity.ImportImported {
		t.Errorf(
			"the second run over the same source ended %q, want imported. A run that cannot be "+
				"repeated cannot be resumed either, and the whole retry path depends on it.",
			second.run().Status,
		)
	}
}

func importingLabels(t *testing.T, team entity.Team, workspace *applied) *harness {
	t.Helper()

	h := newHarness(t).backed()

	h.scopedTo(team.ID)
	h.stage(entity.ImportLabel, threeLabels(t)...)
	onlyTheTeamIsDecided(h, team.ID)
	applyingInto(h, team, workspace)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	return h
}

func TestTheLedgerWriteIsNotOneOfTheThingsAChunkCanSurvive(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportLabel, threeLabels(t)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)
	h.refusedBook = errors.New("write the ledger: connection reset")

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); !errors.Is(err, h.refusedBook) {
		t.Fatalf(
			"the ledger refusing the chunk's own entries returned %v, want the refusal raised. "+
				"A savepoint around a record is what lets the chunk carry on past that record; "+
				"the ledger is the chunk saying what it made, and a chunk that swallowed a failure "+
				"there would leave rows standing that no revert could ever find.",
			err,
		)
	}

	if len(h.world.ledger) != 0 {
		t.Errorf("the ledger holds %d entries after the write that made them failed", len(h.world.ledger))
	}

	for _, external := range []string{sourceLabelBug, sourceLabelWork, sourceLabelOps} {
		if state := h.recordState(entity.ImportLabel, external); state != entity.ImportRecordStaged {
			t.Errorf(
				"%s reads as %q after the chunk carrying it was rolled back, want staged so the "+
					"retry walks it again",
				external, state,
			)
		}
	}

	if h.run().Status != entity.ImportExecuting {
		t.Errorf("run sat at %q, want executing under the retry the queue will bring", h.run().Status)
	}
}
