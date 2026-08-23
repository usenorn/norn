package executionservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type servicesService struct {
	services   repository.ExecutionService
	executions repository.Execution
	runs       service.Executions
	events     service.Events
	transactor repository.Transactor
}

func New(
	services repository.ExecutionService,
	executions repository.Execution,
	runs service.Executions,
	events service.Events,
	transactor repository.Transactor,
) service.ExecutionServices {
	return &servicesService{
		services:   services,
		executions: executions,
		runs:       runs,
		events:     events,
		transactor: transactor,
	}
}

func (s *servicesService) Reported(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.runs.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.Service

	if err := decode(message.Payload, &incoming); err != nil {
		return err
	}

	reported := reportedAt(message, incoming.Occurred)

	reporting := entity.ExecutionService{
		ExecutionID: execution.ID,
		WorkspaceID: execution.WorkspaceID,
		Name:        strings.TrimSpace(incoming.Name),
		State:       entity.ExecutionServiceState(incoming.State),
		Probe:       entity.ExecutionServiceProbe(incoming.Probe),
		Port:        incoming.Port,
		Reason:      strings.TrimSpace(incoming.Reason),
		ReportedAt:  reported,
	}

	if err := entity.ValidateExecutionService("service", reporting); err != nil {
		return err
	}

	if err := s.room(ctx, reporting); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		saved, err := s.services.Save(ctx, reporting)
		if err != nil {
			if errors.Is(err, entity.ErrExecutionServiceStale) {
				return nil
			}

			return err
		}

		return s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventService,
			Actor:       runnerActor(runner),
			Reason:      standing(saved),
			Detail:      serviceDetail(saved),
			SourceID:    message.ID,
			OccurredAt:  reported,
		})
	})
}

func (s *servicesService) ForExecution(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]entity.ExecutionService, error) {
	if _, err := s.runs.Visible(ctx, workspaceID, executionID); err != nil {
		return nil, err
	}

	return s.services.ByExecution(ctx, executionID)
}

func (s *servicesService) room(ctx context.Context, reporting entity.ExecutionService) error {
	held, err := s.services.Count(ctx, reporting.ExecutionID)
	if err != nil {
		return err
	}

	if held < entity.ExecutionServicesMax {
		return nil
	}

	running, err := s.services.ByExecution(ctx, reporting.ExecutionID)
	if err != nil {
		return err
	}

	for _, known := range running {
		if known.Name == reporting.Name {
			return nil
		}
	}

	return fmt.Errorf("%w: %d", entity.ErrExecutionServiceCrowded, entity.ExecutionServicesMax)
}

func (s *servicesService) remember(
	ctx context.Context,
	execution entity.Execution,
	event entity.ExecutionEvent,
) error {
	appended, err := s.executions.AppendEvent(ctx, event)
	if err != nil {
		if errors.Is(err, entity.ErrExecutionEventRecorded) {
			return nil
		}

		return err
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) { s.announce(ctx, execution, appended) })

	return nil
}

func (s *servicesService) announce(
	ctx context.Context,
	execution entity.Execution,
	event entity.ExecutionEvent,
) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.events.Publish(ctx, entity.Event{
		WorkspaceID: execution.WorkspaceID,
		Kind:        entity.EventExecutionEvent,
		TeamID:      execution.TeamID,
		SubjectID:   execution.IssueID,
		IssueID:     execution.IssueID,
		Payload:     payload,
	})
}

func standing(reported entity.ExecutionService) string {
	line := reported.Name + " is " + string(reported.State)

	if reported.Port > 0 {
		line += " on port " + strconv.Itoa(reported.Port)
	}

	if reported.Reason != "" {
		line += ": " + reported.Reason
	}

	return line
}

func serviceDetail(reported entity.ExecutionService) []byte {
	encoded, err := json.Marshal(map[string]any{
		"service": reported.Name,
		"state":   string(reported.State),
		"probe":   string(reported.Probe),
		"port":    reported.Port,
	})
	if err != nil {
		return nil
	}

	return encoded
}

func runnerActor(runner entity.Runner) entity.ExecutionActor {
	return entity.ExecutionActor{
		Kind:     entity.ActorKindAgent,
		AgentID:  runner.AgentID,
		RunnerID: runner.ID,
	}
}

func reportedAt(message entity.ChannelMessage, occurred time.Time) time.Time {
	if !occurred.IsZero() {
		return occurred.UTC()
	}

	if message.IssuedAt.IsZero() {
		return time.Now().UTC()
	}

	return message.IssuedAt.UTC()
}

func decode(payload []byte, into any) error {
	if len(payload) == 0 {
		return nil
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return entity.ErrChannelEnvelopeInvalid
	}

	return nil
}
