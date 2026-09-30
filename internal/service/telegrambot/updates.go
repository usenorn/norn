package telegrambot

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	helpCommand     = "/help"
	startCommand    = "/start"
	settingsSegment = "settings"
	notificationsAt = "notifications"
	issuesSegment   = "issues"
)

type updates struct {
	bots         repository.TelegramBot
	audience     repository.TelegramAudience
	conversation repository.TelegramConversation
	received     repository.TelegramUpdate
	messenger    repository.TelegramMessenger
	agents       repository.Agent
	questions    repository.IssueQuestion
	executions   repository.Execution
	plans        repository.ExecutionPlan
	reviews      repository.ExecutionReview
	changesets   repository.ChangeSet
	issues       repository.Issue
	delegations  repository.IssueDelegation
	settings     repository.NotificationSetting
	workspaces   repository.Workspace
	jobs         repository.JobProducer
	transactor   repository.Transactor
	answers      service.IssueQuestions
	decisions    service.Executions
	hosted       service.HostedAgents
	audit        service.Audit
	app          config.App
	limits       config.Telegram
}

func NewUpdates(
	telegramBots repository.TelegramBot,
	audience repository.TelegramAudience,
	conversation repository.TelegramConversation,
	received repository.TelegramUpdate,
	messenger repository.TelegramMessenger,
	agents repository.Agent,
	questions repository.IssueQuestion,
	executions repository.Execution,
	plans repository.ExecutionPlan,
	reviews repository.ExecutionReview,
	changesets repository.ChangeSet,
	issues repository.Issue,
	delegations repository.IssueDelegation,
	settings repository.NotificationSetting,
	workspaces repository.Workspace,
	jobs repository.JobProducer,
	transactor repository.Transactor,
	answers service.IssueQuestions,
	decisions service.Executions,
	hosted service.HostedAgents,
	audit service.Audit,
	app config.App,
	limits config.Telegram,
) service.TelegramUpdates {
	return &updates{
		bots:         telegramBots,
		audience:     audience,
		conversation: conversation,
		received:     received,
		messenger:    messenger,
		agents:       agents,
		questions:    questions,
		executions:   executions,
		plans:        plans,
		reviews:      reviews,
		changesets:   changesets,
		issues:       issues,
		delegations:  delegations,
		settings:     settings,
		workspaces:   workspaces,
		jobs:         jobs,
		transactor:   transactor,
		answers:      answers,
		decisions:    decisions,
		hosted:       hosted,
		audit:        audit,
		app:          app,
		limits:       limits,
	}
}

type delivery struct {
	bot   entity.TelegramBot
	agent entity.Agent
	token string
}

func (s *updates) Accept(ctx context.Context, botID uuid.UUID, secret string, payload []byte) error {
	bot, err := s.bots.GetByID(ctx, botID)
	if err != nil {
		return err
	}

	expected, err := s.bots.SecretHash(ctx, botID)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(entity.HashTelegramSecret(secret), expected) != 1 {
		return entity.ErrTelegramSecretInvalid
	}

	incoming, err := s.messenger.Decode(payload)
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "an undecodable telegram update was dropped", "error", err.Error())

		return nil
	}

	if !relevant(incoming, bot) {
		return nil
	}

	id, err := s.received.Record(ctx, entity.TelegramUpdate{BotID: botID, UpdateID: incoming.UpdateID, Payload: payload})
	if errors.Is(err, entity.ErrTelegramUpdateDuplicate) {
		stored, err := s.received.UpdateOf(ctx, botID, incoming.UpdateID)
		if err != nil {
			return err
		}

		if stored.Processed() {
			return nil
		}

		id = stored.ID
	} else if err != nil {
		return err
	}

	return s.jobs.EnqueueTelegramUpdate(ctx, entity.TelegramUpdatePayload{UpdateID: id})
}

func relevant(incoming entity.TelegramIncoming, bot entity.TelegramBot) bool {
	switch {
	case incoming.Callback != nil, incoming.Membership != nil:
		return true
	case incoming.Message == nil:
		return false
	}

	message := *incoming.Message

	if message.MigratedTo != 0 || message.AddressedTo(bot) {
		return true
	}

	_, starting := message.Start(bot.Username)

	return starting
}

