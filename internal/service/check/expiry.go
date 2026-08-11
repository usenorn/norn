package check

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *checksService) SweepExpiry(ctx context.Context) error {
	stale, err := s.checks.ListStaleIssues(ctx, entity.CheckTimeLimitDefault, s.sweepBatch)
	if err != nil {
		return err
	}

	announced := 0

	for _, issue := range stale {
		count, err := s.announceExpiry(ctx, issue.WorkspaceID, issue.IssueID)
		if err != nil {
			logging.From(ctx).ErrorContext(
				ctx,
				"announcing expired proof on an issue failed, so the sweep moves on",
				"workspace_id", issue.WorkspaceID.String(),
				"issue_id", issue.IssueID.String(),
				"error", err.Error(),
			)

			continue
		}

		announced += count
	}

	logging.From(ctx).InfoContext(
		ctx, "check expiry sweep finished", "issues", len(stale), "announced", announced,
	)

	return nil
}

func (s *checksService) announceExpiry(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (int, error) {
	checks, err := s.checks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return 0, err
	}

	evidence, err := s.evidence.Digest(ctx, workspaceID, issueID)
	if err != nil {
		return 0, err
	}

	horizon, err := s.horizon(ctx, workspaceID, issueID, time.Now().UTC())
	if err != nil {
		return 0, err
	}

	issue, err := s.issues.GetVisible(
		ctx, workspaceID, issueID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
	)
	if err != nil {
		return 0, err
	}

	announced := 0

	for _, report := range entity.ReportChecks(checks, evidence, horizon) {
		lost, found := expiredProof(report)
		if !found {
			continue
		}

		if err := s.checks.AnnounceExpiry(ctx, workspaceID, report.Check.ID, lost.Evidence.ID); err != nil {
			return announced, err
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issueID),
			Actor:       entity.ActivityAttribution{Kind: entity.ActorKindSystem},
			Kind:        entity.ActivityKindCheckExpired,
			Field:       entity.ActivityFieldCheck,
			FromValue:   string(lost.Expiry),
			ToValue:     report.Check.Statement,
			Version:     issue.Version,
		}); err != nil {
			return announced, err
		}

		announced++
	}

	return announced, nil
}

func expiredProof(report entity.CheckReport) (entity.EvidenceRecord, bool) {
	if !report.Blocks() || report.State != entity.CheckStateUnproven {
		return entity.EvidenceRecord{}, false
	}

	var newest entity.EvidenceRecord

	found := false

	for _, record := range report.Evidence {
		if !record.Expiry.Expired() || !record.Evidence.Verdict.Proves() {
			continue
		}

		if !found || record.Evidence.ReceivedAt.After(newest.Evidence.ReceivedAt) {
			newest, found = record, true
		}
	}

	return newest, found
}
