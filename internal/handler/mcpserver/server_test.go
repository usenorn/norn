package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	searchsvc "github.com/usenorn/norn/internal/service/search"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"

	"github.com/usenorn/norn/internal/handler/mcpserver"
)

type harness struct {
	issues     *issuesvc.MockIssues
	workspaces *workspacesvc.MockWorkspaces
	edge       *mcpserver.Edge
	actor      entity.Actor
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		issues:     issuesvc.NewMockIssues(ctrl),
		workspaces: workspacesvc.NewMockWorkspaces(ctrl),
		actor: entity.Actor{
			Kind:      entity.ActorKindToken,
			AccountID: uuid.New(),
			Scopes:    entity.MCPScopesFor(entity.MCPCapabilityWrite),
		},
	}

	h.edge = mcpserver.New(
		h.issues,
		issuecommentsvc.NewMockIssueComments(ctrl),
		projectsvc.NewMockProjects(ctrl),
		cyclesvc.NewMockCycles(ctrl),
		teamsvc.NewMockTeams(ctrl),
		h.workspaces,
		workflowstatesvc.NewMockWorkflowStates(ctrl),
		labelsvc.NewMockLabels(ctrl),
		searchsvc.NewMockSearches(ctrl),
		config.App{Version: "test"},
		config.MCP{Enabled: true},
	)

	return h
}

func (h *harness) session(t *testing.T) *mcp.ClientSession {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := identity.WithActor(r.Context(), h.actor)
		h.edge.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(server.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "test"}, nil)

	session, err := client.Connect(
		t.Context(),
		&mcp.StreamableClientTransport{Endpoint: server.URL},
		nil,
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestTheActorOnTheRequestContextReachesToolHandlers(t *testing.T) {
	h := newHarness(t)

	workspace := entity.Workspace{ID: uuid.New(), Slug: "acme", Name: "Acme"}

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), h.actor.AccountID).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID) ([]entity.Workspace, error) {
			if _, ok := identity.Actor(ctx); !ok {
				t.Error(
					"the service saw a context without an actor; the SDK dropped the request " +
						"context between the HTTP handler and the tool handler",
				)
			}

			return []entity.Workspace{workspace}, nil
		})

	session := h.session(t)

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "norn_list_workspaces",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool errored: %v", result.Content)
	}

	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	if !strings.Contains(string(payload), "acme") {
		t.Fatalf("structured content %s does not carry the workspace", payload)
	}
}

func TestEveryAdvertisedToolIsRegistered(t *testing.T) {
	h := newHarness(t)
	session := h.session(t)

	tools, err := session.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	registered := make(map[string]bool, len(tools.Tools))

	for _, tool := range tools.Tools {
		registered[tool.Name] = true
	}

	for _, name := range []string{
		"norn_list_workspaces",
		"norn_get_workspace_structure",
		"norn_list_workspace_members",
		"norn_get_issue",
		"norn_list_issues",
		"norn_search",
		"norn_list_projects",
		"norn_get_project",
		"norn_list_cycles",
		"norn_get_cycle",
		"norn_create_issue",
		"norn_update_issue",
		"norn_change_issue_state",
		"norn_create_comment",
	} {
		if !registered[name] {
			t.Errorf("tool %s is not registered", name)
		}
	}

	if len(tools.Tools) != 14 {
		t.Errorf("registered %d tools, want 14", len(tools.Tools))
	}
}

func TestAHiddenWorkspaceAndAMissingWorkspaceLookIdentical(t *testing.T) {
	h := newHarness(t)

	hidden := entity.Workspace{ID: uuid.New(), Slug: "hidden", Name: "Hidden"}
	h.actor.Grants = entity.APITokenGrants{{WorkspaceID: uuid.New(), AllTeams: true}}

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), h.actor.AccountID).
		Return([]entity.Workspace{hidden}, nil).
		Times(2)

	session := h.session(t)

	call := func(workspace string) string {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "norn_get_issue",
			Arguments: map[string]any{"workspace": workspace, "issue": "ENG-1"},
		})
		if err != nil {
			t.Fatalf("call tool: %v", err)
		}

		if !result.IsError {
			t.Fatal("reaching past the connection's grants did not fail")
		}

		payload, err := json.Marshal(result.Content)
		if err != nil {
			t.Fatalf("marshal content: %v", err)
		}

		return string(payload)
	}

	narrowedAway := call("hidden")
	missing := call("no-such-workspace")

	if narrowedAway != missing {
		t.Fatalf(
			"a narrowed-away workspace answers differently from a missing one:\n%s\nvs\n%s\n"+
				"existence must never leak through error wording",
			narrowedAway, missing,
		)
	}
}

func TestDisabledMCPAnswers404(t *testing.T) {
	ctrl := gomock.NewController(t)

	edge := mcpserver.New(
		issuesvc.NewMockIssues(ctrl),
		issuecommentsvc.NewMockIssueComments(ctrl),
		projectsvc.NewMockProjects(ctrl),
		cyclesvc.NewMockCycles(ctrl),
		teamsvc.NewMockTeams(ctrl),
		workspacesvc.NewMockWorkspaces(ctrl),
		workflowstatesvc.NewMockWorkflowStates(ctrl),
		labelsvc.NewMockLabels(ctrl),
		searchsvc.NewMockSearches(ctrl),
		config.App{Version: "test"},
		config.MCP{Enabled: false},
	)

	recorder := httptest.NewRecorder()
	edge.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, mcpserver.Path, nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("disabled mcp answered %d, want 404", recorder.Code)
	}
}
