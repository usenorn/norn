package entity

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	AgentTurnMaxLen           = 8000
	AgentConversationMaxTurns = 40
)

type AgentTurnRole string

const (
	AgentTurnUser      AgentTurnRole = "user"
	AgentTurnAssistant AgentTurnRole = "assistant"
)

func (r AgentTurnRole) Valid() bool {
	return r == AgentTurnUser || r == AgentTurnAssistant
}

type AgentTurn struct {
	Role AgentTurnRole
	Text string
}

func ValidateAgentConversation(field string, turns []AgentTurn) []FieldError {
	switch {
	case len(turns) == 0:
		return []FieldError{{Field: field, Code: ValidationCodeRequired}}
	case len(turns) > AgentConversationMaxTurns:
		return []FieldError{{Field: field, Code: ValidationCodeTooLong}}
	}

	var failures []FieldError

	for i, turn := range turns {
		at := field + "[" + strconv.Itoa(i) + "]"

		switch {
		case !turn.Role.Valid():
			failures = append(failures, FieldError{Field: at + ".role", Code: ValidationCodeUnsupportedValue})
		case strings.TrimSpace(turn.Text) == "":
			failures = append(failures, FieldError{Field: at + ".text", Code: ValidationCodeRequired})
		case utf8.RuneCountInString(turn.Text) > AgentTurnMaxLen:
			failures = append(failures, FieldError{Field: at + ".text", Code: ValidationCodeTooLong})
		}
	}

	if last := turns[len(turns)-1]; last.Role != AgentTurnUser {
		failures = append(failures, FieldError{
			Field: field + "[" + strconv.Itoa(len(turns)-1) + "].role",
			Code:  ValidationCodeUnsupportedValue,
		})
	}

	return failures
}

type AgentConversationStop string

const (
	AgentConversationAnswered   AgentConversationStop = "answered"
	AgentConversationRoundLimit AgentConversationStop = "round_limit"
	AgentConversationTokenLimit AgentConversationStop = "token_limit"
	AgentConversationTimeLimit  AgentConversationStop = "time_limit"
)

type AITokenUsage struct {
	Input  int
	Output int
}

func (u AITokenUsage) Add(other AITokenUsage) AITokenUsage {
	return AITokenUsage{Input: u.Input + other.Input, Output: u.Output + other.Output}
}

func (u AITokenUsage) Total() int {
	return u.Input + u.Output
}

type AgentToolCall struct {
	Name    string
	Refusal string
}

type AgentReply struct {
	Text      string
	ToolCalls []AgentToolCall
	Usage     AITokenUsage
	Stop      AgentConversationStop
}

type AIToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type AIToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

type AIToolOutcome struct {
	Output  json.RawMessage
	Refusal string
}

type AIConversationItemKind string

const (
	AIConversationMessage    AIConversationItemKind = "message"
	AIConversationToolCall   AIConversationItemKind = "function_call"
	AIConversationToolOutput AIConversationItemKind = "function_call_output"
)

type AIConversationItem struct {
	Kind   AIConversationItemKind
	Role   AgentTurnRole
	Text   string
	Call   AIToolCall
	Output string
}

func AIMessage(turn AgentTurn) AIConversationItem {
	return AIConversationItem{Kind: AIConversationMessage, Role: turn.Role, Text: turn.Text}
}

func AIToolCallItem(call AIToolCall) AIConversationItem {
	return AIConversationItem{Kind: AIConversationToolCall, Call: call}
}

func AIToolOutputItem(callID, output string) AIConversationItem {
	return AIConversationItem{Kind: AIConversationToolOutput, Call: AIToolCall{CallID: callID}, Output: output}
}

type AIConversationRequest struct {
	Model           string
	Instructions    string
	Items           []AIConversationItem
	Tools           []AIToolDefinition
	MaxOutputTokens int
}

type AIConversationStep struct {
	Text  string
	Calls []AIToolCall
	Usage AITokenUsage
}
