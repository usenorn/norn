package label

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type labelsService struct {
	labels     repository.Label
	groups     repository.LabelGroup
	teams      repository.Team
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	labels repository.Label,
	groups repository.LabelGroup,
	teams repository.Team,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Labels {
	return &labelsService{
		labels:     labels,
		groups:     groups,
		teams:      teams,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func unsupported(field string) error {
	return entity.NewValidationError(entity.FieldError{
		Field: field,
		Code:  entity.ValidationCodeUnsupportedValue,
	})
}

func (s *labelsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceLabel,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *labelsService) scopedTeam(
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

func (s *labelsService) liveTeam(
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

func (s *labelsService) visible(
	ctx context.Context,
	workspaceID, labelID uuid.UUID,
	decision entity.Decision,
) (entity.Label, error) {
	label, err := s.labels.GetByID(ctx, workspaceID, labelID)
	if err != nil {
		return entity.Label{}, err
	}

	if label.TeamID != uuid.Nil && !decision.Scope.Covers(label.TeamID) {
		return entity.Label{}, entity.ErrLabelNotFound
	}

	return label, nil
}

func (s *labelsService) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.Label, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	return s.labels.ListByWorkspaceID(ctx, workspaceID, decision.Scope)
}

func (s *labelsService) Create(ctx context.Context, input service.CreateLabelInput) (entity.Label, error) {
	decision, err := s.decide(ctx, input.WorkspaceID, entity.ActionManage)
	if err != nil {
		return entity.Label{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateLabelName("name", input.Name),
		entity.ValidateLabelColor("color", input.Color),
	); err != nil {
		return entity.Label{}, err
	}

	if input.TeamID != uuid.Nil {
		if _, err := s.liveTeam(ctx, input.WorkspaceID, input.TeamID, decision); err != nil {
			return entity.Label{}, err
		}
	}

	if input.GroupID != uuid.Nil {
		if _, err := s.groups.GetByID(ctx, input.WorkspaceID, input.GroupID); err != nil {
			return entity.Label{}, err
		}
	}

	return s.labels.Create(ctx, entity.Label{
		WorkspaceID: input.WorkspaceID,
		TeamID:      input.TeamID,
		GroupID:     input.GroupID,
		Name:        strings.TrimSpace(input.Name),
		Color:       input.Color,
	})
}

func (s *labelsService) Update(
	ctx context.Context,
	workspaceID, labelID uuid.UUID,
	input service.UpdateLabelInput,
) (entity.Label, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Label{}, err
	}

	var updated entity.Label

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		current, err := s.labels.LockByID(ctx, workspaceID, labelID)
		if err != nil {
			return err
		}

		if current.TeamID != uuid.Nil {
			if _, err := s.liveTeam(ctx, workspaceID, current.TeamID, decision); err != nil {
				return err
			}
		}

		name := current.Name
		if input.Name != nil {
			name = strings.TrimSpace(*input.Name)
		}

		color := current.Color
		if input.Color != nil {
			color = *input.Color
		}

		if err := entity.NewValidationError(
			entity.ValidateLabelName("name", name),
			entity.ValidateLabelColor("color", color),
		); err != nil {
			return err
		}

		groupID := current.GroupID
		if input.GroupID != nil {
			groupID = *input.GroupID
		}

		if groupID != uuid.Nil && groupID != current.GroupID {
			if _, err := s.groups.GetByID(ctx, workspaceID, groupID); err != nil {
				return err
			}
		}

		updated, err = s.labels.UpdateSettings(ctx, labelID, name, color, groupID)
		if err != nil {
			return err
		}

		return s.labels.SyncApplicationGroup(ctx, labelID, groupID)
	})
	if err != nil {
		return entity.Label{}, err
	}

	return updated, nil
}

func (s *labelsService) Merge(
	ctx context.Context,
	workspaceID, sourceID, targetID uuid.UUID,
) (entity.Label, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Label{}, err
	}

	if sourceID == targetID {
		return entity.Label{}, unsupported("intoLabelId")
	}

	var merged entity.Label

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		first, second := sourceID, targetID
		if second.String() < first.String() {
			first, second = second, first
		}

		if _, err := s.labels.LockByID(ctx, workspaceID, first); err != nil {
			return err
		}

		if _, err := s.labels.LockByID(ctx, workspaceID, second); err != nil {
			return err
		}

		source, err := s.visible(ctx, workspaceID, sourceID, decision)
		if err != nil {
			return err
		}

		target, err := s.visible(ctx, workspaceID, targetID, decision)
		if err != nil {
			return err
		}

		for _, label := range []entity.Label{source, target} {
			if label.TeamID == uuid.Nil {
				continue
			}

			if _, err := s.liveTeam(ctx, workspaceID, label.TeamID, decision); err != nil {
				return err
			}
		}

		if !target.Covers(source) {
			return entity.ErrLabelMergeScopeNarrows
		}

		if target.GroupID != source.GroupID {
			return entity.ErrLabelMergeGroupMismatch
		}

		if err := s.labels.MoveApplications(ctx, source, target); err != nil {
			return err
		}

		if err := s.labels.Delete(ctx, source.ID); err != nil {
			return err
		}

		merged = target

		return nil
	})
	if err != nil {
		return entity.Label{}, err
	}

	return merged, nil
}

