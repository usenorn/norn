package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const hostedInstructions = "You are an agent of this Norn workspace, answering a person in " +
	"conversation. Answer from what the Norn tools return, name the issues, projects and " +
	"people you draw on, and say so plainly when a tool refuses or finds nothing rather than " +
	"guessing. A refusal such as permission_denied is the limit of what this agent may see or " +
	"do; report it, never work around it.\n\n"

type hostedTool struct {
	definition entity.AIToolDefinition
	call       func(ctx context.Context, arguments json.RawMessage) (any, error)
}

type Tools struct {
	byName   map[string]hostedTool
	catalog  []entity.AIToolDefinition
	installs []func(*mcp.Server)
	failure  error
}

var _ service.NornTools = (*Tools)(nil)

func NewTools(
	issues service.Issues,
	relations service.IssueRelations,
	questions service.IssueQuestions,
	agents service.Agents,
	issueComments service.IssueComments,
	projects service.Projects,
	cycles service.Cycles,
	teams service.Teams,
	workspaces service.Workspaces,
	workflowStates service.WorkflowStates,
	labels service.Labels,
	searches service.Searches,
	sourceControl service.SourceControl,
) (*Tools, error) {
	set := &toolset{
		issues:         issues,
		relations:      relations,
		questions:      questions,
		agents:         agents,
		issueComments:  issueComments,
		projects:       projects,
		cycles:         cycles,
		teams:          teams,
		workspaces:     workspaces,
		workflowStates: workflowStates,
		labels:         labels,
		searches:       searches,
		sourceControl:  sourceControl,
	}

	tools := &Tools{byName: map[string]hostedTool{}}
	set.register(tools)

	if tools.failure != nil {
		return nil, tools.failure
	}

	return tools, nil
}

func add[In, Out any](tools *Tools, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		tools.failure = errors.Join(tools.failure, fmt.Errorf("infer %s input: %w", tool.Name, err))

		return
	}

	parameters, err := json.Marshal(schema)
	if err != nil {
		tools.failure = errors.Join(tools.failure, fmt.Errorf("encode %s input: %w", tool.Name, err))

		return
	}

	described := *tool
	described.InputSchema = schema

	definition := entity.AIToolDefinition{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  parameters,
	}

	tools.catalog = append(tools.catalog, definition)
	tools.installs = append(tools.installs, func(server *mcp.Server) {
		mcp.AddTool(server, &described, handler)
	})
	tools.byName[tool.Name] = hostedTool{
		definition: definition,
		call: func(ctx context.Context, arguments json.RawMessage) (any, error) {
			var input In

			if len(arguments) > 0 {
				if err := json.Unmarshal(arguments, &input); err != nil {
					return nil, refusal("invalid_input", "the arguments do not match the tool's input: "+err.Error())
				}
			}

			_, output, err := handler(ctx, &mcp.CallToolRequest{}, input)

			return output, err
		},
	}
}

func (t *Tools) Catalog() []entity.AIToolDefinition {
	return t.catalog
}

func (t *Tools) Instructions() string {
	return hostedInstructions + untrustedContentInstructions
}

func (t *Tools) Call(
	ctx context.Context,
	name string,
	arguments json.RawMessage,
) (entity.AIToolOutcome, error) {
	tool, ok := t.byName[name]
	if !ok {
		return refused(toolError{code: "unknown_tool", message: "there is no Norn tool named " + name})
	}

	output, err := tool.call(ctx, arguments)
	if err != nil {
		var failed toolError
		if errors.As(toolFailure(ctx, err), &failed) {
			return refused(failed)
		}

		return entity.AIToolOutcome{}, err
	}

	encoded, err := json.Marshal(output)
	if err != nil {
		return entity.AIToolOutcome{}, fmt.Errorf("encode %s output: %w", name, err)
	}

	return entity.AIToolOutcome{Output: encoded}, nil
}

func refused(failure toolError) (entity.AIToolOutcome, error) {
	encoded, err := json.Marshal(map[string]string{"error": failure.code, "message": failure.message})
	if err != nil {
		return entity.AIToolOutcome{}, fmt.Errorf("encode refusal: %w", err)
	}

	return entity.AIToolOutcome{Output: encoded, Refusal: failure.code}, nil
}
