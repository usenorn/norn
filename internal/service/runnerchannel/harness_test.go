package runnerchannel_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	runnerrepo "github.com/usenorn/norn/internal/repository/runner"
	channelrepo "github.com/usenorn/norn/internal/repository/runnerchannel"
	sessionrepo "github.com/usenorn/norn/internal/repository/runnersession"
	"github.com/usenorn/norn/internal/service"
	changesetsvc "github.com/usenorn/norn/internal/service/changeset"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	executionservicesvc "github.com/usenorn/norn/internal/service/executionservice"
	questionsvc "github.com/usenorn/norn/internal/service/issuequestion"
	previewsvc "github.com/usenorn/norn/internal/service/preview"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
	channelsvc "github.com/usenorn/norn/internal/service/runnerchannel"
)

type harness struct {
	channels   *channelrepo.MockRunnerChannel
	sessions   *sessionrepo.MockRunnerSession
	runners    *runnerrepo.MockRunner
	machines   *runnersvc.MockRunners
	executions *executionsvc.MockExecutions
	questions  *questionsvc.MockIssueQuestions
	changesets *changesetsvc.MockChangeSets
	previews   *previewsvc.MockPreviews
	services   *executionservicesvc.MockExecutionServices
	service    service.RunnerChannels

	runner  entity.Runner
	actor   entity.Actor
	readied []entity.Runner
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	runnerID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		channels:   channelrepo.NewMockRunnerChannel(ctrl),
		sessions:   sessionrepo.NewMockRunnerSession(ctrl),
		runners:    runnerrepo.NewMockRunner(ctrl),
		machines:   runnersvc.NewMockRunners(ctrl),
		executions: executionsvc.NewMockExecutions(ctrl),
		questions:  questionsvc.NewMockIssueQuestions(ctrl),
		changesets: changesetsvc.NewMockChangeSets(ctrl),
		previews:   previewsvc.NewMockPreviews(ctrl),
		services:   executionservicesvc.NewMockExecutionServices(ctrl),
		runner: entity.Runner{
			ID:          runnerID,
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
		actor: entity.Actor{
			Kind:      entity.ActorKindAgent,
			AccountID: uuid.New(),
			AgentID:   &agentID,
			RunnerID:  &runnerID,
		},
	}

	h.executions.EXPECT().Renew(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.executions.EXPECT().
		Ready(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, machine entity.Runner) error {
			h.readied = append(h.readied, machine)

			return nil
		}).
		AnyTimes()

	h.service = channelsvc.New(
		h.channels, h.sessions, h.runners, h.machines, h.executions, h.questions, h.changesets,
		h.previews, h.services,
	)

	return h
}

func (h *harness) session() service.ChannelSession {
	return service.ChannelSession{Runner: h.runner, Actor: h.actor, Epoch: "epoch", Cursor: "0-0"}
}

func (h *harness) opening(ticket string, leased ...string) {
	h.sessions.EXPECT().
		RedeemTicket(gomock.Any(), entity.HashRunnerSecret(ticket)).
		Return(h.runner.ID, nil)

	h.machines.EXPECT().ActorFor(gomock.Any(), h.runner.ID).Return(h.actor, h.runner, nil)
	h.channels.EXPECT().Attach(gomock.Any(), h.runner.ID, gomock.Any(), gomock.Any()).Return(nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), h.runner.ID, gomock.Any()).Return(nil).AnyTimes()
	h.channels.EXPECT().Cursor(gomock.Any(), h.runner.ID).Return("0-0", nil)

	if leased == nil {
		leased = []string{}
	}

	h.executions.EXPECT().Leased(gomock.Any(), h.runner.ID).Return(leased, nil)
}

func (h *harness) freshMessage(id string, kind entity.ChannelMessageType) entity.ChannelMessage {
	return entity.ChannelMessage{ID: id, Type: kind, IssuedAt: time.Now().UTC()}
}
