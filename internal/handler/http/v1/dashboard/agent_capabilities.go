package dashboard

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/mcpoauth"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func agentSkillDTO(skill entity.AgentSkill) api.AgentSkill {
	dto := api.AgentSkill{
		Id:           skill.ID,
		Name:         skill.Name,
		Description:  skill.Description,
		Source:       api.AgentSkillSource(skill.Source),
		Instructions: skill.Instructions,
		ContentHash:  skill.ContentHash,
		SizeBytes:    skill.SizeBytes,
		FileCount:    int32(skill.FileCount),
		Library:      skill.InLibrary(),
		CreatedAt:    skill.CreatedAt,
		UpdatedAt:    skill.UpdatedAt,
	}

	if skill.Source == entity.AgentSkillGitHub {
		dto.Origin = &api.AgentSkillOrigin{
			Repository: skill.Origin.Repository,
			Path:       skill.Origin.Path,
			Ref:        skill.Origin.Ref,
			Revision:   skill.Origin.Revision,
		}
	}

	return dto
}

func agentMcpServerDTO(view service.AgentMCPServerView) api.AgentMcpServer {
	server := view.Server

	dto := api.AgentMcpServer{
		Id:            server.ID,
		Name:          server.Name,
		Transport:     api.AgentMcpTransport(server.Transport),
		Command:       server.Command,
		Args:          nonNilStrings(server.Args),
		Url:           server.URL,
		Auth:          api.AgentMcpAuth(server.Auth),
		EnvKeys:       nonNilStrings(server.EnvKeys),
		HeaderKeys:    nonNilStrings(server.HeaderKeys),
		OauthClientId: server.OAuthClientID,
		Library:       server.InLibrary(),
		CreatedAt:     server.CreatedAt,
		UpdatedAt:     server.UpdatedAt,
	}

	if server.Registry.Name != "" {
		dto.RegistryName = &server.Registry.Name
		dto.RegistryVersion = &server.Registry.Version
	}

	if view.Connection != nil {
		connection := view.Connection
		dto.Connection = &api.AgentMcpConnection{
			Status:      api.AgentMcpConnectionStatus(connection.Current(time.Now().UTC())),
			Issuer:      connection.Issuer,
			Scopes:      nonNilStrings(connection.Scopes),
			ExpiresAt:   connection.ExpiresAt,
			ConnectedAt: connection.ConnectedAt,
		}

		if connection.Failure != "" {
			failure := api.AgentMcpConnectionFailure(connection.Failure)
			dto.Connection.Failure = &failure
		}
	}

	return dto
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

func agentCapabilitiesDTO(set service.AgentCapabilitySet) api.AgentCapabilities {
	dto := api.AgentCapabilities{
		Skills:     make([]api.AgentSkill, 0, len(set.Skills)),
		McpServers: make([]api.AgentMcpServer, 0, len(set.MCPServers)),
	}

	for _, skill := range set.Skills {
		dto.Skills = append(dto.Skills, agentSkillDTO(skill))
	}

	for _, server := range set.MCPServers {
		dto.McpServers = append(dto.McpServers, agentMcpServerDTO(server))
	}

	return dto
}

func agentLibraryDTO(library service.LibraryCapabilities) api.AgentLibrary {
	dto := api.AgentLibrary{
		Skills:     make([]api.AgentLibrarySkill, 0, len(library.Skills)),
		McpServers: make([]api.AgentLibraryMcpServer, 0, len(library.MCPServers)),
	}

	for _, skill := range library.Skills {
		dto.Skills = append(dto.Skills, api.AgentLibrarySkill{
			Skill:    agentSkillDTO(skill.Skill),
			AgentIds: nonNilIDs(skill.AgentIDs),
		})
	}

	for _, server := range library.MCPServers {
		dto.McpServers = append(dto.McpServers, api.AgentLibraryMcpServer{
			Server:   agentMcpServerDTO(server.View),
			AgentIds: nonNilIDs(server.AgentIDs),
		})
	}

	return dto
}

func nonNilIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}

	return ids
}

