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
) ([]entity.Evidence, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, _, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision); err != nil {
		return nil, err
	}

	return s.evidence.ListByCheck(ctx, workspaceID, checkID)
}

func (s *checksService) Submit(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.SubmitEvidenceInput,
) (entity.Evidence, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Evidence{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateEvidenceVerdict("verdict", input.Verdict),
		entity.ValidateEvidenceChannel("channel", input.Channel),
		entity.ValidateEvidenceCommand("command", input.Command),
	); err != nil {
		return entity.Evidence{}, err
	}

	if input.Output == "" {
		return entity.Evidence{}, entity.ErrEvidenceEmpty
	}

	var stored entity.Evidence

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

		stored, err = s.evidence.Append(ctx, record)
		if err != nil {
			return err
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
		return entity.Evidence{}, err
	}

	return stored, nil
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
