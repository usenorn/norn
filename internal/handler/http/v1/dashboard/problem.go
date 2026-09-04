package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const (
	problemContentType = "application/problem+json"
	problemType        = "about:blank"
	retryAfterHeader   = "Retry-After"
)

type problemResponse struct {
	status     int
	body       any
	retryAfter int
}

func baseProblem(status int, detail string) api.Problem {
	problem := api.Problem{
		Type:   problemType,
		Title:  http.StatusText(status),
		Status: int32(status),
	}

	if detail != "" {
		problem.Detail = &detail
	}

	return problem
}

func newProblem(status int, detail string) problemResponse {
	return problemResponse{status: status, body: baseProblem(status, detail)}
}

func (r problemResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", problemContentType)

	if r.retryAfter > 0 {
		w.Header().Set(retryAfterHeader, strconv.Itoa(r.retryAfter))
	}

	w.WriteHeader(r.status)

	return json.NewEncoder(w).Encode(r.body)
}

func problemFor(err error) (problemResponse, bool) {
	var validation entity.ValidationError
	if errors.As(err, &validation) {
		problem := baseProblem(http.StatusUnprocessableEntity, validation.Error())

		fields := make([]api.FieldError, len(validation.Fields))
		for i, field := range validation.Fields {
			fields[i] = api.FieldError{Field: field.Field, Code: field.Code}
		}

		problem.Errors = &fields

		return problemResponse{status: http.StatusUnprocessableEntity, body: problem}, true
	}

	var rejected entity.SCMCredentialsRejectedError
	if errors.As(err, &rejected) {
		return sourceControlRefused(api.SourceControlCredentialsRejected, err), true
	}

	var unreachable entity.SCMRepositoryUnreachableError
	if errors.As(err, &unreachable) {
		return sourceControlRefused(api.SourceControlRepositoryUnreachable, err), true
	}

	var refusedDestination entity.SCMDestinationRefusedError
	if errors.As(err, &refusedDestination) {
		return sourceControlRefused(api.SourceControlDestinationRefused, err), true
	}

	// A forge asking to be left alone is passed on with how long to wait, so the screen can
	// say when to try again rather than only that something went wrong.
	var limited entity.SCMRateLimitedError
	if errors.As(err, &limited) {
		base := baseProblem(http.StatusTooManyRequests, err.Error())

		return problemResponse{
			status:     http.StatusTooManyRequests,
			body:       base,
			retryAfter: int(limited.RetryAfter.Seconds()),
		}, true
	}

	var stale entity.IssueStaleError
	if errors.As(err, &stale) {
		base := baseProblem(http.StatusConflict, stale.Error())
		version := int32(stale.Version)
		conflicts := stale.Conflicts

		return problemResponse{
			status: http.StatusConflict,
			body: api.IssueConflictProblem{
				Code:      api.IssueConflictProblemCodeIssueStale,
				Conflicts: &conflicts,
				Version:   &version,
				Detail:    base.Detail,
				Instance:  base.Instance,
				Status:    base.Status,
				Title:     base.Title,
				Type:      base.Type,
			},
		}, true
	}

	var childrenOpen entity.IssueChildrenOpenError
	if errors.As(err, &childrenOpen) {
		base := baseProblem(http.StatusConflict, childrenOpen.Error())
		children := issueDTOs(childrenOpen.Children)

		return problemResponse{
			status: http.StatusConflict,
			body: api.IssueConflictProblem{
				Code:     api.IssueConflictProblemCodeIssueChildrenOpen,
				Children: &children,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true
	}

	var tooDeep entity.IssueTooDeepError
	if errors.As(err, &tooDeep) {
		return issueConflictProblem(api.IssueConflictProblemCodeIssueParentTooDeep, tooDeep), true
	}

	var relationHeld entity.IssueRelationExistsError
	if errors.As(err, &relationHeld) {
		return issueConflictProblem(api.IssueConflictProblemCodeIssueRelationExists, relationHeld), true
	}

	var stranded entity.IssueLabelsOutOfScopeError
	if errors.As(err, &stranded) {
		base := baseProblem(http.StatusConflict, stranded.Error())
		labels := labelDTOs(stranded.Labels)

		return problemResponse{
			status: http.StatusConflict,
			body: api.IssueConflictProblem{
				Code:     api.IssueConflictProblemCodeIssueLabelsOutOfScope,
				Labels:   &labels,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true
	}

	var usageChanged entity.LabelUsageChangedError
	if errors.As(err, &usageChanged) {
		problem := labelConflictProblem(api.LabelUsageChanged, err)

		body, ok := problem.body.(api.LabelConflictProblem)
		if ok {
			issues := int32(usageChanged.Issues)
			body.Issues = &issues
			problem.body = body
		}

		return problem, true
	}

	var sharedView entity.SavedViewSharedError
	if errors.As(err, &sharedView) {
		base := baseProblem(http.StatusConflict, sharedView.Error())
		sharing := api.SavedViewSharing(sharedView.Sharing)

		return problemResponse{
			status: http.StatusConflict,
			body: api.SavedViewConflictProblem{
				Code:     api.SavedViewShared,
				Sharing:  &sharing,
				TeamName: nilIfEmpty(sharedView.TeamName),
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true
	}

	var lastAdmin entity.LastWorkspaceAdminError
	if errors.As(err, &lastAdmin) {
		problem := membershipConflictProblem(api.MembershipConflictProblemCodeLastAdmin, lastAdmin)

		body, ok := problem.body.(api.MembershipConflictProblem)
		if ok {
			workspaceIDs := lastAdmin.WorkspaceIDs
			body.WorkspaceIds = &workspaceIDs
			problem.body = body
		}

		return problem, true
	}

	var workspaceDeleted entity.WorkspaceDeletedError
	if errors.As(err, &workspaceDeleted) {
		base := baseProblem(http.StatusConflict, workspaceDeleted.Error())

		return problemResponse{
			status: http.StatusConflict,
			body: api.WorkspaceDeletedProblem{
				Code:       api.WorkspaceDeletedProblemCodeWorkspaceDeleted,
				Detail:     base.Detail,
				Instance:   base.Instance,
				PurgeAfter: workspaceDeleted.PurgeAfter,
				Status:     base.Status,
				Title:      base.Title,
				Type:       base.Type,
			},
		}, true
	}

	var locked entity.AccountLockedError
	if errors.As(err, &locked) {
		base := baseProblem(http.StatusLocked, locked.Error())

		return problemResponse{
			status: http.StatusLocked,
			body: api.AccountLockedProblem{
				Code:      api.AccountLockedProblemCodeAccountLocked,
				Detail:    base.Detail,
				Instance:  base.Instance,
				Status:    base.Status,
				Title:     base.Title,
				Type:      base.Type,
				UnlocksAt: locked.UnlocksAt,
			},
		}, true
	}

	var wrongCode entity.SignInCodeIncorrectError
	if errors.As(err, &wrongCode) {
		base := baseProblem(http.StatusUnauthorized, wrongCode.Error())

		return problemResponse{
			status: http.StatusUnauthorized,
			body: api.SignInCodeIncorrectProblem{
				AttemptsLeft: int32(wrongCode.AttemptsLeft),
				Code:         api.SignInCodeIncorrect,
				Detail:       base.Detail,
				Instance:     base.Instance,
				Status:       base.Status,
				Title:        base.Title,
				Type:         base.Type,
			},
		}, true
	}

	var invalid entity.InvalidCredentialsError
	if errors.As(err, &invalid) {
		base := baseProblem(http.StatusUnauthorized, invalid.Error())

		return problemResponse{
			status: http.StatusUnauthorized,
			body: api.InvalidCredentialsProblem{
				AttemptsLeft: int32(invalid.AttemptsLeft),
				Code:         api.InvalidCredentials,
				Detail:       base.Detail,
				Instance:     base.Instance,
				Status:       base.Status,
				Title:        base.Title,
				Type:         base.Type,
			},
		}, true
	}

	var held entity.AgentActionHeldError
	if errors.As(err, &held) {
		return agentHeldProblem(held), true
	}

	var refused entity.EnforcementRefusedError
	if errors.As(err, &refused) {
		return enforcementProblem(refused.Blocker, refused.Error()), true
	}

	if errors.Is(err, entity.ErrSSOEncryptionKeyMissing) {
		return newProblem(http.StatusInternalServerError, err.Error()), true
	}

	if errors.Is(err, entity.ErrSSONotVerified) {
		return newProblem(http.StatusConflict, err.Error()), true
	}

	if failure, ok := entity.AsSSOError(err); ok {
		base := baseProblem(http.StatusUnprocessableEntity, failure.Message)
		stage := api.SsoStage(failure.Stage)

		problem := api.SsoProblem{
			Code:     api.SsoProblemCodeSsoFailed,
			Stage:    stage,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		}

		if failure.Detail != "" {
			message := failure.Detail
			problem.ProviderMessage = &message
		}

		return problemResponse{status: http.StatusUnprocessableEntity, body: problem}, true
	}

	var denied entity.AccessDeniedError
	if errors.As(err, &denied) && denied.Reason == entity.DenyReasonNoActor {
		return unauthorized(), true
	}

	if errors.As(err, &denied) && denied.Reason == entity.DenyReasonAgentRateLimited {
		return problemResponse{
			status:     http.StatusTooManyRequests,
			body:       baseProblem(http.StatusTooManyRequests, denied.Error()),
			retryAfter: int(entity.AgentActionWindow.Seconds()),
		}, true
	}

	if errors.As(err, &denied) && denied.Reason.Disclosed() {
		base := baseProblem(http.StatusForbidden, denied.Error())
		reason := api.ForbiddenProblemReason(denied.Reason)

		return problemResponse{
			status: http.StatusForbidden,
			body: api.ForbiddenProblem{
				Code:     api.ForbiddenProblemCodeForbidden,
				Reason:   &reason,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true
	}

	var wouldTriage entity.ImportWouldTriageError
	if errors.As(err, &wouldTriage) {
		problem := importConflictProblem(api.ImportWouldTriage, wouldTriage)

		body, ok := problem.body.(api.ImportConflictProblem)
		if ok {
			teams := wouldTriage.Teams
			body.Teams = &teams
			problem.body = body
		}

		return problem, true
	}

	var sourceRefused entity.ImportSourceRefusedError
	if errors.As(err, &sourceRefused) {
		base := baseProblem(http.StatusBadGateway, sourceRefused.Error())

		return problemResponse{
			status: http.StatusBadGateway,
			body: api.ImportSourceRefusedProblem{
				Code:     api.ImportSourceRefusedProblemCodeImportSourceRefused,
				Reason:   nilIfEmpty(sourceRefused.Reason),
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true
	}

	var throttled entity.ImportRateLimitedError
	if errors.As(err, &throttled) {
		base := baseProblem(http.StatusTooManyRequests, throttled.Error())

		return problemResponse{
			status: http.StatusTooManyRequests,
			body: api.RateLimitedProblem{
				Code:     api.RateLimited,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
			retryAfter: int(throttled.RetryAfter.Seconds()),
		}, true
	}

	switch {
	case errors.Is(err, entity.ErrIssueAlreadyOnTeam):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueAlreadyOnTeam, err), true

	case errors.Is(err, entity.ErrIssueDestinationIncapable):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueDestinationIncapable, err), true

	case errors.Is(err, entity.ErrIssueNotWaiting):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueNotWaiting, err), true

	case errors.Is(err, entity.ErrIssueStatusTransition):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueStatusTransition, err), true

	case errors.Is(err, entity.ErrIssueParentCycle):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueParentCycle, err), true

	case errors.Is(err, entity.ErrIssueParentNotActive):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueParentNotActive, err), true

	case errors.Is(err, entity.ErrIssueFilterTooComplex),
		errors.Is(err, entity.ErrIssueFilterAmbiguous),
		errors.Is(err, entity.ErrIssueGroupUnknown):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrBulkChangeEmpty),
		errors.Is(err, entity.ErrBulkChangeConflicting),
		errors.Is(err, entity.ErrBulkSetEmpty),
		errors.Is(err, entity.ErrBulkSetAmbiguous):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrIssueDelegationHeld):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueDelegationHeld, err), true

	case errors.Is(err, entity.ErrIssueDelegationAgentUnusable):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueDelegationAgentUnusable, err), true

	case errors.Is(err, entity.ErrIssueDelegationAgentNotYours):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueDelegationAgentNotYours, err), true

	case errors.Is(err, entity.ErrIssueDelegationUnassigned):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueDelegationUnassigned, err), true

	case errors.Is(err, entity.ErrIssueDelegationNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrRunnerCredentialInvalid):
		return runnerProblem(api.RunnerProblemCodeRunnerCredentialInvalid, err), true

	case errors.Is(err, entity.ErrRunnerRevoked):
		return runnerProblem(api.RunnerProblemCodeRunnerRevoked, err), true

	case errors.Is(err, entity.ErrRunnerAssertionForged):
		return runnerProblem(api.RunnerProblemCodeRunnerAssertionForged, err), true

	case errors.Is(err, entity.ErrRunnerAssertionStale):
		return runnerProblem(api.RunnerProblemCodeRunnerAssertionStale, err), true

	case errors.Is(err, entity.ErrRunnerAssertionReplayed):
		return runnerProblem(api.RunnerProblemCodeRunnerAssertionReplayed, err), true

	case errors.Is(err, entity.ErrRunnerAssertionMismatch):
		return runnerProblem(api.RunnerProblemCodeRunnerAssertionMismatch, err), true

	case errors.Is(err, entity.ErrRunnerNameTaken):
		return runnerConflictProblem(api.RunnerProblemCodeRunnerNameTaken, err), true

	case errors.Is(err, entity.ErrRunnerNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrRunnerEnrolmentNotAgent):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrCodebaseRootTaken):
		return codebaseConflictProblem(api.CodebaseProblemCodeCodebaseRootTaken, err), true

	case errors.Is(err, entity.ErrCodebaseNotDrifted):
		return codebaseConflictProblem(api.CodebaseProblemCodeCodebaseNotDrifted, err), true

	case errors.Is(err, entity.ErrCodebaseDisconnected):
		return codebaseConflictProblem(api.CodebaseProblemCodeCodebaseDisconnected, err), true

	case errors.Is(err, entity.ErrCodebaseNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrCodebaseNotRunner):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrExecutionNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrExecutionTransition):
		return executionConflictProblem(api.ExecutionTransition, err), true

	case errors.Is(err, entity.ErrExecutionFinished):
		return executionConflictProblem(api.ExecutionFinished, err), true

	case errors.Is(err, entity.ErrExecutionUnfinished):
		return executionConflictProblem(api.ExecutionUnfinished, err), true

	case errors.Is(err, entity.ErrExecutionNotReviewable):
		return executionConflictProblem(api.ExecutionNotReviewable, err), true

	case errors.Is(err, entity.ErrExecutionSelfApproval):
		return executionProblem(http.StatusForbidden, api.ExecutionSelfApproval, err), true

	case errors.Is(err, entity.ErrExecutionNoRunner):
		return executionConflictProblem(api.ExecutionNoRunner, err), true

	case errors.Is(err, entity.ErrExecutionChunkConflict):
		return executionConflictProblem(api.ExecutionChunkConflict, err), true

	case errors.Is(err, entity.ErrPreviewNotFound),
		errors.Is(err, entity.ErrPreviewShareNotFound),
		errors.Is(err, entity.ErrPreviewGrantNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrPreviewClosed):
		return previewConflictProblem(api.PreviewClosed, err), true

	case errors.Is(err, entity.ErrPreviewNotRoutable):
		return previewConflictProblem(api.PreviewNotRoutable, err), true

	case errors.Is(err, entity.ErrPreviewCrowded):
		return previewConflictProblem(api.PreviewCrowded, err), true

	case errors.Is(err, entity.ErrPreviewShareExpired):
		return previewConflictProblem(api.PreviewShareExpired, err), true

	case errors.Is(err, entity.ErrPreviewShareRevoked):
		return previewConflictProblem(api.PreviewShareRevoked, err), true

	case errors.Is(err, entity.ErrPreviewShareCrowded):
		return previewConflictProblem(api.PreviewShareCrowded, err), true

	case errors.Is(err, entity.ErrPreviewSharePasscode):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrPreviewShareGuessed):
		return newProblem(http.StatusTooManyRequests, err.Error()), true

	case errors.Is(err, entity.ErrPreviewNotExtendable):
		return executionConflictProblem(api.ExecutionNotReviewable, err), true

	case errors.Is(err, entity.ErrExecutionUploadTooLarge),
		errors.Is(err, entity.ErrExecutionUploadExhausted):
		return newProblem(http.StatusRequestEntityTooLarge, err.Error()), true

	case errors.Is(err, entity.ErrExecutionUploadEmpty),
		errors.Is(err, entity.ErrExecutionUploadCrowded),
		errors.Is(err, entity.ErrExecutionSequenceInvalid),
		errors.Is(err, entity.ErrExecutionTelemetryMinimal):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrIssueQuestionNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrIssueQuestionAnswered),
		errors.Is(err, entity.ErrIssueQuestionSettled):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrIssueQuestionUnanswerable):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrExecutionArtifactNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrRunnerKeyMalformed):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrIssueRelationSelf):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueRelationSelf, err), true

	case errors.Is(err, entity.ErrIssueRelationExists):
		return issueConflictProblem(api.IssueConflictProblemCodeIssueRelationExists, err), true
	}

	switch {
	case errors.Is(err, entity.ErrAccountNotFound),
		errors.Is(err, entity.ErrEmailChangeNotFound),
		errors.Is(err, entity.ErrWorkspaceNotFound),
		errors.Is(err, entity.ErrMembershipNotFound),
		errors.Is(err, entity.ErrTeamNotFound),
		errors.Is(err, entity.ErrTeamMembershipNotFound),
		errors.Is(err, entity.ErrIssueNotFound),
		errors.Is(err, entity.ErrIssueRelationNotFound),
		errors.Is(err, entity.ErrBulkActionNotFound),
		errors.Is(err, entity.ErrWorkflowStateNotFound),
		errors.Is(err, entity.ErrAvatarMissing),
		errors.Is(err, entity.ErrSSOConnectionNotFound),
		errors.Is(err, entity.ErrSSOStateNotFound),
		errors.Is(err, entity.ErrSSOIdentityNotFound),
		errors.Is(err, entity.ErrCycleNotFound),
		errors.Is(err, entity.ErrCycleCadenceNotFound),
		errors.Is(err, entity.ErrProjectNotFound),
		errors.Is(err, entity.ErrProjectMembershipNotFound),
		errors.Is(err, entity.ErrSavedViewNotFound),
		errors.Is(err, entity.ErrIssueCommentNotFound),
		errors.Is(err, entity.ErrAttachmentNotFound),
		errors.Is(err, entity.ErrBlobNotFound),
		errors.Is(err, entity.ErrTriageDisabled),
		errors.Is(err, entity.ErrNotificationNotFound),
		errors.Is(err, entity.ErrBreakGlassCodeInvalid):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrSnoozeNotInFuture),
		errors.Is(err, entity.ErrNotificationCursorInvalid),
		errors.Is(err, entity.ErrSearchQueryEmpty),
		errors.Is(err, entity.ErrSearchKindUnknown):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrSavedViewNotOwner),
		errors.Is(err, entity.ErrSavedViewNotShareable):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrProjectNotLead),
		errors.Is(err, entity.ErrIssueCommentNotAuthor),
		errors.Is(err, entity.ErrIssueCommentNotDeletable):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrAttachmentTooLarge):
		return storageRefusedProblem(api.AttachmentTooLarge, http.StatusRequestEntityTooLarge, err), true

	case errors.Is(err, entity.ErrStorageExhausted):
		return storageRefusedProblem(api.WorkspaceStorageExhausted, http.StatusConflict, err), true

	case errors.Is(err, entity.ErrAttachmentNotPending):
		return storageRefusedProblem(api.AttachmentNotPending, http.StatusConflict, err), true

	case errors.Is(err, entity.ErrAttachmentMissing):
		return storageRefusedProblem(api.AttachmentMissing, http.StatusConflict, err), true

	case errors.Is(err, entity.ErrIssueCommentDeleted):
		return commentConflictProblem(api.CommentConflictProblemCodeCommentDeleted, err), true

	case errors.Is(err, entity.ErrIssueCommentNotReplyable):
		return commentConflictProblem(api.CommentConflictProblemCodeCommentNotReplyable, err), true

	case errors.Is(err, entity.ErrProjectSlugTaken):
		return projectConflictProblem(api.ProjectConflictProblemCodeProjectSlugTaken, err), true

	case errors.Is(err, entity.ErrProjectArchived):
		return projectConflictProblem(api.ProjectConflictProblemCodeProjectArchived, err), true

	case errors.Is(err, entity.ErrProjectNotArchived):
		return projectConflictProblem(api.ProjectConflictProblemCodeProjectNotArchived, err), true

	case errors.Is(err, entity.ErrProjectNotFinished):
		return projectConflictProblem(api.ProjectConflictProblemCodeProjectNotFinished, err), true

	case errors.Is(err, entity.ErrProjectMemberExists):
		return projectConflictProblem(api.ProjectConflictProblemCodeProjectMemberExists, err), true

	case errors.Is(err, entity.ErrCycleClosed):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleClosed, err), true

	case errors.Is(err, entity.ErrCycleOverlaps):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleOverlaps, err), true

	case errors.Is(err, entity.ErrCycleTeamMismatch):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleTeamMismatch, err), true

	case errors.Is(err, entity.ErrCycleRolloverRequired):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleRolloverRequired, err), true

	case errors.Is(err, entity.ErrCycleNotEnded):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleNotEnded, err), true

	case errors.Is(err, entity.ErrCycleNoNextCycle):
		return cycleConflictProblem(api.CycleConflictProblemCodeCycleNoNextCycle, err), true

	case errors.Is(err, entity.ErrBreakGlassNotEnforcing):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrAccountForbidden),
		errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrAccountInvalidCredentials),
		errors.Is(err, entity.ErrSessionRevoked):
		return newProblem(http.StatusUnauthorized, err.Error()), true

	case errors.Is(err, entity.ErrSessionNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrAccountEmailTaken),
		errors.Is(err, entity.ErrWorkspaceSlugTaken),
		errors.Is(err, entity.ErrMembershipExists),
		errors.Is(err, entity.ErrEmailChangePending),
		errors.Is(err, entity.ErrEmailChangeAlreadyDone),
		errors.Is(err, entity.ErrAccountPasswordSet),
		errors.Is(err, entity.ErrAccountPasswordNotSet),
		errors.Is(err, entity.ErrAccountStatusTransition),
		errors.Is(err, entity.ErrAccountDeactivated),
		errors.Is(err, entity.ErrAccountDeleted):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrEmailChangeExpired),
		errors.Is(err, entity.ErrEmailChangeTokenInvalid),
		errors.Is(err, entity.ErrEmailChangeSameAddress):
		return newProblem(http.StatusBadRequest, err.Error()), true

	case errors.Is(err, entity.ErrAvatarTooLarge):
		return newProblem(http.StatusRequestEntityTooLarge, err.Error()), true

	case errors.Is(err, entity.ErrAvatarUnsupportedType):
		return newProblem(http.StatusUnsupportedMediaType, err.Error()), true

	case errors.Is(err, entity.ErrSignInRateLimited):
		base := baseProblem(http.StatusTooManyRequests, err.Error())

		return problemResponse{
			status: http.StatusTooManyRequests,
			body: api.RateLimitedProblem{
				Code:     api.RateLimited,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
			retryAfter: int(entity.SignInAddressCooldown.Seconds()),
		}, true

	case errors.Is(err, entity.ErrDirectoryUnlicensed):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.DirectoryUnlicensedProblem{
				Code:     api.DirectoryUnlicensedProblemCodeDirectoryUnlicensed,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrDirectoryNotConnected),
		errors.Is(err, entity.ErrDirectoryRunNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrMembershipDeactivated):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrAuditUnlicensed):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.AuditUnlicensedProblem{
				Code:     api.AuditUnlicensedProblemCodeAuditUnlicensed,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrAuditNotPermitted):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrSignInChallengeNotFound),
		errors.Is(err, entity.ErrSignInCodeExhausted):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrAuditCursorInvalid):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrSignUpUndeliverable):
		base := baseProblem(http.StatusServiceUnavailable, entity.ErrSignUpUndeliverable.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.SignUpUnavailableProblem{
				Code:     api.SignUpUnavailableProblemCodeSignUpUndeliverable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordBreachCheckUnavailable):
		base := baseProblem(http.StatusServiceUnavailable, entity.ErrPasswordBreachCheckUnavailable.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.BreachCheckUnavailableProblem{
				Code:     api.BreachCheckUnavailableProblemCodeBreachCheckUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordResetAlreadyUsed):
		base := baseProblem(http.StatusConflict, err.Error())

		return problemResponse{
			status: http.StatusConflict,
			body: api.ResetLinkUsedProblem{
				Code:     api.ResetLinkUsed,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrPasswordResetExpired),
		errors.Is(err, entity.ErrPasswordResetTokenInvalid),
		errors.Is(err, entity.ErrPasswordResetNotFound):
		base := baseProblem(http.StatusBadRequest, err.Error())

		return problemResponse{
			status: http.StatusBadRequest,
			body: api.ResetLinkExpiredProblem{
				Code:     api.ResetLinkExpired,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrWorkspacePasswordAuthDisabled),
		errors.Is(err, entity.ErrWorkspaceNotDeleted):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrMembershipSelfRoleChange):
		return membershipConflictProblem(api.MembershipConflictProblemCodeSelfRoleChange, err), true

	case errors.Is(err, entity.ErrMembershipDirectoryManaged):
		return membershipConflictProblem(api.MembershipConflictProblemCodeDirectoryManaged, err), true

	case errors.Is(err, entity.ErrTeamKeyTaken):
		return teamConflictProblem(api.TeamConflictProblemCodeTeamKeyTaken, err), true

	case errors.Is(err, entity.ErrTeamArchived):
		return teamConflictProblem(api.TeamConflictProblemCodeTeamArchived, err), true

	case errors.Is(err, entity.ErrTeamNotArchived):
		return teamConflictProblem(api.TeamConflictProblemCodeTeamNotArchived, err), true

	case errors.Is(err, entity.ErrTeamMembershipExists):
		return teamConflictProblem(api.TeamConflictProblemCodeTeamMemberExists, err), true

	case errors.Is(err, entity.ErrAPITokenNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrAPITokenNameTaken):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenNameTaken, err), true

	case errors.Is(err, entity.ErrAPITokenScopeInvalid):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenScopeInvalid, err), true

	case errors.Is(err, entity.ErrAPITokenScopeExceeds):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenScopeExceeds, err), true

	case errors.Is(err, entity.ErrAPITokenMintForbidden):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenMayNotMint, err), true

	case errors.Is(err, entity.ErrAPITokenGrantInvalid):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenGrantInvalid, err), true

	case errors.Is(err, entity.ErrAgentRateLimited):
		return problemResponse{
			status:     http.StatusTooManyRequests,
			body:       baseProblem(http.StatusTooManyRequests, err.Error()),
			retryAfter: int(entity.AgentActionWindow.Seconds()),
		}, true

	case errors.Is(err, entity.ErrAgentDisabled):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentDisabled, err), true

	case errors.Is(err, entity.ErrAgentActive):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentActive, err), true

	case errors.Is(err, entity.ErrAgentAuthorityMissing):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentAuthorityMissing, err), true

	case errors.Is(err, entity.ErrAgentNameTaken):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentNameTaken, err), true

	case errors.Is(err, entity.ErrAgentOwnerInvalid):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentOwnerInvalid, err), true

	case errors.Is(err, entity.ErrAgentProposalSettled):
		return agentUnusableProblem(api.AgentUnusableProblemCodeAgentProposalSettled, err), true

	case errors.Is(err, entity.ErrAgentNotFound), errors.Is(err, entity.ErrAgentProposalNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrAPITokenGrantMissing):
		return apiTokenUnusableProblem(api.APITokenUnusableProblemCodeTokenGrantMissing, err), true

	case errors.Is(err, entity.ErrWebhookNotFound),
		errors.Is(err, entity.ErrWebhookDeliveryNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrWebhookNotPermitted):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrWebhookCursorInvalid):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrWebhookDeliveryNotReplayable):
		return newProblem(http.StatusConflict, err.Error()), true

	case errors.Is(err, entity.ErrWebhookDestinationRefused):
		base := baseProblem(http.StatusUnprocessableEntity, err.Error())

		return problemResponse{
			status: http.StatusUnprocessableEntity,
			body: api.WebhookDestinationRefusedProblem{
				Code:     api.WebhookDestinationRefusedProblemCodeWebhookDestinationRefused,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrWebhookEncryptionKeyMissing):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.WebhookSigningUnavailableProblem{
				Code:     api.WebhookSigningUnavailableProblemCodeWebhookSigningUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrSCMConnectionNotFound),
		errors.Is(err, entity.ErrCodeLinkNotFound),
		errors.Is(err, entity.ErrIssueMirrorNotFound),
		errors.Is(err, entity.ErrSCMRepositoryNotFound),
		errors.Is(err, entity.ErrSCMRouteNotFound),
		errors.Is(err, entity.ErrSCMIdentityNotFound),
		errors.Is(err, entity.ErrSCMTransitionRuleNotFound),
		errors.Is(err, entity.ErrSCMAppNotFound),
		errors.Is(err, entity.ErrSCMInstallationNotFound),
		errors.Is(err, entity.ErrSCMAppStateNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrSCMIdentityExists):
		return sourceControlConflict(api.SourceControlIdentityMapped, err), true

	case errors.Is(err, entity.ErrSCMRouteExists):
		return sourceControlConflict(api.SourceControlAlreadyRouted, err), true

	case errors.Is(err, entity.ErrSCMRepositoryExists),
		errors.Is(err, entity.ErrSCMConnectionExists):
		return sourceControlConflict(api.SourceControlAlreadyConnected, err), true

	case errors.Is(err, entity.ErrIssueMirrorExists):
		return sourceControlConflict(api.SourceControlAlreadyMirrored, err), true

	case errors.Is(err, entity.ErrSCMTeamOutsideConnection):
		return sourceControlConflict(api.SourceControlTeamOutsideConnection, err), true

	case errors.Is(err, entity.ErrSCMProviderUnsupported),
		errors.Is(err, entity.ErrSCMAppUnsupported):
		return sourceControlRefused(api.SourceControlProviderUnsupported, err), true

	case errors.Is(err, entity.ErrSCMAppExists):
		return sourceControlConflict(api.SourceControlAlreadyConnected, err), true

	case errors.Is(err, entity.ErrSCMAppTokenUnsupported):
		return sourceControlRefused(api.SourceControlProviderUnsupported, err), true

	case errors.Is(err, entity.ErrSCMAppRefused),
		errors.Is(err, entity.ErrSCMAppTokenUnavailable),
		errors.Is(err, entity.ErrSCMPrivateKeyInvalid):
		return sourceControlRefused(api.SourceControlCredentialsRejected, err), true

	case errors.Is(err, entity.ErrSCMEncryptionKeyMissing):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.SourceControlSealingUnavailableProblem{
				Code:     api.SourceControlSealingUnavailableProblemCodeSourceControlSealingUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrImportRunNotFound),
		errors.Is(err, entity.ErrImportSourceUnknown):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrImportStatusTransition):
		return importConflictProblem(api.ImportStatusTransition, err), true

	case errors.Is(err, entity.ErrImportRunLeased):
		return importConflictProblem(api.ImportRunLeased, err), true

	case errors.Is(err, entity.ErrImportNotRevertible):
		return importConflictProblem(api.ImportNotRevertible, err), true

	case errors.Is(err, entity.ErrImportPreviewStale):
		return importConflictProblem(api.ImportPreviewStale, err), true

	case errors.Is(err, entity.ErrImportUnmapped),
		errors.Is(err, entity.ErrImportPreviewRequired),
		errors.Is(err, entity.ErrImportMappingNotMember),
		errors.Is(err, entity.ErrImportMappingUnknownKind),
		errors.Is(err, entity.ErrImportCursorInvalid):
		return newProblem(http.StatusUnprocessableEntity, err.Error()), true

	case errors.Is(err, entity.ErrImportTeamMappingRequired):
		return newProblem(http.StatusForbidden, err.Error()), true

	case errors.Is(err, entity.ErrImportEncryptionKeyMissing):
		base := baseProblem(http.StatusServiceUnavailable, err.Error())

		return problemResponse{
			status: http.StatusServiceUnavailable,
			body: api.ImportSigningUnavailableProblem{
				Code:     api.ImportSigningUnavailableProblemCodeImportSigningUnavailable,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrLabelNameTaken):
		return labelConflictProblem(api.LabelNameTaken, err), true

	case errors.Is(err, entity.ErrLabelGroupNameTaken):
		return labelConflictProblem(api.LabelGroupNameTaken, err), true

	case errors.Is(err, entity.ErrLabelGroupExclusive):
		return labelConflictProblem(api.LabelGroupExclusive, err), true

	case errors.Is(err, entity.ErrLabelOutOfScope):
		return issueConflictProblem(api.IssueConflictProblemCodeLabelOutOfScope, err), true

	case errors.Is(err, entity.ErrLabelGroupInUse):
		return labelConflictProblem(api.LabelGroupInUse, err), true

	case errors.Is(err, entity.ErrLabelMergeScopeNarrows):
		return labelConflictProblem(api.LabelMergeScopeNarrows, err), true

	case errors.Is(err, entity.ErrLabelMergeGroupMismatch):
		return labelConflictProblem(api.LabelMergeGroupMismatch, err), true

	case errors.Is(err, entity.ErrLabelNotFound),
		errors.Is(err, entity.ErrLabelGroupNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true

	case errors.Is(err, entity.ErrWorkflowStateNameTaken):
		return workflowStateConflictProblem(api.StateNameTaken, err), true

	case errors.Is(err, entity.ErrWorkflowStateLastInCategory):
		return workflowStateConflictProblem(api.StateLastInCategory, err), true

	case errors.Is(err, entity.ErrWorkflowStateIsDefault):
		return workflowStateConflictProblem(api.StateIsDefault, err), true

	case errors.Is(err, entity.ErrWorkflowStateIsCompletion):
		return workflowStateConflictProblem(api.StateIsCompletion, err), true

	case errors.Is(err, entity.ErrSignUpsClosed):
		base := baseProblem(http.StatusForbidden, err.Error())

		return problemResponse{
			status: http.StatusForbidden,
			body: api.SignUpClosedProblem{
				Code:     api.SignUpClosed,
				Detail:   base.Detail,
				Instance: base.Instance,
				Status:   base.Status,
				Title:    base.Title,
				Type:     base.Type,
			},
		}, true

	case errors.Is(err, entity.ErrSignUpExpired):
		return signUpExpiredProblem(api.SignUpExpired, err), true

	case errors.Is(err, entity.ErrSignUpTokenInvalid),
		errors.Is(err, entity.ErrSignUpNotFound):
		return signUpExpiredProblem(api.SignUpInvalid, err), true

	case errors.Is(err, entity.ErrSignUpAlreadyConfirmed):
		return signUpUnusableProblem(api.SignUpUsed, err), true

	case errors.Is(err, entity.ErrInvitationExpired):
		return invitationExpiredProblem(api.InvitationExpired, err), true

	case errors.Is(err, entity.ErrInvitationTokenInvalid),
		errors.Is(err, entity.ErrInvitationNotFound):
		return invitationExpiredProblem(api.InvitationInvalid, err), true

	case errors.Is(err, entity.ErrInvitationRevoked):
		return invitationUnusableProblem(api.InvitationUnusableProblemCodeInvitationRevoked, err), true

	case errors.Is(err, entity.ErrInvitationAccepted):
		return invitationUnusableProblem(api.InvitationUnusableProblemCodeInvitationAccepted, err), true

	case errors.Is(err, entity.ErrInvitationAddressMismatch):
		return invitationUnusableProblem(api.InvitationUnusableProblemCodeInvitationAddressMismatch, err), true

	default:
		return problemResponse{}, false
	}
}

