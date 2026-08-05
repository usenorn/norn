package entity

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type Resource string

const (
	ResourceWorkspace      Resource = "workspace"
	ResourceMembership     Resource = "membership"
	ResourceInvitation     Resource = "invitation"
	ResourceTeam           Resource = "team"
	ResourceTeamMembership Resource = "team_membership"
	ResourceIssue          Resource = "issue"
	ResourceCycle          Resource = "cycle"
	ResourceProject        Resource = "project"
	ResourceLabel          Resource = "label"
	ResourceSavedView      Resource = "saved_view"
	ResourceComment        Resource = "comment"
	ResourceNotification   Resource = "notification"
	ResourceAPIToken       Resource = "api_token"
	ResourceAuthPolicy     Resource = "auth_policy"
	ResourceSSOConnection  Resource = "sso_connection"
	ResourceAccount        Resource = "account"
	ResourceSession        Resource = "session"
	ResourceInstance       Resource = "instance"
)

func (r Resource) GatedByAuthMethod() bool {
	return r != ResourceAuthPolicy && r != ResourceSSOConnection
}

func (r Resource) Conceals() bool {
	return r == ResourceIssue || r == ResourceTeam || r == ResourceCycle
}

func (r Resource) NotFound() error {
	switch r {
	case ResourceIssue:
		return ErrIssueNotFound
	case ResourceTeam:
		return ErrTeamNotFound
	case ResourceCycle:
		return ErrCycleNotFound
	default:
		return ErrAccountForbidden
	}
}

func (r Resource) PolicyName() string {
	if r == ResourceAuthPolicy || r == ResourceSSOConnection {
		return string(ResourceWorkspace)
	}

	return string(r)
}

type Action string

const (
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionManage Action = "manage"
)

func (a Action) RequiresLiveWorkspace() bool {
	return a != ActionRead && a != ActionDelete
}

type Permission struct {
	Resource Resource
	Action   Action
}

func NewPermission(resource Resource, action Action) Permission {
	return Permission{Resource: resource, Action: action}
}

type ActorKind string

const (
	ActorKindUser   ActorKind = "user"
	ActorKindToken  ActorKind = "token"
	ActorKindAgent  ActorKind = "agent"
	ActorKindSystem ActorKind = "system"
)

type Actor struct {
	Kind          ActorKind
	AccountID     uuid.UUID
	TokenID       *uuid.UUID
	TokenName     string
	Grants        APITokenGrants
	AuthMethod    SessionAuthMethod
	Scopes        APIScopeSet
	InstanceAdmin bool
}

func UserActor(session Session) Actor {
	return Actor{
		Kind:       ActorKindUser,
		AccountID:  session.AccountID,
		AuthMethod: session.AuthMethod,
	}
}

func (a Actor) Anonymous() bool {
	return a.AccountID == uuid.Nil
}

// ConfinedTo reports whether this actor may act in a workspace at all. A nil grant list is a session
// actor, which is confined by membership rather than by anything carried on the actor.
func (a Actor) ConfinedTo(workspaceID uuid.UUID) bool {
	return a.Grants == nil || a.Grants.Covers(workspaceID)
}

func (a Actor) Holds(permission Permission) bool {
	if a.Scopes == nil {
		return true
	}

	return a.Scopes.Permits(permission.Resource, permission.Action)
}

// NarrowScope intersects a scope resolved from the owner's memberships with what this actor's grant
// allows in that workspace. A token can only ever see less than its owner, never more, so the grant
// narrows the resolved scope and is never substituted for it.
func (a Actor) NarrowScope(scope TeamScope) TeamScope {
	grant, ok := a.Grants.For(scope.WorkspaceID)
	if !ok || grant.AllTeams {
		return scope
	}

	narrowed := TeamScope{
		WorkspaceID:    scope.WorkspaceID,
		IncludePrivate: scope.IncludePrivate,
		TeamIDs:        make([]uuid.UUID, 0, len(grant.TeamIDs)),
	}

	for _, teamID := range grant.TeamIDs {
		if scope.Covers(teamID) {
			narrowed.TeamIDs = append(narrowed.TeamIDs, teamID)
		}
	}

	return narrowed
}

