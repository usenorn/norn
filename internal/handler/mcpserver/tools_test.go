package mcpserver_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func TestAHostedToolCallActsAsTheAgentOnItsContext(t *testing.T) {
	h := newHarness(t)

	agentID := uuid.New()
	agent := entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      uuid.New(),
		AgentID:        &agentID,
		OwnerAccountID: uuid.New(),
		Scopes:         toolScopes,
	}

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), agent.AccountID).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID) ([]entity.Workspace, error) {
			acting, ok := identity.Actor(ctx)
			if !ok || acting.Kind != entity.ActorKindAgent || acting.AgentID == nil || *acting.AgentID != agentID {
				t.Errorf(
					"the service saw %+v; a hosted call must reach it as the agent, or the agent's "+
						"scopes, grants and holds are never applied",
					acting,
				)
			}

			return []entity.Workspace{{ID: uuid.New(), Slug: "acme", Name: "Acme"}}, nil
		})

	outcome, err := h.tools.Call(identity.WithActor(t.Context(), agent), "norn_list_workspaces", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if outcome.Refusal != "" || !strings.Contains(string(outcome.Output), "acme") {
		t.Errorf("outcome = %s (refusal %q), want the workspace listed", outcome.Output, outcome.Refusal)
	}
}

func TestARefusedHostedToolCallTellsTheModelWhyInsteadOfFailing(t *testing.T) {
	h := newHarness(t)
	ctx := identity.WithActor(t.Context(), h.actor)

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), gomock.Any()).
		Return(nil, entity.ErrAccountForbidden)

	cases := []struct {
		name      string
		tool      string
		arguments string
		refusal   string
	}{
		{name: "a permission the agent lacks", tool: "norn_list_workspaces", arguments: `{}`, refusal: "permission_denied"},
		{name: "a tool that does not exist", tool: "norn_drop_database", arguments: `{}`, refusal: "unknown_tool"},
		{name: "arguments of the wrong shape", tool: "norn_get_issue", arguments: `{"issue":42}`, refusal: "invalid_input"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := h.tools.Call(ctx, tc.tool, json.RawMessage(tc.arguments))
			if err != nil {
				t.Fatalf("Call: %v; a refusal must reach the model as an answer, not end the conversation", err)
			}

			var said struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}

			if err := json.Unmarshal(outcome.Output, &said); err != nil {
				t.Fatalf("refusal %s is not JSON: %v", outcome.Output, err)
			}

			if outcome.Refusal != tc.refusal || said.Error != tc.refusal || said.Message == "" {
				t.Errorf("outcome = %s (refusal %q), want %s with a reason", outcome.Output, outcome.Refusal, tc.refusal)
			}
		})
	}
}

func TestAHostedAgentIsOfferedExactlyTheToolsMCPAdvertises(t *testing.T) {
	h := newHarness(t)

	listed, err := h.session(t).ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	advertised := make(map[string]any, len(listed.Tools))

	for _, tool := range listed.Tools {
		advertised[tool.Name] = decoded(t, tool.InputSchema)
	}

	catalog := h.tools.Catalog()
	if len(catalog) != len(advertised) {
		t.Fatalf("hosted catalog has %d tools, MCP advertises %d", len(catalog), len(advertised))
	}

	for _, tool := range catalog {
		schema, ok := advertised[tool.Name]
		if !ok {
			t.Errorf("%s is offered to hosted agents but not over MCP", tool.Name)

			continue
		}

		if offered := decoded(t, tool.Parameters); !reflect.DeepEqual(offered, schema) {
			t.Errorf("%s input differs between hosted and MCP:\n%v\nvs\n%v", tool.Name, offered, schema)
		}
	}
}

func decoded(t *testing.T, schema any) any {
	t.Helper()

	encoded, ok := schema.(json.RawMessage)
	if !ok {
		var err error

		encoded, err = json.Marshal(schema)
		if err != nil {
			t.Fatalf("encode schema: %v", err)
		}
	}

	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatalf("decode schema: %v", err)
	}

	return value
}
