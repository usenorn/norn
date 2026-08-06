package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrWorkspaceAuthMethodNotPermitted = errors.New("workspace does not accept this authentication method")
	ErrEnforcementRefused              = errors.New("single sign-on cannot be required for this workspace yet")
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

func (e AuthEnforcement) PermitsActor(actor Actor, workspaceID uuid.UUID) bool {
	if actor.Kind == ActorKindToken || actor.Kind == ActorKindAgent {
		return true
	}

	if actor.Anonymous() {
		return e == AuthEnforcementAny
	}

	if !e.Permits(actor.AuthMethod) {
		return false
	}

	if e == AuthEnforcementSSO && actor.SSOWorkspaceID != workspaceID {
		return false
	}

	return true
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

type EnforcementBlocker string

const (
	EnforcementBlockerNoConnection  EnforcementBlocker = "no_connection"
	EnforcementBlockerNotVerified   EnforcementBlocker = "not_verified"
	EnforcementBlockerNoLinkedAdmin EnforcementBlocker = "no_linked_admin"
)

type EnforcementRefusedError struct {
	Blocker EnforcementBlocker
}

func (e EnforcementRefusedError) Error() string {
	switch e.Blocker {
	case EnforcementBlockerNoConnection:
		return "this workspace has no single sign-on provider to require"
	case EnforcementBlockerNotVerified:
		return "test the provider before requiring it, so a broken one cannot lock everyone out"
	case EnforcementBlockerNoLinkedAdmin:
		return "no administrator has signed in through the provider yet, so requiring it would " +
			"leave nobody able to administer this workspace"
	default:
		return "single sign-on cannot be required for this workspace yet"
	}
}

func (e EnforcementRefusedError) Unwrap() error { return ErrEnforcementRefused }
