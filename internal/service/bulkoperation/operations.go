package bulkoperation

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type operationsService struct {
	actions    repository.BulkAction
	issues     repository.Issue
	states     repository.WorkflowState
	labels     repository.Label
	activity   repository.Activity
	members    repository.Membership
	jobs       repository.JobProducer
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	actions repository.BulkAction,
	issues repository.Issue,
	states repository.WorkflowState,
	labels repository.Label,
	activity repository.Activity,
	members repository.Membership,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.BulkOperations {
	return &operationsService{
		actions:    actions,
		issues:     issues,
		states:     states,
		labels:     labels,
		activity:   activity,
		members:    members,
		jobs:       jobs,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *operationsService) Apply(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ApplyBulkInput,
) (service.BulkResult, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.BulkResult{}, err
	}

	if err := input.Change.Validate(); err != nil {
		return service.BulkResult{}, err
	}

	if err := input.Set.Validate(); err != nil {
		return service.BulkResult{}, err
	}

	action, err := s.actions.Create(ctx, entity.BulkAction{
		WorkspaceID:         workspaceID,
		RequestedByAccount:  decision.Actor.AccountID,
		RequestedActorKind:  decision.Actor.Kind,
		RequestedAuthMethod: decision.Actor.AuthMethod,
		Change:              input.Change,
		Expected:            input.Set.Expected(),
	})
	if err != nil {
		return service.BulkResult{}, err
	}

	if !input.Set.RunsInline() {
		if err := s.jobs.EnqueueBulkApply(ctx, entity.BulkApplyPayload{
			BulkActionID: action.ID,
			WorkspaceID:  workspaceID,
			IssueIDs:     input.Set.IssueIDs,
			Filter:       input.Set.Filter,
		}); err != nil {
			return service.BulkResult{}, err
		}

		return service.BulkResult{Action: &action}, nil
	}

	outcomes, err := s.process(ctx, action, decision, input.Set.IssueIDs)
	if err != nil {
		return service.BulkResult{}, err
	}

	now := time.Now().UTC()

	if err := s.actions.Claim(ctx, action.ID, now); err != nil {
		return service.BulkResult{}, err
	}

	if err := s.actions.Settle(ctx, action.ID, entity.BulkActionComplete, now); err != nil {
		return service.BulkResult{}, err
	}

	action.Status = entity.BulkActionComplete
	action.Processed = len(outcomes)

	return service.BulkResult{Action: &action, Outcomes: outcomes}, nil
}

func (s *operationsService) Run(ctx context.Context, payload entity.BulkApplyPayload) error {
	action, err := s.actions.GetByID(ctx, payload.WorkspaceID, payload.BulkActionID)
	if err != nil {
		return err
	}

	if action.Status.Settled() {
		return nil
	}

	now := time.Now().UTC()

	if err := s.actions.Claim(ctx, action.ID, now); err != nil {
		return err
	}

	ctx = identity.WithActor(ctx, action.Requester())

	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: payload.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		_ = s.actions.Settle(ctx, action.ID, entity.BulkActionFailed, time.Now().UTC())

		return err
	}

	run := func() error {
		if payload.Filter == nil {
			_, err := s.process(ctx, action, decision, payload.IssueIDs)

			return err
		}

		return s.walk(ctx, action, decision, *payload.Filter)
	}

	if err := run(); err != nil {
		_ = s.actions.Settle(ctx, action.ID, entity.BulkActionFailed, time.Now().UTC())

		return err
	}

	return s.actions.Settle(ctx, action.ID, entity.BulkActionComplete, time.Now().UTC())
}

func (s *operationsService) walk(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	filter entity.BulkFilter,
) error {
	scope := decision.Scope

	if filter.TeamID != nil {
		if !scope.Covers(*filter.TeamID) {
			return entity.ErrTeamNotFound
		}

		scope = entity.TeamScope{
			WorkspaceID: scope.WorkspaceID,
			TeamIDs:     []uuid.UUID{*filter.TeamID},
		}
	}

	page := entity.IssuePage{
		Limit:    entity.BulkChunkSize,
		Statuses: filter.Statuses,
		Filter:   filter.Expression,
	}.Normalized()
	page.Limit = entity.BulkChunkSize

	for {
		issues, err := s.issues.ListVisible(ctx, scope, page)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			return nil
		}

		ids := make([]uuid.UUID, 0, len(issues))
		for _, issue := range issues {
			ids = append(ids, issue.ID)
		}

		if _, err := s.process(ctx, action, decision, ids); err != nil {
			return err
		}

		last := issues[len(issues)-1]
		page.QueryCursor = &entity.IssueQueryCursor{
			Keys:    entity.IssueSortKeys(last, entity.DefaultIssueSort()),
			IssueID: last.ID,
		}
	}
}

