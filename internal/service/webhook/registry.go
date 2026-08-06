package webhook

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type registry struct {
	webhooks   repository.Webhook
	sender     repository.WebhookSender
	authorizer service.Authorizer
	audit      service.Audit
	transactor repository.Transactor
	cfg        config.Webhooks
}

func NewRegistry(
	webhooks repository.Webhook,
	sender repository.WebhookSender,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	cfg config.Webhooks,
) service.Webhooks {
	return &registry{
		webhooks:   webhooks,
		sender:     sender,
		authorizer: authorizer,
		audit:      audit,
		transactor: transactor,
		cfg:        cfg,
	}
}

func (s *registry) administers(ctx context.Context, workspaceID uuid.UUID) (entity.Decision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.Decision{}, err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.Decision{}, entity.ErrWebhookNotPermitted
	}

	return decision, nil
}

func (s *registry) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.Webhook, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.webhooks.ListByWorkspace(ctx, workspaceID)
}

func (s *registry) Get(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.Webhook, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.Webhook{}, err
	}

	return s.webhooks.Get(ctx, workspaceID, webhookID)
}

func (s *registry) Register(
	ctx context.Context,
	input service.RegisterWebhookInput,
) (entity.Webhook, error) {
	decision, err := s.administers(ctx, input.WorkspaceID)
	if err != nil {
		return entity.Webhook{}, err
	}

	events := entity.NormalizeWebhookEvents(input.Events)

	if err := entity.NewValidationError(
		entity.ValidateWebhookName("name", input.Name),
		entity.ValidateWebhookURL("url", input.URL),
		entity.ValidateWebhookEvents("events", events),
	); err != nil {
		return entity.Webhook{}, err
	}

	destination := strings.TrimSpace(input.URL)

	if err := s.sender.Check(ctx, destination); err != nil {
		return entity.Webhook{}, err
	}

	secret, err := entity.NewWebhookSecret()
	if err != nil {
		return entity.Webhook{}, err
	}

	var created entity.Webhook

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		stored, err := s.webhooks.Create(ctx, entity.Webhook{
			WorkspaceID:    input.WorkspaceID,
			OwnerAccountID: decision.Actor.Authority(),
			Name:           strings.TrimSpace(input.Name),
			URL:            destination,
			Events:         events,
			Secret:         secret,
		})
		if err != nil {
			return err
		}

		created = stored

		return nil
	}); err != nil {
		return entity.Webhook{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  input.WorkspaceID,
		Action:       entity.AuditWebhookRegistered,
		ResourceKind: "webhook",
		ResourceID:   created.ID,
		ResourceName: created.Name,
		Detail:       map[string]string{"url": created.URL},
	})

	created.Secret = secret

	return created, nil
}

func (s *registry) Update(
	ctx context.Context,
	workspaceID, webhookID uuid.UUID,
	input service.UpdateWebhookInput,
) (entity.Webhook, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.Webhook{}, err
	}

	hook, err := s.webhooks.Get(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.Webhook{}, err
	}

	if input.Name != nil {
		hook.Name = strings.TrimSpace(*input.Name)
	}

	if input.URL != nil {
		hook.URL = strings.TrimSpace(*input.URL)
	}

	if len(input.Events) > 0 {
		hook.Events = entity.NormalizeWebhookEvents(input.Events)
	}

	if err := entity.NewValidationError(
		entity.ValidateWebhookName("name", hook.Name),
		entity.ValidateWebhookURL("url", hook.URL),
		entity.ValidateWebhookEvents("events", hook.Events),
	); err != nil {
		return entity.Webhook{}, err
	}

	if input.URL != nil {
		if err := s.sender.Check(ctx, hook.URL); err != nil {
			return entity.Webhook{}, err
		}
	}

	updated, err := s.webhooks.Update(ctx, hook)
	if err != nil {
		return entity.Webhook{}, err
	}

	if input.Enabled == nil {
		return updated, nil
	}

	if *input.Enabled {
		if err := s.webhooks.Enable(ctx, webhookID); err != nil {
			return entity.Webhook{}, err
		}
	} else if _, err := s.webhooks.Disable(
		ctx, webhookID, entity.WebhookDisabledByOwner, time.Now().UTC(),
	); err != nil {
		return entity.Webhook{}, err
	}

	return s.webhooks.Get(ctx, workspaceID, webhookID)
}

func (s *registry) RotateSecret(
	ctx context.Context,
	workspaceID, webhookID uuid.UUID,
) (entity.Webhook, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.Webhook{}, err
	}

	hook, err := s.webhooks.Get(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.Webhook{}, err
	}

	secret, err := entity.NewWebhookSecret()
	if err != nil {
		return entity.Webhook{}, err
	}

	if err := s.webhooks.RotateSecret(
		ctx, hook.ID, secret, time.Now().UTC().Add(s.cfg.SecretGrace),
	); err != nil {
		return entity.Webhook{}, err
	}

	rotated, err := s.webhooks.Get(ctx, workspaceID, webhookID)
	if err != nil {
		return entity.Webhook{}, err
	}

	rotated.Secret = secret

	return rotated, nil
}

func (s *registry) Remove(ctx context.Context, workspaceID, webhookID uuid.UUID) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	hook, err := s.webhooks.Get(ctx, workspaceID, webhookID)
	if err != nil {
		return err
	}

	if err := s.webhooks.Delete(ctx, workspaceID, webhookID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditWebhookRemoved,
		ResourceKind: "webhook",
		ResourceID:   hook.ID,
		ResourceName: hook.Name,
	})

	return nil
}
