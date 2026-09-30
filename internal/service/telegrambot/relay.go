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

const (
	executionsSegment = "executions"
	reviewSegment     = "review"
	reviewsSegment    = "reviews"
	planFragment      = "#plan"
)

type outbound struct {
	target   delivery
	about    decisionContext
	text     string
	buttons  []entity.TelegramButton
	template entity.TelegramDecisionMessage
}

func (s *updates) Relay(ctx context.Context, decision entity.TelegramDecision) error {
	prepared, open, err := s.prepare(ctx, decision)
	if err != nil || !open {
		return ignoreAbsent(err)
	}

	chats, err := s.recipients(ctx, prepared.target.bot, decision.WorkspaceID, prepared.about.issueID)
	if err != nil {
		return err
	}

	posted, err := s.conversation.Posted(ctx, decision)
	if err != nil {
		return err
	}

	sent := make(map[int64]bool, len(posted))

	for _, message := range posted {
		if message.BotID == prepared.target.bot.ID && sameRound(message, prepared.template) {
			sent[message.ChatID] = true
		}
	}

	for _, chatID := range chats {
		if sent[chatID] {
			continue
		}

		messageID, err := s.messenger.Send(ctx, prepared.target.token, entity.TelegramOutgoing{
			ChatID:  chatID,
			Text:    prepared.text,
			Buttons: prepared.buttons,
		})
		if errors.Is(err, entity.ErrTelegramChatUnavailable) {
			logging.From(ctx).WarnContext(ctx, "telegram refused a decision for one chat", "error", err.Error())

			continue
		}

		if err != nil {
			return err
		}

		message := prepared.template
		message.BotID = prepared.target.bot.ID
		message.ChatID = chatID
		message.MessageID = messageID

		if err := s.conversation.Remember(ctx, message); err != nil {
			return err
		}

		sent[chatID] = true
	}

	return nil
}

func sameRound(posted, template entity.TelegramDecisionMessage) bool {
	switch template.Kind {
	case entity.TelegramDecisionPlan:
		return posted.PlanRevision == template.PlanRevision
	case entity.TelegramDecisionReview:
		return posted.ReviewHeads.Matches(template.ReviewHeads)
	case entity.TelegramDecisionPublication:
		return posted.Round == template.Round
	default:
		return true
	}
}

func (s *updates) prepare(ctx context.Context, decision entity.TelegramDecision) (outbound, bool, error) {
	switch decision.Kind {
	case entity.TelegramDecisionQuestion:
		return s.prepareQuestion(ctx, decision)
	case entity.TelegramDecisionPlan:
		return s.preparePlan(ctx, decision)
	case entity.TelegramDecisionReview:
		return s.prepareReview(ctx, decision)
	case entity.TelegramDecisionPublication:
		return s.preparePublication(ctx, decision)
	default:
		return outbound{}, false, nil
	}
}

func (s *updates) prepareQuestion(ctx context.Context, decision entity.TelegramDecision) (outbound, bool, error) {
	question, target, err := s.relayed(ctx, decision.WorkspaceID, decision.QuestionID)
	if err != nil || question.Settled() {
		return outbound{}, false, err
	}

	about, err := s.contextOf(ctx, target, question.WorkspaceID, question.IssueID, "")
	if err != nil {
		return outbound{}, false, err
	}

	return outbound{
		target:   target,
		about:    about,
		text:     questionText(about, question),
		buttons:  entity.TelegramOptionButtons(question.Options),
		template: entity.TelegramDecisionMessage{Kind: entity.TelegramDecisionQuestion, QuestionID: question.ID},
	}, true, nil
}

func (s *updates) preparePlan(ctx context.Context, decision entity.TelegramDecision) (outbound, bool, error) {
	execution, target, err := s.executed(ctx, decision)
	if err != nil || execution.State != entity.ExecutionAwaitingPlan {
		return outbound{}, false, err
	}

	plans, err := s.plans.ListByExecution(ctx, execution.ID)
	if err != nil {
		return outbound{}, false, err
	}

	plan, ok := entity.LatestPlan(plans)
	if !ok || !plan.Undecided() {
		return outbound{}, false, nil
	}

	about, err := s.contextOf(ctx, target, execution.WorkspaceID, execution.IssueID, execution.ID)
	if err != nil {
		return outbound{}, false, err
	}

	return outbound{
		target: target,
		about:  about,
		text:   planText(about, plan),
		buttons: []entity.TelegramButton{
			{Label: "Approve plan", Data: entity.TelegramCallbackApprove},
			{Label: "Ask for changes", Data: entity.TelegramCallbackChanges},
		},
		template: entity.TelegramDecisionMessage{
			Kind:         entity.TelegramDecisionPlan,
			ExecutionID:  execution.ID,
			PlanRevision: plan.Revision,
		},
	}, true, nil
}

