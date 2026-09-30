package telegrambot_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	changesetrepo "github.com/usenorn/norn/internal/repository/changeset"
	executionrepo "github.com/usenorn/norn/internal/repository/execution"
	planrepo "github.com/usenorn/norn/internal/repository/executionplan"
	reviewrepo "github.com/usenorn/norn/internal/repository/executionreview"
	snapshotrepo "github.com/usenorn/norn/internal/repository/executionsnapshot"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	delegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	questionrepo "github.com/usenorn/norn/internal/repository/issuequestion"
	jobrepo "github.com/usenorn/norn/internal/repository/jobqueue"
	settingrepo "github.com/usenorn/norn/internal/repository/notificationsetting"
	previewrepo "github.com/usenorn/norn/internal/repository/preview"
	audiencerepo "github.com/usenorn/norn/internal/repository/telegramaudience"
	botrepo "github.com/usenorn/norn/internal/repository/telegrambot"
	conversationrepo "github.com/usenorn/norn/internal/repository/telegramconversation"
	messengerrepo "github.com/usenorn/norn/internal/repository/telegrammessenger"
	updaterepo "github.com/usenorn/norn/internal/repository/telegramupdate"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	hostedsvc "github.com/usenorn/norn/internal/service/hostedagent"
	questionsvc "github.com/usenorn/norn/internal/service/issuequestion"
	"github.com/usenorn/norn/internal/service/telegrambot"
)

const (
	botToken   = "7000000001:AAHharnessharnessharnessharness1234"
	baseURL    = "https://norn.example"
	botUserID  = 7000000001
	senderID   = 5100
	groupChat  = -1001234567890
	privateMsg = 42
)

type harness struct {
	bots         *botrepo.MockTelegramBot
	audience     *audiencerepo.MockTelegramAudience
	conversation *conversationrepo.MockTelegramConversation
	received     *updaterepo.MockTelegramUpdate
	messenger    *messengerrepo.MockTelegramMessenger
	agents       *agentrepo.MockAgent
	questions    *questionrepo.MockIssueQuestion
	executions   *executionrepo.MockExecution
	plans        *planrepo.MockExecutionPlan
	reviews      *reviewrepo.MockExecutionReview
	snapshots    *snapshotrepo.MockExecutionSnapshot
	previews     *previewrepo.MockPreview
	changesets   *changesetrepo.MockChangeSet
	issues       *issuerepo.MockIssue
	delegations  *delegationrepo.MockIssueDelegation
	settings     *settingrepo.MockNotificationSetting
	workspaces   *workspacerepo.MockWorkspace
	jobs         *jobrepo.MockJobProducer
	transactor   *transactorrepo.MockTransactor
	answers      *questionsvc.MockIssueQuestions
	decisions    *executionsvc.MockExecutions
	hosted       *hostedsvc.MockHostedAgents
	audit        *auditsvc.MockAudit
	authorizer   *authorizersvc.MockAuthorizer

	app         config.App
	caller      entity.Actor
	role        entity.MembershipRole
	workspaceID uuid.UUID
	agent       entity.Agent
	bot         entity.TelegramBot
	sent        []entity.TelegramOutgoing
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	workspaceID := uuid.New()
	ownerID := uuid.New()
	agent := entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: ownerID,
		Name:           "Ada",
		Status:         entity.AgentStatusActive,
		Execution:      entity.AgentExecutionHosted,
	}

	h := &harness{
		bots:         botrepo.NewMockTelegramBot(ctrl),
		audience:     audiencerepo.NewMockTelegramAudience(ctrl),
		conversation: conversationrepo.NewMockTelegramConversation(ctrl),
		received:     updaterepo.NewMockTelegramUpdate(ctrl),
		messenger:    messengerrepo.NewMockTelegramMessenger(ctrl),
		agents:       agentrepo.NewMockAgent(ctrl),
		questions:    questionrepo.NewMockIssueQuestion(ctrl),
		executions:   executionrepo.NewMockExecution(ctrl),
		plans:        planrepo.NewMockExecutionPlan(ctrl),
		reviews:      reviewrepo.NewMockExecutionReview(ctrl),
		snapshots:    snapshotrepo.NewMockExecutionSnapshot(ctrl),
		previews:     previewrepo.NewMockPreview(ctrl),
		changesets:   changesetrepo.NewMockChangeSet(ctrl),
		issues:       issuerepo.NewMockIssue(ctrl),
		delegations:  delegationrepo.NewMockIssueDelegation(ctrl),
		settings:     settingrepo.NewMockNotificationSetting(ctrl),
		workspaces:   workspacerepo.NewMockWorkspace(ctrl),
		jobs:         jobrepo.NewMockJobProducer(ctrl),
		transactor:   transactorrepo.NewMockTransactor(ctrl),
		answers:      questionsvc.NewMockIssueQuestions(ctrl),
		decisions:    executionsvc.NewMockExecutions(ctrl),
		hosted:       hostedsvc.NewMockHostedAgents(ctrl),
		audit:        auditsvc.NewMockAudit(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
		app:          config.App{BaseURL: baseURL},
		caller:       entity.Actor{Kind: entity.ActorKindUser, AccountID: ownerID},
		role:         entity.MembershipRoleMember,
		workspaceID:  workspaceID,
		agent:        agent,
		bot: entity.TelegramBot{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agent.ID,
			BotUserID:   botUserID,
			Username:    "ada_bot",
		},
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()
	h.transactor.EXPECT().
		WithSavepoint(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{Actor: h.caller, Role: h.role}, nil
		}).
		AnyTimes()

	h.agents.EXPECT().
		GetByID(gomock.Any(), workspaceID, agent.ID).
		DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) (entity.Agent, error) { return h.agent, nil }).
		AnyTimes()
	h.agents.EXPECT().
		GetByAccountID(gomock.Any(), agent.AccountID).
		DoAndReturn(func(context.Context, uuid.UUID) (entity.Agent, error) { return h.agent, nil }).
		AnyTimes()

	h.bots.EXPECT().GetByID(gomock.Any(), h.bot.ID).DoAndReturn(
		func(context.Context, uuid.UUID) (entity.TelegramBot, error) { return h.bot, nil },
	).AnyTimes()
	h.bots.EXPECT().Token(gomock.Any(), h.bot.ID).Return(botToken, nil).AnyTimes()

	h.workspaces.EXPECT().
		GetByID(gomock.Any(), workspaceID).
		Return(entity.Workspace{ID: workspaceID, Slug: "acme"}, nil).
		AnyTimes()

	h.audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	h.messenger.EXPECT().
		Send(gomock.Any(), botToken, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, message entity.TelegramOutgoing) (int64, error) {
			h.sent = append(h.sent, message)

			return int64(1000 + len(h.sent)), nil
		}).
		AnyTimes()

	return h
}

