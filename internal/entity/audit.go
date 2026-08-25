package entity

import (
	"encoding/base64"
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	AuditPageDefaultSize = 50
	AuditPageMaxSize     = 200
	AuditExportBatch     = 500
)

var (
	ErrAuditNotPermitted  = errors.New("you may not read the audit log for this workspace")
	ErrAuditCursorInvalid = errors.New("audit cursor is invalid")
)

type AuditAction string

const (
	AuditSignedIn        AuditAction = "session.signed_in"
	AuditSignInFailed    AuditAction = "session.sign_in_failed"
	AuditSignInCodeSent  AuditAction = "session.sign_in_code_sent"
	AuditSignedOut       AuditAction = "session.signed_out"
	AuditSessionRevoked  AuditAction = "session.revoked"
	AuditPasswordChanged AuditAction = "account.password_changed"
	AuditPasswordReset   AuditAction = "account.password_reset"
	AuditEmailChanged    AuditAction = "account.email_changed"
	AuditAccountDisabled AuditAction = "account.deactivated"
	AuditAccountDeleted  AuditAction = "account.deleted"

	AuditMemberAdded       AuditAction = "membership.added"
	AuditMemberRoleChanged AuditAction = "membership.role_changed"
	AuditMemberRemoved     AuditAction = "membership.removed"
	AuditAuditAccess       AuditAction = "membership.audit_access_changed"
	AuditTeamMemberAdded   AuditAction = "team_membership.added"
	AuditTeamMemberRemoved AuditAction = "team_membership.removed"

	AuditInvitationCreated  AuditAction = "invitation.created"
	AuditInvitationRevoked  AuditAction = "invitation.revoked"
	AuditInvitationAccepted AuditAction = "invitation.accepted"

	AuditSSOConnectionSaved   AuditAction = "sso.connection_saved"
	AuditSSOConnectionRemoved AuditAction = "sso.connection_removed"
	AuditSSOEnforcement       AuditAction = "sso.enforcement_changed"
	AuditRecoveryCodesIssued  AuditAction = "sso.recovery_codes_issued"
	AuditRecoveryCodeRedeemed AuditAction = "sso.recovery_code_redeemed"
	AuditSSOIdentityUnlinked  AuditAction = "sso.identity_unlinked"
	AuditSSOIdentityLinked    AuditAction = "sso.identity_linked"
	AuditSSOIdentityRefused   AuditAction = "sso.identity_refused"
	AuditSSOAccountOpened     AuditAction = "sso.account_opened"

	AuditTokenMinted  AuditAction = "token.minted"
	AuditTokenRevoked AuditAction = "token.revoked"

	AuditAgentRegistered AuditAction = "agent.registered"
	AuditAgentDisabled   AuditAction = "agent.disabled"
	AuditAgentEnabled    AuditAction = "agent.enabled"
	AuditRunnerEnrolled  AuditAction = "runner.enrolled"
	AuditRunnerRevoked   AuditAction = "runner.revoked"
	AuditRunnerPaused    AuditAction = "runner.paused"
	AuditRunnerResumed   AuditAction = "runner.resumed"
	AuditAgentRotated    AuditAction = "agent.credential_rotated"
	AuditAgentProposal   AuditAction = "agent.proposal_decided"

	AuditCodebaseConnected    AuditAction = "codebase.connected"
	AuditCodebaseDisconnected AuditAction = "codebase.disconnected"

	AuditExecutionCancelled AuditAction = "execution.cancelled"
	AuditExecutionApproved  AuditAction = "execution.approved"
	AuditExecutionResumed   AuditAction = "execution.resumed"
	AuditExecutionStranded  AuditAction = "execution.stranded"
	AuditExecutionRetained  AuditAction = "execution.retained"

	AuditPreviewOpened       AuditAction = "preview.opened"
	AuditPreviewShared       AuditAction = "preview.shared"
	AuditPreviewShareRevoked AuditAction = "preview.share_revoked"

	AuditWebhookRegistered AuditAction = "webhook.registered"
	AuditWebhookRemoved    AuditAction = "webhook.removed"
	AuditWebhookDisabled   AuditAction = "webhook.disabled"

	AuditWorkspaceUpdated  AuditAction = "workspace.updated"
	AuditWorkspaceDeletion AuditAction = "workspace.deletion_requested"
	AuditWorkspaceRestored AuditAction = "workspace.restored"
	AuditWorkspacePurged   AuditAction = "workspace.purged"

	AuditSourceControlConnected     AuditAction = "source_control.connected"
	AuditSourceControlDisconnected  AuditAction = "source_control.disconnected"
	AuditSourceControlTokenReplaced AuditAction = "source_control.token_replaced"
	AuditSourceControlBroken        AuditAction = "source_control.broken"

	AuditDirectoryConnected    AuditAction = "directory.connected"
	AuditDirectoryDisconnected AuditAction = "directory.disconnected"
	AuditDirectoryTokenRotated AuditAction = "directory.token_rotated"

	AuditExported     AuditAction = "audit.exported"
	AuditAccessDenied AuditAction = "access.denied"
)

