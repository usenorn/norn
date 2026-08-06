package imports_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type applied struct {
	teams    []service.CreateTeamInput
	states   []service.CreateWorkflowStateInput
	labels   []service.CreateLabelInput
	projects []service.CreateProjectInput
	issues   []service.CreateIssueInput
	comments []service.PostCommentInput
	held     map[uuid.UUID]entity.Issue
}

func applying(h *harness, team entity.Team) *applied {
	made := &applied{held: map[uuid.UUID]entity.Issue{}}

	h.teamWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateTeamInput) (entity.Team, error) {
			made.teams = append(made.teams, input)

			return team, nil
		}).
		AnyTimes()

	h.stateWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			input service.CreateWorkflowStateInput,
		) (entity.WorkflowState, error) {
			made.states = append(made.states, input)

			return entity.WorkflowState{ID: uuid.New(), Name: input.Name, TeamID: input.TeamID}, nil
		}).
		AnyTimes()

	h.labelWriter.EXPECT().
		CreateGroup(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, name string) (entity.LabelGroup, error) {
			return entity.LabelGroup{ID: uuid.New(), Name: name}, nil
		}).
		AnyTimes()

	h.labelWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateLabelInput) (entity.Label, error) {
			made.labels = append(made.labels, input)

			return entity.Label{ID: uuid.New(), Name: input.Name, Color: input.Color}, nil
		}).
		AnyTimes()

	h.projectWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateProjectInput) (service.ProjectView, error) {
			made.projects = append(made.projects, input)

			return service.ProjectView{Project: entity.Project{
				ID: uuid.New(), Slug: input.Slug, Name: input.Name,
			}}, nil
		}).
		AnyTimes()

	h.issueWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
			made.issues = append(made.issues, input)

			issue := entity.Issue{
				ID:           uuid.New(),
				WorkspaceID:  input.WorkspaceID,
				TeamID:       input.TeamID,
				ReferenceKey: team.Key,
				Number:       len(made.issues),
				Version:      1,
				Title:        input.Title,
				Status:       entity.IssueStatusActive,
			}

			made.held[issue.ID] = issue

			return issue, nil
		}).
		AnyTimes()

	h.issueWriter.EXPECT().
		SetParent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, issueID uuid.UUID,
			input service.SetIssueParentInput,
		) (entity.Issue, error) {
			issue := made.held[issueID]
			issue.Version++
			issue.ParentIssueID = *input.ParentID
			made.held[issueID] = issue

			return issue, nil
		}).
		AnyTimes()

	h.relationWriter.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			input service.AddIssueRelationInput,
		) (entity.IssueRelation, error) {
			return entity.IssueRelation{ID: uuid.New(), Kind: input.Kind}, nil
		}).
		AnyTimes()

	h.commentWriter.EXPECT().
		Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			input service.PostCommentInput,
		) (service.CommentPosted, error) {
			made.comments = append(made.comments, input)

			return service.CommentPosted{Comment: entity.IssueComment{ID: uuid.New(), Body: input.Body}}, nil
		}).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, issueID uuid.UUID,
			_ entity.TeamScope,
		) (entity.Issue, error) {
			issue, carried := made.held[issueID]
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
			_ *uuid.UUID,
			_ int,
			_ time.Time,
		) error {
			issue := made.held[issueID]

			h.followed("unlink " + issue.Reference())

			issue.ParentIssueID = uuid.Nil
			issue.Version++
			made.held[issueID] = issue

			return nil
		}).
		AnyTimes()

	h.issues.EXPECT().
		Purge(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issueID uuid.UUID, _ time.Time) error {
			for _, standing := range made.held {
				if standing.ParentIssueID == issueID {
					return fmt.Errorf(
						"delete issue %s: %s is left at depth with no parent",
						made.held[issueID].Reference(), standing.Reference(),
					)
				}
			}

			h.followed("delete " + made.held[issueID].Reference())

			delete(made.held, issueID)

			return nil
		}).
		AnyTimes()

	return made
}

func executePayload(h *harness) entity.ImportExecutePayload {
	return entity.ImportExecutePayload{
		ImportRunID: h.run().ID,
		WorkspaceID: h.run().WorkspaceID,
		Attempt:     h.run().Attempt,
	}
}

