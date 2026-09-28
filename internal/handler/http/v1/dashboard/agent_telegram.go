package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceAgentTelegram(
	ctx context.Context,
	request api.GetWorkspaceAgentTelegramRequestObject,
) (api.GetWorkspaceAgentTelegramResponseObject, error) {
	view, err := h.telegramBots.Get(ctx, request.WorkspaceId, request.AgentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceAgentTelegram200JSONResponse(agentTelegramDTO(view)), nil
}

func (h *handler) ConnectWorkspaceAgentTelegram(
	ctx context.Context,
	request api.ConnectWorkspaceAgentTelegramRequestObject,
) (api.ConnectWorkspaceAgentTelegramResponseObject, error) {
	view, err := h.telegramBots.Connect(ctx, request.WorkspaceId, request.AgentId, request.Body.Token)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConnectWorkspaceAgentTelegram200JSONResponse(agentTelegramDTO(view)), nil
}

func (h *handler) DisconnectWorkspaceAgentTelegram(
	ctx context.Context,
	request api.DisconnectWorkspaceAgentTelegramRequestObject,
) (api.DisconnectWorkspaceAgentTelegramResponseObject, error) {
	if err := h.telegramBots.Disconnect(ctx, request.WorkspaceId, request.AgentId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisconnectWorkspaceAgentTelegram204Response{}, nil
}

func (h *handler) CreateWorkspaceAgentTelegramLink(
	ctx context.Context,
	request api.CreateWorkspaceAgentTelegramLinkRequestObject,
) (api.CreateWorkspaceAgentTelegramLinkResponseObject, error) {
	invite, err := h.telegramBots.IssueLink(
		ctx, request.WorkspaceId, request.AgentId, entity.TelegramLinkPurpose(request.Body.Purpose),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceAgentTelegramLink201JSONResponse{Url: invite.URL, ExpiresAt: invite.ExpiresAt}, nil
}

func (h *handler) UnlinkWorkspaceAgentTelegram(
	ctx context.Context,
	request api.UnlinkWorkspaceAgentTelegramRequestObject,
) (api.UnlinkWorkspaceAgentTelegramResponseObject, error) {
	if err := h.telegramBots.Unlink(ctx, request.WorkspaceId, request.AgentId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnlinkWorkspaceAgentTelegram204Response{}, nil
}

func (h *handler) UnbindWorkspaceAgentTelegramGroup(
	ctx context.Context,
	request api.UnbindWorkspaceAgentTelegramGroupRequestObject,
) (api.UnbindWorkspaceAgentTelegramGroupResponseObject, error) {
	if err := h.telegramBots.Unbind(ctx, request.WorkspaceId, request.AgentId, request.GroupId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnbindWorkspaceAgentTelegramGroup204Response{}, nil
}

func (h *handler) ListWorkspaceTelegramBots(
	ctx context.Context,
	request api.ListWorkspaceTelegramBotsRequestObject,
) (api.ListWorkspaceTelegramBotsResponseObject, error) {
	bots, err := h.telegramBots.Mine(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	listed := make([]api.MemberTelegramBot, 0, len(bots))

	for _, bot := range bots {
		listed = append(listed, api.MemberTelegramBot{
			AgentId:   bot.Bot.AgentID,
			AgentName: bot.AgentName,
			Username:  bot.Bot.Username,
			Linked:    bot.Linked,
			LinkedAs:  nilIfEmpty(bot.LinkedAs),
		})
	}

	return api.ListWorkspaceTelegramBots200JSONResponse{Bots: listed}, nil
}

func agentTelegramDTO(view service.TelegramBotView) api.AgentTelegramBot {
	members := make([]api.TelegramMember, 0, len(view.Accounts))

	for _, account := range view.Accounts {
		members = append(members, api.TelegramMember{
			AccountId: account.AccountID,
			Name:      account.AccountName,
			Username:  account.Username,
			LinkedAt:  account.LinkedAt,
		})
	}

	groups := make([]api.TelegramGroup, 0, len(view.Groups))

	for _, group := range view.Groups {
		groups = append(groups, api.TelegramGroup{
			Id:      group.ID,
			Title:   group.Title,
			BoundBy: nilIfEmpty(group.BoundByName),
			BoundAt: group.BoundAt,
		})
	}

	return api.AgentTelegramBot{
		Id:          view.Bot.ID,
		Username:    view.Bot.Username,
		Name:        view.Bot.Name,
		TokenHint:   view.Bot.TokenHint,
		Hosted:      view.Hosted,
		Linked:      view.Linked,
		ConnectedBy: nilIfEmpty(view.Bot.ConnectedByName),
		ConnectedAt: view.Bot.ConnectedAt,
		Members:     members,
		Groups:      groups,
	}
}