func agentMcpServerInput(body *api.AgentMcpServerRequest) entity.AgentMCPServerInput {
	input := entity.AgentMCPServerInput{
		Name:              body.Name,
		Transport:         entity.AgentMCPTransport(body.Transport),
		Command:           optionalString(body.Command),
		URL:               optionalString(body.Url),
		OAuthClientID:     optionalString(body.OauthClientId),
		OAuthClientSecret: optionalString(body.OauthClientSecret),
		Registry: entity.AgentMCPRegistryRef{
			Name:    optionalString(body.RegistryName),
			Version: optionalString(body.RegistryVersion),
		},
	}

	if body.Auth != nil {
		input.Auth = entity.AgentMCPAuth(*body.Auth)
	}

	if body.Args != nil {
		input.Args = *body.Args
	}

	if body.Env != nil {
		input.Env = *body.Env
	}

	if body.Headers != nil {
		input.Headers = *body.Headers
	}

	return input
}

func skillDraft(instructions *string, archive *[]byte) service.SkillDraft {
	draft := service.SkillDraft{Instructions: optionalString(instructions)}
	if archive != nil {
		draft.Archive = *archive
	}

	return draft
}

func (h *handler) addSkill(
	ctx context.Context,
	owner service.CapabilityOwner,
	body *api.AddAgentSkillRequest,
) (entity.AgentSkill, error) {
	if body.Kind == api.AddAgentSkillRequestKindImport {
		return h.agentCapabilities.ImportSkill(ctx, owner, service.ImportSkillInput{
			Source: optionalString(body.Source),
			Path:   body.Path,
		})
	}

	return h.agentCapabilities.WriteSkill(ctx, owner, skillDraft(body.Instructions, body.Archive))
}