func manyIssues(t *testing.T, count int) []entity.ImportRecord {
	t.Helper()

	records := make([]entity.ImportRecord, 0, count)

	for index := range count {
		records = append(records, fetched(t, fmt.Sprintf("issue-%03d", index), "",
			service.ImportIssuePayload{
				Title: fmt.Sprintf("Row %d of the backlog", index),
				Team:  sourceTeam,
			}))
	}

	return records
}

func onlyTheTeamIsDecided(h *harness, teamID uuid.UUID) *harness {
	return h.decided(entity.ImportMapping{
		Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
		Decision: entity.ImportDecisionMap, TargetID: teamID,
	})
}

func TestABacklogIsAppliedOneChunkAtATimeAndEachChunkIsItsOwnTransaction(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, manyIssues(t, 60)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if h.transactions != 3 {
		t.Fatalf(
			"applying sixty rows in chunks of %d took %d transactions, want 3. One transaction per "+
				"chunk is what keeps the creates, the ledger, the record states and the counter "+
				"agreeing with each other after a crash.",
			entity.ImportChunkSize, h.transactions,
		)
	}

	wanted := []int{25, 25, 10}

	for index, moved := range h.world.advances {
		if index >= len(wanted) || moved.processed != wanted[index] {
			t.Errorf("chunk %d advanced the run by %d, want %v", index+1, moved.processed, wanted)
		}
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q, want imported", h.run().Status)
	}
}

func TestAChunkThatNeverCommittedIsAppliedAgainAndOnlyOnce(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, manyIssues(t, 60)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)
	h.doomed = 2

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); !errors.Is(err, errLostCommit) {
		t.Fatalf("run execute returned %v, want the lost commit", err)
	}

	if len(h.world.ledger) != entity.ImportChunkSize {
		t.Fatalf(
			"the ledger holds %d entries after the second chunk failed to commit, want the %d from "+
				"the first. Rows the ledger admits to and rows that exist have to be the same set, "+
				"or a revert either misses them or deletes something it never made.",
			len(h.world.ledger), entity.ImportChunkSize,
		)
	}

	for index := entity.ImportChunkSize; index < 2*entity.ImportChunkSize; index++ {
		external := fmt.Sprintf("issue-%03d", index)

		if state := h.recordState(entity.ImportIssue, external); state != entity.ImportRecordStaged {
			t.Fatalf(
				"%s reads as %q after the chunk carrying it was lost. A record settled by a "+
					"transaction that never committed would never be walked again, so the row it "+
					"describes would silently never arrive.",
				external, state,
			)
		}
	}

	if h.run().Status != entity.ImportExecuting {
		t.Errorf(
			"a lost chunk moved the run to %q. The slice is retried by the queue; the import has "+
				"lost nothing it cannot redo.",
			h.run().Status,
		)
	}

	h.doomed = 0

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("second slice: %v", err)
	}

	if len(h.world.ledger) != 60 {
		t.Fatalf("the ledger holds %d entries after the retry, want 60 with nothing applied twice", len(h.world.ledger))
	}

	seen := map[string]int{}

	for _, entry := range h.world.ledger {
		seen[entry.ExternalID]++
	}

	for external, times := range seen {
		if times != 1 {
			t.Errorf(
				"%s is in the ledger %d times. The retry re-walked the chunk that was lost, and a "+
					"row it had already committed being applied again is a duplicate issue nobody "+
					"asked for.",
				external, times,
			)
		}
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q after the retry, want imported", h.run().Status)
	}
}

func TestASliceThatHasLostItsLeaseStopsBeforeWritingAnythingElse(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, manyIssues(t, 60)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)
	h.heartbeat = func() error {
		if len(h.world.ledger) >= entity.ImportChunkSize {
			return entity.ErrImportRunLeased
		}

		return nil
	}

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); !errors.Is(err, entity.ErrImportRunLeased) {
		t.Fatalf("run execute returned %v, want the lost lease", err)
	}

	if len(h.world.ledger) != entity.ImportChunkSize {
		t.Fatalf(
			"a worker whose lease had been taken applied %d rows, want it to have stopped at %d. "+
				"The heartbeat before each chunk is the only thing standing between a rescued run "+
				"and two workers importing the same backlog side by side.",
			len(h.world.ledger), entity.ImportChunkSize,
		)
	}

	if h.run().Status != entity.ImportExecuting {
		t.Errorf("run sat at %q after losing its lease, want executing under whoever holds it", h.run().Status)
	}
}

