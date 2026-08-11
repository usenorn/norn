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

	if issue.State.ID == target.ID {
		return nil
	}

	now := time.Now().UTC()

	claimed, err := s.links.ClaimTransition(ctx, link.ID, link.State, issue.ID, target.ID, now)
	if err != nil {
		return err
	}

	if !claimed {
		return nil
	}

	blocked, err := s.drive(ctx, from, decision, link, issue, target.ID)
	if err != nil {
		return err
	}

	if blocked != "" {
		return s.links.DeferTransition(ctx, link.ID, link.State, blocked, now)
	}

	tally.advanced++

	return nil
}

func (s *sync) drive(
	ctx context.Context,
	from source,
	decision entity.Decision,
	link entity.CodeLink,
	issue entity.Issue,
	target uuid.UUID,
) (entity.CodeTransitionBlock, error) {
	author := s.attribute(ctx, from, decision, link)

	held, waiting, err := s.holds.Hold(ctx, author, issue, []entity.AgentAction{entity.AgentActionStateChange}, entity.AgentChange{
		StateID: &target,
	})
	if err != nil {
		return "", err
	}

	if waiting {
		logging.From(ctx).InfoContext(
			ctx,
			"a change written by an agent is waiting for somebody to approve the move",
			"issue_id", issue.ID.String(),
			"proposal_id", held.ID.String(),
		)

		return "", nil
	}

	return s.moveIssue(identity.WithActor(ctx, actorOf(author, from)), issue, target)
}

func (s *sync) moveIssue(
	ctx context.Context,
	issue entity.Issue,
	stateID uuid.UUID,
) (entity.CodeTransitionBlock, error) {
	for attempt := range 2 {
		_, err := s.issueWriter.Update(ctx, issue.WorkspaceID, issue.ID, service.UpdateIssueInput{
			ExpectedVersion: issue.Version,
			StateID:         &stateID,
		})

		if blocked, deferrable := entity.CodeTransitionBlockedBy(err); deferrable {
			logging.From(ctx).InfoContext(
				ctx,
				"a merged change is waiting to advance its issue, and will when the way clears",
				"issue_id", issue.ID.String(),
				"blocked_by", string(blocked),
			)

			return blocked, nil
		}

		switch {
		case err == nil:
			return "", nil

		case errors.Is(err, entity.ErrIssueStale) && attempt == 0:
			refreshed, readErr := s.issues.GetVisible(
				ctx,
				issue.WorkspaceID,
				issue.ID,
				entity.TeamScope{WorkspaceID: issue.WorkspaceID, AllTeams: true, IncludePrivate: true},
			)
			if readErr != nil {
				return "", nil
			}

			issue = refreshed

		case errors.Is(err, entity.ErrIssueStale):
			logging.From(ctx).InfoContext(
				ctx,
				"a merged change did not advance its issue because somebody was editing it",
				"issue_id", issue.ID.String(),
			)

			return "", nil

		case errors.Is(err, entity.ErrIssueNotFound):
			return "", nil

		default:
			return "", err
		}
	}

	return "", nil
}

func actorOf(decision entity.Decision, from source) entity.Actor {
	if decision.Actor.Kind == entity.ActorKindAgent {
		return decision.Actor
	}

	return from.connection.Actor()
}
