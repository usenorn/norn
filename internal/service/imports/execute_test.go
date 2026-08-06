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
	cycles   []service.CreateCycleInput
	issues   []service.CreateIssueInput
	comments []service.PostCommentInput
	files    []service.AdoptAttachmentInput

	held   map[uuid.UUID]entity.Issue
	said   map[uuid.UUID]entity.IssueComment
	stored map[uuid.UUID]entity.Attachment

	standingTeams     map[string]entity.Team
	standingStates    map[uuid.UUID]entity.WorkflowState
	standingGroups    map[uuid.UUID]entity.LabelGroup
	standingLabels    map[uuid.UUID]entity.Label
	standingProjects  map[uuid.UUID]entity.Project
	standingCycles    map[uuid.UUID]entity.Cycle
	standingRelations map[uuid.UUID]entity.IssueRelation

	purged []uuid.UUID
	refuse error
}

// The refusals below are the ones a workspace produces by already holding something: every one
// of them reaches the apply path as a translated unique or exclusion violation, and every one
// of them is a statement Postgres has rejected, so the transaction carrying it is aborted from
// that point on. A workspace with anything in it, or a source imported twice, produces them on
// ordinary data.
func (s *applied) refuseTeam(key string) error {
	if _, standing := s.standingTeams[key]; standing {
		return entity.ErrTeamKeyTaken
	}

	return nil
}

func (s *applied) refuseState(teamID uuid.UUID, name string) error {
	for _, standing := range s.standingStates {
		if standing.TeamID == teamID && standing.Name == name {
			return entity.ErrWorkflowStateNameTaken
		}
	}

	return nil
}

func (s *applied) refuseGroup(name string) error {
	for _, standing := range s.standingGroups {
		if standing.Name == name {
			return entity.ErrLabelGroupNameTaken
		}
	}

	return nil
}

func (s *applied) refuseLabel(teamID uuid.UUID, name string) error {
	for _, standing := range s.standingLabels {
		if standing.TeamID == teamID && standing.Name == name {
			return entity.ErrLabelNameTaken
		}
	}

	return nil
}

func (s *applied) refuseProject(slug string) error {
	if slug == "" {
		return nil
	}

	for _, standing := range s.standingProjects {
		if standing.Slug == slug {
			return entity.ErrProjectSlugTaken
		}
	}

	return nil
}

func (s *applied) refuseCycle(teamID uuid.UUID, startsOn, endsOn string) error {
	for _, standing := range s.standingCycles {
		if standing.TeamID == teamID && standing.StartsOn <= endsOn && startsOn <= standing.EndsOn {
			return entity.ErrCycleOverlaps
		}
	}

	return nil
}

// rowStamp is what every repository does to a row on the way in, and the fakes here have to
// do the same: a row carries the source's dates when the origin is attributed and the clock
// read inside the transaction when it is not. A fake that leaves a created row's timestamps
// at zero can never notice a ledger entry that dates itself rather than the row it names.
func rowStamp(origin *entity.ImportOrigin) (time.Time, time.Time) {
	return entity.OriginStamp(origin, time.Now().UTC())
}

func applying(h *harness, team entity.Team) *applied {
	return applyingInto(h, team, newWorkspace())
}

func newWorkspace() *applied {
	return &applied{
		held:              map[uuid.UUID]entity.Issue{},
		said:              map[uuid.UUID]entity.IssueComment{},
		stored:            map[uuid.UUID]entity.Attachment{},
		standingTeams:     map[string]entity.Team{},
		standingStates:    map[uuid.UUID]entity.WorkflowState{},
		standingGroups:    map[uuid.UUID]entity.LabelGroup{},
		standingLabels:    map[uuid.UUID]entity.Label{},
		standingProjects:  map[uuid.UUID]entity.Project{},
		standingCycles:    map[uuid.UUID]entity.Cycle{},
		standingRelations: map[uuid.UUID]entity.IssueRelation{},
	}
}

