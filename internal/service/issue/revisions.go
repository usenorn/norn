package issue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

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
