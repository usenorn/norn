package webhook_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	webhookrepo "github.com/usenorn/norn/internal/repository/webhook"
	webhooksenderrepo "github.com/usenorn/norn/internal/repository/webhooksender"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

const (
	ownerEmail    = "rae@northwind.co"
	workspaceName = "Northwind"
	workspaceSlug = "northwind"
	baseURL       = "https://norn.test"
	fanOutBatch   = 25
	sweepBatch    = 200
)

type wakeUp struct {
	payload entity.WebhookDeliverPayload
	at      time.Time
}

type grant struct {
	role     entity.MembershipRole
	allTeams bool
	teams    []uuid.UUID
}

type harness struct {
	webhooks    *webhookrepo.MockWebhook
	outbox      *webhookrepo.MockWebhookOutbox
	deliveries  *webhookrepo.MockWebhookDelivery
	retention   *webhookrepo.MockWebhookRetention
	sender      *webhooksenderrepo.MockWebhookSender
	memberships *membershiprepo.MockMembership
	accounts    *accountrepo.MockAccount
	workspaces  *workspacerepo.MockWorkspace
	mailer      *mailerrepo.MockMailer
	jobs        *jobqueuerepo.MockJobProducer
	authorizer  *authorizersvc.MockAuthorizer
	audit       *auditsvc.MockAudit

	deliverer service.WebhookDispatch
	fanOut    service.WebhookFanOut
	registry  service.Webhooks
	log       service.WebhookDeliveries
	sweeper   service.WebhookRetention

	accountID uuid.UUID
	woken     []wakeUp
	mailed    []entity.Mail
	audited   []entity.AuditEntry
}

func webhookConfig() config.Webhooks {
	return config.Webhooks{
		FanOutBatch: fanOutBatch,
		SweepBatch:  sweepBatch,
		SecretGrace: 24 * time.Hour,
		Retention:   30 * 24 * time.Hour,
	}
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		webhooks:    webhookrepo.NewMockWebhook(ctrl),
		outbox:      webhookrepo.NewMockWebhookOutbox(ctrl),
		deliveries:  webhookrepo.NewMockWebhookDelivery(ctrl),
		retention:   webhookrepo.NewMockWebhookRetention(ctrl),
		sender:      webhooksenderrepo.NewMockWebhookSender(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		workspaces:  workspacerepo.NewMockWorkspace(ctrl),
		mailer:      mailerrepo.NewMockMailer(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		accountID:   uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.jobs.EXPECT().
		EnqueueWebhookDeliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.WebhookDeliverPayload, at time.Time) error {
			h.woken = append(h.woken, wakeUp{payload: payload, at: at})

			return nil
		}).
		AnyTimes()

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			h.mailed = append(h.mailed, mail)

			return nil
		}).
		AnyTimes()

	h.audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) {
			h.audited = append(h.audited, entry)
		}).
		AnyTimes()

	h.accounts.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, accountID uuid.UUID) (entity.Account, error) {
			return entity.Account{ID: accountID, Email: ownerEmail, Status: entity.AccountStatusActive}, nil
		}).
		AnyTimes()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) (entity.Workspace, error) {
			return entity.Workspace{ID: workspaceID, Name: workspaceName, Slug: workspaceSlug}, nil
		}).
		AnyTimes()

	cfg := webhookConfig()

	h.deliverer = webhooksvc.NewDeliverer(
		h.webhooks, h.deliveries, h.sender, h.accounts, h.workspaces,
		h.mailer, h.jobs, h.audit, config.App{Version: "1.2.3", BaseURL: baseURL}, cfg,
	)
	h.fanOut = webhooksvc.NewFanOut(
		h.outbox, h.webhooks, h.deliveries, h.memberships, h.jobs, h.authorizer, transactor, cfg,
	)
	h.registry = webhooksvc.NewRegistry(h.webhooks, h.sender, h.authorizer, h.audit, transactor, cfg)
	h.log = webhooksvc.NewLog(h.webhooks, h.deliveries, h.jobs, h.authorizer, transactor)
	h.sweeper = webhooksvc.NewSweeper(h.retention, cfg)

	return h
}

func (h *harness) actingAs(role entity.MembershipRole) context.Context {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
				Role:  role,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	return identity.Into(context.Background(), h.accountID)
}

