package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const jitterFraction = 0.1

type deliverer struct {
	webhooks   repository.Webhook
	deliveries repository.WebhookDelivery
	sender     repository.WebhookSender
	accounts   repository.Account
	workspaces repository.Workspace
	mailer     repository.Mailer
	jobs       repository.JobProducer
	audit      service.Audit
	app        config.App
	cfg        config.Webhooks
}

func NewDeliverer(
	webhooks repository.Webhook,
	deliveries repository.WebhookDelivery,
	sender repository.WebhookSender,
	accounts repository.Account,
	workspaces repository.Workspace,
	mailer repository.Mailer,
	jobs repository.JobProducer,
	audit service.Audit,
	app config.App,
	cfg config.Webhooks,
) service.WebhookDispatch {
	return &deliverer{
		webhooks:   webhooks,
		deliveries: deliveries,
		sender:     sender,
		accounts:   accounts,
		workspaces: workspaces,
		mailer:     mailer,
		jobs:       jobs,
		audit:      audit,
		app:        app,
		cfg:        cfg,
	}
}

func (s *deliverer) Deliver(ctx context.Context, payload entity.WebhookDeliverPayload) error {
	delivery, err := s.deliveries.ClaimAttempt(ctx, payload.DeliveryID, payload.Attempt)
	if err != nil {
		if errors.Is(err, entity.ErrWebhookDeliveryNotFound) {
			return nil
		}

		return err
	}

	hook, err := s.webhooks.GetForDelivery(ctx, delivery.WebhookID)
	if err != nil {
		if errors.Is(err, entity.ErrWebhookNotFound) {
			return s.deliveries.Settle(ctx, delivery.ID, entity.WebhookDeliveryFailed, time.Now().UTC())
		}

		if errors.Is(err, entity.ErrWebhookEncryptionKeyMissing) {
			return s.unsigned(ctx, delivery)
		}

		return err
	}

	if !hook.Enabled {
		return s.deliveries.Settle(ctx, delivery.ID, entity.WebhookDeliveryFailed, time.Now().UTC())
	}

	response := s.sender.Send(ctx, s.request(hook, delivery))

	if err := s.deliveries.RecordAttempt(ctx, entity.WebhookAttempt{
		DeliveryID:      delivery.ID,
		Attempt:         delivery.Attempt,
		RequestURL:      hook.URL,
		ResolvedAddress: response.ResolvedAddress,
		Outcome:         response.Outcome,
		StatusCode:      response.StatusCode,
		ResponseExcerpt: response.Excerpt,
		Error:           response.Error,
		StartedAt:       response.StartedAt,
		FinishedAt:      response.FinishedAt,
	}); err != nil {
		return err
	}

	if response.Outcome == entity.WebhookAttemptSucceeded {
		if err := s.deliveries.Settle(
			ctx, delivery.ID, entity.WebhookDeliverySucceeded, time.Now().UTC(),
		); err != nil {
			return err
		}

		return s.webhooks.RecordSuccess(ctx, hook.ID, time.Now().UTC())
	}

	return s.retry(ctx, hook, delivery)
}

func (s *deliverer) request(hook entity.Webhook, delivery entity.WebhookDelivery) entity.WebhookRequest {
	now := time.Now().UTC()

	envelope, err := json.Marshal(service.WebhookEnvelope{
		ID:          delivery.ID.String(),
		Event:       string(delivery.Event),
		OccurredAt:  delivery.CreatedAt.UTC().Format(time.RFC3339),
		WorkspaceID: delivery.WorkspaceID.String(),
		Data:        json.RawMessage(delivery.Body),
	})
	if err != nil {
		envelope = delivery.Body
	}

	signatures := ""

	for _, secret := range hook.SigningSecrets(now) {
		if signatures != "" {
			signatures += " "
		}

		signatures += entity.SignWebhook(secret, now, delivery.ID, envelope)
	}

	return entity.WebhookRequest{
		URL:  hook.URL,
		Body: envelope,
		Headers: map[string]string{
			"Content-Type":   "application/json",
			"User-Agent":     "Norn/" + s.app.Version,
			"Norn-Event":     string(delivery.Event),
			"Norn-Webhook":   hook.ID.String(),
			"Norn-Delivery":  delivery.ID.String(),
			"Norn-Attempt":   strconv.Itoa(delivery.Attempt),
			"Norn-Timestamp": strconv.FormatInt(now.Unix(), 10),
			"Norn-Signature": signatures,
		},
	}
}

