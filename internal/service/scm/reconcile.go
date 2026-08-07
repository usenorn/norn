package scm

import (
	"context"
	"errors"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

// Reconcile is what makes a missed delivery temporary rather than permanent. A forge that
// could not reach this instance, a delivery dropped between the edge and the queue, and a
// hook that was never installed all heal here, and a connection whose token was replaced
// discovers that it works again without anybody pressing anything.
func (s *sync) Reconcile(ctx context.Context, at time.Time) error {
	due, err := s.repositories.ClaimDue(ctx, at, s.cfg.ReconcileBatch)
	if err != nil {
		return err
	}

	for _, stored := range due {
		from, err := s.sourceFor(ctx, stored.ID)
		if err != nil {
			logWarn(ctx, "reading a repository to reconcile it failed", stored.ID, err)

			continue
		}

		if err := s.reconcileOne(ctx, from, at); err != nil {
			// One repository's forge being unreachable must not stop the sweep for every
			// other workspace on this instance.
			logging.From(ctx).WarnContext(
				ctx,
				"reconciling a connected repository failed",
				"repository_id", stored.ID.String(),
				"error", err.Error(),
			)
		}
	}

	return nil
}

func (s *sync) reconcileOne(
	ctx context.Context,
	from source,
	at time.Time,
) error {
	secret, err := s.repositories.WebhookSecret(ctx, from.repository.ID)
	if err != nil {
		return err
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	target := from.target()

	// A broken connection costs one call per cycle, not a walk of its history. Reading it
	// fully would burn a rate limit on a credential that is known not to work, and the only
	// question worth asking is whether somebody has replaced it yet.
	if _, err := forge.Repository(ctx, target); err != nil {
		return s.handleForgeError(ctx, from, err)
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return s.handleForgeError(ctx, from, err)
	}

	if err := s.connections.MarkVerified(ctx, from.connection.ID, login, at); err != nil {
		return err
	}

	if from.repository.ExternalHookID == "" {
		s.installMissingHook(ctx, from, target, secret)
	}

	if err := s.drainPending(ctx, from, at); err != nil {
		return err
	}

	decision, err := s.decide(ctx, from)
	if err != nil {
		if errors.Is(err, entity.ErrAccountForbidden) || errors.Is(err, entity.ErrMembershipNotFound) {
			s.markBroken(
				ctx,
				from,
				entity.SCMBrokenCredentialsRejected,
				"the account that established this connection no longer has access to the workspace",
			)

			return nil
		}

		return err
	}

	if err := s.refreshChanges(ctx, from, target, decision, at); err != nil {
		return err
	}

	if err := s.pushMirrors(ctx, from, target, decision, at); err != nil {
		return s.handleForgeError(ctx, from, err)
	}

	return nil
}

func (s *sync) handleForgeError(
	ctx context.Context,
	from source,
	cause error,
) error {
	var limited entity.SCMRateLimitedError
	if errors.As(cause, &limited) {
		wait := entity.ClampImportBackoff(limited.RetryAfter, s.cfg.MinBackoff, s.cfg.MaxBackoff)

		return s.repositories.Park(ctx, from.repository.ID, time.Now().UTC().Add(wait))
	}

	if reason, detail, actionable := entity.SCMBrokenBy(cause); actionable {
		s.markBroken(ctx, from, reason, detail)

		return nil
	}

	return cause
}

func (s *sync) installMissingHook(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	secret string,
) {
	hookID, err := s.forgeHook(ctx, from, target, secret)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"installing a missing source control hook failed; the next sweep will try again",
			"repository_id", from.repository.ID.String(),
			"error", err.Error(),
		)

		return
	}

	if err := s.repositories.RecordHook(ctx, from.repository.ID, hookID); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a source control hook failed",
			"repository_id", from.repository.ID.String(),
			"error", err.Error(),
		)
	}
}

func (s *sync) forgeHook(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	secret string,
) (string, error) {
	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return "", err
	}

	return forge.InstallHook(ctx, service.ForgeHookRequest{
		Target:      target,
		CallbackURL: s.callback(from),
		Secret:      secret,
	})
}

func (s *sync) callback(from source) string {
	return s.baseURL + "/v1/source-control/" +
		string(from.connection.Provider) + "/" + from.repository.ID.String()
}

// drainPending picks up deliveries that were stored but never carried out, which is what a
// lost enqueue looks like from here.
func (s *sync) drainPending(
	ctx context.Context,
	from source,
	at time.Time,
) error {
	pending, err := s.deliveries.ListPending(ctx, from.repository.ID, at, s.cfg.ReconcileBatch)
	if err != nil {
		return err
	}

	for _, delivery := range pending {
		if err := s.Apply(ctx, delivery.ID); err != nil {
			return err
		}
	}

	return nil
}

// refreshChanges asks the forge what has moved since it was last read. This is the half that
// catches what no delivery ever arrived for, so it asks for every state rather than the open
// ones a forge lists by default — the merge it exists to notice has already closed.
func (s *sync) refreshChanges(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	decision entity.Decision,
	at time.Time,
) error {
	since := at.Add(-s.cfg.MaxCatchUp)
	if from.repository.ReconciledAt != nil && from.repository.ReconciledAt.After(since) {
		since = *from.repository.ReconciledAt
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	cursor := ""
	calls := 0

	for calls < s.cfg.CallsPerCycle {
		page, err := forge.Changes(ctx, target, since, cursor)
		if err != nil {
			return s.handleForgeError(ctx, from, err)
		}

		calls++

		for _, change := range page.Changes {
			// The sweep has no delivery to explain, so its tally is discarded.
			if err := s.applyChange(ctx, from, decision, &deliveryTally{}, change); err != nil {
				return err
			}
		}

		if page.Cursor == "" {
			break
		}

		cursor = page.Cursor
	}

	if calls >= s.cfg.CallsPerCycle {
		logging.From(ctx).InfoContext(
			ctx,
			"a reconcile cycle stopped at its call budget; the next cycle carries on from here",
			"repository_id", from.repository.ID.String(),
			"calls", calls,
		)
	}

	return s.repositories.RecordReconciled(ctx, from.repository.ID, "", at)
}
