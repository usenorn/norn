package dashboard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	hostedagentsvc "github.com/usenorn/norn/internal/service/hostedagent"
)

func TestTheChosenExecutionReachesTheAgentService(t *testing.T) {
	ctrl := gomock.NewController(t)
	agents := agentsvc.NewMockAgents(ctrl)
	routes := newEdge(ctrl, edgeServices{agents: agents})
	workspaceID, agentID := uuid.New(), uuid.New()

	agents.EXPECT().
		SetExecution(gomock.Any(), service.SetAgentExecutionInput{
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Execution:   entity.AgentExecutionHosted,
		}).
		Return(service.OwnedAgent{Agent: entity.Agent{ID: agentID, Execution: entity.AgentExecutionHosted}}, nil)

	recorder := send(t, routes, http.MethodPut,
		fmt.Sprintf("/workspaces/%s/agents/%s/execution", workspaceID, agentID),
		map[string]any{"execution": "hosted"},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var answered struct {
		Agent struct {
			Execution string `json:"execution"`
		} `json:"agent"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil || answered.Agent.Execution != "hosted" {
		t.Errorf("answer = %s, want the agent back as hosted", recorder.Body.String())
	}
}

func TestAConversationReachesTheHostedAgentAndItsAnswerComesBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	hosted := hostedagentsvc.NewMockHostedAgents(ctrl)
	routes := newEdge(ctrl, edgeServices{hostedAgents: hosted})
	workspaceID, agentID := uuid.New(), uuid.New()

	hosted.EXPECT().
		Converse(gomock.Any(), workspaceID, agentID, []entity.AgentTurn{
			{Role: entity.AgentTurnUser, Text: "What is in progress?"},
			{Role: entity.AgentTurnAssistant, Text: "NORN-7."},
			{Role: entity.AgentTurnUser, Text: "Who owns it?"},
		}).
		Return(entity.AgentReply{
			Text: "Rae does.",
			ToolCalls: []entity.AgentToolCall{
				{Name: "norn_get_issue"},
				{Name: "norn_list_workspace_members", Refusal: "permission_denied"},
			},
			Usage: entity.AITokenUsage{Input: 300, Output: 12},
			Stop:  entity.AgentConversationAnswered,
		}, nil)

	recorder := send(t, routes, http.MethodPost,
		fmt.Sprintf("/workspaces/%s/agents/%s/conversation", workspaceID, agentID),
		map[string]any{"turns": []map[string]string{
			{"role": "user", "text": "What is in progress?"},
			{"role": "assistant", "text": "NORN-7."},
			{"role": "user", "text": "Who owns it?"},
		}},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var answered struct {
		Text      string `json:"text"`
		Stop      string `json:"stop"`
		ToolCalls []struct {
			Name    string  `json:"name"`
			Refusal *string `json:"refusal"`
		} `json:"toolCalls"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decode answer: %v", err)
	}

	if answered.Text != "Rae does." || answered.Stop != "answered" || answered.Usage.InputTokens != 300 {
		t.Errorf("answer = %s, want the reply as the service gave it", recorder.Body.String())
	}

	if len(answered.ToolCalls) != 2 || answered.ToolCalls[0].Refusal != nil ||
		answered.ToolCalls[1].Refusal == nil || *answered.ToolCalls[1].Refusal != "permission_denied" {
		t.Errorf("tool calls = %s, want the refusal on the second call only", recorder.Body.String())
	}
}

func TestAConversationRefusalNamesWhatTheAdministratorCanFix(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{err: entity.ErrAIProviderNotConfigured, status: http.StatusConflict, code: "ai_provider_not_configured"},
		{err: entity.ErrAgentNotHosted, status: http.StatusConflict, code: "agent_not_hosted"},
		{err: entity.ErrAgentAuthorityMissing, status: http.StatusConflict, code: "agent_authority_missing"},
		{
			err:    fmt.Errorf("%w: quota", entity.ErrAIProviderQuotaExceeded),
			status: http.StatusUnprocessableEntity,
			code:   "quota_exceeded",
		},
		{err: entity.ErrAIProviderEncryptionKeyMissing, status: http.StatusServiceUnavailable, code: "ai_provider_sealing_unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			hosted := hostedagentsvc.NewMockHostedAgents(ctrl)
			routes := newEdge(ctrl, edgeServices{hostedAgents: hosted})

			hosted.EXPECT().
				Converse(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID, []entity.AgentTurn) (entity.AgentReply, error) {
					return entity.AgentReply{}, tc.err
				})

			recorder := send(t, routes, http.MethodPost,
				fmt.Sprintf("/workspaces/%s/agents/%s/conversation", uuid.New(), uuid.New()),
				map[string]any{"turns": []map[string]string{{"role": "user", "text": "Hello?"}}},
			)

			var problem struct {
				Code string `json:"code"`
			}

			_ = json.Unmarshal(recorder.Body.Bytes(), &problem)

			if recorder.Code != tc.status || problem.Code != tc.code {
				t.Errorf("answered %d %s, want %d with code %s", recorder.Code, recorder.Body.String(), tc.status, tc.code)
			}
		})
	}
}
