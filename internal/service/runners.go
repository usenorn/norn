package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=runners.go -destination=runner/mock_runners.go -package=runner -mock_names=Runners=MockRunners

type EnrolRunnerInput struct {
	Name      string
	PublicKey string
	Host      entity.RunnerHost
}

type EnrolledRunner struct {
	Runner       entity.Runner
	RefreshToken string
}

type ExchangeRunnerTokenInput struct {
	RefreshToken string
	RunnerID     uuid.UUID
	Nonce        string
	IssuedAt     time.Time
	Audience     string
	Signature    string
}

type RunnerSession struct {
	Runner        entity.Runner
	AccessToken   string
	AccessTTL     time.Duration
	Ticket        string
	TicketTTL     time.Duration
	TunnelTicket  string
	TunnelTTL     time.Duration
	Gateway       string
	PreviewDomain string
	PreviewScheme string
}

type RunnerLoad struct {
	Connected    bool
	Capacity     int
	Used         int
	Free         int
	DiskPressure bool
}

type RunnerState struct {
	Runner entity.Runner
	Load   RunnerLoad
}

func LoadOf(presence entity.RunnerPresence) RunnerLoad {
	if !presence.Live() {
		return RunnerLoad{}
	}

	return RunnerLoad{
		Connected:    true,
		Capacity:     presence.Load.Capacity,
		Used:         presence.Load.Used,
		Free:         presence.Free(),
		DiskPressure: presence.Load.DiskPressure,
	}
}

type Runners interface {
	Enrol(ctx context.Context, input EnrolRunnerInput) (EnrolledRunner, error)
	Exchange(ctx context.Context, input ExchangeRunnerTokenInput) (RunnerSession, error)
	Authenticate(ctx context.Context, token string) (entity.Actor, error)
	AcceptTunnel(ctx context.Context, ticket string) (entity.Runner, error)
	ActorFor(ctx context.Context, runnerID uuid.UUID) (entity.Actor, entity.Runner, error)
	Self(ctx context.Context) (entity.Runner, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]RunnerState, error)
	Pause(ctx context.Context, workspaceID, runnerID uuid.UUID) (entity.Runner, error)
	Resume(ctx context.Context, workspaceID, runnerID uuid.UUID) (entity.Runner, error)
	Revoke(ctx context.Context, workspaceID, runnerID uuid.UUID) error
}
