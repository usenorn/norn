package imports

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
)

const (
	reasonTouchedSince = "this has changed since the import created it"
	reasonStillInUse   = "work that did not come from this import still depends on it"
	reasonLeftStanding = "this is no longer the import's to take back out"
	reasonBodyGoesWith = "the rewritten body goes out with the row it was written on"
)

func revertDetail(resource entity.ImportResource, outcome entity.ImportOutcome) map[string]string {
	switch outcome {
	case entity.ImportOutcomeSkippedModified:
		return map[string]string{"reason": reasonTouchedSince}
	case entity.ImportOutcomeSkippedInUse:
		return map[string]string{"reason": reasonStillInUse}
	case entity.ImportOutcomeRetained:
		if resource == entity.ImportEmbed {
			return map[string]string{"reason": reasonBodyGoesWith}
		}

		return map[string]string{"reason": reasonLeftStanding}
	default:
		return nil
	}
}

func (s *importsService) Revert(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (entity.ImportRun, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if !run.Status.Revertible() {
		return entity.ImportRun{}, entity.ErrImportNotRevertible
	}

	now := time.Now().UTC()

	if err := s.runs.RecordReverter(ctx, run.ID, decision.Actor, now); err != nil {
		return entity.ImportRun{}, err
	}

	if err := s.jobs.EnqueueImportRevert(ctx, entity.ImportRevertPayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt,
	}, now); err != nil {
		return entity.ImportRun{}, err
	}

	run.RevertedByAccount = decision.Actor.AccountID
	run.RevertedActorKind = decision.Actor.Kind
	run.RevertedAuthMethod = decision.Actor.AuthMethod

	return run, nil
}

func (s *importsService) RunRevert(ctx context.Context, payload entity.ImportRevertPayload) error {
	run, err := s.runs.GetByID(ctx, payload.WorkspaceID, payload.ImportRunID)
	if err != nil {
		return err
	}

	if run.Status == entity.ImportReverted {
		return nil
	}

	from := entity.ImportImported
	if run.Status == entity.ImportFailed {
		from = entity.ImportFailed
	}

	token, _, err := s.lease(ctx, run, from, entity.ImportReverting)
	if err != nil {
		if errors.Is(err, entity.ErrImportRunLeased) {
			return nil
		}

		return err
	}

	ctx = identity.WithActor(ctx, reverterOf(run))

	decision, err := s.decide(ctx, run.WorkspaceID)
	if err != nil {
		return s.fail(ctx, run.ID, err)
	}

	done, err := s.unwindWhileLeased(ctx, run, token, decision)
	if err != nil {
		return s.settleSlice(ctx, run, token, err)
	}

	if done {
		return s.runs.Settle(ctx, run.ID, entity.ImportReverted, "", time.Now().UTC())
	}

	return s.resumeReverting(ctx, run, token)
}

func (s *importsService) unwindWhileLeased(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	decision entity.Decision,
) (bool, error) {
	started := time.Now()
	before := int64(0)
	log := newRecorder(entity.ImportPhaseRevert)

	for {
		if time.Since(started) >= s.cfg.SliceBudget {
			return false, nil
		}

		if err := s.beat(ctx, run.ID, token); err != nil {
			return false, err
		}

		entries, err := s.ledger.Walk(ctx, run.ID, entity.ImportLedgerWalk{
			Before: before,
			Limit:  s.cfg.ChunkSize,
		})
		if err != nil {
			return false, err
		}

		if len(entries) == 0 {
			return true, nil
		}

		if err := s.unwindChunk(ctx, run, decision, entries, log); err != nil {
			return false, err
		}

		before = entries[len(entries)-1].OrderSeq
	}
}

