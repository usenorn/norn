package webhook_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) probesDestinations(refusal error) *[]string {
	probed := new([]string)

	h.sender.EXPECT().
		Check(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, url string) error {
			*probed = append(*probed, url)

			return refusal
		}).
		AnyTimes()

	return probed
}

func (h *harness) storesWebhooks() *[]entity.Webhook {
	stored := new([]entity.Webhook)

	h.webhooks.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hook entity.Webhook) (entity.Webhook, error) {
			*stored = append(*stored, hook)

			hook.ID = uuid.New()
			hook.SecretHint = entity.WebhookSecretHint(hook.Secret)
			hook.Secret = ""
			hook.Enabled = true

			return hook, nil
		}).
		AnyTimes()

	return stored
}

func registration(workspaceID uuid.UUID) service.RegisterWebhookInput {
	return service.RegisterWebhookInput{
		WorkspaceID: workspaceID,
		Name:        "Northwind relay",
		URL:         "https://hooks.northwind.co/norn",
		Events:      []entity.WebhookEvent{entity.WebhookIssueCreated, entity.WebhookIssueUpdated},
	}
}

func TestOnlyAWorkspaceAdministratorMayRegisterAWebhook(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.MembershipRoleMember)

	probed := h.probesDestinations(nil)
	stored := h.storesWebhooks()

	_, err := h.registry.Register(ctx, registration(workspaceID))

	if !errors.Is(err, entity.ErrWebhookNotPermitted) {
		t.Fatalf(
			"Register error = %v, want ErrWebhookNotPermitted. A webhook is a standing export of "+
				"everything its owner can see, so registering one is an administrative act.",
			err,
		)
	}

	if len(*stored) != 0 {
		t.Fatal("a subscription was created for somebody who was refused")
	}

	if len(*probed) != 0 {
		t.Error("the destination was probed on behalf of a caller who may not register one, which is a request they could not otherwise make")
	}
}

func TestADestinationTheInstanceRefusesIsNeverRegistered(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.probesDestinations(entity.ErrWebhookDestinationRefused)

	stored := h.storesWebhooks()

	_, err := h.registry.Register(ctx, registration(workspaceID))

	if !errors.Is(err, entity.ErrWebhookDestinationRefused) {
		t.Fatalf(
			"Register error = %v, want ErrWebhookDestinationRefused. The guard that keeps a "+
				"webhook from being pointed at the instance's own network is only worth having if "+
				"its refusal stops the registration.",
			err,
		)
	}

	if len(*stored) != 0 {
		t.Fatal("a subscription to a refused destination was stored anyway, so the guard would have to be re-run on every delivery")
	}
}

func TestTheSecretIsHandedOverOnceAndNeverReadBackAfterwards(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.probesDestinations(nil)

	stored := h.storesWebhooks()

	created, err := h.registry.Register(ctx, registration(workspaceID))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if created.Secret == "" {
		t.Fatal(
			"registration returned no secret. It is generated here and never retrievable again, " +
				"so a caller that does not receive it now can never verify a signature.",
		)
	}

	if !strings.HasPrefix(created.Secret, entity.WebhookSecretPrefix) {
		t.Errorf("the returned secret %q carries no %q prefix, so a leak is not greppable", created.Secret, entity.WebhookSecretPrefix)
	}

	if len(*stored) != 1 {
		t.Fatalf("registration stored %d subscriptions, want one", len(*stored))
	}

	if (*stored)[0].Secret != created.Secret {
		t.Fatalf(
			"the secret handed to the caller is not the secret that was stored, so every signature " +
				"the receiver checks would fail",
		)
	}

	h.webhooks.EXPECT().
		Get(gomock.Any(), workspaceID, created.ID).
		Return(entity.Webhook{ID: created.ID, WorkspaceID: workspaceID, SecretHint: created.SecretHint}, nil)

	fetched, err := h.registry.Get(ctx, workspaceID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if fetched.Secret != "" {
		t.Fatalf(
			"reading the webhook back returned the secret %q. Anybody who reaches the registry "+
				"once could then take over signing, which is exactly what showing it only at "+
				"creation is meant to prevent.",
			fetched.Secret,
		)
	}
}

func TestRegisteringOnAnInstanceWithNoEncryptionKeySaysSo(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.probesDestinations(nil)

	h.webhooks.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.Webhook{}, entity.ErrWebhookEncryptionKeyMissing)

	_, err := h.registry.Register(ctx, registration(workspaceID))

	if !errors.Is(err, entity.ErrWebhookEncryptionKeyMissing) {
		t.Fatalf(
			"Register error = %v, want ErrWebhookEncryptionKeyMissing. An operator who has not "+
				"configured a key needs to be told that, not handed a subscription that can never "+
				"sign anything.",
			err,
		)
	}
}

func TestAMalformedSubscriptionIsRefusedBeforeTheDestinationIsProbed(t *testing.T) {
	for name, mutate := range map[string]func(*service.RegisterWebhookInput){
		"a destination that is not an http url": func(input *service.RegisterWebhookInput) {
			input.URL = "ftp://hooks.northwind.co/norn"
		},
		"no events at all": func(input *service.RegisterWebhookInput) {
			input.Events = nil
		},
		"an event this instance does not emit": func(input *service.RegisterWebhookInput) {
			input.Events = []entity.WebhookEvent{"issue.exploded"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspaceID := uuid.New()
			ctx := h.actingAs(entity.MembershipRoleAdmin)

			probed := h.probesDestinations(nil)
			stored := h.storesWebhooks()

			input := registration(workspaceID)
			mutate(&input)

			var invalid entity.ValidationError

			_, err := h.registry.Register(ctx, input)

			if !errors.As(err, &invalid) {
				t.Fatalf(
					"Register error = %v, want a validation error naming the field. The caller has "+
						"to be told which part of the subscription is wrong, not handed a generic "+
						"failure.",
					err,
				)
			}

			if len(*probed) != 0 {
				t.Errorf(
					"the destination was probed for an invalid subscription. The probe opens an " +
						"outbound connection chosen by the caller, so it must sit behind " +
						"validation rather than in front of it.",
				)
			}

			if len(*stored) != 0 {
				t.Error("an invalid subscription was stored")
			}
		})
	}
}
