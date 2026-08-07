package scm

import (
	"context"
	"errors"
	"net/http"
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
	connections repository.SCMConnection
	deliveries  repository.SCMDelivery
	links       repository.CodeLink
	mirrors     repository.IssueMirror
	settings    repository.SCMTeamSetting
	states      repository.WorkflowState
	issues      repository.Issue
	activity    repository.Activity
	memberships repository.Membership
	forges      service.Forges
	authorizer  service.Authorizer
	issueWriter service.Issues
	comments    service.IssueComments
	jobs        repository.JobProducer
	transactor  repository.Transactor
	cfg         config.SourceControl
	baseURL     string
}

func NewSync(
	connections repository.SCMConnection,
	deliveries repository.SCMDelivery,
	links repository.CodeLink,
	mirrors repository.IssueMirror,
	settings repository.SCMTeamSetting,
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
		connections: connections,
		deliveries:  deliveries,
		links:       links,
		mirrors:     mirrors,
		settings:    settings,
		states:      states,
		issues:      issues,
		activity:    activity,
		memberships: memberships,
		forges:      forges,
		authorizer:  authorizer,
		issueWriter: issueWriter,
		comments:    comments,
		jobs:        jobs,
		transactor:  transactor,
		cfg:         cfg,
		baseURL:     strings.TrimRight(app.BaseURL, "/"),
	}
}

