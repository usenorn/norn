package agentcapability_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentmcpconnectionrepo "github.com/usenorn/norn/internal/repository/agentmcpconnection"
	agentmcpoauthstaterepo "github.com/usenorn/norn/internal/repository/agentmcpoauthstate"
	agentmcpserverrepo "github.com/usenorn/norn/internal/repository/agentmcpserver"
	agentskillrepo "github.com/usenorn/norn/internal/repository/agentskill"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	mcpauthorizerrepo "github.com/usenorn/norn/internal/repository/mcpauthorizer"
	mcpregistryrepo "github.com/usenorn/norn/internal/repository/mcpregistry"
	skillsourcerepo "github.com/usenorn/norn/internal/repository/skillsource"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	agentcapabilitysvc "github.com/usenorn/norn/internal/service/agentcapability"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
)

const redirectURI = "https://norn.test/v1/agent-mcp/oauth/callback"

type signIn struct {
	connection entity.AgentMCPConnection
	tokens     entity.AgentMCPTokens
}

type harness struct {
	agents      *agentrepo.MockAgent
	skills      *agentskillrepo.MockAgentSkill
	servers     *agentmcpserverrepo.MockAgentMCPServer
	connections *agentmcpconnectionrepo.MockAgentMCPConnection
	states      *agentmcpoauthstaterepo.MockAgentMCPOAuthState
	sources     *skillsourcerepo.MockSkillSource
	registry    *mcpregistryrepo.MockMCPRegistry
	oauth       *mcpauthorizerrepo.MockMCPAuthorizer
	blobs       *blobrepo.MockBlob
	authorizer  *authorizersvc.MockAuthorizer
	audit       *auditsvc.MockAudit
	service     service.AgentCapabilities
	toolkits    service.AgentToolkits

	workspaceID uuid.UUID
	caller      uuid.UUID
	role        entity.MembershipRole
	actorKind   entity.ActorKind
	agentsByID  map[uuid.UUID]entity.Agent

	skillRows   map[uuid.UUID]entity.AgentSkill
	serverRows  map[uuid.UUID]entity.AgentMCPServer
	secrets     map[uuid.UUID]entity.AgentMCPSecrets
	attached    map[uuid.UUID]map[uuid.UUID]bool
	signIns     map[uuid.UUID]signIn
	pending     map[string]entity.AgentMCPOAuthState
	stored      map[string][]byte
	audited     []entity.AuditEntry
	refreshed   int
	refreshWith error
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		agents:      agentrepo.NewMockAgent(ctrl),
		skills:      agentskillrepo.NewMockAgentSkill(ctrl),
		servers:     agentmcpserverrepo.NewMockAgentMCPServer(ctrl),
		connections: agentmcpconnectionrepo.NewMockAgentMCPConnection(ctrl),
		states:      agentmcpoauthstaterepo.NewMockAgentMCPOAuthState(ctrl),
		sources:     skillsourcerepo.NewMockSkillSource(ctrl),
		registry:    mcpregistryrepo.NewMockMCPRegistry(ctrl),
		oauth:       mcpauthorizerrepo.NewMockMCPAuthorizer(ctrl),
		blobs:       blobrepo.NewMockBlob(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		workspaceID: uuid.New(),
		caller:      uuid.New(),
		role:        entity.MembershipRoleMember,
		actorKind:   entity.ActorKindUser,
		agentsByID:  map[uuid.UUID]entity.Agent{},
		skillRows:   map[uuid.UUID]entity.AgentSkill{},
		serverRows:  map[uuid.UUID]entity.AgentMCPServer{},
		secrets:     map[uuid.UUID]entity.AgentMCPSecrets{},
		attached:    map[uuid.UUID]map[uuid.UUID]bool{},
		signIns:     map[uuid.UUID]signIn{},
		pending:     map[string]entity.AgentMCPOAuthState{},
		stored:      map[string][]byte{},
	}

	h.expectAccess()
	h.expectSkills()
	h.expectServers()
	h.expectSignIns()

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.service = agentcapabilitysvc.New(
		h.agents, h.skills, h.servers, h.connections, h.states, h.sources, h.registry, h.oauth,
		h.blobs, h.authorizer, h.audit, transactor, config.Instance{},
	)
	h.toolkits = agentcapabilitysvc.NewToolkits(
		h.skills, h.servers, h.connections, h.oauth, h.blobs, config.Instance{},
		config.AgentTooling{RefreshLead: 5 * time.Minute, DownloadTTL: 15 * time.Minute},
	)

	return h
}