func (s *importsService) unwindChunk(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entries []entity.ImportLedgerEntry,
	log *recorder,
) error {
	log.reset()

	failures := 0

	for _, entry := range entries {
		outcome, err := s.unwind(ctx, run, decision, entry, log)
		if err != nil {
			failures++

			logging.From(ctx).WarnContext(ctx, "undoing an imported row failed",
				"import_run_id", run.ID.String(),
				"resource", string(entry.Resource),
				"created_id", entry.CreatedID.String(),
				"error", err.Error(),
			)

			outcome = entity.ImportOutcomeFailed

			log.failed(entry.Resource, entry.ExternalID, entry.Reference, err.Error())
		} else {
			log.note(entry.Resource, entry.ExternalID, entry.Reference, outcome,
				revertDetail(entry.Resource, outcome))
		}

		entry.RevertOutcome = outcome
		log.created(entry)
	}

	if failures > 0 {
		logging.From(ctx).WarnContext(ctx, "an import revert chunk left rows behind",
			"import_run_id", run.ID.String(),
			"failed", failures,
			"of", len(entries),
		)
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.ledger.Restamp(ctx, run.ID, log.restamped); err != nil {
			return err
		}

		if err := s.ledger.Settle(ctx, run.ID, log.ledger, time.Now().UTC()); err != nil {
			return err
		}

		if err := s.report.Record(ctx, run.ID, log.lines); err != nil {
			return err
		}

		return s.runs.Advance(ctx, run.ID, 0, len(log.ledger))
	})
}

func (s *importsService) unwind(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
	log *recorder,
) (entity.ImportOutcome, error) {
	switch entry.Resource {
	// An embed made nothing of its own: it rewrote a description that the issue entry beside
	// it will delete outright. Saying so in its own case rather than letting it fall through
	// keeps the default arm meaning "a resource nothing here knows how to undo".
	case entity.ImportEmbed:
		return entity.ImportOutcomeRetained, nil
	case entity.ImportAttachment:
		return s.removeAttachment(ctx, run, entry)
	case entity.ImportIssueParent:
		return s.unlinkParent(ctx, run, decision, entry, log)
	case entity.ImportIssueRelation:
		return s.unlinkRelation(ctx, run, entry)
	case entity.ImportComment:
		return s.removeComment(ctx, run, entry)
	case entity.ImportIssue:
		return s.removeIssue(ctx, run, decision, entry, log)
	case entity.ImportCycle:
		return s.removeCycle(ctx, run, decision, entry)
	case entity.ImportLabel:
		return s.removeLabel(ctx, run, decision, entry)
	case entity.ImportLabelGroup:
		return s.removeLabelGroup(ctx, run, entry)
	case entity.ImportProject:
		return s.removeProject(ctx, run, decision, entry)
	case entity.ImportWorkflowState:
		return s.removeState(ctx, run, decision, entry)
	case entity.ImportTeam:
		return s.removeTeam(ctx, run, entry)
	default:
		return entity.ImportOutcomeRetained, nil
	}
}

