package workflowstate

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type workflowStatesService struct {
	states     repository.WorkflowState
	issues     repository.Issue
	teams      repository.Team
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	states repository.WorkflowState,
	issues repository.Issue,
	teams repository.Team,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.WorkflowStates {
	return &workflowStatesService{
		states:     states,
		issues:     issues,
		teams:      teams,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *workflowStatesService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *workflowStatesService) scopedTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (entity.Team, error) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.Team{}, err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.Team{}, entity.ErrTeamNotFound
	}

	return team, nil
}

func (s *workflowStatesService) liveTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (entity.Team, error) {
	team, err := s.scopedTeam(ctx, workspaceID, teamID, decision)
	if err != nil {
		return entity.Team{}, err
	}

	if team.Archived() {
		return entity.Team{}, entity.ErrTeamArchived
	}

	return team, nil
}

func find(states []entity.WorkflowState, id uuid.UUID) (entity.WorkflowState, bool) {
	for _, state := range states {
		if state.ID == id {
			return state, true
		}
	}

	return entity.WorkflowState{}, false
}

func sharesCategory(states []entity.WorkflowState, target entity.WorkflowState) bool {
	for _, state := range states {
		if state.ID != target.ID && state.Category == target.Category {
			return true
		}
	}

	return false
}

func unsupported(field string) error {
	return entity.NewValidationError(entity.FieldError{
		Field: field,
		Code:  entity.ValidationCodeUnsupportedValue,
	})
}

func (s *workflowStatesService) List(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) ([]entity.WorkflowState, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return nil, err
	}

	return s.states.ListByTeamID(ctx, teamID)
}

func (s *workflowStatesService) Create(
	ctx context.Context,
	input service.CreateWorkflowStateInput,
) (entity.WorkflowState, error) {
	decision, err := s.decide(ctx, input.WorkspaceID, entity.ActionManage)
	if err != nil {
		return entity.WorkflowState{}, err
	}

	if err := entity.NewValidationError(entity.ValidateWorkflowStateName("name", input.Name)); err != nil {
		return entity.WorkflowState{}, err
	}

	if !input.Category.Valid() {
		return entity.WorkflowState{}, unsupported("category")
	}

	if _, err := s.liveTeam(ctx, input.WorkspaceID, input.TeamID, decision); err != nil {
		return entity.WorkflowState{}, err
	}

	var created entity.WorkflowState

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		existing, err := s.states.LockByTeamID(ctx, input.TeamID)
		if err != nil {
			return err
		}

		created, err = s.states.Create(ctx, entity.WorkflowState{
			WorkspaceID: input.WorkspaceID,
			TeamID:      input.TeamID,
			Name:        input.Name,
			Category:    input.Category,
			Position:    len(existing) + 1,
			Origin:      input.Origin,
		})

		return err
	})
	if err != nil {
		return entity.WorkflowState{}, err
	}

	return created, nil
}

func (s *workflowStatesService) Update(
	ctx context.Context,
	workspaceID, teamID, stateID uuid.UUID,
	input service.UpdateWorkflowStateInput,
) (entity.WorkflowState, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.WorkflowState{}, err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return entity.WorkflowState{}, err
	}

	var updated entity.WorkflowState

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		states, err := s.states.LockByTeamID(ctx, teamID)
		if err != nil {
			return err
		}

		current, ok := find(states, stateID)
		if !ok {
			return entity.ErrWorkflowStateNotFound
		}

		name := current.Name
		if input.Name != nil {
			name = *input.Name
		}

		category := current.Category
		if input.Category != nil {
			category = *input.Category
		}

		if err := entity.NewValidationError(entity.ValidateWorkflowStateName("name", name)); err != nil {
			return err
		}

		if !category.Valid() {
			return unsupported("category")
		}

		if category != current.Category {
			if current.IsCompletion {
				return entity.ErrWorkflowStateIsCompletion
			}

			if !sharesCategory(states, current) {
				return entity.ErrWorkflowStateLastInCategory
			}
		}

		updated, err = s.states.UpdateSettings(ctx, stateID, name, category)

		return err
	})
	if err != nil {
		return entity.WorkflowState{}, err
	}

	return updated, nil
}