func applyingInto(h *harness, team entity.Team, made *applied) *applied {
	h.teamWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateTeamInput) (entity.Team, error) {
			if err := h.statement(made.refuseTeam(input.Key)); err != nil {
				return entity.Team{}, err
			}

			made.teams = append(made.teams, input)

			standing := team
			standing.CreatedAt, standing.UpdatedAt = rowStamp(nil)

			made.standingTeams[input.Key] = standing

			return standing, nil
		}).
		AnyTimes()

	h.stateWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			input service.CreateWorkflowStateInput,
		) (entity.WorkflowState, error) {
			if err := h.statement(made.refuseState(input.TeamID, input.Name)); err != nil {
				return entity.WorkflowState{}, err
			}

			made.states = append(made.states, input)

			state := entity.WorkflowState{
				ID: uuid.New(), Name: input.Name, TeamID: input.TeamID, Category: input.Category,
			}
			state.CreatedAt, state.UpdatedAt = rowStamp(input.Origin)

			made.standingStates[state.ID] = state

			return state, nil
		}).
		AnyTimes()

	h.labelWriter.EXPECT().
		CreateGroup(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID, name string) (entity.LabelGroup, error) {
			if err := h.statement(made.refuseGroup(name)); err != nil {
				return entity.LabelGroup{}, err
			}

			group := entity.LabelGroup{ID: uuid.New(), WorkspaceID: workspaceID, Name: name}
			group.CreatedAt, group.UpdatedAt = rowStamp(nil)

			made.standingGroups[group.ID] = group

			return group, nil
		}).
		AnyTimes()

	h.labelWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateLabelInput) (entity.Label, error) {
			if err := h.statement(made.refuseLabel(input.TeamID, input.Name)); err != nil {
				return entity.Label{}, err
			}

			made.labels = append(made.labels, input)

			label := entity.Label{
				ID: uuid.New(), WorkspaceID: input.WorkspaceID, TeamID: input.TeamID,
				GroupID: input.GroupID, Name: input.Name, Color: input.Color, Origin: input.Origin,
			}
			label.CreatedAt, label.UpdatedAt = rowStamp(input.Origin)

			made.standingLabels[label.ID] = label

			return label, nil
		}).
		AnyTimes()

	h.projectWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateProjectInput) (service.ProjectView, error) {
			if err := h.statement(made.refuseProject(input.Slug)); err != nil {
				return service.ProjectView{}, err
			}

			made.projects = append(made.projects, input)

			project := entity.Project{
				ID: uuid.New(), WorkspaceID: input.WorkspaceID, Slug: input.Slug, Name: input.Name,
			}
			project.CreatedAt, project.UpdatedAt = rowStamp(input.Origin)

			made.standingProjects[project.ID] = project

			return service.ProjectView{Project: project}, nil
		}).
		AnyTimes()

	h.cycleWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateCycleInput) (service.CycleView, error) {
			if !entity.OriginAttributed(input.Origin) {
				return service.CycleView{}, entity.ErrCycleCreateRequiresOrigin
			}

			refusal := made.refuseCycle(input.TeamID, input.StartsOn, input.EndsOn)

			if err := h.statement(refusal); err != nil {
				return service.CycleView{}, err
			}

			made.cycles = append(made.cycles, input)

			cycle := entity.Cycle{
				ID:          uuid.New(),
				WorkspaceID: input.WorkspaceID,
				TeamID:      input.TeamID,
				TeamKey:     team.Key,
				Number:      len(made.cycles),
				StartsOn:    input.StartsOn,
				EndsOn:      input.EndsOn,
				ClosedAt:    input.ClosedAt,
			}
			cycle.CreatedAt, cycle.UpdatedAt = rowStamp(input.Origin)

			made.standingCycles[cycle.ID] = cycle

			return service.CycleView{Cycle: cycle, Phase: entity.CyclePhaseEnded}, nil
		}).
		AnyTimes()

	h.issueWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
			if err := h.statement(nil); err != nil {
				return entity.Issue{}, err
			}

			made.issues = append(made.issues, input)

			issue := entity.Issue{
				ID:           uuid.New(),
				WorkspaceID:  input.WorkspaceID,
				TeamID:       input.TeamID,
				ReferenceKey: team.Key,
				Number:       len(made.issues),
				Version:      1,
				Title:        input.Title,
				Description:  input.Description,
				Status:       entity.IssueStatusActive,
				State:        entity.IssueState{ID: input.StateID},
				CycleID:      input.CycleID,
				ProjectID:    input.ProjectID,
				Labels:       wearing(input.LabelIDs),
			}
			issue.CreatedAt, issue.UpdatedAt = rowStamp(input.Origin)

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
			if err := h.statement(nil); err != nil {
				return entity.Issue{}, err
			}

			issue := made.held[issueID]
			issue.Version++
			issue.ParentIssueID = *input.ParentID
			issue.UpdatedAt = time.Now().UTC()
			made.held[issueID] = issue

			return issue, nil
		}).
		AnyTimes()

	h.issueWriter.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, issueID uuid.UUID,
			input service.UpdateIssueInput,
		) (entity.Issue, error) {
			issue := made.held[issueID]

			if input.ExpectedVersion != issue.Version {
				return entity.Issue{}, entity.IssueStaleError{
					Version:   issue.Version,
					Conflicts: []string{entity.IssueFieldDescription},
				}
			}

			if input.Description != nil {
				issue.Description = *input.Description
			}

			issue.Version++
			issue.UpdatedAt = time.Now().UTC()
			made.held[issueID] = issue

			h.followed("rewrite " + issue.Reference())

			return issue, nil
		}).
		AnyTimes()

	h.fileWriter.EXPECT().
		Adopt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			workspaceID, issueID uuid.UUID,
			input service.AdoptAttachmentInput,
		) (entity.Attachment, error) {
			if err := h.statement(made.refuse); err != nil {
				return entity.Attachment{}, err
			}

			made.files = append(made.files, input)

			file := entity.Attachment{
				ID:          uuid.New(),
				WorkspaceID: workspaceID,
				IssueID:     issueID,
				CommentID:   input.CommentID,
				ObjectKey:   input.ObjectKey,
				FileName:    input.FileName,
				ContentType: entity.AttachmentServedType(input.ContentType),
				SizeBytes:   input.SizeBytes,
				Status:      entity.AttachmentStatusStored,
				Origin:      input.Origin,
			}
			file.CreatedAt, file.UpdatedAt = rowStamp(input.Origin)

			made.stored[file.ID] = file

			return file, nil
		}).
		AnyTimes()

	h.files.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, fileID uuid.UUID,
		) (entity.Attachment, error) {
			file, kept := made.stored[fileID]
			if !kept {
				return entity.Attachment{}, entity.ErrAttachmentNotFound
			}

			return file, nil
		}).
		AnyTimes()

	h.fileWriter.EXPECT().
		Remove(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, fileID uuid.UUID) error {
			h.followed("discard " + made.stored[fileID].FileName)

			delete(made.stored, fileID)

			return nil
		}).
		AnyTimes()

	h.comments.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, commentID uuid.UUID,
		) (entity.IssueComment, error) {
			comment, kept := made.said[commentID]
			if !kept {
				return entity.IssueComment{}, entity.ErrIssueCommentNotFound
			}

			return comment, nil
		}).
		AnyTimes()

	h.commentWriter.EXPECT().
		Edit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _, commentID uuid.UUID,
			input service.EditCommentInput,
		) (entity.IssueComment, error) {
			comment := made.said[commentID]

			if !comment.EditableBy(h.run().RequestedByAccount) {
				return entity.IssueComment{}, entity.ErrIssueCommentNotAuthor
			}

			edited := time.Now().UTC()
			comment.Body = input.Body
			comment.EditedAt = &edited
			comment.UpdatedAt = edited
			made.said[commentID] = comment

			h.followed("rewrite comment " + commentID.String())

			return comment, nil
		}).
		AnyTimes()

	h.relationWriter.EXPECT().
		Add(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _ uuid.UUID,
			input service.AddIssueRelationInput,
		) (entity.IssueRelation, error) {
			if err := h.statement(nil); err != nil {
				return entity.IssueRelation{}, err
			}

			relation := entity.IssueRelation{
				ID: uuid.New(), Kind: input.Kind, CreatedAt: time.Now().UTC(),
			}

			made.standingRelations[relation.ID] = relation

			return relation, nil
		}).
		AnyTimes()

	h.commentWriter.EXPECT().
		Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			workspaceID, issueID uuid.UUID,
			input service.PostCommentInput,
		) (service.CommentPosted, error) {
			if err := h.statement(nil); err != nil {
				return service.CommentPosted{}, err
			}

			made.comments = append(made.comments, input)

			comment := entity.IssueComment{
				ID:              uuid.New(),
				WorkspaceID:     workspaceID,
				IssueID:         issueID,
				ParentCommentID: input.ParentCommentID,
				AuthorAccountID: entity.OriginAuthor(input.Origin, h.run().RequestedByAccount),
				Body:            input.Body,
				Origin:          input.Origin,
			}
			comment.CreatedAt, comment.UpdatedAt = rowStamp(input.Origin)

			made.said[comment.ID] = comment

			return service.CommentPosted{Comment: comment}, nil
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

	purging(h, made.held, &made.purged)

	return made
}

