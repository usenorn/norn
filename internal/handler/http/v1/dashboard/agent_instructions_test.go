package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
)

func send(t *testing.T, routes http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	request := httptest.NewRequest(method, path, &body)
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(identity.WithActor(request.Context(), entity.Actor{
		Kind:      entity.ActorKindUser,
		AccountID: uuid.New(),
	}))

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	return recorder
}

func TestInstructionsInTheRequestReachTheWorkspaceService(t *testing.T) {
	ctrl := gomock.NewController(t)
	workspaces := workspacesvc.NewMockWorkspaces(ctrl)
	routes := newEdge(ctrl, edgeServices{workspaces: workspaces})
	workspaceID := uuid.New()

	var captured service.UpdateWorkspaceInput

	workspaces.EXPECT().
		Update(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, input service.UpdateWorkspaceInput) (entity.Workspace, error) {
			captured = input

			return entity.Workspace{ID: id, AgentInstructions: "Ship small."}, nil
		})

	recorder := send(t, routes, http.MethodPatch, "/workspaces/"+workspaceID.String(), map[string]any{
		"agentInstructions": "Ship small.",
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	if captured.AgentInstructions == nil || *captured.AgentInstructions != "Ship small." {
		t.Fatalf(
			"the service was handed %v. A field the edge decodes but never passes on is saved "+
				"nowhere, and every test below the edge still passes.",
			captured.AgentInstructions,
		)
	}

	var answered map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decode answer: %v", err)
	}

	if answered["agentInstructions"] != "Ship small." {
		t.Errorf("the answer carried %v, want the instructions back", answered["agentInstructions"])
	}
}

func TestInstructionsInTheRequestReachTheProjectService(t *testing.T) {
	ctrl := gomock.NewController(t)
	projects := projectsvc.NewMockProjects(ctrl)
	routes := newEdge(ctrl, edgeServices{projects: projects})
	workspaceID, projectID := uuid.New(), uuid.New()

	var captured service.UpdateProjectInput

	projects.EXPECT().
		Update(gomock.Any(), workspaceID, projectID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, id uuid.UUID,
			input service.UpdateProjectInput,
		) (service.ProjectView, error) {
			captured = input

			return service.ProjectView{Project: entity.Project{
				ID:                id,
				WorkspaceID:       workspaceID,
				Slug:              "checkout-rebuild",
				Name:              "Checkout rebuild",
				State:             entity.ProjectStateActive,
				AgentInstructions: "Touch the ledger only.",
			}}, nil
		})

	recorder := send(
		t,
		routes,
		http.MethodPatch,
		"/workspaces/"+workspaceID.String()+"/projects/"+projectID.String(),
		map[string]any{"agentInstructions": "Touch the ledger only."},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	if captured.AgentInstructions == nil || *captured.AgentInstructions != "Touch the ledger only." {
		t.Fatalf("the service was handed %v, want the instructions in the body", captured.AgentInstructions)
	}
}

func TestInstructionsInTheRequestReachTheAgentService(t *testing.T) {
	ctrl := gomock.NewController(t)
	agents := agentsvc.NewMockAgents(ctrl)
	routes := newEdge(ctrl, edgeServices{agents: agents})
	workspaceID, agentID := uuid.New(), uuid.New()

	var captured service.SetAgentInstructionsInput

	agents.EXPECT().
		SetInstructions(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.SetAgentInstructionsInput) (service.OwnedAgent, error) {
			captured = input

			return service.OwnedAgent{Agent: entity.Agent{
				ID:                agentID,
				WorkspaceID:       workspaceID,
				Name:              "triage-bot",
				AgentInstructions: "Ask before deleting.",
			}}, nil
		})

	recorder := send(
		t,
		routes,
		http.MethodPut,
		"/workspaces/"+workspaceID.String()+"/agents/"+agentID.String()+"/instructions",
		map[string]any{"instructions": "Ask before deleting."},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	if captured.WorkspaceID != workspaceID || captured.AgentID != agentID ||
		captured.Instructions != "Ask before deleting." {
		t.Fatalf("the service was handed %+v, want the agent named in the path and the body", captured)
	}
}
