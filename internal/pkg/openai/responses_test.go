package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestRespondCarriesTheConversationAndTheToolsTheAgentMayUse(t *testing.T) {
	var sent map[string]any

	client := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/responses" {
			t.Errorf("request = %s %s, want POST /responses", request.Method, request.URL.Path)
		}

		if err := json.NewDecoder(request.Body).Decode(&sent); err != nil {
			t.Errorf("decode request: %v", err)
		}

		_, _ = writer.Write([]byte(`{"status":"completed","output":[
			{"type":"reasoning","summary":[]},
			{"type":"function_call","call_id":"call_2","name":"norn_get_issue","arguments":"{\"issue\":\"NORN-7\"}"}
		],"usage":{"input_tokens":120,"output_tokens":18}}`))
	})

	step, err := client.Respond(context.Background(), entity.AIProviderEndpoint{}, "sk-test", entity.AIConversationRequest{
		Model:        "gpt-6-luna",
		Instructions: "Answer from Norn data.",
		Items: []entity.AIConversationItem{
			entity.AIMessage(entity.AgentTurn{Role: entity.AgentTurnUser, Text: "What is NORN-7 about?"}),
			entity.AIToolCallItem(entity.AIToolCall{CallID: "call_1", Name: "norn_search", Arguments: `{"query":"NORN-7"}`}),
			entity.AIToolOutputItem("call_1", `{"results":[]}`),
		},
		Tools: []entity.AIToolDefinition{{
			Name:        "norn_get_issue",
			Description: "Fetch one issue.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"issue":{"type":"string"}}}`),
		}},
		MaxOutputTokens: 2048,
	})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}

	if sent["store"] != false {
		t.Errorf("store = %v; a workspace's conversation must not be kept on the provider's side", sent["store"])
	}

	input, _ := sent["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input = %v, want the question, the earlier call and its output in order", input)
	}

	for i, want := range []string{"message", "function_call", "function_call_output"} {
		if item, _ := input[i].(map[string]any); item["type"] != want {
			t.Errorf("input[%d] type = %v, want %s", i, item["type"], want)
		}
	}

	if output, _ := input[2].(map[string]any); output["call_id"] != "call_1" {
		t.Errorf("tool output answers call %v, want call_1", output["call_id"])
	}

	tools, _ := sent["tools"].([]any)
	if tool, _ := tools[0].(map[string]any); len(tools) != 1 || tool["type"] != "function" || tool["name"] != "norn_get_issue" {
		t.Errorf("tools = %v, want norn_get_issue offered as a function", tools)
	}

	if len(step.Calls) != 1 || step.Calls[0].CallID != "call_2" || step.Calls[0].Name != "norn_get_issue" {
		t.Errorf("calls = %+v, want the one function call the model made", step.Calls)
	}

	if step.Usage != (entity.AITokenUsage{Input: 120, Output: 18}) {
		t.Errorf("usage = %+v, want what the provider billed", step.Usage)
	}
}

func TestRespondJoinsTheTextOfTheAnswer(t *testing.T) {
	client := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","content":[
			{"type":"output_text","text":"Three issues "},{"type":"output_text","text":"are in progress."}
		]}],"usage":{"input_tokens":10,"output_tokens":5}}`))
	})

	step, err := client.Respond(context.Background(), entity.AIProviderEndpoint{}, "sk-test", entity.AIConversationRequest{
		Model: "gpt-6-luna",
		Items: []entity.AIConversationItem{
			entity.AIMessage(entity.AgentTurn{Role: entity.AgentTurnUser, Text: "What is in progress?"}),
		},
	})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}

	if step.Text != "Three issues are in progress." || len(step.Calls) != 0 {
		t.Errorf("step = %+v, want the whole answer and no calls", step)
	}
}

func TestRespondReportsAProviderRefusalAsTheAdministratorsVerdict(t *testing.T) {
	client := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"message":"quota","code":"insufficient_quota"}}`))
	})

	_, err := client.Respond(context.Background(), entity.AIProviderEndpoint{}, "sk-test", entity.AIConversationRequest{
		Model: "gpt-6-luna",
	})
	if !errors.Is(err, entity.ErrAIProviderQuotaExceeded) {
		t.Errorf("err = %v, want ErrAIProviderQuotaExceeded", err)
	}
}
