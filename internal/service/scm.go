package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=scm.go -destination=scm/mock_scm.go -package=scm -mock_names=Forge=MockForge,Forges=MockForges,SourceControl=MockSourceControl,SourceControlSync=MockSourceControlSync

type ForgeReviewer struct {
	Login      string
	Verdict    entity.ReviewVerdict
	URL        string
	ReviewedAt *time.Time
}

type ForgeChange struct {
	ExternalID string
	Number     int
	Title      string
	Body       string
	URL        string
	// State is what the change itself says. What its reviewers said arrives separately, and
	// entity.ResolveChangeState is the single place the two are combined.
	State  entity.CodeChangeState
	Checks entity.CodeChecks
	// KnowsChecks and ReviewsMoved say what this event is evidence of. A payload carries one
	// review, not the set, so claiming to know the reviews from it would wipe every other
	// answer; ReviewsMoved asks for the set to be read instead.
	KnowsChecks  bool
	ReviewsMoved bool
	Author       string
	HeadBranch   string
	BaseBranch   string
	UpdatedAt    time.Time
	MergedAt     *time.Time
	ClosedAt     *time.Time
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
	Assignees  []string
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
	Title    *string
	Body     *string
	Closed   *bool
	Assignee *string
	Labels   []string
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

	Repository(ctx context.Context, target entity.SCMTarget) (entity.SCMRemoteRepository, error)
	Identity(ctx context.Context, target entity.SCMTarget) (string, error)
	InstallHook(ctx context.Context, request ForgeHookRequest) (string, error)
	// RepairHook brings an installed hook up to the events this version needs. A hook
	// installed by an earlier version keeps the list it was created with, so a new event
	// simply never arrives and the feature looks dead on every repository already connected.
	RepairHook(ctx context.Context, request ForgeHookRequest, hookID string) (bool, error)
	RemoveHook(ctx context.Context, target entity.SCMTarget, hookID string) error

	Changes(ctx context.Context, target entity.SCMTarget, since time.Time, cursor string) (ForgeChangePage, error)
	// ChangedPaths is what routing needs and no event carries: both forges describe a change
	// without saying which files it touches.
	ChangedPaths(ctx context.Context, target entity.SCMTarget, number int) ([]string, error)
	// Reviews reads every answer a change currently has. A review event carries one review,
	// so a second approval would erase the first if the set were rebuilt from events.
	Reviews(ctx context.Context, target entity.SCMTarget, number int) ([]ForgeReviewer, error)
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
	Provider    entity.SCMProvider
	BaseURL     string
	Label       string
	Token       string
}

type UpdateConnectionInput struct {
	Label string
}

type AddRepositoryInput struct {
	ConnectionID uuid.UUID
	FullName     string
	MirrorLabel  string
	PollInterval time.Duration
}

// ConnectedRepository carries the webhook address and secret back to whoever added the
// repository. Norn installs the hook itself when the token may, and this is what a person
// needs when it may not.
type ConnectedRepository struct {
	Repository    entity.SCMRepository
	WebhookURL    string
	WebhookSecret string
	HookInstalled bool
}

type UpdateRepositoryInput struct {
	MirrorLabel   string
	SyncDirection entity.MirrorDirection
	PollInterval  time.Duration
}

type MapSCMIdentityInput struct {
	AccountID uuid.UUID
	Provider  entity.SCMProvider
	Login     string
}

type AddRouteInput struct {
	RepositoryID uuid.UUID
	TeamID       uuid.UUID
	PathPrefix   string
}

type SetTeamSCMSettingsInput struct {
	BranchTemplate string
}

type SetTransitionRuleInput struct {
	Trigger entity.CodeChangeState
	StateID uuid.UUID
}

type TeamTransitionRule struct {
	Rule      entity.SCMTransitionRule
	StateName string
}

type LinkIssueCodeInput struct {
	URL string
}

type MirrorIssueInput struct {
	RepositoryID uuid.UUID
	Reference    string
}

