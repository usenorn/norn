package imports_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type standing struct {
	issues map[uuid.UUID]entity.Issue
	purged []uuid.UUID
}

func unwinding(h *harness) *standing {
	held := &standing{issues: map[uuid.UUID]entity.Issue{}}

	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, issueID uuid.UUID,
			_ entity.TeamScope,
		) (entity.Issue, error) {
			issue, carried := held.issues[issueID]
			if !carried {
				return entity.Issue{}, entity.ErrIssueNotFound
			}

			return issue, nil
		}).
		AnyTimes()

	h.issues.EXPECT().
		SetParent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			issueID uuid.UUID,
			_ int,
			parentID *uuid.UUID,
			_ int,
			_ time.Time,
		) error {
			h.followed("unlink " + held.issues[issueID].Reference())

			issue := held.issues[issueID]
			issue.ParentIssueID = uuid.Nil
			held.issues[issueID] = issue

			return nil
		}).
		AnyTimes()

	purging(h, held.issues, &held.purged)

	return held
}

func (s *standing) carrying(reference string, version int) entity.Issue {
	issue := entity.Issue{
		ID:           uuid.New(),
		ReferenceKey: reference,
		Number:       len(s.issues) + 1,
		Version:      version,
		Status:       entity.IssueStatusActive,
	}

	s.issues[issue.ID] = issue

	return issue
}

func issueEntry(issue entity.Issue, externalID string) entity.ImportLedgerEntry {
	return entity.ImportLedgerEntry{
		Resource:        entity.ImportIssue,
		CreatedID:       issue.ID,
		ExternalID:      externalID,
		Reference:       issue.Reference(),
		VersionAtCreate: issue.Version,
	}
}

func revertPayload(h *harness) entity.ImportRevertPayload {
	return entity.ImportRevertPayload{
		ImportRunID: h.run().ID,
		WorkspaceID: h.run().WorkspaceID,
		Attempt:     h.run().Attempt,
	}
}

func outcomeOfEntry(
	t *testing.T,
	h *harness,
	resource entity.ImportResource,
	createdID uuid.UUID,
) entity.ImportOutcome {
	t.Helper()

	for _, entry := range h.world.ledger {
		if entry.Resource == resource && entry.CreatedID == createdID {
			return entry.RevertOutcome
		}
	}

	t.Fatalf("the ledger has no %s entry for %s", resource, createdID)

	return ""
}

func outcomeOf(t *testing.T, h *harness, externalID string) entity.ImportOutcome {
	t.Helper()

	for _, entry := range h.world.ledger {
		if entry.ExternalID == externalID {
			return entry.RevertOutcome
		}
	}

	t.Fatalf("the ledger has no entry for %q", externalID)

	return ""
}

func chainedIssues(t *testing.T) []entity.ImportRecord {
	t.Helper()

	return []entity.ImportRecord{
		fetched(t, "issue-top", "", service.ImportIssuePayload{
			Title: "Rebuild the hub", Team: sourceTeam,
		}),
		fetched(t, "issue-middle", "issue-top", service.ImportIssuePayload{
			Title: "Rebuild the hub header", Team: sourceTeam,
		}),
		fetched(t, "issue-bottom", "issue-middle", service.ImportIssuePayload{
			Title: "Redraw the hub header logo", Team: sourceTeam,
		}),
	}
}

func importedChain(t *testing.T, h *harness, team entity.Team) *applied {
	t.Helper()

	h.offering(sourceOfIssues(chainedIssues(t)...))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	return made
}

