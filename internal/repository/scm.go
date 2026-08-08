package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=scm.go -destination=scm/mock_scm.go -package=scm -mock_names=SCMApp=MockSCMApp,SCMRelease=MockSCMRelease,SCMDeployment=MockSCMDeployment,SCMIdentity=MockSCMIdentity,MirrorConflict=MockMirrorConflict,SCMTeamSetting=MockSCMTeamSetting,SCMConnection=MockSCMConnection,SCMRepository=MockSCMRepository,SCMRoute=MockSCMRoute,SCMDelivery=MockSCMDelivery,CodeLink=MockCodeLink,IssueMirror=MockIssueMirror,SCMTransitionRule=MockSCMTransitionRule

type SCMConnectionInput struct {
	Connection entity.SCMConnection
	Token      string
}

type SCMConnection interface {
	Create(ctx context.Context, input SCMConnectionInput) (entity.SCMConnection, error)
	GetByID(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	GetForDelivery(ctx context.Context, connectionID uuid.UUID) (entity.SCMConnection, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMConnection, error)
	Token(ctx context.Context, connectionID uuid.UUID) (string, error)
	ReplaceToken(ctx context.Context, connectionID uuid.UUID, token, hint, login string, at time.Time) error
	UpdateLabel(ctx context.Context, connectionID uuid.UUID, label string) (entity.SCMConnection, error)
	MarkVerified(ctx context.Context, connectionID uuid.UUID, login string, capabilities entity.SCMCapabilitySet, at time.Time) error
	MarkBroken(ctx context.Context, connectionID uuid.UUID, reason entity.SCMBrokenReason, detail string, at time.Time) error
	Delete(ctx context.Context, connectionID uuid.UUID) error
}

type SCMRepositoryInput struct {
	Repository    entity.SCMRepository
	WebhookSecret string
}

type SCMRepositorySettings struct {
	MirrorLabel      string
	SyncDirection    entity.MirrorDirection
	WebhooksDisabled bool
	PollInterval     time.Duration
}

type SCMRepository interface {
	Create(ctx context.Context, input SCMRepositoryInput) (entity.SCMRepository, error)
	GetByID(ctx context.Context, workspaceID, repositoryID uuid.UUID) (entity.SCMRepository, error)
	GetForDelivery(ctx context.Context, repositoryID uuid.UUID) (entity.SCMRepository, error)
	ListByConnection(ctx context.Context, connectionID uuid.UUID) ([]entity.SCMRepository, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMRepository, error)
	WebhookSecret(ctx context.Context, repositoryID uuid.UUID) (string, error)
	UpdateSettings(ctx context.Context, repositoryID uuid.UUID, settings SCMRepositorySettings) (entity.SCMRepository, error)
	RecordHook(ctx context.Context, repositoryID uuid.UUID, externalHookID string) error
	RecordSeen(ctx context.Context, repositoryID uuid.UUID, at time.Time) error
	RecordReconciled(ctx context.Context, repositoryID uuid.UUID, cursor string, at time.Time) error
	RecordBackfilled(ctx context.Context, repositoryID uuid.UUID, at time.Time) error
	Park(ctx context.Context, repositoryID uuid.UUID, until time.Time) error
	ClaimDue(ctx context.Context, at time.Time, limit int) ([]entity.SCMRepository, error)
	Delete(ctx context.Context, workspaceID, repositoryID uuid.UUID) error
}

type SCMRoute interface {
	Create(ctx context.Context, route entity.SCMRoute) (entity.SCMRoute, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID) (entity.SCMRoutes, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) (entity.SCMRoutes, error)
	Delete(ctx context.Context, workspaceID, routeID uuid.UUID) (entity.SCMRoute, error)
}

type SCMDelivery interface {
	Record(ctx context.Context, delivery entity.SCMDelivery) (uuid.UUID, error)
	GetByID(ctx context.Context, deliveryID uuid.UUID) (entity.SCMDelivery, error)
	Settle(ctx context.Context, deliveryID uuid.UUID, outcome entity.SCMDeliveryOutcome, detail string, at time.Time) error
	Reschedule(ctx context.Context, deliveryID uuid.UUID, attempt int, retryAfter time.Time, failure string) error
	ListPending(ctx context.Context, repositoryID uuid.UUID, at time.Time, limit int) ([]entity.SCMDelivery, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID, limit int) ([]entity.SCMDelivery, error)
	DeleteSettledBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

type CodeLink interface {
	Upsert(ctx context.Context, link entity.CodeLink) (entity.CodeLink, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CodeLink, error)
	ListByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repositoryName, externalID string) ([]entity.CodeLink, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID) ([]entity.CodeLink, error)
	ClaimTransition(ctx context.Context, linkID uuid.UUID, transition entity.CodeChangeState, issueID uuid.UUID, at time.Time) (bool, error)
	Delete(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) (entity.CodeLink, error)
	DetachRepository(ctx context.Context, repositoryID uuid.UUID) error

	SetChecks(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repositoryName, externalID string, checks entity.CodeChecks) (int, error)
	ReplaceReviewers(ctx context.Context, linkID uuid.UUID, reviewers entity.CodeReviewers) error
	ListReviewers(ctx context.Context, linkIDs []uuid.UUID) (map[uuid.UUID]entity.CodeReviewers, error)
}

type SCMAppInput struct {
	App           entity.SCMApp
	PrivateKey    string
	WebhookSecret string
	ClientSecret  string
}

type SCMApp interface {
	Upsert(ctx context.Context, input SCMAppInput) (entity.SCMApp, error)
	Get(ctx context.Context, provider entity.SCMProvider, baseURL string) (entity.SCMApp, error)
	GetByID(ctx context.Context, appID uuid.UUID) (entity.SCMApp, error)
	Secrets(ctx context.Context, appID uuid.UUID) (entity.SCMApp, error)
	List(ctx context.Context) ([]entity.SCMApp, error)
}

type SCMRelease interface {
	Upsert(ctx context.Context, release entity.SCMRelease) (entity.SCMRelease, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID, limit int) (entity.SCMReleases, error)
	LinkChanges(ctx context.Context, releaseID uuid.UUID, links []entity.CodeLink) error
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.SCMReleases, error)
}

