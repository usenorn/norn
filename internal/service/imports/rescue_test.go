package imports_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

// stalled answers the way the query does: a run whose lease expired, or one that was
// released and never re-enqueued and has sat untouched for longer than a lease.
func (h *harness) stalled(run entity.ImportRun, updatedAt time.Time) {
	h.runs.EXPECT().
		ListLeaseExpired(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			at, idleSince time.Time,
			_ int,
		) ([]entity.ImportRun, error) {
			if run.LeaseExpiresAt != nil {
				if run.LeaseExpiresAt.Before(at) {
					return []entity.ImportRun{run}, nil
				}

				return nil, nil
			}

			if updatedAt.Before(idleSince) {
				return []entity.ImportRun{run}, nil
			}

			return nil, nil
		})
}

func executingRun() entity.ImportRun {
	return entity.ImportRun{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		Status:      entity.ImportExecuting,
		Attempt:     3,
	}
}

func TestAReleasedRunWhoseContinuationWasLostIsDrivenAgain(t *testing.T) {
	h := newHarness(t)
	run := executingRun()

	h.stalled(run, time.Now().UTC().Add(-2*testLeaseTTL))
	h.cursors.EXPECT().List(gomock.Any(), run.ID).Return(nil, nil)

	var enqueued entity.ImportExecutePayload

	h.jobs.EXPECT().
		EnqueueImportExecute(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			payload entity.ImportExecutePayload,
			_ time.Time,
		) error {
			enqueued = payload

			return nil
		})

	rescued, err := h.rescue.Rescue(context.Background())
	if err != nil {
		t.Fatalf("rescue: %v", err)
	}

	if rescued != 1 {
		t.Fatalf("rescued %d runs, want 1", rescued)
	}

	if enqueued.ImportRunID != run.ID {
		t.Errorf("re-drove run %s, want %s", enqueued.ImportRunID, run.ID)
	}

	if enqueued.Attempt != run.Attempt+1 {
		t.Errorf("re-drove at attempt %d, want %d", enqueued.Attempt, run.Attempt+1)
	}
}

func TestARunReleasedAMomentAgoIsLeftToItsOwnContinuation(t *testing.T) {
	h := newHarness(t)
	run := executingRun()

	h.stalled(run, time.Now().UTC().Add(-time.Second))

	rescued, err := h.rescue.Rescue(context.Background())
	if err != nil {
		t.Fatalf("rescue: %v", err)
	}

	if rescued != 0 {
		t.Fatalf("rescued %d runs, want none: the enqueue is still in flight", rescued)
	}
}

func TestARunParkedOnASourceRateLimitIsNotWokenEarly(t *testing.T) {
	h := newHarness(t)

	run := executingRun()
	run.Status = entity.ImportStaging

	expires := time.Now().UTC().Add(-time.Minute)
	run.LeaseExpiresAt = &expires

	h.stalled(run, time.Now().UTC())

	until := time.Now().UTC().Add(time.Hour)

	h.cursors.EXPECT().List(gomock.Any(), run.ID).Return([]entity.ImportCursor{
		{RunID: run.ID, Resource: entity.ImportIssue, RetryAfter: &until},
	}, nil)

	rescued, err := h.rescue.Rescue(context.Background())
	if err != nil {
		t.Fatalf("rescue: %v", err)
	}

	if rescued != 0 {
		t.Fatalf("rescued %d runs, want none: the source asked to be left alone", rescued)
	}
}