func TestEveryParentLinkIsUnpickedBeforeAnyIssueIsDeleted(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	importedChain(t, h, team)

	if links := len(h.ledgerFor(entity.ImportIssueParent)); links != 2 {
		t.Fatalf(
			"a chain of three issues left %d parent links in the ledger, want 2. The links were "+
				"derived from the issues as they were staged, so a chain that arrives flat means "+
				"the hierarchy never came over at all.",
			links,
		)
	}

	h.trace = nil
	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	unlinked := slices.IndexFunc(h.trace, func(step string) bool {
		return strings.HasPrefix(step, "unlink ")
	})
	deleted := slices.IndexFunc(h.trace, func(step string) bool {
		return strings.HasPrefix(step, "delete ")
	})

	if deleted < 0 {
		t.Fatalf("the revert deleted nothing at all: %v", h.trace)
	}

	if unlinked < 0 || unlinked > deleted {
		t.Fatalf(
			"the revert reached an issue before the parent link hanging off it: %v. "+
				"workspace_issues clears a child's parent when the parent goes and then refuses "+
				"the row for having a depth with no parent, so the delete fails on the constraint "+
				"and the revert stalls with half the import still standing.",
			h.trace,
		)
	}

	for _, step := range h.trace[:deleted] {
		if !strings.HasPrefix(step, "unlink ") {
			t.Errorf("the revert did %q before it had finished unpicking the hierarchy", step)
		}
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func (s *applied) issueTitled(t *testing.T, title string) entity.Issue {
	t.Helper()

	for _, issue := range s.held {
		if issue.Title == title {
			return issue
		}
	}

	t.Fatalf("no issue titled %q was created", title)

	return entity.Issue{}
}

func (s *applied) standing() []string {
	references := make([]string, 0, len(s.held))

	for _, issue := range s.held {
		references = append(references, issue.Reference())
	}

	slices.Sort(references)

	return references
}

func (h *harness) retained() []string {
	named := make([]string, 0, len(h.world.lines))

	for _, line := range h.world.lines {
		if line.Phase != entity.ImportPhaseRevert || line.Outcome.Reverted() {
			continue
		}

		named = append(named, line.Subject)
	}

	return named
}

func editedSince(made *applied, issue entity.Issue) {
	issue.Version += 2
	made.held[issue.ID] = issue
}

func TestAnImportedChainIsTakenBackOutWholeRatherThanLeavingItsChildren(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedChain(t, h, team)

	h.trace = nil
	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if standing := made.standing(); len(standing) != 0 {
		t.Fatalf(
			"reverting an imported hierarchy left %v behind. Every one of these was created by "+
				"this import and touched since by nothing but this import, which reparented it "+
				"and has just unpicked that again; refusing them as somebody else's work leaves "+
				"the requester to delete by hand exactly the rows the revert exists to remove.",
			standing,
		)
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestAChildSomebodyEditedKeepsBothItsWorkAndItsPlaceInTheHierarchy(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedChain(t, h, team)
	bottom := made.issueTitled(t, "Redraw the hub header logo")

	editedSince(made, bottom)

	h.trace = nil
	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	kept, standing := made.held[bottom.ID]

	if !standing {
		t.Fatalf(
			"the revert deleted %s, which somebody had worked on after the import. The unlink is "+
				"the revert's own edit and moves the version again, so restamping the ledger "+
				"before the row has been proven untouched makes the check agree with itself and "+
				"deletes the very work it exists to protect. The version has to be judged at the "+
				"first thing the revert touches, which is the unlink.",
			bottom.Reference(),
		)
	}

	if kept.ParentIssueID == uuid.Nil {
		t.Fatalf(
			"%s was kept but its parent link was unpicked anyway, so an issue somebody is still "+
				"working on has been quietly torn out of the hierarchy it sat in.",
			bottom.Reference(),
		)
	}

	if slices.Contains(h.trace, "unlink "+bottom.Reference()) {
		t.Errorf("the revert unlinked %s before deciding to keep it: %v", bottom.Reference(), h.trace)
	}

	for _, resource := range []entity.ImportResource{entity.ImportIssue, entity.ImportIssueParent} {
		if outcome := outcomeOfEntry(t, h, resource, bottom.ID); outcome != entity.ImportOutcomeSkippedModified {
			t.Errorf("the %s entry was recorded as %q, want skipped_modified", resource, outcome)
		}
	}

	if !slices.Contains(h.retained(), bottom.Reference()) {
		t.Errorf(
			"the report never names %s among the rows left standing, so the requester is told the "+
				"import was reverted and has no way to learn that part of it still exists",
			bottom.Reference(),
		)
	}
}

func TestAnEditInTheMiddleOfAChainStopsAtTheRowsItActuallyTouches(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedChain(t, h, team)

	top := made.issueTitled(t, "Rebuild the hub")
	middle := made.issueTitled(t, "Rebuild the hub header")
	bottom := made.issueTitled(t, "Redraw the hub header logo")

	editedSince(made, middle)

	h.trace = nil
	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if _, kept := made.held[middle.ID]; !kept {
		t.Fatalf("the edited middle issue %s was deleted", middle.Reference())
	}

	if _, kept := made.held[bottom.ID]; kept {
		t.Fatalf(
			"%s was left standing. Nobody touched it, the revert unpicked its link cleanly, and a "+
				"revert that keeps untouched rows because something above them was edited gives "+
				"back less than it safely can.",
			bottom.Reference(),
		)
	}

	if _, kept := made.held[top.ID]; !kept {
		t.Fatalf(
			"%s was deleted while %s still hangs off it. workspace_issues nulls a child's parent "+
				"on delete and then refuses the child for having depth without one, so this "+
				"delete cannot have succeeded against the real table.",
			top.Reference(), middle.Reference(),
		)
	}

	named := h.retained()

	for _, reference := range made.standing() {
		if !slices.Contains(named, reference) {
			t.Errorf(
				"%s is still standing and the report never mentions it. Whatever shape a "+
					"half-undone hierarchy is left in, a person can only finish it by hand if "+
					"every row that survived is written down.",
				reference,
			)
		}
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestAnIssueSomebodyHasWorkedOnSinceIsLeftWhereItIs(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	held := unwinding(h)

	untouched := held.carrying("PLA", 1)
	worked := held.carrying("PLA", 1)

	entry := issueEntry(worked, "issue-worked")

	worked.Version = 4
	held.issues[worked.ID] = worked

	h.alreadyMade(issueEntry(untouched, "issue-untouched"), entry)

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, "issue-worked"); outcome != entity.ImportOutcomeSkippedModified {
		t.Fatalf(
			"an issue somebody had edited since the import was recorded as %q. An import is "+
				"undone on the understanding that nothing has happened to what it made; the "+
				"moment somebody has worked on a row, deleting it destroys their work rather "+
				"than the import's.",
			outcome,
		)
	}

	if slices.Contains(h.trace, "delete "+worked.Reference()) {
		t.Fatalf("the edited issue was deleted anyway: %v", h.trace)
	}

	if !slices.Contains(h.trace, "delete "+untouched.Reference()) {
		t.Errorf("the untouched issue beside it was not deleted: %v", h.trace)
	}

	line, found := h.lineFor("issue-worked")

	if !found || line.Detail["reason"] == "" {
		t.Errorf(
			"the report does not say why %s was left standing. A revert that quietly leaves rows "+
				"behind reads as one that failed.",
			worked.Reference(),
		)
	}
}

func cycleReverting(h *harness, issues []entity.Issue) entity.Cycle {
	cycle := entity.Cycle{
		ID:        uuid.New(),
		TeamKey:   "CORE",
		Number:    7,
		StartsOn:  sourceCycleStartsOn,
		EndsOn:    sourceCycleEndsOn,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), h.run().WorkspaceID, cycle.ID, gomock.Any()).
		Return(cycle, nil)

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			if page.CycleID == nil || *page.CycleID != cycle.ID {
				return nil, nil
			}

			return issues, nil
		}).
		AnyTimes()

	h.cycles.EXPECT().
		Delete(gomock.Any(), cycle.ID).
		DoAndReturn(func(_ context.Context, id uuid.UUID) error {
			h.followed("delete cycle " + id.String())

			return nil
		}).
		AnyTimes()

	h.alreadyMade(entity.ImportLedgerEntry{
		Resource:          entity.ImportCycle,
		CreatedID:         cycle.ID,
		ExternalID:        sourceCycle,
		Reference:         "Sprint 7",
		CreatedAtRecorded: time.Now().UTC(),
	})

	h.at(entity.ImportImported)

	return cycle
}

func TestACycleTheImportMadeIsTakenBackOutWithIt(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	cycle := cycleReverting(h, nil)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceCycle); outcome != entity.ImportOutcomeDeleted {
		t.Fatalf(
			"a cycle the import made and nothing has been planned into was recorded as %q. A "+
				"revert that leaves the sprints behind leaves the team a cadence of empty windows "+
				"nobody asked for.",
			outcome,
		)
	}

	if !slices.Contains(h.trace, "delete cycle "+cycle.ID.String()) {
		t.Errorf("the cycle was never deleted: %v", h.trace)
	}
}

func TestACycleSomebodyHasPlannedWorkIntoSurvivesTheRevert(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	cycle := cycleReverting(h, []entity.Issue{{
		ID: uuid.New(), ReferenceKey: "CORE", Number: 12, Status: entity.IssueStatusActive,
	}})

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceCycle); outcome != entity.ImportOutcomeSkippedInUse {
		t.Fatalf(
			"a cycle with an issue still planned into it was recorded as %q on the way out. "+
				"workspace_issues.cycle_id clears itself on delete, so nothing in the database "+
				"refuses this: the issue somebody moved in after the import would simply come out "+
				"of the sprint, with no record that it had ever been in one.",
			outcome,
		)
	}

	if slices.Contains(h.trace, "delete cycle "+cycle.ID.String()) {
		t.Errorf("the cycle was deleted anyway: %v", h.trace)
	}
}

