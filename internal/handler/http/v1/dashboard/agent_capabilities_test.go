package dashboard_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	agentcapabilitysvc "github.com/usenorn/norn/internal/service/agentcapability"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type capabilityHarness struct {
	capabilities *agentcapabilitysvc.MockAgentCapabilities
	routes       http.Handler
	workspace    uuid.UUID
	agent        uuid.UUID
}

func newCapabilityHarness(t *testing.T) *capabilityHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &capabilityHarness{
		capabilities: agentcapabilitysvc.NewMockAgentCapabilities(ctrl),
		workspace:    uuid.New(),
		agent:        uuid.New(),
	}
	h.routes = newEdge(ctrl, edgeServices{agentCapabilities: h.capabilities})

	return h
}

func (h *capabilityHarness) call(t *testing.T, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	request := httptest.NewRequest(method, "/workspaces/"+h.workspace.String()+path, &body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	return recorder
}

func TestAServerIsListedWithTheNamesOfItsSecretsButNeverTheirValues(t *testing.T) {
	h := newCapabilityHarness(t)
	expired := time.Now().UTC().Add(-time.Minute)

	h.capabilities.EXPECT().ListForAgent(gomock.Any(), h.workspace, h.agent).Return(service.AgentCapabilitySet{
		MCPServers: []service.AgentMCPServerView{{
			Server: entity.AgentMCPServer{
				ID:         uuid.New(),
				AgentID:    &h.agent,
				Name:       "linear",
				Transport:  entity.AgentMCPHTTP,
				URL:        "https://mcp.linear.test/mcp",
				Auth:       entity.AgentMCPAuthOAuth,
				HeaderKeys: []string{"X-Team"},
			},
			Connection: &entity.AgentMCPConnection{Status: entity.AgentMCPConnected, ExpiresAt: &expired},
		}},
	}, nil)

	recorder := h.call(t, http.MethodGet, "/agents/"+h.agent.String()+"/capabilities", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.AgentCapabilities](t, recorder)
	if len(got.McpServers) != 1 || got.McpServers[0].Library || got.McpServers[0].HeaderKeys[0] != "X-Team" {
		t.Fatalf("servers = %+v", got.McpServers)
	}

	if got.McpServers[0].Connection == nil || got.McpServers[0].Connection.Status != api.AgentMcpConnectionStatusExpired {
		t.Errorf("connection = %+v, want expired once the access token has run out", got.McpServers[0].Connection)
	}

	if got.Skills == nil {
		t.Error("skills is null instead of an empty list")
	}

	for _, field := range []string{"secret", "token", "\"env\"", "\"headers\""} {
		if strings.Contains(strings.ToLower(recorder.Body.String()), field) {
			t.Errorf("the response carries %s: %s", field, recorder.Body)
		}
	}
}

func TestConnectingSendsTheCallbackOnThisInstance(t *testing.T) {
	h := newCapabilityHarness(t)
	serverID := uuid.New()

	h.capabilities.EXPECT().
		BeginMCPConnect(gomock.Any(), service.BeginMCPConnectInput{
			WorkspaceID: h.workspace,
			ServerID:    serverID,
			ReturnTo:    "/acme/settings/agents",
			RedirectURI: "https://norn.test/v1/agent-mcp/oauth/callback",
		}).
		Return("https://auth.linear.test/authorize?state=s", nil)

	recorder := h.call(t, http.MethodPost, "/agent-mcp-servers/"+serverID.String()+"/connect",
		api.ConnectAgentMcpServerRequest{ReturnTo: "/acme/settings/agents"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	if got := decodeBody[api.AgentMcpAuthorization](t, recorder); got.AuthorizationUrl != "https://auth.linear.test/authorize?state=s" {
		t.Errorf("authorization = %+v", got)
	}
}

func TestCapabilityFailuresCarryACodeTheScreenCanName(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{entity.ErrAgentSkillNameTaken, http.StatusConflict, "skill_name_taken"},
		{entity.ErrAgentMCPOAuthUnsupported, http.StatusConflict, "oauth_unsupported"},
		{fmt.Errorf("%w: dial", entity.ErrAgentSkillSourceUnreachable), http.StatusBadGateway, "skill_source_unreachable"},
		{entity.ErrAgentCapabilityEncryptionKeyMissing, http.StatusServiceUnavailable, "agent_capability_sealing_unavailable"},
		{entity.ErrAgentSkillSourceNotFound, http.StatusNotFound, ""},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			h := newCapabilityHarness(t)
			h.capabilities.EXPECT().ImportSkill(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.AgentSkill{}, tc.err)

			source := "anthropics/skills"
			recorder := h.call(t, http.MethodPost, "/agents/"+h.agent.String()+"/skills",
				api.AddAgentSkillRequest{Kind: api.AddAgentSkillRequestKindImport, Source: &source})

			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, tc.status, recorder.Body)
			}

			var problem struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(recorder.Body.Bytes(), &problem)

			if problem.Code != tc.code {
				t.Errorf("code = %q, want %q", problem.Code, tc.code)
			}
		})
	}
}

func TestAnImportGoesToTheAgentAndAWrittenSkillToTheLibrary(t *testing.T) {
	h := newCapabilityHarness(t)
	path := "skills/pdf"

	h.capabilities.EXPECT().
		ImportSkill(gomock.Any(), service.CapabilityOwner{WorkspaceID: h.workspace, AgentID: &h.agent}, service.ImportSkillInput{
			Source: "anthropics/skills", Path: &path,
		}).
		Return(entity.AgentSkill{ID: uuid.New(), AgentID: &h.agent, Name: "pdf", Source: entity.AgentSkillGitHub}, nil)

	source := "anthropics/skills"
	recorder := h.call(t, http.MethodPost, "/agents/"+h.agent.String()+"/skills",
		api.AddAgentSkillRequest{Kind: api.AddAgentSkillRequestKindImport, Source: &source, Path: &path})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("import: status = %d, body %s", recorder.Code, recorder.Body)
	}

	if got := decodeBody[api.AgentSkill](t, recorder); got.Origin == nil || got.Library {
		t.Errorf("imported skill = %+v, want an origin and not a library item", got)
	}

	instructions := "---\nname: house-style\ndescription: d\n---\n\nBody."
	h.capabilities.EXPECT().
		WriteSkill(gomock.Any(), service.CapabilityOwner{WorkspaceID: h.workspace}, service.SkillDraft{Instructions: instructions}).
		Return(entity.AgentSkill{ID: uuid.New(), Name: "house-style", Source: entity.AgentSkillManual}, nil)

	recorder = h.call(t, http.MethodPost, "/agent-library/skills",
		api.AddAgentSkillRequest{Kind: api.AddAgentSkillRequestKindManual, Instructions: &instructions})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("library: status = %d, body %s", recorder.Code, recorder.Body)
	}

	if got := decodeBody[api.AgentSkill](t, recorder); !got.Library || got.Origin != nil {
		t.Errorf("library skill = %+v", got)
	}
}
