package telegrambot

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	httpsScheme = "https"
	updatesPath = "v1/telegram/bots"
	updatesLeaf = "updates"
)

type bots struct {
	bots       repository.TelegramBot
	audience   repository.TelegramAudience
	messenger  repository.TelegramMessenger
	agents     repository.Agent
	transactor repository.Transactor
	authorizer service.Authorizer
	audit      service.Audit
	app        config.App
	limits     config.Telegram
}

func NewBots(
	telegramBots repository.TelegramBot,
	audience repository.TelegramAudience,
	messenger repository.TelegramMessenger,
	agents repository.Agent,
	transactor repository.Transactor,
	authorizer service.Authorizer,
	audit service.Audit,
	app config.App,
	limits config.Telegram,
) service.TelegramBots {
	return &bots{
		bots:       telegramBots,
		audience:   audience,
		messenger:  messenger,
		agents:     agents,
		transactor: transactor,
		authorizer: authorizer,
		audit:      audit,
		app:        app,
		limits:     limits,
	}
}

func (s *bots) managed(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.Decision, entity.Agent, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.Decision{}, entity.Agent{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Decision{}, entity.Agent{}, entity.ErrAccountForbidden
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return entity.Decision{}, entity.Agent{}, err
	}

	if !agent.ManageableBy(decision.Actor.Authority(), decision.Role) {
		return entity.Decision{}, entity.Agent{}, entity.ErrAgentNotFound
	}

	return decision, agent, nil
}

func (s *bots) member(ctx context.Context, workspaceID uuid.UUID) (entity.Decision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return entity.Decision{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	return decision, nil
}

func (s *bots) Get(ctx context.Context, workspaceID, agentID uuid.UUID) (service.TelegramBotView, error) {
	decision, agent, err := s.managed(ctx, workspaceID, agentID)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	bot, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	return s.view(ctx, bot, agent, decision.Actor.AccountID)
}

func (s *bots) view(
	ctx context.Context,
	bot entity.TelegramBot,
	agent entity.Agent,
	viewer uuid.UUID,
) (service.TelegramBotView, error) {
	accounts, err := s.audience.Accounts(ctx, bot.ID)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	groups, err := s.audience.Groups(ctx, bot.ID)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	view := service.TelegramBotView{Bot: bot, Hosted: agent.Hosted(), Accounts: accounts, Groups: groups}

	for _, account := range accounts {
		if account.AccountID == viewer {
			view.Linked = true
		}
	}

	return view, nil
}

func (s *bots) Connect(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	token string,
) (service.TelegramBotView, error) {
	decision, agent, err := s.managed(ctx, workspaceID, agentID)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	token = strings.TrimSpace(token)

	if err := entity.NewValidationError(entity.ValidateTelegramBotToken("token", token)); err != nil {
		return service.TelegramBotView{}, err
	}

	if agent.Disabled() {
		return service.TelegramBotView{}, entity.ErrAgentDisabled
	}

	if !s.deliverable() {
		return service.TelegramBotView{}, entity.ErrTelegramOriginInsecure
	}

	identity, err := s.messenger.Identify(ctx, token)
	if err != nil {
		return service.TelegramBotView{}, err
	}

	holder, err := s.bots.GetByBotUser(ctx, identity.BotUserID)

	switch {
	case err == nil && holder.AgentID != agentID:
		return service.TelegramBotView{}, entity.ErrTelegramBotTaken
	case err != nil && !errors.Is(err, entity.ErrTelegramBotNotFound):
		return service.TelegramBotView{}, err
	}

	current, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil && !errors.Is(err, entity.ErrTelegramBotNotFound) {
		return service.TelegramBotView{}, err
	}

	connected := err == nil
	replaced := connected && current.BotUserID != identity.BotUserID

	var retiredToken string

	if replaced {
		if retiredToken, err = s.bots.Token(ctx, current.ID); err != nil {
			return service.TelegramBotView{}, err
		}
	}

	secret, secretHash, err := entity.NewTelegramWebhookSecret()
	if err != nil {
		return service.TelegramBotView{}, err
	}

	bot := entity.TelegramBot{
		ID:          current.ID,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		BotUserID:   identity.BotUserID,
		Username:    identity.Username,
		Name:        identity.Name,
		ConnectedBy: decision.Actor.AccountID,
	}

	var saved entity.TelegramBot

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		switch {
		case connected && !replaced:
			saved, err = s.bots.Rekey(ctx, bot, token, secretHash)
		case replaced:
			if err := s.bots.Delete(ctx, workspaceID, agentID); err != nil {
				return err
			}

			bot.ID = uuid.Nil
			saved, err = s.bots.Save(ctx, bot, token, secretHash)
		default:
			saved, err = s.bots.Save(ctx, bot, token, secretHash)
		}

		if err != nil {
			return err
		}

		return s.messenger.Register(ctx, token, s.updatesURL(saved.ID), secret)
	}); err != nil {
		return service.TelegramBotView{}, err
	}

	if replaced {
		s.retire(ctx, retiredToken)
	}

	s.record(ctx, saved, agent, entity.AuditAgentTelegramConnected, nil)

	return s.view(ctx, saved, agent, decision.Actor.AccountID)
}

