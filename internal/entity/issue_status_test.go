package entity_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestEveryOrderedPairOfIssueStatusesIsDecidedDeliberately(t *testing.T) {
	permitted := map[entity.IssueStatus][]entity.IssueStatus{
		entity.IssueStatusActive:          {entity.IssueStatusArchived, entity.IssueStatusPendingDeletion},
		entity.IssueStatusArchived:        {entity.IssueStatusActive, entity.IssueStatusPendingDeletion},
		entity.IssueStatusPendingDeletion: {entity.IssueStatusActive, entity.IssueStatusArchived},
	}

	for _, from := range entity.IssueStatuses() {
		for _, to := range entity.IssueStatuses() {
			want := false

			for _, allowed := range permitted[from] {
				if allowed == to {
					want = true
				}
			}

			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s is %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestNoIssueStatusTransitionsToItself(t *testing.T) {
	for _, status := range entity.IssueStatuses() {
		if status.CanTransitionTo(status) {
			t.Errorf(
				"%s -> %s is permitted; re-archiving an archived issue would rewrite archived_at "+
					"and lose when it was actually archived",
				status, status,
			)
		}
	}
}

func TestArchivingRecordsWhenAndNeverSchedulesAPurge(t *testing.T) {
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	archived := entity.ApplyIssueStatus(entity.IssueStatusArchived, nil, now)

	if archived.ArchivedAt == nil || !archived.ArchivedAt.Equal(now) {
		t.Fatal("archiving did not record when it happened")
	}

	if archived.PurgeAfter != nil || archived.DeletionRequestedAt != nil {
		t.Fatal(
			"archiving scheduled a purge. An archive is kept indefinitely and stays retrievable; " +
				"only a delete is ever collected.",
		)
	}
}

func TestDeletingAnArchivedIssueKeepsWhenItWasArchived(t *testing.T) {
	archivedAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	deleted := entity.ApplyIssueStatus(entity.IssueStatusPendingDeletion, &archivedAt, now)

	if deleted.ArchivedAt == nil || !deleted.ArchivedAt.Equal(archivedAt) {
		t.Fatal(
			"deleting an archived issue forgot that it was archived, so restoring it would " +
				"silently return it to the active list",
		)
	}

	if deleted.PurgeAfter == nil || !deleted.PurgeAfter.Equal(now.Add(entity.IssuePurgeGrace)) {
		t.Fatal("deleting did not schedule the purge one grace period out")
	}
}

func TestARestoredIssueReturnsToTheShelfItWasTakenFrom(t *testing.T) {
	archivedAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

	if got := entity.RestoredIssueStatus(&archivedAt); got != entity.IssueStatusArchived {
		t.Errorf("an archived issue restored to %q, want archived", got)
	}

	if got := entity.RestoredIssueStatus(nil); got != entity.IssueStatusActive {
		t.Errorf("an active issue restored to %q, want active", got)
	}
}

func TestRestoringClearsTheScheduledPurge(t *testing.T) {
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	restored := entity.ApplyIssueStatus(entity.IssueStatusActive, nil, now)

	if restored.PurgeAfter != nil || restored.DeletionRequestedAt != nil {
		t.Fatal(
			"a restored issue kept its purge schedule; the deferred job would collect an issue " +
				"the owner had already recovered",
		)
	}
}

func TestAPurgeIsOnlyDueOnceTheGraceHasElapsedAndOnlyForADeletedIssue(t *testing.T) {
	requested := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	due := requested.Add(entity.IssuePurgeGrace)

	deleted := entity.ApplyIssueStatus(entity.IssueStatusPendingDeletion, nil, requested)

	cases := map[string]struct {
		lifecycle entity.IssueLifecycle
		at        time.Time
		want      bool
	}{
		"a moment before the grace elapses": {deleted, due.Add(-time.Second), false},
		"exactly when the grace elapses":    {deleted, due, true},
		"long after the grace elapses":      {deleted, due.Add(time.Hour), true},
		"restored before the grace elapsed": {
			entity.ApplyIssueStatus(entity.IssueStatusActive, nil, requested), due.Add(time.Hour), false,
		},
		"archived, never scheduled": {
			entity.ApplyIssueStatus(entity.IssueStatusArchived, nil, requested), due.Add(time.Hour), false,
		},
	}

	for name, tc := range cases {
		if got := tc.lifecycle.PurgeDue(tc.at); got != tc.want {
			t.Errorf("%s: purge due = %v, want %v", name, got, tc.want)
		}
	}
}
