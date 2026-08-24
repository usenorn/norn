package execution_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func kept(t *testing.T, executionID string, keepUntil time.Time) entity.ChannelMessage {
	t.Helper()

	payload, err := json.Marshal(channelv1.Retention{KeepUntil: keepUntil})
	if err != nil {
		t.Fatalf("write a retention report: %v", err)
	}

	return entity.ChannelMessage{
		ID:          "msg-keep",
		Type:        entity.ChannelExecutionKept,
		ExecutionID: executionID,
		IssuedAt:    time.Now().UTC(),
		Payload:     payload,
	}
}

func TestTheDeadlineOnRecordIsTheOneTheMachineReported(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionCompleted)
	h.holding(execution)

	deadline := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)

	var written time.Time

	h.executions.EXPECT().
		Keep(gomock.Any(), execution.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, id string, until time.Time) (entity.Execution, error) {
			written = until
			held := execution
			held.KeepUntil = &until

			return held, nil
		})

	if err := h.service.Kept(context.Background(), h.runner, kept(t, execution.ID, deadline)); err != nil {
		t.Fatalf("take the machine's deadline: %v", err)
	}

	if !written.Equal(deadline) {
		t.Fatalf(
			"norn wrote down %s but the machine said %s. The machine owns the sweep, so a "+
				"deadline norn invents is one the screen shows and the workspace ignores",
			written, deadline,
		)
	}
}

func TestANewDeadlineReachesEverybodyWatchingTheRun(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionCompleted)
	h.holding(execution)

	deadline := time.Now().UTC().Add(time.Hour)

	h.executions.EXPECT().
		Keep(gomock.Any(), execution.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, until time.Time) (entity.Execution, error) {
			held := execution
			held.KeepUntil = &until

			return held, nil
		})

	if err := h.service.Kept(context.Background(), h.runner, kept(t, execution.ID, deadline)); err != nil {
		t.Fatalf("take the machine's deadline: %v", err)
	}

	event, announced := h.announced(entity.EventExecutionUpdated)
	if !announced {
		t.Fatal(
			"nobody was told. Somebody watching a run they just approved has to see when its " +
				"preview goes without reloading the page",
		)
	}

	var carried entity.Execution
	if err := json.Unmarshal(event.Payload, &carried); err != nil {
		t.Fatalf("decode what was announced: %v", err)
	}

	if carried.KeepUntil == nil || !carried.KeepUntil.Equal(deadline.UTC()) {
		t.Fatalf("the announcement carried %v rather than the new deadline %s", carried.KeepUntil, deadline)
	}
}

func TestAReportWithNoDeadlineChangesNothing(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)

	if err := h.service.Kept(
		context.Background(), h.runner, kept(t, execution.ID, time.Time{}),
	); err != nil {
		t.Fatalf("take an empty retention report: %v", err)
	}

	if _, announced := h.announced(entity.EventExecutionUpdated); announced {
		t.Fatal("an empty report moved the deadline, so a screen would show a time nothing set")
	}
}

func TestAMachineCannotSetTheDeadlineOfARunItIsNotHolding(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionCompleted)
	execution.RunnerID = h.issue.ID
	h.holding(execution)

	err := h.service.Kept(context.Background(), h.runner, kept(t, execution.ID, time.Now().UTC()))
	if err == nil {
		t.Fatal(
			"a machine set the deadline of a run another machine holds, which is a machine " +
				"giving somebody else's workspace away",
		)
	}
}
