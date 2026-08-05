package service

import (
	"time"

	"github.com/usenorn/norn/internal/entity"
)

type MintAPITokenInput struct {
	Name      string
	Scopes    entity.APIScopeSet
	Grants    entity.APITokenGrants
	ExpiresAt *time.Time
}

type MintedAPIToken struct {
	Token entity.APIToken
	Value string
}

type OwnedAPIToken struct {
	Token      entity.APIToken
	OwnerName  string
	OwnerEmail string
}
