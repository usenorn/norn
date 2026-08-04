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
	ResourceLabel          Resource = "label"
	ResourceAPIToken       Resource = "api_token"
	ResourceAuthPolicy     Resource = "auth_policy"
	ResourceAccount        Resource = "account"
	ResourceSession        Resource = "session"
	ResourceInstance       Resource = "instance"
)

func (r Resource) GatedByAuthMethod() bool {
	return r != ResourceAuthPolicy
}

func (r Resource) Conceals() bool {
	return r == ResourceIssue || r == ResourceTeam
}

func (r Resource) NotFound() error {
	switch r {
	case ResourceIssue:
		return ErrIssueNotFound
	case ResourceTeam:
		return ErrTeamNotFound
	default:
		return ErrAccountForbidden
	}
}

func (r Resource) PolicyName() string {
	if r == ResourceAuthPolicy {
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
	ActorKindUser  ActorKind = "user"
	ActorKindToken ActorKind = "token"
	ActorKindAgent ActorKind = "agent"
)

type Actor struct {
	Kind          ActorKind
	AccountID     uuid.UUID
	TokenID       *uuid.UUID
	WorkspaceID   *uuid.UUID
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

func (a Actor) ConfinedTo(workspaceID uuid.UUID) bool {
	return a.WorkspaceID == nil || *a.WorkspaceID == workspaceID
}

func (a Actor) Holds(permission Permission) bool {
	if a.Scopes == nil {
		return true
	}

	return a.Scopes.Permits(permission.Resource, permission.Action)
}

type AccessRequest struct {
	Resource    Resource
	Action      Action
	WorkspaceID uuid.UUID
	Subject     uuid.UUID
	Joining     bool
	Scoped      bool
}

type TeamScope struct {
	WorkspaceID uuid.UUID
	AllTeams    bool
	TeamIDs     []uuid.UUID
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
	default:
		return ErrAccountForbidden
	}
}
