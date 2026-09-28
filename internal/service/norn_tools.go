package service

import (
	"context"
	"encoding/json"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=norn_tools.go -destination=hostedagent/mock_norn_tools.go -package=hostedagent -mock_names=NornTools=MockNornTools

type NornTools interface {
	Catalog() []entity.AIToolDefinition
	Instructions() string
	Call(ctx context.Context, name string, arguments json.RawMessage) (entity.AIToolOutcome, error)
}
