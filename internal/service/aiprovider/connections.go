package aiprovider

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const auditResourceKind = "ai_provider"

type providers struct {
	connections repository.AIProvider
	models      repository.AIModel
	authorizer  service.Authorizer
	audit       service.Audit
	instance    config.Instance
}

func New(
	connections repository.AIProvider,
	models repository.AIModel,
	authorizer service.Authorizer,
	audit service.Audit,
	instance config.Instance,
) service.AIProviders {
	return &providers{
		connections: connections,
		models:      models,
		authorizer:  authorizer,
		audit:       audit,
		instance:    instance,
	}
}

func (s *providers) administers(ctx context.Context, workspaceID uuid.UUID) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.ErrAccountForbidden
	}

	return nil
}

func (s *providers) Get(ctx context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error) {
	if err := s.administers(ctx, workspaceID); err != nil {
		return entity.AIProviderConnection{}, err
	}

	return s.connections.Get(ctx, workspaceID)
}

func (s *providers) Models(ctx context.Context, workspaceID uuid.UUID) ([]string, error) {
	if err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	connection, err := s.connections.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	apiKey, err := s.connections.APIKey(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return s.models.List(ctx, connection.Endpoint, apiKey)
}

func (s *providers) Configure(
	ctx context.Context,
	workspaceID uuid.UUID,
	input entity.AIProviderInput,
) (entity.AIProviderConnection, error) {
	if err := s.administers(ctx, workspaceID); err != nil {
		return entity.AIProviderConnection{}, err
	}

	input = input.Normalized()

	current, err := s.connections.Get(ctx, workspaceID)
	if err != nil && !errors.Is(err, entity.ErrAIProviderNotConfigured) {
		return entity.AIProviderConnection{}, err
	}

	configured := err == nil

	if err := input.Validate(configured); err != nil {
		return entity.AIProviderConnection{}, err
	}

	if input.Endpoint.AllowPrivateAddress && !s.instance.SelfHosted {
		return entity.AIProviderConnection{}, entity.AIProviderPrivateAddressRefused()
	}

	keyChanged := input.APIKey != ""
	modelChanged := input.DefaultModel != current.DefaultModel
	endpointChanged := input.Endpoint != current.Endpoint

	if configured && !keyChanged && !modelChanged && !endpointChanged {
		return current, nil
	}

	if endpointChanged && !keyChanged {
		return entity.AIProviderConnection{}, entity.AIProviderKeyRequired()
	}

	apiKey := input.APIKey
	if !keyChanged {
		if apiKey, err = s.connections.APIKey(ctx, workspaceID); err != nil {
			return entity.AIProviderConnection{}, err
		}
	}

	if keyChanged || endpointChanged || input.DefaultModel != "" {
		models, err := s.models.List(ctx, input.Endpoint, apiKey)
		if err != nil {
			return entity.AIProviderConnection{}, err
		}

		if input.DefaultModel != "" && !slices.Contains(models, input.DefaultModel) {
			return entity.AIProviderConnection{}, entity.ErrAIProviderModelUnavailable
		}
	}

	connection := entity.AIProviderConnection{
		WorkspaceID:  workspaceID,
		Provider:     input.Provider,
		Endpoint:     input.Endpoint,
		DefaultModel: input.DefaultModel,
	}

	var saved entity.AIProviderConnection

	if keyChanged {
		saved, err = s.connections.Save(ctx, connection, input.APIKey)
	} else {
		saved, err = s.connections.Update(ctx, connection)
	}

	if err != nil {
		return entity.AIProviderConnection{}, err
	}

	if !configured {
		s.record(ctx, saved, entity.AuditAIProviderConfigured)

		return saved, nil
	}

	for _, change := range []struct {
		changed bool
		action  entity.AuditAction
	}{
		{keyChanged, entity.AuditAIProviderKeyReplaced},
		{endpointChanged, entity.AuditAIProviderEndpointChanged},
		{modelChanged, entity.AuditAIProviderModelChanged},
	} {
		if change.changed {
			s.record(ctx, saved, change.action)
		}
	}

	return saved, nil
}

func (s *providers) Test(ctx context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error) {
	if err := s.administers(ctx, workspaceID); err != nil {
		return entity.AIProviderConnection{}, err
	}

	connection, err := s.connections.Get(ctx, workspaceID)
	if err != nil {
		return entity.AIProviderConnection{}, err
	}

	if connection.DefaultModel == "" {
		return entity.AIProviderConnection{}, entity.AIProviderModelRequired()
	}

	apiKey, err := s.connections.APIKey(ctx, workspaceID)
	if err != nil {
		return entity.AIProviderConnection{}, err
	}

	if probed := s.models.Probe(ctx, connection.Endpoint, apiKey, connection.DefaultModel); probed != nil {
		failure, verdict := entity.AIProviderFailureOf(probed)
		if !verdict {
			return entity.AIProviderConnection{}, probed
		}

		if err := s.connections.MarkFailed(ctx, workspaceID, failure, time.Now().UTC()); err != nil {
			return entity.AIProviderConnection{}, err
		}

		return entity.AIProviderConnection{}, probed
	}

	if err := s.connections.MarkVerified(ctx, workspaceID, time.Now().UTC()); err != nil {
		return entity.AIProviderConnection{}, err
	}

	return s.connections.Get(ctx, workspaceID)
}

func (s *providers) Remove(ctx context.Context, workspaceID uuid.UUID) error {
	if err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	connection, err := s.connections.Get(ctx, workspaceID)
	if err != nil {
		return err
	}

	if err := s.connections.Delete(ctx, workspaceID); err != nil {
		return err
	}

	s.record(ctx, connection, entity.AuditAIProviderRemoved)

	return nil
}

func (s *providers) record(
	ctx context.Context,
	connection entity.AIProviderConnection,
	action entity.AuditAction,
) {
	detail := map[string]string{}
	if connection.DefaultModel != "" {
		detail["model"] = connection.DefaultModel
	}

	if connection.Endpoint.BaseURL != "" {
		detail["endpoint"] = connection.Endpoint.BaseURL
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       action,
		ResourceKind: auditResourceKind,
		ResourceID:   connection.WorkspaceID,
		ResourceName: connection.Provider.Label(),
		Detail:       detail,
	})
}
