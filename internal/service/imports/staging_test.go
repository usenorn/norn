package imports_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func stagePayload(h *harness) entity.ImportStagePayload {
	return entity.ImportStagePayload{
		ImportRunID: h.run().ID,
		WorkspaceID: h.run().WorkspaceID,
		Attempt:     h.run().Attempt,
	}
}

func pagesOver(count int) int {
	return (count+testPageSize-1)/testPageSize + 1
}

func savesFor(h *harness, resource entity.ImportResource) int {
	saved := 0

	for _, cursor := range h.world.saves {
		if cursor.Resource == resource {
			saved++
		}
	}

	return saved
}

func TestEveryPageAStagingSliceReadsIsSavedBeforeItAsksForTheNext(t *testing.T) {
	h := newHarness(t).backed()
	source := newStaticSource(t)

	h.offering(source)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	wanted := 0

	for _, resource := range entity.ImportPhases() {
		if resource.Fetched() {
			wanted += pagesOver(source.countOf(resource))
		}
	}

	if len(h.world.saves) != wanted {
		t.Errorf(
			"staging saved %d cursors for %d pages. Every page is saved in the same transaction "+
				"as the rows it carried, so a worker dying between two pages resumes at the page "+
				"it had finished rather than at the beginning.",
			len(h.world.saves), wanted,
		)
	}

	if saved := savesFor(h, entity.ImportIssue); saved != 3 {
		t.Errorf(
			"the six issues took %d cursor saves, want 3 for three pages of %d",
			saved, testPageSize,
		)
	}

	carried := source.fetchedCount() + source.parentedCount() + source.embeddedCount()

	if len(h.world.records) != carried {
		t.Errorf("staged %d records, want %d", len(h.world.records), carried)
	}

	if h.run().Staged != carried {
		t.Errorf(
			"the run counted %d staged rows against %d actually staged; the counter is what the "+
				"requester watches while a slow source is being drained",
			h.run().Staged, carried,
		)
	}

	if h.run().Status != entity.ImportStaged {
		t.Fatalf("run ended %q, want staged", h.run().Status)
	}
}

func TestStagingTheSamePageTwiceCarriesNothingNewAndCountsNothingTwice(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(newStaticSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	carried := len(h.world.records)
	counted := h.run().Staged

	for index, cursor := range h.world.cursors {
		if cursor.Resource == entity.ImportIssue {
			h.world.cursors[index].Cursor = "3"
			h.world.cursors[index].Exhausted = false
		}
	}

	h.world.advances = nil
	h.at(entity.ImportStaging)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("replay stage: %v", err)
	}

	if len(h.world.advances) == 0 {
		t.Fatal("the replay staged no pages at all, so this proves nothing about staging them twice")
	}

	for _, moved := range h.world.advances {
		if moved.staged != 0 {
			t.Errorf(
				"a replayed page advanced the staged count by %d. The repository reports rows it "+
					"inserted rather than rows it saw, and an import that counted a re-read page "+
					"again would report a backlog that grows every time a slice is retried.",
				moved.staged,
			)
		}
	}

	if len(h.world.records) != carried {
		t.Errorf("replay left %d records, want the original %d", len(h.world.records), carried)
	}

	if h.run().Staged != counted {
		t.Errorf("replay moved the staged count to %d, want it left at %d", h.run().Staged, counted)
	}
}

func TestAThrottledSourceParksTheImportRatherThanEndingIt(t *testing.T) {
	h := newHarness(t).backed()
	source := newFlakySource(t)

	h.offering(source)

	started := stagePayload(h)

	var (
		resumed time.Time
		payload entity.ImportStagePayload
	)

	h.jobs.EXPECT().
		EnqueueImportStage(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			queued entity.ImportStagePayload,
			processAt time.Time,
		) error {
			payload = queued
			resumed = processAt

			return nil
		})

	before := time.Now().UTC()

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if h.run().Status == entity.ImportFailed {
		t.Fatalf(
			"a rate limited source failed the run with %q. A source asking to be called back in "+
				"thirty seconds is telling us how to finish, not that we cannot.",
			h.run().PhaseError,
		)
	}

	if h.run().Status != entity.ImportStaging {
		t.Errorf("run sat at %q after a rate limit, want staging", h.run().Status)
	}

	if len(h.world.parks) != 1 || h.world.parks[0].resource != entity.ImportIssue {
		t.Fatalf("parked %v, want exactly the issue cursor", h.world.parks)
	}

	waited := h.world.parks[0].until.Sub(before)

	if waited < 29*time.Second || waited > 31*time.Second {
		t.Errorf(
			"parked the cursor for %s, want the thirty seconds the source asked for",
			waited.Round(time.Second),
		)
	}

	if queued := resumed.Sub(before); queued < 29*time.Second || queued > 31*time.Second {
		t.Errorf(
			"the continuation was queued to run in %s. Parking the cursor without deferring the "+
				"task would have a worker pick the run straight back up, find it parked and put "+
				"it down again, in a loop for as long as the source is throttling.",
			queued.Round(time.Second),
		)
	}

	if payload.Attempt != started.Attempt+1 {
		t.Errorf(
			"the continuation carries attempt %d rather than %d. The queue keys an import slice on "+
				"the run and the attempt, so a continuation reusing the attempt that parked would "+
				"collide with the task already sitting in the queue and be dropped.",
			payload.Attempt, started.Attempt+1,
		)
	}

	wanted := testPageSize + 1

	for _, resource := range entity.ImportPhases() {
		if resource == entity.ImportIssue {
			break
		}

		if resource.Fetched() {
			wanted += source.countOf(resource)
		}
	}

	if staged := len(h.world.records); staged != wanted {
		t.Errorf("staged %d rows before the throttle, want the %d already read", staged, wanted)
	}
}

