package issue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

// remember keeps what a description said at this version. The issue row holds only the text as
// it stands now, so without this an edit erases what it replaced and nobody can see what a
// requirement was when the work started.
func (s *issuesService) remember(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	source entity.RevisionSource,
) error {
	now := time.Now().UTC()

	writing := entity.IssueDescriptionRevision{
		WorkspaceID:     issue.WorkspaceID,
		IssueID:         issue.ID,
		IssueVersion:    issue.Version,
		Doc:             issue.DescriptionDoc,
		Markdown:        issue.Description,
		AuthorAccountID: decision.Actor.AccountID,
		Source:          source,
		CreatedAt:       now,
	}

	// A description saved as it is typed would leave an entry per pause. One sitting by one
	// person is one entry, so the history reads as the times the text was rewritten.
	held, err := s.revisions.Latest(ctx, issue.WorkspaceID, issue.ID)
	if err == nil && held.Continues(decision.Actor.AccountID, source, now) {
		writing.ID = held.ID

		return s.revisions.Replace(ctx, writing)
	}

	if err != nil && !errors.Is(err, entity.ErrIssueRevisionNotFound) {
		return err
	}

	return s.revisions.Record(ctx, writing)
}

func (s *issuesService) DescriptionRevisions(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	limit int,
) ([]entity.IssueDescriptionRevision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > entity.IssueRevisionPageMaxSize {
		limit = entity.IssueRevisionPageDefaultSize
	}

	return s.revisions.List(ctx, workspaceID, issueID, limit)
}

// RestoreDescription writes the old text forward rather than rewinding to it, so the text it
// replaced is kept as well and the restore itself can be undone.
func (s *issuesService) RestoreDescription(
	ctx context.Context,
	workspaceID, issueID, revisionID uuid.UUID,
	expectedVersion int,
) (entity.Issue, error) {
	revision, err := s.revisions.GetByID(ctx, workspaceID, revisionID)
	if err != nil {
		return entity.Issue{}, err
	}

	if revision.IssueID != issueID {
		return entity.Issue{}, entity.ErrIssueRevisionNotFound
	}

	restored := revision.Doc

	return s.Update(ctx, workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: expectedVersion,
		DescriptionDoc:  &restored,
		Restoring:       true,
	})
}