type SourceControl interface {
	ListConnections(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMConnection, error)
	GetConnection(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	Connect(ctx context.Context, input ConnectSourceControlInput) (entity.SCMConnection, error)
	UpdateConnection(ctx context.Context, workspaceID, connectionID uuid.UUID, input UpdateConnectionInput) (entity.SCMConnection, error)
	ReplaceToken(ctx context.Context, workspaceID, connectionID uuid.UUID, token string) (entity.SCMConnection, error)
	VerifyConnection(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	Disconnect(ctx context.Context, workspaceID, connectionID uuid.UUID) error

	ListRepositories(ctx context.Context, workspaceID, connectionID uuid.UUID) ([]entity.SCMRepository, error)
	AddRepository(ctx context.Context, workspaceID uuid.UUID, input AddRepositoryInput) (ConnectedRepository, error)
	UpdateRepository(ctx context.Context, workspaceID, repositoryID uuid.UUID, input UpdateRepositoryInput) (entity.SCMRepository, error)
	RemoveRepository(ctx context.Context, workspaceID, repositoryID uuid.UUID) error
	Deliveries(ctx context.Context, workspaceID, repositoryID uuid.UUID) ([]entity.SCMDelivery, error)

	ListRoutes(ctx context.Context, workspaceID, repositoryID uuid.UUID) (entity.SCMRoutes, error)
	AddRoute(ctx context.Context, workspaceID uuid.UUID, input AddRouteInput) (entity.SCMRoute, error)
	RemoveRoute(ctx context.Context, workspaceID, routeID uuid.UUID) error

	Identities(ctx context.Context, workspaceID uuid.UUID) (entity.SCMIdentities, error)
	MapIdentity(ctx context.Context, workspaceID uuid.UUID, input MapSCMIdentityInput) (entity.SCMIdentity, error)
	UnmapIdentity(ctx context.Context, workspaceID, identityID uuid.UUID) error
	Conflicts(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.MirrorConflict, error)

	TeamSettings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.SCMTeamSettings, error)
	SetTeamSettings(ctx context.Context, workspaceID, teamID uuid.UUID, input SetTeamSCMSettingsInput) (entity.SCMTeamSettings, error)
	BranchName(ctx context.Context, workspaceID, issueID uuid.UUID) (string, error)
	SuppressAutomation(ctx context.Context, workspaceID, issueID uuid.UUID, suppressed bool) error

	TeamRules(ctx context.Context, workspaceID, teamID uuid.UUID) ([]TeamTransitionRule, error)
	SetTeamRule(ctx context.Context, workspaceID, teamID uuid.UUID, input SetTransitionRuleInput) ([]TeamTransitionRule, error)
	ClearTeamRule(ctx context.Context, workspaceID, teamID uuid.UUID, trigger entity.CodeChangeState) ([]TeamTransitionRule, error)

	Links(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CodeLink, map[uuid.UUID]entity.CodeReviewers, error)
	Link(ctx context.Context, workspaceID, issueID uuid.UUID, input LinkIssueCodeInput) (entity.CodeLink, error)
	Unlink(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) error

	Mirror(ctx context.Context, workspaceID, issueID uuid.UUID, input MirrorIssueInput) (entity.IssueMirror, error)
	Mirrors(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.IssueMirror, error)
	Unmirror(ctx context.Context, workspaceID, issueID, mirrorID uuid.UUID) error
}

// SourceControlSync is the job-facing half. It is separate because everything on it runs
// with no person waiting and rebuilds its own actor from the connection, which is a
// different contract from the dashboard operations above.
type SourceControlSync interface {
	Accept(ctx context.Context, repositoryID uuid.UUID, provider entity.SCMProvider, header http.Header, body []byte) (uuid.UUID, error)
	Apply(ctx context.Context, deliveryID uuid.UUID) error
	Reconcile(ctx context.Context, at time.Time) error
}