func TestATransportFailureLosesTheSliceButNotThePagesAlreadyStaged(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(newFlakySource(t))
	h.jobs.EXPECT().EnqueueImportStage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	carried := len(h.world.records)

	for index := range h.world.cursors {
		h.world.cursors[index].RetryAfter = nil
	}

	err := h.runner.RunStage(context.Background(), stagePayload(h))

	var unavailable entity.ImportSourceUnavailableError

	if !errors.As(err, &unavailable) {
		t.Fatalf("second slice returned %v, want the source to have been unreachable", err)
	}

	if h.run().Status != entity.ImportStaging {
		t.Errorf(
			"a dropped connection moved the run to %q. The slice is lost and asynq will retry it; "+
				"the import itself has nothing wrong with it.",
			h.run().Status,
		)
	}

	if len(h.world.records) != carried {
		t.Errorf(
			"a failed slice left %d records where %d were already staged. Each page committed with "+
				"its own cursor, so nothing a failure touches can undo a page that was already saved.",
			len(h.world.records), carried,
		)
	}
}

func TestASourceThatRefusesToHandOverAResourceEndsTheRun(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(&scriptedSource{
		answer: func(request service.ImportFetchRequest) (service.ImportFetchPage, error) {
			return service.ImportFetchPage{}, entity.ImportSourceRefusedError{
				Resource: request.Resource,
				Reason:   "the token cannot read issues",
			}
		},
	})

	err := h.runner.RunStage(context.Background(), stagePayload(h))

	var refused entity.ImportSourceRefusedError

	if !errors.As(err, &refused) {
		t.Fatalf("run stage returned %v, want the refusal", err)
	}

	if h.run().Status != entity.ImportFailed {
		t.Fatalf(
			"a refused source left the run at %q. Being told no is not a transient condition, and "+
				"retrying it four more times only spends the requester's rate limit on a decision "+
				"that will not change.",
			h.run().Status,
		)
	}

	if !strings.Contains(h.run().PhaseError, "cannot read issues") {
		t.Errorf("the run recorded %q, want the source's own reason", h.run().PhaseError)
	}
}

func TestARowTheSourceCannotNameIsRefusedByResource(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(&scriptedSource{
		answer: func(_ service.ImportFetchRequest) (service.ImportFetchPage, error) {
			return service.ImportFetchPage{
				Records: []entity.ImportRecord{{Payload: payloadOf(t, service.ImportIssuePayload{
					Title: "Nameless",
				})}},
				Done: true,
			}, nil
		},
	})

	h.jobs.EXPECT().EnqueueImportStage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err := h.runner.RunStage(context.Background(), stagePayload(h))
	if err == nil {
		t.Fatal(
			"a row with no identifier of its own was staged. The external id is what a re-run " +
				"matches on and what the ledger names, so a row that cannot be addressed can " +
				"neither be resumed nor taken back out.",
		)
	}

	if !strings.Contains(err.Error(), string(entity.ImportIssue)) {
		t.Errorf(
			"the refusal reads %q and never names the resource, which is all an operator has to go "+
				"on when the adapter is somebody else's code",
			err.Error(),
		)
	}

	if len(h.world.records) != 0 {
		t.Errorf("staged %d records from a page that was refused", len(h.world.records))
	}
}

func parentLinkFor(t *testing.T, h *harness, externalID string) service.ImportIssueParentPayload {
	t.Helper()

	at := h.world.recordAt(entity.ImportIssueParent, externalID)
	if at < 0 {
		t.Fatalf(
			"staging %q left no parent link behind. The child names its parent and nothing else "+
				"does, so a link the staging pass does not derive is a hierarchy that can never "+
				"arrive however faithfully the source described it.",
			externalID,
		)
	}

	var payload service.ImportIssueParentPayload

	if err := json.Unmarshal(h.world.records[at].Payload, &payload); err != nil {
		t.Fatalf("decode the derived parent link: %v", err)
	}

	return payload
}

