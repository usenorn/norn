package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type QuestionExpirySweepHandler struct {
	questions service.IssueQuestions
}

func NewQuestionExpirySweepHandler(questions service.IssueQuestions) *QuestionExpirySweepHandler {
	return &QuestionExpirySweepHandler{questions: questions}
}

func (h *QuestionExpirySweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.questions.SweepExpired(ctx)
}
