package issue

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type issuesService struct {
	issues      repository.Issue
	states      repository.WorkflowState
	activity    repository.IssueActivity
	labels      repository.Label
	memberships repository.Membership
	jobs        repository.JobProducer
	authorizer  service.Authorizer
	transactor  repository.Transactor
}

func New(
	issues repository.Issue,
	states repository.WorkflowState,
	activity repository.IssueActivity,
	labels repository.Label,
	memberships repository.Membership,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Issues {
	return &issuesService{
		issues:      issues,
		states:      states,
		activity:    activity,
		labels:      labels,
		memberships: memberships,
		jobs:        jobs,
		authorizer:  authorizer,
		transactor:  transactor,
	}
}

func (s *issuesService) Create(ctx context.Context, input service.CreateIssueInput) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	if err := entity.NewValidationError(entity.ValidateIssueTitle("title", input.Title)); err != nil {
		return entity.Issue{}, err
	}

	if !decision.Scope.Covers(input.TeamID) {
		return entity.Issue{}, entity.ErrTeamNotFound
	}

	state, err := s.states.DefaultForTeam(ctx, input.TeamID)
	if err != nil {
		return entity.Issue{}, err
	}

	var created entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err = s.issues.Create(ctx, entity.Issue{
			WorkspaceID:        input.WorkspaceID,
			TeamID:             input.TeamID,
			Title:              input.Title,
			State:              entity.IssueState{ID: state.ID},
			CreatedByAccountID: decision.Actor.AccountID,
		})
		if err != nil {
			return err
		}

		return s.activity.Record(ctx, entity.IssueActivity{
			WorkspaceID:    created.WorkspaceID,
			IssueID:        created.ID,
			ActorAccountID: decision.Actor.AccountID,
			Kind:           entity.IssueActivityKindCreated,
			ToStateID:      created.State.ID,
			ToState:        created.State.Name,
		})
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return created, nil
}

func (s *issuesService) Get(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
}

func (s *issuesService) GetByReference(
	ctx context.Context,
	workspaceID uuid.UUID,
	reference string,
) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	parsed, err := entity.ParseIssueReference(reference)
	if err != nil {
		return entity.Issue{}, entity.NewValidationError(entity.FieldError{
			Field: "reference",
			Code:  entity.ValidationCodeMalformed,
		})
	}

	return s.issues.GetVisibleByReference(ctx, workspaceID, parsed, decision.Scope)
}

func (s *issuesService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListIssuesInput,
) (service.IssuePage, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.IssuePage{}, err
	}

	page := entity.IssuePage{Limit: input.Limit, Statuses: entity.RequestedIssueStatuses(input.Statuses)}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeIssueCursor(input.Cursor)
		if err != nil {
			return service.IssuePage{}, entity.NewValidationError(entity.FieldError{
				Field: "cursor",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		page.Cursor = &cursor
	}

	issues, err := s.issues.ListVisible(ctx, narrowed(decision.Scope, input.TeamID), page.Lookahead())
	if err != nil {
		return service.IssuePage{}, err
	}

	if len(issues) <= page.Limit {
		return service.IssuePage{Issues: issues}, nil
	}

	issues = issues[:page.Limit]

	return service.IssuePage{
		Issues:     issues,
		NextCursor: issues[len(issues)-1].Cursor().Encode(),
	}, nil
}

func (s *issuesService) Update(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.UpdateIssueInput,
) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	if err := validateUpdate(input); err != nil {
		return entity.Issue{}, err
	}

	change := entity.IssueChange{
		Title:         input.Title,
		StateID:       input.StateID,
		Description:   input.Description,
		Priority:      input.Priority,
		Assignee:      input.AssigneeID,
		Estimate:      input.Estimate,
		DueOn:         input.DueOn,
		ClearAssignee: slices.Contains(input.Clear, entity.IssueFieldAssignee),
		ClearEstimate: slices.Contains(input.Clear, entity.IssueFieldEstimate),
		ClearDueOn:    slices.Contains(input.Clear, entity.IssueFieldDueOn),
	}

	touched := change.Touched()
	if len(touched) == 0 {
		return s.Get(ctx, workspaceID, issueID)
	}

	var updated entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		conflicts := entity.IssueConflicts(issue.FieldVersions, issue.Version, input.ExpectedVersion, touched)
		if len(conflicts) > 0 {
			return entity.IssueStaleError{Version: issue.Version, Conflicts: conflicts}
		}

		var target entity.WorkflowState

		if change.StateID != nil {
			states, err := s.states.ListByTeamID(ctx, issue.TeamID)
			if err != nil {
				return err
			}

			for _, state := range states {
				if state.ID == *change.StateID {
					target = state

					break
				}
			}

			if target.ID == uuid.Nil {
				return entity.NewValidationError(entity.FieldError{
					Field: "stateId",
					Code:  entity.ValidationCodeUnsupportedValue,
				})
			}
		}

		if change.Assignee != nil {
			if err := s.assignable(ctx, issue, *change.Assignee, decision); err != nil {
				return err
			}
		}

		now := time.Now().UTC()

		var timestamps *entity.StateTimestamps

		if target.ID != uuid.Nil {
			applied := entity.ApplyStateTransition(
				issue.State.Category, target.Category, issue.StateEnteredAt, now,
			)
			timestamps = &applied
		}

		if err := s.issues.Update(ctx, issueID, issue.Version, change, timestamps, now); err != nil {
			return err
		}

		if err := s.recordChanges(ctx, issue, decision, change); err != nil {
			return err
		}

		if target.ID != uuid.Nil && target.ID != issue.State.ID {
			if err := s.activity.Record(ctx, entity.IssueActivity{
				WorkspaceID:    workspaceID,
				IssueID:        issueID,
				ActorAccountID: decision.Actor.AccountID,
				Kind:           entity.IssueActivityKindStateChanged,
				FromStateID:    issue.State.ID,
				ToStateID:      target.ID,
				FromState:      issue.State.Name,
				ToState:        target.Name,
				Version:        issue.Version + 1,
			}); err != nil {
				return err
			}
		}

		refreshed, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		updated = refreshed

		return nil
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return updated, nil
}