func (h *harness) expectAccess() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.WorkspaceID != h.workspaceID {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{
				Actor: entity.Actor{Kind: h.actorKind, AccountID: h.caller},
				Role:  h.role,
			}, nil
		}).
		AnyTimes()

	h.agents.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID, agentID uuid.UUID) (entity.Agent, error) {
			agent, ok := h.agentsByID[agentID]
			if !ok || workspaceID != h.workspaceID {
				return entity.Agent{}, entity.ErrAgentNotFound
			}

			return agent, nil
		}).
		AnyTimes()

	h.audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) { h.audited = append(h.audited, entry) }).
		AnyTimes()

	h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key, _ string, body io.Reader, _ int64) error {
			content, err := io.ReadAll(body)
			h.stored[key] = content

			return err
		}).
		AnyTimes()

	h.blobs.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key string) error {
			delete(h.stored, key)

			return nil
		}).
		AnyTimes()

	h.blobs.EXPECT().
		PresignGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key string, _ entity.ServeSpec, _ time.Duration) (string, error) {
			return "https://blobs.test/" + key, nil
		}).
		AnyTimes()
}

func (h *harness) mine(agentID uuid.UUID, capability *uuid.UUID, id uuid.UUID) bool {
	return (capability != nil && *capability == agentID) || h.attached[agentID][id]
}

func (h *harness) expectSkills() {
	h.skills.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, skill entity.AgentSkill) (entity.AgentSkill, error) {
			h.skillRows[skill.ID] = skill

			return skill, nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		Replace(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, skill entity.AgentSkill) (entity.AgentSkill, error) {
			h.skillRows[skill.ID] = skill

			return skill, nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, skillID uuid.UUID) (entity.AgentSkill, error) {
			skill, ok := h.skillRows[skillID]
			if !ok {
				return entity.AgentSkill{}, entity.ErrAgentSkillNotFound
			}

			return skill, nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		ListByAgent(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, agentID uuid.UUID) ([]entity.AgentSkill, error) {
			var skills []entity.AgentSkill

			for _, skill := range h.skillRows {
				if h.mine(agentID, skill.AgentID, skill.ID) {
					skills = append(skills, skill)
				}
			}

			return skills, nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		CountOwned(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, agentID *uuid.UUID) (int, error) {
			count := 0

			for _, skill := range h.skillRows {
				if (agentID == nil && skill.AgentID == nil) ||
					(agentID != nil && skill.AgentID != nil && *agentID == *skill.AgentID) {
					count++
				}
			}

			return count, nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		Delete(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, skillID uuid.UUID) error {
			delete(h.skillRows, skillID)

			return nil
		}).
		AnyTimes()

	h.skills.EXPECT().
		Attach(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, attachment entity.AgentCapabilityAttachment) error {
			return h.attach(attachment)
		}).
		AnyTimes()
}

func (h *harness) attach(attachment entity.AgentCapabilityAttachment) error {
	if h.attached[attachment.AgentID] == nil {
		h.attached[attachment.AgentID] = map[uuid.UUID]bool{}
	}

	if h.attached[attachment.AgentID][attachment.CapabilityID] {
		return entity.ErrAgentCapabilityAttached
	}

	h.attached[attachment.AgentID][attachment.CapabilityID] = true

	return nil
}

func (h *harness) expectServers() {
	h.servers.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, server entity.AgentMCPServer, secrets entity.AgentMCPSecrets,
		) (entity.AgentMCPServer, error) {
			server.EnvKeys = entity.AgentMCPVariableKeys(secrets.Env)
			server.HeaderKeys = entity.AgentMCPVariableKeys(secrets.Headers)
			h.serverRows[server.ID] = server
			h.secrets[server.ID] = secrets

			return server, nil
		}).
		AnyTimes()

	h.servers.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, server entity.AgentMCPServer, secrets entity.AgentMCPSecrets,
		) (entity.AgentMCPServer, error) {
			server.EnvKeys = entity.AgentMCPVariableKeys(secrets.Env)
			server.HeaderKeys = entity.AgentMCPVariableKeys(secrets.Headers)
			h.serverRows[server.ID] = server
			h.secrets[server.ID] = secrets

			return server, nil
		}).
		AnyTimes()

	h.servers.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, serverID uuid.UUID) (entity.AgentMCPServer, error) {
			server, ok := h.serverRows[serverID]
			if !ok {
				return entity.AgentMCPServer{}, entity.ErrAgentMCPServerNotFound
			}

			return server, nil
		}).
		AnyTimes()

	h.servers.EXPECT().
		Secrets(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, serverID uuid.UUID) (entity.AgentMCPSecrets, error) {
			return h.secrets[serverID], nil
		}).
		AnyTimes()

	h.servers.EXPECT().
		ListByAgent(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, agentID uuid.UUID) ([]entity.AgentMCPServer, error) {
			var servers []entity.AgentMCPServer

			for _, server := range h.serverRows {
				if h.mine(agentID, server.AgentID, server.ID) {
					servers = append(servers, server)
				}
			}

			return servers, nil
		}).
		AnyTimes()

	h.servers.EXPECT().
		CountOwned(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).
		AnyTimes()

	h.servers.EXPECT().
		Attach(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, attachment entity.AgentCapabilityAttachment) error {
			return h.attach(attachment)
		}).
		AnyTimes()
}

