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
	issues       repository.Issue
	states       repository.WorkflowState
	activity     repository.IssueActivity
	labels       repository.Label
	memberships  repository.Membership
	cycles       repository.Cycle
	scopeChanges repository.CycleScopeChange
	projects     repository.Project
	teams        repository.Team
	triage       repository.Triage
	jobs         repository.JobProducer
	authorizer   service.Authorizer
	transactor   repository.Transactor
}

func New(
	issues repository.Issue,
	states repository.WorkflowState,
	activity repository.IssueActivity,
	labels repository.Label,
	memberships repository.Membership,
	cycles repository.Cycle,
	scopeChanges repository.CycleScopeChange,
	projects repository.Project,
	teams repository.Team,
	triage repository.Triage,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Issues {
	return &issuesService{
		issues:       issues,
		states:       states,
		activity:     activity,
		labels:       labels,
		memberships:  memberships,
		cycles:       cycles,
		scopeChanges: scopeChanges,
		projects:     projects,
		teams:        teams,
		triage:       triage,
		jobs:         jobs,
		authorizer:   authorizer,
		transactor:   transactor,
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

	arriving := entity.Issue{
		WorkspaceID:        input.WorkspaceID,
		TeamID:             input.TeamID,
		Title:              input.Title,
		Description:        input.Description,
		Priority:           input.Priority,
		AssigneeAccountID:  input.AssigneeAccountID,
		Estimate:           input.Estimate,
		DueOn:              input.DueOn,
		State:              entity.IssueState{ID: state.ID},
		CreatedByAccountID: decision.Actor.AccountID,
	}

	if arriving.Priority == "" {
		arriving.Priority = entity.IssuePriorityNone
	}

	if err := s.route(ctx, &arriving, decision); err != nil {
		return entity.Issue{}, err
	}

	var created entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err = s.issues.Create(ctx, arriving)
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

	page := entity.IssuePage{
		Limit:     input.Limit,
		Statuses:  entity.RequestedIssueStatuses(input.Statuses),
		CycleID:   input.CycleID,
		ProjectID: input.ProjectID,
	}.Normalized()

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
		CycleID:       input.CycleID,
		ProjectID:     input.ProjectID,
		ClearAssignee: slices.Contains(input.Clear, entity.IssueFieldAssignee),
		ClearEstimate: slices.Contains(input.Clear, entity.IssueFieldEstimate),
		ClearDueOn:    slices.Contains(input.Clear, entity.IssueFieldDueOn),
		ClearCycle:    slices.Contains(input.Clear, entity.IssueFieldCycle),
		ClearProject:  slices.Contains(input.Clear, entity.IssueFieldProject),
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

		var joining entity.Cycle

		if change.CycleID != nil {
			joining, err = s.joinable(ctx, issue, *change.CycleID, decision)
			if err != nil {
				return err
			}
		}

		if change.ProjectID != nil {
			if err := s.projectOpen(ctx, issue, *change.ProjectID); err != nil {
				return err
			}
		}

		if target.Category == entity.StateCategoryComplete &&
			issue.State.Category != entity.StateCategoryComplete &&
			!input.AcknowledgeOpenChildren {
			children, err := s.issues.ListChildren(ctx, workspaceID, issueID, decision.Scope)
			if err != nil {
				return err
			}

			if open := entity.OpenIssues(children); len(open) > 0 {
				return entity.IssueChildrenOpenError{Children: open}
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

		if err := s.recordChanges(ctx, issue, decision, change, joining); err != nil {
			return err
		}

		if err := s.recordScope(ctx, issue, decision, joining, change, now); err != nil {
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
	input service.ProgressInput,
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

	if input.CycleID != nil {
		return s.issues.ProgressByCycle(ctx, decision.Scope, *input.CycleID)
	}

	if input.ProjectID != nil {
		return s.issues.ProgressByProject(ctx, decision.Scope, *input.ProjectID)
	}

	return s.issues.ProgressByCategory(ctx, decision.Scope, input.TeamID)
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
	entity.IssueFieldCycle,
	entity.IssueFieldProject,
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

func (s *issuesService) joinable(
	ctx context.Context,
	issue entity.Issue,
	cycleID uuid.UUID,
	decision entity.Decision,
) (entity.Cycle, error) {
	cycle, err := s.cycles.GetVisible(ctx, issue.WorkspaceID, cycleID, decision.Scope)
	if err != nil {
		return entity.Cycle{}, err
	}

	if cycle.TeamID != issue.TeamID {
		return entity.Cycle{}, entity.ErrCycleTeamMismatch
	}

	if cycle.Closed() {
		return entity.Cycle{}, entity.ErrCycleClosed
	}

	return cycle, nil
}

func (s *issuesService) projectOpen(
	ctx context.Context,
	issue entity.Issue,
	projectID uuid.UUID,
) error {
	project, err := s.projects.GetByID(ctx, issue.WorkspaceID, projectID)
	if err != nil {
		return err
	}

	if project.Archived() {
		return entity.ErrProjectArchived
	}

	return nil
}

func (s *issuesService) recordScope(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	joining entity.Cycle,
	change entity.IssueChange,
	now time.Time,
) error {
	if change.CycleID == nil && !change.ClearCycle {
		return nil
	}

	today := entity.Today(now, decision.Workspace.Timezone)

	if issue.CycleID != uuid.Nil && issue.CycleID != joining.ID {
		left, err := s.cycles.GetVisible(ctx, issue.WorkspaceID, issue.CycleID, decision.Scope)
		if err != nil {
			return err
		}

		if left.Closed() {
			return entity.ErrCycleClosed
		}

		if err := s.markScope(ctx, left, issue, decision, entity.CycleScopeChangeRemoved, today, now); err != nil {
			return err
		}
	}

	if joining.ID == uuid.Nil || joining.ID == issue.CycleID {
		return nil
	}

	return s.markScope(ctx, joining, issue, decision, entity.CycleScopeChangeAdded, today, now)
}

func (s *issuesService) markScope(
	ctx context.Context,
	cycle entity.Cycle,
	issue entity.Issue,
	decision entity.Decision,
	kind entity.CycleScopeChangeKind,
	today string,
	now time.Time,
) error {
	if !cycle.Started(today) {
		return nil
	}

	return s.scopeChanges.Record(ctx, entity.CycleScopeChange{
		CycleID:        cycle.ID,
		IssueID:        issue.ID,
		Change:         kind,
		ActorAccountID: decision.Actor.AccountID,
		ChangedAt:      now,
	})
}

func (s *issuesService) SetParent(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.SetIssueParentInput,
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

	if input.ParentID != nil && *input.ParentID == issueID {
		return entity.Issue{}, entity.ErrIssueParentCycle
	}

	var reparented entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		conflicts := entity.IssueConflicts(
			issue.FieldVersions, issue.Version, input.ExpectedVersion, []string{entity.IssueFieldParent},
		)
		if len(conflicts) > 0 {
			return entity.IssueStaleError{Version: issue.Version, Conflicts: conflicts}
		}

		depth := entity.IssueRootDepth

		var parent entity.Issue

		if input.ParentID != nil {
			parent, err = s.issues.GetVisible(ctx, workspaceID, *input.ParentID, decision.Scope)
			if err != nil {
				return err
			}

			if parent.Status != entity.IssueStatusActive {
				return entity.ErrIssueParentNotActive
			}

			ancestors, err := s.issues.Ancestors(ctx, parent.ID)
			if err != nil {
				return err
			}

			if slices.Contains(ancestors, issueID) {
				return entity.ErrIssueParentCycle
			}

			height, err := s.issues.SubtreeHeight(ctx, issueID)
			if err != nil {
				return err
			}

			if !entity.FitsWithinDepth(parent.Depth, height) {
				return entity.IssueTooDeepError{
					Depth: parent.Depth + 1 + height,
					Max:   entity.IssueMaxDepth,
				}
			}

			depth = parent.Depth + 1
		}

		if issue.ParentIssueID == uuid.Nil && input.ParentID == nil {
			reparented = issue

			return nil
		}

		if input.ParentID != nil && issue.ParentIssueID == *input.ParentID {
			reparented = issue

			return nil
		}

		now := time.Now().UTC()

		if err := s.issues.SetParent(
			ctx, issueID, issue.Version, input.ParentID, depth, now,
		); err != nil {
			return err
		}

		if err := s.recordProperty(
			ctx, issue, decision, entity.IssueFieldParent, issue.ParentReference, parent.Reference(),
		); err != nil {
			return err
		}

		if err := s.recordHierarchy(
			ctx, workspaceID, issue, decision, entity.IssueActivityKindChildRemoved, issue.ParentIssueID,
		); err != nil {
			return err
		}

		if err := s.recordHierarchy(
			ctx, workspaceID, issue, decision, entity.IssueActivityKindChildAdded, parent.ID,
		); err != nil {
			return err
		}

		refreshed, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		reparented = refreshed

		return nil
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return reparented, nil
}

func (s *issuesService) recordHierarchy(
	ctx context.Context,
	workspaceID uuid.UUID,
	child entity.Issue,
	decision entity.Decision,
	kind entity.IssueActivityKind,
	onIssueID uuid.UUID,
) error {
	if onIssueID == uuid.Nil {
		return nil
	}

	entry := entity.IssueActivity{
		WorkspaceID:    workspaceID,
		IssueID:        onIssueID,
		ActorAccountID: decision.Actor.AccountID,
		Kind:           kind,
		Field:          entity.IssueFieldChildren,
	}

	if kind == entity.IssueActivityKindChildAdded {
		entry.ToValue = child.Reference()
	} else {
		entry.FromValue = child.Reference()
	}

	return s.activity.Record(ctx, entry)
}

func (s *issuesService) Children(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Issue, entity.IssueProgress, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, entity.IssueProgress{}, err
	}

	parent, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return nil, entity.IssueProgress{}, err
	}

	children, err := s.issues.ListChildren(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return nil, entity.IssueProgress{}, err
	}

	return children, parent.Children, nil
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

		if err := s.recordScope(
			ctx, issue, decision, entity.Cycle{}, entity.IssueChange{ClearCycle: true}, now,
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
	joining entity.Cycle,
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
		{entity.IssueFieldCycle, cycleName(issue.CycleNumber), cycleChange(change, joining), change.CycleID != nil || change.ClearCycle},
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

func cycleName(number int) string {
	if number == 0 {
		return ""
	}

	return "Cycle " + strconv.Itoa(number)
}

func cycleChange(change entity.IssueChange, joining entity.Cycle) string {
	if change.CycleID == nil {
		return ""
	}

	return cycleName(joining.Number)
}

func labelNames(labels []entity.Label) string {
	names := make([]string, 0, len(labels))

	for _, label := range labels {
		names = append(names, label.Name)
	}

	slices.Sort(names)

	return strings.Join(names, ", ")
}