func (s *updates) Apply(ctx context.Context, updateID uuid.UUID) error {
	var chat *pendingChat

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		stored, err := s.received.Lock(ctx, updateID)
		if err != nil {
			return err
		}

		if stored.Processed() {
			return nil
		}

		incoming, err := s.messenger.Decode(stored.Payload)
		if err != nil {
			return s.received.Settle(ctx, updateID, entity.TelegramUpdateFailed, time.Now().UTC())
		}

		target, err := s.deliveryFor(ctx, stored.BotID)
		if err != nil {
			return err
		}

		outcome, deferred, err := s.route(ctx, target, incoming)
		if err != nil {
			return err
		}

		chat = deferred

		return s.received.Settle(ctx, updateID, outcome, time.Now().UTC())
	}); err != nil {
		if errors.Is(err, entity.ErrTelegramUpdateNotFound) || errors.Is(err, entity.ErrTelegramBotNotFound) {
			return nil
		}

		return err
	}

	if chat != nil {
		s.converse(ctx, *chat)
	}

	return nil
}

func (s *updates) deliveryFor(ctx context.Context, botID uuid.UUID) (delivery, error) {
	bot, err := s.bots.GetByID(ctx, botID)
	if err != nil {
		return delivery{}, err
	}

	agent, err := s.agents.GetByID(ctx, bot.WorkspaceID, bot.AgentID)
	if err != nil {
		return delivery{}, err
	}

	token, err := s.bots.Token(ctx, bot.ID)
	if err != nil {
		return delivery{}, err
	}

	return delivery{bot: bot, agent: agent, token: token}, nil
}

type pendingChat struct {
	target  delivery
	account entity.TelegramAccount
	message entity.TelegramMessage
	prompt  string
}

func (s *updates) route(
	ctx context.Context,
	target delivery,
	incoming entity.TelegramIncoming,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	switch {
	case incoming.Callback != nil:
		return s.callback(ctx, target, *incoming.Callback)
	case incoming.Membership != nil:
		return s.membership(ctx, target, *incoming.Membership)
	case incoming.Message != nil:
		return s.message(ctx, target, *incoming.Message)
	default:
		return entity.TelegramUpdateIgnored, nil, nil
	}
}

func (s *updates) membership(
	ctx context.Context,
	target delivery,
	membership entity.TelegramMembership,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	if !membership.Removed || !membership.ChatType.Grouped() {
		return entity.TelegramUpdateIgnored, nil, nil
	}

	if err := s.audience.UnbindChat(ctx, target.bot.ID, membership.ChatID); err != nil {
		return "", nil, err
	}

	return entity.TelegramUpdateApplied, nil, nil
}

func (s *updates) message(
	ctx context.Context,
	target delivery,
	message entity.TelegramMessage,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	if message.MigratedTo != 0 {
		if err := s.audience.MoveGroup(ctx, target.bot.ID, message.ChatID, message.MigratedTo); err != nil {
			return "", nil, err
		}

		return entity.TelegramUpdateApplied, nil, nil
	}

	if !message.Sender.Person() {
		return entity.TelegramUpdateIgnored, nil, nil
	}

	if code, ok := message.Start(target.bot.Username); ok {
		return s.redeem(ctx, target, message, code)
	}

	grouped := message.ChatType.Grouped()

	if grouped {
		if _, err := s.audience.Group(ctx, target.bot.ID, message.ChatID); err != nil {
			if errors.Is(err, entity.ErrTelegramGroupNotFound) {
				return entity.TelegramUpdateIgnored, nil, nil
			}

			return "", nil, err
		}
	} else if message.ChatType != entity.TelegramChatPrivate {
		return entity.TelegramUpdateIgnored, nil, nil
	}

	account, err := s.audience.AccountOf(ctx, target.bot.ID, message.Sender.ID)
	if err != nil && !errors.Is(err, entity.ErrTelegramAccountNotLinked) {
		return "", nil, err
	}

	linked := err == nil

	if message.ReplyToID != 0 {
		decision, err := s.conversation.DecisionAt(ctx, target.bot.ID, message.ChatID, message.ReplyToID)

		switch {
		case err == nil:
			return s.replied(ctx, target, message, account, linked, decision)
		case !errors.Is(err, entity.ErrTelegramDecisionNotFound):
			return "", nil, err
		}
	}

	if !message.AddressedTo(target.bot) {
		return entity.TelegramUpdateIgnored, nil, nil
	}

	prompt := message.Prompt(target.bot.Username)

	if !linked {
		return s.say(ctx, target, message, s.unlinkedText(ctx, target))
	}

	if prompt == "" || prompt == startCommand || prompt == helpCommand {
		return s.say(ctx, target, message, s.helpText(target))
	}

	if !target.agent.Hosted() {
		return s.say(ctx, target, message, plain(fmt.Sprintf(
			"%s works through its runner and only asks its questions here. Answer them with the buttons "+
				"or by replying to the question.", target.agent.Name,
		)))
	}

	return entity.TelegramUpdateApplied, &pendingChat{
		target:  target,
		account: account,
		message: message,
		prompt:  clipped(prompt, entity.AgentTurnMaxLen),
	}, nil
}

