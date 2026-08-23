package executionservice_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	executionrepo "github.com/usenorn/norn/internal/repository/execution"
	executionservicerepo "github.com/usenorn/norn/internal/repository/executionservice"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	executionservicesvc "github.com/usenorn/norn/internal/service/executionservice"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type harness struct {
	services   *executionservicerepo.MockExecutionService
	executions *executionrepo.MockExecution
	runs       *executionsvc.MockExecutions
	events     *eventsvc.MockEvents
	service    service.ExecutionServices

	workspaceID uuid.UUID
	runner      entity.Runner
	execution   entity.Execution

	held      []entity.ExecutionService
	timeline  []entity.ExecutionEvent
	published []entity.Event
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		services:    executionservicerepo.NewMockExecutionService(ctrl),
		executions:  executionrepo.NewMockExecution(ctrl),
		runs:        executionsvc.NewMockExecutions(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		workspaceID: workspaceID,
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
	}

	h.execution = entity.Execution{
		ID:          "exec-01ABC",
		WorkspaceID: workspaceID,
		IssueID:     uuid.New(),
		TeamID:      teamID,
		AgentID:     agentID,
		RunnerID:    h.runner.ID,
		State:       entity.ExecutionRunning,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.expectStore()

	h.events.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, event entity.Event) { h.published = append(h.published, event) }).
		AnyTimes()

	h.service = executionservicesvc.New(
		h.services, h.executions, h.runs, h.events, transactor,
	)

	return h
}

func (h *harness) expectStore() {
	h.services.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, reporting entity.ExecutionService,
		) (entity.ExecutionService, error) {
			for index, known := range h.held {
				if known.Name != reporting.Name {
					continue
				}

				if reporting.ReportedAt.Before(known.ReportedAt) {
					return entity.ExecutionService{}, entity.ErrExecutionServiceStale
				}

				if reporting.Port == 0 {
					reporting.Port = known.Port
				}

				if reporting.Probe == "" {
					reporting.Probe = known.Probe
				}

				reporting.ID = known.ID
				h.held[index] = reporting

				return reporting, nil
			}

			reporting.ID = uuid.New()
			h.held = append(h.held, reporting)

			return reporting, nil
		}).
		AnyTimes()

	h.services.EXPECT().
		ByExecution(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) ([]entity.ExecutionService, error) {
			return h.held, nil
		}).
		AnyTimes()

	h.services.EXPECT().
		Count(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (int, error) {
			return len(h.held), nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		AppendEvent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, event entity.ExecutionEvent,
		) (entity.ExecutionEvent, error) {
			for _, recorded := range h.timeline {
				if event.SourceID != "" && recorded.SourceID == event.SourceID {
					return entity.ExecutionEvent{}, entity.ErrExecutionEventRecorded
				}
			}

			event.ID = uuid.New()
			event.Sequence = int64(len(h.timeline) + 1)
			h.timeline = append(h.timeline, event)

			return event, nil
		}).
		AnyTimes()
}

func (h *harness) holding() {
	h.runs.EXPECT().
		Held(gomock.Any(), h.runner, h.execution.ID).
		Return(h.execution, nil).
		AnyTimes()
}

func (h *harness) visible() {
	h.runs.EXPECT().
		Visible(gomock.Any(), h.workspaceID, h.execution.ID).
		Return(h.execution, nil).
		AnyTimes()
}

func (h *harness) message(payload channelv1.Service) entity.ChannelMessage {
	return h.messageWithID(uuid.NewString(), payload)
}

func (h *harness) messageWithID(id string, payload channelv1.Service) entity.ChannelMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return entity.ChannelMessage{
		ID:          id,
		Type:        entity.ChannelServiceState,
		ExecutionID: h.execution.ID,
		IssuedAt:    time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
		Payload:     body,
	}
}

func (h *harness) known(name string) (entity.ExecutionService, bool) {
	for _, known := range h.held {
		if known.Name == name {
			return known, true
		}
	}

	return entity.ExecutionService{}, false
}

func at(seconds int) time.Time {
	return time.Date(2026, 8, 23, 10, 0, seconds, 0, time.UTC)
}