func TestASliceThatRunsOutOfTimeHandsOnToATaskThatCanBeQueuedBesideIt(t *testing.T) {
	cfg := testImportConfig()
	cfg.SliceBudget = time.Nanosecond

	h := newTunedHarness(t, cfg).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, manyIssues(t, 60)...)
	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)

	started := executePayload(h)

	var queued entity.ImportExecutePayload

	h.jobs.EXPECT().
		EnqueueImportExecute(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			payload entity.ImportExecutePayload,
			_ time.Time,
		) error {
			queued = payload

			return nil
		})

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if queued.Attempt != started.Attempt+1 {
		t.Fatalf(
			"the continuation carries attempt %d rather than %d. The queue keys an import slice on "+
				"the run and the attempt and drops a duplicate id, so a continuation reusing the "+
				"attempt of the slice that queued it would be swallowed and the run would stop "+
				"halfway with nothing to wake it.",
			queued.Attempt, started.Attempt+1,
		)
	}

	if h.run().Status != entity.ImportExecuting {
		t.Fatalf(
			"a slice that ran out of its budget left the run at %q. It stopped with sixty rows "+
				"still staged, and a run recorded as anything but executing is one nobody will "+
				"pick up again.",
			h.run().Status,
		)
	}
}

func TestAMappedAuthorReachesTheRowWithItsOwnAccountAndItsOwnDates(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Platform")
	author := uuid.New()

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
		Title: "Rework the hub", Team: sourceTeam, Assignee: rae, Author: rae,
	}))
	h.decided(
		entity.ImportMapping{
			Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
			Decision: entity.ImportDecisionMap, TargetID: team.ID,
		},
		entity.ImportMapping{
			Kind: entity.ImportMapUser, SourceKey: rae.Key,
			Decision: entity.ImportDecisionMap, TargetID: author,
		},
	)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if len(made.issues) != 1 {
		t.Fatalf("created %d issues, want 1", len(made.issues))
	}

	origin := made.issues[0].Origin

	if origin == nil || !origin.Attributed() {
		t.Fatalf(
			"the issue was created without an attributed origin. Every repository ignores an "+
				"origin that was not built by NewImportOrigin, so an unattributed one silently "+
				"stamps the row with now and credits it to whoever pressed the button: %#v",
			origin,
		)
	}

	if origin.AuthorAccountID != author {
		t.Errorf("the row was credited to %v, want the account %s was mapped onto (%v)", origin.AuthorAccountID, rae.Email, author)
	}

	if !origin.CreatedAt.Equal(sourceCreatedAt()) || !origin.UpdatedAt.Equal(sourceUpdatedAt()) {
		t.Errorf(
			"the row carries %s / %s, want the source's own %s / %s. An import that stamps "+
				"everything with the hour it ran destroys the one thing a migrated backlog is "+
				"read by, which is when the work actually happened.",
			origin.CreatedAt, origin.UpdatedAt, sourceCreatedAt(), sourceUpdatedAt(),
		)
	}

	if made.issues[0].AssigneeAccountID != author {
		t.Errorf("the issue was assigned to %v, want %v", made.issues[0].AssigneeAccountID, author)
	}
}

