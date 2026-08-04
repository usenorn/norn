package savedview

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type savedViewsService struct {
	views      repository.SavedView
	references repository.IssueFilterReference
	teams      repository.Team
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	views repository.SavedView,
	references repository.IssueFilterReference,
	teams repository.Team,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.SavedViews {
	return &savedViewsService{
		views:      views,
		references: references,
		teams:      teams,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *savedViewsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
	scoped bool,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceSavedView,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      scoped,
	})
}

func (s *savedViewsService) audience(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
) ([]uuid.UUID, error) {
	teams, err := s.teams.ListByWorkspaceMember(ctx, workspaceID, decision.Actor.AccountID)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(teams))
	for _, team := range teams {
		ids = append(ids, team.ID)
	}

	return ids, nil
}

func (s *savedViewsService) visible(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
	decision entity.Decision,
) (entity.SavedView, error) {
	view, err := s.views.GetByID(ctx, workspaceID, savedViewID)
	if err != nil {
		return entity.SavedView{}, err
	}

	teamIDs, err := s.audience(ctx, workspaceID, decision)
	if err != nil {
		return entity.SavedView{}, err
	}

	if view.VisibleTo(decision.Actor.AccountID, teamIDs) {
		return view, nil
	}

	if view.Shared() && decision.Role == entity.MembershipRoleAdmin {
		return view, nil
	}

	return entity.SavedView{}, entity.ErrSavedViewNotFound
}

func editable(view entity.SavedView, decision entity.Decision) error {
	if view.EditableBy(decision.Actor.AccountID, decision.Role) {
		return nil
	}

	return entity.ErrSavedViewNotOwner
}

func shareable(sharing entity.SavedViewSharing, decision entity.Decision) error {
	if !sharing.Shared() {
		return nil
	}

	if decision.Role == entity.MembershipRoleViewer {
		return entity.ErrSavedViewNotShareable
	}

	return nil
}

func (s *savedViewsService) scopedTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) error {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.NewValidationError(entity.FieldError{
			Field: "teamId",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return nil
}

func summarise(views []entity.SavedView, decision entity.Decision) []service.SavedViewSummary {
	summaries := make([]service.SavedViewSummary, 0, len(views))

	for _, view := range views {
		summaries = append(summaries, service.SavedViewSummary{
			View:     view,
			Editable: view.EditableBy(decision.Actor.AccountID, decision.Role),
		})
	}

	return summaries
}

func (s *savedViewsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.SavedViewSummary, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead, false)
	if err != nil {
		return nil, err
	}

	teamIDs, err := s.audience(ctx, workspaceID, decision)
	if err != nil {
		return nil, err
	}

	views, err := s.views.ListFor(ctx, workspaceID, decision.Actor.AccountID, teamIDs)
	if err != nil {
		return nil, err
	}

	return summarise(views, decision), nil
}

func (s *savedViewsService) Get(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
) (service.SavedViewDetail, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead, true)
	if err != nil {
		return service.SavedViewDetail{}, err
	}

	view, err := s.visible(ctx, workspaceID, savedViewID, decision)
	if err != nil {
		return service.SavedViewDetail{}, err
	}

	references := entity.IssueFilterReferences(view.Filter)

	if len(references) > 0 {
		references, err = s.references.Resolve(ctx, workspaceID, decision.Scope, references)
		if err != nil {
			return service.SavedViewDetail{}, err
		}
	}

	return service.SavedViewDetail{
		Summary: service.SavedViewSummary{
			View:     view,
			Editable: view.EditableBy(decision.Actor.AccountID, decision.Role),
		},
		References: references,
	}, nil
}

func (s *savedViewsService) Create(
	ctx context.Context,
	input service.CreateSavedViewInput,
) (service.SavedViewSummary, error) {
	decision, err := s.decide(ctx, input.WorkspaceID, entity.ActionManage, true)
	if err != nil {
		return service.SavedViewSummary{}, err
	}

	if input.Sharing == "" {
		input.Sharing = entity.SavedViewSharingPersonal
	}

	view := entity.SavedView{
		WorkspaceID: input.WorkspaceID,
		AuthorID:    decision.Actor.AccountID,
		Sharing:     input.Sharing,
		TeamID:      input.TeamID,
		Name:        input.Name,
		Sort:        input.Sort,
		GroupBy:     input.GroupBy,
	}

	if input.Filter != nil {
		view.Filter = *input.Filter
	}

	if err := view.Validate(); err != nil {
		return service.SavedViewSummary{}, err
	}

	if err := shareable(view.Sharing, decision); err != nil {
		return service.SavedViewSummary{}, err
	}

	if view.TeamID != uuid.Nil {
		if err := s.scopedTeam(ctx, input.WorkspaceID, view.TeamID, decision); err != nil {
			return service.SavedViewSummary{}, err
		}
	}

	created, err := s.views.Create(ctx, view)
	if err != nil {
		return service.SavedViewSummary{}, err
	}

	return service.SavedViewSummary{
		View:     created,
		Editable: created.EditableBy(decision.Actor.AccountID, decision.Role),
	}, nil
}