func (s *labelsService) Usage(
	ctx context.Context,
	workspaceID, labelID uuid.UUID,
) (entity.LabelUsage, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.LabelUsage{}, err
	}

	if _, err := s.visible(ctx, workspaceID, labelID, decision); err != nil {
		return entity.LabelUsage{}, err
	}

	return s.labels.Usage(ctx, labelID, decision.Scope)
}

func (s *labelsService) Remove(
	ctx context.Context,
	workspaceID, labelID uuid.UUID,
	acknowledgedIssues int,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if acknowledgedIssues < 0 {
		return unsupported("acknowledgedIssues")
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.labels.LockByID(ctx, workspaceID, labelID); err != nil {
			return err
		}

		label, err := s.visible(ctx, workspaceID, labelID, decision)
		if err != nil {
			return err
		}

		if label.TeamID != uuid.Nil {
			if _, err := s.liveTeam(ctx, workspaceID, label.TeamID, decision); err != nil {
				return err
			}
		}

		usage, err := s.labels.Usage(ctx, labelID, decision.Scope)
		if err != nil {
			return err
		}

		if usage.Issues > acknowledgedIssues {
			return entity.LabelUsageChangedError(usage)
		}

		return s.labels.Delete(ctx, labelID)
	})
}

func (s *labelsService) Groups(ctx context.Context, workspaceID uuid.UUID) ([]entity.LabelGroup, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.groups.ListByWorkspaceID(ctx, workspaceID)
}

func (s *labelsService) CreateGroup(
	ctx context.Context,
	workspaceID uuid.UUID,
	name string,
) (entity.LabelGroup, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return entity.LabelGroup{}, err
	}

	if err := entity.NewValidationError(entity.ValidateLabelGroupName("name", name)); err != nil {
		return entity.LabelGroup{}, err
	}

	return s.groups.Create(ctx, entity.LabelGroup{
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(name),
	})
}

func (s *labelsService) RenameGroup(
	ctx context.Context,
	workspaceID, groupID uuid.UUID,
	name string,
) (entity.LabelGroup, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return entity.LabelGroup{}, err
	}

	if err := entity.NewValidationError(entity.ValidateLabelGroupName("name", name)); err != nil {
		return entity.LabelGroup{}, err
	}

	if _, err := s.groups.GetByID(ctx, workspaceID, groupID); err != nil {
		return entity.LabelGroup{}, err
	}

	return s.groups.UpdateName(ctx, groupID, strings.TrimSpace(name))
}

func (s *labelsService) RemoveGroup(ctx context.Context, workspaceID, groupID uuid.UUID) error {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.groups.GetByID(ctx, workspaceID, groupID); err != nil {
			return err
		}

		if err := s.groups.Ungroup(ctx, groupID); err != nil {
			return err
		}

		return s.groups.Delete(ctx, groupID)
	})
}
