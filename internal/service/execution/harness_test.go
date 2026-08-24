package execution_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	changesetrepo "github.com/usenorn/norn/internal/repository/changeset"
	codebaserepo "github.com/usenorn/norn/internal/repository/codebase"
	executionrepo "github.com/usenorn/norn/internal/repository/execution"
	executionservicerepo "github.com/usenorn/norn/internal/repository/executionservice"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	previewrepo "github.com/usenorn/norn/internal/repository/preview"
	runnerrepo "github.com/usenorn/norn/internal/repository/runner"
	channelrepo "github.com/usenorn/norn/internal/repository/runnerchannel"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	statesrepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
)

type harness struct {
	executions  *executionrepo.MockExecution
	changesets  *changesetrepo.MockChangeSet
	previews    *previewrepo.MockPreview
	services    *executionservicerepo.MockExecutionService
	runners     *runnerrepo.MockRunner
	codebases   *codebaserepo.MockCodebase
	issues      *issuerepo.MockIssue
	states      *statesrepo.MockWorkflowState
	channels    *channelrepo.MockRunnerChannel
	writer      *issuesvc.MockIssues
	source      *scmsvc.MockSourceControl
	branch      string
	branchFails error
	events      *eventsvc.MockEvents
	authorizer  *authorizersvc.MockAuthorizer
	audit       *auditsvc.MockAudit
	service     service.Executions

	workspaceID uuid.UUID
	issue       entity.Issue
	runner      entity.Runner
	caller      uuid.UUID
	callerAgent *uuid.UUID
	codebase    uuid.UUID
	spooled     []entity.ChannelMessage
	bound       []entity.Execution
	recorded    []entity.ExecutionEvent
	published   []entity.Event
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		executions:  executionrepo.NewMockExecution(ctrl),
		changesets:  changesetrepo.NewMockChangeSet(ctrl),
		previews:    previewrepo.NewMockPreview(ctrl),
		services:    executionservicerepo.NewMockExecutionService(ctrl),
		runners:     runnerrepo.NewMockRunner(ctrl),
		codebases:   codebaserepo.NewMockCodebase(ctrl),
		codebase:    uuid.New(),
		issues:      issuerepo.NewMockIssue(ctrl),
		states:      statesrepo.NewMockWorkflowState(ctrl),
		channels:    channelrepo.NewMockRunnerChannel(ctrl),
		writer:      issuesvc.NewMockIssues(ctrl),
		source:      scmsvc.NewMockSourceControl(ctrl),
		branch:      "rae/norn-1-a-run",
		events:      eventsvc.NewMockEvents(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		workspaceID: workspaceID,
		caller:      uuid.New(),
		issue: entity.Issue{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			TeamID:       teamID,
			ReferenceKey: "NORN",
			Number:       34,
			Title:        "Execution lifecycle",
			Version:      3,
		},
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
			Authority:   entity.RequestedAuthority{AllTeams: true},
		},
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()
	h.events.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, event entity.Event) {
			h.published = append(h.published, event)
		}).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), workspaceID, h.issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, _ entity.TeamScope) (entity.Issue, error) {
			return h.issue, nil
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			kind := entity.ActorKindUser
			if h.callerAgent != nil {
				kind = entity.ActorKindAgent
			}

			return entity.Decision{
				Actor: entity.Actor{Kind: kind, AccountID: h.caller, AgentID: h.callerAgent},
				Role:  entity.MembershipRoleAdmin,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		AppendEvent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, event entity.ExecutionEvent) (entity.ExecutionEvent, error) {
			for _, recorded := range h.recorded {
				if event.SourceID != "" && recorded.SourceID == event.SourceID {
					return entity.ExecutionEvent{}, entity.ErrExecutionEventRecorded
				}
			}

			event.ID = uuid.New()
			event.Sequence = int64(len(h.recorded)) + 1
			event.RecordedAt = time.Now().UTC()
			h.recorded = append(h.recorded, event)

			return event, nil
		}).
		AnyTimes()

	h.channels.EXPECT().
		Append(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, message entity.ChannelMessage) (string, error) {
			h.spooled = append(h.spooled, message)

			return "1-0", nil
		}).
		AnyTimes()

	h.previews.EXPECT().
		ByExecution(gomock.Any(), gomock.Any()).
		Return([]entity.PreviewSession{}, nil).
		AnyTimes()

	h.previews.EXPECT().
		CloseByExecution(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.source.EXPECT().
		BranchNameForAgent(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.Issue, _ uuid.UUID) (string, error) {
			return h.branch, h.branchFails
		}).
		AnyTimes()

	h.service = executionsvc.New(
		h.executions, h.changesets, h.previews, h.services, h.runners, h.codebases, h.issues,
		h.states,
		h.channels, h.writer, h.source, h.events, h.authorizer, h.audit, transactor,
	)

	return h
}

func (h *harness) execution(state entity.ExecutionState) entity.Execution {
	return entity.Execution{
		ID:             "exec-01ABC",
		WorkspaceID:    h.workspaceID,
		IssueID:        h.issue.ID,
		IssueReference: h.issue.Reference(),
		TeamID:         h.issue.TeamID,
		DelegationID:   uuid.New(),
		AgentID:        h.runner.AgentID,
		RunnerID:       h.runner.ID,
		Attempt:        1,
		State:          state,
		QueuedAt:       time.Now().UTC(),
	}
}