func (h *harness) expectSignIns() {
	h.connections.EXPECT().
		List(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, serverIDs []uuid.UUID) ([]entity.AgentMCPConnection, error) {
			var connections []entity.AgentMCPConnection

			for _, id := range serverIDs {
				if row, ok := h.signIns[id]; ok {
					connections = append(connections, row.connection)
				}
			}

			return connections, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, serverID uuid.UUID) (entity.AgentMCPConnection, error) {
			row, ok := h.signIns[serverID]
			if !ok {
				return entity.AgentMCPConnection{}, entity.ErrAgentMCPConnectionNotFound
			}

			return row.connection, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Tokens(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, serverID uuid.UUID) (entity.AgentMCPTokens, error) {
			return h.signIns[serverID].tokens, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, connection entity.AgentMCPConnection, tokens entity.AgentMCPTokens,
		) (entity.AgentMCPConnection, error) {
			connection.Status = entity.AgentMCPConnected
			connection.ExpiresAt = tokens.ExpiresAt
			connection.Failure = ""
			h.signIns[connection.ServerID] = signIn{connection: connection, tokens: tokens}

			return connection, nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		MarkFailed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, serverID uuid.UUID, failure entity.AgentMCPConnectionFailure, _ time.Time,
		) error {
			row := h.signIns[serverID]
			row.connection.Status, row.connection.Failure = entity.AgentMCPFailed, failure
			h.signIns[serverID] = row

			return nil
		}).
		AnyTimes()

	h.connections.EXPECT().
		Delete(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, serverID uuid.UUID) error {
			if _, ok := h.signIns[serverID]; !ok {
				return entity.ErrAgentMCPConnectionNotFound
			}

			delete(h.signIns, serverID)

			return nil
		}).
		AnyTimes()

	h.states.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state string, attempt entity.AgentMCPOAuthState) error {
			h.pending[state] = attempt

			return nil
		}).
		AnyTimes()

	h.states.EXPECT().
		Take(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, state string) (entity.AgentMCPOAuthState, error) {
			attempt, ok := h.pending[state]
			if !ok {
				return entity.AgentMCPOAuthState{}, entity.ErrAgentMCPOAuthStateNotFound
			}

			delete(h.pending, state)

			return attempt, nil
		}).
		AnyTimes()

	h.oauth.EXPECT().
		Refresh(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ entity.AgentMCPConnection, tokens entity.AgentMCPTokens, _ bool,
		) (entity.AgentMCPTokens, error) {
			h.refreshed++

			if h.refreshWith != nil {
				return entity.AgentMCPTokens{}, h.refreshWith
			}

			expiry := time.Now().UTC().Add(time.Hour)

			return entity.AgentMCPTokens{
				ClientSecret: tokens.ClientSecret,
				AccessToken:  "at-refreshed",
				RefreshToken: tokens.RefreshToken,
				ExpiresAt:    &expiry,
			}, nil
		}).
		AnyTimes()
}

func (h *harness) agent(owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	h.agentsByID[id] = entity.Agent{ID: id, WorkspaceID: h.workspaceID, OwnerAccountID: owner, Name: "triage"}

	return id
}

func (h *harness) owner(agentID *uuid.UUID) service.CapabilityOwner {
	return service.CapabilityOwner{WorkspaceID: h.workspaceID, AgentID: agentID}
}

func (h *harness) librarySkill(name string) entity.AgentSkill {
	skill := entity.AgentSkill{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Name:        name,
		Source:      entity.AgentSkillManual,
		ObjectKey:   "agent-skills/" + name,
	}
	h.skillRows[skill.ID] = skill

	return skill
}

func (h *harness) server(agentID *uuid.UUID, auth entity.AgentMCPAuth, secrets entity.AgentMCPSecrets) entity.AgentMCPServer {
	server := entity.AgentMCPServer{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     agentID,
		Name:        "linear-" + strings.ToLower(uuid.NewString()[:4]),
		Transport:   entity.AgentMCPHTTP,
		URL:         "https://mcp.linear.test/mcp",
		Auth:        auth,
		HeaderKeys:  entity.AgentMCPVariableKeys(secrets.Headers),
	}
	h.serverRows[server.ID] = server
	h.secrets[server.ID] = secrets

	return server
}

func (h *harness) actions() []entity.AuditAction {
	actions := make([]entity.AuditAction, len(h.audited))
	for i, entry := range h.audited {
		actions[i] = entry.Action
	}

	return actions
}

func manifest(name string) string {
	return "---\nname: " + name + "\ndescription: Writes release notes.\n---\n\nGroup merged pull requests by area.\n"
}
