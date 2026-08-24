package scm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/agenthold"
)

type sync struct {
	connections  repository.SCMConnection
	repositories repository.SCMRepository
	routes       repository.SCMRoute
	rules        repository.SCMTransitionRule
	teamSettings repository.SCMTeamSetting
	identities   repository.SCMIdentity
	conflicts    repository.MirrorConflict
	labels       repository.Label
	agents       repository.Agent
	holds        *agenthold.Gate
	releases     repository.SCMRelease
	deployments  repository.SCMDeployment
	deliveries   repository.SCMDelivery
	links        repository.CodeLink
	mirrors      repository.IssueMirror
	states       repository.WorkflowState
	issues       repository.Issue
	workspaces   repository.Workspace
	activity     repository.Activity
	memberships  repository.Membership
	forges       service.Forges
	credentials  *credentials
	apps         repository.SCMApp
	authorizer   service.Authorizer
	issueWriter  service.Issues
	comments     service.IssueComments
	jobs         repository.JobProducer
	transactor   repository.Transactor
	cfg          config.SourceControl
	baseURL      string
}

func NewSync(
	connections repository.SCMConnection,
	repositories repository.SCMRepository,
	routes repository.SCMRoute,
	rules repository.SCMTransitionRule,
	teamSettings repository.SCMTeamSetting,
	identities repository.SCMIdentity,
	conflicts repository.MirrorConflict,
	labels repository.Label,
	agents repository.Agent,
	holds *agenthold.Gate,
	releases repository.SCMRelease,
	deployments repository.SCMDeployment,
	deliveries repository.SCMDelivery,
	links repository.CodeLink,
	mirrors repository.IssueMirror,
	states repository.WorkflowState,
	issues repository.Issue,
	workspaces repository.Workspace,
	activity repository.Activity,
	memberships repository.Membership,
	apps repository.SCMApp,
	forges service.Forges,
	cache *forge.Credentials,
	authorizer service.Authorizer,
	issueWriter service.Issues,
	comments service.IssueComments,
	jobs repository.JobProducer,
	transactor repository.Transactor,
	cfg config.SourceControl,
	app config.App,
) service.SourceControlSync {
	return &sync{
		credentials:  newCredentials(connections, apps, forges, cache),
		apps:         apps,
		connections:  connections,
		repositories: repositories,
		routes:       routes,
		rules:        rules,
		teamSettings: teamSettings,
		identities:   identities,
		conflicts:    conflicts,
		labels:       labels,
		agents:       agents,
		holds:        holds,
		releases:     releases,
		deployments:  deployments,
		deliveries:   deliveries,
		links:        links,
		mirrors:      mirrors,
		states:       states,
		issues:       issues,
		workspaces:   workspaces,
		activity:     activity,
		memberships:  memberships,
		forges:       forges,
		authorizer:   authorizer,
		issueWriter:  issueWriter,
		comments:     comments,
		jobs:         jobs,
		transactor:   transactor,
		cfg:          cfg,
		baseURL:      strings.TrimRight(app.BaseURL, "/"),
	}
}

type deliveryTally struct {
	references int
	resolved   int
	linked     int
	advanced   int
}

func (t *deliveryTally) describe() string {
	switch {
	case t.references == 0:
		return "nothing in this delivery named an issue"
	case t.resolved == 0:
		return fmt.Sprintf(
			"named %d issue reference(s), none of which resolved to an issue this connection reaches",
			t.references,
		)
	default:
		return fmt.Sprintf("linked %d, advanced %d", t.linked, t.advanced)
	}
}

func (s *sync) Accept(
	ctx context.Context,
	repositoryID uuid.UUID,
	provider entity.SCMProvider,
	header http.Header,
	body []byte,
) (uuid.UUID, error) {
	stored, err := s.repositories.GetForDelivery(ctx, repositoryID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s", entity.ErrSCMRepositoryNotFound, repositoryID)
	}

	if stored.Provider != provider {
		return uuid.Nil, fmt.Errorf(
			"%w: %s is connected to %s, not %s",
			entity.ErrSCMRepositoryNotFound, repositoryID, stored.Provider, provider,
		)
	}

	forge, err := s.forges.Lookup(stored.Provider)
	if err != nil {
		return uuid.Nil, entity.ErrSCMAppNotFound
	}

	secret, err := s.repositories.WebhookSecret(ctx, repositoryID)
	if err != nil {
		return uuid.Nil, err
	}

	delivery, err := forge.Verify(secret, header, body)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	return s.hold(ctx, stored, delivery)
}