func (s *issuesService) recordProperty(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	field, from, to string,
) error {
	return s.activity.Record(ctx, entity.IssueActivity{
		WorkspaceID:    issue.WorkspaceID,
		IssueID:        issue.ID,
		ActorAccountID: decision.Actor.AccountID,
		Kind:           entity.IssueActivityKindPropertyChanged,
		Field:          field,
		FromValue:      from,
		ToValue:        to,
		Version:        issue.Version + 1,
	})
}

func (s *issuesService) SetLabels(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.SetIssueLabelsInput,
) ([]entity.Label, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	var applied []entity.Label

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		locked, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		conflicts := entity.IssueConflicts(
			locked.FieldVersions, locked.Version, input.ExpectedVersion, []string{entity.IssueFieldLabels},
		)
		if len(conflicts) > 0 {
			return entity.IssueStaleError{Version: locked.Version, Conflicts: conflicts}
		}

		issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		labels, err := s.labels.ListByIDs(ctx, workspaceID, input.LabelIDs)
		if err != nil {
			return err
		}

		if len(labels) != len(unique(input.LabelIDs)) {
			return entity.ErrLabelNotFound
		}

		for _, label := range labels {
			if !label.AppliesTo(issue.TeamID) {
				return entity.ErrLabelOutOfScope
			}

			if label.TeamID != uuid.Nil && !decision.Scope.Covers(label.TeamID) {
				return entity.ErrLabelNotFound
			}
		}

		if entity.GroupedLabelConflict(labels) {
			return entity.NewValidationError(entity.FieldError{
				Field: "labelIds",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		before := labelNames(issue.Labels)
		after := labelNames(labels)

		if before == after {
			applied = issue.Labels

			return nil
		}

		if err := s.labels.SetForIssue(ctx, issue, labels); err != nil {
			return err
		}

		now := time.Now().UTC()

		if err := s.issues.StampLabels(ctx, issueID, locked.Version, now); err != nil {
			return err
		}

		if err := s.recordProperty(
			ctx, locked, decision, entity.IssueFieldLabels, before, after,
		); err != nil {
			return err
		}

		applied = labels

		return nil
	})
	if err != nil {
		return nil, err
	}

	return applied, nil
}

func unique(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	distinct := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		distinct = append(distinct, id)
	}

	return distinct
}

func (s *issuesService) Progress(
	ctx context.Context,
	workspaceID uuid.UUID,
	teamID *uuid.UUID,
) (entity.IssueProgress, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.IssueProgress{}, err
	}

	return s.issues.ProgressByCategory(ctx, decision.Scope, teamID)
}