func (s *deliverer) unsigned(ctx context.Context, delivery entity.WebhookDelivery) error {
	now := time.Now().UTC()

	if err := s.deliveries.RecordAttempt(ctx, entity.WebhookAttempt{
		DeliveryID: delivery.ID,
		Attempt:    delivery.Attempt,
		Outcome:    entity.WebhookAttemptRefused,
		Error:      entity.ErrWebhookEncryptionKeyMissing.Error(),
		StartedAt:  now,
		FinishedAt: now,
	}); err != nil {
		return err
	}

	delay, again := entity.WebhookRetryDelay(delivery.Attempt)
	if !again {
		return s.deliveries.Settle(ctx, delivery.ID, entity.WebhookDeliveryFailed, now)
	}

	return s.schedule(ctx, delivery, now.Add(delay))
}

func (s *deliverer) retry(
	ctx context.Context,
	hook entity.Webhook,
	delivery entity.WebhookDelivery,
) error {
	now := time.Now().UTC()

	delay, again := entity.WebhookRetryDelay(delivery.Attempt)
	if again {
		return s.schedule(ctx, delivery, now.Add(jittered(delay)))
	}

	if err := s.deliveries.Settle(ctx, delivery.ID, entity.WebhookDeliveryFailed, now); err != nil {
		return err
	}

	streak, err := s.webhooks.RecordFailure(ctx, hook.ID)
	if err != nil {
		return err
	}

	if streak < entity.WebhookFailureLimit {
		return nil
	}

	disabled, err := s.webhooks.Disable(ctx, hook.ID, entity.WebhookDisabledSustainedFault, now)
	if err != nil {
		return err
	}

	if !disabled {
		return nil
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  hook.WorkspaceID,
		Action:       entity.AuditWebhookDisabled,
		Actor:        entity.AuditActor{Kind: entity.ActorKindSystem},
		ResourceKind: "webhook",
		ResourceID:   hook.ID,
		ResourceName: hook.Name,
		Detail:       map[string]string{"reason": string(entity.WebhookDisabledSustainedFault)},
	})

	s.warn(ctx, hook, delivery, streak)

	return nil
}

func (s *deliverer) schedule(ctx context.Context, delivery entity.WebhookDelivery, at time.Time) error {
	if err := s.deliveries.Reschedule(ctx, delivery.ID, at); err != nil {
		return err
	}

	if err := s.jobs.EnqueueWebhookDeliver(ctx, entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    delivery.Attempt,
	}, at); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"queueing a webhook retry failed, so the scheduled rescue will carry it",
			"delivery_id", delivery.ID.String(),
			"error", err.Error(),
		)
	}

	return nil
}

func (s *deliverer) Rescue(ctx context.Context) (int, error) {
	due, err := s.deliveries.ListDue(ctx, time.Now().UTC(), s.cfg.FanOutBatch)
	if err != nil {
		return 0, err
	}

	rescued := 0

	for _, delivery := range due {
		if err := s.jobs.EnqueueWebhookDeliver(ctx, entity.WebhookDeliverPayload{
			DeliveryID: delivery.ID,
			Attempt:    delivery.Attempt,
		}, time.Now().UTC()); err != nil {
			return rescued, err
		}

		rescued++
	}

	return rescued, nil
}

func jittered(delay time.Duration) time.Duration {
	spread := float64(delay) * jitterFraction

	return delay + time.Duration((rand.Float64()*2-1)*spread)
}
