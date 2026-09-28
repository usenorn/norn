package telegrambot

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *updates) Relay(ctx context.Context, workspaceID, questionID uuid.UUID) error {
	question, target, err := s.relayed(ctx, workspaceID, questionID)
	if err != nil || question.Settled() {
		return ignoreAbsent(err)
	}

	about, err := s.contextOf(ctx, target, question)
	if err != nil {
		return err
	}

	chats, err := s.recipients(ctx, target.bot, question)
	if err != nil {
		return err
	}

	posted, err := s.conversation.Posted(ctx, question.ID)
	if err != nil {
		return err
	}

	sent := make(map[int64]bool, len(posted))
	for _, message := range posted {
		sent[message.ChatID] = true
	}

	text := questionText(about, question)

	for _, chatID := range chats {
		if sent[chatID] {
			continue
		}

		messageID, err := s.messenger.Send(ctx, target.token, entity.TelegramOutgoing{
			ChatID:  chatID,
			Text:    text,
			Options: question.Options,
		})
		if errors.Is(err, entity.ErrTelegramChatUnavailable) {
			logging.From(ctx).WarnContext(ctx, "telegram refused a question for one chat", "error", err.Error())

			continue
		}

		if err != nil {
			return err
		}

		if err := s.conversation.Remember(ctx, entity.TelegramQuestionMessage{
			BotID:      target.bot.ID,
			ChatID:     chatID,
			MessageID:  messageID,
			QuestionID: question.ID,
		}); err != nil {
			return err
		}

		sent[chatID] = true
	}

	return nil
}

func (s *updates) Settle(ctx context.Context, workspaceID, questionID uuid.UUID) error {
	question, target, err := s.relayed(ctx, workspaceID, questionID)
	if err != nil || !question.Settled() {
		return ignoreAbsent(err)
	}

	posted, err := s.conversation.Posted(ctx, question.ID)
	if err != nil || len(posted) == 0 {
		return err
	}

	about, err := s.contextOf(ctx, target, question)
	if err != nil {
		return err
	}

	text := settledText(about, question)

	for _, message := range posted {
		if message.Settled || message.BotID != target.bot.ID {
			continue
		}

		err := s.messenger.Edit(ctx, target.token, message.ChatID, message.MessageID, text)
		if err != nil && !errors.Is(err, entity.ErrTelegramChatUnavailable) {
			return err
		}

		if err := s.conversation.MarkSettled(ctx, message, time.Now().UTC()); err != nil {
			return err
		}
	}

	return nil
}

func (s *updates) relayed(
	ctx context.Context,
	workspaceID, questionID uuid.UUID,
) (entity.IssueQuestion, delivery, error) {
	question, err := s.questions.GetByID(ctx, workspaceID, questionID)
	if err != nil {
		return entity.IssueQuestion{}, delivery{}, err
	}

	if question.AskedByAccountID == uuid.Nil {
		return entity.IssueQuestion{}, delivery{}, entity.ErrAgentNotFound
	}

	agent, err := s.agents.GetByAccountID(ctx, question.AskedByAccountID)
	if err != nil {
		return entity.IssueQuestion{}, delivery{}, err
	}

	if agent.WorkspaceID != workspaceID || agent.Disabled() {
		return entity.IssueQuestion{}, delivery{}, entity.ErrAgentNotFound
	}

	bot, err := s.bots.Get(ctx, workspaceID, agent.ID)
	if err != nil {
		return entity.IssueQuestion{}, delivery{}, err
	}

	token, err := s.bots.Token(ctx, bot.ID)
	if err != nil {
		return entity.IssueQuestion{}, delivery{}, err
	}

	return question, delivery{bot: bot, agent: agent, token: token}, nil
}

func (s *updates) contextOf(
	ctx context.Context,
	target delivery,
	question entity.IssueQuestion,
) (questionContext, error) {
	issue, err := s.issues.GetVisible(ctx, question.WorkspaceID, question.IssueID, entity.TeamScope{
		WorkspaceID:    question.WorkspaceID,
		AllTeams:       true,
		IncludePrivate: true,
	})
	if err != nil {
		return questionContext{}, err
	}

	workspace, err := s.workspaces.GetByID(ctx, question.WorkspaceID)
	if err != nil {
		return questionContext{}, err
	}

	link, err := url.JoinPath(s.app.BaseURL, workspace.Slug, issuesSegment, issue.Reference())
	if err != nil {
		return questionContext{}, err
	}

	return questionContext{
		agentName: target.agent.Name,
		reference: issue.Reference(),
		title:     strings.TrimSpace(issue.Title),
		issueURL:  link,
	}, nil
}

func (s *updates) recipients(
	ctx context.Context,
	bot entity.TelegramBot,
	question entity.IssueQuestion,
) ([]int64, error) {
	var chats []int64

	delegation, err := s.delegations.Open(ctx, question.WorkspaceID, question.IssueID)

	switch {
	case err == nil && delegation.DelegatedByAccountID != uuid.Nil:
		linked, err := s.audience.AccountFor(ctx, bot.ID, delegation.DelegatedByAccountID)

		switch {
		case err == nil:
			chats = append(chats, linked.ChatID)
		case !errors.Is(err, entity.ErrTelegramAccountNotLinked):
			return nil, err
		}
	case err != nil && !errors.Is(err, entity.ErrIssueDelegationNotFound):
		return nil, err
	}

	groups, err := s.audience.Groups(ctx, bot.ID)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		chats = append(chats, group.ChatID)
	}

	return chats, nil
}

func ignoreAbsent(err error) error {
	switch {
	case err == nil,
		errors.Is(err, entity.ErrIssueQuestionNotFound),
		errors.Is(err, entity.ErrAgentNotFound),
		errors.Is(err, entity.ErrTelegramBotNotFound):
		return nil
	default:
		return err
	}
}