func (s *issuesService) Activity(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ListIssueActivityInput,
) (service.IssueActivityPage, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.IssueActivityPage{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return service.IssueActivityPage{}, err
	}

	page := entity.IssueActivityPage{Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeIssueActivityCursor(input.Cursor)
		if err != nil {
			return service.IssueActivityPage{}, entity.NewValidationError(entity.FieldError{
				Field: "cursor",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		page.Cursor = &cursor
	}

	entries, err := s.activity.ListByIssueID(ctx, issueID, page.Lookahead())
	if err != nil {
		return service.IssueActivityPage{}, err
	}

	if len(entries) <= page.Limit {
		return service.IssueActivityPage{Entries: entries}, nil
	}

	entries = entries[:page.Limit]

	return service.IssueActivityPage{
		Entries:    entries,
		NextCursor: entries[len(entries)-1].Cursor().Encode(),
	}, nil
}

func narrowed(scope entity.TeamScope, teamID *uuid.UUID) entity.TeamScope {
	if teamID == nil {
		return scope
	}

	if !scope.Covers(*teamID) {
		return entity.TeamScope{WorkspaceID: scope.WorkspaceID, TeamIDs: []uuid.UUID{}}
	}

	return entity.TeamScope{WorkspaceID: scope.WorkspaceID, TeamIDs: []uuid.UUID{*teamID}}
}

func validateUpdate(input service.UpdateIssueInput) error {
	fields := make([]entity.FieldError, 0, 5)

	if input.Title != nil {
		fields = append(fields, entity.ValidateIssueTitle("title", *input.Title))
	}

	if input.Description != nil {
		fields = append(fields, entity.ValidateIssueDescription("description", *input.Description))
	}

	if input.Priority != nil {
		fields = append(fields, entity.ValidateIssuePriority("priority", *input.Priority))
	}

	if input.Estimate != nil {
		fields = append(fields, entity.ValidateIssueEstimate("estimate", *input.Estimate))
	}

	if input.DueOn != nil {
		if _, err := time.Parse(time.DateOnly, *input.DueOn); err != nil {
			fields = append(fields, entity.FieldError{
				Field: "dueOn",
				Code:  entity.ValidationCodeMalformed,
			})
		}
	}

	for _, field := range input.Clear {
		if !slices.Contains(clearable, field) {
			fields = append(fields, entity.FieldError{
				Field: "clear",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	return entity.NewValidationError(fields...)
}

var clearable = []string{
	entity.IssueFieldAssignee,
	entity.IssueFieldEstimate,
	entity.IssueFieldDueOn,
}

func (s *issuesService) assignable(
	ctx context.Context,
	issue entity.Issue,
	accountID uuid.UUID,
	decision entity.Decision,
) error {
	if _, err := s.memberships.Get(ctx, issue.WorkspaceID, accountID); err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return entity.NewValidationError(entity.FieldError{
				Field: "assigneeId",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		return err
	}

	if !decision.Scope.Covers(issue.TeamID) {
		return entity.ErrIssueNotFound
	}

	return nil
}

func (s *issuesService) SetStatus(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.SetIssueStatusInput,
) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	if !input.Status.Valid() {
		return entity.Issue{}, entity.NewValidationError(entity.FieldError{
			Field: "status",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	var updated entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		target := input.Status
		if target == entity.IssueStatusActive && issue.Status == entity.IssueStatusPendingDeletion {
			target = entity.RestoredIssueStatus(issue.ArchivedAt)
		}

		if !issue.Status.CanTransitionTo(target) {
			return entity.ErrIssueStatusTransition
		}

		conflicts := entity.IssueConflicts(
			issue.FieldVersions, issue.Version, input.ExpectedVersion, []string{entity.IssueFieldStatus},
		)
		if len(conflicts) > 0 {
			return entity.IssueStaleError{Version: issue.Version, Conflicts: conflicts}
		}

		now := time.Now().UTC()
		lifecycle := entity.ApplyIssueStatus(target, issue.ArchivedAt, now)

		if err := s.issues.SetStatus(ctx, issueID, issue.Version, lifecycle, now); err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.IssueActivity{
			WorkspaceID:    workspaceID,
			IssueID:        issueID,
			ActorAccountID: decision.Actor.AccountID,
			Kind:           statusActivityKind(issue.Status, lifecycle.Status),
			Field:          entity.IssueFieldStatus,
			FromValue:      string(issue.Status),
			ToValue:        string(lifecycle.Status),
			Version:        issue.Version + 1,
		}); err != nil {
			return err
		}

		if lifecycle.Status == entity.IssueStatusPendingDeletion {
			if err := s.jobs.EnqueueIssuePurge(
				ctx, entity.IssuePurgePayload{IssueID: issueID}, *lifecycle.PurgeAfter,
			); err != nil {
				return err
			}
		}

		refreshed, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		updated = refreshed

		return nil
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return updated, nil
}

func (s *issuesService) Purge(ctx context.Context, issueID uuid.UUID) error {
	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.issues.Purge(ctx, issueID, time.Now().UTC())
	})
}

func statusActivityKind(from, to entity.IssueStatus) entity.IssueActivityKind {
	switch {
	case to == entity.IssueStatusArchived && from == entity.IssueStatusActive:
		return entity.IssueActivityKindArchived
	case to == entity.IssueStatusPendingDeletion:
		return entity.IssueActivityKindDeleted
	case from == entity.IssueStatusPendingDeletion:
		return entity.IssueActivityKindRestored
	default:
		return entity.IssueActivityKindUnarchived
	}
}

func (s *issuesService) MoveToTeam(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.MoveIssueInput,
) (entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	if input.TeamID == uuid.Nil {
		return entity.Issue{}, entity.NewValidationError(entity.FieldError{
			Field: "teamId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if !decision.Scope.Covers(input.TeamID) {
		return entity.Issue{}, entity.ErrTeamNotFound
	}

	var moved entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		if issue.TeamID == input.TeamID {
			return entity.ErrIssueAlreadyOnTeam
		}

		conflicts := entity.IssueConflicts(
			issue.FieldVersions, issue.Version, input.ExpectedVersion, []string{entity.IssueFieldTeam},
		)
		if len(conflicts) > 0 {
			return entity.IssueStaleError{Version: issue.Version, Conflicts: conflicts}
		}

		labelled, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		stranded := entity.LabelsOutOfScope(labelled.Labels, input.TeamID)
		if len(stranded) > 0 && !input.AcknowledgeLabelLoss {
			return entity.IssueLabelsOutOfScopeError{Labels: stranded}
		}

		states, err := s.states.ListByTeamID(ctx, input.TeamID)
		if err != nil {
			return err
		}

		target, found := entity.CounterpartState(states, issue.State.Category)
		if !found {
			return entity.ErrIssueDestinationIncapable
		}

		now := time.Now().UTC()
		timestamps := entity.ApplyStateTransition(
			issue.State.Category, target.Category, issue.StateEnteredAt, now,
		)

		if err := s.issues.MoveToTeam(
			ctx, issueID, issue.Version, input.TeamID, target.ID, timestamps, now,
		); err != nil {
			return err
		}

		refreshed, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.IssueActivity{
			WorkspaceID:    workspaceID,
			IssueID:        issueID,
			ActorAccountID: decision.Actor.AccountID,
			Kind:           entity.IssueActivityKindTeamMoved,
			Field:          entity.IssueFieldTeam,
			FromValue:      issue.TeamKey,
			ToValue:        refreshed.TeamKey,
			Version:        issue.Version + 1,
		}); err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.IssueActivity{
			WorkspaceID:    workspaceID,
			IssueID:        issueID,
			ActorAccountID: decision.Actor.AccountID,
			Kind:           entity.IssueActivityKindStateChanged,
			FromStateID:    issue.State.ID,
			ToStateID:      target.ID,
			FromState:      issue.State.Name,
			ToState:        target.Name,
			Version:        issue.Version + 1,
		}); err != nil {
			return err
		}

		moved = refreshed

		return nil
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return moved, nil
}

func (s *issuesService) recordChanges(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	change entity.IssueChange,
) error {
	changes := []struct {
		field string
		from  string
		to    string
		when  bool
	}{
		{entity.IssueFieldTitle, issue.Title, deref(change.Title), change.Title != nil && *change.Title != issue.Title},
		{entity.IssueFieldDescription, "", "", change.Description != nil && *change.Description != issue.Description},
		{entity.IssueFieldPriority, string(issue.Priority), priorityOf(change.Priority), change.Priority != nil && *change.Priority != issue.Priority},
		{entity.IssueFieldEstimate, estimateOf(issue.Estimate), estimateChange(change), change.Estimate != nil || change.ClearEstimate},
		{entity.IssueFieldDueOn, issue.DueOn, dueChange(change), change.DueOn != nil || change.ClearDueOn},
		{entity.IssueFieldAssignee, "", "", change.Assignee != nil || change.ClearAssignee},
	}

	for _, entry := range changes {
		if !entry.when {
			continue
		}

		if err := s.recordProperty(ctx, issue, decision, entry.field, entry.from, entry.to); err != nil {
			return err
		}
	}

	return nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func priorityOf(value *entity.IssuePriority) string {
	if value == nil {
		return ""
	}

	return string(*value)
}

func estimateOf(estimate int) string {
	if estimate == 0 {
		return ""
	}

	return strconv.Itoa(estimate)
}

func estimateChange(change entity.IssueChange) string {
	if change.Estimate == nil {
		return ""
	}

	return strconv.Itoa(*change.Estimate)
}

func dueChange(change entity.IssueChange) string {
	if change.DueOn == nil {
		return ""
	}

	return *change.DueOn
}

func labelNames(labels []entity.Label) string {
	names := make([]string, 0, len(labels))

	for _, label := range labels {
		names = append(names, label.Name)
	}

	slices.Sort(names)

	return strings.Join(names, ", ")
}
