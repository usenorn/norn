package scm

import (
	"context"
	"net/url"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *sync) announce(ctx context.Context, from source, link entity.CodeLink, issue entity.Issue) {
	if link.Kind != entity.CodeLinkChange || !link.Resolving || link.Number <= 0 {
		return
	}

	if link.State.Settled() {
		return
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return
	}

	address, err := s.issueURL(ctx, issue)
	if err != nil {
		logWarn(ctx, "reading the workspace to announce a change failed", from.repository.ID, err)

		return
	}

	claimed, err := s.links.ClaimAnnouncement(ctx, link.ID, time.Now().UTC())
	if err != nil {
		logWarn(ctx, "claiming a change announcement failed", from.repository.ID, err)

		return
	}

	if !claimed {
		return
	}

	if err := forge.PostChangeComment(
		ctx,
		from.target(),
		link.Number,
		entity.AnnounceIssueOnChange(issue.Reference(), issue.Title, address),
	); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"announcing the issue a change resolves failed; the next delivery will try again",
			"link_id", link.ID.String(),
			"error", err.Error(),
		)

		if err := s.links.ReleaseAnnouncement(ctx, link.ID); err != nil {
			logWarn(ctx, "releasing a change announcement failed", from.repository.ID, err)
		}
	}
}

func (s *sync) issueURL(ctx context.Context, issue entity.Issue) (string, error) {
	workspace, err := s.workspaces.GetByID(ctx, issue.WorkspaceID)
	if err != nil {
		return "", err
	}

	return url.JoinPath(s.baseURL, workspace.Slug, "issues", issue.Reference())
}