func TestACommentWithAReplyUnderneathIsLeftStanding(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	comment := entity.IssueComment{ID: uuid.New(), IssueID: uuid.New(), Body: "The header is the worst of it"}
	comment.Replies = []entity.IssueComment{{ID: uuid.New(), ParentCommentID: comment.ID}}

	h.comments.EXPECT().
		GetByID(gomock.Any(), h.run().WorkspaceID, comment.ID).
		Return(comment, nil)
	h.comments.EXPECT().
		ListThread(gomock.Any(), comment.IssueID, gomock.Any(), gomock.Any()).
		Return([]entity.IssueComment{comment}, nil)

	h.alreadyMade(entity.ImportLedgerEntry{
		Resource:   entity.ImportComment,
		CreatedID:  comment.ID,
		ExternalID: sourceComment,
		Reference:  comment.ID.String(),
	})

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceComment); outcome != entity.ImportOutcomeSkippedInUse {
		t.Fatalf(
			"a comment somebody had replied to was recorded as %q. Removing it would orphan a "+
				"reply written here, after the import, by somebody reading a conversation that "+
				"would then start halfway through.",
			outcome,
		)
	}
}

func TestATeamTheImportMadeIsArchivedRatherThanDeleted(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	team := teamNamed("Core")

	h.teams.EXPECT().GetByID(gomock.Any(), team.ID).Return(team, nil)
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	h.teamMembers.EXPECT().ListByTeamID(gomock.Any(), team.ID).Return(nil, nil)
	h.teams.EXPECT().
		Archive(gomock.Any(), team.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, at time.Time) (entity.Team, error) {
			h.followed("archive " + id.String())

			team.Status = entity.TeamStatusArchived
			team.ArchivedAt = &at

			return team, nil
		})

	h.alreadyMade(entity.ImportLedgerEntry{
		Resource:   entity.ImportTeam,
		CreatedID:  team.ID,
		ExternalID: sourceTeam,
		Reference:  team.Key,
	})

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceTeam); outcome != entity.ImportOutcomeArchived {
		t.Fatalf(
			"a team the import created was recorded as %q. A team carries its key, its number "+
				"series and every reference ever written to one of its issues; archiving takes it "+
				"off the board without making those references point at nothing.",
			outcome,
		)
	}
}

