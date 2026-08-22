package execution

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

func (s *executionsService) project(ctx context.Context, execution entity.Execution) {
	if !entity.MovesTheIssue(execution.State) {
		return
	}

	states, err := s.states.ListByTeamID(ctx, execution.TeamID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"an execution could not read the states its team uses",
			slog.String("execution_id", execution.ID),
			slog.String("error", err.Error()),
		)

		return
	}

	target, wanted := entity.IssueStateFor(execution.State, states)
	if !wanted {
		return
	}

	issue, err := s.issues.GetVisible(ctx, execution.WorkspaceID, execution.IssueID, everyTeam(execution))
	if err != nil {
		return
	}

	if issue.State.ID == target.ID {
		return
	}

	s.moveIssue(ctx, execution, issue, target.ID)
}

func everyTeam(execution entity.Execution) entity.TeamScope {
	return entity.TeamScope{
		WorkspaceID:    execution.WorkspaceID,
		AllTeams:       true,
		IncludePrivate: true,
	}
}

func (s *executionsService) moveIssue(
	ctx context.Context,
	execution entity.Execution,
	issue entity.Issue,
	stateID uuid.UUID,
) {
	for attempt := range 2 {
		_, err := s.writer.Update(ctx, issue.WorkspaceID, issue.ID, service.UpdateIssueInput{
			ExpectedVersion: issue.Version,
			StateID:         &stateID,
		})

		switch {
		case err == nil:
			return

		case errors.Is(err, entity.ErrIssueStale) && attempt == 0:
			refreshed, readErr := s.issues.GetVisible(
				ctx, issue.WorkspaceID, issue.ID, everyTeam(execution),
			)
			if readErr != nil {
				return
			}

			issue = refreshed

		default:
			logging.From(ctx).InfoContext(
				ctx,
				"an execution did not move its issue",
				slog.String("execution_id", execution.ID),
				slog.String("issue_id", issue.ID.String()),
				slog.String("error", err.Error()),
			)

			return
		}
	}
}
