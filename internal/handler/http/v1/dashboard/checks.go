package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueChecks(
	ctx context.Context,
	request api.ListWorkspaceIssueChecksRequestObject,
) (api.ListWorkspaceIssueChecksResponseObject, error) {
	checks, err := h.checks.List(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueChecks200JSONResponse(issueCheckListDTO(checks)), nil
}

func (h *handler) AddWorkspaceIssueChecks(
	ctx context.Context,
	request api.AddWorkspaceIssueChecksRequestObject,
) (api.AddWorkspaceIssueChecksResponseObject, error) {
	drafted := make([]service.NewCheckInput, 0, len(request.Body.Checks))

	for _, entry := range request.Body.Checks {
		drafted = append(drafted, service.NewCheckInput{
			Statement: entry.Statement,
			Method:    entity.CheckMethod(entry.Method),
			Proof:     entry.Proof,
			TimeLimit: checkTimeLimit(entry.TimeLimitSeconds),
		})
	}

	added, err := h.checks.Add(ctx, request.WorkspaceId, request.IssueId, service.AddChecksInput{
		Checks:    drafted,
		Reasoning: agentReasoningFrom(request.Body.Reasoning),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceIssueChecks201JSONResponse(issueCheckDTOs(added)), nil
}

func (h *handler) RemoveWorkspaceIssueCheck(
	ctx context.Context,
	request api.RemoveWorkspaceIssueCheckRequestObject,
) (api.RemoveWorkspaceIssueCheckResponseObject, error) {
	if err := h.checks.Remove(ctx, request.WorkspaceId, request.IssueId, request.CheckId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceIssueCheck204Response{}, nil
}

func (h *handler) DecideWorkspaceIssueCheck(
	ctx context.Context,
	request api.DecideWorkspaceIssueCheckRequestObject,
) (api.DecideWorkspaceIssueCheckResponseObject, error) {
	decided, err := h.checks.Decide(
		ctx,
		request.WorkspaceId,
		request.IssueId,
		request.CheckId,
		service.DecideCheckInput{Approval: entity.CheckApproval(request.Body.Approval)},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DecideWorkspaceIssueCheck200JSONResponse(issueCheckDTO(decided)), nil
}

func (h *handler) WaiveWorkspaceIssueCheck(
	ctx context.Context,
	request api.WaiveWorkspaceIssueCheckRequestObject,
) (api.WaiveWorkspaceIssueCheckResponseObject, error) {
	waived, err := h.checks.Waive(
		ctx,
		request.WorkspaceId,
		request.IssueId,
		request.CheckId,
		service.WaiveCheckInput{Reason: request.Body.Reason},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.WaiveWorkspaceIssueCheck200JSONResponse(issueCheckDTO(waived)), nil
}

func (h *handler) DeclareWorkspaceIssueCheckGap(
	ctx context.Context,
	request api.DeclareWorkspaceIssueCheckGapRequestObject,
) (api.DeclareWorkspaceIssueCheckGapResponseObject, error) {
	input := service.DeclareGapInput{Reason: request.Body.Reason}

	if request.Body.Title != nil {
		input.Title = *request.Body.Title
	}

	declared, err := h.checks.DeclareGap(
		ctx, request.WorkspaceId, request.IssueId, request.CheckId, input,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeclareWorkspaceIssueCheckGap201JSONResponse{
		Check: issueCheckDTO(declared.Check),
		Issue: issueDTO(declared.Child),
	}, nil
}

func (h *handler) ListWorkspaceIssueCheckEvidence(
	ctx context.Context,
	request api.ListWorkspaceIssueCheckEvidenceRequestObject,
) (api.ListWorkspaceIssueCheckEvidenceResponseObject, error) {
	records, err := h.checks.Evidence(
		ctx, request.WorkspaceId, request.IssueId, request.CheckId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueCheckEvidence200JSONResponse(checkEvidenceRecordDTOs(records)), nil
}

func (h *handler) SubmitWorkspaceIssueCheckEvidence(
	ctx context.Context,
	request api.SubmitWorkspaceIssueCheckEvidenceRequestObject,
) (api.SubmitWorkspaceIssueCheckEvidenceResponseObject, error) {
	input := service.SubmitEvidenceInput{
		Verdict: entity.EvidenceVerdict(request.Body.Verdict),
		Channel: entity.EvidenceChannel(request.Body.Channel),
		Output:  request.Body.Output,
	}

	if request.Body.ObservedAt != nil {
		input.ObservedAt = *request.Body.ObservedAt
	}

	if request.Body.Command != nil {
		input.Command = *request.Body.Command
	}

	if request.Body.ExitCode != nil {
		code := int(*request.Body.ExitCode)
		input.ExitCode = &code
	}

	submitted, err := h.checks.Submit(
		ctx, request.WorkspaceId, request.IssueId, request.CheckId, input,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SubmitWorkspaceIssueCheckEvidence201JSONResponse{
		Evidence: checkEvidenceRecordDTO(submitted.Record),
		Check:    issueCheckReportDTO(submitted.Report),
	}, nil
}

func checkTimeLimit(seconds *int32) *time.Duration {
	if seconds == nil {
		return nil
	}

	window := time.Duration(*seconds) * time.Second

	return &window
}
