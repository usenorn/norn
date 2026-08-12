package check_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func draft() service.NewCheckInput {
	return service.NewCheckInput{
		Statement: "payments retry without duplicating a charge",
		Method:    entity.CheckMethodCommand,
		Proof:     "go test ./internal/payments/...",
	}
}

func TestAChecksAPersonWritesCountsImmediately(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	h.expectIssue(issue)
	h.expectNoDelegation()
	h.checks.EXPECT().ListByIssue(gomock.Any(), h.workspaceID, issue.ID).Return(nil, nil)

	var written entity.Check

	h.checks.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, check entity.Check) (entity.Check, error) {
			written = check

			return check, nil
		})

	if _, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{draft()},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if written.Approval != entity.CheckApprovalApproved {
		t.Fatalf("a person's check arrived %q, want it approved on arrival", written.Approval)
	}

	if written.ApprovedAt == nil || written.ApprovedByAccountID != h.actorID {
		t.Fatal("a person's own check does not record who approved it")
	}
}

func TestAChecksAnAgentProposesWaitsForAPerson(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()

	h.expectIssue(issue)
	h.expectNoDelegation()
	h.checks.EXPECT().ListByIssue(gomock.Any(), h.workspaceID, issue.ID).Return(nil, nil)

	var written entity.Check

	h.checks.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, check entity.Check) (entity.Check, error) {
			written = check

			return check, nil
		})

	if _, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{draft()},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if written.Approval != entity.CheckApprovalPending {
		t.Fatalf("an agent's check arrived %q, want it waiting for a person", written.Approval)
	}

	if written.ApprovedAt != nil {
		t.Fatal("an agent's check recorded an approval nobody gave")
	}
}

func TestACheckWrittenAfterTheWorkWasHandedOverIsMarkedAsSuch(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()

	h.expectIssue(issue)
	h.checks.EXPECT().ListByIssue(gomock.Any(), h.workspaceID, issue.ID).Return(nil, nil)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentName: "opsy"}, nil).
		AnyTimes()

	var written entity.Check

	h.checks.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, check entity.Check) (entity.Check, error) {
			written = check

			return check, nil
		})

	if _, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{draft()},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if !written.AddedAfterDelegation {
		t.Fatal("a check written while an agent held the issue is not marked as a late addition")
	}
}

func TestACheckMustSayHowItWillBeProven(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	blank := draft()
	blank.Proof = "   "

	_, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{blank},
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a check with no proof returned %v, want a validation error", err)
	}
}

func TestAnIssueCannotCarryMoreChecksThanItMay(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	held := make([]entity.Check, entity.ChecksPerIssueMax)

	h.expectIssue(issue)
	h.expectNoDelegation()
	h.checks.EXPECT().ListByIssue(gomock.Any(), h.workspaceID, issue.ID).Return(held, nil)

	_, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{draft()},
	})

	if !errors.Is(err, entity.ErrCheckLimitReached) {
		t.Fatalf("adding past the limit returned %v, want %v", err, entity.ErrCheckLimitReached)
	}
}

func TestAnAgentCannotWaiveItsOwnCheck(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	_, err := h.service.Waive(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.WaiveCheckInput{Reason: "the sandbox cannot reach the payment provider"},
	)

	if !errors.Is(err, entity.ErrCheckWaiverNotPersonal) {
		t.Fatalf("an agent waiving returned %v, want %v", err, entity.ErrCheckWaiverNotPersonal)
	}
}

func TestATokenCannotWaiveACheckEither(t *testing.T) {
	h := newHarness(t, entity.ActorKindToken)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	_, err := h.service.Waive(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.WaiveCheckInput{Reason: "not reachable from here"},
	)

	if !errors.Is(err, entity.ErrCheckWaiverNotPersonal) {
		t.Fatalf("a token waiving returned %v, want %v", err, entity.ErrCheckWaiverNotPersonal)
	}
}

func TestWaivingACheckNeedsAReason(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()
	check := h.check(issue)

	_, err := h.service.Waive(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.WaiveCheckInput{Reason: "  "},
	)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("waiving with no reason returned %v, want a validation error", err)
	}
}

