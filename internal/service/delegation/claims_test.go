package delegation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestADelegationQueueCarriesOnlyTheWorkHandedToTheCallingAgent(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	issue := h.issue()
	h.expectIssue(issue)

	h.delegations.EXPECT().
		ListOpenByAgent(gomock.Any(), h.workspaceID, agent.ID).
		Return([]entity.IssueDelegation{{IssueID: issue.ID, AgentID: agent.ID, Brief: "ship it"}}, nil)

	queue, err := h.service.Queue(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("read the queue: %v", err)
	}

	if len(queue) != 1 {
		t.Fatalf("queue carried %d entries, want 1", len(queue))
	}

	if queue[0].Issue.ID != issue.ID {
		t.Error("the queue entry did not carry the issue it was delegated on")
	}

	if queue[0].Delegation.Brief != "ship it" {
		t.Error(
			"the queue dropped the brief; it is the only place the person's instructions live, " +
				"and a runner that cannot read it works blind",
		)
	}
}

func TestTheQueueLeadsWithTheWorkTheTrackerRanksFirst(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	low := h.issue()
	low.Priority = entity.IssuePriorityLow

	urgent := h.issue()
	urgent.Priority = entity.IssuePriorityUrgent

	medium := h.issue()
	medium.Priority = entity.IssuePriorityMedium

	for _, issue := range []entity.Issue{low, urgent, medium} {
		h.expectIssue(issue)
	}

	h.delegations.EXPECT().
		ListOpenByAgent(gomock.Any(), h.workspaceID, agent.ID).
		Return([]entity.IssueDelegation{
			{IssueID: low.ID, AgentID: agent.ID},
			{IssueID: urgent.ID, AgentID: agent.ID},
			{IssueID: medium.ID, AgentID: agent.ID},
		}, nil)

	queue, err := h.service.Queue(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("read the queue: %v", err)
	}

	want := []entity.IssuePriority{
		entity.IssuePriorityUrgent, entity.IssuePriorityMedium, entity.IssuePriorityLow,
	}

	for i, priority := range want {
		if queue[i].Issue.Priority != priority {
			t.Fatalf(
				"queue position %d held %s, want %s; a runner works this list top down and never "+
					"chooses for itself, so this order is the whole of its judgment",
				i, queue[i].Issue.Priority, priority,
			)
		}
	}
}

func TestAnArchivedIssueLeavesTheQueueEvenWhileItsDelegationStands(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	archived := h.issue()
	archived.Status = entity.IssueStatusArchived
	h.expectIssue(archived)

	h.delegations.EXPECT().
		ListOpenByAgent(gomock.Any(), h.workspaceID, agent.ID).
		Return([]entity.IssueDelegation{{IssueID: archived.ID, AgentID: agent.ID}}, nil)

	queue, err := h.service.Queue(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("read the queue: %v", err)
	}

	if len(queue) != 0 {
		t.Fatal(
			"an archived issue stayed in the queue; the runner would start work somebody had " +
				"already taken off the board",
		)
	}
}

func TestOnlyAnAgentHasADelegationQueue(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.Queue(context.Background(), h.workspaceID); !errors.Is(
		err, entity.ErrDelegationQueueNotAgent,
	) {
		t.Fatalf("error = %v, want %v", err, entity.ErrDelegationQueueNotAgent)
	}
}

func TestClaimingWorkDelegatedToAnotherAgentIsRefused(t *testing.T) {
	h := newHarness(t)
	h.asAgent(uuid.New())

	issue := h.issue()
	h.expectIssue(issue)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentID: uuid.New()}, nil)

	_, err := h.service.Claim(context.Background(), h.workspaceID, issue.ID, service.ClaimDelegationInput{
		Runner: "laptop-a",
		TTL:    entity.DelegationClaimTTLDefault,
	})

	if !errors.Is(err, entity.ErrIssueDelegationNotYours) {
		t.Fatalf("error = %v, want %v", err, entity.ErrIssueDelegationNotYours)
	}
}

func TestASecondRunnerCannotTakeAnIssueAnotherRunnerAlreadyHolds(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	issue := h.issue()
	h.expectIssue(issue)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentID: agent.ID}, nil)

	h.delegations.EXPECT().
		Claim(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(entity.IssueDelegation{}, entity.ErrDelegationClaimHeld)

	_, err := h.service.Claim(context.Background(), h.workspaceID, issue.ID, service.ClaimDelegationInput{
		Runner: "laptop-b",
		TTL:    entity.DelegationClaimTTLDefault,
	})

	if !errors.Is(err, entity.ErrDelegationClaimHeld) {
		t.Fatalf(
			"error = %v, want %v; the refusal has to reach the second runner unwrapped, because "+
				"moving on to the next issue is the only correct response to it",
			err, entity.ErrDelegationClaimHeld,
		)
	}
}

