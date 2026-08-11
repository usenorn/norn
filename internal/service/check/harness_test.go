package check_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	checkrepo "github.com/usenorn/norn/internal/repository/check"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	delegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	checksvc "github.com/usenorn/norn/internal/service/check"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
)

type harness struct {
	checks      *checkrepo.MockCheck
	evidence    *checkrepo.MockCheckEvidence
	issues      *issuerepo.MockIssue
	delegations *delegationrepo.MockIssueDelegation
	codeLinks   *scmrepo.MockCodeLink
	activity    *activityrepo.MockActivity
	issueWriter *issuesvc.MockIssues
	authorizer  *authorizersvc.MockAuthorizer
	service     service.Checks

	workspaceID uuid.UUID
	actorID     uuid.UUID
	actorKind   entity.ActorKind
}

func newHarness(t *testing.T, kind entity.ActorKind) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		checks:      checkrepo.NewMockCheck(ctrl),
		evidence:    checkrepo.NewMockCheckEvidence(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		delegations: delegationrepo.NewMockIssueDelegation(ctrl),
		codeLinks:   scmrepo.NewMockCodeLink(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		issueWriter: issuesvc.NewMockIssues(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		actorID:     uuid.New(),
		actorKind:   kind,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: h.actorKind, AccountID: h.actorID},
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.service = checksvc.New(
		h.checks,
		h.evidence,
		h.issues,
		h.delegations,
		h.codeLinks,
		h.activity,
		h.authorizer,
		h.issueWriter,
		transactor,
	)

	return h
}

func (h *harness) issue() entity.Issue {
	return entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		TeamID:      uuid.New(),
		Version:     2,
		State:       entity.IssueState{ID: uuid.New(), Category: entity.StateCategoryActive},
	}
}

func (h *harness) expectIssue(issue entity.Issue) {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, issue.ID, gomock.Any()).
		Return(issue, nil).
		AnyTimes()
}

func (h *harness) expectCheck(check entity.Check) {
	h.checks.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, check.ID).
		Return(check, nil).
		AnyTimes()
}

func (h *harness) expectNoDelegation() {
	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound).
		AnyTimes()
}

func (h *harness) expectNoCodeLinks() {
	h.codeLinks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(nil, nil).
		AnyTimes()
}

func (h *harness) check(issue entity.Issue) entity.Check {
	return entity.Check{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		IssueID:     issue.ID,
		Statement:   "payments retry without duplicating a charge",
		Method:      entity.CheckMethodCommand,
		Proof:       "go test ./internal/payments/...",
		Approval:    entity.CheckApprovalApproved,
		Resolution:  entity.CheckResolutionNone,
	}
}