func (s *sync) AcceptFromApp(
	ctx context.Context,
	provider entity.SCMProvider,
	header http.Header,
	body []byte,
) (uuid.UUID, error) {
	forgeApp, err := s.forges.App(provider)
	if err != nil {
		return uuid.Nil, entity.ErrSCMAppNotFound
	}

	route, err := forgeApp.Route(body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", entity.ErrSCMDeliveryUnroutable, err)
	}

	forge, err := s.forges.Lookup(provider)
	if err != nil {
		return uuid.Nil, entity.ErrSCMAppNotFound
	}

	registered, err := s.apps.Get(ctx, provider, forge.Endpoint())
	if err != nil {
		return uuid.Nil, entity.ErrSCMAppNotFound
	}

	secrets, err := s.apps.Secrets(ctx, registered.ID)
	if err != nil {
		return uuid.Nil, err
	}

	// Only a failure here is a signature failure. Everything below is Norn not knowing what the
	// delivery is about, which is a different thing and must not be reported as forgery.
	delivery, err := forge.Verify(secrets.WebhookSecret, header, body)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	connection, err := s.connections.GetByInstallation(ctx, registered.ID, route.InstallationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf(
			"%w: installation %s", entity.ErrSCMConnectionNotFound, route.InstallationID,
		)
	}

	stored, err := s.repositories.GetByFullName(ctx, connection.ID, route.FullName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s", entity.ErrSCMRepositoryNotFound, route.FullName)
	}

	return s.hold(ctx, stored, delivery)
}

func (s *sync) hold(
	ctx context.Context,
	stored entity.SCMRepository,
	delivery entity.SCMDelivery,
) (uuid.UUID, error) {
	delivery.RepositoryID = stored.ID
	delivery.WorkspaceID = stored.WorkspaceID

	if delivery.ExternalID == "" {
		delivery.ExternalID = uuid.NewString()
	}

	deliveryID, err := s.deliveries.Record(ctx, delivery)
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.repositories.RecordSeen(ctx, stored.ID, time.Now().UTC()); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a source control delivery arrival failed",
			"repository_id", stored.ID.String(),
			"error", err.Error(),
		)
	}

	if err := s.jobs.EnqueueSCMDelivery(ctx, entity.SCMDeliveryPayload{
		DeliveryID: deliveryID,
		Attempt:    0,
	}); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"queueing a source control delivery failed; the reconcile sweep will pick it up",
			"delivery_id", deliveryID.String(),
			"error", err.Error(),
		)
	}

	return deliveryID, nil
}

func (s *sync) Apply(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := s.deliveries.GetByID(ctx, deliveryID)
	if err != nil {
		return err
	}

	if delivery.ProcessedAt != nil {
		return nil
	}

	from, err := s.sourceFor(ctx, delivery.RepositoryID)
	if err != nil {
		return err
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	events, err := forge.Translate(delivery)
	if err != nil {
		return s.settle(ctx, deliveryID, entity.SCMDeliveryFailed, err.Error())
	}

	if len(events) == 0 {
		return s.settle(
			ctx,
			deliveryID,
			entity.SCMDeliveryIgnored,
			"this instance does not act on "+delivery.Event,
		)
	}

	decision, err := s.decide(ctx, from)
	if err != nil {
		if errors.Is(err, entity.ErrAccountForbidden) || errors.Is(err, entity.ErrMembershipNotFound) {
			s.markBroken(ctx, from, entity.SCMBrokenCredentialsRejected,
				"the account that established this connection no longer has access to the workspace")

			return s.settle(ctx, deliveryID, entity.SCMDeliveryFailed, err.Error())
		}

		return err
	}

	applied := &deliveryTally{}

	for _, event := range events {
		if err := s.applyOne(ctx, from, decision, event, applied); err != nil {
			var limited entity.SCMRateLimitedError
			if errors.As(err, &limited) {
				return s.park(ctx, from, delivery, limited.RetryAfter)
			}

			return s.settle(ctx, deliveryID, entity.SCMDeliveryFailed, err.Error())
		}
	}

	return s.settle(ctx, deliveryID, entity.SCMDeliveryApplied, applied.describe())
}

func (s *sync) settle(
	ctx context.Context,
	deliveryID uuid.UUID,
	outcome entity.SCMDeliveryOutcome,
	detail string,
) error {
	return s.deliveries.Settle(ctx, deliveryID, outcome, detail, time.Now().UTC())
}

func (s *sync) decide(
	ctx context.Context,
	from source,
) (entity.Decision, error) {
	membership, err := s.memberships.Get(ctx, from.workspaceID(), from.connection.OwnerAccountID)
	if err != nil {
		return entity.Decision{}, err
	}

	if membership.Deactivated() {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	scoped := identity.WithActor(ctx, from.connection.Actor())

	return s.authorizer.Decide(scoped, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: from.workspaceID(),
		Scoped:      true,
	})
}

func (s *sync) park(
	ctx context.Context,
	from source,
	delivery entity.SCMDelivery,
	wait time.Duration,
) error {
	wait = entity.ClampImportBackoff(wait, s.cfg.MinBackoff, s.cfg.MaxBackoff)
	until := time.Now().UTC().Add(wait)

	if err := s.repositories.Park(ctx, from.repository.ID, until); err != nil {
		return err
	}

	if delivery.Attempt+1 >= s.cfg.MaxAttempts {
		return s.settle(
			ctx,
			delivery.ID,
			entity.SCMDeliveryFailed,
			"the forge kept rate limiting this delivery",
		)
	}

	return s.deliveries.Reschedule(
		ctx,
		delivery.ID,
		delivery.Attempt+1,
		until,
		"waiting out a rate limit",
	)
}

func (s *sync) markBroken(
	ctx context.Context,
	from source,
	reason entity.SCMBrokenReason,
	detail string,
) {
	// The connection is what breaks, and it is what has to be marked. Handing this the
	// repository's id matches no row, so the failure is recorded nowhere and the connection goes
	// on claiming it works.
	if err := s.connections.MarkBroken(
		ctx,
		from.connection.WorkspaceID,
		from.connection.ID,
		reason,
		detail,
		time.Now().UTC(),
	); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"recording a broken source control connection failed",
			"connection_id", from.connection.ID.String(),
			"repository_id", from.repository.ID.String(),
			"error", err.Error(),
		)
	}
}

