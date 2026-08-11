package delegation_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestDelegatingAnIssueTellsTheOutsideWorldWhoIsWorkingOnIt(t *testing.T) {
	h := newHarness(t)
	issue := h.issue()
	agent := h.agent()

	h.expectIssue(issue)
	h.expectAgent(agent)

	h.delegations.EXPECT().
		Delegate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, held entity.IssueDelegation) (entity.IssueDelegation, error) {
			if held.IssueID != issue.ID || held.AgentID != agent.ID {
				t.Fatalf("delegation recorded %s to %s, want %s to %s",
					held.IssueID, held.AgentID, issue.ID, agent.ID)
			}

			if held.DelegatedByAccountID != h.actorID {
				t.Fatalf("delegated by %s, want %s", held.DelegatedByAccountID, h.actorID)
			}

			held.ID = agent.ID
			held.AgentName = agent.Name
			held.AgentAccountID = agent.AccountID

			return held, nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	var emitted entity.WebhookOutboxEntry

	h.emitter.EXPECT().
		Emit(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.WebhookOutboxEntry) error {
			emitted = entry

			return nil
		})

	if _, err := h.service.Delegate(context.Background(), h.workspaceID, issue.ID, service.DelegateIssueInput{
		AgentAccountID: agent.AccountID,
		Brief:          " fix the retry loop ",
	}); err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if emitted.Event != entity.WebhookIssueDelegated {
		t.Fatalf("emitted %s, want %s", emitted.Event, entity.WebhookIssueDelegated)
	}

	if emitted.TeamID != issue.TeamID {
		t.Fatalf("emitted for team %s, want %s", emitted.TeamID, issue.TeamID)
	}

	var payload service.WebhookDelegationPayload
	if err := json.Unmarshal(emitted.Body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.Brief != "fix the retry loop" {
		t.Fatalf("payload brief %q, want the trimmed brief", payload.Brief)
	}

	if payload.AgentAccountID != agent.AccountID.String() {
		t.Fatalf("payload names account %s, want the agent's own account %s",
			payload.AgentAccountID, agent.AccountID)
	}
}

func TestADisabledAgentCannotBeHandedWork(t *testing.T) {
	h := newHarness(t)
	issue := h.issue()

	agent := h.agent()
	agent.Status = entity.AgentStatusDisabled

	h.expectIssue(issue)
	h.expectAgent(agent)

	_, err := h.service.Delegate(context.Background(), h.workspaceID, issue.ID, service.DelegateIssueInput{
		AgentAccountID: agent.AccountID,
	})

	if !errors.Is(err, entity.ErrIssueDelegationAgentUnusable) {
		t.Fatalf("delegate to a disabled agent returned %v, want %v",
			err, entity.ErrIssueDelegationAgentUnusable)
	}
}

func TestABriefLongerThanTheLimitIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	h := newHarness(t)
	issue := h.issue()
	agent := h.agent()

	_, err := h.service.Delegate(context.Background(), h.workspaceID, issue.ID, service.DelegateIssueInput{
		AgentAccountID: agent.AccountID,
		Brief:          strings.Repeat("x", entity.IssueBriefMaxLen+1),
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("an oversized brief returned %v, want a validation error", err)
	}
}

func TestRecallingAnIssueNobodyHoldsSaysSoRatherThanSucceedingQuietly(t *testing.T) {
	h := newHarness(t)
	issue := h.issue()

	h.expectIssue(issue)

	h.delegations.EXPECT().
		Recall(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(entity.IssueDelegation{}, entity.ErrIssueDelegationNotFound)

	_, err := h.service.Recall(context.Background(), h.workspaceID, issue.ID)

	if !errors.Is(err, entity.ErrIssueDelegationNotFound) {
		t.Fatalf("recall returned %v, want %v", err, entity.ErrIssueDelegationNotFound)
	}
}

func TestRecallingDoesNotAnnounceItselfToSubscribers(t *testing.T) {
	h := newHarness(t)
	issue := h.issue()
	agent := h.agent()

	h.expectIssue(issue)

	h.delegations.EXPECT().
		Recall(gomock.Any(), h.workspaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ any, _ any) (entity.IssueDelegation, error) {
			ended := time.Now().UTC()

			return entity.IssueDelegation{
				IssueID:    issue.ID,
				AgentID:    agent.ID,
				AgentName:  agent.Name,
				RecalledAt: &ended,
			}, nil
		})

	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil)

	recalled, err := h.service.Recall(context.Background(), h.workspaceID, issue.ID)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}

	if recalled.Open() {
		t.Fatal("a recalled delegation still reads as open")
	}
}
