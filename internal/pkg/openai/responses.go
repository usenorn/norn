package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/entity"
)

const (
	functionTool      = "function"
	outputText        = "output_text"
	outputRefusal     = "refusal"
	outputMessageType = "message"
)

type responseRequest struct {
	Model           string          `json:"model"`
	Instructions    string          `json:"instructions,omitempty"`
	Input           []responseInput `json:"input"`
	Tools           []responseTool  `json:"tools,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Store           bool            `json:"store"`
}

type responseInput struct {
	Type      string `json:"type"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responseTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

type responseBody struct {
	Output []struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Content   []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *Client) Respond(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	apiKey string,
	request entity.AIConversationRequest,
) (entity.AIConversationStep, error) {
	payload, err := json.Marshal(responseRequestFrom(request))
	if err != nil {
		return entity.AIConversationStep{}, fmt.Errorf("encode response request: %w", err)
	}

	body, err := c.call(ctx, endpoint, http.MethodPost, responsesPath, apiKey, payload)
	if err != nil {
		return entity.AIConversationStep{}, err
	}

	var answered responseBody
	if err := json.Unmarshal(body, &answered); err != nil {
		return entity.AIConversationStep{}, fmt.Errorf(
			"%w: the response was not JSON: %v", entity.ErrAIProviderUnreachable, err,
		)
	}

	return stepFrom(answered), nil
}

func responseRequestFrom(request entity.AIConversationRequest) responseRequest {
	input := make([]responseInput, 0, len(request.Items))

	for _, item := range request.Items {
		switch item.Kind {
		case entity.AIConversationToolCall:
			input = append(input, responseInput{
				Type:      string(item.Kind),
				CallID:    item.Call.CallID,
				Name:      item.Call.Name,
				Arguments: item.Call.Arguments,
			})
		case entity.AIConversationToolOutput:
			input = append(input, responseInput{
				Type:   string(item.Kind),
				CallID: item.Call.CallID,
				Output: item.Output,
			})
		default:
			input = append(input, responseInput{
				Type:    outputMessageType,
				Role:    string(item.Role),
				Content: item.Text,
			})
		}
	}

	tools := make([]responseTool, 0, len(request.Tools))

	for _, tool := range request.Tools {
		tools = append(tools, responseTool{
			Type:        functionTool,
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	return responseRequest{
		Model:           request.Model,
		Instructions:    request.Instructions,
		Input:           input,
		Tools:           tools,
		MaxOutputTokens: request.MaxOutputTokens,
	}
}

func stepFrom(answered responseBody) entity.AIConversationStep {
	var text strings.Builder

	step := entity.AIConversationStep{
		Usage: entity.AITokenUsage{
			Input:  answered.Usage.InputTokens,
			Output: answered.Usage.OutputTokens,
		},
	}

	for _, item := range answered.Output {
		switch item.Type {
		case string(entity.AIConversationToolCall):
			step.Calls = append(step.Calls, entity.AIToolCall{
				CallID:    item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})
		case outputMessageType:
			for _, part := range item.Content {
				switch part.Type {
				case outputText:
					text.WriteString(part.Text)
				case outputRefusal:
					text.WriteString(part.Refusal)
				}
			}
		}
	}

	step.Text = strings.TrimSpace(text.String())

	return step
}
