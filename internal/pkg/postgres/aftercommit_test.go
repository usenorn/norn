package postgres

import (
	"context"
	"testing"
)

func TestOutsideATransactionThereIsNothingToWaitFor(t *testing.T) {
	ran := false

	AfterCommit(context.Background(), func(context.Context) { ran = true })

	if !ran {
		t.Fatal(
			"a callback registered outside a transaction never ran. There is no commit coming, " +
				"so deferring it would drop the work silently.",
		)
	}
}

func TestNothingRunsUntilTheCallbacksAreDrained(t *testing.T) {
	ctx := withCallbacks(context.Background())
	ran := false

	AfterCommit(ctx, func(context.Context) { ran = true })

	if ran {
		t.Fatal(
			"a callback ran at registration time. Inside a transaction the change is not durable " +
				"yet, and a rollback after this point would have announced work that never happened.",
		)
	}

	runCallbacks(ctx)

	if !ran {
		t.Fatal("draining did not run the callback")
	}
}

func TestCallbacksRunInTheOrderTheyWereRegistered(t *testing.T) {
	ctx := withCallbacks(context.Background())
	order := make([]int, 0, 3)

	for i := range 3 {
		AfterCommit(ctx, func(context.Context) { order = append(order, i) })
	}

	runCallbacks(ctx)

	for i, seen := range order {
		if seen != i {
			t.Fatalf("callbacks ran as %v, want ascending registration order", order)
		}
	}
}

func TestDrainingTwiceDoesNotRepeatTheWork(t *testing.T) {
	ctx := withCallbacks(context.Background())
	runs := 0

	AfterCommit(ctx, func(context.Context) { runs++ })

	runCallbacks(ctx)
	runCallbacks(ctx)

	if runs != 1 {
		t.Fatalf(
			"the callback ran %d times. A nested WithTx that reached the drain twice would "+
				"publish every event twice, and duplicate events are indistinguishable from real ones.",
			runs,
		)
	}
}

func TestARegistrationMadeWhileDrainingIsNotLost(t *testing.T) {
	ctx := withCallbacks(context.Background())
	nested := false

	AfterCommit(ctx, func(inner context.Context) {
		AfterCommit(inner, func(context.Context) { nested = true })
	})

	runCallbacks(ctx)

	if !nested {
		t.Fatal(
			"a callback registered from inside another callback was dropped. WithTx drains once, " +
				"so anything queued during the drain has to be picked up by that same drain or it " +
				"is lost with no trace.",
		)
	}
}
