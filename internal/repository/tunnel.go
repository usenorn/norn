package repository

import (
	"context"
	"net"

	"github.com/google/uuid"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

//go:generate go tool mockgen -source=tunnel.go -destination=tunnel/mock_tunnel.go -package=tunnel -mock_names=Tunnel=MockTunnel

type Tunnel interface {
	Hold(ctx context.Context, runnerID uuid.UUID, socket net.Conn) error
	Open(ctx context.Context, runnerID uuid.UUID, open channelv1.StreamOpen) (net.Conn, error)
	Live(runnerID uuid.UUID) bool
	Holding() int
}
