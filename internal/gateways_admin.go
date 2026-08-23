package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/service"
)

type GatewaysAdmin struct {
	gateways service.PreviewGateways
	out      io.Writer
}

func NewGatewaysAdmin(gateways service.PreviewGateways) *GatewaysAdmin {
	return &GatewaysAdmin{gateways: gateways, out: os.Stdout}
}

func (a *GatewaysAdmin) Enrol(ctx context.Context, name string) error {
	gateway, secret, err := a.gateways.Enrol(ctx, name)
	if err != nil {
		return err
	}

	return a.write(map[string]any{
		"id":        gateway.ID,
		"name":      gateway.Name,
		"status":    gateway.Status,
		"secret":    secret,
		"createdAt": gateway.CreatedAt,
	})
}

func (a *GatewaysAdmin) List(ctx context.Context) error {
	gateways, err := a.gateways.List(ctx)
	if err != nil {
		return err
	}

	return a.write(gateways)
}

func (a *GatewaysAdmin) Revoke(ctx context.Context, gatewayID string) error {
	parsed, err := uuid.Parse(gatewayID)
	if err != nil {
		return fmt.Errorf("parse preview gateway id: %w", err)
	}

	revoked, err := a.gateways.Revoke(ctx, parsed)
	if err != nil {
		return err
	}

	return a.write(revoked)
}

func (a *GatewaysAdmin) write(payload any) error {
	encoder := json.NewEncoder(a.out)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("write preview gateway output: %w", err)
	}

	return nil
}
