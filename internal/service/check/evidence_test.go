package check_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func observation() service.SubmitEvidenceInput {
	return service.SubmitEvidenceInput{
		Verdict:    entity.EvidencePassed,
		Channel:    entity.EvidenceChannelCommand,
		Command:    "go test ./internal/payments/...",
		Output:     "ok  github.com/usenorn/norn/internal/payments 1.204s",
		ObservedAt: time.Now().UTC().Add(-time.Minute),
	}
}

func (h *harness) captureEvidence(t *testing.T) *entity.Evidence {
	t.Helper()

	captured := &entity.Evidence{}

	h.evidence.EXPECT().
		Append(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, record entity.Evidence) (entity.Evidence, error) {
			*captured = record

			return record, nil
		})

	return captured
}

func TestASecretInSubmittedOutputNeverReachesStorage(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()

	stored := h.captureEvidence(t)

	input := observation()
	input.Output = "connecting to postgres://norn:s3cr3t-pw@db.internal:5432/norn\nok"
	input.Command = `curl -H "Authorization: Bearer eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM"`

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, input,
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if strings.Contains(stored.Output, "s3cr3t-pw") {
		t.Fatalf("the password reached the repository: %q", stored.Output)
	}

	if strings.Contains(stored.Command, "eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM") {
		t.Fatalf("the bearer token reached the repository: %q", stored.Command)
	}

	if stored.Redactions != 2 {
		t.Fatalf("recorded %d redactions, want one for the output and one for the command", stored.Redactions)
	}
}

func TestEvidenceIsStampedWithNornsClockNotTheOneItWasHandedTest(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()

	stored := h.captureEvidence(t)

	before := time.Now().UTC()

	input := observation()
	input.ObservedAt = before.Add(48 * time.Hour)

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, input,
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if stored.ReceivedAt.Before(before) {
		t.Fatal("the received stamp came from somewhere other than Norn's own clock")
	}

	if stored.ObservedAt.After(stored.ReceivedAt) {
		t.Fatalf(
			"an observation claimed for %s was stored ahead of when Norn received it at %s",
			stored.ObservedAt, stored.ReceivedAt,
		)
	}
}

func TestAnObservationClaimedInThePastKeepsItsClaimedTime(t *testing.T) {
	h := newHarness(t, entity.ActorKindUser)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()

	stored := h.captureEvidence(t)

	input := observation()
	claimed := time.Now().UTC().Add(-2 * time.Hour)
	input.ObservedAt = claimed

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, input,
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if !stored.ObservedAt.Equal(claimed) {
		t.Fatalf("stored the observation at %s, want the claimed %s", stored.ObservedAt, claimed)
	}
}

func TestEvidenceIsStampedWithTheHeadCommitNornHoldsRatherThanOneItWasTold(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)

	link := entity.CodeLink{
		ID:        uuid.New(),
		Kind:      entity.CodeLinkChange,
		State:     entity.CodeChangeOpen,
		HeadSHA:   "9f2c1ab4d5e6f708192a3b4c5d6e7f8091a2b3c4",
		CreatedAt: time.Now().UTC(),
	}

	h.codeLinks.EXPECT().
		ListByIssue(gomock.Any(), h.workspaceID, issue.ID).
		Return([]entity.CodeLink{link}, nil)

	stored := h.captureEvidence(t)

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, observation(),
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if stored.CodeLinkID != link.ID || stored.CommitSHA != link.HeadSHA {
		t.Fatalf(
			"evidence was stamped with link %s at %q, want %s at %q",
			stored.CodeLinkID, stored.CommitSHA, link.ID, link.HeadSHA,
		)
	}
}

func TestEvidenceOnAnIssueWithNoLinkedChangeCarriesNoCommit(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()

	stored := h.captureEvidence(t)

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, observation(),
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if stored.CodeLinkID != uuid.Nil || stored.CommitSHA != "" {
		t.Fatal("evidence with no linked change was stamped with one anyway")
	}
}

func TestOversizedOutputIsCutRatherThanRefused(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()

	stored := h.captureEvidence(t)

	input := observation()
	input.Output = strings.Repeat("noise\n", entity.EvidenceOutputMaxBytes)

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, input,
	); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if !stored.Truncated {
		t.Fatal("oversized output was stored without being marked truncated")
	}

	if len(stored.Output) > entity.EvidenceOutputMaxBytes {
		t.Fatalf("stored %d bytes, want at most %d", len(stored.Output), entity.EvidenceOutputMaxBytes)
	}
}

func TestEvidenceWithNoOutputIsRefused(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	input := observation()
	input.Output = ""

	_, err := h.service.Submit(context.Background(), h.workspaceID, issue.ID, check.ID, input)

	if !errors.Is(err, entity.ErrEvidenceEmpty) {
		t.Fatalf("submitting without output returned %v, want %v", err, entity.ErrEvidenceEmpty)
	}
}

func TestADeclinedCheckTakesNoEvidence(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()

	check := h.check(issue)
	check.Approval = entity.CheckApprovalDeclined

	h.expectIssue(issue)
	h.expectCheck(check)

	_, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, observation(),
	)

	if !errors.Is(err, entity.ErrCheckDeclined) {
		t.Fatalf("submitting to a declined check returned %v, want %v", err, entity.ErrCheckDeclined)
	}
}

func TestSubmittingEvidenceNeverRewritesTheCheckItself(t *testing.T) {
	h := newHarness(t, entity.ActorKindAgent)
	issue := h.issue()
	check := h.check(issue)

	h.expectIssue(issue)
	h.expectCheck(check)
	h.expectNoCodeLinks()
	h.captureEvidence(t)

	if _, err := h.service.Submit(
		context.Background(), h.workspaceID, issue.ID, check.ID, observation(),
	); err != nil {
		t.Fatalf("submit: %v", err)
	}
}