func membershipConflictProblem(code api.MembershipConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.MembershipConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func teamConflictProblem(code api.TeamConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.TeamConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func importConflictProblem(code api.ImportConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.ImportConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func agentUnusableProblem(code api.AgentUnusableProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.AgentUnusableProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func agentHeldProblem(held entity.AgentActionHeldError) problemResponse {
	base := baseProblem(http.StatusAccepted, held.Error())

	return problemResponse{
		status: http.StatusAccepted,
		body: api.AgentHeldProblem{
			Code:       api.AgentActionHeld,
			ProposalId: held.ProposalID,
			Detail:     base.Detail,
			Instance:   base.Instance,
			Status:     base.Status,
			Title:      base.Title,
			Type:       base.Type,
		},
	}
}

func apiTokenUnusableProblem(code api.APITokenUnusableProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.APITokenUnusableProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func labelConflictProblem(code api.LabelConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.LabelConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func workflowStateConflictProblem(code api.WorkflowStateConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.WorkflowStateConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func signUpExpiredProblem(code api.SignUpExpiredProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusBadRequest, err.Error())

	return problemResponse{
		status: http.StatusBadRequest,
		body: api.SignUpExpiredProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func signUpUnusableProblem(code api.SignUpUnusableProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.SignUpUnusableProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func invitationExpiredProblem(code api.InvitationExpiredProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusBadRequest, err.Error())

	return problemResponse{
		status: http.StatusBadRequest,
		body: api.InvitationExpiredProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func invitationUnusableProblem(code api.InvitationUnusableProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.InvitationUnusableProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func unauthorized() problemResponse {
	return newProblem(http.StatusUnauthorized, "a valid session is required")
}

func (r problemResponse) VisitGetInstanceLicenceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceDirectoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConnectWorkspaceDirectoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRotateWorkspaceDirectoryTokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfigureWorkspaceDirectoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisconnectWorkspaceDirectoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceDirectoryAvailabilityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceDirectoryRunsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceDirectoryChangesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceAuditResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListInstanceAuditResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceAuditAvailabilityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceMemberAuditAccessResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceAgentsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListGrantableAgentScopesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRegisterWorkspaceAgentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceAgentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisableWorkspaceAgentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitEnableWorkspaceAgentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRotateWorkspaceAgentCredentialResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceAgentActivityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetTeamAgentSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamAgentSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceAgentProposalsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitApproveWorkspaceAgentProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRejectWorkspaceAgentProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListAPITokensResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceWebhooksResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceWebhookResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceWebhookResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceWebhookResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteWorkspaceWebhookResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRotateWorkspaceWebhookSecretResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSendWorkspaceWebhookTestResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceWebhookDeliveriesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceWebhookDeliveryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReplayWorkspaceWebhookDeliveryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMintAPITokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevokeAPITokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceAPITokensResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevokeWorkspaceAPITokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkflowStatesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMoveWorkspaceIssueToTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceIssueStatusResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueByReferenceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceIssueParentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueChildrenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueQuestionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAskWorkspaceIssueQuestionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAnswerWorkspaceIssueQuestionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueDelegationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDelegateWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueDelegationTargetsResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}

func (r problemResponse) VisitRecallWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueRelationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceIssueRelationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceIssueRelationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceLabelsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceLabelResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceLabelResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceLabelResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceLabelUsageResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMergeWorkspaceLabelResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceLabelGroupsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceLabelGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRenameWorkspaceLabelGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceLabelGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceIssueLabelsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkflowStateResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkflowStateResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReorderWorkflowStatesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetDefaultWorkflowStateResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetCompletionWorkflowStateResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkflowStateResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMoveWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueActivityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueProgressResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssuesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitQueryWorkspaceIssuesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceSavedViewsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceTriageResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAcceptWorkspaceTriageIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeclineWorkspaceTriageIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMergeWorkspaceTriageIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReassignWorkspaceTriageIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetTeamTriageSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamTriageSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteTeamTriageSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceSavedViewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceSavedViewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceSavedViewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceSavedViewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReorderWorkspaceSavedViewsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceProjectsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitArchiveWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnarchiveWorkspaceProjectResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceProjectMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceProjectMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceProjectMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceProjectStatusResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPostWorkspaceProjectStatusResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceCyclesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListCurrentWorkspaceCyclesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceCycleResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceCycleScopeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCloseWorkspaceCycleResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetTeamCycleCadenceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamCycleCadenceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteTeamCycleCadenceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRequestSignUpResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitConfirmSignUpResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitSignInResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitVerifySignInCodeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSignOutResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRequestPasswordResetResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfirmPasswordResetResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeactivateCurrentAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetPasswordResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitChangePasswordResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitGetPendingEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRequestEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfirmEmailChangeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadAvatarResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRemoveAvatarResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitListWorkspacesResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitChangeWorkspaceMemberRoleResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListSignedInAccountsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSignOutEveryAccountResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListSessionsResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRevokeSessionResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitRevokeAllSessionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceResponse(w http.ResponseWriter) error { return r.write(w) }

func (r problemResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDeleteWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceInvitationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceInvitationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevokeWorkspaceInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitResendWorkspaceInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPreviewInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceAuthPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPreviewWorkspaceMemberRemovalResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceTeamsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitArchiveWorkspaceTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnarchiveWorkspaceTeamResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceTeamMembersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceTeamMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceTeamMemberResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceAuthPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func projectConflictProblem(code api.ProjectConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.ProjectConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func cycleConflictProblem(code api.CycleConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.CycleConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func commentConflictProblem(code api.CommentConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.CommentConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func storageRefusedProblem(code api.StorageRefusedProblemCode, status int, err error) problemResponse {
	base := baseProblem(status, err.Error())
	body := api.StorageRefusedProblem{
		Code:     code,
		Detail:   base.Detail,
		Instance: base.Instance,
		Status:   base.Status,
		Title:    base.Title,
		Type:     base.Type,
	}

	var oversized entity.AttachmentTooLargeError
	if errors.As(err, &oversized) {
		body.ByteSize = &oversized.SizeBytes
		body.MaxBytes = &oversized.MaxBytes
	}

	var exhausted entity.StorageExhaustedError
	if errors.As(err, &exhausted) {
		body.ByteSize = &exhausted.SizeBytes
		body.StoredBytes = &exhausted.StoredBytes
		body.MaxBytes = &exhausted.MaxBytes
	}

	return problemResponse{status: status, body: body}
}

func issueConflictProblem(code api.IssueConflictProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.IssueConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func codebaseConflictProblem(code api.CodebaseProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.CodebaseProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func previewConflictProblem(code api.PreviewProblemCode, err error) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.PreviewProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func executionConflictProblem(code api.ExecutionProblemCode, err error) problemResponse {
	return executionProblem(http.StatusConflict, code, err)
}

func executionProblem(status int, code api.ExecutionProblemCode, err error) problemResponse {
	base := baseProblem(status, err.Error())

	return problemResponse{
		status: status,
		body: api.ExecutionProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func runnerProblem(code api.RunnerProblemCode, err error) problemResponse {
	return runnerProblemAt(http.StatusUnauthorized, code, err)
}

func runnerConflictProblem(code api.RunnerProblemCode, err error) problemResponse {
	return runnerProblemAt(http.StatusConflict, code, err)
}

func runnerProblemAt(status int, code api.RunnerProblemCode, err error) problemResponse {
	base := baseProblem(status, err.Error())

	return problemResponse{
		status: status,
		body: api.RunnerProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func (r problemResponse) VisitEnrolRunnerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitExchangeRunnerTokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetCurrentRunnerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceRunnersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPauseWorkspaceRunnerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitResumeWorkspaceRunnerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevokeWorkspaceRunnerResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConnectCodebaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListCurrentRunnerCodebasesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfirmCodebaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisconnectCodebaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisconnectAgentCodebaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListAgentCodebasesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueCommentsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPostWorkspaceIssueCommentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitEditWorkspaceIssueCommentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceIssueCommentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReactToWorkspaceIssueCommentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnreactToWorkspaceIssueCommentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueAttachmentsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReserveWorkspaceIssueAttachmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitFinalizeWorkspaceIssueAttachmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceIssueAttachmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDownloadWorkspaceAttachmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDownloadAccountAvatarResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceStorageResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceProjectActivityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceNotificationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReadAllWorkspaceNotificationsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReadWorkspaceNotificationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSnoozeWorkspaceNotificationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueFollowResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueFollowersResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceIssueFollowResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceNotificationSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceNotificationSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceTeamNotificationSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceTeamNotificationSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitClearWorkspaceTeamNotificationSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSearchWorkspaceResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListImportSourcesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceImportsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCreateWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConfigureWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadWorkspaceImportFileResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceImportCatalogueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitStageWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceImportMappingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDecideWorkspaceImportMappingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitPreviewWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitExecuteWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevertWorkspaceImportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceImportReportResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func sourceControlRefused(
	code api.SourceControlRefusedProblemCode,
	err error,
) problemResponse {
	base := baseProblem(http.StatusUnprocessableEntity, err.Error())

	return problemResponse{
		status: http.StatusUnprocessableEntity,
		body: api.SourceControlRefusedProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func sourceControlConflict(
	code api.SourceControlConflictProblemCode,
	err error,
) problemResponse {
	base := baseProblem(http.StatusConflict, err.Error())

	return problemResponse{
		status: http.StatusConflict,
		body: api.SourceControlConflictProblem{
			Code:     code,
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func (r problemResponse) VisitListWorkspaceIssueExecutionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueChangeSetResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionTimelineResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitCancelWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRestartWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitResumeWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitApproveWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadExecutionLogsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadExecutionTranscriptResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUploadExecutionArtifactResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetExecutionStreamsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionLogsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionTranscriptResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionArtifactsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDownloadWorkspaceExecutionArtifactResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceExecutionPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetWorkspaceExecutionPolicyResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionServicesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionPreviewsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSharePreviewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRevokePreviewShareLinkResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRetainWorkspaceExecutionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAuthorizePreviewResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDismissWorkspaceIssueQuestionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceExecutionQuestionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