// Accept answers a forge. It verifies against the connection's own secret, stores the
// delivery and hands the work to a worker, because a forge gives its endpoint a few seconds
// and a delivery that arrives while an issue is being written must not hold the connection
// open. An unknown connection and a bad signature are the same refusal, so this endpoint
// cannot be used to discover which connections exist.
func (s *sync) Accept(
	ctx context.Context,
	connectionID uuid.UUID,
	provider entity.SCMProvider,
	header http.Header,
	body []byte,
) (uuid.UUID, error) {
	connection, err := s.connections.GetForDelivery(ctx, connectionID)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	if connection.Provider != provider {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	credentials, err := s.connections.Credentials(ctx, connectionID)
	if err != nil {
		return uuid.Nil, err
	}

	delivery, err := forge.Verify(credentials.WebhookSecret, header, body)
	if err != nil {
		return uuid.Nil, entity.ErrSCMSignatureInvalid
	}

	delivery.ConnectionID = connectionID
	delivery.WorkspaceID = connection.WorkspaceID

	if delivery.ExternalID == "" {
		delivery.ExternalID = uuid.NewString()
	}

	deliveryID, err := s.deliveries.Record(ctx, delivery)
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.connections.RecordSeen(ctx, connectionID, time.Now().UTC()); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording a source control delivery arrival failed",
			"connection_id", connectionID.String(),
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

	connection, err := s.connections.GetForDelivery(ctx, delivery.ConnectionID)
	if err != nil {
		return err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return err
	}

	events, err := forge.Translate(delivery)
	if err != nil {
		// A payload this instance cannot read will not become readable on a retry, and a
		// delivery retried for ever holds a queue slot no other work can use.
		return s.deliveries.Settle(ctx, deliveryID, err.Error(), time.Now().UTC())
	}

	if len(events) == 0 {
		return s.deliveries.Settle(ctx, deliveryID, "", time.Now().UTC())
	}

	decision, err := s.decide(ctx, connection)
	if err != nil {
		if errors.Is(err, entity.ErrAccountForbidden) || errors.Is(err, entity.ErrMembershipNotFound) {
			s.markBroken(ctx, connection, entity.SCMBrokenCredentialsRejected,
				"the account that established this connection no longer has access to the workspace")

			return s.deliveries.Settle(ctx, deliveryID, err.Error(), time.Now().UTC())
		}

		return err
	}

	for _, event := range events {
		if err := s.applyOne(ctx, connection, decision, event); err != nil {
			var limited entity.SCMRateLimitedError
			if errors.As(err, &limited) {
				return s.park(ctx, connection, delivery, limited.RetryAfter)
			}

			return s.deliveries.Settle(ctx, deliveryID, err.Error(), time.Now().UTC())
		}
	}

	return s.deliveries.Settle(ctx, deliveryID, "", time.Now().UTC())
}

// decide rebuilds the person who established the connection and asks again, every time. The
// reach is theirs, so revoking their membership or narrowing their teams narrows the
// integration at the next event rather than at the next restart.
func (s *sync) decide(
	ctx context.Context,
	connection entity.SCMConnection,
) (entity.Decision, error) {
	membership, err := s.memberships.Get(ctx, connection.WorkspaceID, connection.OwnerAccountID)
	if err != nil {
		return entity.Decision{}, err
	}

	if membership.Deactivated() {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	scoped := identity.WithActor(ctx, connection.Actor())

	return s.authorizer.Decide(scoped, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: connection.WorkspaceID,
		Scoped:      true,
	})
}

func (s *sync) park(
	ctx context.Context,
	connection entity.SCMConnection,
	delivery entity.SCMDelivery,
	wait time.Duration,
) error {
	wait = entity.ClampImportBackoff(wait, s.cfg.MinBackoff, s.cfg.MaxBackoff)
	until := time.Now().UTC().Add(wait)

	if err := s.connections.Park(ctx, connection.ID, until); err != nil {
		return err
	}

	if delivery.Attempt+1 >= s.cfg.MaxAttempts {
		return s.deliveries.Settle(
			ctx,
			delivery.ID,
			"the forge kept rate limiting this delivery",
			time.Now().UTC(),
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
	connection entity.SCMConnection,
	reason entity.SCMBrokenReason,
	detail string,
) {
	if err := s.connections.MarkBroken(
		ctx,
		connection.ID,
		reason,
		detail,
		time.Now().UTC(),
	); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"recording a broken source control connection failed",
			"connection_id", connection.ID.String(),
			"error", err.Error(),
		)
	}
}

func (s *sync) applyOne(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	event service.ForgeEvent,
) error {
	switch event.Kind {
	case service.ForgeEventBranchPushed:
		return s.linkReferences(ctx, connection, decision, event.Branch.Name, entity.CodeLink{
			Kind:       entity.CodeLinkBranch,
			ExternalID: event.Branch.Name,
			Title:      event.Branch.Name,
			URL:        event.Branch.URL,
			State:      entity.CodeChangeOpen,
			Author:     event.Author,
			DetectedIn: "the branch name",
		})

	case service.ForgeEventCommitPushed:
		return s.linkReferences(ctx, connection, decision, event.Commit.Message, entity.CodeLink{
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
		return s.applyChange(ctx, connection, decision, event.Change)

	case service.ForgeEventIssueChanged:
		return s.mirrorIssue(ctx, connection, decision, event.Issue)

	case service.ForgeEventCommented:
		return s.mirrorComment(ctx, connection, decision, event.Issue, event.Comment)

	default:
		return nil
	}
}

// linkReferences scans free text for anything shaped like an issue reference and keeps only
// what resolves. The scanner is generous on purpose; a key that names no team, an issue that
// does not exist, and a team this connection or this actor cannot reach are all dropped
// without a word, because a wrong link is worse than a missing one.
func (s *sync) linkReferences(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	text string,
	template entity.CodeLink,
) error {
	for _, reference := range entity.ScanIssueReferences(text) {
		issue, err := s.issues.GetVisibleByReference(
			ctx,
			connection.WorkspaceID,
			reference,
			decision.Scope,
		)
		if err != nil {
			continue
		}

		if !connection.Covers(issue.TeamID) {
			continue
		}

		link := template
		link.WorkspaceID = connection.WorkspaceID
		link.IssueID = issue.ID
		link.ConnectionID = connection.ID
		link.Provider = connection.Provider
		link.Repository = connection.Repository

		stored, err := s.links.Upsert(ctx, link)
		if err != nil {
			return err
		}

		if stored.CreatedAt.Equal(stored.UpdatedAt) {
			s.recordLinkActivity(ctx, issue, stored, decision, entity.ActivityKindCodeLinked)
		}
	}

	return nil
}

func (s *sync) applyChange(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	change service.ForgeChange,
) error {
	text := strings.Join([]string{change.HeadBranch, change.Title, change.Body}, "\n")

	if err := s.linkReferences(ctx, connection, decision, text, entity.CodeLink{
		Kind:            entity.CodeLinkChange,
		ExternalID:      change.ExternalID,
		Number:          change.Number,
		Title:           change.Title,
		URL:             change.URL,
		State:           change.State,
		Author:          change.Author,
		DetectedIn:      "the change",
		SourceUpdatedAt: stamp(change.UpdatedAt),
		MergedAt:        change.MergedAt,
		ClosedAt:        change.ClosedAt,
	}); err != nil {
		return err
	}

	if !change.State.Merged() {
		return nil
	}

	links, err := s.links.ListByExternalID(
		ctx,
		connection.WorkspaceID,
		connection.Provider,
		connection.Repository,
		change.ExternalID,
	)
	if err != nil {
		return err
	}

	for _, link := range links {
		if err := s.advance(ctx, connection, decision, link); err != nil {
			return err
		}
	}

	return nil
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
