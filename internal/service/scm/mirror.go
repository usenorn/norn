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
	from source,
	decision entity.Decision,
	found service.ForgeIssue,
) error {
	// A repository set to write-only takes nothing from the forge. Bringing an issue across
	// from one is exactly what somebody chose that setting to prevent.
	if !from.repository.Direction().Pulls() {
		return nil
	}

	mirror, err := s.mirrors.GetByExternalID(
		ctx,
		from.workspaceID(),
		from.connection.Provider,
		from.repository.FullName,
		found.ExternalID,
	)

	switch {
	case errors.Is(err, entity.ErrIssueMirrorNotFound):
		if !labelled(found.Labels, from.repository.MirrorLabel) {
			return nil
		}

		return s.openMirroredIssue(ctx, from, decision, found)

	case err != nil:
		return err

	default:
		return s.reconcileMirror(ctx, from, decision, mirror, found)
	}
}

func labelled(labels []string, wanted string) bool {
	return slices.ContainsFunc(labels, func(label string) bool {
		return strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(wanted))
	})
}

// openMirroredIssue needs somewhere to put the issue. A platform issue names no files, so it
// belongs to whoever holds the repository's default route; a repository routed to two teams
// by default has no single answer and picking one would be a guess, so the issue is not
// created and the reason is logged rather than a team being invented.
func (s *sync) openMirroredIssue(
	ctx context.Context,
	from source,
	decision entity.Decision,
	found service.ForgeIssue,
) error {
	teams, err := s.teamsFor(ctx, from, nil)
	if err != nil {
		return err
	}

	if len(teams) != 1 {
		logging.From(ctx).InfoContext(
			ctx,
			"a labelled platform issue was not mirrored because its repository has no single "+
				"team to put it in",
			"repository_id", from.repository.ID.String(),
			"external_id", found.ExternalID,
			"teams", len(teams),
		)

		return nil
	}

	teamID := teams[0]

	if !decision.Scope.Covers(teamID) {
		return nil
	}

	scoped := identity.WithActor(ctx, from.connection.Actor())

	created, err := s.issueWriter.Create(scoped, service.CreateIssueInput{
		WorkspaceID: from.workspaceID(),
		TeamID:      teamID,
		Title:       found.Title,
		Description: mirroredBody(from, found),
	})
	if err != nil {
		return err
	}

	mirror, err := s.mirrors.Create(ctx, entity.IssueMirror{
		WorkspaceID:    from.workspaceID(),
		IssueID:        created.ID,
		RepositoryID:   from.repository.ID,
		Provider:       from.connection.Provider,
		RepositoryName: from.repository.FullName,
		ExternalID:     found.ExternalID,
		ExternalNumber: found.Number,
		URL:            found.URL,
		Origin:         entity.MirrorOriginPlatform,
		Direction:      entity.MirrorBoth,
	})
	if err != nil {
		return err
	}

	return s.mirrors.RecordPull(
		ctx,
		mirror.ID,
		entity.HashesOf(found.Title, mirroredBody(from, found), found.State),
		found.UpdatedAt,
		created.Version,
		time.Now().UTC(),
	)
}

