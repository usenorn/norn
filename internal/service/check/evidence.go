package check

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *checksService) Evidence(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
) ([]entity.EvidenceRecord, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	_, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
	if err != nil {
		return nil, err
	}

	records, err := s.evidence.ListByCheck(ctx, workspaceID, checkID)
	if err != nil {
		return nil, err
	}

	horizon, err := s.horizon(ctx, workspaceID, issueID, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	return entity.NewCheckReport(held, records, horizon).Evidence, nil
}

func (s *checksService) Submit(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.SubmitEvidenceInput,
) (service.SubmittedEvidence, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.SubmittedEvidence{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateEvidenceVerdict("verdict", input.Verdict),
		entity.ValidateEvidenceChannel("channel", input.Channel),
		entity.ValidateEvidenceCommand("command", input.Command),
	); err != nil {
		return service.SubmittedEvidence{}, err
	}

	if input.Output == "" {
		return service.SubmittedEvidence{}, entity.ErrEvidenceEmpty
	}

	var submitted service.SubmittedEvidence

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if held.Approval == entity.CheckApprovalDeclined {
			return entity.ErrCheckDeclined
		}

		if held.Resolved() {
			return entity.ErrCheckSettled
		}

		received := time.Now().UTC()

		output, outputRedactions := entity.RedactEvidence(input.Output)
		command, commandRedactions := entity.RedactEvidence(input.Command)

		output, truncated := entity.TruncateEvidenceOutput(output)

		record := entity.Evidence{
			WorkspaceID: workspaceID,
			IssueID:     issue.ID,
			CheckID:     held.ID,
			Verdict:     input.Verdict,
			Channel:     input.Channel,
			Command:     command,
			Output:      output,
			Truncated:   truncated,
			Redactions:  outputRedactions + commandRedactions,
			ExitCode:    input.ExitCode,
			ObservedAt:  entity.ObservationTime(input.ObservedAt, received),
			ReceivedAt:  received,
			Actor:       decision.ActivityActor(),
		}

		if err := s.stampHead(ctx, workspaceID, issue.ID, &record); err != nil {
			return err
		}

		stored, err := s.evidence.Append(ctx, record)
		if err != nil {
			return err
		}

		horizon, err := s.horizon(ctx, workspaceID, issue.ID, received)
		if err != nil {
			return err
		}

		filed, err := s.evidence.Digest(ctx, workspaceID, issue.ID)
		if err != nil {
			return err
		}

		submitted = service.SubmittedEvidence{
			Record: entity.EvidenceRecord{Evidence: stored, Expiry: held.Expiry(stored, horizon)},
			Report: entity.NewCheckReport(held, filed, horizon),
		}

		if submitted.Report.State == entity.CheckStateFailed &&
			entity.NewCheckReport(held, without(filed, stored.ID), horizon).State !=
				entity.CheckStateFailed {
			if err := s.announce(
				ctx, issue, decision, entity.NotificationKindCheckFailed,
			); err != nil {
				return err
			}
		}

		return s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issue.ID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindEvidenceAdded,
			Field:       entity.ActivityFieldEvidence,
			ToValue:     held.Statement,
			Version:     issue.Version,
		})
	})
	if err != nil {
		return service.SubmittedEvidence{}, err
	}

	s.resumeWhenClear(ctx, workspaceID, issueID)

	return submitted, nil
}

func (s *checksService) stampHead(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	record *entity.Evidence,
) error {
	links, err := s.codeLinks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}

	link, found := entity.EvidenceLink(links)
	if !found {
		return nil
	}

	record.CodeLinkID = link.ID
	record.CommitSHA = link.HeadSHA

	return nil
}

func without(evidence []entity.Evidence, id uuid.UUID) []entity.Evidence {
	kept := make([]entity.Evidence, 0, len(evidence))

	for _, piece := range evidence {
		if piece.ID != id {
			kept = append(kept, piece)
		}
	}

	return kept
}
