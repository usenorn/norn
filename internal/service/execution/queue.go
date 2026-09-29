package execution

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *executionsService) Queue(ctx context.Context, workspaceID uuid.UUID) (service.ReviewQueue, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.ReviewQueue{}, err
	}

	questions, err := s.questions.ListWaiting(ctx, decision.Scope, entity.QuestionWaitingMax)
	if err != nil {
		return service.ReviewQueue{}, err
	}

	runs, err := s.executions.ListVisible(ctx, decision.Scope, entity.ExecutionPage{
		States: []entity.ExecutionState{entity.ExecutionAwaitingPlan, entity.ExecutionAwaitingReview},
		Limit:  entity.ExecutionListMaxSize,
	})
	if err != nil {
		return service.ReviewQueue{}, err
	}

	issues := make([]uuid.UUID, 0, len(questions)+len(runs))
	for _, question := range questions {
		issues = append(issues, question.IssueID)
	}

	for _, run := range runs {
		issues = append(issues, run.Execution.IssueID)
	}

	authorities, err := s.delegates.Authorities(ctx, workspaceID, issues)
	if err != nil {
		return service.ReviewQueue{}, err
	}

	right := func(issueID uuid.UUID) service.DecisionRight {
		authority := authorities[issueID]

		return service.DecisionRight{CanDecide: authority.Permits(decision), MakerName: authority.MakerName()}
	}

	queue := service.ReviewQueue{
		Questions: make([]service.ReviewQuestion, 0, len(questions)),
		Plans:     make([]service.ReviewRun, 0),
		Changes:   make([]service.ReviewRun, 0),
	}

	for _, question := range questions {
		queue.Questions = append(queue.Questions, service.ReviewQuestion{Question: question, Right: right(question.IssueID)})
	}

	for _, run := range runs {
		waiting := service.ReviewRun{Listing: run, Right: right(run.Execution.IssueID)}

		if run.Execution.State == entity.ExecutionAwaitingPlan {
			queue.Plans = append(queue.Plans, waiting)
		} else {
			queue.Changes = append(queue.Changes, waiting)
		}
	}

	return queue, nil
}

func (s *executionsService) DecisionRight(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (service.DecisionRight, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.DecisionRight{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return service.DecisionRight{}, err
	}

	authority, err := s.delegates.Authority(ctx, workspaceID, issueID)
	if err != nil {
		return service.DecisionRight{}, err
	}

	return service.DecisionRight{CanDecide: authority.Permits(decision), MakerName: authority.MakerName()}, nil
}