type SCMDeployment interface {
	Upsert(ctx context.Context, deployment entity.SCMDeployment) error
	ListByCommits(ctx context.Context, repositoryID uuid.UUID, commits []string) (entity.SCMDeployments, error)
}

type SCMIdentity interface {
	List(ctx context.Context, workspaceID uuid.UUID) (entity.SCMIdentities, error)
	Create(ctx context.Context, identity entity.SCMIdentity) (entity.SCMIdentity, error)
	Delete(ctx context.Context, workspaceID, identityID uuid.UUID) error
}

type MirrorConflict interface {
	Record(ctx context.Context, conflict entity.MirrorConflict) error
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID, limit int) ([]entity.MirrorConflict, error)
}

type SCMTeamSetting interface {
	Get(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.SCMTeamSettings, error)
	Upsert(ctx context.Context, settings entity.SCMTeamSettings) (entity.SCMTeamSettings, error)
}

type IssueMirror interface {
	Create(ctx context.Context, mirror entity.IssueMirror) (entity.IssueMirror, error)
	GetByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueMirror, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueMirror, error)
	GetByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repositoryName, externalID string) (entity.IssueMirror, error)
	ListByRepository(ctx context.Context, repositoryID uuid.UUID, limit int) ([]entity.IssueMirror, error)
	RecordPull(ctx context.Context, mirrorID uuid.UUID, hashes entity.MirrorHashes, sourceUpdatedAt time.Time, version int, at time.Time) error
	RecordPush(ctx context.Context, mirrorID uuid.UUID, hashes entity.MirrorHashes, version int, at time.Time) error
	Delete(ctx context.Context, workspaceID, issueID, mirrorID uuid.UUID) error
	DetachRepository(ctx context.Context, repositoryID uuid.UUID) error

	CreateComment(ctx context.Context, mirror entity.CommentMirror) (entity.CommentMirror, error)
	GetCommentByExternalID(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider, repositoryName, externalID string) (entity.CommentMirror, error)
	ListCommentsByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CommentMirror, error)
}

type SCMTransitionRule interface {
	ListByTeam(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.SCMTransitionRules, error)
	Upsert(ctx context.Context, rule entity.SCMTransitionRule) (entity.SCMTransitionRule, error)
	Delete(ctx context.Context, workspaceID, teamID uuid.UUID, trigger entity.CodeChangeState) error
}