func (s *sync) applyOne(
	ctx context.Context,
	from source,
	decision entity.Decision,
	event service.ForgeEvent,
	tally *deliveryTally,
) error {
	routed, err := s.teamsFor(ctx, from, nil)
	if err != nil {
		return err
	}

	switch event.Kind {
	case service.ForgeEventBranchPushed:
		return s.linkReferences(ctx, from, decision, tally, entity.ScanIssueReferences(event.Branch.Name), routed, entity.CodeLink{
			Kind:       entity.CodeLinkBranch,
			ExternalID: event.Branch.Name,
			Title:      event.Branch.Name,
			URL:        event.Branch.URL,
			State:      entity.CodeChangeOpen,
			Author:     event.Author,
			DetectedIn: "the branch name",
		})

	case service.ForgeEventCommitPushed:
		return s.linkReferences(ctx, from, decision, tally, entity.ScanIssueReferences(event.Commit.Message), routed, entity.CodeLink{
			Kind:       entity.CodeLinkCommit,
			ExternalID: event.Commit.SHA,
			Title:      firstLine(event.Commit.Message),
			URL:        event.Commit.URL,
			State:      entity.CodeChangeMerged,
			Author:     event.Commit.Author,
			DetectedIn: "the commit message",
			MergedAt:   stamp(event.Commit.At),
		})

	case service.ForgeEventChangeChanged:
		return s.applyChange(ctx, from, decision, tally, event.Change)

	case service.ForgeEventIssueChanged:
		return s.mirrorIssue(ctx, from, decision, event.Issue)

	case service.ForgeEventCommented:
		return s.mirrorComment(ctx, from, decision, event.Issue, event.Comment)

	default:
		return nil
	}
}

func (s *sync) linkReferences(
	ctx context.Context,
	from source,
	decision entity.Decision,
	tally *deliveryTally,
	references []entity.ScannedReference,
	routed entity.SCMRouting,
	template entity.CodeLink,
) error {
	tally.references += len(references)

	for _, scanned := range references {
		issue, err := s.issues.GetVisibleByReference(
			ctx,
			from.workspaceID(),
			scanned.Reference,
			decision.Scope,
		)
		if err != nil {
			continue
		}

		if !routed.Covers(issue.TeamID) {
			continue
		}

		link := template
		link.WorkspaceID = from.workspaceID()
		link.IssueID = issue.ID
		link.RepositoryID = from.repository.ID
		link.Provider = from.connection.Provider
		link.RepositoryName = from.repository.FullName
		link.Resolving = scanned.Resolving

		tally.resolved++

		stored, err := s.links.Upsert(ctx, link)
		if err != nil {
			return err
		}

		tally.linked++

		if stored.CreatedAt.Equal(stored.UpdatedAt) {
			s.recordLinkActivity(ctx, issue, stored, decision, entity.ActivityKindCodeLinked)
		}
	}

	return nil
}

