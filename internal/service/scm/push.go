package scm

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

// pushMirrors is the half that makes "in step" mean both directions. A delivery brings the
// platform's side in; nothing carries Norn's side out, so without this a mirrored issue
// edited here drifts away from the one it is paired with and never comes back.
//
// It reads the platform issue first rather than pushing blind, because that is the only way
// to tell "Norn changed" from "both changed" — and when both did, the same arbitration the
// inbound path uses decides it, so the two directions cannot disagree.
func (s *sync) pushMirrors(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	decision entity.Decision,
	at time.Time,
) error {
	mirrors, err := s.mirrors.ListByRepository(ctx, from.repository.ID, s.cfg.ReconcileBatch)
	if err != nil {
		return err
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	for _, mirror := range mirrors {
		if err := s.pushOne(ctx, from, target, decision, forge, mirror, at); err != nil {
			return err
		}
	}

	return nil
}

func (s *sync) pushOne(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	decision entity.Decision,
	forge service.Forge,
	mirror entity.IssueMirror,
	at time.Time,
) error {
	// Reach is checked before anything is fetched. It can be lost after a pairing was made,
	// and going quiet is right: the connection is bounded by a person's permissions and
	// those just narrowed.
	if _, err := s.issues.GetVisible(
		ctx,
		from.workspaceID(),
		mirror.IssueID,
		decision.Scope,
	); err != nil {
		return nil
	}

	found, err := forge.Issue(ctx, target, mirror.ExternalNumber)
	if err != nil {
		return err
	}

	// The inbound half first, so a platform edit is applied and recorded before anything is
	// sent back. Running the push first would overwrite an edit we had not read yet.
	if err := s.reconcileMirror(ctx, from, decision, mirror, found); err != nil {
		return err
	}

	refreshed, err := s.mirrors.GetByIssue(ctx, from.workspaceID(), mirror.IssueID)
	if err != nil {
		return err
	}

	current, err := s.issues.GetVisible(ctx, from.workspaceID(), mirror.IssueID, decision.Scope)
	if err != nil {
		return nil
	}

	patch, changed := outboundPatch(refreshed, current)

	if changed {
		if _, err := forge.AmendIssue(ctx, target, refreshed.ExternalNumber, patch); err != nil {
			return err
		}

		if err := s.mirrors.RecordPush(
			ctx,
			refreshed.ID,
			entity.HashesOf(current.Title, current.Description, found.State),
			current.Version,
			at,
		); err != nil {
			return err
		}
	}

	return s.pushComments(ctx, from, target, forge, refreshed, at)
}

// outboundPatch sends only what differs from the value both sides last agreed on. Comparing
// against the stored hash rather than a timestamp is what stops the sweep pushing the same
// unchanged title on every cycle, which would burn the rate limit and rewrite the platform
// issue's history for nothing.
func outboundPatch(
	mirror entity.IssueMirror,
	issue entity.Issue,
) (service.ForgeIssuePatch, bool) {
	var (
		patch   service.ForgeIssuePatch
		changed bool
	)

	if !mirror.Echoes(entity.IssueFieldTitle, issue.Title) {
		title := issue.Title
		patch.Title = &title
		changed = true
	}

	if !mirror.Echoes(entity.IssueFieldDescription, issue.Description) {
		body := issue.Description
		patch.Body = &body
		changed = true
	}

	return patch, changed
}

// pushComments carries what people wrote here out to the platform. Three things stop a
// comment going round for ever: one written by this connection's own account came from the
// platform in the first place, one that already holds a mirror row has been sent, and the
// row is written in the same pass as the send.
func (s *sync) pushComments(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	forge service.Forge,
	mirror entity.IssueMirror,
	at time.Time,
) error {
	scoped := identity.WithActor(ctx, from.connection.Actor())

	thread, err := s.comments.List(
		scoped,
		from.workspaceID(),
		mirror.IssueID,
		service.ListCommentsInput{Limit: s.cfg.ReconcileBatch},
	)
	if err != nil {
		return nil
	}

	mirrored, err := s.mirrors.ListCommentsByIssue(ctx, from.workspaceID(), mirror.IssueID)
	if err != nil {
		return err
	}

	sent := make(map[uuid.UUID]bool, len(mirrored))
	for _, record := range mirrored {
		sent[record.CommentID] = true
	}

	for _, comment := range thread.Comments {
		if comment.Deleted() ||
			comment.AuthorAccountID == from.connection.IntegrationAccountID ||
			sent[comment.ID] {
			continue
		}

		posted, err := forge.PostComment(ctx, target, mirror.ExternalNumber, comment.Body)
		if err != nil {
			var limited entity.SCMRateLimitedError
			if errors.As(err, &limited) {
				return err
			}

			logging.From(ctx).WarnContext(
				ctx,
				"sending a comment to the forge failed; the next sweep will try again",
				"comment_id", comment.ID.String(),
				"error", err.Error(),
			)

			return nil
		}

		if _, err := s.mirrors.CreateComment(ctx, entity.CommentMirror{
			WorkspaceID:     from.workspaceID(),
			IssueID:         mirror.IssueID,
			CommentID:       comment.ID,
			MirrorID:        mirror.ID,
			Provider:        from.connection.Provider,
			RepositoryName:  from.repository.FullName,
			ExternalID:      posted.ExternalID,
			ExternalAuthor:  posted.Author,
			Origin:          entity.MirrorOriginNorn,
			BodyHash:        entity.MirrorHash(comment.Body),
			SourceUpdatedAt: stamp(at),
		}); err != nil {
			return err
		}
	}

	return nil
}