func (h *harness) live(machine entity.Runner, capacity, used int) {
	h.channels.EXPECT().
		Presence(gomock.Any(), machine.ID).
		Return(entity.RunnerPresence{
			RunnerID: machine.ID,
			Epoch:    "live",
			Load:     entity.RunnerLoad{Capacity: capacity, Used: used},
		}, nil).
		AnyTimes()

	h.executions.EXPECT().
		CountHeldSlots(gomock.Any(), machine.ID).
		Return(used, nil).
		AnyTimes()

	h.codebases.EXPECT().
		ListByRunnerID(gomock.Any(), machine.ID).
		Return([]entity.Codebase{{
			ID:       h.codebase,
			RunnerID: machine.ID,
			State:    entity.CodebaseStateActive,
		}}, nil).
		AnyTimes()
}

func (h *harness) offline(machine entity.Runner) {
	h.channels.EXPECT().
		Presence(gomock.Any(), machine.ID).
		Return(entity.RunnerPresence{RunnerID: machine.ID}, nil).
		AnyTimes()

	h.executions.EXPECT().
		CountHeldSlots(gomock.Any(), machine.ID).
		Return(0, nil).
		AnyTimes()
}

func (h *harness) binding() {
	h.executions.EXPECT().
		Bind(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, id string, binding repository.ExecutionBinding,
		) (entity.Execution, error) {
			bound := h.execution(entity.ExecutionQueued)
			bound.ID = id
			bound.RunnerID = binding.RunnerID
			bound.CodebaseID = binding.CodebaseID
			bound.QueuedReason = binding.QueuedReason
			h.bound = append(h.bound, bound)

			return bound, nil
		}).
		AnyTimes()
}

func (h *harness) opening(attempt int) *repository.NewExecution {
	created := &repository.NewExecution{}

	h.executions.EXPECT().NextAttempt(gomock.Any(), h.issue.ID).Return(attempt, nil)

	h.executions.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, request repository.NewExecution,
		) (entity.Execution, error) {
			*created = request
			opened := h.execution(entity.ExecutionQueued)
			opened.ID = request.ID
			opened.Attempt = request.Attempt
			opened.Params = request.Params
			opened.RunnerID = request.RunnerID
			opened.CodebaseID = request.CodebaseID
			opened.QueuedReason = request.QueuedReason

			return opened, nil
		})

	return created
}

func (h *harness) holding(execution entity.Execution) {
	h.executions.EXPECT().GetByID(gomock.Any(), execution.ID).Return(execution, nil).AnyTimes()
}

func (h *harness) moving() {
	h.executions.EXPECT().
		Move(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, id string, move repository.ExecutionMove,
		) (entity.Execution, error) {
			moved := h.execution(move.To)
			moved.ID = id
			moved.Reason = move.Reason
			moved.LeaseExpiresAt = move.LeaseExpiresAt

			return moved, nil
		}).
		AnyTimes()
}

func (h *harness) sent(kind entity.ChannelMessageType) (entity.ChannelMessage, bool) {
	for _, message := range h.spooled {
		if message.Type == kind {
			return message, true
		}
	}

	return entity.ChannelMessage{}, false
}

func (h *harness) announced(kind entity.EventKind) (entity.Event, bool) {
	for _, event := range h.published {
		if event.Kind == kind {
			return event, true
		}
	}

	return entity.Event{}, false
}

func (h *harness) states34() []entity.WorkflowState {
	return []entity.WorkflowState{
		{ID: uuid.New(), TeamID: h.issue.TeamID, Name: "Todo", Category: entity.StateCategoryNotStarted, Position: 1},
		{ID: uuid.New(), TeamID: h.issue.TeamID, Name: "In progress", Category: entity.StateCategoryActive, Position: 2},
		{ID: uuid.New(), TeamID: h.issue.TeamID, Name: "In review", Category: entity.StateCategoryActive, Position: 3},
		{ID: uuid.New(), TeamID: h.issue.TeamID, Name: "Done", Category: entity.StateCategoryComplete, Position: 4, IsCompletion: true},
	}
}

func message(id string, kind entity.ChannelMessageType, executionID string, payload any) entity.ChannelMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return entity.ChannelMessage{
		ID:          id,
		Type:        kind,
		ExecutionID: executionID,
		IssuedAt:    time.Now().UTC(),
		Payload:     body,
	}
}

func (h *harness) moved(to entity.ExecutionState) (entity.ExecutionEvent, bool) {
	for _, event := range h.recorded {
		if event.Kind == entity.ExecutionEventTransition && event.ToState == to {
			return event, true
		}
	}

	return entity.ExecutionEvent{}, false
}

func (h *harness) entry(kind entity.ExecutionEventKind) (entity.ExecutionEvent, bool) {
	for _, event := range h.recorded {
		if event.Kind == kind {
			return event, true
		}
	}

	return entity.ExecutionEvent{}, false
}
