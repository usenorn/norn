package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=runner_session.go -destination=runnersession/mock_runner_session.go -package=runnersession -mock_names=RunnerSession=MockRunnerSession

type RunnerSession interface {
	ClaimNonce(ctx context.Context, runnerID uuid.UUID, nonce string) (bool, error)
	Grant(ctx context.Context, tokenHash []byte, runnerID uuid.UUID, ttl time.Duration) error
	Resolve(ctx context.Context, tokenHash []byte) (uuid.UUID, error)
	IssueTicket(ctx context.Context, ticketHash []byte, runnerID uuid.UUID, ttl time.Duration) error
	RedeemTicket(ctx context.Context, ticketHash []byte) (uuid.UUID, error)
	IssueTunnelTicket(ctx context.Context, ticketHash []byte, runnerID uuid.UUID, ttl time.Duration) error
	RedeemTunnelTicket(ctx context.Context, ticketHash []byte) (uuid.UUID, error)
}
