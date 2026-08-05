package issuerelation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type relationsService struct {
	relations  repository.IssueRelation
	issues     repository.Issue
	states     repository.WorkflowState
	activity   repository.Activity
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	relations repository.IssueRelation,
	issues repository.Issue,
	states repository.WorkflowState,
	activity repository.Activity,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.IssueRelations {
	return &relationsService{
		relations:  relations,
		issues:     issues,
		states:     states,
		activity:   activity,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *relationsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *relationsService) Add(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.AddIssueRelationInput,
) (entity.IssueRelation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueRelation{}, err
	}

	if !input.Kind.Valid() {
		return entity.IssueRelation{}, entity.NewValidationError(entity.FieldError{
			Field: "kind",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if input.CounterpartID == uuid.Nil {
		return entity.IssueRelation{}, entity.NewValidationError(entity.FieldError{
			Field: "issueId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if input.CounterpartID == issueID {
		return entity.IssueRelation{}, entity.ErrIssueRelationSelf
	}

	var created entity.IssueRelation

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		subject, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		counterpart, err := s.issues.GetVisible(ctx, workspaceID, input.CounterpartID, decision.Scope)
		if err != nil {
			return err
		}

		if err := s.unheld(ctx, workspaceID, subject.ID, counterpart.ID, decision); err != nil {
			return err
		}

		kind, subjectIsSource := input.Kind.Stored()

		source, target := subject, counterpart
		if !subjectIsSource {
			source, target = counterpart, subject
		}

		if kind.Symmetric() {
			if low, _ := entity.NormalisePair(source.ID, target.ID); low != source.ID {
				source, target = target, source
			}
		}

		stored, err := s.relations.Create(ctx, entity.StoredIssueRelation{
			WorkspaceID:        workspaceID,
			SourceIssueID:      source.ID,
			TargetIssueID:      target.ID,
			Kind:               kind,
			CreatedByAccountID: decision.Actor.AccountID,
		})
		if err != nil {
			return err
		}

		if err := s.record(
			ctx, workspaceID, decision, entity.ActivityKindRelationAdded,
			subject, counterpart, kind.As(subjectIsSource), kind.As(!subjectIsSource),
		); err != nil {
			return err
		}

		if input.CloseDuplicate && kind == entity.IssueRelationDuplicates {
			if err := s.close(ctx, workspaceID, source, decision); err != nil {
				return err
			}
		}

		created = entity.IssueRelation{
			ID:        stored.ID,
			Kind:      kind.As(subjectIsSource),
			Issue:     counterpart,
			CreatedAt: stored.CreatedAt,
		}

		return nil
	})
	if err != nil {
		return entity.IssueRelation{}, err
	}

	return created, nil
}

func (s *relationsService) unheld(
	ctx context.Context,
	workspaceID, issueID, counterpartID uuid.UUID,
	decision entity.Decision,
) error {
	held, err := s.relations.FindPair(ctx, workspaceID, issueID, counterpartID, decision.Scope)
	if err == nil {
		return entity.IssueRelationExistsError{Kind: held.Kind, Reference: held.Issue.Reference()}
	}

	if errors.Is(err, entity.ErrIssueRelationNotFound) {
		return nil
	}

	return err
}

func (s *relationsService) close(
	ctx context.Context,
	workspaceID uuid.UUID,
	duplicate entity.Issue,
	decision entity.Decision,
) error {
	states, err := s.states.ListByTeamID(ctx, duplicate.TeamID)
	if err != nil {
		return err
	}

	target, found := entity.CounterpartState(states, entity.StateCategoryAbandoned)
	if !found {
		return entity.ErrIssueDestinationIncapable
	}

	if duplicate.State.ID == target.ID {
		return nil
	}

	now := time.Now().UTC()
	timestamps := entity.ApplyStateTransition(
		duplicate.State.Category, target.Category, duplicate.StateEnteredAt, now,
	)

	if err := s.issues.Update(
		ctx,
		duplicate.ID,
		duplicate.Version,
		entity.IssueChange{StateID: &target.ID},
		&timestamps,
		now,
	); err != nil {
		return err
	}

	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: workspaceID,
		Subject:     entity.IssueSubject(duplicate.ID),
		Actor:       decision.ActivityActor(),
		Kind:        entity.ActivityKindStateChanged,
		FromState:   duplicate.State.Name,
		ToState:     target.Name,
		Version:     duplicate.Version + 1,
	})
}

func (s *relationsService) record(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
	kind entity.ActivityKind,
	subject, counterpart entity.Issue,
	subjectView, counterpartView entity.IssueRelationView,
) error {
	entries := []entity.Activity{
		{Subject: entity.IssueSubject(subject.ID), Field: string(subjectView), ToValue: counterpart.Reference()},
		{Subject: entity.IssueSubject(counterpart.ID), Field: string(counterpartView), ToValue: subject.Reference()},
	}

	for _, entry := range entries {
		if kind == entity.ActivityKindRelationRemoved {
			entry.FromValue, entry.ToValue = entry.ToValue, ""
		}

		entry.WorkspaceID = workspaceID
		entry.Actor = decision.ActivityActor()
		entry.Kind = kind

		if err := s.activity.Record(ctx, entry); err != nil {
			return err
		}
	}

	return nil
}

func (s *relationsService) Remove(ctx context.Context, workspaceID, issueID, relationID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		stored, err := s.relations.GetByID(ctx, workspaceID, relationID)
		if err != nil {
			return err
		}

		if stored.SourceIssueID != issueID && stored.TargetIssueID != issueID {
			return entity.ErrIssueRelationNotFound
		}

		counterpartID, subjectIsSource := stored.Counterpart(issueID)

		subject, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		counterpart, err := s.issues.GetVisible(ctx, workspaceID, counterpartID, decision.Scope)
		if err != nil {
			return err
		}

		if err := s.relations.Delete(ctx, relationID); err != nil {
			return err
		}

		return s.record(
			ctx, workspaceID, decision, entity.ActivityKindRelationRemoved,
			subject, counterpart,
			stored.Kind.As(subjectIsSource), stored.Kind.As(!subjectIsSource),
		)
	})
}

func (s *relationsService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueRelationGroup, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return nil, err
	}

	relations, err := s.relations.ListForIssue(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return nil, err
	}

	return entity.GroupRelations(relations), nil
}