func (s *operationsService) Get(
	ctx context.Context,
	workspaceID, actionID uuid.UUID,
) (entity.BulkAction, []entity.BulkActionOutcome, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	}); err != nil {
		return entity.BulkAction{}, nil, err
	}

	action, err := s.actions.GetByID(ctx, workspaceID, actionID)
	if err != nil {
		return entity.BulkAction{}, nil, err
	}

	outcomes, err := s.actions.ListOutcomes(ctx, actionID)
	if err != nil {
		return entity.BulkAction{}, nil, err
	}

	return action, outcomes, nil
}

func (s *operationsService) process(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issueIDs []uuid.UUID,
) ([]entity.BulkActionOutcome, error) {
	recorded := make([]entity.BulkActionOutcome, 0, len(issueIDs))

	for _, chunk := range entity.Chunks(issueIDs, entity.BulkChunkSize) {
		outcomes := make([]entity.BulkActionOutcome, 0, len(chunk))

		err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
			for _, issueID := range chunk {
				outcomes = append(outcomes, s.applyOne(ctx, action, decision, issueID))
			}

			return s.actions.RecordOutcomes(ctx, action.ID, outcomes)
		})
		if err != nil {
			return nil, err
		}

		if err := s.actions.Advance(ctx, action.ID, len(chunk)); err != nil {
			return nil, err
		}

		recorded = append(recorded, outcomes...)
	}

	return recorded, nil
}

func (s *operationsService) applyOne(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issueID uuid.UUID,
) entity.BulkActionOutcome {
	outcome := entity.BulkActionOutcome{IssueID: issueID}

	issue, err := s.issues.LockByID(ctx, action.WorkspaceID, issueID, decision.Scope)
	if err != nil {
		outcome.Outcome = entity.OutcomeFor(err)

		return outcome
	}

	outcome.Reference = issue.Reference()

	if !decision.Scope.Covers(issue.TeamID) {
		outcome.Outcome = entity.BulkOutcomeForbidden

		return outcome
	}

	if err := s.change(ctx, action, decision, issue); err != nil {
		outcome.Outcome = entity.OutcomeFor(err)

		return outcome
	}

	outcome.Outcome = entity.BulkOutcomeApplied

	return outcome
}

func (s *operationsService) change(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
) error {
	now := time.Now().UTC()

	if action.Change.Lifecycle() {
		return s.restatus(ctx, action, decision, issue, now)
	}

	if action.Change.AddLabelID != nil {
		if err := s.label(ctx, action, decision, issue, now); err != nil {
			return err
		}
	}

	return s.edit(ctx, action, decision, issue, now)
}

func (s *operationsService) restatus(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
	now time.Time,
) error {
	target := *action.Change.Status

	if target == entity.IssueStatusActive && issue.Status == entity.IssueStatusPendingDeletion {
		target = entity.RestoredIssueStatus(issue.ArchivedAt)
	}

	if issue.Status == target {
		return nil
	}

	if !issue.Status.CanTransitionTo(target) {
		return entity.ErrIssueStatusTransition
	}

	lifecycle := entity.ApplyIssueStatus(target, issue.ArchivedAt, now)

	if err := s.issues.SetStatus(ctx, issue.ID, issue.Version, lifecycle, now); err != nil {
		return err
	}

	return s.record(ctx, action, decision, issue, entity.Activity{
		Kind:      statusKind(issue.Status, lifecycle.Status),
		Field:     entity.IssueFieldStatus,
		FromValue: string(issue.Status),
		ToValue:   string(lifecycle.Status),
	})
}

func statusKind(from, to entity.IssueStatus) entity.ActivityKind {
	switch {
	case to == entity.IssueStatusArchived && from == entity.IssueStatusActive:
		return entity.ActivityKindArchived
	case to == entity.IssueStatusPendingDeletion:
		return entity.ActivityKindDeleted
	case from == entity.IssueStatusPendingDeletion:
		return entity.ActivityKindRestored
	default:
		return entity.ActivityKindUnarchived
	}
}

