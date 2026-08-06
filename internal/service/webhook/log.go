package webhook

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type deliveryLog struct {
	webhooks   repository.Webhook
	deliveries repository.WebhookDelivery
	jobs       repository.JobProducer
	authorizer service.Authorizer
	transactor repository.Transactor
}

func NewLog(
	webhooks repository.Webhook,
	deliveries repository.WebhookDelivery,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.WebhookDeliveries {
	return &deliveryLog{
		webhooks:   webhooks,
		deliveries: deliveries,
		jobs:       jobs,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *deliveryLog) reachable(
	ctx context.Context,
	workspaceID, webhookID uuid.UUID,
) (entity.Webhook, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.Webhook{}, err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.Webhook{}, entity.ErrWebhookNotPermitted
	}

	return s.webhooks.Get(ctx, workspaceID, webhookID)
}

func (s *deliveryLog) List(
	ctx context.Context,
	workspaceID, webhookID uuid.UUID,
	input service.ListWebhookDeliveriesInput,
) (service.WebhookDeliveryPage, error) {
	hook, err := s.reachable(ctx, workspaceID, webhookID)
	if err != nil {
		return service.WebhookDeliveryPage{}, err
	}

	filter := entity.WebhookDeliveryFilter{
		WebhookID: hook.ID,
		Events:    input.Events,
		States:    input.States,
		Limit:     input.Limit,
	}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeWebhookDeliveryCursor(input.Cursor)
		if err != nil {
			return service.WebhookDeliveryPage{}, err
		}

		filter.Cursor = &cursor
	}

	found, err := s.deliveries.List(ctx, filter.Lookahead())
	if err != nil {
		return service.WebhookDeliveryPage{}, err
	}

	page := service.WebhookDeliveryPage{Deliveries: found}

	if len(found) > filter.Limit {
		page.Deliveries = found[:filter.Limit]
		page.NextCursor = page.Deliveries[len(page.Deliveries)-1].Cursor().Encode()
	}

	return page, nil
}

func (s *deliveryLog) Get(
	ctx context.Context,
	workspaceID, webhookID, deliveryID uuid.UUID,
) (entity.WebhookDelivery, error) {
	hook, err := s.reachable(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	delivery, err := s.deliveries.Get(ctx, hook.ID, deliveryID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	attempts, err := s.deliveries.Attempts(ctx, delivery.ID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	delivery.Attempts = attempts

	return delivery, nil
}

func (s *deliveryLog) Test(
	ctx context.Context,
	workspaceID, webhookID uuid.UUID,
) (entity.WebhookDelivery, error) {
	hook, err := s.reachable(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	body, err := json.Marshal(service.WebhookTest(hook))
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	return s.queue(ctx, entity.WebhookDelivery{
		ID:          uuid.New(),
		WebhookID:   hook.ID,
		WorkspaceID: workspaceID,
		Event:       entity.WebhookTest,
		SubjectKind: "webhook",
		SubjectID:   hook.ID,
		Body:        body,
		NextAttempt: time.Now().UTC(),
	})
}

func (s *deliveryLog) Replay(
	ctx context.Context,
	workspaceID, webhookID, deliveryID uuid.UUID,
) (entity.WebhookDelivery, error) {
	hook, err := s.reachable(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	original, err := s.deliveries.Get(ctx, hook.ID, deliveryID)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	if !original.Replayable() {
		return entity.WebhookDelivery{}, entity.ErrWebhookDeliveryNotReplayable
	}

	return s.queue(ctx, entity.WebhookDelivery{
		ID:          uuid.New(),
		WebhookID:   hook.ID,
		WorkspaceID: workspaceID,
		ReplayOf:    original.ID,
		Event:       original.Event,
		SubjectKind: original.SubjectKind,
		SubjectID:   original.SubjectID,
		TeamID:      original.TeamID,
		Body:        original.Body,
		NextAttempt: time.Now().UTC(),
	})
}

func (s *deliveryLog) queue(
	ctx context.Context,
	delivery entity.WebhookDelivery,
) (entity.WebhookDelivery, error) {
	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.deliveries.Queue(ctx, []entity.WebhookDelivery{delivery})
	}); err != nil {
		return entity.WebhookDelivery{}, err
	}

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

	delivery.State = entity.WebhookDeliveryPending

	return delivery, nil
}
