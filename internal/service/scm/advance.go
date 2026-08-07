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

// advance moves an issue when the team routed the state its change just reached. Everything
// that can stop it — a team with no rule for this state, a deleted target state, an issue
// already there, open children, a person editing at the same moment — leaves the link
// recorded and the issue alone. Reflecting the state of a change and moving the issue are
// two separate promises, and the first must keep working when the second is switched off.
func (s *sync) advance(
	ctx context.Context,
	from source,
	decision entity.Decision,
	tally *deliveryTally,
	link entity.CodeLink,
) error {
	issue, err := s.issues.GetVisible(ctx, from.workspaceID(), link.IssueID, decision.Scope)
	if err != nil {
		// Reach was lost between recording the link and acting on it. Going quiet is right:
		// the connection is bounded by a person's permissions and those just narrowed.
		return nil
	}

	// An issue can opt out entirely. The link is still recorded and still renders; only the
	// moving stops, which is the whole point of an exception.
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

	// The claim comes before the move, so a redelivered event cannot move an issue a person
	// has since moved back. A link that already drove this state is done with it.
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
