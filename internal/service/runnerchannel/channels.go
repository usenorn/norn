package runnerchannel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type channelsService struct {
	channels   repository.RunnerChannel
	sessions   repository.RunnerSession
	runners    repository.Runner
	machines   service.Runners
	executions service.Executions
	questions  service.IssueQuestions
	changesets service.ChangeSets
	previews   service.Previews
}

func New(
	channels repository.RunnerChannel,
	sessions repository.RunnerSession,
	runners repository.Runner,
	machines service.Runners,
	executions service.Executions,
	questions service.IssueQuestions,
	changesets service.ChangeSets,
	previews service.Previews,
) service.RunnerChannels {
	return &channelsService{
		channels:   channels,
		sessions:   sessions,
		runners:    runners,
		machines:   machines,
		executions: executions,
		questions:  questions,
		changesets: changesets,
		previews:   previews,
	}
}

func (s *channelsService) Open(ctx context.Context, ticket string) (service.ChannelSession, error) {
	runnerID, err := s.sessions.RedeemTicket(ctx, entity.HashRunnerSecret(ticket))
	if err != nil {
		return service.ChannelSession{}, entity.ErrChannelTicketInvalid
	}

	actor, held, err := s.machines.ActorFor(ctx, runnerID)
	if err != nil {
		return service.ChannelSession{}, err
	}

	now := time.Now().UTC()
	epoch := ulid.Make().String()

	if err := s.channels.Attach(ctx, held.ID, epoch, now); err != nil {
		return service.ChannelSession{}, err
	}

	if err := s.runners.RecordSeen(ctx, held.ID, now); err != nil {
		return service.ChannelSession{}, err
	}

	cursor, err := s.channels.Cursor(ctx, held.ID)
	if err != nil {
		return service.ChannelSession{}, err
	}

	session := service.ChannelSession{Runner: held, Actor: actor, Epoch: epoch, Cursor: cursor}

	if err := s.sync(ctx, held.ID); err != nil {
		return service.ChannelSession{}, err
	}

	return session, nil
}

func (s *channelsService) Deliver(
	ctx context.Context,
	session service.ChannelSession,
	cursor string,
) ([]entity.SpooledMessage, string, error) {
	return s.channels.Read(ctx, session.Runner.ID, cursor)
}

func (s *channelsService) Acknowledge(
	ctx context.Context,
	session service.ChannelSession,
	cursor string,
) error {
	return s.channels.Acknowledge(ctx, session.Runner.ID, cursor)
}

func (s *channelsService) Heartbeat(
	ctx context.Context,
	session service.ChannelSession,
	load entity.RunnerLoad,
) error {
	now := time.Now().UTC()

	if err := s.channels.Renew(ctx, session.Runner.ID, session.Epoch, load, now); err != nil {
		return err
	}

	held, err := s.runners.GetByID(ctx, session.Runner.ID)
	if err != nil {
		return err
	}

	if held.Revoked() {
		return entity.ErrRunnerRevoked
	}

	if err := s.executions.Renew(ctx, held); err != nil {
		return err
	}

	if err := s.runners.RecordSeen(ctx, held.ID, now); err != nil {
		return err
	}

	return s.executions.Ready(ctx, held)
}

func (s *channelsService) Verify(ctx context.Context, session service.ChannelSession) error {
	held, err := s.channels.Presence(ctx, session.Runner.ID)
	if err != nil {
		return err
	}

	if held.Epoch != session.Epoch {
		return entity.ErrChannelDisplaced
	}

	machine, err := s.runners.GetByID(ctx, session.Runner.ID)
	if err != nil {
		return err
	}

	if machine.Revoked() {
		return entity.ErrRunnerRevoked
	}

	return nil
}

func (s *channelsService) Close(ctx context.Context, session service.ChannelSession) {
	if err := s.channels.Detach(ctx, session.Runner.ID, session.Epoch); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"releasing a runner channel failed",
			slog.String("runner_id", session.Runner.ID.String()),
			slog.String("error", err.Error()),
		)
	}
}

func (s *channelsService) sync(ctx context.Context, runnerID uuid.UUID) error {
	held, err := s.executions.Leased(ctx, runnerID)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(channelv1.Leased{Executions: held})
	if err != nil {
		return fmt.Errorf("encode the channel sync: %w", err)
	}

	message, err := entity.NewServerMessage(entity.ChannelSync, "", payload, time.Now().UTC())
	if err != nil {
		return err
	}

	if _, err := s.channels.Append(ctx, runnerID, message); err != nil {
		return err
	}

	return nil
}
