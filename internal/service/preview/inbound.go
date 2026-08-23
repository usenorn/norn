package preview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *previewsService) Reported(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.runs.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.Preview

	if err := decode(message.Payload, &incoming); err != nil {
		return err
	}

	reported := reportedAt(message, incoming.Occurred)

	preview := entity.PreviewSession{
		ExecutionID: execution.ID,
		WorkspaceID: execution.WorkspaceID,
		Name:        strings.TrimSpace(incoming.Name),
		Service:     strings.TrimSpace(incoming.Service),
		Path:        strings.TrimSpace(incoming.Path),
		Mode:        entity.PreviewBySubdomain,
		State:       entity.PreviewState(incoming.State),
		OpenedAt:    reported,
		ReportedAt:  reported,
	}

	preview.Host = entity.PreviewHost(
		preview.Name, execution.ID, preview.Mode, s.settings.BaseDomain,
	)

	if preview.State == entity.PreviewClosed {
		preview.ClosedAt = reported
	}

	if err := entity.ValidatePreviewSession("preview", preview); err != nil {
		return err
	}

	if err := s.room(ctx, preview); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		saved, err := s.previews.Save(ctx, preview)
		if err != nil {
			return err
		}

		return s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventPreview,
			Actor:       runnerActor(runner),
			Reason:      opening(saved, s.settings),
			Detail:      previewDetail(saved, s.settings),
			SourceID:    message.ID,
			OccurredAt:  reported,
		})
	})
}

func (s *previewsService) room(ctx context.Context, preview entity.PreviewSession) error {
	if preview.State != entity.PreviewOpen {
		return nil
	}

	held, err := s.previews.Count(ctx, preview.ExecutionID)
	if err != nil {
		return err
	}

	if held < entity.PreviewsMax {
		return nil
	}

	_, err = s.previews.ByName(ctx, preview.ExecutionID, preview.Name)
	if errors.Is(err, entity.ErrPreviewNotFound) {
		return fmt.Errorf("%w: %d", entity.ErrPreviewCrowded, entity.PreviewsMax)
	}

	return err
}

func (s *previewsService) remember(
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

func (s *previewsService) announce(
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

func opening(preview entity.PreviewSession, settings config.Previews) string {
	if preview.State == entity.PreviewClosed {
		return preview.Name + " is closed"
	}

	address := preview.URL(settings.Scheme)
	if address == "" {
		return preview.Name + " is open on the service " + preview.Service +
			", and reaches nobody until this server serves a preview domain"
	}

	return preview.Name + " is open at " + address + ", on the service " + preview.Service
}

func previewDetail(preview entity.PreviewSession, settings config.Previews) []byte {
	return object(map[string]string{
		"preview": preview.Name,
		"service": preview.Service,
		"state":   string(preview.State),
		"url":     preview.URL(settings.Scheme),
	})
}

func object(detail map[string]string) []byte {
	encoded, err := json.Marshal(detail)
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
