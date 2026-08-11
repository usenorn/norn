package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func refused(ctx context.Context, decision entity.Decision) bool {
	return decision.Actor.Kind == entity.ActorKindAgent && !identity.Approved(ctx)
}

func overrider(ctx context.Context, decision entity.Decision) entity.ActivityAttribution {
	if approver, ok := identity.Approver(ctx); ok {
		return entity.ActivityAttribution{AccountID: approver, Kind: entity.ActorKindUser}
	}

	return decision.ActivityActor()
}

func (s *issuesService) recordOverride(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	overridden []entity.Check,
	acknowledged bool,
) error {
	if len(overridden) == 0 {
		return nil
	}

	shown := ""
	if acknowledged || identity.Approved(ctx) {
		shown = entity.OverrideAcknowledged
	}

	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       overrider(ctx, decision),
		Kind:        entity.ActivityKindChecksOverridden,
		Field:       entity.ActivityFieldChecks,
		FromValue:   shown,
		ToValue:     entity.CheckStatements(overridden),
		Version:     issue.Version + 1,
	})
}

func (s *issuesService) resumeParent(ctx context.Context, issue entity.Issue, target entity.WorkflowState) {
	if issue.ParentIssueID == uuid.Nil || target.ID == uuid.Nil || entity.OpenCategory(target.Category) {
		return
	}

	if err := s.jobs.EnqueueSCMResume(ctx, entity.SCMResumePayload{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ParentIssueID,
	}); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"asking source control to reconsider a deferred move failed; it stays deferred until "+
				"the issue is touched again",
			"issue_id", issue.ParentIssueID.String(),
			"error", err.Error(),
		)
	}
}
