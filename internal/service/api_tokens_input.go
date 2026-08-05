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

// OwnedAPIToken is a token seen by a workspace administrator rather than by its owner, so it names
// the person it acts for. The owner's own listing never needs this.
type OwnedAPIToken struct {
	Token      entity.APIToken
	OwnerName  string
	OwnerEmail string
}
