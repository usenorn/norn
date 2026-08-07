package scm

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *sync) advance(
	ctx context.Context,
	from source,
	decision entity.Decision,
	tally *deliveryTally,
	link entity.CodeLink,
) error {
	issue, err := s.issues.GetVisible(ctx, from.workspaceID(), link.IssueID, decision.Scope)
	if err != nil {
		return nil
	}

	if issue.SCMAutomationSuppressed {
		return nil
	}

	rules, err := s.rules.ListByTeam(ctx, from.workspaceID(), issue.TeamID)
	if err != nil {
		return err
	}

	rule, routed := rules.For(link)
	if !routed {
		return nil
	}

	states, err := s.states.ListByTeamID(ctx, issue.TeamID)
	if err != nil {
		return err
	}

	target, found := rule.TargetState(states)
	if !found {
		logging.From(ctx).WarnContext(
			ctx,
			"a change could not advance its issue because the state the team chose no longer "+
				"exists",
			"issue_id", issue.ID.String(),
			"team_id", issue.TeamID.String(),
			"trigger", string(rule.Trigger),
		)

		return nil
	}

	claimed, err := s.links.ClaimTransition(
		ctx, link.ID, link.State, issue.ID, time.Now().UTC(),
	)
	if err != nil {
		return err
	}

	if !claimed {
		return nil
	}

	if issue.State.ID == target.ID {
		return nil
	}

	tally.advanced++

	scoped := identity.WithActor(ctx, from.connection.Actor())

	return s.moveIssue(scoped, issue, target.ID)
}

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
