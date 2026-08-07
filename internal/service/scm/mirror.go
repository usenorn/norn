package scm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

// mirrorIssue keeps a platform issue and a Norn issue in step. A platform issue becomes a
// Norn one only when it carries the label the connection was told to watch for, because a
// team using both systems is a supported way to work and a repository's whole backlog
// arriving unasked is not.
func (s *sync) mirrorIssue(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	found service.ForgeIssue,
) error {
	mirror, err := s.mirrors.GetByExternalID(
		ctx,
		connection.WorkspaceID,
		connection.Provider,
		connection.Repository,
		found.ExternalID,
	)

	switch {
	case errors.Is(err, entity.ErrIssueMirrorNotFound):
		if !labelled(found.Labels, connection.MirrorLabel) {
			return nil
		}

		return s.openMirroredIssue(ctx, connection, decision, found)

	case err != nil:
		return err

	default:
		return s.reconcileMirror(ctx, connection, decision, mirror, found)
	}
}

func labelled(labels []string, wanted string) bool {
	return slices.ContainsFunc(labels, func(label string) bool {
		return strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(wanted))
	})
}

// openMirroredIssue needs somewhere to put the issue. A connection attached to the whole
// workspace names no team, and picking one would be a guess, so the issue is not created and
// the reason is logged rather than a team being invented.
func (s *sync) openMirroredIssue(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	found service.ForgeIssue,
) error {
	if connection.TeamID == uuid.Nil {
		logging.From(ctx).InfoContext(
			ctx,
			"a labelled platform issue was not mirrored because the connection names no team",
			"connection_id", connection.ID.String(),
			"external_id", found.ExternalID,
		)

		return nil
	}

	if !decision.Scope.Covers(connection.TeamID) {
		return nil
	}

	scoped := identity.WithActor(ctx, connection.Actor())

	created, err := s.issueWriter.Create(scoped, service.CreateIssueInput{
		WorkspaceID: connection.WorkspaceID,
		TeamID:      connection.TeamID,
		Title:       found.Title,
		Description: mirroredBody(connection, found),
	})
	if err != nil {
		return err
	}

	mirror, err := s.mirrors.Create(ctx, entity.IssueMirror{
		WorkspaceID:    connection.WorkspaceID,
		IssueID:        created.ID,
		ConnectionID:   connection.ID,
		Provider:       connection.Provider,
		Repository:     connection.Repository,
		ExternalID:     found.ExternalID,
		ExternalNumber: found.Number,
		URL:            found.URL,
		Origin:         entity.MirrorOriginPlatform,
	})
	if err != nil {
		return err
	}

	return s.mirrors.RecordPull(
		ctx,
		mirror.ID,
		entity.HashesOf(found.Title, mirroredBody(connection, found), found.State),
		found.UpdatedAt,
		created.Version,
		time.Now().UTC(),
	)
}

// mirroredBody names the person who wrote it. The change itself is the integration's — that
// is what the rules require — but a body with no author reads as though nobody wrote it, and
// the platform account behind it usually has no Norn account to attribute it to.
func mirroredBody(connection entity.SCMConnection, found service.ForgeIssue) string {
	if strings.TrimSpace(found.Author) == "" {
		return found.Body
	}

	opening := fmt.Sprintf(
		"%s on %s by %s: %s",
		strings.ToUpper(string(connection.Provider[:1]))+string(connection.Provider[1:]),
		connection.Repository,
		found.Author,
		found.URL,
	)

	if strings.TrimSpace(found.Body) == "" {
		return opening
	}

	return opening + "\n\n" + found.Body
}