func (s *updates) prepareReview(ctx context.Context, decision entity.TelegramDecision) (outbound, bool, error) {
	execution, target, err := s.executed(ctx, decision)
	if err != nil || execution.State != entity.ExecutionAwaitingReview {
		return outbound{}, false, err
	}

	latest, err := s.snapshots.Latest(ctx, execution.ID)
	if err != nil {
		return outbound{}, false, err
	}

	about, err := s.contextOf(ctx, target, execution.WorkspaceID, execution.IssueID, execution.ID)
	if err != nil {
		return outbound{}, false, err
	}

	return outbound{
		target: target,
		about:  about,
		text:   reviewText(about, latest),
		buttons: []entity.TelegramButton{
			{Label: "Approve and publish", Data: entity.TelegramCallbackApprove},
			{Label: "Request changes", Data: entity.TelegramCallbackChanges},
		},
		template: entity.TelegramDecisionMessage{
			Kind:        entity.TelegramDecisionReview,
			ExecutionID: execution.ID,
			ReviewHeads: latest.Heads(),
		},
	}, true, nil
}

func (s *updates) preparePublication(
	ctx context.Context,
	decision entity.TelegramDecision,
) (outbound, bool, error) {
	execution, target, err := s.executed(ctx, decision)
	if err != nil || execution.State != entity.ExecutionApproved {
		return outbound{}, false, err
	}

	changeset, err := s.changesets.Get(ctx, execution.ID)
	if err != nil || !changeset.PublicationFailed() {
		return outbound{}, false, err
	}

	about, err := s.contextOf(ctx, target, execution.WorkspaceID, execution.IssueID, execution.ID)
	if err != nil {
		return outbound{}, false, err
	}

	return outbound{
		target: target,
		about:  about,
		text:   publicationText(about, changeset),
		buttons: []entity.TelegramButton{
			{Label: "Retry publication", Data: entity.TelegramCallbackRetry},
			{Label: "Give up", Data: entity.TelegramCallbackAbandon},
		},
		template: entity.TelegramDecisionMessage{
			Kind:        entity.TelegramDecisionPublication,
			ExecutionID: execution.ID,
			Round:       decision.Round,
		},
	}, true, nil
}

func (s *updates) Settle(ctx context.Context, decision entity.TelegramDecision) error {
	posted, err := s.conversation.Posted(ctx, decision)
	if err != nil || len(posted) == 0 {
		return err
	}

	settle, err := s.settlement(ctx, decision)
	if err != nil {
		return ignoreAbsent(err)
	}

	for _, message := range posted {
		if message.Settled || message.BotID != settle.target.bot.ID {
			continue
		}

		text, closed := settle.text(message)
		if !closed {
			continue
		}

		err := s.messenger.Edit(ctx, settle.target.token, message.ChatID, message.MessageID, text)
		if err != nil && !errors.Is(err, entity.ErrTelegramChatUnavailable) {
			return err
		}

		if err := s.conversation.MarkSettled(ctx, message, time.Now().UTC()); err != nil {
			return err
		}
	}

	return nil
}

type settlement struct {
	target delivery
	text   func(message entity.TelegramDecisionMessage) (string, bool)
}

func (s *updates) settlement(ctx context.Context, decision entity.TelegramDecision) (settlement, error) {
	if decision.Kind == entity.TelegramDecisionQuestion {
		question, target, err := s.relayed(ctx, decision.WorkspaceID, decision.QuestionID)
		if err != nil {
			return settlement{}, err
		}

		about, err := s.contextOf(ctx, target, question.WorkspaceID, question.IssueID, "")
		if err != nil {
			return settlement{}, err
		}

		return settlement{target: target, text: func(entity.TelegramDecisionMessage) (string, bool) {
			return settledText(about, question), question.Settled()
		}}, nil
	}

	execution, target, err := s.executed(ctx, decision)
	if err != nil {
		return settlement{}, err
	}

	about, err := s.contextOf(ctx, target, execution.WorkspaceID, execution.IssueID, execution.ID)
	if err != nil {
		return settlement{}, err
	}

	if decision.Kind == entity.TelegramDecisionPlan {
		plans, err := s.plans.ListByExecution(ctx, execution.ID)
		if err != nil {
			return settlement{}, err
		}

		return settlement{target: target, text: func(message entity.TelegramDecisionMessage) (string, bool) {
			return planSettled(about, execution, plans, message.PlanRevision)
		}}, nil
	}

	if decision.Kind == entity.TelegramDecisionPublication {
		changeset, err := s.changesets.Get(ctx, execution.ID)
		if err != nil {
			return settlement{}, err
		}

		return settlement{target: target, text: func(entity.TelegramDecisionMessage) (string, bool) {
			return publicationSettled(about, execution, changeset)
		}}, nil
	}

	reviews, err := s.reviews.ListReviews(ctx, execution.ID)
	if err != nil {
		return settlement{}, err
	}

	latest, err := s.snapshots.Latest(ctx, execution.ID)
	if err != nil {
		return settlement{}, err
	}

	current := latest.Heads()

	return settlement{target: target, text: func(message entity.TelegramDecisionMessage) (string, bool) {
		return reviewSettled(about, execution, reviews, current, message.ReviewHeads)
	}}, nil
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

	target, err := s.agentDelivery(ctx, workspaceID, agent)

	return question, target, err
}