// unlinkParent is where a linked child is judged, because the unlink is the first thing
// the revert does to it and re-versions it in passing. This entry records the version the
// import left the child at once it had reparented it, so a child above that number has
// been edited by somebody outside this run: the link stands, and removeIssue then keeps
// the child for the same reason. Unlinking first and restamping afterwards would hand
// removeIssue a baseline that agrees with a row a person had edited, and delete their work.
func (s *importsService) unlinkParent(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
	log *recorder,
) (entity.ImportOutcome, error) {
	child, err := s.issues.GetVisible(ctx, run.WorkspaceID, entry.CreatedID, decision.Scope)
	if err != nil {
		if errors.Is(err, entity.ErrIssueNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if child.Version > entry.VersionAtCreate {
		return entity.ImportOutcomeSkippedModified, nil
	}

	if child.ParentIssueID == uuid.Nil {
		return entity.ImportOutcomeRetained, nil
	}

	if err := s.issues.SetParent(
		ctx, child.ID, child.Version, nil, entity.IssueRootDepth, time.Now().UTC(),
	); err != nil {
		return "", err
	}

	log.restamp(entity.ImportLedgerEntry{
		Resource:        entity.ImportIssue,
		CreatedID:       child.ID,
		VersionAtCreate: child.Version + 1,
	})

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) unlinkRelation(
	ctx context.Context,
	run entity.ImportRun,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	if _, err := s.relations.GetByID(ctx, run.WorkspaceID, entry.CreatedID); err != nil {
		if errors.Is(err, entity.ErrIssueRelationNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if err := s.relations.Delete(ctx, entry.CreatedID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeAttachment(
	ctx context.Context,
	run entity.ImportRun,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	file, err := s.files.GetByID(ctx, run.WorkspaceID, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrAttachmentNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if !file.Stored() {
		return entity.ImportOutcomeRetained, nil
	}

	if err := s.fileWriter.Remove(ctx, run.WorkspaceID, file.IssueID, file.ID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeComment(
	ctx context.Context,
	run entity.ImportRun,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	comment, err := s.comments.GetByID(ctx, run.WorkspaceID, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrIssueCommentNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if comment.Edited() || comment.Deleted() {
		return entity.ImportOutcomeSkippedModified, nil
	}

	replied, err := s.replied(ctx, comment)
	if err != nil {
		return "", err
	}

	if replied {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	if err := s.comments.PurgeImported(
		ctx, run.WorkspaceID, []uuid.UUID{comment.ID},
	); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) replied(ctx context.Context, comment entity.IssueComment) (bool, error) {
	if !comment.Root() {
		return false, nil
	}

	page := entity.CommentPage{Limit: s.cfg.PageSize}.Normalized()

	for {
		thread, err := s.comments.ListThread(ctx, comment.IssueID, uuid.Nil, page.Lookahead())
		if err != nil {
			return false, err
		}

		roots := thread
		if len(thread) > page.Limit {
			roots = thread[:page.Limit]
		}

		for _, root := range roots {
			if root.ID == comment.ID {
				return len(root.Replies) > 0, nil
			}
		}

		if len(thread) <= page.Limit {
			return false, nil
		}

		cursor := roots[len(roots)-1].Cursor()
		page.Cursor = &cursor
	}
}

func (s *importsService) removeIssue(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
	log *recorder,
) (entity.ImportOutcome, error) {
	issue, err := s.issues.GetVisible(ctx, run.WorkspaceID, entry.CreatedID, decision.Scope)
	if err != nil {
		if errors.Is(err, entity.ErrIssueNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if issue.Version > log.baseline(entry) {
		return entity.ImportOutcomeSkippedModified, nil
	}

	if issue.Status != entity.IssueStatusActive {
		return entity.ImportOutcomeRetained, nil
	}

	// The delayed purge is no use here: it finishes a deletion somebody asked for and refuses
	// an issue that is not already soft-deleted with its grace period spent, which an imported
	// issue never is. Sending the import back through that lifecycle instead would leave a
	// reverted backlog sitting pending deletion for the whole grace period, which is worse than
	// the import it was meant to undo.
	purged, err := s.issues.PurgeImported(ctx, run.WorkspaceID, []uuid.UUID{issue.ID})
	if err != nil {
		return "", err
	}

	if purged == 0 {
		return entity.ImportOutcomeRetained, nil
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeCycle(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	cycle, err := s.cycles.GetVisible(ctx, run.WorkspaceID, entry.CreatedID, decision.Scope)
	if err != nil {
		if errors.Is(err, entity.ErrCycleNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if cycle.UpdatedAt.After(entry.CreatedAtRecorded) {
		return entity.ImportOutcomeSkippedModified, nil
	}

	// workspace_issues.cycle_id is ON DELETE SET NULL, so nothing in the database refuses
	// this delete: an issue somebody planned into the cycle after the import would simply
	// come out of it, with no record of where it had been put.
	carried, err := s.issues.ListVisible(ctx, decision.Scope, entity.IssuePage{
		Limit:    1,
		Statuses: entity.IssueStatuses(),
		CycleID:  &cycle.ID,
	})
	if err != nil {
		return "", err
	}

	if len(carried) > 0 {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	if err := s.cycles.Delete(ctx, cycle.ID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeLabel(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	label, err := s.labels.GetByID(ctx, run.WorkspaceID, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrLabelNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if label.UpdatedAt.After(entry.CreatedAtRecorded) {
		return entity.ImportOutcomeSkippedModified, nil
	}

	usage, err := s.labels.Usage(ctx, label.ID, decision.Scope)
	if err != nil {
		return "", err
	}

	if usage.Issues > 0 {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	if err := s.labels.Delete(ctx, label.ID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeLabelGroup(
	ctx context.Context,
	run entity.ImportRun,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	group, err := s.groups.GetByID(ctx, run.WorkspaceID, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrLabelGroupNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if group.UpdatedAt.After(entry.CreatedAtRecorded) {
		return entity.ImportOutcomeSkippedModified, nil
	}

	held, err := s.labels.ListByWorkspaceID(ctx, run.WorkspaceID, entity.TeamScope{
		WorkspaceID:    run.WorkspaceID,
		AllTeams:       true,
		IncludePrivate: true,
	})
	if err != nil {
		return "", err
	}

	for _, label := range held {
		if label.GroupID == group.ID {
			return entity.ImportOutcomeSkippedInUse, nil
		}
	}

	if err := s.groups.Delete(ctx, group.ID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeProject(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	project, err := s.projects.GetByID(ctx, run.WorkspaceID, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrProjectNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if project.UpdatedAt.After(entry.CreatedAtRecorded) {
		return entity.ImportOutcomeSkippedModified, nil
	}

	concealed, err := s.projects.HasConcealedWork(ctx, decision.Scope, project.ID)
	if err != nil {
		return "", err
	}

	if concealed {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	carried, err := s.issues.ListVisible(ctx, decision.Scope, entity.IssuePage{
		Limit:     1,
		Statuses:  entity.IssueStatuses(),
		ProjectID: &project.ID,
	})
	if err != nil {
		return "", err
	}

	if len(carried) > 0 {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	if err := s.projects.Delete(ctx, project.ID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) removeState(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	held, err := s.holds(ctx, decision.Scope, entity.IssueFilterFieldState, entry.CreatedID)
	if err != nil {
		return "", err
	}

	if held {
		return entity.ImportOutcomeSkippedInUse, nil
	}

	if err := s.states.Delete(ctx, entry.CreatedID); err != nil {
		return "", err
	}

	return entity.ImportOutcomeDeleted, nil
}

func (s *importsService) holds(
	ctx context.Context,
	scope entity.TeamScope,
	field entity.IssueFilterField,
	id uuid.UUID,
) (bool, error) {
	filter := entity.IssueFilter{
		Field:  field,
		Op:     entity.IssueFilterOpIs,
		Values: []string{id.String()},
	}

	carried, err := s.issues.ListVisible(ctx, scope, entity.IssuePage{
		Limit:    1,
		Statuses: entity.IssueStatuses(),
		Filter:   &filter,
	})
	if err != nil {
		return false, err
	}

	return len(carried) > 0, nil
}

func (s *importsService) removeTeam(
	ctx context.Context,
	run entity.ImportRun,
	entry entity.ImportLedgerEntry,
) (entity.ImportOutcome, error) {
	team, err := s.teams.GetByID(ctx, entry.CreatedID)
	if err != nil {
		if errors.Is(err, entity.ErrTeamNotFound) {
			return entity.ImportOutcomeRetained, nil
		}

		return "", err
	}

	if team.Archived() {
		return entity.ImportOutcomeRetained, nil
	}

	onlyTeam := entity.TeamScope{
		WorkspaceID:    run.WorkspaceID,
		IncludePrivate: true,
		TeamIDs:        []uuid.UUID{team.ID},
	}

	carried, err := s.issues.ListVisible(ctx, onlyTeam, entity.IssuePage{
		Limit:    1,
		Statuses: entity.IssueStatuses(),
	})
	if err != nil {
		return "", err
	}

	if len(carried) > 0 {
		return entity.ImportOutcomeRetained, nil
	}

	joined, err := s.teamMembers.ListByTeamID(ctx, team.ID)
	if err != nil {
		return "", err
	}

	if len(joined) > 0 {
		return entity.ImportOutcomeRetained, nil
	}

	if _, err := s.teams.Archive(ctx, team.ID, time.Now().UTC()); err != nil {
		return "", err
	}

	return entity.ImportOutcomeArchived, nil
}

func (s *importsService) resumeReverting(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
) error {
	if err := s.runs.Release(ctx, run.ID, token, time.Now().UTC()); err != nil {
		return err
	}

	return s.jobs.EnqueueImportRevert(ctx, entity.ImportRevertPayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt + 1,
	}, time.Now().UTC())
}

func reverterOf(run entity.ImportRun) entity.Actor {
	reverter := run.Reverter()
	if reverter.AccountID == uuid.Nil {
		return run.Requester()
	}

	return reverter
}
