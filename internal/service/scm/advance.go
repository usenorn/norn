package scm

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

// advance moves an issue when the team asked for it and a linked change merged. Everything
// that can stop it — an unconfigured team, a deleted target state, an issue already there,
// open children, a person editing at the same moment — leaves the link recorded and the
// issue alone. Reflecting the state of a change and moving the issue are two separate
// promises, and the first must keep working when the second is switched off.
func (s *sync) advance(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	link entity.CodeLink,
) error {
	issue, err := s.issues.GetVisible(ctx, connection.WorkspaceID, link.IssueID, decision.Scope)
	if err != nil {
		// Reach was lost between recording the link and acting on it. Going quiet is right:
		// the connection is bounded by a person's permissions and those just narrowed.
		return nil
	}

	settings, err := s.settings.Settings(ctx, connection.WorkspaceID, issue.TeamID)
	if errors.Is(err, entity.ErrSCMTeamSettingsNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if !settings.Advances(link) {
		return nil
	}

	states, err := s.states.ListByTeamID(ctx, issue.TeamID)
	if err != nil {
		return err
	}

	target, found := settings.TargetState(states)
	if !found {
		logging.From(ctx).WarnContext(
			ctx,
			"a merged change could not advance its issue because the state the team chose no "+
				"longer exists",
			"issue_id", issue.ID.String(),
			"team_id", issue.TeamID.String(),
		)

		return nil
	}

	if issue.State.ID == target.ID {
		return s.links.MarkAdvanced(ctx, link.ID)
	}

	scoped := identity.WithActor(ctx, connection.Actor())

	if err := s.moveIssue(scoped, issue, target.ID); err != nil {
		return err
	}

	return s.links.MarkAdvanced(ctx, link.ID)
}

// moveIssue reads the issue's version and offers it back, so the conflict machinery decides
// whether a person moved it first. One retry covers the ordinary race of a person saving as
// a merge arrives; a second conflict means somebody is actively working on it, and an
// integration that keeps trying would be overwriting them rather than reflecting a merge.
func (s *sync) moveIssue(ctx context.Context, issue entity.Issue, stateID uuid.UUID) error {
	for attempt := range 2 {
		_, err := s.issueWriter.Update(ctx, issue.WorkspaceID, issue.ID, service.UpdateIssueInput{
			ExpectedVersion: issue.Version,
			StateID:         &stateID,
		})

		switch {
		case err == nil:
			return nil

		case errors.Is(err, entity.ErrIssueStale) && attempt == 0:
			refreshed, readErr := s.issues.GetVisible(
				ctx,
				issue.WorkspaceID,
				issue.ID,
				entity.TeamScope{WorkspaceID: issue.WorkspaceID, AllTeams: true, IncludePrivate: true},
			)
			if readErr != nil {
				return nil
			}

			issue = refreshed

		case errors.Is(err, entity.ErrIssueStale):
			logging.From(ctx).InfoContext(
				ctx,
				"a merged change did not advance its issue because somebody was editing it",
				"issue_id", issue.ID.String(),
			)

			return nil

		case errors.Is(err, entity.ErrIssueChildrenOpen):
			logging.From(ctx).InfoContext(
				ctx,
				"a merged change did not advance its issue because it still has open children",
				"issue_id", issue.ID.String(),
			)

			return nil

		case errors.Is(err, entity.ErrIssueNotFound):
			return nil

		default:
			return err
		}
	}

	return nil
}
