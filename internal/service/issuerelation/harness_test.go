package issuerelation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	relationrepo "github.com/usenorn/norn/internal/repository/issuerelation"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	relationsvc "github.com/usenorn/norn/internal/service/issuerelation"
)

type harness struct {
	relations  *relationrepo.MockIssueRelation
	issues     *issuerepo.MockIssue
	states     *workflowstaterepo.MockWorkflowState
	activity   *activityrepo.MockActivity
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.IssueRelations
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		relations:  relationrepo.NewMockIssueRelation(ctrl),
		issues:     issuerepo.NewMockIssue(ctrl),
		states:     workflowstaterepo.NewMockWorkflowState(ctrl),
		activity:   activityrepo.NewMockActivity(ctrl),
		authorizer: authorizersvc.NewMockAuthorizer(ctrl),
		transactor: transactorrepo.NewMockTransactor(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = relationsvc.New(
		h.relations, h.issues, h.states, h.activity, h.authorizer, h.transactor,
	)

	return h
}

func (h *harness) expectScope(workspaceID uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil)
}

func (h *harness) expectNoRelationHeld() {
	h.relations.EXPECT().
		FindPair(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.IssueRelation{}, entity.ErrIssueRelationNotFound).
		AnyTimes()
}

func issue(reference string, number int) entity.Issue {
	return entity.Issue{
		ID:           uuid.New(),
		ReferenceKey: reference,
		Number:       number,
		Version:      1,
		State:        entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
}
