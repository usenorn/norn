package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type MintAPITokenInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Scopes      entity.APIScopeSet
	ExpiresAt   *time.Time
}

type MintedAPIToken struct {
	Token entity.APIToken
	Value string
}
