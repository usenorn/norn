package triage

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type triagesService struct {
	triage     repository.Triage
	issues     repository.Issue
	states     repository.WorkflowState
	activity   repository.Activity
	teams      repository.Team
	relations  service.IssueRelations
	issueMoves service.Issues
	comments   service.IssueComments
	authorizer service.Authorizer
	emitter    service.WebhookEmitter
	transactor repository.Transactor
}

func New(
	triage repository.Triage,
	issues repository.Issue,
	states repository.WorkflowState,
	activity repository.Activity,
	teams repository.Team,
	relations service.IssueRelations,
	issueMoves service.Issues,
	comments service.IssueComments,
	authorizer service.Authorizer,
	emitter service.WebhookEmitter,
	transactor repository.Transactor,
) service.Triages {
	return &triagesService{
		triage:     triage,
		issues:     issues,
		states:     states,
		activity:   activity,
		teams:      teams,
		relations:  relations,
		issueMoves: issueMoves,
		comments:   comments,
		authorizer: authorizer,
		emitter:    emitter,
		transactor: transactor,
	}
}

func (s *triagesService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	resource entity.Resource,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    resource,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *triagesService) Queue(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.TriageQueueInput,
) (service.TriageQueue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue, entity.ActionRead)
	if err != nil {
		return service.TriageQueue{}, err
	}

	page := entity.IssuePage{
		Limit:    input.Limit,
		Waiting:  true,
		Statuses: []entity.IssueStatus{entity.IssueStatusActive},
	}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeIssueQueryCursor(input.Cursor)
		if err != nil {
			return service.TriageQueue{}, entity.NewValidationError(entity.FieldError{
				Field: "cursor",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		page.QueryCursor = &cursor
	}

	issues, err := s.issues.ListVisible(ctx, decision.Scope, page.Lookahead())
	if err != nil {
		return service.TriageQueue{}, err
	}

	queue := service.TriageQueue{Issues: issues}

	if len(issues) > page.Limit {
		queue.Issues = issues[:page.Limit]

		last := queue.Issues[len(queue.Issues)-1]
		queue.NextCursor = entity.IssueQueryCursor{
			Keys:    entity.IssueSortKeys(last, entity.DefaultIssueSort()),
			IssueID: last.ID,
		}.Encode()
	}

	teams, err := s.issues.TallyByGroup(ctx, decision.Scope, page, entity.IssueGroupByTeam)
	if err != nil {
		return service.TriageQueue{}, err
	}

	queue.Teams = teams

	return queue, nil
}

func (s *triagesService) waiting(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	decision entity.Decision,
) (entity.Issue, error) {
	issue, err := s.issues.LockByID(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Issue{}, err
	}

	if !issue.Waiting() {
		return entity.Issue{}, entity.ErrIssueNotWaiting
	}

	return issue, nil
}

func (s *triagesService) record(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	state entity.TriageState,
	object string,
) error {
	now := time.Now().UTC()

	if err := s.triage.Decide(ctx, issue.ID, state, decision.Actor.AccountID, now); err != nil {
		return err
	}

	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        entity.ActivityKindTriaged,
		Field:       string(state),
		FromValue:   string(entity.TriageStateWaiting),
		ToValue:     object,
	})
}

func (s *triagesService) decided(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	decision entity.Decision,
) (entity.Issue, error) {
	return s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
}

func (s *triagesService) Accept(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue, entity.ActionManage)
	if err != nil {
		return entity.Issue{}, err
	}

	var accepted entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.waiting(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		if err := s.record(ctx, issue, decision, entity.TriageStateAccepted, ""); err != nil {
			return err
		}

		accepted, err = s.decided(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		return s.emit(ctx, entity.WebhookIssueTriaged, accepted, decision)
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return accepted, nil
}

func (s *triagesService) Decline(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.DeclineTriageInput,
) (entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue, entity.ActionManage)
	if err != nil {
		return entity.Issue{}, err
	}

	if err := entity.NewValidationError(
		validateDeclineReason("reason", input.Reason),
		validateDeclineNote("note", input.Note),
	); err != nil {
		return entity.Issue{}, err
	}

	var declined entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.waiting(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		if err := s.close(ctx, issue, decision); err != nil {
			return err
		}

		if err := s.record(ctx, issue, decision, entity.TriageStateDeclined, string(input.Reason)); err != nil {
			return err
		}

		declined, err = s.decided(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		return s.emit(ctx, entity.WebhookIssueTriaged, declined, decision)
	})
	if err != nil {
		return entity.Issue{}, err
	}

	if note := strings.TrimSpace(input.Note); note != "" {
		if _, err := s.comments.Post(ctx, workspaceID, issueID, service.PostCommentInput{
			Body: note,
		}); err != nil {
			return entity.Issue{}, err
		}
	}

	return declined, nil
}

func validateDeclineReason(field string, reason entity.TriageDeclineReason) entity.FieldError {
	if reason == "" {
		return entity.FieldError{Field: field, Code: entity.ValidationCodeRequired}
	}

	if !reason.Valid() {
		return entity.FieldError{Field: field, Code: entity.ValidationCodeUnsupportedValue}
	}

	return entity.FieldError{}
}

