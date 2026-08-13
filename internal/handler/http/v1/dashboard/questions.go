package dashboard

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueQuestions(
	ctx context.Context,
	request api.ListWorkspaceIssueQuestionsRequestObject,
) (api.ListWorkspaceIssueQuestionsResponseObject, error) {
	questions, err := h.questions.List(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueQuestions200JSONResponse{
		Questions: issueQuestionDTOs(questions),
	}, nil
}

func (h *handler) AskWorkspaceIssueQuestion(
	ctx context.Context,
	request api.AskWorkspaceIssueQuestionRequestObject,
) (api.AskWorkspaceIssueQuestionResponseObject, error) {
	asked, err := h.questions.Ask(ctx, request.WorkspaceId, request.IssueId, service.AskQuestionInput{
		Question: request.Body.Question,
		Default:  request.Body.Default,
		Options:  questionOptions(request.Body.Options),
		Wait:     questionWait(request.Body.WaitSeconds),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AskWorkspaceIssueQuestion201JSONResponse(issueQuestionDTO(asked)), nil
}

func (h *handler) AnswerWorkspaceIssueQuestion(
	ctx context.Context,
	request api.AnswerWorkspaceIssueQuestionRequestObject,
) (api.AnswerWorkspaceIssueQuestionResponseObject, error) {
	answered, err := h.questions.Answer(
		ctx, request.WorkspaceId, request.IssueId, request.QuestionId,
		service.AnswerQuestionInput{Answer: request.Body.Answer},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AnswerWorkspaceIssueQuestion200JSONResponse(issueQuestionDTO(answered)), nil
}

func questionOptions(options *[]string) []string {
	if options == nil {
		return nil
	}

	return *options
}

func questionWait(seconds *int32) time.Duration {
	if seconds == nil || *seconds == 0 {
		return entity.QuestionWaitDefault
	}

	return time.Duration(*seconds) * time.Second
}
