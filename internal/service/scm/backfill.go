package scm

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *sync) Backfill(ctx context.Context, repositoryID uuid.UUID) error {
	from, err := s.sourceFor(ctx, repositoryID)
	if err != nil {
		if errors.Is(err, entity.ErrSCMRepositoryNotFound) {
			return nil
		}

		return err
	}

	if from.repository.BackfilledAt != nil {
		return nil
	}

	decision, err := s.decide(ctx, from)
	if err != nil {
		if errors.Is(err, entity.ErrAccountForbidden) ||
			errors.Is(err, entity.ErrMembershipNotFound) {
			return nil
		}

		return err
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	target := from.target()

	cursor := ""
	calls := 0
	linked := 0

	for calls < s.cfg.CallsPerCycle {
		page, err := forge.Changes(ctx, target, time.Time{}, cursor)
		if err != nil {
			return s.handleForgeError(ctx, from, err)
		}

		calls++

		for _, change := range page.Changes {
			if change.State.Settled() {
				continue
			}

			tally := &deliveryTally{}

			if err := s.applyChange(ctx, from, decision, tally, change); err != nil {
				return err
			}

			linked += tally.linked
		}

		if page.Cursor == "" {
			break
		}

		cursor = page.Cursor
	}

	logging.From(ctx).InfoContext(
		ctx,
		"read the changes a repository already had when it was connected",
		"repository_id", repositoryID.String(),
		"linked", linked,
		"exhausted", calls >= s.cfg.CallsPerCycle,
	)

	return s.repositories.RecordBackfilled(ctx, repositoryID, time.Now().UTC())
}
