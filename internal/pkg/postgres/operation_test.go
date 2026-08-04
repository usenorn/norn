package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestOutsideATransactionEveryRowStandsAlone(t *testing.T) {
	subject := uuid.New()
	first, second := uuid.New(), uuid.New()

	if got := Operation(context.Background(), subject, first); got != first {
		t.Fatalf("a lone write adopted %v rather than becoming its own operation", got)
	}

	if got := Operation(context.Background(), subject, second); got != second {
		t.Fatal(
			"two writes outside a transaction were grouped. Nothing ties them together — they " +
				"are separate saves and must read as separate events.",
		)
	}
}

func TestInsideATransactionEveryRowAboutOneSubjectJoinsTheFirst(t *testing.T) {
	ctx := withOperations(context.Background())
	subject := uuid.New()
	leader, follower, third := uuid.New(), uuid.New(), uuid.New()

	if got := Operation(ctx, subject, leader); got != leader {
		t.Fatalf("the first write took %v as its operation rather than leading one", got)
	}

	for _, id := range []uuid.UUID{follower, third} {
		if got := Operation(ctx, subject, id); got != leader {
			t.Fatalf(
				"a later write in the same transaction took %v rather than joining the leader. "+
					"One save that changes four properties has to read as one event.",
				got,
			)
		}
	}
}

func TestTwoSubjectsInOneTransactionEachLeadTheirOwn(t *testing.T) {
	ctx := withOperations(context.Background())
	issue, parent := uuid.New(), uuid.New()
	onIssue, onParent := uuid.New(), uuid.New()

	if got := Operation(ctx, issue, onIssue); got != onIssue {
		t.Fatalf("the issue's first row took %v", got)
	}

	if got := Operation(ctx, parent, onParent); got != onParent {
		t.Fatal(
			"a row about a second subject joined the first subject's operation. Its feed pages " +
				"over leaders, so it would have no leader of its own and would show nothing — " +
				"which is exactly what a bulk chunk of twenty-five issues would have done.",
		)
	}
}