func wearing(labelIDs []uuid.UUID) []entity.Label {
	worn := make([]entity.Label, 0, len(labelIDs))

	for _, id := range labelIDs {
		worn = append(worn, entity.Label{ID: id})
	}

	return worn
}

// purging answers for both ways out of an issue, because which one the revert reaches for is
// the whole question. Purge finishes a deletion somebody asked for and its WHERE clause takes
// only a row already pending deletion with its grace period spent, so an imported issue —
// active, with no deadline on it — is refused every single time. PurgeImported is the revert's
// own delete and takes the rows the ledger names, subject to the one thing the database will
// not do quietly: workspace_issues clears a child's parent on delete and then refuses the
// child for having a depth with no parent.
func purging(h *harness, issues map[uuid.UUID]entity.Issue, purged *[]uuid.UUID) {
	h.issues.EXPECT().
		Purge(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, issueID uuid.UUID, _ time.Time) error {
			if issues[issueID].Status != entity.IssueStatusPendingDeletion {
				return entity.ErrIssuePurgeNotDue
			}

			h.followed("delete " + issues[issueID].Reference())

			delete(issues, issueID)

			return nil
		}).
		AnyTimes()

	h.issues.EXPECT().
		PurgeImported(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			workspaceID uuid.UUID,
			ids []uuid.UUID,
		) (int, error) {
			if workspaceID != h.run().WorkspaceID {
				h.t.Errorf(
					"the revert purged in workspace %s, want %s. A delete this direct is safe only "+
						"because it is fenced to the workspace the ledger belongs to.",
					workspaceID, h.run().WorkspaceID,
				)
			}

			removed := 0

			for _, id := range ids {
				issue, standing := issues[id]

				if !standing || issue.Status != entity.IssueStatusActive {
					continue
				}

				for _, child := range issues {
					if child.ParentIssueID == id {
						return 0, fmt.Errorf(
							"delete issue %s: %s is left at depth with no parent",
							issue.Reference(), child.Reference(),
						)
					}
				}

				h.followed("delete " + issue.Reference())

				*purged = append(*purged, id)

				delete(issues, id)

				removed++
			}

			return removed, nil
		}).
		AnyTimes()
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