func TestOneEntryFailingDoesNotAbandonTheRestOfTheWalk(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	held := unwinding(h)

	issue := held.carrying("PLA", 1)
	relation := uuid.New()

	h.relations.EXPECT().
		GetByID(gomock.Any(), h.run().WorkspaceID, relation).
		Return(entity.StoredIssueRelation{}, errors.New("connection reset"))

	h.alreadyMade(
		issueEntry(issue, "issue-kept"),
		entity.ImportLedgerEntry{
			Resource:   entity.ImportIssueRelation,
			CreatedID:  relation,
			ExternalID: sourceRelation,
			Reference:  "issue-kin relates_to issue-hub",
		},
	)

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceRelation); outcome != entity.ImportOutcomeFailed {
		t.Errorf("the entry that went wrong was recorded as %q, want failed", outcome)
	}

	if !slices.Contains(h.trace, "delete "+issue.Reference()) {
		t.Fatalf(
			"one entry failing left the rest of the ledger untouched: %v. A revert that stops at "+
				"the first row it cannot undo leaves the workspace holding most of an import that "+
				"somebody has already decided to take back.",
			h.trace,
		)
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted with the failure named in the report", h.run().Status)
	}

	line, found := h.lineFor(sourceRelation)

	if !found || line.Outcome != entity.ImportOutcomeFailed {
		t.Errorf("the report never names the entry that failed")
	}
}

