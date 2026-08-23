package entity

import (
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTunnelMissing  = errors.New("that machine is not holding a preview tunnel")
	ErrTunnelCrowded  = errors.New("that machine is already carrying as much as it may")
	ErrTunnelRefused  = errors.New("that machine is no longer running this preview")
	ErrGatewayUnready = errors.New("this gateway holds no credential for norn yet")
)

type PreviewAsk struct {
	Host      string
	Grant     string
	IP        netip.Addr
	UserAgent string
}

type PreviewReply struct {
	Verdict     PreviewVerdict
	ExecutionID string
	RunnerID    uuid.UUID
	Preview     string
	Mode        PreviewMode
	Path        string
	Reason      string
	Redirect    string
	ExpiresAt   time.Time
}

type PreviewGrantReply struct {
	Grant     string
	Cookie    string
	Path      string
	ExpiresAt time.Time
}

type PreviewGatewayToken struct {
	Token     string
	ExpiresAt time.Time
}

func (t PreviewGatewayToken) Live(now time.Time, lead time.Duration) bool {
	return t.Token != "" && t.ExpiresAt.After(now.Add(lead))
}

type TunnelClaim struct {
	RunnerID    uuid.UUID
	WorkspaceID uuid.UUID
	Runner      string
}

type GatewayReach string

const (
	GatewayReachable    GatewayReach = "reachable"
	GatewayUnreachable  GatewayReach = "unreachable"
	GatewayUnconfigured GatewayReach = "unconfigured"
)

func GatewayReaches() []GatewayReach {
	return []GatewayReach{GatewayReachable, GatewayUnreachable, GatewayUnconfigured}
}

func (r GatewayReach) Valid() bool {
	return slices.Contains(GatewayReaches(), r)
}