func (s *updates) executed(ctx context.Context, decision entity.TelegramDecision) (entity.Execution, delivery, error) {
	execution, err := s.executions.GetByID(ctx, decision.ExecutionID)
	if err != nil {
		return entity.Execution{}, delivery{}, err
	}

	if execution.WorkspaceID != decision.WorkspaceID {
		return entity.Execution{}, delivery{}, entity.ErrExecutionNotFound
	}

	agent, err := s.agents.GetByID(ctx, execution.WorkspaceID, execution.AgentID)
	if err != nil {
		return entity.Execution{}, delivery{}, err
	}

	target, err := s.agentDelivery(ctx, execution.WorkspaceID, agent)

	return execution, target, err
}

func (s *updates) agentDelivery(ctx context.Context, workspaceID uuid.UUID, agent entity.Agent) (delivery, error) {
	if agent.WorkspaceID != workspaceID || agent.Disabled() {
		return delivery{}, entity.ErrAgentNotFound
	}

	bot, err := s.bots.Get(ctx, workspaceID, agent.ID)
	if err != nil {
		return delivery{}, err
	}

	token, err := s.bots.Token(ctx, bot.ID)
	if err != nil {
		return delivery{}, err
	}

	return delivery{bot: bot, agent: agent, token: token}, nil
}

func (s *updates) contextOf(
	ctx context.Context,
	target delivery,
	workspaceID, issueID uuid.UUID,
	executionID string,
) (decisionContext, error) {
	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, entity.TeamScope{
		WorkspaceID:    workspaceID,
		AllTeams:       true,
		IncludePrivate: true,
	})
	if err != nil {
		return decisionContext{}, err
	}

	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return decisionContext{}, err
	}

	issueURL, err := url.JoinPath(s.app.BaseURL, workspace.Slug, issuesSegment, issue.Reference())
	if err != nil {
		return decisionContext{}, err
	}

	reviewsURL, err := url.JoinPath(s.app.BaseURL, workspace.Slug, reviewsSegment)
	if err != nil {
		return decisionContext{}, err
	}

	about := decisionContext{
		agentName:  target.agent.Name,
		issueID:    issue.ID,
		reference:  issue.Reference(),
		title:      strings.TrimSpace(issue.Title),
		issueURL:   issueURL,
		reviewsURL: reviewsURL,
	}

	if executionID == "" {
		return about, nil
	}

	if about.runURL, err = url.JoinPath(s.app.BaseURL, workspace.Slug, executionsSegment, executionID); err != nil {
		return decisionContext{}, err
	}

	if about.reviewURL, err = url.JoinPath(about.runURL, reviewSegment); err != nil {
		return decisionContext{}, err
	}

	return about, nil
}

func (s *updates) recipients(
	ctx context.Context,
	bot entity.TelegramBot,
	workspaceID, issueID uuid.UUID,
) ([]int64, error) {
	var chats []int64

	authority, err := s.delegations.Authority(ctx, workspaceID, issueID)
	if err != nil {
		return nil, err
	}

	if maker := authority.Maker(); maker != uuid.Nil {
		channel, err := s.settings.DecisionChannel(ctx, workspaceID, maker)
		if err != nil {
			return nil, err
		}

		if channel == entity.DecisionChannelTelegram {
			linked, err := s.audience.AccountFor(ctx, bot.ID, maker)

			switch {
			case err == nil:
				chats = append(chats, linked.ChatID)
			case !errors.Is(err, entity.ErrTelegramAccountNotLinked):
				return nil, err
			}
		}
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
		errors.Is(err, entity.ErrExecutionNotFound),
		errors.Is(err, entity.ErrAgentNotFound),
		errors.Is(err, entity.ErrTelegramBotNotFound):
		return nil
	default:
		return err
	}
}