func TestARevertThatRunsTwiceTakesNothingOutTheSecondTime(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	held := unwinding(h)
	issue := held.carrying("PLA", 1)

	h.alreadyMade(issueEntry(issue, "issue-once"))

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	h.trace = nil
	h.at(entity.ImportReverting)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("second revert: %v", err)
	}

	if len(h.trace) != 0 {
		t.Fatalf(
			"a revert re-driven after its own settlement was lost went back and did %v. Every "+
				"entry is marked reverted in the same transaction that recorded the outcome, and "+
				"the walk only returns entries that are not, so a retried slice finds nothing "+
				"left to undo rather than deleting whatever now occupies those identifiers.",
			h.trace,
		)
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestAnImportThatFailedHalfwayCanStillBeTakenBackOut(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	held := unwinding(h)
	issue := held.carrying("PLA", 1)

	h.alreadyMade(issueEntry(issue, "issue-half"))

	h.at(entity.ImportFailed)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if !slices.Contains(h.trace, "delete "+issue.Reference()) {
		t.Fatalf(
			"a failed import was left standing: %v. A run that stopped halfway had already "+
				"created everything the ledger names, and the ledger exists precisely so that "+
				"half an import can be undone.",
			h.trace,
		)
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestARevertIsQueuedForWhoeverAsksForItRatherThanWhoRanTheImport(t *testing.T) {
	h := newHarness(t).backed()

	reverter := uuid.New()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: reverter},
			Role:  entity.MembershipRoleAdmin,
			Scope: entity.TeamScope{WorkspaceID: h.run().WorkspaceID, AllTeams: true},
		}, nil).
		AnyTimes()

	h.at(entity.ImportFailed)

	var queued entity.ImportRevertPayload

	h.jobs.EXPECT().
		EnqueueImportRevert(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			payload entity.ImportRevertPayload,
			_ time.Time,
		) error {
			queued = payload

			return nil
		})

	if _, err := h.imports.Revert(context.Background(), h.run().WorkspaceID, h.run().ID); err != nil {
		t.Fatalf("revert: %v", err)
	}

	if queued.ImportRunID != h.run().ID {
		t.Fatalf("queued a revert of %s, want %s", queued.ImportRunID, h.run().ID)
	}

	if h.run().RevertedByAccount != reverter {
		t.Fatalf(
			"the run remembers %v as the reverter, want %v. The walk deletes issues and archives "+
				"teams as whoever asked for it, and the account that ran the import months ago "+
				"may have left the workspace since.",
			h.run().RevertedByAccount, reverter,
		)
	}
}