func (s *savedViewsService) Update(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
	input service.UpdateSavedViewInput,
) (service.SavedViewSummary, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage, true)
	if err != nil {
		return service.SavedViewSummary{}, err
	}

	var updated entity.SavedView

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.views.LockByID(ctx, workspaceID, savedViewID); err != nil {
			return err
		}

		view, err := s.visible(ctx, workspaceID, savedViewID, decision)
		if err != nil {
			return err
		}

		if err := editable(view, decision); err != nil {
			return err
		}

		merged := merge(view, input)

		if err := merged.Validate(); err != nil {
			return err
		}

		if merged.Sharing != view.Sharing {
			if err := shareable(merged.Sharing, decision); err != nil {
				return err
			}
		}

		if merged.TeamID != uuid.Nil && merged.TeamID != view.TeamID {
			if err := s.scopedTeam(ctx, workspaceID, merged.TeamID, decision); err != nil {
				return err
			}
		}

		updated, err = s.views.UpdateSettings(ctx, savedViewID, repository.SavedViewSettings{
			Name:    merged.Name,
			Sharing: merged.Sharing,
			TeamID:  merged.TeamID,
			Filter:  merged.Filter,
			Sort:    merged.Sort,
			GroupBy: merged.GroupBy,
		})

		return err
	})
	if err != nil {
		return service.SavedViewSummary{}, err
	}

	return service.SavedViewSummary{
		View:     updated,
		Editable: updated.EditableBy(decision.Actor.AccountID, decision.Role),
	}, nil
}

func merge(view entity.SavedView, input service.UpdateSavedViewInput) entity.SavedView {
	if input.Name != nil {
		view.Name = *input.Name
	}

	if input.Sharing != nil {
		view.Sharing = *input.Sharing

		if view.Sharing != entity.SavedViewSharingTeam {
			view.TeamID = uuid.Nil
		}
	}

	if input.TeamID != nil {
		view.TeamID = *input.TeamID
	}

	if input.Filter != nil {
		view.Filter = *input.Filter
	}

	if input.Sort != nil {
		view.Sort = *input.Sort
	}

	if input.GroupBy != nil {
		view.GroupBy = *input.GroupBy
	}

	return view
}

func (s *savedViewsService) Remove(
	ctx context.Context,
	workspaceID, savedViewID uuid.UUID,
	acknowledgedSharing entity.SavedViewSharing,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage, true)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.views.LockByID(ctx, workspaceID, savedViewID); err != nil {
			return err
		}

		view, err := s.visible(ctx, workspaceID, savedViewID, decision)
		if err != nil {
			return err
		}

		if err := editable(view, decision); err != nil {
			return err
		}

		if view.Sharing != acknowledgedSharing {
			return entity.SavedViewSharedError{
				Sharing:  view.Sharing,
				TeamID:   view.TeamID,
				TeamName: view.TeamName,
			}
		}

		return s.views.Delete(ctx, savedViewID)
	})
}

func (s *savedViewsService) Reorder(
	ctx context.Context,
	workspaceID uuid.UUID,
	orderedIDs []uuid.UUID,
) ([]service.SavedViewSummary, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage, false)
	if err != nil {
		return nil, err
	}

	var reordered []entity.SavedView

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		teamIDs, err := s.audience(ctx, workspaceID, decision)
		if err != nil {
			return err
		}

		views, err := s.views.ListFor(ctx, workspaceID, decision.Actor.AccountID, teamIDs)
		if err != nil {
			return err
		}

		if err := reachable(views, orderedIDs); err != nil {
			return err
		}

		if err := s.views.Place(ctx, workspaceID, decision.Actor.AccountID, orderedIDs); err != nil {
			return err
		}

		reordered, err = s.views.ListFor(ctx, workspaceID, decision.Actor.AccountID, teamIDs)

		return err
	})
	if err != nil {
		return nil, err
	}

	return summarise(reordered, decision), nil
}

func reachable(views []entity.SavedView, orderedIDs []uuid.UUID) error {
	unsupported := entity.NewValidationError(entity.FieldError{
		Field: "savedViewIds",
		Code:  entity.ValidationCodeUnsupportedValue,
	})

	known := make(map[uuid.UUID]bool, len(views))
	for _, view := range views {
		known[view.ID] = true
	}

	placed := make(map[uuid.UUID]bool, len(orderedIDs))

	for _, id := range orderedIDs {
		if !known[id] || placed[id] {
			return unsupported
		}

		placed[id] = true
	}

	return nil
}