func (s *bots) Disconnect(ctx context.Context, workspaceID, agentID uuid.UUID) error {
	_, agent, err := s.managed(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	bot, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	token, err := s.bots.Token(ctx, bot.ID)
	if err != nil && !errors.Is(err, entity.ErrTelegramEncryptionKeyMissing) {
		return err
	}

	if err := s.bots.Delete(ctx, workspaceID, agentID); err != nil {
		return err
	}

	if token != "" {
		s.retire(ctx, token)
	}

	s.record(ctx, bot, agent, entity.AuditAgentTelegramDisconnected, nil)

	return nil
}

func (s *bots) IssueLink(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	purpose entity.TelegramLinkPurpose,
) (service.TelegramLinkInvite, error) {
	if err := entity.NewValidationError(entity.ValidateTelegramLinkPurpose("purpose", purpose)); err != nil {
		return service.TelegramLinkInvite{}, err
	}

	var holder uuid.UUID

	if purpose == entity.TelegramLinkGroup {
		decision, _, err := s.managed(ctx, workspaceID, agentID)
		if err != nil {
			return service.TelegramLinkInvite{}, err
		}

		holder = decision.Actor.AccountID
	} else {
		decision, err := s.member(ctx, workspaceID)
		if err != nil {
			return service.TelegramLinkInvite{}, err
		}

		holder = decision.Actor.AccountID
	}

	bot, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil {
		return service.TelegramLinkInvite{}, err
	}

	code, hash, err := entity.NewTelegramLinkCode()
	if err != nil {
		return service.TelegramLinkInvite{}, err
	}

	expiresAt := time.Now().UTC().Add(s.limits.LinkTTL)

	if err := s.audience.IssueCode(ctx, entity.TelegramLinkCode{
		BotID:     bot.ID,
		AccountID: holder,
		Purpose:   purpose,
		ExpiresAt: expiresAt,
	}, hash); err != nil {
		return service.TelegramLinkInvite{}, err
	}

	return service.TelegramLinkInvite{URL: bot.DeepLink(purpose, code), ExpiresAt: expiresAt}, nil
}

func (s *bots) Unlink(ctx context.Context, workspaceID, agentID uuid.UUID) error {
	decision, err := s.member(ctx, workspaceID)
	if err != nil {
		return err
	}

	bot, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	return s.audience.Unlink(ctx, bot.ID, decision.Actor.AccountID)
}

func (s *bots) Unbind(ctx context.Context, workspaceID, agentID, groupID uuid.UUID) error {
	_, agent, err := s.managed(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	bot, err := s.bots.Get(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	group, err := s.audience.Unbind(ctx, bot.ID, groupID)
	if err != nil {
		return err
	}

	s.record(ctx, bot, agent, entity.AuditAgentTelegramGroupUnbound, map[string]string{"group": group.Title})

	return nil
}

func (s *bots) Mine(ctx context.Context, workspaceID uuid.UUID) ([]service.MemberTelegramBot, error) {
	decision, err := s.member(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	connected, err := s.bots.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	mine := make([]service.MemberTelegramBot, 0, len(connected))

	for _, bot := range connected {
		agent, err := s.agents.GetByID(ctx, workspaceID, bot.AgentID)
		if err != nil {
			return nil, err
		}

		if agent.Disabled() {
			continue
		}

		entry := service.MemberTelegramBot{Bot: bot, AgentName: agent.Name}

		linked, err := s.audience.AccountFor(ctx, bot.ID, decision.Actor.AccountID)

		switch {
		case err == nil:
			entry.Linked = true
			entry.LinkedAs = linked.Username
		case !errors.Is(err, entity.ErrTelegramAccountNotLinked):
			return nil, err
		}

		mine = append(mine, entry)
	}

	return mine, nil
}

func (s *bots) deliverable() bool {
	parsed, err := url.Parse(s.app.BaseURL)

	return err == nil && parsed.Scheme == httpsScheme && parsed.Host != ""
}

func (s *bots) updatesURL(botID uuid.UUID) string {
	joined, err := url.JoinPath(s.app.BaseURL, updatesPath, botID.String(), updatesLeaf)
	if err != nil {
		return ""
	}

	return joined
}

func (s *bots) retire(ctx context.Context, token string) {
	postgres.AfterCommit(ctx, func(ctx context.Context) {
		if err := s.messenger.Unregister(ctx, token); err != nil {
			logging.From(ctx).WarnContext(ctx, "removing a retired telegram webhook failed", "error", err.Error())
		}
	})
}

func (s *bots) record(
	ctx context.Context,
	bot entity.TelegramBot,
	agent entity.Agent,
	action entity.AuditAction,
	detail map[string]string,
) {
	if detail == nil {
		detail = map[string]string{}
	}

	detail["bot"] = "@" + bot.Username

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  bot.WorkspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceAgent),
		ResourceID:   agent.ID,
		ResourceName: agent.Name,
		Detail:       detail,
	})
}
