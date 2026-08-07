package scm

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *connections) TeamSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.SCMTeamSettings, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.SCMTeamSettings{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return entity.SCMTeamSettings{}, entity.ErrTeamNotFound
	}

	return s.teamSettings.Get(ctx, workspaceID, teamID)
}

func (s *connections) SetTeamSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	input service.SetTeamSCMSettingsInput,
) (entity.SCMTeamSettings, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return entity.SCMTeamSettings{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return entity.SCMTeamSettings{}, entity.ErrTeamNotFound
	}

	template := strings.TrimSpace(input.BranchTemplate)

	if field := entity.ValidateSCMBranchTemplate(
		"branchTemplate", template,
	); field.Field != "" {
		return entity.SCMTeamSettings{}, entity.ValidationError{
			Fields: []entity.FieldError{field},
		}
	}

	return s.teamSettings.Upsert(ctx, entity.SCMTeamSettings{
		TeamID:         teamID,
		WorkspaceID:    workspaceID,
		BranchTemplate: template,
	})
}

func (s *connections) BranchName(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (string, error) {
	decision, issue, err := s.reads(ctx, workspaceID, issueID)
	if err != nil {
		return "", err
	}

	settings, err := s.teamSettings.Get(ctx, workspaceID, issue.TeamID)
	if err != nil {
		return "", err
	}

	handle := "norn"

	if account, err := s.accounts.GetByID(ctx, decision.Actor.Authority()); err == nil {
		handle = entity.SlugifyBranchPart(account.DisplayName)
	}
	reference := entity.IssueReferenceText(issue.ReferenceKey, issue.Number)

	return entity.BranchNameFor(settings, handle, issue, reference), nil
}

func (s *connections) SuppressAutomation(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	suppressed bool,
) error {
	if _, _, err := s.manages(ctx, workspaceID, issueID); err != nil {
		return err
	}

	return s.issues.SetSCMAutomationSuppressed(ctx, workspaceID, issueID, suppressed)
}
