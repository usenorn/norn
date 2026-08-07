package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=scm.go -destination=scm/mock_scm.go -package=scm -mock_names=SCMConnection=MockSCMConnection,SCMDelivery=MockSCMDelivery,CodeLink=MockCodeLink,IssueMirror=MockIssueMirror,SCMTeamSetting=MockSCMTeamSetting

type SCMCredentials struct {
	Token         string
	WebhookSecret string
}

type SCMConnectionInput struct {
	Connection    entity.SCMConnection
	Token         string
	WebhookSecret string
}

type SCMConnection interface {
	Create(ctx context.Context, input SCMConnectionInput) (entity.SCMConnection, error)
	GetByID(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	GetForDelivery(ctx context.Context, connectionID uuid.UUID) (entity.SCMConnection, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMConnection, error)
	Credentials(ctx context.Context, connectionID uuid.UUID) (SCMCredentials, error)
	ReplaceToken(ctx context.Context, connectionID uuid.UUID, token, hint, login string, at time.Time) error
	UpdateSettings(ctx context.Context, connectionID, teamID uuid.UUID, mirrorLabel string) (entity.SCMConnection, error)
	RecordHook(ctx context.Context, connectionID uuid.UUID, externalHookID string) error
	MarkVerified(ctx context.Context, connectionID uuid.UUID, repository entity.SCMRepository, login string, at time.Time) error
	MarkBroken(ctx context.Context, connectionID uuid.UUID, reason entity.SCMBrokenReason, detail string, at time.Time) error
	Park(ctx context.Context, connectionID uuid.UUID, until time.Time) error
	RecordReconciled(ctx context.Context, connectionID uuid.UUID, cursor string, at time.Time) error
	RecordSeen(ctx context.Context, connectionID uuid.UUID, at time.Time) error
	ClaimDue(ctx context.Context, at time.Time, limit int) ([]entity.SCMConnection, error)
	Delete(ctx context.Context, connectionID uuid.UUID) error
}

type SCMDelivery interface {
	Record(ctx context.Context, delivery entity.SCMDelivery) (uuid.UUID, error)
	GetByID(ctx context.Context, deliveryID uuid.UUID) (entity.SCMDelivery, error)
	Settle(ctx context.Context, deliveryID uuid.UUID, outcome entity.SCMDeliveryOutcome, detail string, at time.Time) error
	Reschedule(ctx context.Context, deliveryID uuid.UUID, attempt int, retryAfter time.Time, failure string) error
	ListPending(ctx context.Context, connectionID uuid.UUID, at time.Time, limit int) ([]entity.SCMDelivery, error)
	ListByConnection(ctx context.Context, connectionID uuid.UUID, limit int) ([]entity.SCMDelivery, error)
	DeleteSettledBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

type CodeLink interface {
	Upsert(ctx context.Context, link entity.CodeLink) (entity.CodeLink, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CodeLink, error)
	ListByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repository, externalID string) ([]entity.CodeLink, error)
	MarkAdvanced(ctx context.Context, linkID uuid.UUID) error
	Delete(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) (entity.CodeLink, error)
	DetachConnection(ctx context.Context, connectionID uuid.UUID) error
}

type IssueMirror interface {
	Create(ctx context.Context, mirror entity.IssueMirror) (entity.IssueMirror, error)
	GetByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueMirror, error)
	GetByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repository, externalID string) (entity.IssueMirror, error)
	ListByConnection(ctx context.Context, connectionID uuid.UUID, limit int) ([]entity.IssueMirror, error)
	RecordPull(ctx context.Context, mirrorID uuid.UUID, hashes entity.MirrorHashes, sourceUpdatedAt time.Time, version int, at time.Time) error
	RecordPush(ctx context.Context, mirrorID uuid.UUID, hashes entity.MirrorHashes, version int, at time.Time) error
	Delete(ctx context.Context, workspaceID, issueID uuid.UUID) error
	DetachConnection(ctx context.Context, connectionID uuid.UUID) error

	CreateComment(ctx context.Context, mirror entity.CommentMirror) (entity.CommentMirror, error)
	GetCommentByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repository, externalID string) (entity.CommentMirror, error)
	ListCommentsByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CommentMirror, error)
	DetachConnectionComments(ctx context.Context, connectionID uuid.UUID) error
}

type SCMTeamSetting interface {
	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.SCMTeamSettings, error)
	Upsert(ctx context.Context, settings entity.SCMTeamSettings) (entity.SCMTeamSettings, error)
	Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error
}
