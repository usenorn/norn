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
	due, err := s.connections.ClaimDue(ctx, at, s.cfg.ReconcileBatch)
	if err != nil {
		return err
	}

	for _, connection := range due {
		if err := s.reconcileOne(ctx, connection, at); err != nil {
			// One connection's forge being unreachable must not stop the sweep for every
			// other workspace on this instance.
			logging.From(ctx).WarnContext(
				ctx,
				"reconciling a source control connection failed",
				"connection_id", connection.ID.String(),
				"error", err.Error(),
			)
		}
	}

	return nil
}

func (s *sync) reconcileOne(
	ctx context.Context,
	connection entity.SCMConnection,
	at time.Time,
) error {
	credentials, err := s.connections.Credentials(ctx, connection.ID)
	if err != nil {
		return err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return err
	}

	target := connection.Target(credentials.Token)

	// A broken connection costs one call per cycle, not a walk of its history. Reading it
	// fully would burn a rate limit on a credential that is known not to work, and the only
	// question worth asking is whether somebody has replaced it yet.
	found, err := forge.Repository(ctx, target)
	if err != nil {
		return s.handleForgeError(ctx, connection, err)
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return s.handleForgeError(ctx, connection, err)
	}

	if err := s.connections.MarkVerified(ctx, connection.ID, found, login, at); err != nil {
		return err
	}

	if connection.Broken() {
		// It has only just come back. Reading from the last verified point would replay
		// however long it was broken, so the catch-up window bounds it instead.
		connection.Status = entity.SCMConnectionConnected
	}

	if connection.ExternalHookID == "" {
		s.installMissingHook(ctx, connection, target, credentials.WebhookSecret)
	}

	if err := s.drainPending(ctx, connection, at); err != nil {
		return err
	}

	decision, err := s.decide(ctx, connection)
	if err != nil {
		if errors.Is(err, entity.ErrAccountForbidden) || errors.Is(err, entity.ErrMembershipNotFound) {
			s.markBroken(
				ctx,
				connection,
				entity.SCMBrokenCredentialsRejected,
				"the account that established this connection no longer has access to the workspace",
			)

			return nil
		}

		return err
	}

	if err := s.refreshChanges(ctx, connection, target, decision, at); err != nil {
		return err
	}

	if err := s.pushMirrors(ctx, connection, target, decision, at); err != nil {
		return s.handleForgeError(ctx, connection, err)
	}

	return nil
}

func (s *sync) handleForgeError(
	ctx context.Context,
	connection entity.SCMConnection,
	cause error,
) error {
	var limited entity.SCMRateLimitedError
	if errors.As(cause, &limited) {
		wait := entity.ClampImportBackoff(limited.RetryAfter, s.cfg.MinBackoff, s.cfg.MaxBackoff)

		return s.connections.Park(ctx, connection.ID, time.Now().UTC().Add(wait))
	}

	if reason, detail, actionable := entity.SCMBrokenBy(cause); actionable {
		s.markBroken(ctx, connection, reason, detail)

		return nil
	}

	return cause
}

func (s *sync) installMissingHook(
	ctx context.Context,
	connection entity.SCMConnection,
	target entity.SCMTarget,
	secret string,
) {
	hookID, err := s.forgeHook(ctx, connection, target, secret)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"installing a missing source control hook failed; the next sweep will try again",
			"connection_id", connection.ID.String(),
			"error", err.Error(),
		)

		return
	}

	if err := s.connections.RecordHook(ctx, connection.ID, hookID); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a source control hook failed",
			"connection_id", connection.ID.String(),
			"error", err.Error(),
		)
	}
}

func (s *sync) forgeHook(
	ctx context.Context,
	connection entity.SCMConnection,
	target entity.SCMTarget,
	secret string,
) (string, error) {
	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return "", err
	}

	return forge.InstallHook(ctx, service.ForgeHookRequest{
		Target:      target,
		CallbackURL: s.callback(connection),
		Secret:      secret,
	})
}

func (s *sync) callback(connection entity.SCMConnection) string {
	return s.baseURL + "/v1/source-control/" +
		string(connection.Provider) + "/" + connection.ID.String()
}

// drainPending picks up deliveries that were stored but never carried out, which is what a
// lost enqueue looks like from here.
func (s *sync) drainPending(
	ctx context.Context,
	connection entity.SCMConnection,
	at time.Time,
) error {
	pending, err := s.deliveries.ListPending(ctx, connection.ID, at, s.cfg.ReconcileBatch)
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
	connection entity.SCMConnection,
	target entity.SCMTarget,
	decision entity.Decision,
	at time.Time,
) error {
	since := at.Add(-s.cfg.MaxCatchUp)
	if connection.ReconciledAt != nil && connection.ReconciledAt.After(since) {
		since = *connection.ReconciledAt
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return err
	}

	cursor := ""
	calls := 0

	for calls < s.cfg.CallsPerCycle {
		page, err := forge.Changes(ctx, target, since, cursor)
		if err != nil {
			return s.handleForgeError(ctx, connection, err)
		}

		calls++

		for _, change := range page.Changes {
			if err := s.applyChange(ctx, connection, decision, change); err != nil {
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
			"connection_id", connection.ID.String(),
			"calls", calls,
		)
	}

	return s.connections.RecordReconciled(ctx, connection.ID, "", at)
}