func (s *workflowStatesService) Reorder(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	orderedStateIDs []uuid.UUID,
) ([]entity.WorkflowState, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return nil, err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return nil, err
	}

	var reordered []entity.WorkflowState

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		states, err := s.states.LockByTeamID(ctx, teamID)
		if err != nil {
			return err
		}

		if !isPermutation(states, orderedStateIDs) {
			return unsupported("stateIds")
		}

		if err := s.states.Reposition(ctx, teamID, orderedStateIDs); err != nil {
			return err
		}

		reordered = make([]entity.WorkflowState, 0, len(orderedStateIDs))

		for i, id := range orderedStateIDs {
			state, _ := find(states, id)
			state.Position = i + 1
			reordered = append(reordered, state)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reordered, nil
}

func isPermutation(states []entity.WorkflowState, ids []uuid.UUID) bool {
	if len(states) != len(ids) {
		return false
	}

	seen := make(map[uuid.UUID]bool, len(ids))

	for _, id := range ids {
		if seen[id] {
			return false
		}

		if _, ok := find(states, id); !ok {
			return false
		}

		seen[id] = true
	}

	return true
}

func (s *workflowStatesService) SetDefault(
	ctx context.Context,
	workspaceID, teamID, stateID uuid.UUID,
) ([]entity.WorkflowState, error) {
	return s.setFlag(ctx, workspaceID, teamID, stateID, false)
}

func (s *workflowStatesService) SetCompletion(
	ctx context.Context,
	workspaceID, teamID, stateID uuid.UUID,
) ([]entity.WorkflowState, error) {
	return s.setFlag(ctx, workspaceID, teamID, stateID, true)
}

func (s *workflowStatesService) setFlag(
	ctx context.Context,
	workspaceID, teamID, stateID uuid.UUID,
	completion bool,
) ([]entity.WorkflowState, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return nil, err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return nil, err
	}

	var flagged []entity.WorkflowState

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		states, err := s.states.LockByTeamID(ctx, teamID)
		if err != nil {
			return err
		}

		target, ok := find(states, stateID)
		if !ok {
			return entity.ErrWorkflowStateNotFound
		}

		if completion {
			if target.Category != entity.StateCategoryComplete {
				return unsupported("stateId")
			}

			if err := s.states.SetCompletion(ctx, teamID, stateID); err != nil {
				return err
			}
		} else if err := s.states.SetDefault(ctx, teamID, stateID); err != nil {
			return err
		}

		for i := range states {
			if completion {
				states[i].IsCompletion = states[i].ID == stateID
			} else {
				states[i].IsDefault = states[i].ID == stateID
			}
		}

		flagged = states

		return nil
	})
	if err != nil {
		return nil, err
	}

	return flagged, nil
}

func (s *workflowStatesService) Remove(
	ctx context.Context,
	workspaceID, teamID, stateID, replacementStateID uuid.UUID,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		states, err := s.states.LockByTeamID(ctx, teamID)
		if err != nil {
			return err
		}

		target, ok := find(states, stateID)
		if !ok {
			return entity.ErrWorkflowStateNotFound
		}

		if _, ok := find(states, replacementStateID); !ok || replacementStateID == stateID {
			return unsupported("replacementStateId")
		}

		if target.IsDefault {
			return entity.ErrWorkflowStateIsDefault
		}

		if target.IsCompletion {
			return entity.ErrWorkflowStateIsCompletion
		}

		if !sharesCategory(states, target) {
			return entity.ErrWorkflowStateLastInCategory
		}

		if err := s.issues.ReassignState(ctx, stateID, replacementStateID); err != nil {
			return err
		}

		if err := s.states.Delete(ctx, stateID); err != nil {
			return err
		}

		survivors := make([]uuid.UUID, 0, len(states)-1)

		for _, state := range states {
			if state.ID != stateID {
				survivors = append(survivors, state.ID)
			}
		}

		return s.states.Reposition(ctx, teamID, survivors)
	})
}
