package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=scm.go -destination=scm/mock_scm.go -package=scm -mock_names=Forge=MockForge,Forges=MockForges,SourceControl=MockSourceControl,SourceControlSync=MockSourceControlSync

type ForgeChange struct {
	ExternalID string
	Number     int
	Title      string
	Body       string
	URL        string
	State      entity.CodeChangeState
	Author     string
	HeadBranch string
	BaseBranch string
	UpdatedAt  time.Time
	MergedAt   *time.Time
	ClosedAt   *time.Time
}

type ForgeCommit struct {
	SHA     string
	Message string
	URL     string
	Author  string
	At      time.Time
}

type ForgeBranch struct {
	Name string
	URL  string
}

type ForgeIssue struct {
	ExternalID string
	Number     int
	Title      string
	Body       string
	URL        string
	State      string
	Author     string
	Labels     []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ForgeComment struct {
	ExternalID string
	Body       string
	Author     string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ForgeIssuePatch struct {
	Title  *string
	Body   *string
	Closed *bool
}

type ForgeChangePage struct {
	Changes []ForgeChange
	Cursor  string
}

type ForgeIssuePage struct {
	Issues []ForgeIssue
	Cursor string
}

// ForgeEvent is what one delivery means once the wire format is gone. A single delivery can
// carry several — a push names many commits — and every one of them is something the rest of
// the feature can act on without knowing which forge it came from.
type ForgeEvent struct {
	Kind    ForgeEventKind
	Change  ForgeChange
	Commit  ForgeCommit
	Branch  ForgeBranch
	Issue   ForgeIssue
	Comment ForgeComment
	Author  string
	At      time.Time
}

type ForgeEventKind string

const (
	ForgeEventBranchPushed  ForgeEventKind = "branch_pushed"
	ForgeEventCommitPushed  ForgeEventKind = "commit_pushed"
	ForgeEventChangeChanged ForgeEventKind = "change_changed"
	ForgeEventIssueChanged  ForgeEventKind = "issue_changed"
	ForgeEventCommented     ForgeEventKind = "commented"
)

type ForgeHookRequest struct {
	Target      entity.SCMTarget
	CallbackURL string
	Secret      string
}

// Forge is what a platform has to be able to do. Verify and Translate take no context and no
// credential on purpose: signature checking and payload reading are the highest-risk code
// here and this keeps them provable against recorded payloads, with no network and no token.
type Forge interface {
	Provider() entity.SCMProvider
	Endpoint() string

	Verify(secret string, header http.Header, body []byte) (entity.SCMDelivery, error)
	Translate(delivery entity.SCMDelivery) ([]ForgeEvent, error)

	Repository(ctx context.Context, target entity.SCMTarget) (entity.SCMRepository, error)
	Identity(ctx context.Context, target entity.SCMTarget) (string, error)
	InstallHook(ctx context.Context, request ForgeHookRequest) (string, error)
	RemoveHook(ctx context.Context, target entity.SCMTarget, hookID string) error

	Changes(ctx context.Context, target entity.SCMTarget, since time.Time, cursor string) (ForgeChangePage, error)
	Issues(ctx context.Context, target entity.SCMTarget, label string, since time.Time, cursor string) (ForgeIssuePage, error)
	// An issue is addressed by the number its repository counts with, not by the identity it
	// is stored under. Both forges put the number in the path and the id in the payload, so
	// passing one where the other belongs reads a different issue or none at all.
	Issue(ctx context.Context, target entity.SCMTarget, number int) (ForgeIssue, error)
	AmendIssue(ctx context.Context, target entity.SCMTarget, number int, patch ForgeIssuePatch) (ForgeIssue, error)
	Comments(ctx context.Context, target entity.SCMTarget, number int, since time.Time) ([]ForgeComment, error)
	PostComment(ctx context.Context, target entity.SCMTarget, number int, body string) (ForgeComment, error)
}

type Forges interface {
	Lookup(provider entity.SCMProvider) (Forge, error)
	Providers() []entity.SCMProvider
}

type ConnectSourceControlInput struct {
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	Provider    entity.SCMProvider
	BaseURL     string
	Repository  string
	Token       string
	MirrorLabel string
}

type ConnectedSourceControl struct {
	Connection    entity.SCMConnection
	WebhookURL    string
	WebhookSecret string
}

type UpdateSourceControlInput struct {
	TeamID      uuid.UUID
	ClearTeam   bool
	MirrorLabel string
}

type SetTeamSourceControlInput struct {
	AdvanceOnMerge bool
	MergedStateID  uuid.UUID
	ClearState     bool
}

type TeamSourceControlSettings struct {
	Settings       entity.SCMTeamSettings
	TargetResolved bool
	TargetName     string
}

type LinkIssueCodeInput struct {
	URL string
}

type MirrorIssueInput struct {
	ConnectionID uuid.UUID
	Reference    string
}

type SourceControl interface {
	List(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMConnection, error)
	Get(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	Connect(ctx context.Context, input ConnectSourceControlInput) (ConnectedSourceControl, error)
	Update(ctx context.Context, workspaceID, connectionID uuid.UUID, input UpdateSourceControlInput) (entity.SCMConnection, error)
	ReplaceToken(ctx context.Context, workspaceID, connectionID uuid.UUID, token string) (entity.SCMConnection, error)
	Verify(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	Disconnect(ctx context.Context, workspaceID, connectionID uuid.UUID) error

	TeamSettings(ctx context.Context, workspaceID, teamID uuid.UUID) (TeamSourceControlSettings, error)
	SetTeamSettings(ctx context.Context, workspaceID, teamID uuid.UUID, input SetTeamSourceControlInput) (TeamSourceControlSettings, error)

	Links(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CodeLink, error)
	Link(ctx context.Context, workspaceID, issueID uuid.UUID, input LinkIssueCodeInput) (entity.CodeLink, error)
	Unlink(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) error

	Mirror(ctx context.Context, workspaceID, issueID uuid.UUID, input MirrorIssueInput) (entity.IssueMirror, error)
	MirrorOf(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueMirror, error)
	Unmirror(ctx context.Context, workspaceID, issueID uuid.UUID) error
}

// SourceControlSync is the job-facing half. It is separate because everything on it runs
// with no person waiting and rebuilds its own actor from the connection, which is a
// different contract from the dashboard operations above.
type SourceControlSync interface {
	Accept(ctx context.Context, connectionID uuid.UUID, provider entity.SCMProvider, header http.Header, body []byte) (uuid.UUID, error)
	Apply(ctx context.Context, deliveryID uuid.UUID) error
	Reconcile(ctx context.Context, at time.Time) error
}