func AuditActions() []AuditAction {
	return []AuditAction{
		AuditSignedIn, AuditSignInFailed, AuditSignInCodeSent, AuditSignedOut, AuditSessionRevoked,
		AuditPasswordChanged, AuditPasswordReset, AuditEmailChanged,
		AuditAccountDisabled, AuditAccountDeleted,
		AuditMemberAdded, AuditMemberRoleChanged, AuditMemberRemoved, AuditAuditAccess,
		AuditTeamMemberAdded, AuditTeamMemberRemoved,
		AuditInvitationCreated, AuditInvitationRevoked, AuditInvitationAccepted,
		AuditSSOConnectionSaved, AuditSSOConnectionRemoved, AuditSSOEnforcement,
		AuditRecoveryCodesIssued, AuditRecoveryCodeRedeemed, AuditSSOIdentityUnlinked,
		AuditSSOIdentityLinked, AuditSSOIdentityRefused, AuditSSOAccountOpened,
		AuditTokenMinted, AuditTokenRevoked,
		AuditAgentRegistered, AuditAgentDisabled, AuditAgentEnabled, AuditAgentRotated,
		AuditAgentProposal,
		AuditRunnerEnrolled, AuditRunnerRevoked, AuditRunnerPaused, AuditRunnerResumed,
		AuditCodebaseConnected, AuditCodebaseDisconnected,
		AuditExecutionCancelled, AuditExecutionApproved, AuditExecutionResumed,
		AuditExecutionStranded, AuditExecutionRetained,
		AuditPreviewOpened, AuditPreviewShared, AuditPreviewShareRevoked,
		AuditWebhookRegistered, AuditWebhookRemoved, AuditWebhookDisabled,
		AuditWorkspaceUpdated, AuditWorkspaceDeletion, AuditWorkspaceRestored, AuditWorkspacePurged,
		AuditDirectoryConnected, AuditDirectoryDisconnected, AuditDirectoryTokenRotated,
		AuditSourceControlConnected, AuditSourceControlDisconnected,
		AuditSourceControlTokenReplaced, AuditSourceControlBroken,
		AuditExported, AuditAccessDenied,
	}
}

func (a AuditAction) Valid() bool {
	return slices.Contains(AuditActions(), a)
}

type AuditOutcome string

const (
	AuditSucceeded AuditOutcome = "succeeded"
	AuditFailed    AuditOutcome = "failed"
	AuditDenied    AuditOutcome = "denied"
)

func (o AuditOutcome) Valid() bool {
	return o == AuditSucceeded || o == AuditFailed || o == AuditDenied
}

type AuditAuthMethod string

const (
	AuditByPassword AuditAuthMethod = "password"
	AuditBySSO      AuditAuthMethod = "sso"
	AuditByToken    AuditAuthMethod = "api_token"
	AuditByAgent    AuditAuthMethod = "agent"
	AuditBySystem   AuditAuthMethod = "system"
)

type AuditActor struct {
	AccountID  uuid.UUID
	Kind       ActorKind
	Name       string
	TokenID    *uuid.UUID
	TokenName  string
	AgentID    *uuid.UUID
	AgentName  string
	AuthMethod SessionAuthMethod
}

func (a AuditActor) How() AuditAuthMethod {
	switch a.Kind {
	case ActorKindAgent:
		return AuditByAgent
	case ActorKindToken:
		return AuditByToken
	case ActorKindSystem:
		return AuditBySystem
	default:
		if a.AuthMethod == SessionAuthMethodSSO {
			return AuditBySSO
		}

		return AuditByPassword
	}
}

func ActorAudited(actor Actor, name string) AuditActor {
	audited := AuditActor{
		AccountID:  actor.AccountID,
		Kind:       actor.Kind,
		Name:       name,
		TokenID:    actor.TokenID,
		TokenName:  actor.TokenName,
		AgentID:    actor.AgentID,
		AuthMethod: actor.AuthMethod,
	}

	if audited.Kind == "" {
		audited.Kind = ActorKindSystem
	}

	if actor.Kind == ActorKindAgent {
		audited.AgentName = name
	}

	if actor.ConnectionID != nil && audited.TokenID == nil {
		audited.TokenID = actor.ConnectionID
		audited.TokenName = actor.ConnectionName
	}

	return audited
}

type AuditEntry struct {
	ID            uuid.UUID
	OccurredAt    time.Time
	WorkspaceID   uuid.UUID
	WorkspaceName string
	Action        AuditAction
	Outcome       AuditOutcome
	Actor         AuditActor
	SourceIP      netip.Addr
	UserAgent     string
	ResourceKind  string
	ResourceID    uuid.UUID
	ResourceName  string
	Detail        map[string]string
}

type AuditFilter struct {
	WorkspaceID    uuid.UUID
	InstanceWide   bool
	ActorAccountID uuid.UUID
	Actions        []AuditAction
	ResourceKind   string
	ResourceID     uuid.UUID
	From           *time.Time
	To             *time.Time
	Cursor         *AuditCursor
	Limit          int
}

func (f AuditFilter) Normalized() AuditFilter {
	if f.Limit <= 0 {
		f.Limit = AuditPageDefaultSize
	}

	if f.Limit > AuditPageMaxSize {
		f.Limit = AuditPageMaxSize
	}

	return f
}

func (f AuditFilter) Lookahead() AuditFilter {
	f.Limit++

	return f
}

type AuditCursor struct {
	OccurredAt time.Time
	ID         uuid.UUID
}

func (c AuditCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.ID.String() + c.OccurredAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeAuditCursor(raw string) (AuditCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < ActivityCursorIDLen {
		return AuditCursor{}, ErrAuditCursorInvalid
	}

	id, err := uuid.Parse(string(decoded[:ActivityCursorIDLen]))
	if err != nil {
		return AuditCursor{}, ErrAuditCursorInvalid
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, string(decoded[ActivityCursorIDLen:]))
	if err != nil {
		return AuditCursor{}, ErrAuditCursorInvalid
	}

	return AuditCursor{OccurredAt: occurredAt, ID: id}, nil
}

func (e AuditEntry) Cursor() AuditCursor {
	return AuditCursor{OccurredAt: e.OccurredAt, ID: e.ID}
}
