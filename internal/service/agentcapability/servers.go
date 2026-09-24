package agentcapability

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *capabilities) SearchRegistry(
	ctx context.Context,
	workspaceID uuid.UUID,
	query string,
) ([]entity.AgentMCPRegistryEntry, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return nil, err
	}

	return s.registry.Search(ctx, query)
}

func (s *capabilities) CreateMCPServer(
	ctx context.Context,
	owner service.CapabilityOwner,
	input entity.AgentMCPServerInput,
) (service.AgentMCPServerView, error) {
	decision, err := s.owns(ctx, owner)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	input = input.Normalized()

	if err := input.Validate(); err != nil {
		return service.AgentMCPServerView{}, err
	}

	secrets, err := merged(input, entity.AgentMCPSecrets{})
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	owned, err := s.servers.CountOwned(ctx, owner.WorkspaceID, owner.AgentID)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	if owned >= entity.AgentMCPServersPerOwner {
		return service.AgentMCPServerView{}, entity.ErrAgentMCPServerLimitReached
	}

	if err := s.serverNameFree(ctx, owner, input.Name, uuid.Nil); err != nil {
		return service.AgentMCPServerView{}, err
	}

	created, err := s.servers.Create(ctx, server(input, uuid.New(), owner, decision.Actor.AccountID), secrets)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	s.record(ctx, created.WorkspaceID, entity.AuditAgentMCPServerAdded, auditServerKind, created.ID, created.Name, created.AgentID)

	return service.AgentMCPServerView{Server: created}, nil
}

func server(
	input entity.AgentMCPServerInput,
	id uuid.UUID,
	owner service.CapabilityOwner,
	createdBy uuid.UUID,
) entity.AgentMCPServer {
	return entity.AgentMCPServer{
		ID:            id,
		WorkspaceID:   owner.WorkspaceID,
		AgentID:       owner.AgentID,
		Name:          input.Name,
		Transport:     input.Transport,
		Command:       input.Command,
		Args:          input.Args,
		URL:           input.URL,
		Auth:          input.Auth,
		OAuthClientID: input.OAuthClientID,
		Registry:      input.Registry,
		CreatedBy:     createdBy,
	}
}

func merged(input entity.AgentMCPServerInput, stored entity.AgentMCPSecrets) (entity.AgentMCPSecrets, error) {
	env, err := mergedValues("env", input.Env, stored.Env)
	if err != nil {
		return entity.AgentMCPSecrets{}, err
	}

	headers, err := mergedValues("headers", input.Headers, stored.Headers)
	if err != nil {
		return entity.AgentMCPSecrets{}, err
	}

	clientSecret := input.OAuthClientSecret
	if clientSecret == "" && input.OAuthClientID != "" {
		clientSecret = stored.OAuthClientSecret
	}

	return entity.AgentMCPSecrets{Env: env, Headers: headers, OAuthClientSecret: clientSecret}, nil
}

func mergedValues(field string, given, stored map[string]string) (map[string]string, error) {
	if len(given) == 0 {
		return nil, nil
	}

	values := make(map[string]string, len(given))

	for key, value := range given {
		if value == "" {
			kept, ok := stored[key]
			if !ok {
				return nil, entity.AgentMCPSecretRequired(field)
			}

			value = kept
		}

		values[key] = value
	}

	return values, nil
}

func (s *capabilities) serverNameFree(
	ctx context.Context,
	owner service.CapabilityOwner,
	name string,
	except uuid.UUID,
) error {
	if owner.AgentID == nil {
		return nil
	}

	existing, err := s.servers.ListByAgent(ctx, owner.WorkspaceID, *owner.AgentID)
	if err != nil {
		return err
	}

	for _, server := range existing {
		if server.ID != except && strings.EqualFold(server.Name, name) {
			return entity.ErrAgentMCPServerNameTaken
		}
	}

	return nil
}

