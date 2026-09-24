package agentcapability

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	auditSkillKind  = "agent_skill"
	auditServerKind = "agent_mcp_server"
)

type capabilities struct {
	agents      repository.Agent
	skills      repository.AgentSkill
	servers     repository.AgentMCPServer
	connections repository.AgentMCPConnection
	states      repository.AgentMCPOAuthState
	sources     repository.SkillSource
	registry    repository.MCPRegistry
	oauth       repository.MCPAuthorizer
	blobs       repository.Blob
	authorizer  service.Authorizer
	audit       service.Audit
	transactor  repository.Transactor
	instance    config.Instance
}

func New(
	agents repository.Agent,
	skills repository.AgentSkill,
	servers repository.AgentMCPServer,
	connections repository.AgentMCPConnection,
	states repository.AgentMCPOAuthState,
	sources repository.SkillSource,
	registry repository.MCPRegistry,
	oauth repository.MCPAuthorizer,
	blobs repository.Blob,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	instance config.Instance,
) service.AgentCapabilities {
	return &capabilities{
		agents:      agents,
		skills:      skills,
		servers:     servers,
		connections: connections,
		states:      states,
		sources:     sources,
		registry:    registry,
		oauth:       oauth,
		blobs:       blobs,
		authorizer:  authorizer,
		audit:       audit,
		transactor:  transactor,
		instance:    instance,
	}
}

func (s *capabilities) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      action,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.Decision{}, err
	}

	if action != entity.ActionRead && decision.Actor.Kind != entity.ActorKindUser {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	return decision, nil
}

func (s *capabilities) agent(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	decision, err := s.decide(ctx, workspaceID, action)
	if err != nil {
		return entity.Decision{}, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return entity.Decision{}, err
	}

	if !agent.ManageableBy(decision.Actor.Authority(), decision.Role) {
		return entity.Decision{}, entity.ErrAgentNotFound
	}

	return decision, nil
}

func (s *capabilities) librarian(ctx context.Context, workspaceID uuid.UUID) (entity.Decision, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Decision{}, err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	return decision, nil
}

func (s *capabilities) owns(ctx context.Context, owner service.CapabilityOwner) (entity.Decision, error) {
	if owner.AgentID == nil {
		return s.librarian(ctx, owner.WorkspaceID)
	}

	return s.agent(ctx, owner.WorkspaceID, *owner.AgentID, entity.ActionManage)
}

func (s *capabilities) ListForAgent(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
) (service.AgentCapabilitySet, error) {
	if _, err := s.agent(ctx, workspaceID, agentID, entity.ActionRead); err != nil {
		return service.AgentCapabilitySet{}, err
	}

	skills, err := s.skills.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return service.AgentCapabilitySet{}, err
	}

	servers, err := s.servers.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return service.AgentCapabilitySet{}, err
	}

	views, err := s.views(ctx, workspaceID, servers)
	if err != nil {
		return service.AgentCapabilitySet{}, err
	}

	return service.AgentCapabilitySet{Skills: skills, MCPServers: views}, nil
}

func (s *capabilities) ListLibrary(
	ctx context.Context,
	workspaceID uuid.UUID,
) (service.LibraryCapabilities, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionRead); err != nil {
		return service.LibraryCapabilities{}, err
	}

	skills, err := s.skills.ListLibrary(ctx, workspaceID)
	if err != nil {
		return service.LibraryCapabilities{}, err
	}

	skillAgents, err := s.skills.AttachedAgents(ctx, workspaceID)
	if err != nil {
		return service.LibraryCapabilities{}, err
	}

	servers, err := s.servers.ListLibrary(ctx, workspaceID)
	if err != nil {
		return service.LibraryCapabilities{}, err
	}

	serverAgents, err := s.servers.AttachedAgents(ctx, workspaceID)
	if err != nil {
		return service.LibraryCapabilities{}, err
	}

	views, err := s.views(ctx, workspaceID, servers)
	if err != nil {
		return service.LibraryCapabilities{}, err
	}

	library := service.LibraryCapabilities{
		Skills:     make([]service.LibrarySkill, 0, len(skills)),
		MCPServers: make([]service.LibraryMCPServer, 0, len(views)),
	}

	for _, skill := range skills {
		library.Skills = append(library.Skills, service.LibrarySkill{Skill: skill, AgentIDs: skillAgents[skill.ID]})
	}

	for _, view := range views {
		library.MCPServers = append(library.MCPServers, service.LibraryMCPServer{
			View:     view,
			AgentIDs: serverAgents[view.Server.ID],
		})
	}

	return library, nil
}

func (s *capabilities) views(
	ctx context.Context,
	workspaceID uuid.UUID,
	servers []entity.AgentMCPServer,
) ([]service.AgentMCPServerView, error) {
	var signedIn []uuid.UUID

	for _, server := range servers {
		if server.Auth == entity.AgentMCPAuthOAuth {
			signedIn = append(signedIn, server.ID)
		}
	}

	connections, err := s.connections.List(ctx, workspaceID, signedIn)
	if err != nil {
		return nil, err
	}

	byServer := make(map[uuid.UUID]entity.AgentMCPConnection, len(connections))
	for _, connection := range connections {
		byServer[connection.ServerID] = connection
	}

	views := make([]service.AgentMCPServerView, 0, len(servers))

	for _, server := range servers {
		view := service.AgentMCPServerView{Server: server}

		if connection, ok := byServer[server.ID]; ok {
			view.Connection = &connection
		}

		views = append(views, view)
	}

	return views, nil
}

func (s *capabilities) view(ctx context.Context, server entity.AgentMCPServer) (service.AgentMCPServerView, error) {
	views, err := s.views(ctx, server.WorkspaceID, []entity.AgentMCPServer{server})
	if err != nil {
		return service.AgentMCPServerView{}, err
	}

	return views[0], nil
}

func (s *capabilities) record(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.AuditAction,
	kind string,
	id uuid.UUID,
	name string,
	agentID *uuid.UUID,
) {
	detail := map[string]string{}
	if agentID != nil {
		detail["agent_id"] = agentID.String()
	} else {
		detail["scope"] = "library"
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       action,
		ResourceKind: kind,
		ResourceID:   id,
		ResourceName: name,
		Detail:       detail,
	})
}

func ignoreMissing(err, missing error) error {
	if errors.Is(err, missing) {
		return nil
	}

	return err
}
