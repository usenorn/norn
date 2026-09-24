package aiprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	aimodelrepo "github.com/usenorn/norn/internal/repository/aimodel"
	aiproviderrepo "github.com/usenorn/norn/internal/repository/aiprovider"
	"github.com/usenorn/norn/internal/service"
	aiprovidersvc "github.com/usenorn/norn/internal/service/aiprovider"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
)

type stored struct {
	connection entity.AIProviderConnection
	apiKey     string
}

type probe struct {
	endpoint entity.AIProviderEndpoint
	apiKey   string
}

type harness struct {
	connections *aiproviderrepo.MockAIProvider
	models      *aimodelrepo.MockAIModel
	authorizer  *authorizersvc.MockAuthorizer
	audit       *auditsvc.MockAudit
	service     service.AIProviders

	roles   map[uuid.UUID]entity.MembershipRole
	store   map[uuid.UUID]stored
	offered map[string][]string
	listed  []entity.AIProviderEndpoint
	probed  []probe
	audited []entity.AuditEntry
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		connections: aiproviderrepo.NewMockAIProvider(ctrl),
		models:      aimodelrepo.NewMockAIModel(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		roles:       map[uuid.UUID]entity.MembershipRole{},
		store:       map[uuid.UUID]stored{},
		offered:     map[string][]string{},
	}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			role, member := h.roles[request.WorkspaceID]
			if !member {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{Role: role}, nil
		}).
		AnyTimes()

	h.audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) { h.audited = append(h.audited, entry) }).
		AnyTimes()

	h.connections.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error) {
			row, ok := h.store[workspaceID]
			if !ok {
				return entity.AIProviderConnection{}, entity.ErrAIProviderNotConfigured
			}

			return row.connection, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		APIKey(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) (string, error) {
			row, ok := h.store[workspaceID]
			if !ok {
				return "", entity.ErrAIProviderNotConfigured
			}

			return row.apiKey, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, connection entity.AIProviderConnection, apiKey string,
		) (entity.AIProviderConnection, error) {
			connection.KeyHint = entity.AIProviderKeyHint(apiKey)
			h.store[connection.WorkspaceID] = stored{connection: connection, apiKey: apiKey}

			return connection, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, connection entity.AIProviderConnection,
		) (entity.AIProviderConnection, error) {
			row := h.store[connection.WorkspaceID]
			row.connection.Provider = connection.Provider
			row.connection.Endpoint = connection.Endpoint
			row.connection.DefaultModel = connection.DefaultModel
			row.connection.VerifiedAt, row.connection.FailedAt, row.connection.Failure = nil, nil, ""
			h.store[connection.WorkspaceID] = row

			return row.connection, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		MarkVerified(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID, at time.Time) error {
			row := h.store[workspaceID]
			row.connection.VerifiedAt, row.connection.FailedAt, row.connection.Failure = &at, nil, ""
			h.store[workspaceID] = row

			return nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		MarkFailed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, workspaceID uuid.UUID, failure entity.AIProviderFailure, at time.Time,
		) error {
			row := h.store[workspaceID]
			row.connection.FailedAt, row.connection.Failure = &at, failure
			h.store[workspaceID] = row

			return nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) error {
			delete(h.store, workspaceID)

			return nil
		}).
		AnyTimes()

	h.models.EXPECT().
		List(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, endpoint entity.AIProviderEndpoint, apiKey string) ([]string, error) {
			h.listed = append(h.listed, endpoint)

			models, known := h.offered[apiKey]
			if !known {
				return nil, entity.ErrAIProviderKeyRejected
			}

			return models, nil
		}).
		AnyTimes()

	h.service = aiprovidersvc.New(h.connections, h.models, h.authorizer, h.audit, config.Instance{SelfHosted: true})

	return h
}

func (h *harness) workspace(role entity.MembershipRole) uuid.UUID {
	id := uuid.New()
	h.roles[id] = role

	return id
}

func (h *harness) configured(workspaceID uuid.UUID, apiKey, model string) {
	h.offered[apiKey] = []string{model}
	h.store[workspaceID] = stored{
		connection: entity.AIProviderConnection{
			WorkspaceID:  workspaceID,
			Provider:     entity.AIProviderOpenAI,
			KeyHint:      entity.AIProviderKeyHint(apiKey),
			DefaultModel: model,
		},
		apiKey: apiKey,
	}
}

func (h *harness) probesAnswer(err error) {
	h.models.EXPECT().
		Probe(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, endpoint entity.AIProviderEndpoint, apiKey, _ string) error {
			h.probed = append(h.probed, probe{endpoint: endpoint, apiKey: apiKey})

			return err
		}).
		AnyTimes()
}

func (h *harness) actions() []entity.AuditAction {
	actions := make([]entity.AuditAction, len(h.audited))
	for i, entry := range h.audited {
		actions[i] = entry.Action
	}

	return actions
}
