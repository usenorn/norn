package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestARetryRepublishesTheApprovedRevisionEvenAfterANewerOneLanded(t *testing.T) {
	h := newHarness(t)
	execution := h.execution(entity.ExecutionApproved)
	h.holding(execution)
	h.changed("api", "head-1")
	h.submitted = append(h.submitted, entity.ExecutionReview{
		Verdict: entity.VerdictApprove, Revision: 1, Heads: entity.ReviewHeads{"api": "head-1"},
	})
	h.changes = []entity.ExecutionChange{{Repository: "api", HeadSHA: "head-2"}}
	h.revision = 2

	if _, err := h.service.RetryPublication(context.Background(), h.workspaceID, execution.ID); err != nil {
		t.Fatalf("retry publication: %v", err)
	}

	instruction := h.instruction(t)

	if instruction.Reason != channelv1.ResumePublish || instruction.Revision != 1 ||
		instruction.Heads["api"] != "head-1" {
		t.Fatalf(
			"a retry told the machine %+v; it must name the revision a person approved, never "+
				"the latest one the machine happened to report",
			instruction,
		)
	}
}

func TestOnlyARunWaitingToPublishCanRetryOrGiveUp(t *testing.T) {
	for _, state := range []entity.ExecutionState{
		entity.ExecutionAwaitingReview, entity.ExecutionCompleted, entity.ExecutionFailed,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)
			execution := h.execution(state)
			h.holding(execution)

			if _, err := h.service.RetryPublication(
				context.Background(), h.workspaceID, execution.ID,
			); !errors.Is(err, entity.ErrPublicationNotPending) {
				t.Fatalf("retrying a %s run returned %v", state, err)
			}

			if _, err := h.service.AbandonPublication(
				context.Background(), h.workspaceID, execution.ID,
			); !errors.Is(err, entity.ErrPublicationNotPending) {
				t.Fatalf("giving up on a %s run returned %v", state, err)
			}

			if _, sent := h.sent(entity.ChannelExecutionResume); sent {
				t.Fatalf("a %s run was told to publish", state)
			}
		})
	}
}

func TestGivingUpTellsTheMachineToStopPublishing(t *testing.T) {
	h := newHarness(t)
	execution := h.execution(entity.ExecutionApproved)
	h.holding(execution)
	h.changed("api", "head-1")
	h.submitted = append(h.submitted, entity.ExecutionReview{
		Verdict: entity.VerdictApprove, Revision: 1, Heads: entity.ReviewHeads{"api": "head-1"},
	})

	if _, err := h.service.AbandonPublication(context.Background(), h.workspaceID, execution.ID); err != nil {
		t.Fatalf("give up: %v", err)
	}

	if instruction := h.instruction(t); instruction.Reason != channelv1.ResumeAbandon {
		t.Fatalf("giving up told the machine %+v", instruction)
	}
}
