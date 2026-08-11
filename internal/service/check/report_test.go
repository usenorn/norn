package check_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestReadingAnIssuesChecksDerivesEachStateFromTheEvidenceOnFile(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	proven := h.check(issue)

	failed := h.check(issue)
	failed.ID = uuid.New()

	quiet := h.check(issue)
	quiet.ID = uuid.New()

	proposed := h.check(issue)
	proposed.ID = uuid.New()
	proposed.Approval = entity.CheckApprovalPending

	h.expectIssue(issue)

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Check{proven, failed, quiet, proposed}, nil)

	filed := time.Now().UTC().Add(-time.Minute)

	h.evidence.EXPECT().
		Digest(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Evidence{
			{ID: uuid.New(), CheckID: proven.ID, Verdict: entity.EvidencePassed, ReceivedAt: filed},
			{ID: uuid.New(), CheckID: failed.ID, Verdict: entity.EvidenceFailed, ReceivedAt: filed},
			{ID: uuid.New(), CheckID: quiet.ID, Verdict: entity.EvidenceAbsentNegative, ReceivedAt: filed},
		}, nil)

	read, err := h.service.List(context.Background(), h.workspaceID, issue.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	states := map[uuid.UUID]entity.CheckState{}
	for _, report := range read.Reports {
		states[report.Check.ID] = report.State
	}

	for _, want := range []struct {
		id    uuid.UUID
		state entity.CheckState
		label string
	}{
		{proven.ID, entity.CheckStateProven, "the one with a passing run"},
		{failed.ID, entity.CheckStateFailed, "the one with a failing run"},
		{quiet.ID, entity.CheckStateUnproven, "the one where nothing bad appeared"},
		{proposed.ID, entity.CheckStateUnproven, "the one nobody approved"},
	} {
		if states[want.id] != want.state {
			t.Errorf("%s reads as %q, want %q", want.label, states[want.id], want.state)
		}
	}

	if read.Summary.Blocking != 2 {
		t.Fatalf(
			"summary says %d checks block, want the failed one and the quiet one",
			read.Summary.Blocking,
		)
	}

	if read.Summary.RestingOnAbsence != 1 {
		t.Fatalf("summary says %d rest on absence, want 1", read.Summary.RestingOnAbsence)
	}
}

func TestReadingChecksNeverPullsStoredOutputItDoesNotNeed(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()

	h.expectIssue(issue)
	h.checks.EXPECT().ListByIssue(gomock.Any(), h.workspaceID, issue.ID).Return(nil, nil)
	h.evidence.EXPECT().Digest(gomock.Any(), h.workspaceID, issue.ID).Return(nil, nil)

	if _, err := h.service.List(context.Background(), h.workspaceID, issue.ID); err != nil {
		t.Fatalf("list: %v", err)
	}
}
