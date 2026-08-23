package preview

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

const signInPath = "/sign-in"

func (s *previewsService) Authorize(
	ctx context.Context,
	host, returnTo string,
) (entity.PreviewAccess, error) {
	preview, err := s.serving(ctx, host)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	ctx, actor, err := s.acting(ctx, preview)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	if actor.Anonymous() {
		return entity.PreviewAccess{
			Verdict:  entity.PreviewSignIn,
			Preview:  preview,
			Redirect: s.signIn(host, returnTo),
		}, nil
	}

	if _, err := s.runs.Visible(ctx, preview.WorkspaceID, preview.ExecutionID); err != nil {
		return entity.PreviewAccess{}, err
	}

	now := time.Now().UTC()

	ticket, err := s.grants.IssueTicket(
		ctx,
		entity.NewPreviewGrant(
			preview, actor.Authority(), uuid.Nil, now, now.Add(s.settings.SessionTTL),
		),
		s.settings.TicketTTL,
	)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	return entity.PreviewAccess{
		Verdict:  entity.PreviewAllowed,
		Preview:  preview,
		Redirect: handover(preview, s.settings.Scheme, ticket, returnTo),
	}, nil
}

func (s *previewsService) acting(
	ctx context.Context,
	preview entity.PreviewSession,
) (context.Context, entity.Actor, error) {
	if actor, ok := identity.Actor(ctx); ok {
		return ctx, actor, nil
	}

	held := slices.Clone(identity.SignedIn(ctx))
	slices.SortStableFunc(held, func(a, b entity.Session) int {
		return a.IssuedAt.Compare(b.IssuedAt)
	})

	for _, session := range held {
		candidate := identity.WithSession(ctx, session)

		if _, err := s.runs.Visible(
			candidate, preview.WorkspaceID, preview.ExecutionID,
		); err != nil {
			if refusedFor(err) {
				continue
			}

			return ctx, entity.Actor{}, err
		}

		actor, _ := identity.Actor(candidate)

		return candidate, actor, nil
	}

	return ctx, entity.Actor{}, nil
}

func refusedFor(err error) bool {
	var denied entity.AccessDeniedError

	return errors.As(err, &denied) || errors.Is(err, entity.ErrExecutionNotFound)
}

func (s *previewsService) Introspect(
	ctx context.Context,
	host, grant string,
	client entity.SessionClient,
) (entity.PreviewAccess, error) {
	route, err := s.previews.RouteByHost(ctx, host)
	if err != nil {
		if errors.Is(err, entity.ErrPreviewNotFound) {
			return refused(entity.PreviewSession{}, entity.ErrPreviewNotFound), nil
		}

		return entity.PreviewAccess{}, err
	}

	preview := route.Preview

	if !preview.Open() {
		return refused(preview, entity.ErrPreviewClosed), nil
	}

	if grant == "" {
		return s.turnedAway(preview, host), nil
	}

	held, err := s.grants.Read(ctx, grant)
	if err != nil {
		if errors.Is(err, entity.ErrPreviewGrantNotFound) {
			return s.turnedAway(preview, host), nil
		}

		return entity.PreviewAccess{}, err
	}

	if held.Spent(preview.ID, time.Now().UTC()) {
		return s.turnedAway(preview, host), nil
	}

	if err := s.looked(ctx, preview, held, client); err != nil {
		return entity.PreviewAccess{}, err
	}

	return entity.PreviewAccess{
		Verdict:   entity.PreviewAllowed,
		Preview:   preview,
		RunnerID:  route.RunnerID,
		Path:      preview.Path,
		ExpiresAt: held.ExpiresAt,
	}, nil
}

func (s *previewsService) RedeemTicket(
	ctx context.Context,
	ticket string,
) (entity.PreviewAccess, error) {
	held, err := s.grants.RedeemTicket(ctx, ticket)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	now := time.Now().UTC()

	if !held.ExpiresAt.After(now) {
		return entity.PreviewAccess{}, entity.ErrPreviewGrantNotFound
	}

	token, err := s.grants.Issue(ctx, held, held.ExpiresAt.Sub(now))
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	return entity.PreviewAccess{
		Verdict:   entity.PreviewAllowed,
		Token:     token,
		Path:      held.Path,
		ExpiresAt: held.ExpiresAt,
	}, nil
}