func (s *updates) redeem(
	ctx context.Context,
	target delivery,
	message entity.TelegramMessage,
	code string,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	hash := entity.HashTelegramSecret(code)

	redeemed, err := s.audience.LinkCode(ctx, target.bot.ID, hash, time.Now().UTC())
	if errors.Is(err, entity.ErrTelegramLinkCodeInvalid) {
		return s.say(ctx, target, message, plain(
			"This link has expired or was already used. Open Norn and create a new one.",
		))
	}

	if err != nil {
		return "", nil, err
	}

	switch {
	case redeemed.Purpose == entity.TelegramLinkGroup && !message.ChatType.Grouped():
		return s.say(ctx, target, message, plain(fmt.Sprintf(
			"This link connects a group. Add @%s to a group with it instead.", target.bot.Username,
		)))

	case redeemed.Purpose == entity.TelegramLinkPrivate && message.ChatType != entity.TelegramChatPrivate:
		return s.say(ctx, target, message, plain(fmt.Sprintf(
			"This link is for your private chat with @%s. Open it from Telegram directly.", target.bot.Username,
		)))

	case redeemed.Purpose == entity.TelegramLinkGroup:
		return s.bind(ctx, target, message, redeemed, hash)
	}

	if err := s.audience.Link(ctx, entity.TelegramAccount{
		BotID:          target.bot.ID,
		AccountID:      redeemed.AccountID,
		TelegramUserID: message.Sender.ID,
		ChatID:         message.ChatID,
		Username:       message.Sender.Username,
	}); err != nil {
		return "", nil, err
	}

	if err := s.audience.SpendCode(ctx, target.bot.ID, hash); err != nil {
		return "", nil, err
	}

	return s.say(ctx, target, message, plain(fmt.Sprintf(
		"Linked. Choose Telegram for decision requests in Norn's notification settings, and %s "+
			"will send you its questions, plans and changes here.", target.agent.Name,
	)))
}

func (s *updates) bind(
	ctx context.Context,
	target delivery,
	message entity.TelegramMessage,
	redeemed entity.TelegramLinkCode,
	hash []byte,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	account, err := s.audience.AccountOf(ctx, target.bot.ID, message.Sender.ID)
	if err != nil && !errors.Is(err, entity.ErrTelegramAccountNotLinked) {
		return "", nil, err
	}

	if err != nil || account.AccountID != redeemed.AccountID {
		return s.say(ctx, target, message, plain(fmt.Sprintf(
			"Only the person who created this link in Norn can connect this group, once they have linked "+
				"their own Telegram to @%s.", target.bot.Username,
		)))
	}

	group, err := s.audience.Bind(ctx, entity.TelegramGroup{
		BotID:   target.bot.ID,
		ChatID:  message.ChatID,
		Title:   message.ChatTitle,
		BoundBy: account.AccountID,
	})
	if err != nil {
		return "", nil, err
	}

	if err := s.audience.SpendCode(ctx, target.bot.ID, hash); err != nil {
		return "", nil, err
	}

	s.audit.Record(identity.WithActor(ctx, account.Actor()), entity.AuditEntry{
		WorkspaceID:  target.bot.WorkspaceID,
		Action:       entity.AuditAgentTelegramGroupBound,
		ResourceKind: string(entity.ResourceAgent),
		ResourceID:   target.agent.ID,
		ResourceName: target.agent.Name,
		Detail:       map[string]string{"bot": "@" + target.bot.Username, "group": group.Title},
	})

	return s.say(ctx, target, message, plain(fmt.Sprintf(
		"Connected. %s will post its questions here, and linked members can answer them.", target.agent.Name,
	)))
}

func (s *updates) replied(
	ctx context.Context,
	target delivery,
	message entity.TelegramMessage,
	account entity.TelegramAccount,
	linked bool,
	decision entity.TelegramDecisionMessage,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	if !linked {
		return s.say(ctx, target, message, s.unlinkedText(ctx, target))
	}

	if decision.Kind != entity.TelegramDecisionQuestion {
		outcome, err := s.feedback(ctx, account, decision, message.Text)
		if err != nil {
			return "", nil, err
		}

		return s.say(ctx, target, message, plain(outcome))
	}

	question, err := s.questions.GetByID(ctx, target.bot.WorkspaceID, decision.QuestionID)
	if err != nil {
		return "", nil, err
	}

	if !question.AllowFreeText {
		return s.say(ctx, target, message, plain("Pick one of the buttons on the question to answer it."))
	}

	outcome, err := s.answer(ctx, account, question, message.Text)
	if err != nil {
		return "", nil, err
	}

	return s.say(ctx, target, message, plain(outcome))
}