func validateDeclineNote(field, note string) entity.FieldError {
	if utf8.RuneCountInString(note) > entity.TriageDeclineNoteMaxLen {
		return entity.FieldError{Field: field, Code: entity.ValidationCodeTooLong}
	}

	return entity.FieldError{}
}

func (s *triagesService) close(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
) error {
	states, err := s.states.ListByTeamID(ctx, issue.TeamID)
	if err != nil {
		return err
	}

	target, found := entity.CounterpartState(states, entity.StateCategoryAbandoned)
	if !found {
		return entity.ErrIssueDestinationIncapable
	}

	if issue.State.ID == target.ID {
		return nil
	}

	now := time.Now().UTC()
	timestamps := entity.ApplyStateTransition(
		issue.State.Category, target.Category, issue.StateEnteredAt, now,
	)

	if err := s.issues.Update(ctx, issue.ID, issue.Version, entity.IssueChange{
		StateID: &target.ID,
	}, &timestamps, now); err != nil {
		return err
	}

	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        entity.ActivityKindStateChanged,
		FromState:   issue.State.Name,
		ToState:     target.Name,
		Version:     issue.Version + 1,
	})
}

func (s *triagesService) Merge(
	ctx context.Context,
	workspaceID, issueID, duplicateOfID uuid.UUID,
) (entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue, entity.ActionManage)
	if err != nil {
		return entity.Issue{}, err
	}

	if duplicateOfID == uuid.Nil {
		return entity.Issue{}, entity.NewValidationError(entity.FieldError{
			Field: "duplicateOfId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	var merged entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.waiting(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		survivor, err := s.issues.GetVisible(ctx, workspaceID, duplicateOfID, decision.Scope)
		if err != nil {
			return err
		}

		if _, err := s.relations.Add(ctx, workspaceID, issueID, service.AddIssueRelationInput{
			Kind:           entity.IssueRelationViewDuplicates,
			CounterpartID:  duplicateOfID,
			CloseDuplicate: true,
		}); err != nil {
			return err
		}

		if err := s.record(
			ctx, issue, decision, entity.TriageStateMerged, survivor.Reference(),
		); err != nil {
			return err
		}

		merged, err = s.decided(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		return s.emit(ctx, entity.WebhookIssueMerged, merged, decision)
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return merged, nil
}

func (s *triagesService) Reassign(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ReassignTriageInput,
) (entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceIssue, entity.ActionManage)
	if err != nil {
		return entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Issue{}, err
	}

	if !issue.Waiting() {
		return entity.Issue{}, entity.ErrIssueNotWaiting
	}

	moved, err := s.issueMoves.MoveToTeam(ctx, workspaceID, issueID, service.MoveIssueInput{
		TeamID:               input.TeamID,
		ExpectedVersion:      input.ExpectedVersion,
		AcknowledgeLabelLoss: input.AcknowledgeLabelLoss,
	})
	if err != nil {
		return entity.Issue{}, err
	}

	team, err := s.teams.GetByID(ctx, input.TeamID)
	if err != nil {
		return entity.Issue{}, err
	}

	var reassigned entity.Issue

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.triage.Settings(ctx, workspaceID, input.TeamID); err != nil {
			if !errors.Is(err, entity.ErrTriageDisabled) {
				return err
			}

			if err := s.record(
				ctx, moved, decision, entity.TriageStateAccepted, team.Name,
			); err != nil {
				return err
			}

			reassigned, err = s.decided(ctx, workspaceID, issueID, decision)
			if err != nil {
				return err
			}

			return s.emit(ctx, entity.WebhookIssueTriaged, reassigned, decision)
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issueID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindTriaged,
			Field:       triageDecisionReassigned,
			FromValue:   string(entity.TriageStateWaiting),
			ToValue:     team.Name,
		}); err != nil {
			return err
		}

		reassigned, err = s.decided(ctx, workspaceID, issueID, decision)
		if err != nil {
			return err
		}

		return s.emit(ctx, entity.WebhookIssueTriaged, reassigned, decision)
	})
	if err != nil {
		return entity.Issue{}, err
	}

	return reassigned, nil
}

const triageDecisionReassigned = "reassigned"

func (s *triagesService) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.TriageSettings, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceTeam, entity.ActionRead)
	if err != nil {
		return entity.TriageSettings{}, err
	}

	if err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return entity.TriageSettings{}, err
	}

	return s.triage.Settings(ctx, workspaceID, teamID)
}

func (s *triagesService) scopedTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) error {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.ErrTeamNotFound
	}

	return nil
}

func (s *triagesService) Configure(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	input service.ConfigureTriageInput,
) (entity.TriageSettings, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceTeam, entity.ActionManage)
	if err != nil {
		return entity.TriageSettings{}, err
	}

	if err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return entity.TriageSettings{}, err
	}

	return s.triage.Upsert(ctx, entity.TriageSettings{
		TeamID:            teamID,
		WorkspaceID:       workspaceID,
		RouteAgents:       input.RouteAgents,
		RouteIntegrations: input.RouteIntegrations,
		RouteNonMembers:   input.RouteNonMembers,
	})
}

func (s *triagesService) Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ResourceTeam, entity.ActionManage)
	if err != nil {
		return err
	}

	if err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return err
	}

	return s.triage.Disable(ctx, workspaceID, teamID)
}
