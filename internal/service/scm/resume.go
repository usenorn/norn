package scm

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *sync) Resume(ctx context.Context, workspaceID, issueID uuid.UUID) error {
	deferred, err := s.links.ListDeferredTransitions(ctx, issueID)
	if err != nil || len(deferred) == 0 {
		return err
	}

	scope := entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true, IncludePrivate: true}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, scope)
	if err != nil {
		if errors.Is(err, entity.ErrIssueNotFound) {
			return nil
		}

		return err
	}

	links, err := s.links.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}

	for _, pending := range deferred {
		if pending.StateID == uuid.Nil {
			continue
		}

		if issue.State.ID == pending.StateID {
			if err := s.links.SettleTransition(ctx, pending.LinkID, pending.Transition); err != nil {
				return err
			}

			continue
		}

		link, found := linkByID(links, pending.LinkID)
		if !found {
			continue
		}

		from, err := s.sourceFor(ctx, link.RepositoryID)
		if err != nil {
			continue
		}

		decision, err := s.decide(ctx, from)
		if err != nil {
			continue
		}

		blocked, err := s.drive(ctx, from, decision, link, issue, pending.StateID)
		if err != nil {
			return err
		}

		if blocked != "" {
			if err := s.links.DeferTransition(
				ctx, pending.LinkID, pending.Transition, blocked, time.Now().UTC(),
			); err != nil {
				return err
			}

			continue
		}

		if err := s.links.SettleTransition(ctx, pending.LinkID, pending.Transition); err != nil {
			return err
		}

		refreshed, err := s.issues.GetVisible(ctx, workspaceID, issueID, scope)
		if err != nil {
			return err
		}

		issue = refreshed

		logging.From(ctx).InfoContext(
			ctx,
			"a merged change advanced its issue now that the way is clear",
			"issue_id", issueID.String(),
			"blocked_by", string(pending.BlockedBy),
		)
	}

	return nil
}

func linkByID(links []entity.CodeLink, id uuid.UUID) (entity.CodeLink, bool) {
	for _, link := range links {
		if link.ID == id {
			return link, true
		}
	}

	return entity.CodeLink{}, false
}
