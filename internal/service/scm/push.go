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

func (s *sync) pushMirrors(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	decision entity.Decision,
	at time.Time,
) error {
	if !from.repository.Direction().Pushes() {
		return nil
	}

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

	if login, mapped := s.assigneeLogin(ctx, from, current); mapped {
		patch.Assignee = &login
		changed = true
	}

	if names := entity.LabelNames(current.Labels); len(names) > 0 {
		patch.Labels = names
		changed = true
	}

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

func (s *sync) assigneeLogin(
	ctx context.Context,
	from source,
	issue entity.Issue,
) (string, bool) {
	if issue.AssigneeAccountID == uuid.Nil {
		return "", false
	}

	identities, err := s.identities.List(ctx, from.workspaceID())
	if err != nil {
		logWarn(ctx, "reading platform identities failed", from.repository.ID, err)

		return "", false
	}

	return identities.LoginFor(from.connection.Provider, issue.AssigneeAccountID)
}

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