func (s *updates) callback(
	ctx context.Context,
	target delivery,
	callback entity.TelegramCallback,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	if !callback.Sender.Person() {
		return entity.TelegramUpdateIgnored, nil, nil
	}

	notice, err := s.chosen(ctx, target, callback)
	if err != nil {
		return "", nil, err
	}

	if err := s.messenger.AnswerCallback(ctx, target.token, callback.ID, notice); err != nil &&
		!errors.Is(err, entity.ErrTelegramChatUnavailable) {
		return "", nil, err
	}

	return entity.TelegramUpdateApplied, nil, nil
}

func (s *updates) chosen(ctx context.Context, target delivery, callback entity.TelegramCallback) (string, error) {
	decision, err := s.conversation.DecisionAt(ctx, target.bot.ID, callback.ChatID, callback.MessageID)
	if errors.Is(err, entity.ErrTelegramDecisionNotFound) {
		return "This is no longer open.", nil
	}

	if err != nil {
		return "", err
	}

	if decision.Settled {
		return "This was already decided.", nil
	}

	account, err := s.audience.AccountOf(ctx, target.bot.ID, callback.Sender.ID)
	if errors.Is(err, entity.ErrTelegramAccountNotLinked) {
		return "Link your Telegram in Norn to decide.", nil
	}

	if err != nil {
		return "", err
	}

	if decision.Kind != entity.TelegramDecisionQuestion {
		return s.pressed(ctx, account, decision, callback.Data)
	}

	question, err := s.questions.GetByID(ctx, target.bot.WorkspaceID, decision.QuestionID)
	if err != nil {
		return "", err
	}

	index, ok := callback.Option()
	if !ok || index >= len(question.Options) {
		return "That option is not available.", nil
	}

	return s.answer(ctx, account, question, question.Options[index])
}

func (s *updates) answer(
	ctx context.Context,
	account entity.TelegramAccount,
	question entity.IssueQuestion,
	text string,
) (string, error) {
	err := s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
		_, err := s.answers.Answer(
			identity.WithActor(ctx, account.Actor()),
			question.WorkspaceID, question.IssueID, question.ID,
			service.AnswerQuestionInput{Answer: text},
		)

		return err
	})

	if errors.Is(err, entity.ErrIssueQuestionUnanswerable) {
		return "That answer is not one this question accepts.", nil
	}

	return s.outcome(ctx, question.WorkspaceID, question.IssueID, err, "Answered.")
}

func (s *updates) say(
	ctx context.Context,
	target delivery,
	message entity.TelegramMessage,
	text string,
) (entity.TelegramUpdateOutcome, *pendingChat, error) {
	outgoing := entity.TelegramOutgoing{ChatID: message.ChatID, Text: text}
	if message.ChatType.Grouped() {
		outgoing.ReplyTo = message.MessageID
	}

	if _, err := s.messenger.Send(ctx, target.token, outgoing); err != nil {
		if errors.Is(err, entity.ErrTelegramChatUnavailable) {
			return entity.TelegramUpdateIgnored, nil, nil
		}

		return "", nil, err
	}

	return entity.TelegramUpdateApplied, nil, nil
}

func (s *updates) converse(ctx context.Context, chat pendingChat) {
	target := chat.target
	logger := logging.From(ctx)

	if err := s.messenger.Typing(ctx, target.token, chat.message.ChatID); err != nil {
		logger.WarnContext(ctx, "telegram typing indicator failed", "error", err.Error())
	}

	text, err := s.reply(ctx, chat)
	if err != nil {
		logger.ErrorContext(ctx, "a telegram conversation failed", "error", err.Error(), "agent_id", target.agent.ID)

		text = plain(fmt.Sprintf("%s could not answer just now. Try again in a moment.", target.agent.Name))
	}

	outgoing := entity.TelegramOutgoing{ChatID: chat.message.ChatID, Text: text}
	if chat.message.ChatType.Grouped() {
		outgoing.ReplyTo = chat.message.MessageID
	}

	if _, err := s.messenger.Send(ctx, target.token, outgoing); err != nil {
		logger.WarnContext(ctx, "sending a telegram reply failed", "error", err.Error(), "agent_id", target.agent.ID)
	}
}