func TestTheParentAnIssueNamesIsStagedAsALinkOfItsOwn(t *testing.T) {
	h := newHarness(t).backed()
	source := newStaticSource(t)

	h.offering(source)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	payload := parentLinkFor(t, h, sourceIssueChild)

	if payload.Issue != sourceIssueChild || payload.Parent != sourceIssueHub {
		t.Errorf(
			"the derived link reads %q → %q, want %q → %q",
			payload.Issue, payload.Parent, sourceIssueChild, sourceIssueHub,
		)
	}

	links := 0

	for _, record := range h.world.records {
		if record.Resource == entity.ImportIssueParent {
			links++
		}
	}

	if links != source.parentedCount() {
		t.Errorf(
			"staging derived %d parent links for %d issues that name a parent; one link per child "+
				"is what makes the child's own identifier enough to address it",
			links, source.parentedCount(),
		)
	}

	if h.recordState(entity.ImportIssueRelation, sourceRelation) != entity.ImportRecordStaged {
		t.Fatalf(
			"nothing staged the relation the source offered. A relation is symmetric and rides on " +
				"neither issue, so unless it is fetched as a resource of its own there is no way " +
				"for a source to express one at all.",
		)
	}
}

func TestALinkIsStagedInTheSameTransactionAsThePageItWasDerivedFrom(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(sourceOfIssues(
		fetched(t, "issue-top", "", service.ImportIssuePayload{Title: "Top", Team: sourceTeam}),
		fetched(t, "issue-middle", "issue-top", service.ImportIssuePayload{
			Title: "Middle", Team: sourceTeam,
		}),
	))

	h.doomed = 1

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); !errors.Is(err, errLostCommit) {
		t.Fatalf("run stage returned %v, want the lost commit", err)
	}

	for _, resource := range []entity.ImportResource{entity.ImportIssue, entity.ImportIssueParent} {
		for _, record := range h.world.records {
			if record.Resource == resource {
				t.Fatalf(
					"a page whose transaction was lost left a %s record behind. The link is derived "+
						"from the page and committed with it, so a link that outlives its page names "+
						"a child that was never staged, and a link that dies with a page that "+
						"survived leaves a hierarchy silently flat.",
					resource,
				)
			}
		}
	}

	h.doomed = 0
	h.transactions = 0
	h.at(entity.ImportStaging)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("second slice: %v", err)
	}

	parentLinkFor(t, h, "issue-middle")

	if h.run().Staged != 3 {
		t.Errorf(
			"the run counted %d staged rows, want two issues and the link between them",
			h.run().Staged,
		)
	}

	link := h.stagedIn(entity.ImportIssueParent, "issue-middle")
	issue := h.stagedIn(entity.ImportIssue, "issue-middle")

	if link != issue {
		t.Fatalf(
			"the link was committed in transaction %d and the issue that named it in %d. A link "+
				"that commits apart from its page can outlive a page that rolled back or die with "+
				"one that did not, and either way the hierarchy and the issues it joins stop "+
				"agreeing with no slice left that would notice.",
			link, issue,
		)
	}
}

func TestReplayingAPageDoesNotDeriveASecondCopyOfItsLinks(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(sourceOfIssues(
		fetched(t, "issue-top", "", service.ImportIssuePayload{Title: "Top", Team: sourceTeam}),
		fetched(t, "issue-middle", "issue-top", service.ImportIssuePayload{
			Title: "Middle", Team: sourceTeam,
		}),
	))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	carried := len(h.world.records)
	counted := h.run().Staged

	for index := range h.world.cursors {
		h.world.cursors[index].Cursor = ""
		h.world.cursors[index].Exhausted = false
	}

	h.world.advances = nil
	h.at(entity.ImportStaging)

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("replay stage: %v", err)
	}

	if len(h.world.records) != carried || h.run().Staged != counted {
		t.Fatalf(
			"replaying the page left %d records counted as %d, want the original %d and %d. The "+
				"link is addressed by the child that named it, so a page read twice upserts onto "+
				"the link it already derived rather than filing a second one.",
			len(h.world.records), h.run().Staged, carried, counted,
		)
	}
}

func TestARowTheSourceSentTwiceInOnePageIsCarriedOnce(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(newHostileSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	carried := 0

	for _, record := range h.world.records {
		if record.ExternalID == hostileDuplicate {
			carried++
		}
	}

	if carried != 1 {
		t.Fatalf(
			"a row the export listed twice in one page was staged %d times. An identifier is what "+
				"a re-run matches on, so two rows claiming the same one would be applied twice and "+
				"the ledger would name the same source row against two issues.",
			carried,
		)
	}

	if h.run().Staged != len(h.world.records) {
		t.Errorf(
			"the run counted %d staged rows against the %d it holds; the repeated row was counted "+
				"as new work",
			h.run().Staged, len(h.world.records),
		)
	}
}

func TestAStagingRunIsNeverStartedTwiceAtOnce(t *testing.T) {
	h := newHarness(t).backed()

	h.offering(newStaticSource(t))
	h.at(entity.ImportStaging)

	leased := time.Now().UTC().Add(testLeaseTTL)
	h.world.run.LeaseToken = uuid.New()
	h.world.run.LeaseExpiresAt = &leased

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if len(h.world.records) != 0 {
		t.Fatalf(
			"a second worker staged %d rows into a run another worker still holds. Both would "+
				"fetch the same pages and both would move the same cursor.",
			len(h.world.records),
		)
	}
}