func TestAnImportStillRunningCannotBeReverted(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	for _, status := range []entity.ImportStatus{
		entity.ImportDraft,
		entity.ImportStaged,
		entity.ImportMapped,
		entity.ImportExecuting,
		entity.ImportReverted,
	} {
		h.at(status)

		_, err := h.imports.Revert(context.Background(), h.run().WorkspaceID, h.run().ID)

		if !errors.Is(err, entity.ErrImportNotRevertible) {
			t.Errorf(
				"reverting a run at %q returned %v. A run that has not finished either has a "+
					"worker still writing to it or has already given everything back, and "+
					"walking its ledger now would race that worker over the same rows.",
				status, err,
			)
		}
	}
}

func (s *applied) matching(page entity.IssuePage) []entity.Issue {
	found := make([]entity.Issue, 0, len(s.held))

	for _, issue := range s.held {
		if page.CycleID != nil && issue.CycleID != *page.CycleID {
			continue
		}

		if page.ProjectID != nil && issue.ProjectID != *page.ProjectID {
			continue
		}

		if page.Filter != nil && !filterHolds(*page.Filter, issue) {
			continue
		}

		found = append(found, issue)
	}

	if page.Limit > 0 && len(found) > page.Limit {
		found = found[:page.Limit]
	}

	return found
}

func filterHolds(filter entity.IssueFilter, issue entity.Issue) bool {
	if filter.Field != entity.IssueFilterFieldState || filter.Op != entity.IssueFilterOpIs {
		return true
	}

	return slices.Contains(filter.Values, issue.State.ID.String())
}

func (s *applied) wearers(labelID uuid.UUID) int {
	worn := 0

	for _, issue := range s.held {
		for _, label := range issue.Labels {
			if label.ID == labelID {
				worn++
			}
		}
	}

	return worn
}

func (s *applied) labelNamed(t *testing.T, name string) entity.Label {
	t.Helper()

	for _, label := range s.standingLabels {
		if label.Name == name {
			return label
		}
	}

	t.Fatalf("no label named %q was created", name)

	return entity.Label{}
}

func (s *applied) leftStanding() []string {
	named := make([]string, 0)

	for _, issue := range s.held {
		named = append(named, "issue "+issue.Reference())
	}

	for _, label := range s.standingLabels {
		named = append(named, "label "+label.Name)
	}

	for _, group := range s.standingGroups {
		named = append(named, "label group "+group.Name)
	}

	for _, project := range s.standingProjects {
		named = append(named, "project "+project.Slug)
	}

	for _, state := range s.standingStates {
		named = append(named, "state "+state.Name)
	}

	for _, cycle := range s.standingCycles {
		named = append(named, "cycle "+strconv.Itoa(cycle.Number))
	}

	slices.Sort(named)

	return named
}

