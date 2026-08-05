package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *issuesService) follow(ctx context.Context, issue entity.Issue, accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return nil
	}

	return s.followers.Follow(ctx, entity.IssueFollower{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AccountID:   accountID,
	})
}

func (s *issuesService) notifyAssigned(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	assignee uuid.UUID,
	bulkActionID uuid.UUID,
) error {
	if assignee == uuid.Nil {
		return nil
	}

	if err := s.follow(ctx, issue, assignee); err != nil {
		return err
	}

	attribution := decision.ActivityActor()

	return s.notify.Record(ctx, entity.NotificationEvent{
		WorkspaceID:  issue.WorkspaceID,
		Subject:      entity.NotifyIssue(issue.ID),
		Kind:         entity.NotificationKindAssigned,
		Actor:        attribution.AccountID,
		ActorKind:    attribution.Kind,
		Target:       assignee,
		BulkActionID: bulkActionID,
	})
}

func (s *issuesService) notifyStateChanged(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	bulkActionID uuid.UUID,
) error {
	attribution := decision.ActivityActor()

	return s.notify.Record(ctx, entity.NotificationEvent{
		WorkspaceID:  issue.WorkspaceID,
		Subject:      entity.NotifyIssue(issue.ID),
		Kind:         entity.NotificationKindStateChanged,
		Actor:        attribution.AccountID,
		ActorKind:    attribution.Kind,
		BulkActionID: bulkActionID,
	})
}