func (s *applied) issueCreated(t *testing.T, title string) service.CreateIssueInput {
	t.Helper()

	for _, input := range s.issues {
		if input.Title == title {
			return input
		}
	}

	t.Fatalf("nothing created an issue titled %q", title)

	return service.CreateIssueInput{}
}

func importedFully(t *testing.T, h *harness, team entity.Team) *applied {
	t.Helper()

	h.offering(newStaticSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	answeredFully(h, answers{accounts: map[string]uuid.UUID{
		rae.Key: uuid.New(),
		ida.Key: uuid.New(),
	}})

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	return made
}

func TestACycleTheSourceRanArrivesWithTheIssueThatWasPlannedIntoIt(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedFully(t, h, team)

	if len(made.cycles) != 1 {
		t.Fatalf("the import created %d cycles, want the one the source ran", len(made.cycles))
	}

	window := made.cycles[0]

	if window.StartsOn != sourceCycleStartsOn || window.EndsOn != sourceCycleEndsOn {
		t.Errorf(
			"the cycle was created over %s → %s, want the source's own %s → %s. The window is the "+
				"whole of what a cycle is; moved by a day it no longer holds the work that was "+
				"planned into it.",
			window.StartsOn, window.EndsOn, sourceCycleStartsOn, sourceCycleEndsOn,
		)
	}

	if window.Number != 0 {
		t.Errorf(
			"the import asked for cycle number %d. A team numbers its cycles in the order it runs "+
				"them, so carrying the source's number over collides with the numbers this team "+
				"already has.",
			window.Number,
		)
	}

	entries := h.ledgerFor(entity.ImportCycle)

	if len(entries) != 1 {
		t.Fatalf("the ledger holds %d cycles, want the one that was created", len(entries))
	}

	planned := made.issueCreated(t, "Set the cadence").CycleID

	if planned != entries[0].CreatedID {
		t.Fatalf(
			"the issue that belonged to the source's cycle arrived in cycle %v, want the cycle "+
				"this run made (%v). Carrying the cycle and then filing its issues outside it "+
				"leaves the team an empty sprint and a backlog it has to replan by hand.",
			planned, entries[0].CreatedID,
		)
	}

	if loose := made.issueCreated(t, "Rework the hub").CycleID; loose != uuid.Nil {
		t.Errorf("an issue the source left out of every cycle arrived in cycle %v", loose)
	}
}

func TestACycleWhoseWindowHasAlreadyRunOutArrivesClosed(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := importedFully(t, h, team)

	if len(made.cycles) != 1 {
		t.Fatalf("the import created %d cycles, want the one the source ran", len(made.cycles))
	}

	closed := made.cycles[0].ClosedAt

	if closed == nil {
		t.Fatalf(
			"a cycle that ended on %s was created open. A team is shown its current cycle, and an "+
				"open cycle whose window is in the past answers that question forever: every "+
				"dashboard on the team would sit on a sprint the source finished, and nothing "+
				"generated afterwards would ever take its place.",
			sourceCycleEndsOn,
		)
	}

	if entity.FormatCalendarDate(*closed) <= sourceCycleEndsOn {
		t.Errorf(
			"the cycle was closed at %s, on or before the last day it covered (%s). Closing it "+
				"inside its own window reads as a sprint abandoned early rather than one that ran "+
				"its course.",
			closed, sourceCycleEndsOn,
		)
	}
}

func TestAnIssueKeepsItsPlaceWhenOnlyItsCycleCannotBeCarried(t *testing.T) {
	h := newHarness(t).backed().resolving(entity.ImportUnknownCreate)
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue, fetched(t, sourceIssueCadence, "", service.ImportIssuePayload{
		Title: "Set the cadence", Team: sourceTeam, Cycle: "cycle-never-exported",
	}))
	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if len(made.issues) != 1 {
		t.Fatalf(
			"the import created %d issues. An issue is worth more than the cycle it sat in: a "+
				"cycle the export left behind drops the membership, not the work.",
			len(made.issues),
		)
	}

	if planned := made.issues[0].CycleID; planned != uuid.Nil {
		t.Errorf("the issue arrived in cycle %v, want no cycle at all", planned)
	}

	line, found := h.lineOf(entity.ImportIssue, sourceIssueCadence)

	if !found || line.Outcome != entity.ImportOutcomeDropped {
		t.Fatalf(
			"the report says %q about an issue that lost its cycle. Nothing else records that it "+
				"was ever in one, so a line here is the only chance anybody has of noticing before "+
				"they go looking for a sprint that never came over.",
			line.Outcome,
		)
	}
}