// reverting answers the reads and deletes the walk makes on everything the apply pass built,
// out of the same rows applying handed back. Nothing here decides whether a row may go: that
// is what is under test, and a fake that keeps its own view of which rows are the import's
// would answer the question for it.
func reverting(h *harness, made *applied) {
	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ entity.TeamScope,
			page entity.IssuePage,
		) ([]entity.Issue, error) {
			return made.matching(page), nil
		}).
		AnyTimes()

	h.labels.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, labelID uuid.UUID) (entity.Label, error) {
			label, standing := made.standingLabels[labelID]
			if !standing {
				return entity.Label{}, entity.ErrLabelNotFound
			}

			return label, nil
		}).
		AnyTimes()

	h.labels.EXPECT().
		ListByWorkspaceID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			_ entity.TeamScope,
		) ([]entity.Label, error) {
			return slices.Collect(maps.Values(made.standingLabels)), nil
		}).
		AnyTimes()

	h.labels.EXPECT().
		Usage(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			labelID uuid.UUID,
			_ entity.TeamScope,
		) (entity.LabelUsage, error) {
			return entity.LabelUsage{Issues: made.wearers(labelID)}, nil
		}).
		AnyTimes()

	h.labels.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, labelID uuid.UUID) error {
			h.followed("delete label " + made.standingLabels[labelID].Name)

			delete(made.standingLabels, labelID)

			return nil
		}).
		AnyTimes()

	h.groups.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, groupID uuid.UUID) (entity.LabelGroup, error) {
			group, standing := made.standingGroups[groupID]
			if !standing {
				return entity.LabelGroup{}, entity.ErrLabelGroupNotFound
			}

			return group, nil
		}).
		AnyTimes()

	h.groups.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, groupID uuid.UUID) error {
			h.followed("delete label group " + made.standingGroups[groupID].Name)

			delete(made.standingGroups, groupID)

			return nil
		}).
		AnyTimes()

	h.projects.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, projectID uuid.UUID) (entity.Project, error) {
			project, standing := made.standingProjects[projectID]
			if !standing {
				return entity.Project{}, entity.ErrProjectNotFound
			}

			return project, nil
		}).
		AnyTimes()

	h.projects.EXPECT().
		HasConcealedWork(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil).
		AnyTimes()

	h.projects.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, projectID uuid.UUID) error {
			h.followed("delete project " + made.standingProjects[projectID].Slug)

			delete(made.standingProjects, projectID)

			return nil
		}).
		AnyTimes()

	h.cycles.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, cycleID uuid.UUID,
			_ entity.TeamScope,
		) (entity.Cycle, error) {
			cycle, standing := made.standingCycles[cycleID]
			if !standing {
				return entity.Cycle{}, entity.ErrCycleNotFound
			}

			return cycle, nil
		}).
		AnyTimes()

	h.cycles.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cycleID uuid.UUID) error {
			h.followed("delete cycle " + strconv.Itoa(made.standingCycles[cycleID].Number))

			delete(made.standingCycles, cycleID)

			return nil
		}).
		AnyTimes()

	h.states.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, stateID uuid.UUID) error {
			h.followed("delete state " + made.standingStates[stateID].Name)

			delete(made.standingStates, stateID)

			return nil
		}).
		AnyTimes()
}

// importedFromCSV runs a whole import of rows that carry no dates of their own and leaves the
// run ready to be taken back out. Every row it creates is therefore stamped by its repository
// from a clock read inside the applying transaction, which is the one arrangement in which a
// ledger entry dated by the database — where now() is frozen at the transaction's start —
// lands a hair before the rows it names.
func importedFromCSV(t *testing.T, h *harness, team entity.Team) *applied {
	t.Helper()

	h.stageEverything(newUndatedSource(t))
	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if h.run().Status != entity.ImportImported {
		t.Fatalf("the import ended %q, so there is nothing here to take back out", h.run().Status)
	}

	reverting(h, made)

	h.trace = nil
	h.at(entity.ImportImported)

	return made
}

func TestAnImportedIssueIsDeletedOutrightRatherThanSentThroughTheDelayedPurge(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	held := unwinding(h)
	issue := held.carrying("PLA", 1)

	h.alreadyMade(issueEntry(issue, "issue-plain"))

	h.at(entity.ImportImported)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, "issue-plain"); outcome != entity.ImportOutcomeDeleted {
		t.Fatalf(
			"an issue this import created and nobody has touched since came out of the revert as "+
				"%q. The delayed purge finishes a deletion somebody asked for and takes only a row "+
				"already soft-deleted with its grace period spent; an imported issue is active and "+
				"has no deadline on it, so a revert routed through that path refuses every issue it "+
				"walks and hands back a run marked reverted with the whole backlog still standing.",
			outcome,
		)
	}

	if !slices.Equal(held.purged, []uuid.UUID{issue.ID}) {
		t.Fatalf(
			"the revert deleted %v, want exactly the one id the ledger names (%v). A delete this "+
				"direct has no grace period behind it, so the ids it is handed are the only thing "+
				"standing between a revert and somebody else's work.",
			held.purged, issue.ID,
		)
	}

	if _, standing := held.issues[issue.ID]; standing {
		t.Fatalf(
			"%s is still there after a revert that reported success. Somebody asked for the import "+
				"to be taken back out, was told it had been, and their workspace is unchanged.",
			issue.Reference(),
		)
	}
}

