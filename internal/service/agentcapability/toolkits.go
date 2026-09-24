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
	skills      repository.AgentSkill
	servers     repository.AgentMCPServer
	connections repository.AgentMCPConnection
	oauth       repository.MCPAuthorizer
	blobs       repository.Blob
	instance    config.Instance
	tooling     config.AgentTooling
}

func NewToolkits(
	skills repository.AgentSkill,
	servers repository.AgentMCPServer,
	connections repository.AgentMCPConnection,
	oauth repository.MCPAuthorizer,
	blobs repository.Blob,
	instance config.Instance,
	tooling config.AgentTooling,
) service.AgentToolkits {
	return &toolkits{
		skills:      skills,
		servers:     servers,
		connections: connections,
		oauth:       oauth,
		blobs:       blobs,
		instance:    instance,
		tooling:     tooling,
	}
}

func (s *toolkits) Resolve(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.AgentToolkit, error) {
	skills, err := s.skills.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return entity.AgentToolkit{}, err
	}

	servers, err := s.servers.ListByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return entity.AgentToolkit{}, err
	}

	toolkit := entity.AgentToolkit{
		Skills:     make([]entity.AgentToolkitSkill, 0, len(skills)),
		MCPServers: make([]entity.AgentToolkitServer, 0, len(servers)),
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
		resolved, usable, err := s.server(ctx, server)
		if err != nil {
			return entity.AgentToolkit{}, err
		}

		if usable {
			toolkit.MCPServers = append(toolkit.MCPServers, resolved)
		}
	}

	return toolkit, nil
}

func (s *toolkits) server(
	ctx context.Context,
	server entity.AgentMCPServer,
) (entity.AgentToolkitServer, bool, error) {
	secrets, err := s.servers.Secrets(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		return entity.AgentToolkitServer{}, false, err
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
		return resolved, true, nil
	}

	accessToken, usable, err := s.accessToken(ctx, server)
	if err != nil || !usable {
		return entity.AgentToolkitServer{}, false, err
	}

	if resolved.Headers == nil {
		resolved.Headers = map[string]string{}
	}

	resolved.Headers[entity.AgentMCPAuthorizationHeader] = entity.AgentMCPBearer(accessToken)

	return resolved, true, nil
}

func (s *toolkits) accessToken(ctx context.Context, server entity.AgentMCPServer) (string, bool, error) {
	connection, err := s.connections.Get(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		if errors.Is(err, entity.ErrAgentMCPConnectionNotFound) {
			return "", false, nil
		}

		return "", false, err
	}

	if connection.Status == entity.AgentMCPFailed {
		return "", false, nil
	}

	tokens, err := s.connections.Tokens(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		return "", false, err
	}

	now := time.Now().UTC()

	if !connection.DueForRefresh(now, s.tooling.RefreshLead) {
		return tokens.AccessToken, true, nil
	}

	refreshed, err := s.oauth.Refresh(ctx, connection, tokens, s.instance.SelfHosted)
	if err != nil {
		if errors.Is(err, entity.ErrAgentMCPOAuthRefused) {
			return "", false, s.connections.MarkFailed(
				ctx, server.WorkspaceID, server.ID, entity.AgentMCPFailureRefreshRejected, now,
			)
		}

		if connection.Current(now) == entity.AgentMCPConnected {
			return tokens.AccessToken, true, nil
		}

		return "", false, nil
	}

	if _, err := s.connections.Save(ctx, connection, refreshed); err != nil {
		return "", false, err
	}

	return refreshed.AccessToken, true, nil
}
