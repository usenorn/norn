package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type fanOut struct {
	outbox      repository.WebhookOutbox
	webhooks    repository.Webhook
	deliveries  repository.WebhookDelivery
	memberships repository.Membership
	jobs        repository.JobProducer
	authorizer  service.Authorizer
	transactor  repository.Transactor
	cfg         config.Webhooks
}

func NewFanOut(
	outbox repository.WebhookOutbox,
	webhooks repository.Webhook,
	deliveries repository.WebhookDelivery,
	memberships repository.Membership,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	cfg config.Webhooks,
) service.WebhookFanOut {
	return &fanOut{
		outbox:      outbox,
		webhooks:    webhooks,
		deliveries:  deliveries,
		memberships: memberships,
		jobs:        jobs,
		authorizer:  authorizer,
		transactor:  transactor,
		cfg:         cfg,
	}
}

func (s *fanOut) FanOut(ctx context.Context) (int, error) {
	entries, err := s.outbox.ClaimPending(ctx, s.cfg.FanOutBatch)
	if err != nil {
		return 0, err
	}

	queued := 0

	var failures []error

	for _, entry := range entries {
		delivered, err := s.spread(ctx, entry)
		if err != nil {
			failures = append(failures, fmt.Errorf("spread webhook outbox entry %s: %w", entry.ID, err))

			continue
		}

		queued += delivered
	}

	if queued > 0 {
		logging.From(ctx).InfoContext(
			ctx, "webhook events fanned out", "events", len(entries), "deliveries", queued,
		)
	}

	return queued, errors.Join(failures...)
}

func (s *fanOut) spread(ctx context.Context, entry entity.WebhookOutboxEntry) (int, error) {
	hooks, err := s.webhooks.ListSubscribed(ctx, entry.WorkspaceID, entry.Event)
	if err != nil {
		return 0, err
	}

	deliveries := make([]entity.WebhookDelivery, 0, len(hooks))

	for _, hook := range hooks {
		reaches, err := s.reaches(ctx, hook, entry)
		if err != nil {
			return 0, err
		}

		if !reaches {
			continue
		}

		deliveries = append(deliveries, entity.WebhookDelivery{
			ID:          uuid.New(),
			WebhookID:   hook.ID,
			WorkspaceID: entry.WorkspaceID,
			OutboxID:    entry.ID,
			Event:       entry.Event,
			SubjectKind: entry.SubjectKind,
			SubjectID:   entry.SubjectID,
			TeamID:      entry.TeamID,
			Body:        entry.Body,
			NextAttempt: time.Now().UTC(),
		})
	}

	if len(deliveries) == 0 {
		return 0, nil
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.deliveries.Queue(ctx, deliveries)
	}); err != nil {
		return 0, err
	}

	for _, delivery := range deliveries {
		s.wake(ctx, delivery)
	}

	return len(deliveries), nil
}

func (s *fanOut) reaches(
	ctx context.Context,
	hook entity.Webhook,
	entry entity.WebhookOutboxEntry,
) (bool, error) {
	membership, err := s.memberships.Get(ctx, entry.WorkspaceID, hook.OwnerAccountID)
	if err != nil {
		return false, nil
	}

	if membership.Deactivated() {
		return false, nil
	}

	scoped := identity.WithActor(ctx, entity.Actor{
		Kind:           entity.ActorKindToken,
		AccountID:      hook.OwnerAccountID,
		OwnerAccountID: hook.OwnerAccountID,
		TokenName:      hook.Name,
	})

	decision, err := s.authorizer.Decide(scoped, entity.AccessRequest{
		Resource:    entity.WebhookEventResource(entry.Event),
		Action:      entity.ActionRead,
		WorkspaceID: entry.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		return false, nil
	}

	switch entity.WebhookEventScope(entry.Event) {
	case entity.WebhookScopeWorkspace:
		return decision.Role.Valid(), nil
	case entity.WebhookScopeAdmin:
		return decision.Role == entity.MembershipRoleAdmin, nil
	case entity.WebhookScopeAdminTeam:
		return decision.Role == entity.MembershipRoleAdmin && covers(decision.Scope, entry.TeamID), nil
	default:
		return covers(decision.Scope, entry.TeamID), nil
	}
}

func covers(scope entity.TeamScope, teamID uuid.UUID) bool {
	if teamID == uuid.Nil {
		return true
	}

	return scope.Covers(teamID)
}

func (s *fanOut) wake(ctx context.Context, delivery entity.WebhookDelivery) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		if err := s.jobs.EnqueueWebhookDeliver(ctx, entity.WebhookDeliverPayload{
			DeliveryID: delivery.ID,
			Attempt:    0,
		}, delivery.NextAttempt); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"queueing a webhook delivery failed, so the scheduled rescue will carry it",
				"delivery_id", delivery.ID.String(),
				"error", err.Error(),
			)
		}
	})
}