func TestAnAgentCannotApproveTheCriteriaItProposed(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	_, err := h.service.Decide(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.DecideCheckInput{Approval: entity.CheckApprovalApproved},
	)

	if !errors.Is(err, entity.ErrCheckDecisionNotPersonal) {
		t.Fatalf("an agent approving returned %v, want %v", err, entity.ErrCheckDecisionNotPersonal)
	}
}

func TestACheckIsDecidedOnlyOnce(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	_, err := h.service.Decide(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.DecideCheckInput{Approval: entity.CheckApprovalDeclined},
	)

	if !errors.Is(err, entity.ErrCheckDecided) {
		t.Fatalf("deciding a decided check returned %v, want %v", err, entity.ErrCheckDecided)
	}
}

func TestACheckBelongingToAnotherIssueIsNotFoundHere(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	elsewhere := h.check(h.issue())

	h.expectIssue(issue)
	h.expectCheck(elsewhere)

	err := h.service.Remove(context.Background(), h.workspaceID, issue.ID, elsewhere.ID)

	if !errors.Is(err, entity.ErrCheckNotFound) {
		t.Fatalf("removing another issue's check returned %v, want %v", err, entity.ErrCheckNotFound)
	}
}

func TestDeclaringAGapFilesAChildIssueUnderTheParent(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	child := h.issue()

	var created service.CreateIssueInput

	h.issueWriter.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
			created = input

			return child, nil
		})

	var parented service.SetIssueParentInput

	h.issueWriter.EXPECT().
		SetParent(gomock.Any(), h.workspaceID, child.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ any, input service.SetIssueParentInput,
		) (entity.Issue, error) {
			parented = input

			return child, nil
		})

	var resolved repository.CheckResolutionInput

	h.checks.EXPECT().
		Resolve(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ any, input repository.CheckResolutionInput,
		) (entity.Check, error) {
			resolved = input

			return check, nil
		})

	declared, err := h.service.DeclareGap(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.DeclareGapInput{Reason: "the provider has no sandbox for duplicate charges"},
	)
	if err != nil {
		t.Fatalf("declare gap: %v", err)
	}

	if created.TeamID != issue.TeamID {
		t.Fatalf("the gap issue landed on team %s, want the parent's team %s", created.TeamID, issue.TeamID)
	}

	if !strings.Contains(created.Title, check.Statement) {
		t.Fatalf("the gap issue is titled %q, want it to name the criterion", created.Title)
	}

	if parented.ParentID == nil || *parented.ParentID != issue.ID {
		t.Fatal("the gap issue was not filed under the issue that declared it")
	}

	if resolved.Resolution != entity.CheckResolutionGap || resolved.GapIssueID != child.ID {
		t.Fatal("the check does not point at the child issue the gap filed")
	}

	if declared.Child.ID != child.ID {
		t.Fatal("the declared gap does not carry the child issue back")
	}
}

func TestDeclaringAGapNeedsAReason(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	_, err := h.service.DeclareGap(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.DeclareGapInput{Reason: ""},
	)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a gap with no reason returned %v, want a validation error", err)
	}
}

func TestASettledCheckTakesNoFurtherDecision(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	check := h.check(issue)
	check.Resolution = entity.CheckResolutionWaived

	h.expectIssue(issue)
	h.expectCheck(check)

	_, err := h.service.Waive(
		context.Background(), h.workspaceID, issue.ID, check.ID,
		service.WaiveCheckInput{Reason: "again"},
	)

	if !errors.Is(err, entity.ErrCheckSettled) {
		t.Fatalf("waiving a settled check returned %v, want %v", err, entity.ErrCheckSettled)
	}
}

func TestATimeLimitOutsideWhatNornWillHoldIsRefused(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	brief := time.Minute
	drafted := draft()
	drafted.TimeLimit = &brief

	_, err := h.service.Add(context.Background(), h.workspaceID, issue.ID, service.AddChecksInput{
		Checks: []service.NewCheckInput{drafted},
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a one-minute time limit returned %v, want a validation error", err)
	}
}

func TestAnAgentCannotRemoveTheCriterionItIsGradedAgainst(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	err := h.service.Remove(context.Background(), h.workspaceID, issue.ID, check.ID)

	if !errors.Is(err, entity.ErrCheckRemovalNotPersonal) {
		t.Fatalf("an agent removing returned %v, want %v", err, entity.ErrCheckRemovalNotPersonal)
	}
}