func TestAnAuthorNobodyClaimedIsLeftToTheImporterAndSaidSoOnce(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, fetched(t, sourceIssueDrift, "", service.ImportIssuePayload{
		Title: "Chase the drift", Team: sourceTeam, Assignee: otto, Author: otto,
	}))
	h.decided(
		entity.ImportMapping{
			Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
			Decision: entity.ImportDecisionMap, TargetID: team.ID,
		},
		entity.ImportMapping{
			Kind: entity.ImportMapUser, SourceKey: otto.Key,
			SourceLabel: otto.Name, Decision: entity.ImportDecisionUnattributed,
		},
	)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if len(made.issues) != 1 {
		t.Fatalf("created %d issues, want 1", len(made.issues))
	}

	origin := made.issues[0].Origin

	if origin == nil || origin.Attributes() {
		t.Fatalf(
			"an issue whose author was left unattributed still names an author (%#v). Attribution "+
				"is a claim about a person, and the person this names never agreed to be in this "+
				"workspace; the row falls back to the account that ran the import.",
			origin,
		)
	}

	if !origin.Attributed() {
		t.Error(
			"leaving the author unattributed also threw away the source's dates. Whose work it " +
				"was and when it happened are separate questions.",
		)
	}

	if made.issues[0].AssigneeAccountID != uuid.Nil {
		t.Errorf(
			"the issue arrived assigned to %v. There is nobody here to assign it to, and assigning "+
				"it to the importer would put somebody else's backlog on their plate.",
			made.issues[0].AssigneeAccountID,
		)
	}

	named := h.linesFor(entity.ImportOutcomeUnattributed)

	if len(named) != 1 {
		t.Fatalf("the report names %d unattributed rows, want exactly 1", len(named))
	}

	if named[0].Detail["source_user"] != otto.Name {
		t.Errorf(
			"the report names %q, want %q. The report is the only place the requester can see "+
				"whose work arrived without them, which is what a second pass at the mapping is "+
				"read from.",
			named[0].Detail["source_user"], otto.Name,
		)
	}
}

func TestATeamNobodyCanSeeIsDecidedOnceRatherThanOncePerRow(t *testing.T) {
	h := newHarness(t).backed()

	elsewhere := uuid.New()
	asked := 0

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.AccessRequest) (entity.Decision, error) {
			asked++

			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.run().RequestedByAccount},
				Role:  entity.MembershipRoleAdmin,
				Scope: entity.TeamScope{
					WorkspaceID: h.run().WorkspaceID,
					TeamIDs:     []uuid.UUID{uuid.New()},
				},
			}, nil
		}).
		AnyTimes()

	h.stage(entity.ImportIssue, manyIssues(t, 100)...)
	onlyTheTeamIsDecided(h, elsewhere)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if asked != 1 {
		t.Fatalf(
			"the run asked the authorizer %d times to apply 100 rows. A slice decides once, on the "+
				"requester it recorded, and carries the answer down; asking per row writes a "+
				"hundred denials into the audit log for one refusal a person can act on, and "+
				"buries the entries somebody is actually watching for.",
			asked,
		)
	}

	skipped := h.linesFor(entity.ImportOutcomeSkipped)

	if len(skipped) != 100 {
		t.Fatalf("the report holds %d skipped rows, want one for each of the 100", len(skipped))
	}

	for _, line := range skipped {
		if !strings.Contains(line.Detail["reason"], "not visible") {
			t.Fatalf("a row was skipped for %q, want the visibility reason", line.Detail["reason"])
		}
	}
}

