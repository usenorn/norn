package scm

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *connections) ListRoutes(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
) (entity.SCMRoutes, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	if _, err := s.repositories.GetByID(ctx, workspaceID, repositoryID); err != nil {
		return nil, err
	}

	return s.routes.ListByRepository(ctx, repositoryID)
}

func (s *connections) AddRoute(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.AddRouteInput,
) (entity.SCMRoute, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return entity.SCMRoute{}, err
	}

	if field := entity.ValidateSCMPathPrefix("pathPrefix", input.PathPrefix); field.Field != "" {
		return entity.SCMRoute{}, entity.ValidationError{Fields: []entity.FieldError{field}}
	}

	if !decision.Scope.Covers(input.TeamID) {
		return entity.SCMRoute{}, entity.ErrTeamNotFound
	}

	stored, err := s.repositories.GetByID(ctx, workspaceID, input.RepositoryID)
	if err != nil {
		return entity.SCMRoute{}, err
	}

	return s.routes.Create(ctx, entity.SCMRoute{
		RepositoryID: stored.ID,
		WorkspaceID:  workspaceID,
		TeamID:       input.TeamID,
		PathPrefix:   entity.NormalizeSCMPathPrefix(input.PathPrefix),
	})
}

func (s *connections) RemoveRoute(ctx context.Context, workspaceID, routeID uuid.UUID) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	_, err := s.routes.Delete(ctx, workspaceID, routeID)

	return err
}

func (s *connections) TeamRules(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) ([]service.TeamTransitionRule, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	if !decision.Scope.Covers(teamID) {
		return nil, entity.ErrTeamNotFound
	}

	return s.describeRules(ctx, workspaceID, teamID)
}

func (s *connections) describeRules(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) ([]service.TeamTransitionRule, error) {
	rules, err := s.rules.ListByTeam(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}

	described := make([]service.TeamTransitionRule, 0, len(rules))

	if len(rules) == 0 {
		return described, nil
	}

	states, err := s.states.ListByTeamID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		one := service.TeamTransitionRule{Rule: rule}

		if state, found := rule.TargetState(states); found {
			one.StateName = state.Name
		}

		described = append(described, one)
	}

	return described, nil
}

func (s *connections) SetTeamRule(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	input service.SetTransitionRuleInput,
) ([]service.TeamTransitionRule, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if !decision.Scope.Covers(teamID) {
		return nil, entity.ErrTeamNotFound
	}

	if !input.Trigger.Valid() {
		return nil, entity.ValidationError{
			Fields: []entity.FieldError{{
				Field: "trigger",
				Code:  entity.ValidationCodeUnsupportedValue,
			}},
		}
	}

	states, err := s.states.ListByTeamID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	known := false

	for _, state := range states {
		if state.ID == input.StateID {
			known = true

			break
		}
	}

	if !known {
		return nil, entity.ValidationError{
			Fields: []entity.FieldError{{
				Field: "stateId",
				Code:  entity.ValidationCodeUnsupportedValue,
			}},
		}
	}

	if _, err := s.rules.Upsert(ctx, entity.SCMTransitionRule{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Trigger:     input.Trigger,
		StateID:     input.StateID,
	}); err != nil {
		return nil, err
	}

	return s.describeRules(ctx, workspaceID, teamID)
}

func (s *connections) ClearTeamRule(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	trigger entity.CodeChangeState,
) ([]service.TeamTransitionRule, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if !decision.Scope.Covers(teamID) {
		return nil, entity.ErrTeamNotFound
	}

	if err := s.rules.Delete(ctx, workspaceID, teamID, trigger); err != nil {
		return nil, err
	}

	return s.describeRules(ctx, workspaceID, teamID)
}