func TestTheLedgerRemembersTheOrderTheImportBuiltThingsIn(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Core")

	h.scopedTo(team.ID)

	importedFully(t, h, team)

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

func TestACycleTheSourceNeverDatedIsCarriedFromTheDayItBegan(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)
	h.stage(entity.ImportCycle, entity.ImportRecord{
		ExternalID: sourceCycle,
		Payload: payloadOf(t, service.ImportCyclePayload{
			Team: sourceTeam, StartsOn: sourceCycleStartsOn, EndsOn: sourceCycleEndsOn,
		}),
	})
	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if len(made.cycles) != 1 {
		t.Fatalf(
			"a cycle carrying its window but no paperwork was not created. Creating a cycle is "+
				"reserved for an import and refuses an unattributed origin, and "+
				"ErrCycleCreateRequiresOrigin is not something a run recovers from: the record "+
				"settles failed rather than skipped, so a CSV — which carries a sprint's dates and "+
				"nothing else — would never import a cycle at all. Report state: %q.",
			h.run().Status,
		)
	}

	origin := made.cycles[0].Origin

	if origin == nil || !origin.Attributed() {
		t.Fatalf("the cycle was created with %#v, want an attributed origin", origin)
	}

	if entity.FormatCalendarDate(origin.CreatedAt) != sourceCycleStartsOn {
		t.Errorf(
			"the cycle is dated %s, want the day it started (%s). A cycle existed no later than "+
				"the day it began, so that date is derived rather than invented; stamping it with "+
				"the hour the import ran would file a sprint from 2024 as this week's work.",
			entity.FormatCalendarDate(origin.CreatedAt), sourceCycleStartsOn,
		)
	}
}
