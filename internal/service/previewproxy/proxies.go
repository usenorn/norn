package previewproxy

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type proxyService struct {
	norn    repository.Norn
	tunnels repository.Tunnel
	cfg     config.Gateway

	mu    sync.RWMutex
	held  entity.PreviewGatewayToken
	pause time.Duration
}

func New(norn repository.Norn, tunnels repository.Tunnel, cfg config.Gateway) service.PreviewProxy {
	return &proxyService{norn: norn, tunnels: tunnels, cfg: cfg, pause: cfg.RetryMin}
}

func (s *proxyService) Run(ctx context.Context) {
	for {
		wait := s.renew(ctx)

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			timer.Stop()

			return
		case <-timer.C:
		}
	}
}

func (s *proxyService) renew(ctx context.Context) time.Duration {
	if s.cfg.Secret == "" || s.cfg.Server == "" {
		return s.cfg.RetryMax
	}

	token, err := s.norn.Exchange(ctx, s.cfg.Secret)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"this gateway could not trade its secret with norn for an access token",
			slog.String("error", err.Error()),
		)

		return s.backoff()
	}

	s.mu.Lock()
	s.held = token
	s.pause = s.cfg.RetryMin
	s.mu.Unlock()

	return renewIn(token, time.Now().UTC(), s.cfg.RefreshLead, s.cfg.RetryMin)
}

func renewIn(
	token entity.PreviewGatewayToken,
	now time.Time,
	lead, floor time.Duration,
) time.Duration {
	within := token.ExpiresAt.Sub(now) - lead
	if within < floor {
		return floor
	}

	return within
}

func (s *proxyService) backoff() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	waiting := s.pause

	s.pause *= 2
	if s.pause > s.cfg.RetryMax {
		s.pause = s.cfg.RetryMax
	}

	spread := waiting / 4
	if spread <= 0 {
		return waiting
	}

	return waiting - time.Duration(rand.Int64N(int64(spread)))
}

func (s *proxyService) token() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.held.Token == "" {
		return "", entity.ErrGatewayUnready
	}

	return s.held.Token, nil
}

func (s *proxyService) Ready() bool {
	_, err := s.token()

	return err == nil
}

func (s *proxyService) Route(
	ctx context.Context,
	ask entity.PreviewAsk,
) (entity.PreviewReply, error) {
	token, err := s.token()
	if err != nil {
		return entity.PreviewReply{}, err
	}

	return s.norn.Introspect(ctx, token, ask)
}

func (s *proxyService) Session(
	ctx context.Context,
	ticket string,
) (entity.PreviewGrantReply, error) {
	token, err := s.token()
	if err != nil {
		return entity.PreviewGrantReply{}, err
	}

	return s.norn.Session(ctx, token, ticket)
}

func (s *proxyService) Redeem(
	ctx context.Context,
	host, share, passcode string,
) (entity.PreviewGrantReply, error) {
	token, err := s.token()
	if err != nil {
		return entity.PreviewGrantReply{}, err
	}

	return s.norn.Redeem(ctx, token, host, share, passcode)
}

func (s *proxyService) Accept(
	ctx context.Context,
	ticket string,
) (entity.TunnelClaim, error) {
	token, err := s.token()
	if err != nil {
		return entity.TunnelClaim{}, err
	}

	return s.norn.Tunnel(ctx, token, ticket)
}

func (s *proxyService) Hold(
	ctx context.Context,
	runnerID uuid.UUID,
	socket net.Conn,
) error {
	return s.tunnels.Hold(ctx, runnerID, socket)
}

func (s *proxyService) Dial(
	ctx context.Context,
	reply entity.PreviewReply,
) (net.Conn, error) {
	if reply.Verdict != entity.PreviewAllowed {
		return nil, entity.ErrTunnelRefused
	}

	return s.tunnels.Open(ctx, reply.RunnerID, channelv1.StreamOpen{
		Execution: reply.ExecutionID,
		Preview:   reply.Preview,
	})
}