func TestOneRowGoingWrongIsAnOutcomeRatherThanTheEndOfTheChunk(t *testing.T) {
	cases := []struct {
		name    string
		failure error
		state   entity.ImportRecordState
		outcome entity.ImportOutcome
	}{
		{
			name: "a row the workspace refuses",
			failure: entity.NewValidationError(entity.FieldError{
				Field: "description", Code: entity.ValidationCodeTooLong,
			}),
			state:   entity.ImportRecordSkipped,
			outcome: entity.ImportOutcomeSkipped,
		},
		{
			name:    "a row nothing could be done with",
			failure: errors.New("connection reset"),
			state:   entity.ImportRecordFailed,
			outcome: entity.ImportOutcomeFailed,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t).backed()
			team := teamNamed("Platform")

			h.scopedTo(team.ID)
			h.stage(entity.ImportIssue, manyIssues(t, 3)...)
			onlyTheTeamIsDecided(h, team.ID)

			created := 0

			h.issueWriter.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
					if strings.HasPrefix(input.Title, "Row 1 ") {
						return entity.Issue{}, testCase.failure
					}

					created++

					return entity.Issue{
						ID: uuid.New(), Title: input.Title, ReferenceKey: team.Key,
						Number: created, Version: 1,
					}, nil
				}).
				AnyTimes()

			h.at(entity.ImportMapped)

			if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
				t.Fatalf("run execute: %v", err)
			}

			if len(h.world.ledger) != 2 {
				t.Fatalf(
					"one row of three failed and the ledger holds %d entries, want the other two. "+
						"A backlog of any size contains rows this workspace will not accept, and "+
						"an import that stopped at the first would never finish.",
					len(h.world.ledger),
				)
			}

			if state := h.recordState(entity.ImportIssue, "issue-001"); state != testCase.state {
				t.Errorf("the refused row reads as %q, want %q", state, testCase.state)
			}

			line, found := h.lineFor("issue-001")

			if !found || line.Outcome != testCase.outcome {
				t.Errorf("the report says %q about the refused row, want %q", line.Outcome, testCase.outcome)
			}

			if h.run().Status != entity.ImportImported {
				t.Errorf("run ended %q, want imported with the failure recorded rather than raised", h.run().Status)
			}
		})
	}
}

func TestAParentTheExportLeftBehindIsSkippedRatherThanFatal(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.offering(sourceOfIssues(
		fetched(t, "issue-here", "", service.ImportIssuePayload{Title: "Here", Team: sourceTeam}),
		fetched(t, "issue-orphan", "issue-never-exported", service.ImportIssuePayload{
			Title: "Filed under something the export left behind", Team: sourceTeam,
		}),
	))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	onlyTheTeamIsDecided(h, team.ID)
	applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf(
			"an issue naming a parent the export never included ended the run: %v. A partial "+
				"export is the normal case, and a link that cannot be made is one line of the "+
				"report rather than a reason to abandon a backlog that arrived intact.",
			err,
		)
	}

	if made := len(h.ledgerFor(entity.ImportIssue)); made != 2 {
		t.Errorf("the ledger holds %d issues, want both of them", made)
	}

	if links := len(h.ledgerFor(entity.ImportIssueParent)); links != 0 {
		t.Errorf("the ledger claims %d parent links were made, want none", links)
	}

	if state := h.recordState(entity.ImportIssueParent, "issue-orphan"); state != entity.ImportRecordSkipped {
		t.Errorf("the link reads as %q, want skipped", state)
	}

	line, found := h.lineOf(entity.ImportIssueParent, "issue-orphan")

	if !found || line.Outcome != entity.ImportOutcomeSkipped {
		t.Fatalf(
			"nothing in the report says the hierarchy came over incomplete. The requester chose " +
				"what to export and is the only person who can decide whether a missing parent " +
				"matters.",
		)
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q, want imported", h.run().Status)
	}
}

func TestTheLedgerRemembersTheOrderTheImportBuiltThingsIn(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.offering(newStaticSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	answeredFully(h, answers{accounts: map[string]uuid.UUID{
		rae.Key: uuid.New(),
		ida.Key: uuid.New(),
	}})

	applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if h.run().Status != entity.ImportImported {
		t.Fatalf("run ended %q, want imported", h.run().Status)
	}

	reached := 0

	for _, entry := range h.world.ledger {
		at := indexOfPhase(t, entry.Resource)

		if at < reached {
			t.Fatalf(
				"%s was recorded after a %s. A revert walks this ledger backwards and undoes each "+
					"row in the reverse of the order it was made; out of order, it reaches an issue "+
					"before the link that hangs off it.",
				entry.Resource, entity.ImportPhases()[reached],
			)
		}

		reached = at
	}

	for _, resource := range entity.ImportPhases() {
		if len(h.ledgerFor(resource)) == 0 {
			t.Errorf(
				"nothing of the %s phase reached the ledger, so this pass never exercised the "+
					"branch that applies one",
				resource,
			)
		}
	}
}

func indexOfPhase(t *testing.T, resource entity.ImportResource) int {
	t.Helper()

	for at, phase := range entity.ImportPhases() {
		if phase == resource {
			return at
		}
	}

	t.Fatalf("%q is not a phase", resource)

	return 0
}