// mirroredBody names the person who wrote it. The change itself is the integration's — that
// is what the rules require — but a body with no author reads as though nobody wrote it, and
// the platform account behind it usually has no Norn account to attribute it to.
func mirroredBody(from source, found service.ForgeIssue) string {
	if strings.TrimSpace(found.Author) == "" {
		return found.Body
	}

	opening := fmt.Sprintf(
		"%s on %s by %s: %s",
		strings.ToUpper(string(from.connection.Provider[:1]))+string(from.connection.Provider[1:]),
		from.repository.FullName,
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
// side moved goes that way; and when both moved, whichever moved last wins.
//
// Whichever loses is kept in full. An excerpt on the feed says an edit existed; it does not
// give it back, and the whole reason a rule is acceptable is that the person who wrote the
// losing side can recover what they wrote.
func (s *sync) reconcileMirror(
	ctx context.Context,
	from source,
	decision entity.Decision,
	mirror entity.IssueMirror,
	found service.ForgeIssue,
) error {
	if from.connection.Wrote(found.Author) {
		return nil
	}

	issue, err := s.issues.GetVisible(ctx, from.workspaceID(), mirror.IssueID, decision.Scope)
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

		bothMoved := mirror.NornChanged(field.name, issue) &&
			mirror.SourceChanged(field.name, field.remote)

		switch winner {
		case entity.MirrorWinnerSource:
			field.apply(field.remote)

			if bothMoved {
				s.keepDiscarded(ctx, from, mirror, issue, field.name, winner, field.local, field.remote)

				overwritten = append(overwritten, discardedActivity(
					from, decision, issue, field.name, field.local, field.remote,
				))
			}
		case entity.MirrorWinnerNorn:
			if bothMoved {
				s.keepDiscarded(ctx, from, mirror, issue, field.name, winner, field.remote, field.local)

				overwritten = append(overwritten, discardedActivity(
					from, decision, issue, field.name, field.remote, field.local,
				))
			}
		case entity.MirrorWinnerNeither:
		}
	}

	if err := s.applyAssignee(ctx, from, issue, found, &change); err != nil {
		return err
	}

	if err := s.applyLabels(ctx, from, decision, issue, found); err != nil {
		return err
	}

	if change.Title != nil || change.Description != nil || change.AssigneeID != nil {
		scoped := identity.WithActor(ctx, from.connection.Actor())

		if _, err := s.issueWriter.Update(
			scoped,
			from.workspaceID(),
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

	refreshed, err := s.issues.GetVisible(ctx, from.workspaceID(), issue.ID, decision.Scope)
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
	from source,
	decision entity.Decision,
	found service.ForgeIssue,
	comment service.ForgeComment,
) error {
	if !from.repository.Direction().Pulls() || from.connection.Wrote(comment.Author) {
		return nil
	}

	mirror, err := s.mirrors.GetByExternalID(
		ctx,
		from.workspaceID(),
		from.connection.Provider,
		from.repository.FullName,
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
		from.workspaceID(),
		from.connection.Provider,
		from.repository.FullName,
		comment.ExternalID,
	)

	switch {
	case err == nil:
		return nil
	case !errors.Is(err, entity.ErrIssueMirrorNotFound):
		return err
	}

	issue, err := s.issues.GetVisible(ctx, from.workspaceID(), mirror.IssueID, decision.Scope)
	if err != nil {
		return nil
	}

	routed, err := s.reaches(ctx, from, issue.TeamID)
	if err != nil {
		return err
	}

	if !routed {
		return nil
	}

	body := commentBody(comment)
	scoped := identity.WithActor(ctx, from.connection.Actor())

	posted, err := s.comments.Post(scoped, from.workspaceID(), mirror.IssueID, service.PostCommentInput{
		Body: body,
	})
	if err != nil {
		return err
	}

	_, err = s.mirrors.CreateComment(ctx, entity.CommentMirror{
		WorkspaceID:     from.workspaceID(),
		IssueID:         mirror.IssueID,
		CommentID:       posted.Comment.ID,
		MirrorID:        mirror.ID,
		Provider:        from.connection.Provider,
		RepositoryName:  from.repository.FullName,
		ExternalID:      comment.ExternalID,
		ExternalAuthor:  comment.Author,
		Origin:          entity.MirrorOriginPlatform,
		BodyHash:        entity.MirrorHash(body),
		SourceUpdatedAt: stamp(comment.UpdatedAt),
	})

	return err
}

// applyAssignee moves the issue to whoever the platform says holds it, but only when this
// workspace has said who that person is. An unmapped login leaves the assignee alone: a
// forge handle that resembles a name is not evidence, and acting on it puts work on a
// stranger.
func (s *sync) applyAssignee(
	ctx context.Context,
	from source,
	issue entity.Issue,
	found service.ForgeIssue,
	change *service.UpdateIssueInput,
) error {
	if len(found.Assignees) == 0 {
		return nil
	}

	identities, err := s.identities.List(ctx, from.workspaceID())
	if err != nil {
		return err
	}

	for _, login := range found.Assignees {
		account, mapped := identities.AccountFor(from.connection.Provider, login)
		if !mapped {
			continue
		}

		if account == issue.AssigneeAccountID {
			return nil
		}

		change.AssigneeID = &account

		return nil
	}

	logging.From(ctx).InfoContext(
		ctx,
		"a platform assignee is not mapped to anybody here, so the issue keeps its assignee",
		"issue_id", issue.ID.String(),
	)

	return nil
}

// applyLabels applies what this workspace already has and reports the rest. Creating a label
// because an outside system used the word would let anybody with push access add labels to
// somebody else's workspace.
func (s *sync) applyLabels(
	ctx context.Context,
	from source,
	decision entity.Decision,
	issue entity.Issue,
	found service.ForgeIssue,
) error {
	if len(found.Labels) == 0 {
		return nil
	}

	available, err := s.labels.ListByWorkspaceID(ctx, from.workspaceID(), decision.Scope)
	if err != nil {
		return err
	}

	matched, unmapped := entity.MapLabels(found.Labels, available)

	if len(unmapped) > 0 {
		logging.From(ctx).InfoContext(
			ctx,
			"a platform label has no counterpart here and was not applied",
			"issue_id", issue.ID.String(),
			"labels", strings.Join(unmapped, ", "),
		)
	}

	if len(matched) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(matched))
	for i, label := range matched {
		ids[i] = label.ID
	}

	scoped := identity.WithActor(ctx, from.connection.Actor())

	if _, err := s.issueWriter.SetLabels(scoped, from.workspaceID(), issue.ID, service.SetIssueLabelsInput{
		ExpectedVersion: issue.Version,
		LabelIDs:        ids,
	}); err != nil && !errors.Is(err, entity.ErrIssueStale) {
		return err
	}

	return nil
}

// keepDiscarded stores the losing value whole. A failure to store it must not stop the
// reconcile, but it does have to be loud: from that moment the rule really is silent
// last-write-wins, which is what this exists to prevent.
func (s *sync) keepDiscarded(
	ctx context.Context,
	from source,
	mirror entity.IssueMirror,
	issue entity.Issue,
	field string,
	winner entity.MirrorWinner,
	discarded, kept string,
) {
	if err := s.conflicts.Record(ctx, entity.MirrorConflict{
		WorkspaceID: from.workspaceID(),
		MirrorID:    mirror.ID,
		IssueID:     issue.ID,
		Field:       field,
		Winner:      winner,
		Discarded:   discarded,
		Kept:        kept,
		OccurredAt:  time.Now().UTC(),
	}); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"storing an edit that lost arbitration failed; that edit is now unrecoverable",
			"issue_id", issue.ID.String(),
			"field", field,
			"error", err.Error(),
		)
	}
}

func discardedActivity(
	from source,
	decision entity.Decision,
	issue entity.Issue,
	field, discarded, kept string,
) entity.Activity {
	return entity.Activity{
		WorkspaceID: from.workspaceID(),
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        entity.ActivityKindPropertyChanged,
		Field:       field,
		FromValue:   excerpt(discarded),
		ToValue:     excerpt(kept),
		Version:     issue.Version,
	}
}

func commentBody(comment service.ForgeComment) string {
	if strings.TrimSpace(comment.Author) == "" {
		return comment.Body
	}

	return comment.Author + " wrote:\n\n" + comment.Body
}
