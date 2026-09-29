package agentcapability

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const skillArchiveDisposition = "attachment"

type toolkits struct {
	workspaces  repository.Workspace
	projects    repository.Project
	agents      repository.Agent
	skills      repository.AgentSkill
	servers     repository.AgentMCPServer
	connections repository.AgentMCPConnection
	oauth       repository.MCPAuthorizer
	blobs       repository.Blob
	instance    config.Instance
	tooling     config.AgentTooling
}

func NewToolkits(
	workspaces repository.Workspace,
	projects repository.Project,
	agents repository.Agent,
	skills repository.AgentSkill,
	servers repository.AgentMCPServer,
	connections repository.AgentMCPConnection,
	oauth repository.MCPAuthorizer,
	blobs repository.Blob,
	instance config.Instance,
	tooling config.AgentTooling,
) service.AgentToolkits {
	return &toolkits{
		workspaces:  workspaces,
		projects:    projects,
		agents:      agents,
		skills:      skills,
		servers:     servers,
		connections: connections,
		oauth:       oauth,
		blobs:       blobs,
		instance:    instance,
		tooling:     tooling,
	}
}

func (s *toolkits) Resolve(
	ctx context.Context,
	workspaceID, agentID, projectID uuid.UUID,
) (entity.AgentToolkit, error) {
	instructions, err := s.instructions(ctx, workspaceID, agentID, projectID)
	if err != nil {
		return entity.AgentToolkit{}, err
	}

	skills, err := s.skills.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return entity.AgentToolkit{}, err
	}

	servers, err := s.servers.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return entity.AgentToolkit{}, err
	}

	toolkit := entity.AgentToolkit{
		Instructions: instructions,
		Skills:       make([]entity.AgentToolkitSkill, 0, len(skills)),
		MCPServers:   make([]entity.AgentToolkitServer, 0, len(servers)),
	}

	for _, skill := range skills {
		link, err := s.blobs.PresignGet(ctx, skill.ObjectKey, entity.ServeSpec{
			ContentType: entity.AgentSkillBundleContentType,
			Disposition: skillArchiveDisposition,
			FileName:    skill.Name + ".tar.gz",
		}, s.tooling.DownloadTTL)
		if err != nil {
			return entity.AgentToolkit{}, err
		}

		toolkit.Skills = append(toolkit.Skills, entity.AgentToolkitSkill{
			Name:        skill.Name,
			ContentHash: skill.ContentHash,
			DownloadURL: link,
		})
	}

	for _, server := range servers {
		resolved, unavailable, err := s.server(ctx, server)
		if err != nil {
			return entity.AgentToolkit{}, err
		}

		if unavailable != "" {
			toolkit.Unavailable = append(toolkit.Unavailable, entity.AgentToolkitGap{
				Server: server.Name,
				Reason: unavailable,
			})

			continue
		}

		toolkit.MCPServers = append(toolkit.MCPServers, resolved)
	}

	return toolkit, nil
}

func (s *toolkits) instructions(
	ctx context.Context,
	workspaceID, agentID, projectID uuid.UUID,
) (string, error) {
	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return "", err
	}

	var projectInstructions string

	if projectID != uuid.Nil {
		project, err := s.projects.GetByID(ctx, workspaceID, projectID)
		if err != nil {
			return "", err
		}

		projectInstructions = project.AgentInstructions
	}

	return entity.ComposeAgentInstructions(
		workspace.AgentInstructions,
		projectInstructions,
		agent.AgentInstructions,
	), nil
}

func (s *toolkits) server(
	ctx context.Context,
	server entity.AgentMCPServer,
) (entity.AgentToolkitServer, entity.AgentMCPUnavailability, error) {
	secrets, err := s.servers.Secrets(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		return entity.AgentToolkitServer{}, "", err
	}

	resolved := entity.AgentToolkitServer{
		Name:      server.Name,
		Transport: server.Transport,
		Command:   server.Command,
		Args:      server.Args,
		Env:       maps.Clone(secrets.Env),
		URL:       server.URL,
		Headers:   maps.Clone(secrets.Headers),
	}

	if server.Auth != entity.AgentMCPAuthOAuth {
		return resolved, "", nil
	}

	accessToken, unavailable, err := s.accessToken(ctx, server)
	if err != nil || unavailable != "" {
		return entity.AgentToolkitServer{}, unavailable, err
	}

	if resolved.Headers == nil {
		resolved.Headers = map[string]string{}
	}

	resolved.Headers[entity.AgentMCPAuthorizationHeader] = entity.AgentMCPBearer(accessToken)

	return resolved, "", nil
}

func (s *toolkits) accessToken(
	ctx context.Context,
	server entity.AgentMCPServer,
) (string, entity.AgentMCPUnavailability, error) {
	connection, err := s.connections.Get(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		if errors.Is(err, entity.ErrAgentMCPConnectionNotFound) {
			return "", entity.AgentMCPNotSignedIn, nil
		}

		return "", "", err
	}

	if connection.Status == entity.AgentMCPFailed {
		return "", entity.AgentMCPSignInFailed, nil
	}

	tokens, err := s.connections.Tokens(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		return "", "", err
	}

	now := time.Now().UTC()

	if !connection.DueForRefresh(now, s.tooling.RefreshLead) {
		return tokens.AccessToken, "", nil
	}

	refreshed, err := s.oauth.Refresh(ctx, connection, tokens, s.instance.SelfHosted)
	if err != nil {
		if errors.Is(err, entity.ErrAgentMCPOAuthRefused) {
			return "", entity.AgentMCPSignInFailed, s.connections.MarkFailed(
				ctx, server.WorkspaceID, server.ID, entity.AgentMCPFailureRefreshRejected, now,
			)
		}

		if connection.Current(now) == entity.AgentMCPConnected {
			return tokens.AccessToken, "", nil
		}

		return "", entity.AgentMCPSignInExpired, nil
	}

	if _, err := s.connections.Save(ctx, connection, refreshed); err != nil {
		return "", "", err
	}

	return refreshed.AccessToken, "", nil
}