func (s *capabilities) editableServer(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (entity.AgentMCPServer, entity.Decision, error) {
	server, err := s.servers.Get(ctx, workspaceID, serverID)
	if err != nil {
		return entity.AgentMCPServer{}, entity.Decision{}, err
	}

	decision, err := s.owns(ctx, service.CapabilityOwner{WorkspaceID: workspaceID, AgentID: server.AgentID})
	if err != nil {
		return entity.AgentMCPServer{}, entity.Decision{}, err
	}

	return server, decision, nil
}

func (s *capabilities) UpdateMCPServer(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
	input entity.AgentMCPServerInput,
) (service.AgentMCPServerView, error) {
	current, _, err := s.editableServer(ctx, workspaceID, serverID)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	input = input.Normalized()

	if err := input.Validate(); err != nil {
		return service.AgentMCPServerView{}, err
	}

	stored, err := s.servers.Secrets(ctx, workspaceID, serverID)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	secrets, err := merged(input, stored)
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	owner := service.CapabilityOwner{WorkspaceID: workspaceID, AgentID: current.AgentID}
	if err := s.serverNameFree(ctx, owner, input.Name, current.ID); err != nil {
		return service.AgentMCPServerView{}, err
	}

	next := server(input, current.ID, owner, current.CreatedBy)
	signInChanged := next.URL != current.URL || next.Transport != current.Transport ||
		next.Auth != current.Auth || next.OAuthClientID != current.OAuthClientID ||
		secrets.OAuthClientSecret != stored.OAuthClientSecret

	var updated entity.AgentMCPServer

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if signInChanged {
			if err := ignoreMissing(
				s.connections.Delete(ctx, workspaceID, serverID),
				entity.ErrAgentMCPConnectionNotFound,
			); err != nil {
				return err
			}
		}

		updated, err = s.servers.Update(ctx, next, secrets)

		return err
	}); err != nil {
		return service.AgentMCPServerView{}, err
	}

	s.record(ctx, workspaceID, entity.AuditAgentMCPServerUpdated, auditServerKind, updated.ID, updated.Name, updated.AgentID)

	return s.view(ctx, updated)
}

func (s *capabilities) DeleteMCPServer(ctx context.Context, workspaceID, serverID uuid.UUID) error {
	server, _, err := s.editableServer(ctx, workspaceID, serverID)
	if err != nil {
		return err
	}

	if err := s.servers.Delete(ctx, workspaceID, serverID); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentMCPServerRemoved, auditServerKind, server.ID, server.Name, server.AgentID)

	return nil
}

func (s *capabilities) AttachMCPServer(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error {
	decision, err := s.agent(ctx, workspaceID, agentID, entity.ActionManage)
	if err != nil {
		return err
	}

	server, err := s.servers.Get(ctx, workspaceID, serverID)
	if err != nil {
		return err
	}

	if !server.InLibrary() {
		return entity.ErrAgentCapabilityNotLibrary
	}

	owner := service.CapabilityOwner{WorkspaceID: workspaceID, AgentID: &agentID}
	if err := s.serverNameFree(ctx, owner, server.Name, server.ID); err != nil {
		return err
	}

	if err := s.servers.Attach(ctx, entity.AgentCapabilityAttachment{
		WorkspaceID:  workspaceID,
		AgentID:      agentID,
		CapabilityID: serverID,
		AttachedBy:   decision.Actor.AccountID,
	}); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentCapabilityAttached, auditServerKind, server.ID, server.Name, &agentID)

	return nil
}

func (s *capabilities) DetachMCPServer(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error {
	if _, err := s.agent(ctx, workspaceID, agentID, entity.ActionManage); err != nil {
		return err
	}

	server, err := s.servers.Get(ctx, workspaceID, serverID)
	if err != nil {
		return err
	}

	if err := s.servers.Detach(ctx, workspaceID, agentID, serverID); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentCapabilityDetached, auditServerKind, server.ID, server.Name, &agentID)

	return nil
}