func TestAClaimCarriesTheRunnerItBelongsToAndExpiresOnItsOwn(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	issue := h.issue()
	h.expectIssue(issue)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentID: agent.ID}, nil)

	var captured repository.ClaimDelegation

	h.delegations.EXPECT().
		Claim(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, claim repository.ClaimDelegation,
		) (entity.IssueDelegation, error) {
			captured = claim

			return entity.IssueDelegation{IssueID: issue.ID, AgentID: agent.ID}, nil
		})

	if _, err := h.service.Claim(
		context.Background(), h.workspaceID, issue.ID,
		service.ClaimDelegationInput{Runner: "  laptop-a  ", TTL: time.Minute},
	); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if captured.Runner != "laptop-a" {
		t.Errorf("runner = %q, want the trimmed name", captured.Runner)
	}

	if captured.AgentID != agent.ID {
		t.Error("the claim was not bound to the calling agent")
	}

	if captured.Token == uuid.Nil {
		t.Fatal(
			"the claim carried no token; the token is what lets a heartbeat prove it is still the " +
				"same runner and not one that lapsed and was replaced",
		)
	}

	if got := captured.ExpiresAt.Sub(captured.ClaimedAt); got != time.Minute {
		t.Errorf(
			"the claim runs for %v, want the requested minute; a claim that never lapses strands "+
				"the issue when the runner dies",
			got,
		)
	}
}

func TestAClaimOutsideTheAllowedLifetimeIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	h := newHarness(t)
	h.asAgent(uuid.New())

	_, err := h.service.Claim(
		context.Background(), h.workspaceID, uuid.New(),
		service.ClaimDelegationInput{Runner: "laptop-a", TTL: entity.DelegationClaimTTLMax + time.Second},
	)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want a validation error", err)
	}
}

func TestARunnerMustNameItselfBeforeClaiming(t *testing.T) {
	h := newHarness(t)
	h.asAgent(uuid.New())

	_, err := h.service.Claim(
		context.Background(), h.workspaceID, uuid.New(),
		service.ClaimDelegationInput{TTL: entity.DelegationClaimTTLDefault},
	)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf(
			"error = %v, want a validation error; an unnamed runner cannot re-take its own claim "+
				"after a restart, which is how a restart turns into a stranded issue",
			err,
		)
	}
}

func TestAHeartbeatExtendsOnlyTheClaimWhoseTokenItPresents(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	issue := h.issue()
	h.expectIssue(issue)
	token := uuid.New()

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentID: agent.ID}, nil)

	var captured repository.HeartbeatDelegation

	h.delegations.EXPECT().
		Heartbeat(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, beat repository.HeartbeatDelegation,
		) (entity.IssueDelegation, error) {
			captured = beat

			return entity.IssueDelegation{IssueID: issue.ID}, nil
		})

	if _, err := h.service.Heartbeat(
		context.Background(), h.workspaceID, issue.ID,
		service.HeartbeatDelegationInput{Token: token, TTL: time.Minute},
	); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	if captured.Token != token {
		t.Error("the heartbeat did not carry the token it was given")
	}

	if !captured.ExpiresAt.After(time.Now().UTC()) {
		t.Error("the heartbeat did not push the claim's expiry into the future")
	}
}

func TestReleasingAClaimNobodyHoldsSaysSoRatherThanSucceedingQuietly(t *testing.T) {
	h := newHarness(t)
	agent := h.agent()
	h.asAgent(agent.ID)

	issue := h.issue()
	h.expectIssue(issue)

	h.delegations.EXPECT().
		Open(gomock.Any(), h.workspaceID, issue.ID).
		Return(entity.IssueDelegation{IssueID: issue.ID, AgentID: agent.ID}, nil)

	h.delegations.EXPECT().
		ReleaseClaim(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(entity.IssueDelegation{}, entity.ErrDelegationClaimLost)

	_, err := h.service.ReleaseClaim(context.Background(), h.workspaceID, issue.ID, uuid.New())

	if !errors.Is(err, entity.ErrDelegationClaimLost) {
		t.Fatalf("error = %v, want %v", err, entity.ErrDelegationClaimLost)
	}
}