type AccessRequest struct {
	Resource    Resource
	Action      Action
	WorkspaceID uuid.UUID
	Subject     uuid.UUID
	Joining     bool
	Scoped      bool
}

// TeamScope carries two independent questions. AllTeams says the holder reaches every team in the
// workspace; IncludePrivate says they may see private ones. An admin holding both is the common
// case, but a token pinned to named teams keeps the second while losing the first, so they cannot
// be collapsed into one field.
type TeamScope struct {
	WorkspaceID    uuid.UUID
	AllTeams       bool
	IncludePrivate bool
	TeamIDs        []uuid.UUID
}

func (s TeamScope) Covers(teamID uuid.UUID) bool {
	return s.AllTeams || slices.Contains(s.TeamIDs, teamID)
}

type Decision struct {
	Actor     Actor
	Role      MembershipRole
	Workspace Workspace
	Scope     TeamScope
}

// ActivityAttribution is who a record says did something. A token acts for its owner, so both are
// carried: the account answers "whose authority" and the token answers "which credential".
type ActivityAttribution struct {
	AccountID uuid.UUID
	Kind      ActorKind
	TokenID   *uuid.UUID
	TokenName string
}

func (d Decision) ActivityActor() ActivityAttribution {
	if d.Actor.Anonymous() {
		return ActivityAttribution{Kind: ActorKindSystem}
	}

	kind := d.Actor.Kind
	if kind == "" {
		kind = ActorKindUser
	}

	return ActivityAttribution{
		AccountID: d.Actor.AccountID,
		Kind:      kind,
		TokenID:   d.Actor.TokenID,
		TokenName: d.Actor.TokenName,
	}
}

type DenyReason string

const (
	DenyReasonNoActor                DenyReason = "no_actor"
	DenyReasonNotAMember             DenyReason = "not_a_member"
	DenyReasonRoleLacksAction        DenyReason = "role_lacks_action"
	DenyReasonAuthMethodNotPermitted DenyReason = "auth_method_not_permitted"
	DenyReasonTokenPermissionMissing DenyReason = "token_permission_missing"
	DenyReasonTokenWorkspaceMismatch DenyReason = "token_workspace_mismatch"
	DenyReasonWorkspaceDeleted       DenyReason = "workspace_deleted"
	DenyReasonInstanceAdminRequired  DenyReason = "instance_admin_required"
	DenyReasonNotSelf                DenyReason = "not_self"
	DenyReasonUnknownRole            DenyReason = "unknown_role"
)

func (r DenyReason) Conceals() bool {
	return r == DenyReasonNotAMember
}

func (r DenyReason) Disclosed() bool {
	switch r {
	case DenyReasonRoleLacksAction,
		DenyReasonAuthMethodNotPermitted,
		DenyReasonTokenPermissionMissing,
		DenyReasonTokenWorkspaceMismatch,
		DenyReasonInstanceAdminRequired:
		return true
	default:
		return false
	}
}

type AccessDeniedError struct {
	Reason      DenyReason
	Resource    Resource
	Action      Action
	WorkspaceID uuid.UUID
	ActorKind   ActorKind
	AccountID   uuid.UUID
	TokenID     uuid.UUID
	PurgeAfter  *time.Time
}

func (e AccessDeniedError) Error() string {
	return e.surface().Error()
}

func (e AccessDeniedError) Unwrap() error {
	return e.surface()
}

func (e AccessDeniedError) surface() error {
	switch {
	case e.Reason == DenyReasonWorkspaceDeleted:
		return WorkspaceDeletedError{PurgeAfter: e.PurgeAfter}
	case e.Reason.Conceals() && e.Resource.Conceals():
		return e.Resource.NotFound()
	case e.Reason == DenyReasonAuthMethodNotPermitted:
		return ErrWorkspaceAuthMethodNotPermitted
	default:
		return ErrAccountForbidden
	}
}