func (s *updates) reply(ctx context.Context, chat pendingChat) (string, error) {
	target := chat.target
	userTurn := entity.AgentTurn{Role: entity.AgentTurnUser, Text: chat.prompt}

	if err := s.conversation.Append(ctx, target.bot.ID, chat.message.ChatID, userTurn); err != nil {
		return "", err
	}

	history, err := s.conversation.History(ctx, target.bot.ID, chat.message.ChatID, s.limits.HistoryTurns)
	if err != nil {
		return "", err
	}

	var denied entity.AccessDeniedError

	answered, err := s.hosted.Chat(
		identity.WithActor(ctx, chat.account.Actor()), target.bot.WorkspaceID, target.agent.ID, conversable(history),
	)

	switch {
	case errors.Is(err, entity.ErrAgentNotFound), errors.Is(err, entity.ErrAccountForbidden),
		errors.As(err, &denied):
		return plain(fmt.Sprintf(
			"Only %s's owner or a workspace admin can talk to it here. It still sends its questions to everyone linked.",
			target.agent.Name,
		)), nil
	case errors.Is(err, entity.ErrAgentDisabled):
		return plain(fmt.Sprintf("%s is disabled.", target.agent.Name)), nil
	case errors.Is(err, entity.ErrAIProviderNotConfigured):
		return plain("This workspace has no AI provider set up, so the agent cannot answer."), nil
	case err != nil:
		return "", err
	}

	if answered.Stop != entity.AgentConversationAnswered || answered.Text == "" {
		logging.From(ctx).WarnContext(ctx, "a telegram conversation stopped early",
			"stop", string(answered.Stop),
			"input_tokens", answered.Usage.Input,
			"output_tokens", answered.Usage.Output,
			"agent_id", target.agent.ID,
		)
	}

	if answered.Text == "" {
		return plain(stoppedText(target.agent.Name, answered.Stop)), nil
	}

	if err := s.conversation.Append(ctx, target.bot.ID, chat.message.ChatID, entity.AgentTurn{
		Role: entity.AgentTurnAssistant,
		Text: clipped(answered.Text, entity.AgentTurnMaxLen),
	}); err != nil {
		return "", err
	}

	if err := s.conversation.Prune(ctx, target.bot.ID, chat.message.ChatID, s.limits.HistoryTurns); err != nil {
		return "", err
	}

	return replyText(answered.Text), nil
}

func stoppedText(agentName string, stop entity.AgentConversationStop) string {
	switch stop {
	case entity.AgentConversationRoundLimit:
		return fmt.Sprintf("%s stopped after using as many tools as one answer may.", agentName)
	case entity.AgentConversationTokenLimit:
		return fmt.Sprintf("%s stopped at this instance's limit on model usage for one answer.", agentName)
	case entity.AgentConversationTimeLimit:
		return fmt.Sprintf("%s ran out of time before it finished.", agentName)
	default:
		return fmt.Sprintf("%s had nothing to say.", agentName)
	}
}

func conversable(history []entity.AgentTurn) []entity.AgentTurn {
	for len(history) > 0 && history[0].Role != entity.AgentTurnUser {
		history = history[1:]
	}

	return history
}

func (s *updates) unlinkedText(ctx context.Context, target delivery) string {
	settings := s.settingsURL(ctx, target.bot.WorkspaceID)
	if settings == "" {
		return plain("Link your Telegram in Norn's notification settings first.")
	}

	return fmt.Sprintf(
		"Link your Telegram in Norn first: <a href=\"%s\">notification settings</a>.", plain(settings),
	)
}

func (s *updates) helpText(target delivery) string {
	if target.agent.Hosted() {
		return plain(fmt.Sprintf(
			"%s posts its questions here. Answer with the buttons or by replying to the question. "+
				"Its owner and workspace admins can also ask it anything.", target.agent.Name,
		))
	}

	return plain(fmt.Sprintf(
		"%s posts its questions here. Answer with the buttons or by replying to the question.", target.agent.Name,
	))
}

func (s *updates) settingsURL(ctx context.Context, workspaceID uuid.UUID) string {
	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return ""
	}

	joined, err := url.JoinPath(s.app.BaseURL, workspace.Slug, settingsSegment, notificationsAt)
	if err != nil {
		return ""
	}

	return joined
}

func (s *updates) Sweep(ctx context.Context) (int, error) {
	return s.received.Sweep(ctx, time.Now().UTC().Add(-s.limits.UpdateRetention), entity.TelegramUpdateSweepBatch)
}
