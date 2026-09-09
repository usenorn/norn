package issue

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *issuesService) route(
	ctx context.Context,
	arriving *entity.Issue,
	decision entity.Decision,
	declared entity.TriageSource,
) error {
	settings, err := s.triage.Settings(ctx, arriving.WorkspaceID, arriving.TeamID)
	if err != nil {
		if errors.Is(err, entity.ErrTriageDisabled) {
			return nil
		}

		return err
	}

	onTeam, err := s.onTeam(ctx, arriving.WorkspaceID, arriving.TeamID, decision)
	if err != nil {
		return err
	}

	source := declared
	if source == "" {
		source = entity.TriageSourceOf(decision.Actor.Kind)
	}

	if !settings.Routes(source, onTeam) {
		return nil
	}

	arriving.TriageState = entity.TriageStateWaiting
	arriving.TriageSource = source

	return nil
}

func (s *issuesService) onTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (bool, error) {
	teams, err := s.teams.ListByWorkspaceMember(ctx, workspaceID, decision.Actor.AccountID)
	if err != nil {
		return false, err
	}

	for _, team := range teams {
		if team.ID == teamID {
			return true, nil
		}
	}

	return false, nil
}
