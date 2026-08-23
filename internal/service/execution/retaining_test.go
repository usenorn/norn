package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestKeepingARunLongerAsksTheMachineRatherThanOnlyNornsOwnRecords(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)

	if _, err := h.service.Retain(
		context.Background(), h.workspaceID, execution.ID, time.Hour,
	); err != nil {
		t.Fatalf("keep the run longer: %v", err)
	}

	sent, delivered := h.sent(entity.ChannelExecutionRetain)
	if !delivered {
		t.Fatal(
			"nothing reached the machine. The machine owns the clock its sweep runs on, so a " +
				"deadline only norn knows about leaves the preview open onto a workspace that " +
				"has already been given back",
		)
	}

	var asked channelv1.Retention
	if err := json.Unmarshal(sent.Payload, &asked); err != nil {
		t.Fatalf("decode what the machine was asked: %v", err)
	}

	if !asked.KeepUntil.After(time.Now().UTC()) {
		t.Fatalf("the machine was asked to hold the run until %s, which is not later", asked.KeepUntil)
	}
}

func TestARunIsOnlyKeptLongerWhileSomebodyIsStillReviewingIt(t *testing.T) {
	for _, state := range []entity.ExecutionState{
		entity.ExecutionRunning,
		entity.ExecutionCompleted,
		entity.ExecutionCancelled,
		entity.ExecutionWaitingForInput,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)

			execution := h.execution(state)
			h.holding(execution)

			_, err := h.service.Retain(context.Background(), h.workspaceID, execution.ID, time.Hour)
			if !errors.Is(err, entity.ErrPreviewNotExtendable) {
				t.Fatalf(
					"a run in %s was kept longer and answered %v. The extension exists for the "+
						"reviewer looking at it, and asking a machine to hold a run it already "+
						"cleared away is asking for nothing",
					state, err,
				)
			}

			if _, delivered := h.sent(entity.ChannelExecutionRetain); delivered {
				t.Fatal("the machine was asked to hold a run that is not under review")
			}
		})
	}
}

func TestARunCannotBeHeldForLongerThanTheLongestWindowThereIs(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)

	for name, longer := range map[string]time.Duration{
		"no time at all":            0,
		"backwards":                 -time.Hour,
		"longer than a day is long": entity.PreviewRetentionLongest + time.Minute,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := h.service.Retain(
				context.Background(), h.workspaceID, execution.ID, longer,
			); err == nil {
				t.Fatalf(
					"a run was held for %s. A machine told to keep a workspace indefinitely "+
						"never gives its disk back",
					longer,
				)
			}
		})
	}
}

func TestKeepingARunLongerSaysSoOnItsOwnTimeline(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)

	if _, err := h.service.Retain(
		context.Background(), h.workspaceID, execution.ID, time.Hour,
	); err != nil {
		t.Fatalf("keep the run longer: %v", err)
	}

	if _, recorded := h.entry(entity.ExecutionEventPhase); !recorded {
		t.Fatal(
			"nothing went on the timeline. A workspace that outlives its window with no line " +
				"saying why looks like the sweep failing rather than somebody asking",
		)
	}
}

func TestARunWhoseMachineIsGoneCannotBeAskedToHoldAnything(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	execution.RunnerID = uuid.Nil
	h.holding(execution)

	_, err := h.service.Retain(context.Background(), h.workspaceID, execution.ID, time.Hour)
	if !errors.Is(err, entity.ErrExecutionNoRunner) {
		t.Fatalf(
			"a run with no machine behind it was held longer and answered %v. Answering the "+
				"reviewer that it worked would promise something nothing is doing",
			err,
		)
	}
}