func TestAnImportWhoseRowsCarriedNoDatesIsStillTakenBackOutWhole(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedFromCSV(t, h, team)

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if standing := made.leftStanding(); len(standing) != 0 {
		t.Fatalf(
			"the revert left %v standing. Nobody had touched any of it: the run created every one "+
				"of these rows minutes earlier and the ledger names them all. A row is judged "+
				"modified by comparing its own timestamp against what the ledger recorded, so a "+
				"ledger that dates itself from the transaction clock rather than from the row "+
				"reads every row as edited a moment later by somebody who was never there, and the "+
				"revert hands back an untouched workspace and calls it undone.",
			standing,
		)
	}

	for _, entry := range h.world.ledger {
		if entry.RevertOutcome != entity.ImportOutcomeDeleted {
			t.Errorf(
				"the %s %q was recorded as %q on the way out, want deleted",
				entry.Resource, entry.Reference, entry.RevertOutcome,
			)
		}
	}

	if h.run().Status != entity.ImportReverted {
		t.Errorf("run ended %q, want reverted", h.run().Status)
	}
}

func TestALabelSomebodyRenamedAfterTheImportIsLeftAlone(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedFromCSV(t, h, team)

	renamed := made.labelNamed(t, "Chore")
	renamed.Name = "Housekeeping"
	renamed.UpdatedAt = time.Now().UTC()
	made.standingLabels[renamed.ID] = renamed

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceLabelWork); outcome != entity.ImportOutcomeSkippedModified {
		t.Fatalf(
			"a label somebody had renamed after the import was recorded as %q. The import made the "+
				"label, but the name on it now is somebody's own decision, and deleting it takes "+
				"that away along with the label from every issue wearing it.",
			outcome,
		)
	}

	if _, standing := made.standingLabels[renamed.ID]; !standing {
		t.Fatalf("the renamed label was deleted anyway: %v", h.trace)
	}

	if outcome := outcomeOf(t, h, sourceLabelBug); outcome != entity.ImportOutcomeDeleted {
		t.Errorf(
			"the untouched label beside it was recorded as %q. One label somebody worked on does "+
				"not make the rest of the import theirs.",
			outcome,
		)
	}

	if outcome := outcomeOf(t, h, sourceGroup); outcome != entity.ImportOutcomeSkippedInUse {
		t.Errorf(
			"the group still holding that label was recorded as %q. Deleting a group out from "+
				"under a label somebody kept leaves their label loose in the workspace.",
			outcome,
		)
	}
}

func TestALabelStillWornByWorkTheImportNeverMadeSurvivesTheRevert(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedFromCSV(t, h, team)
	bug := made.labelNamed(t, "Bug")

	own := entity.Issue{
		ID:           uuid.New(),
		WorkspaceID:  h.run().WorkspaceID,
		TeamID:       team.ID,
		ReferenceKey: team.Key,
		Number:       99,
		Version:      1,
		Title:        "Filed here, after the import",
		Status:       entity.IssueStatusActive,
		Labels:       []entity.Label{{ID: bug.ID}},
	}
	made.held[own.ID] = own

	if err := h.runner.RunRevert(context.Background(), revertPayload(h)); err != nil {
		t.Fatalf("run revert: %v", err)
	}

	if outcome := outcomeOf(t, h, sourceLabelBug); outcome != entity.ImportOutcomeSkippedInUse {
		t.Fatalf(
			"a label worn by an issue this import never made was recorded as %q on the way out. "+
				"The label came from the import but the work wearing it did not, and deleting it "+
				"strips that issue of a label somebody chose deliberately.",
			outcome,
		)
	}

	if _, standing := made.standingLabels[bug.ID]; !standing {
		t.Fatalf("the label was deleted anyway: %v", h.trace)
	}

	if _, standing := made.held[own.ID]; !standing {
		t.Fatalf("the revert deleted %s, which no ledger entry names", own.Reference())
	}
}
