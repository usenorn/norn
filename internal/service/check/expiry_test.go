package check_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

func (h *harness) stale(issue entity.Issue) {
	h.checks.EXPECT().
		ListStaleIssues(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]repository.StaleIssue{{WorkspaceID: h.workspaceID, IssueID: issue.ID}}, nil)
}

func provenAt(check entity.Check, received time.Time) entity.Evidence {
	return entity.Evidence{
		ID:         uuid.New(),
		CheckID:    check.ID,
		IssueID:    check.IssueID,
		Verdict:    entity.EvidencePassed,
		Channel:    entity.EvidenceChannelCommand,
		ReceivedAt: received,
		ObservedAt: received,
		Actor:      entity.ActivityAttribution{Kind: entity.ActorKindAgent},
	}
}

func TestTheSweepAnnouncesOnlyWhenACheckActuallyLostItsProof(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)

	issue := h.issue()
	timedOut := h.check(issue)
	fresh := h.check(issue)

	h.expectIssue(issue)
	h.stale(issue)

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Check{timedOut, fresh}, nil)

	h.evidence.EXPECT().
		Digest(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Evidence{
			provenAt(timedOut, time.Now().UTC().Add(-entity.CheckTimeLimitDefault-time.Hour)),
			provenAt(fresh, time.Now().UTC().Add(-time.Hour)),
		}, nil)

	var announced []uuid.UUID

	h.checks.EXPECT().
		AnnounceExpiry(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, checkID, _ uuid.UUID) error {
			announced = append(announced, checkID)

			return nil
		}).
		AnyTimes()

	if err := h.service.SweepExpiry(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(announced) != 1 || announced[0] != timedOut.ID {
		t.Fatalf(
			"announced %v, want only the check whose proof timed out; the sweep must re-derive "+
				"state rather than trust the query that narrowed the candidates",
			announced,
		)
	}

	if len(h.announced()) != 1 {
		t.Fatalf("recorded %d timeline entries, want 1", len(h.announced()))
	}

	if h.announced()[0].ToValue != timedOut.Statement {
		t.Errorf("the entry does not name the criterion: %+v", h.announced()[0])
	}

	if h.announced()[0].FromValue != string(entity.EvidenceTimedOut) {
		t.Errorf("the entry does not say why the proof stopped counting: %+v", h.announced()[0])
	}
}

func TestTheSweepSaysNothingAboutACheckAPersonAlreadyWaived(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)

	issue := h.issue()
	waived := h.check(issue)
	waived.Resolution = entity.CheckResolutionWaived

	h.expectIssue(issue)
	h.stale(issue)

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Check{waived}, nil)

	h.evidence.EXPECT().
		Digest(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Evidence{
			provenAt(waived, time.Now().UTC().Add(-entity.CheckTimeLimitDefault-time.Hour)),
		}, nil)

	h.checks.EXPECT().
		AnnounceExpiry(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	if err := h.service.SweepExpiry(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(h.announced()) != 0 {
		t.Fatalf(
			"the sweep announced expiry on a waived check; a person already decided it does not " +
				"apply, and telling them its proof aged out is noise",
		)
	}
}

func TestTheSweepAnnouncesWhenTheChangeAProofWasTakenAtMovedOn(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)

	issue := h.issue()
	check := h.check(issue)
	linkID := uuid.New()

	h.expectIssue(issue)
	h.stale(issue)
	h.linking(entity.CodeLink{
		ID:      linkID,
		Kind:    entity.CodeLinkChange,
		State:   entity.CodeChangeOpen,
		HeadSHA: "bbbbbbb",
	})

	proof := provenAt(check, time.Now().UTC().Add(-time.Hour))
	proof.CodeLinkID = linkID
	proof.CommitSHA = "aaaaaaa"

	h.checks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Check{check}, nil)

	h.evidence.EXPECT().
		Digest(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.Evidence{proof}, nil)

	h.checks.EXPECT().
		AnnounceExpiry(gomock.Any(), h.workspaceID, check.ID, proof.ID).
		Return(nil)

	if err := h.service.SweepExpiry(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(h.announced()) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(h.announced()))
	}

	if h.announced()[0].FromValue != string(entity.EvidenceHeadMoved) {
		t.Errorf(
			"the entry says %q, want head_moved so the reader knows to run it again rather than "+
				"wait for a clock",
			h.announced()[0].FromValue,
		)
	}
}
