package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrWorkspaceAuthMethodNotPermitted = errors.New("workspace does not accept this authentication method")
	ErrWorkspacePasswordAuthDisabled   = errors.New("workspace policy disables password authentication")
)

type AuthEnforcement string

const (
	AuthEnforcementAny AuthEnforcement = "any"
	AuthEnforcementSSO AuthEnforcement = "sso"
)

func (e AuthEnforcement) Valid() bool {
	switch e {
	case AuthEnforcementAny, AuthEnforcementSSO:
		return true
	default:
		return false
	}
}

func (e AuthEnforcement) Permits(method SessionAuthMethod) bool {
	switch e {
	case AuthEnforcementAny:
		return method.Valid()
	case AuthEnforcementSSO:
		return method == SessionAuthMethodSSO
	default:
		return false
	}
}

type WorkspaceAuthPolicy struct {
	WorkspaceID uuid.UUID
	Enforcement AuthEnforcement
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func DefaultWorkspaceAuthPolicy(workspaceID uuid.UUID) WorkspaceAuthPolicy {
	return WorkspaceAuthPolicy{WorkspaceID: workspaceID, Enforcement: AuthEnforcementAny}
}

func SSOEnforcedEverywhere(enforcements []AuthEnforcement) bool {
	if len(enforcements) == 0 {
		return false
	}

	for _, enforcement := range enforcements {
		if enforcement != AuthEnforcementSSO {
			return false
		}
	}

	return true
}
