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
	ExternalID     string
	Number         int
	Title          string
	Body           string
	URL            string
	State          entity.CodeChangeState
	Checks         entity.CodeChecks
	MergeCommitSHA string
	KnowsChecks    bool
	ReviewsMoved   bool
	Author         string
	HeadBranch     string
	HeadSHA        string
	BaseBranch     string
	UpdatedAt      time.Time
	MergedAt       *time.Time
	ClosedAt       *time.Time
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

type ForgeRelease struct {
	ExternalID  string
	Tag         string
	Name        string
	URL         string
	CommitSHA   string
	Prerelease  bool
	PublishedAt *time.Time
}

type ForgeDeployment struct {
	ExternalID  string
	Environment string
	State       entity.DeploymentState
	URL         string
	CommitSHA   string
	OccurredAt  *time.Time
}

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

type Forge interface {
	Provider() entity.SCMProvider
	Endpoint() string
	Capabilities() entity.SCMCapabilitySet

	Verify(secret string, header http.Header, body []byte) (entity.SCMDelivery, error)
	Translate(delivery entity.SCMDelivery) ([]ForgeEvent, error)

	Repository(ctx context.Context, target entity.SCMTarget) (entity.SCMRemoteRepository, error)
	Identity(ctx context.Context, target entity.SCMTarget) (string, error)
	InstallHook(ctx context.Context, request ForgeHookRequest) (string, error)
	RepairHook(ctx context.Context, request ForgeHookRequest, hookID string) (bool, error)
	RemoveHook(ctx context.Context, target entity.SCMTarget, hookID string) error

	Changes(ctx context.Context, target entity.SCMTarget, since time.Time, cursor string) (ForgeChangePage, error)
	ChangedPaths(ctx context.Context, target entity.SCMTarget, number int) ([]string, error)
	Reviews(ctx context.Context, target entity.SCMTarget, number int) ([]ForgeReviewer, error)
	Releases(ctx context.Context, target entity.SCMTarget, limit int) ([]ForgeRelease, error)
	ReleaseCommits(ctx context.Context, target entity.SCMTarget, from, to string) ([]string, error)
	Deployments(ctx context.Context, target entity.SCMTarget, limit int) ([]ForgeDeployment, error)
	Issues(ctx context.Context, target entity.SCMTarget, label string, since time.Time, cursor string) (ForgeIssuePage, error)
	Issue(ctx context.Context, target entity.SCMTarget, number int) (ForgeIssue, error)
	AmendIssue(ctx context.Context, target entity.SCMTarget, number int, patch ForgeIssuePatch) (ForgeIssue, error)
	Comments(ctx context.Context, target entity.SCMTarget, number int, since time.Time) ([]ForgeComment, error)
	PostComment(ctx context.Context, target entity.SCMTarget, number int, body string) (ForgeComment, error)
	PostChangeComment(ctx context.Context, target entity.SCMTarget, number int, body string) error
}

type SCMApplication struct {
	App       entity.SCMApp
	Installed bool
	Accounts  []string
}

type ForgeApp interface {
	MintInstallationToken(
		ctx context.Context,
		app entity.SCMApp,
		installationID string,
	) (entity.SCMCredential, error)

	Route(body []byte) (entity.SCMDeliveryRoute, error)

	ManifestTarget(baseURL, organization string) string
	ConvertManifest(ctx context.Context, intended entity.SCMApp, code string) (entity.SCMApp, error)

	AuthorizeURL(app entity.SCMApp, state, redirect string) string
	ExchangeCode(ctx context.Context, app entity.SCMApp, code, redirect string) (string, error)

	AppInstallations(ctx context.Context, app entity.SCMApp) ([]entity.SCMInstallation, error)
	Installations(
		ctx context.Context,
		app entity.SCMApp,
		userToken string,
	) ([]entity.SCMInstallation, error)
	InstallationRepositories(
		ctx context.Context,
		app entity.SCMApp,
		credential string,
	) ([]entity.SCMRemoteRepository, error)
}

type Forges interface {
	Lookup(provider entity.SCMProvider) (Forge, error)
	App(provider entity.SCMProvider) (ForgeApp, error)
	Providers() []entity.SCMProvider
}

type ConnectSourceControlInput struct {
	WorkspaceID         uuid.UUID
	Provider            entity.SCMProvider
	BaseURL             string
	Label               string
	Token               string
	InstallationHandle  string
	InstallationID      string
	AllowPrivateAddress bool
	CACertificate       string
}

func (i ConnectSourceControlInput) UsesApp() bool {
	return i.InstallationHandle != "" || i.InstallationID != ""
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

type ConnectedRepository struct {
	Repository    entity.SCMRepository
	WebhookURL    string
	WebhookSecret string
	HookInstalled bool
}

type UpdateRepositoryInput struct {
	MirrorLabel      string
	SyncDirection    entity.MirrorDirection
	WebhooksDisabled *bool
	PollInterval     time.Duration
}

type IssueShipping struct {
	Releases    entity.SCMReleases
	Deployments entity.SCMDeployments
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

type SCMAppChoice struct {
	Handle        string
	WorkspaceID   uuid.UUID
	WorkspaceSlug string
	Installations []entity.SCMInstallation
}

type RegisterSCMAppInput struct {
	WorkspaceID         uuid.UUID
	Organization        string
	InstanceURL         string
	InstanceName        string
	HookURL             string
	RedirectURL         string
	CallbackURL         string
	AllowPrivateAddress bool
	CACertificate       string
}

type SourceControlApps interface {
	Application(ctx context.Context, workspaceID uuid.UUID, provider entity.SCMProvider) (SCMApplication, error)
	Registration(ctx context.Context, input RegisterSCMAppInput) (entity.SCMAppRegistration, error)
	CompleteRegistration(ctx context.Context, code, state string) (entity.SCMAppState, error)
	Authorization(ctx context.Context, workspaceID uuid.UUID, callbackURL string) (string, error)
	CompleteAuthorization(ctx context.Context, code, state, callbackURL string) (SCMAppChoice, error)
	Choice(ctx context.Context, workspaceID uuid.UUID, handle string) (SCMAppChoice, error)
}

type SourceControl interface {
	ListConnections(ctx context.Context, workspaceID uuid.UUID) ([]entity.SCMConnection, error)
	GetConnection(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	Connect(ctx context.Context, input ConnectSourceControlInput) (entity.SCMConnection, error)
	UpdateConnection(ctx context.Context, workspaceID, connectionID uuid.UUID, input UpdateConnectionInput) (entity.SCMConnection, error)
	ReplaceToken(ctx context.Context, workspaceID, connectionID uuid.UUID, token string) (entity.SCMConnection, error)
	VerifyConnection(ctx context.Context, workspaceID, connectionID uuid.UUID) (entity.SCMConnection, error)
	AvailableRepositories(ctx context.Context, workspaceID, connectionID uuid.UUID) ([]entity.SCMRemoteRepository, error)
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
	Shipped(ctx context.Context, workspaceID, issueID uuid.UUID) (IssueShipping, error)

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

type SourceControlSync interface {
	Accept(ctx context.Context, repositoryID uuid.UUID, provider entity.SCMProvider, header http.Header, body []byte) (uuid.UUID, error)
	AcceptFromApp(ctx context.Context, provider entity.SCMProvider, header http.Header, body []byte) (uuid.UUID, error)
	Apply(ctx context.Context, deliveryID uuid.UUID) error
	Reconcile(ctx context.Context, at time.Time) error
	Backfill(ctx context.Context, repositoryID uuid.UUID) error
	Resume(ctx context.Context, workspaceID, issueID uuid.UUID) error
}
