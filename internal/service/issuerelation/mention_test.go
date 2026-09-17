package issuerelation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) mentionable(workspaceID uuid.UUID, known ...entity.Issue) {
	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), workspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, reference entity.IssueReference, _ entity.TeamScope,
		) (entity.Issue, error) {
			for _, candidate := range known {
				if candidate.ReferenceKey == reference.Key && candidate.Number == reference.Number {
					return candidate, nil
				}
			}

			return entity.Issue{}, entity.ErrIssueNotFound
		}).
		AnyTimes()
}

func (h *harness) captureCreated() *[]entity.StoredIssueRelation {
	written := &[]entity.StoredIssueRelation{}

	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r entity.StoredIssueRelation) (entity.StoredIssueRelation, error) {
			*written = append(*written, r)

			return r, nil
		}).
		AnyTimes()

	return written
}

func TestAMentionedIssueIsRelatedToTheIssueThatMentionsIt(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	subject, mentioned := issue("MOB", 1), issue("PLT", 4)
	subject.WorkspaceID = workspaceID

	h.expectScope(workspaceID)
	h.mentionable(workspaceID, subject, mentioned)
	h.expectNoRelationHeld()
	written := h.captureCreated()

	var recorded []entity.Activity

	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.Activity) error {
			recorded = append(recorded, entry)

			return nil
		}).
		Times(2)

	if err := h.service.RelateMentioned(context.Background(), subject, "Same cause as plt-4 and PLT-4."); err != nil {
		t.Fatalf("RelateMentioned: %v", err)
	}

	if len(*written) != 1 {
		t.Fatalf("%d relations written for one mentioned issue, want 1", len(*written))
	}

	low, high := entity.NormalisePair(subject.ID, mentioned.ID)
	relation := (*written)[0]

	if relation.Kind != entity.IssueRelationRelatesTo || relation.SourceIssueID != low || relation.TargetIssueID != high {
		t.Fatalf("stored %+v, want a relates_to on the normalised pair", relation)
	}

	for _, entry := range recorded {
		if entry.Kind != entity.ActivityKindRelationAdded || entry.Field != string(entity.IssueRelationViewRelatesTo) {
			t.Fatalf("recorded %+v, want relation_added as relates_to on both issues", entry)
		}
	}
}

func TestMentionsThatNeedNoRelationAreSkipped(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	subject, related := issue("MOB", 1), issue("MOB", 2)
	subject.WorkspaceID = workspaceID

	h.expectScope(workspaceID)
	h.mentionable(workspaceID, subject, related)
	h.relations.EXPECT().
		FindPair(gomock.Any(), workspaceID, subject.ID, related.ID, gomock.Any()).
		Return(entity.IssueRelation{Kind: entity.IssueRelationViewBlocks, Issue: related}, nil)
	written := h.captureCreated()

	text := "Split out of MOB-1, blocks MOB-2, unlike SHA-256 or the hidden OPS-9."

	if err := h.service.RelateMentioned(context.Background(), subject, text); err != nil {
		t.Fatalf("RelateMentioned: %v", err)
	}

	if len(*written) != 0 {
		t.Fatalf(
			"wrote %+v; the issue itself, a pair already related in any kind, and a reference "+
				"that is missing or not visible each relate nothing",
			*written,
		)
	}
}

func TestAFailedLookupOfAMentionIsReturnedRatherThanSkipped(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	subject := issue("MOB", 1)
	subject.WorkspaceID = workspaceID

	unavailable := errors.New("connection reset")

	h.expectScope(workspaceID)
	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), workspaceID, gomock.Any(), gomock.Any()).
		Return(entity.Issue{}, unavailable)

	err := h.service.RelateMentioned(context.Background(), subject, "See MOB-2.")
	if !errors.Is(err, unavailable) {
		t.Fatalf("RelateMentioned returned %v; only a missing or hidden issue may be skipped", err)
	}
}

func TestAFailedPairLookupIsReturnedRatherThanSkipped(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	subject, mentioned := issue("MOB", 1), issue("MOB", 2)
	subject.WorkspaceID = workspaceID

	unavailable := errors.New("connection reset")

	h.expectScope(workspaceID)
	h.mentionable(workspaceID, subject, mentioned)
	h.relations.EXPECT().
		FindPair(gomock.Any(), workspaceID, subject.ID, mentioned.ID, gomock.Any()).
		Return(entity.IssueRelation{}, unavailable)

	err := h.service.RelateMentioned(context.Background(), subject, "See MOB-2.")
	if !errors.Is(err, unavailable) {
		t.Fatalf("RelateMentioned returned %v, want the lookup failure", err)
	}
}

func TestAPairRelatedConcurrentlyIsSkipped(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	subject, mentioned := issue("MOB", 1), issue("MOB", 2)
	subject.WorkspaceID = workspaceID

	h.expectScope(workspaceID)
	h.mentionable(workspaceID, subject, mentioned)
	h.expectNoRelationHeld()
	h.relations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.StoredIssueRelation{}, entity.ErrIssueRelationExists)

	if err := h.service.RelateMentioned(context.Background(), subject, "See MOB-2."); err != nil {
		t.Fatalf("RelateMentioned: %v; a relation written between the check and the insert is the one wanted", err)
	}
}

func TestADescriptionWithoutMentionsAsksNothing(t *testing.T) {
	h := newHarness(t)

	if err := h.service.RelateMentioned(context.Background(), issue("MOB", 1), "Retries drop the key."); err != nil {
		t.Fatalf("RelateMentioned: %v", err)
	}
}