func (h *harness) ownersHold(access map[uuid.UUID]grant) context.Context {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, request entity.AccessRequest) (entity.Decision, error) {
			actor, ok := identity.Actor(ctx)
			if !ok {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			granted, ok := access[actor.Authority()]
			if !ok {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{
				Actor: actor,
				Role:  granted.role,
				Scope: entity.TeamScope{
					WorkspaceID: request.WorkspaceID,
					AllTeams:    granted.allTeams,
					TeamIDs:     granted.teams,
				},
			}, nil
		}).
		AnyTimes()

	return context.Background()
}

func (h *harness) membersEverywhere() {
	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID, accountID uuid.UUID) (entity.Membership, error) {
			return entity.Membership{
				ID:          uuid.New(),
				WorkspaceID: workspaceID,
				AccountID:   accountID,
				Role:        entity.MembershipRoleMember,
			}, nil
		}).
		AnyTimes()
}

func (h *harness) subscriptions(hooks ...entity.Webhook) {
	h.webhooks.EXPECT().
		ListSubscribed(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, workspaceID uuid.UUID, event entity.WebhookEvent,
		) ([]entity.Webhook, error) {
			subscribed := make([]entity.Webhook, 0, len(hooks))

			for _, hook := range hooks {
				if hook.WorkspaceID == workspaceID && hook.Subscribes(event) {
					subscribed = append(subscribed, hook)
				}
			}

			return subscribed, nil
		}).
		AnyTimes()
}

func (h *harness) capturesQueue() *[]entity.WebhookDelivery {
	queued := new([]entity.WebhookDelivery)

	h.deliveries.EXPECT().
		Queue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, batch []entity.WebhookDelivery) error {
			*queued = append(*queued, batch...)

			return nil
		}).
		AnyTimes()

	return queued
}

func (h *harness) singleEvent(entry entity.WebhookOutboxEntry) {
	h.outbox.EXPECT().
		ClaimPending(gomock.Any(), fanOutBatch).
		Return([]entity.WebhookOutboxEntry{entry}, nil)
}

func enabledWebhook(workspaceID, ownerID uuid.UUID, events ...entity.WebhookEvent) entity.Webhook {
	return entity.Webhook{
		ID:             uuid.New(),
		WorkspaceID:    workspaceID,
		OwnerAccountID: ownerID,
		Name:           "Northwind relay",
		URL:            "https://hooks.northwind.co/norn",
		Events:         events,
		Secret:         "nrnwhs_current",
		Enabled:        true,
		CreatedAt:      time.Now().UTC(),
	}
}

func outboxEntry(workspaceID, teamID uuid.UUID, event entity.WebhookEvent) entity.WebhookOutboxEntry {
	return entity.WebhookOutboxEntry{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Event:       event,
		SubjectKind: "issue",
		SubjectID:   uuid.New(),
		TeamID:      teamID,
		Body:        []byte(`{"identifier":"NOR-14","title":"Ship the relay"}`),
		OccurredAt:  time.Now().UTC(),
	}
}

func claimedDelivery(hook entity.Webhook, attempt int) entity.WebhookDelivery {
	return entity.WebhookDelivery{
		ID:          uuid.New(),
		WebhookID:   hook.ID,
		WorkspaceID: hook.WorkspaceID,
		OutboxID:    uuid.New(),
		Event:       entity.WebhookIssueCreated,
		SubjectKind: "issue",
		SubjectID:   uuid.New(),
		Body:        []byte(`{"identifier":"NOR-14"}`),
		State:       entity.WebhookDeliveryPending,
		Attempt:     attempt,
		CreatedAt:   time.Now().UTC(),
	}
}

func refusedResponse() entity.WebhookResponse {
	now := time.Now().UTC()

	return entity.WebhookResponse{
		Outcome:    entity.WebhookAttemptRejected,
		StatusCode: 500,
		Excerpt:    "internal server error",
		StartedAt:  now,
		FinishedAt: now,
	}
}

func acceptedResponse() entity.WebhookResponse {
	now := time.Now().UTC()

	return entity.WebhookResponse{
		Outcome:    entity.WebhookAttemptSucceeded,
		StatusCode: 200,
		StartedAt:  now,
		FinishedAt: now,
	}
}
