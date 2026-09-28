package hostedagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type conversations struct {
	agents     repository.Agent
	tokens     repository.APIToken
	workspaces repository.Workspace
	projects   repository.Project
	providers  repository.AIProvider
	models     repository.AIModel
	authorizer service.Authorizer
	tools      service.NornTools
	limits     config.AgentHosting
}

func New(
	agents repository.Agent,
	tokens repository.APIToken,
	workspaces repository.Workspace,
	projects repository.Project,
	providers repository.AIProvider,
	models repository.AIModel,
	authorizer service.Authorizer,
	tools service.NornTools,
	limits config.AgentHosting,
) service.HostedAgents {
	return &conversations{
		agents:     agents,
		tokens:     tokens,
		workspaces: workspaces,
		projects:   projects,
		providers:  providers,
		models:     models,
		authorizer: authorizer,
		tools:      tools,
		limits:     limits,
	}
}

type conversation struct {
	endpoint entity.AIProviderEndpoint
	apiKey   string
	request  entity.AIConversationRequest
}

func (s *conversations) Converse(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	turns []entity.AgentTurn,
) (entity.AgentReply, error) {
	return s.converse(ctx, workspaceID, agentID, turns, entity.ActorKindUser)
}

func (s *conversations) Chat(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	turns []entity.AgentTurn,
) (entity.AgentReply, error) {
	return s.converse(ctx, workspaceID, agentID, turns, entity.ActorKindUser, entity.ActorKindToken)
}

func (s *conversations) converse(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	turns []entity.AgentTurn,
	speakers ...entity.ActorKind,
) (entity.AgentReply, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.AgentReply{}, err
	}

	if !slices.Contains(speakers, decision.Actor.Kind) {
		return entity.AgentReply{}, entity.ErrAccountForbidden
	}

	if err := entity.NewValidationError(entity.ValidateAgentConversation("turns", turns)...); err != nil {
		return entity.AgentReply{}, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return entity.AgentReply{}, err
	}

	if !agent.ManageableBy(decision.Actor.Authority(), decision.Role) {
		return entity.AgentReply{}, entity.ErrAgentNotFound
	}

	if agent.Disabled() {
		return entity.AgentReply{}, entity.ErrAgentDisabled
	}

	if !agent.Hosted() {
		return entity.AgentReply{}, entity.ErrAgentNotHosted
	}

	acting, err := s.actingAs(ctx, agent)
	if err != nil {
		return entity.AgentReply{}, err
	}

	prepared, err := s.prepare(ctx, agent, turns)
	if err != nil {
		return entity.AgentReply{}, err
	}

	bounded, cancel := context.WithTimeout(identity.WithActor(ctx, acting), s.limits.Timeout)
	defer cancel()

	reply, err := s.run(bounded, prepared)
	if err != nil && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		reply.Stop = entity.AgentConversationTimeLimit

		return reply, nil
	}

	return reply, err
}

func (s *conversations) actingAs(ctx context.Context, agent entity.Agent) (entity.Actor, error) {
	token, err := s.tokens.GetLatestByOwner(ctx, agent.AccountID)
	if err != nil {
		if errors.Is(err, entity.ErrAPITokenNotFound) {
			return entity.Actor{}, entity.ErrAgentAuthorityMissing
		}

		return entity.Actor{}, err
	}

	return agent.ActingWith(token, time.Now().UTC())
}

func (s *conversations) prepare(
	ctx context.Context,
	agent entity.Agent,
	turns []entity.AgentTurn,
) (conversation, error) {
	connection, err := s.providers.Get(ctx, agent.WorkspaceID)
	if err != nil {
		return conversation{}, err
	}

	apiKey, err := s.providers.APIKey(ctx, agent.WorkspaceID)
	if err != nil {
		return conversation{}, err
	}

	workspace, err := s.workspaces.GetByID(ctx, agent.WorkspaceID)
	if err != nil {
		return conversation{}, err
	}

	var projectInstructions string

	if agent.ProjectID != nil {
		project, err := s.projects.GetByID(ctx, agent.WorkspaceID, *agent.ProjectID)
		if err != nil {
			return conversation{}, err
		}

		projectInstructions = project.AgentInstructions
	}

	items := make([]entity.AIConversationItem, 0, len(turns))

	for _, turn := range turns {
		items = append(items, entity.AIMessage(turn))
	}

	return conversation{
		endpoint: connection.Endpoint,
		apiKey:   apiKey,
		request: entity.AIConversationRequest{
			Model: connection.DefaultModel,
			Instructions: strings.Join([]string{
				s.tools.Instructions(),
				entity.ComposeAgentInstructions(
					workspace.AgentInstructions,
					projectInstructions,
					agent.AgentInstructions,
				),
			}, "\n\n"),
			Items: items,
			Tools: s.tools.Catalog(),
		},
	}, nil
}

func (s *conversations) run(ctx context.Context, prepared conversation) (entity.AgentReply, error) {
	var reply entity.AgentReply

	request := prepared.request

	for round := 1; ; round++ {
		remaining := s.limits.MaxTotalTokens - reply.Usage.Total()
		if remaining < entity.AIMinOutputTokens {
			reply.Stop = entity.AgentConversationTokenLimit

			return reply, nil
		}

		request.MaxOutputTokens = min(s.limits.MaxOutputTokens, remaining)

		step, err := s.models.Converse(ctx, prepared.endpoint, prepared.apiKey, request)
		if err != nil {
			return reply, err
		}

		reply.Usage = reply.Usage.Add(step.Usage)

		if step.Text != "" {
			reply.Text = step.Text
		}

		if step.Truncated {
			reply.Stop = entity.AgentConversationTokenLimit

			return reply, nil
		}

		if len(step.Calls) == 0 {
			reply.Stop = entity.AgentConversationAnswered

			return reply, nil
		}

		if round >= s.limits.MaxToolRounds {
			reply.Stop = entity.AgentConversationRoundLimit

			return reply, nil
		}

		if reply.Usage.Total() >= s.limits.MaxTotalTokens {
			reply.Stop = entity.AgentConversationTokenLimit

			return reply, nil
		}

		for _, call := range step.Calls {
			outcome, err := s.tools.Call(ctx, call.Name, json.RawMessage(call.Arguments))
			if err != nil {
				return reply, fmt.Errorf("call %s: %w", call.Name, err)
			}

			reply.ToolCalls = append(reply.ToolCalls, entity.AgentToolCall{Name: call.Name, Refusal: outcome.Refusal})
			request.Items = append(
				request.Items,
				entity.AIToolCallItem(call),
				entity.AIToolOutputItem(call.CallID, s.bounded(outcome.Output)),
			)
		}
	}
}

func (s *conversations) bounded(output json.RawMessage) string {
	if len(output) <= s.limits.MaxToolResultBytes {
		return string(output)
	}

	truncated, err := json.Marshal(struct {
		Truncated bool   `json:"truncated"`
		Partial   string `json:"partial"`
	}{
		Truncated: true,
		Partial:   strings.ToValidUTF8(string(output[:s.limits.MaxToolResultBytes]), ""),
	})
	if err != nil {
		return string(output[:s.limits.MaxToolResultBytes])
	}

	return string(truncated)
}