func (s *previewsService) serving(
	ctx context.Context,
	host string,
) (entity.PreviewSession, error) {
	if !s.settings.Routable() {
		return entity.PreviewSession{}, entity.ErrPreviewNotRoutable
	}

	preview, err := s.previews.ByHost(ctx, host)
	if err != nil {
		return entity.PreviewSession{}, err
	}

	if !preview.Open() {
		return entity.PreviewSession{}, entity.ErrPreviewClosed
	}

	return preview, nil
}

func (s *previewsService) signIn(host, returnTo string) string {
	authorize := url.Values{}
	authorize.Set("host", host)

	if returnTo != "" {
		authorize.Set("return", returnTo)
	}

	back := url.Values{}
	back.Set("return", entity.PreviewAuthorizePath+"?"+authorize.Encode())

	return strings.TrimSuffix(s.app.BaseURL, "/") + signInPath + "?" + back.Encode()
}

func (s *previewsService) turnedAway(
	preview entity.PreviewSession,
	host string,
) entity.PreviewAccess {
	return entity.PreviewAccess{
		Verdict:  entity.PreviewSignIn,
		Preview:  preview,
		Redirect: s.signIn(host, ""),
	}
}

func (s *previewsService) looked(
	ctx context.Context,
	preview entity.PreviewSession,
	grant entity.PreviewGrant,
	client entity.SessionClient,
) error {
	viewer := preview.ID.String() + ":" + grant.Viewer() + ":" + client.IP.String()

	first, err := s.grants.FirstLook(ctx, viewer, s.settings.AuditWindow)
	if err != nil || !first {
		return err
	}

	execution, err := s.executions.GetByID(ctx, preview.ExecutionID)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.remember(ctx, execution, entity.ExecutionEvent{
			ExecutionID: execution.ID,
			Kind:        entity.ExecutionEventPreview,
			Actor:       viewerActor(grant),
			Reason:      looking(preview, grant, client),
			Detail:      lookDetail(preview, grant, client),
			OccurredAt:  time.Now().UTC(),
		}); err != nil {
			return err
		}

		s.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  execution.WorkspaceID,
			Action:       entity.AuditPreviewOpened,
			ResourceKind: string(entity.ResourceIssue),
			ResourceID:   execution.IssueID,
			ResourceName: execution.Reference(),
			SourceIP:     client.IP,
			UserAgent:    client.UserAgent,
			Detail:       map[string]string{"preview": preview.Name},
		})

		return nil
	})
}

func refused(preview entity.PreviewSession, reason error) entity.PreviewAccess {
	return entity.PreviewAccess{
		Verdict: entity.PreviewRefused,
		Preview: preview,
		Reason:  reason.Error(),
	}
}

func viewerActor(grant entity.PreviewGrant) entity.ExecutionActor {
	if grant.LinkID != uuid.Nil {
		return entity.ExecutionActor{Kind: entity.ActorKindSystem}
	}

	return entity.ExecutionActor{Kind: entity.ActorKindUser, AccountID: grant.AccountID}
}

func looking(
	preview entity.PreviewSession,
	grant entity.PreviewGrant,
	client entity.SessionClient,
) string {
	who := "somebody with a share link"
	if grant.LinkID == uuid.Nil {
		who = "a member of this workspace"
	}

	seen := who + " opened " + preview.Name
	if client.IP.IsValid() {
		seen += " from " + client.IP.String()
	}

	return seen
}

func lookDetail(
	preview entity.PreviewSession,
	grant entity.PreviewGrant,
	client entity.SessionClient,
) []byte {
	detail := map[string]string{
		"preview": preview.Name,
		"via":     "membership",
	}

	if grant.LinkID != uuid.Nil {
		detail["via"] = "share_link"
		detail["shareLinkId"] = grant.LinkID.String()
	}

	if client.IP.IsValid() {
		detail["ip"] = client.IP.String()
	}

	if client.UserAgent != "" {
		detail["userAgent"] = client.UserAgent
	}

	return object(detail)
}