func (s *sync) applyChange(
	ctx context.Context,
	from source,
	decision entity.Decision,
	tally *deliveryTally,
	change service.ForgeChange,
) error {
	if change.State == "" && change.KnowsChecks {
		return s.recordChecks(ctx, from, tally, change)
	}

	references := entity.ScanChangeReferences(change.HeadBranch, change.Title, change.Body)

	paths := s.changedPaths(ctx, from, change)

	reviewers, reviewed := s.reviewersOf(ctx, from, change)

	state := change.State
	if reviewed {
		state = entity.ResolveChangeState(change.State, reviewers)
	}

	routed, err := s.teamsFor(ctx, from, paths)
	if err != nil {
		return err
	}

	if err := s.linkReferences(ctx, from, decision, tally, references, routed, entity.CodeLink{
		Kind:            entity.CodeLinkChange,
		ExternalID:      change.ExternalID,
		Number:          change.Number,
		Title:           change.Title,
		URL:             change.URL,
		State:           state,
		Checks:          change.Checks,
		MergeCommitSHA:  change.MergeCommitSHA,
		Author:          change.Author,
		HeadBranch:      change.HeadBranch,
		HeadSHA:         change.HeadSHA,
		BaseBranch:      change.BaseBranch,
		Paths:           paths,
		DetectedIn:      "the change",
		SourceUpdatedAt: stamp(change.UpdatedAt),
		MergedAt:        change.MergedAt,
		ClosedAt:        change.ClosedAt,
	}); err != nil {
		return err
	}

	links, err := s.links.ListByExternalID(
		ctx,
		from.workspaceID(),
		from.connection.Provider,
		from.repository.FullName,
		change.ExternalID,
	)
	if err != nil {
		return err
	}

	for _, link := range links {
		if reviewed {
			if err := s.links.ReplaceReviewers(ctx, link.ID, reviewers); err != nil {
				return err
			}
		}

		issue, err := s.issues.GetVisible(ctx, from.workspaceID(), link.IssueID, decision.Scope)
		if err != nil {
			continue
		}

		s.announce(ctx, from, link, issue)

		if err := s.advance(ctx, from, decision, tally, link, issue); err != nil {
			return err
		}
	}

	return nil
}

func (s *sync) recordChecks(
	ctx context.Context,
	from source,
	tally *deliveryTally,
	change service.ForgeChange,
) error {
	touched, err := s.links.SetChecks(
		ctx,
		from.workspaceID(),
		from.connection.Provider,
		from.repository.FullName,
		change.ExternalID,
		change.Checks,
	)
	if err != nil {
		return err
	}

	tally.linked += touched

	return nil
}

func (s *sync) reviewersOf(
	ctx context.Context,
	from source,
	change service.ForgeChange,
) (entity.CodeReviewers, bool) {
	if !change.ReviewsMoved || change.Number <= 0 {
		return nil, false
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return nil, false
	}

	found, err := forge.Reviews(ctx, from.target(), change.Number)
	if err != nil {
		logWarn(ctx, "reading who is reviewing a change failed", from.repository.ID, err)

		return nil, false
	}

	reviewers := make(entity.CodeReviewers, 0, len(found))
	for _, one := range found {
		reviewers = append(reviewers, entity.CodeReviewer{
			Login:      one.Login,
			Verdict:    one.Verdict,
			URL:        one.URL,
			ReviewedAt: one.ReviewedAt,
		})
	}

	return reviewers, true
}

func (s *sync) changedPaths(
	ctx context.Context,
	from source,
	change service.ForgeChange,
) []string {
	if change.Number <= 0 {
		return nil
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return nil
	}

	paths, err := forge.ChangedPaths(ctx, from.target(), change.Number)
	if err != nil {
		logWarn(ctx, "reading the files a change touches failed", from.repository.ID, err)

		return nil
	}

	return paths
}

func firstLine(message string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(message), "\n")

	return line
}

func stamp(at time.Time) *time.Time {
	if at.IsZero() {
		return nil
	}

	return &at
}

func (s *sync) recordLinkActivity(
	ctx context.Context,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
	kind entity.ActivityKind,
) {
	if err := s.activity.Record(ctx, entity.Activity{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        kind,
		Field:       string(link.Kind),
		ToValue:     linkLabel(link),
		Version:     issue.Version,
	}); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a code link on the issue feed failed",
			"link_id", link.ID.String(),
			"error", err.Error(),
		)
	}
}