// reconcileMirror decides per field. A value that comes back exactly as it was pushed is the
// forge echoing this instance and is dropped before anything is written; a field only one
// side moved goes that way; and when both moved, whichever moved last wins and the other is
// recorded on the issue's own feed rather than disappearing.
func (s *sync) reconcileMirror(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	mirror entity.IssueMirror,
	found service.ForgeIssue,
) error {
	if connection.Wrote(found.Author) {
		return nil
	}

	issue, err := s.issues.GetVisible(ctx, connection.WorkspaceID, mirror.IssueID, decision.Scope)
	if err != nil {
		return nil
	}

	body := found.Body

	change := service.UpdateIssueInput{ExpectedVersion: issue.Version}
	overwritten := make([]entity.Activity, 0, 2)

	for _, field := range []struct {
		name   string
		remote string
		local  string
		apply  func(string)
	}{
		{
			name:   entity.IssueFieldTitle,
			remote: found.Title,
			local:  issue.Title,
			apply:  func(value string) { change.Title = &value },
		},
		{
			name:   entity.IssueFieldDescription,
			remote: body,
			local:  issue.Description,
			apply:  func(value string) { change.Description = &value },
		},
	} {
		winner := entity.ArbitrateMirror(
			mirror.NornChanged(field.name, issue),
			mirror.SourceChanged(field.name, field.remote),
			issue.UpdatedAt,
			found.UpdatedAt,
		)

		switch winner {
		case entity.MirrorWinnerSource:
			field.apply(field.remote)
		case entity.MirrorWinnerNorn:
			if mirror.SourceChanged(field.name, field.remote) {
				overwritten = append(overwritten, entity.Activity{
					WorkspaceID: connection.WorkspaceID,
					Subject:     entity.IssueSubject(issue.ID),
					Actor:       decision.ActivityActor(),
					Kind:        entity.ActivityKindPropertyChanged,
					Field:       field.name,
					FromValue:   excerpt(field.remote),
					ToValue:     excerpt(field.local),
					Version:     issue.Version,
				})
			}
		case entity.MirrorWinnerNeither:
		}
	}

	if change.Title != nil || change.Description != nil {
		scoped := identity.WithActor(ctx, connection.Actor())

		if _, err := s.issueWriter.Update(
			scoped,
			connection.WorkspaceID,
			issue.ID,
			change,
		); err != nil && !errors.Is(err, entity.ErrIssueStale) {
			return err
		}
	}

	for _, record := range overwritten {
		if err := s.activity.Record(ctx, record); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"recording an overwritten mirrored value failed",
				"issue_id", issue.ID.String(),
				"error", err.Error(),
			)
		}
	}

	refreshed, err := s.issues.GetVisible(ctx, connection.WorkspaceID, issue.ID, decision.Scope)
	if err != nil {
		return nil
	}

	return s.mirrors.RecordPull(
		ctx,
		mirror.ID,
		entity.HashesOf(refreshed.Title, refreshed.Description, found.State),
		found.UpdatedAt,
		refreshed.Version,
		time.Now().UTC(),
	)
}

const mirrorExcerptMax = 512

func excerpt(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > mirrorExcerptMax {
		return trimmed[:mirrorExcerptMax]
	}

	return trimmed
}

// mirrorComment carries a platform comment onto the Norn issue once. Three things stop it
// bouncing: a comment already mirrored is recognised by its own id, a comment written by
// this connection's own token is this instance's voice coming back, and a comment created
// here is never pushed out again because it holds a mirror row from the moment it exists.
func (s *sync) mirrorComment(
	ctx context.Context,
	connection entity.SCMConnection,
	decision entity.Decision,
	found service.ForgeIssue,
	comment service.ForgeComment,
) error {
	if connection.Wrote(comment.Author) {
		return nil
	}

	mirror, err := s.mirrors.GetByExternalID(
		ctx,
		connection.WorkspaceID,
		connection.Provider,
		connection.Repository,
		found.ExternalID,
	)
	if errors.Is(err, entity.ErrIssueMirrorNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	_, err = s.mirrors.GetCommentByExternalID(
		ctx,
		connection.WorkspaceID,
		connection.Provider,
		connection.Repository,
		comment.ExternalID,
	)

	switch {
	case err == nil:
		return nil
	case !errors.Is(err, entity.ErrIssueMirrorNotFound):
		return err
	}

	issue, err := s.issues.GetVisible(ctx, connection.WorkspaceID, mirror.IssueID, decision.Scope)
	if err != nil || !connection.Covers(issue.TeamID) {
		return nil
	}

	body := commentBody(comment)
	scoped := identity.WithActor(ctx, connection.Actor())

	posted, err := s.comments.Post(scoped, connection.WorkspaceID, mirror.IssueID, service.PostCommentInput{
		Body: body,
	})
	if err != nil {
		return err
	}

	_, err = s.mirrors.CreateComment(ctx, entity.CommentMirror{
		WorkspaceID:     connection.WorkspaceID,
		IssueID:         mirror.IssueID,
		CommentID:       posted.Comment.ID,
		ConnectionID:    connection.ID,
		Provider:        connection.Provider,
		Repository:      connection.Repository,
		ExternalID:      comment.ExternalID,
		ExternalAuthor:  comment.Author,
		Origin:          entity.MirrorOriginPlatform,
		BodyHash:        entity.MirrorHash(body),
		SourceUpdatedAt: stamp(comment.UpdatedAt),
	})

	return err
}

func commentBody(comment service.ForgeComment) string {
	if strings.TrimSpace(comment.Author) == "" {
		return comment.Body
	}

	return comment.Author + " wrote:\n\n" + comment.Body
}