func (s *operationsService) label(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
	now time.Time,
) error {
	found, err := s.labels.ListByIDs(ctx, action.WorkspaceID, []uuid.UUID{*action.Change.AddLabelID})
	if err != nil {
		return err
	}

	if len(found) == 0 {
		return entity.ErrLabelNotFound
	}

	label := found[0]

	if !label.AppliesTo(issue.TeamID) {
		return entity.ErrLabelOutOfScope
	}

	if label.TeamID != uuid.Nil && !decision.Scope.Covers(label.TeamID) {
		return entity.ErrLabelNotFound
	}

	current, err := s.issues.GetVisible(ctx, action.WorkspaceID, issue.ID, decision.Scope)
	if err != nil {
		return err
	}

	for _, held := range current.Labels {
		if held.ID == label.ID {
			return nil
		}

		if held.GroupID != uuid.Nil && held.GroupID == label.GroupID {
			return entity.NewValidationError(entity.FieldError{
				Field: "addLabelId",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	if err := s.labels.SetForIssue(ctx, current, append(current.Labels, label)); err != nil {
		return err
	}

	if err := s.issues.StampLabels(ctx, issue.ID, issue.Version, now); err != nil {
		return err
	}

	return s.record(ctx, action, decision, issue, entity.Activity{
		Kind:      entity.ActivityKindPropertyChanged,
		Field:     entity.IssueFieldLabels,
		FromValue: labelNames(current.Labels),
		ToValue:   labelNames(append(current.Labels, label)),
	})
}

func labelNames(labels []entity.Label) string {
	names := make([]string, 0, len(labels))

	for _, label := range labels {
		names = append(names, label.Name)
	}

	slices.Sort(names)

	return strings.Join(names, ", ")
}

func (s *operationsService) edit(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
	now time.Time,
) error {
	change := entity.IssueChange{
		Priority:      action.Change.Priority,
		Assignee:      action.Change.AssigneeID,
		ClearAssignee: action.Change.ClearAssignee,
	}

	var (
		timestamps *entity.StateTimestamps
		target     entity.WorkflowState
	)

	if action.Change.StateID != nil {
		states, err := s.states.ListByTeamID(ctx, issue.TeamID)
		if err != nil {
			return err
		}

		for _, state := range states {
			if state.ID == *action.Change.StateID {
				target = state

				break
			}
		}

		if target.ID == uuid.Nil {
			return entity.ErrWorkflowStateNotFound
		}

		change.StateID = &target.ID
		applied := entity.ApplyStateTransition(issue.State.Category, target.Category, issue.StateEnteredAt, now)
		timestamps = &applied
	}

	if action.Change.AssigneeID != nil {
		if _, err := s.members.Get(ctx, action.WorkspaceID, *action.Change.AssigneeID); err != nil {
			return entity.NewValidationError(entity.FieldError{
				Field: "assigneeId",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	if len(change.Touched()) == 0 {
		return nil
	}

	if err := s.issues.Update(ctx, issue.ID, issue.Version, change, timestamps, now); err != nil {
		return err
	}

	if target.ID != uuid.Nil && target.ID != issue.State.ID {
		if err := s.record(ctx, action, decision, issue, entity.Activity{
			Kind:      entity.ActivityKindStateChanged,
			FromState: issue.State.Name,
			ToState:   target.Name,
		}); err != nil {
			return err
		}
	}

	if action.Change.Priority != nil {
		if err := s.record(ctx, action, decision, issue, entity.Activity{
			Kind:      entity.ActivityKindPropertyChanged,
			Field:     entity.IssueFieldPriority,
			FromValue: string(issue.Priority),
			ToValue:   string(*action.Change.Priority),
		}); err != nil {
			return err
		}
	}

	if action.Change.AssigneeID != nil || action.Change.ClearAssignee {
		if err := s.record(ctx, action, decision, issue, entity.Activity{
			Kind:  entity.ActivityKindPropertyChanged,
			Field: entity.IssueFieldAssignee,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *operationsService) record(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
	entry entity.Activity,
) error {
	entry.WorkspaceID = action.WorkspaceID
	entry.Subject = entity.IssueSubject(issue.ID)
	entry.Actor = decision.ActivityActor()
	entry.Version = issue.Version + 1
	entry.BulkActionID = action.ID

	return s.activity.Record(ctx, entry)
}