func (h *handler) GetAgentCapabilities(
	ctx context.Context,
	request api.GetAgentCapabilitiesRequestObject,
) (api.GetAgentCapabilitiesResponseObject, error) {
	set, err := h.agentCapabilities.ListForAgent(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.GetAgentCapabilities200JSONResponse(agentCapabilitiesDTO(set)), nil
}

func (h *handler) AddAgentSkill(
	ctx context.Context,
	request api.AddAgentSkillRequestObject,
) (api.AddAgentSkillResponseObject, error) {
	agentID := request.AgentId

	skill, err := h.addSkill(ctx, service.CapabilityOwner{WorkspaceID: request.WorkspaceId, AgentID: &agentID}, request.Body)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AddAgentSkill201JSONResponse(agentSkillDTO(skill)), nil
}

func (h *handler) AddLibrarySkill(
	ctx context.Context,
	request api.AddLibrarySkillRequestObject,
) (api.AddLibrarySkillResponseObject, error) {
	skill, err := h.addSkill(ctx, service.CapabilityOwner{WorkspaceID: request.WorkspaceId}, request.Body)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AddLibrarySkill201JSONResponse(agentSkillDTO(skill)), nil
}

func (h *handler) AddAgentMcpServer(
	ctx context.Context,
	request api.AddAgentMcpServerRequestObject,
) (api.AddAgentMcpServerResponseObject, error) {
	agentID := request.AgentId

	view, err := h.agentCapabilities.CreateMCPServer(
		ctx,
		service.CapabilityOwner{WorkspaceID: request.WorkspaceId, AgentID: &agentID},
		agentMcpServerInput(request.Body),
	)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AddAgentMcpServer201JSONResponse(agentMcpServerDTO(view)), nil
}

func (h *handler) AddLibraryMcpServer(
	ctx context.Context,
	request api.AddLibraryMcpServerRequestObject,
) (api.AddLibraryMcpServerResponseObject, error) {
	view, err := h.agentCapabilities.CreateMCPServer(
		ctx,
		service.CapabilityOwner{WorkspaceID: request.WorkspaceId},
		agentMcpServerInput(request.Body),
	)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AddLibraryMcpServer201JSONResponse(agentMcpServerDTO(view)), nil
}

func (h *handler) AttachLibrarySkill(
	ctx context.Context,
	request api.AttachLibrarySkillRequestObject,
) (api.AttachLibrarySkillResponseObject, error) {
	if err := h.agentCapabilities.AttachSkill(ctx, request.WorkspaceId, request.AgentId, request.SkillId); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AttachLibrarySkill204Response{}, nil
}

func (h *handler) DetachLibrarySkill(
	ctx context.Context,
	request api.DetachLibrarySkillRequestObject,
) (api.DetachLibrarySkillResponseObject, error) {
	if err := h.agentCapabilities.DetachSkill(ctx, request.WorkspaceId, request.AgentId, request.SkillId); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.DetachLibrarySkill204Response{}, nil
}

func (h *handler) AttachLibraryMcpServer(
	ctx context.Context,
	request api.AttachLibraryMcpServerRequestObject,
) (api.AttachLibraryMcpServerResponseObject, error) {
	if err := h.agentCapabilities.AttachMCPServer(
		ctx, request.WorkspaceId, request.AgentId, request.ServerId,
	); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.AttachLibraryMcpServer204Response{}, nil
}

func (h *handler) DetachLibraryMcpServer(
	ctx context.Context,
	request api.DetachLibraryMcpServerRequestObject,
) (api.DetachLibraryMcpServerResponseObject, error) {
	if err := h.agentCapabilities.DetachMCPServer(
		ctx, request.WorkspaceId, request.AgentId, request.ServerId,
	); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.DetachLibraryMcpServer204Response{}, nil
}

func (h *handler) GetAgentLibrary(
	ctx context.Context,
	request api.GetAgentLibraryRequestObject,
) (api.GetAgentLibraryResponseObject, error) {
	library, err := h.agentCapabilities.ListLibrary(ctx, request.WorkspaceId)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.GetAgentLibrary200JSONResponse(agentLibraryDTO(library)), nil
}

func (h *handler) RewriteAgentSkill(
	ctx context.Context,
	request api.RewriteAgentSkillRequestObject,
) (api.RewriteAgentSkillResponseObject, error) {
	skill, err := h.agentCapabilities.RewriteSkill(
		ctx, request.WorkspaceId, request.SkillId, skillDraft(request.Body.Instructions, request.Body.Archive),
	)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.RewriteAgentSkill200JSONResponse(agentSkillDTO(skill)), nil
}

func (h *handler) DeleteAgentSkill(
	ctx context.Context,
	request api.DeleteAgentSkillRequestObject,
) (api.DeleteAgentSkillResponseObject, error) {
	if err := h.agentCapabilities.DeleteSkill(ctx, request.WorkspaceId, request.SkillId); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.DeleteAgentSkill204Response{}, nil
}

func (h *handler) PullAgentSkill(
	ctx context.Context,
	request api.PullAgentSkillRequestObject,
) (api.PullAgentSkillResponseObject, error) {
	skill, err := h.agentCapabilities.PullSkill(ctx, request.WorkspaceId, request.SkillId)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.PullAgentSkill200JSONResponse(agentSkillDTO(skill)), nil
}

func (h *handler) UpdateAgentMcpServer(
	ctx context.Context,
	request api.UpdateAgentMcpServerRequestObject,
) (api.UpdateAgentMcpServerResponseObject, error) {
	view, err := h.agentCapabilities.UpdateMCPServer(
		ctx, request.WorkspaceId, request.ServerId, agentMcpServerInput(request.Body),
	)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.UpdateAgentMcpServer200JSONResponse(agentMcpServerDTO(view)), nil
}

func (h *handler) DeleteAgentMcpServer(
	ctx context.Context,
	request api.DeleteAgentMcpServerRequestObject,
) (api.DeleteAgentMcpServerResponseObject, error) {
	if err := h.agentCapabilities.DeleteMCPServer(ctx, request.WorkspaceId, request.ServerId); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.DeleteAgentMcpServer204Response{}, nil
}

func (h *handler) ConnectAgentMcpServer(
	ctx context.Context,
	request api.ConnectAgentMcpServerRequestObject,
) (api.ConnectAgentMcpServerResponseObject, error) {
	authorizationURL, err := h.agentCapabilities.BeginMCPConnect(ctx, service.BeginMCPConnectInput{
		WorkspaceID: request.WorkspaceId,
		ServerID:    request.ServerId,
		ReturnTo:    request.Body.ReturnTo,
		RedirectURI: strings.TrimRight(h.app.BaseURL, "/") + mcpoauth.CallbackPath,
	})
	if err != nil {
		return agentCapabilityFailure(err)
	}

	return api.ConnectAgentMcpServer200JSONResponse{AuthorizationUrl: authorizationURL}, nil
}

func (h *handler) DisconnectAgentMcpServer(
	ctx context.Context,
	request api.DisconnectAgentMcpServerRequestObject,
) (api.DisconnectAgentMcpServerResponseObject, error) {
	if err := h.agentCapabilities.DisconnectMCP(ctx, request.WorkspaceId, request.ServerId); err != nil {
		return agentCapabilityFailure(err)
	}

	return api.DisconnectAgentMcpServer204Response{}, nil
}

func (h *handler) ResolveSkillSource(
	ctx context.Context,
	request api.ResolveSkillSourceRequestObject,
) (api.ResolveSkillSourceResponseObject, error) {
	discovery, err := h.agentCapabilities.ResolveSkillSource(ctx, request.WorkspaceId, request.Params.Source)
	if err != nil {
		return agentCapabilityFailure(err)
	}

	dto := api.ResolveSkillSource200JSONResponse{
		Repository: discovery.Location.Repository,
		Ref:        discovery.Location.Ref,
		Revision:   discovery.Revision,
		Candidates: make([]api.SkillCandidate, 0, len(discovery.Candidates)),
	}

	for _, candidate := range discovery.Candidates {
		dto.Candidates = append(dto.Candidates, api.SkillCandidate{
			Name:        candidate.Name,
			Description: candidate.Description,
			Path:        candidate.Path,
		})
	}

	if picked, ok := discovery.Pick(discovery.Location); ok {
		dto.Suggested = &picked.Path
	}

	return dto, nil
}

func (h *handler) SearchMcpRegistry(
	ctx context.Context,
	request api.SearchMcpRegistryRequestObject,
) (api.SearchMcpRegistryResponseObject, error) {
	entries, err := h.agentCapabilities.SearchRegistry(ctx, request.WorkspaceId, optionalString(request.Params.Search))
	if err != nil {
		return agentCapabilityFailure(err)
	}

	dto := make(api.SearchMcpRegistry200JSONResponse, 0, len(entries))

	for _, entry := range entries {
		templates := make([]api.McpServerTemplate, 0, len(entry.Templates))

		for _, template := range entry.Templates {
			templates = append(templates, api.McpServerTemplate{
				Transport: api.AgentMcpTransport(template.Transport),
				Command:   template.Command,
				Args:      nonNilStrings(template.Args),
				Url:       template.URL,
				Env:       variableSpecs(template.Env),
				Headers:   variableSpecs(template.Headers),
			})
		}

		registered := api.McpRegistryEntry{
			Name:        entry.Name,
			Title:       entry.Title,
			Description: entry.Description,
			Version:     entry.Version,
			Templates:   templates,
		}

		if entry.WebsiteURL != "" {
			registered.WebsiteUrl = &entry.WebsiteURL
		}

		if entry.Repository != "" {
			registered.Repository = &entry.Repository
		}

		dto = append(dto, registered)
	}

	return dto, nil
}

func variableSpecs(specs []entity.AgentMCPVariableSpec) []api.McpVariableSpec {
	dto := make([]api.McpVariableSpec, 0, len(specs))

	for _, spec := range specs {
		variable := api.McpVariableSpec{Key: spec.Key, Required: spec.Required, Secret: spec.Secret}

		if spec.Description != "" {
			variable.Description = &spec.Description
		}

		if spec.Default != "" {
			variable.Default = &spec.Default
		}

		dto = append(dto, variable)
	}

	return dto
}

func agentCapabilityFailure(err error) (problemResponse, error) {
	if problem, ok := problemFor(err); ok {
		return problem, nil
	}

	return problemResponse{}, err
}

func (r problemResponse) VisitGetAgentCapabilitiesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddAgentSkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddLibrarySkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddAgentMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddLibraryMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAttachLibrarySkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDetachLibrarySkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAttachLibraryMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDetachLibraryMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetAgentLibraryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRewriteAgentSkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteAgentSkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPullAgentSkillResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateAgentMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteAgentMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConnectAgentMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisconnectAgentMcpServerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitResolveSkillSourceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSearchMcpRegistryResponse(w http.ResponseWriter) error {
	return r.write(w)
}