func (h *harness) limits() config.Telegram {
	return config.Telegram{LinkTTL: 15 * time.Minute, HistoryTurns: 20, UpdateRetention: time.Hour}
}

func (h *harness) botsService() service.TelegramBots {
	return telegrambot.NewBots(
		h.bots, h.audience, h.messenger, h.agents, h.transactor, h.authorizer, h.audit, h.app, h.limits(),
	)
}

func (h *harness) updatesService() service.TelegramUpdates {
	return telegrambot.NewUpdates(
		h.bots, h.audience, h.conversation, h.received, h.messenger, h.agents, h.questions,
		h.executions, h.plans, h.reviews, h.snapshots, h.previews, h.changesets, h.issues, h.delegations, h.settings, h.workspaces,
		h.jobs, h.transactor, h.answers, h.decisions, h.hosted, h.audit, h.app,
		config.Previews{Scheme: "https"}, h.limits(),
	)
}

func (h *harness) linked(accountID uuid.UUID, telegramUserID int64) entity.TelegramAccount {
	return entity.TelegramAccount{
		BotID:          h.bot.ID,
		AccountID:      accountID,
		TelegramUserID: telegramUserID,
		ChatID:         telegramUserID,
	}
}

func (h *harness) applying(incoming entity.TelegramIncoming) uuid.UUID {
	id := uuid.New()
	payload := []byte(`{}`)

	h.received.EXPECT().Lock(gomock.Any(), id).Return(entity.TelegramUpdate{
		ID: id, BotID: h.bot.ID, UpdateID: incoming.UpdateID, Payload: payload,
	}, nil)
	h.messenger.EXPECT().Decode(payload).Return(incoming, nil)

	return id
}

func (h *harness) settles(id uuid.UUID, outcome entity.TelegramUpdateOutcome) {
	h.received.EXPECT().Settle(gomock.Any(), id, outcome, gomock.Any()).Return(nil)
}
