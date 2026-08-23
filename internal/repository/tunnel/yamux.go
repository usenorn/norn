package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type held struct {
	session *yamux.Session
	streams int
}

type tunnelRepository struct {
	cfg config.Gateway

	mu     sync.Mutex
	tunnel map[uuid.UUID]*held
}

func New(cfg config.Gateway) repository.Tunnel {
	return &tunnelRepository{cfg: cfg, tunnel: make(map[uuid.UUID]*held)}
}

func (r *tunnelRepository) Hold(
	ctx context.Context,
	runnerID uuid.UUID,
	socket net.Conn,
) error {
	settings := yamux.DefaultConfig()
	settings.KeepAliveInterval = r.cfg.Heartbeat
	settings.ConnectionWriteTimeout = r.cfg.RequestTimeout
	settings.LogOutput = io.Discard

	session, err := yamux.Server(socket, settings)
	if err != nil {
		return fmt.Errorf("open tunnel session: %w", err)
	}

	r.attach(runnerID, session)

	defer r.detach(runnerID, session)
	defer func() { _ = session.Close() }()

	select {
	case <-session.CloseChan():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *tunnelRepository) attach(runnerID uuid.UUID, session *yamux.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if displaced, holding := r.tunnel[runnerID]; holding {
		_ = displaced.session.Close()
	}

	r.tunnel[runnerID] = &held{session: session}
}

func (r *tunnelRepository) detach(runnerID uuid.UUID, session *yamux.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if holding, found := r.tunnel[runnerID]; found && holding.session == session {
		delete(r.tunnel, runnerID)
	}
}

func (r *tunnelRepository) Open(
	ctx context.Context,
	runnerID uuid.UUID,
	open channelv1.StreamOpen,
) (net.Conn, error) {
	holding, err := r.take(runnerID)
	if err != nil {
		return nil, err
	}

	stream, err := holding.session.OpenStream()
	if err != nil {
		r.give(runnerID)

		return nil, errors.Join(entity.ErrTunnelMissing, err)
	}

	carrying := &carried{Stream: channelv1.NewStream(stream), repo: r, runner: runnerID}

	if err := carrying.ask(ctx, open, r.cfg.StreamOpenTimeout); err != nil {
		_ = carrying.Close()

		return nil, err
	}

	return carrying, nil
}

func (r *tunnelRepository) take(runnerID uuid.UUID) (*held, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	holding, found := r.tunnel[runnerID]
	if !found {
		return nil, entity.ErrTunnelMissing
	}

	if holding.streams >= r.cfg.MaxStreamsPerRunner {
		return nil, entity.ErrTunnelCrowded
	}

	holding.streams++

	return holding, nil
}

func (r *tunnelRepository) give(runnerID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if holding, found := r.tunnel[runnerID]; found && holding.streams > 0 {
		holding.streams--
	}
}

func (r *tunnelRepository) Live(runnerID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, found := r.tunnel[runnerID]

	return found
}

func (r *tunnelRepository) Holding() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.tunnel)
}

type carried struct {
	*channelv1.Stream

	repo   *tunnelRepository
	runner uuid.UUID
	closed sync.Once
}

func (c *carried) ask(
	ctx context.Context,
	open channelv1.StreamOpen,
	within time.Duration,
) error {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > within {
		deadline = time.Now().Add(within)
	}

	if err := c.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set tunnel stream deadline: %w", err)
	}

	if err := c.WriteFrame(open); err != nil {
		return errors.Join(entity.ErrTunnelMissing, err)
	}

	var ready channelv1.StreamReady

	if err := c.ReadFrame(&ready); err != nil {
		return errors.Join(entity.ErrTunnelMissing, err)
	}

	if !ready.Open {
		return entity.ErrTunnelRefused
	}

	return c.SetDeadline(time.Time{})
}

func (c *carried) Close() error {
	c.closed.Do(func() { c.repo.give(c.runner) })

	return c.Stream.Close()
}
