package scm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type sync struct {
	connections  repository.SCMConnection
	repositories repository.SCMRepository
	routes       repository.SCMRoute
	rules        repository.SCMTransitionRule
	deliveries   repository.SCMDelivery
	links        repository.CodeLink
	mirrors      repository.IssueMirror
	states       repository.WorkflowState
	issues       repository.Issue
	activity     repository.Activity
	memberships  repository.Membership
	forges       service.Forges
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
	deliveries repository.SCMDelivery,
	links repository.CodeLink,
	mirrors repository.IssueMirror,
	states repository.WorkflowState,
	issues repository.Issue,
	activity repository.Activity,
	memberships repository.Membership,
	forges service.Forges,
	authorizer service.Authorizer,
	issueWriter service.Issues,
	comments service.IssueComments,
	jobs repository.JobProducer,
	transactor repository.Transactor,
	cfg config.SourceControl,
	app config.App,
) service.SourceControlSync {
	return &sync{
		connections:  connections,
		repositories: repositories,
		routes:       routes,
		rules:        rules,
		deliveries:   deliveries,
		links:        links,
		mirrors:      mirrors,
		states:       states,
		issues:       issues,
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

// deliveryTally is what turns "applied" into an answer. A delivery that named no issue, one
// that named an issue nobody here can reach, and one that linked two changes are three
// different situations, and a log that calls all of them "processed" answers nothing.
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

// Accept answers a forge. It verifies against the repository's own secret, stores the
// delivery and hands the work to a worker, because a forge gives its endpoint a few seconds
// and a delivery that arrives while an issue is being written must not hold the connection
// open. An unknown repository and a bad signature are the same refusal, so this endpoint
// cannot be used to discover which repositories exist.
func (s *sync) Accept(
	ctx context.Context,
	repositoryID uuid.UUID,
	provider entity.SCMProvider,
	header http.Header,
	body []byte,
) (uuid.UUID, error) {
	stored, err := s.repositories.GetForDelivery(ctx, repositoryID)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	if stored.Provider != provider {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	forge, err := s.forges.Lookup(stored.Provider)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	secret, err := s.repositories.WebhookSecret(ctx, repositoryID)
	if err != nil {
		return uuid.Nil, err
	}

	delivery, err := forge.Verify(secret, header, body)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	delivery.RepositoryID = repositoryID
	delivery.WorkspaceID = stored.WorkspaceID

	if delivery.ExternalID == "" {
		delivery.ExternalID = uuid.NewString()
	}

	deliveryID, err := s.deliveries.Record(ctx, delivery)
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.repositories.RecordSeen(ctx, repositoryID, time.Now().UTC()); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a source control delivery arrival failed",
			"repository_id", repositoryID.String(),
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
		// A payload this instance cannot read will not become readable on a retry, and a
		// delivery retried for ever holds a queue slot no other work can use.
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

// settle is the one place a delivery stops. Every exit records why, because the whole point
// of the log is that "processed" and "did nothing" stop looking alike.
func (s *sync) settle(
	ctx context.Context,
	deliveryID uuid.UUID,
	outcome entity.SCMDeliveryOutcome,
	detail string,
) error {
	return s.deliveries.Settle(ctx, deliveryID, outcome, detail, time.Now().UTC())
}

// decide rebuilds the person who established the connection and asks again, every time. The
// reach is theirs, so revoking their membership or narrowing their teams narrows the
// integration at the next event rather than at the next restart.
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
	if err := s.connections.MarkBroken(
		ctx,
		from.repository.ID,
		reason,
		detail,
		time.Now().UTC(),
	); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"recording a broken source control connection failed",
			"connection_id", from.repository.ID.String(),
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
	// A branch and a commit name no files here — a push payload lists them, but a branch is
	// not a change and routing it by the files of one push would be wrong the moment the next
	// push touched something else. Both fall to the repository's default route.
	routed, err := s.teamsFor(ctx, from, nil)
	if err != nil {
		return err
	}

	switch event.Kind {
	case service.ForgeEventBranchPushed:
		return s.linkReferences(ctx, from, decision, tally, event.Branch.Name, routed, entity.CodeLink{
			Kind:       entity.CodeLinkBranch,
			ExternalID: event.Branch.Name,
			Title:      event.Branch.Name,
			URL:        event.Branch.URL,
			State:      entity.CodeChangeOpen,
			Author:     event.Author,
			DetectedIn: "the branch name",
		})

	case service.ForgeEventCommitPushed:
		return s.linkReferences(ctx, from, decision, tally, event.Commit.Message, routed, entity.CodeLink{
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

// linkReferences scans free text for anything shaped like an issue reference and keeps only
// what resolves. The scanner is generous on purpose; a key that names no team, an issue that
// does not exist, and a team the change was not routed to or this actor cannot reach are all
// dropped without a word, because a wrong link is worse than a missing one.
func (s *sync) linkReferences(
	ctx context.Context,
	from source,
	decision entity.Decision,
	tally *deliveryTally,
	text string,
	routed []uuid.UUID,
	template entity.CodeLink,
) error {
	references := entity.ScanIssueReferences(text)
	tally.references += len(references)

	for _, reference := range references {
		issue, err := s.issues.GetVisibleByReference(
			ctx,
			from.workspaceID(),
			reference,
			decision.Scope,
		)
		if err != nil {
			continue
		}

		if !slices.Contains(routed, issue.TeamID) {
			continue
		}

		link := template
		link.WorkspaceID = from.workspaceID()
		link.IssueID = issue.ID
		link.RepositoryID = from.repository.ID
		link.Provider = from.connection.Provider
		link.RepositoryName = from.repository.FullName

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
	text := strings.Join([]string{change.HeadBranch, change.Title, change.Body}, "\n")

	paths := s.changedPaths(ctx, from, change)

	routed, err := s.teamsFor(ctx, from, paths)
	if err != nil {
		return err
	}

	if err := s.linkReferences(ctx, from, decision, tally, text, routed, entity.CodeLink{
		Kind:            entity.CodeLinkChange,
		ExternalID:      change.ExternalID,
		Number:          change.Number,
		Title:           change.Title,
		URL:             change.URL,
		State:           change.State,
		Action:          change.Action,
		Author:          change.Author,
		HeadBranch:      change.HeadBranch,
		BaseBranch:      change.BaseBranch,
		Paths:           paths,
		Resolving:       true,
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
		if err := s.advance(ctx, from, decision, tally, link); err != nil {
			return err
		}
	}

	return nil
}

// changedPaths is best-effort. A route that matches on path needs them, but a forge that
// refuses the call must not stop the change being linked at all — an unread file list falls
// back to the repository default rather than to nothing.
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
