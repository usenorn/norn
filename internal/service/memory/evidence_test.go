package memory_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) cite(t *testing.T, statement string, evidence ...entity.MemoryEvidence) entity.Memory {
	t.Helper()

	input := content(statement)
	input.Evidence = evidence

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
	})
	if err != nil {
		t.Fatalf("create %q: %v", statement, err)
	}

	return written.Memory
}

func (h *harness) correct(t *testing.T, memory entity.Memory, statement string, evidence ...entity.MemoryEvidence) {
	t.Helper()

	input := content(statement)
	input.Evidence = evidence

	if _, err := h.service.Correct(context.Background(), h.workspaceID, memory.ID, service.CorrectMemoryInput{
		MemoryContent:   input,
		ExpectedVersion: memory.Version,
	}); err != nil {
		t.Fatalf("correct %q: %v", statement, err)
	}
}

func pinned(version *int) int {
	if version == nil {
		return 0
	}

	return *version
}

func TestACitationPinsTheRevisionItWasDerivedFrom(t *testing.T) {
	h := newHarness(t)
	issueID, commentID, commentedIssue := uuid.New(), uuid.New(), uuid.New()
	h.visibleIssues = []uuid.UUID{issueID, commentedIssue}
	h.issueVersions[issueID] = 4
	h.comments[commentID] = entity.IssueComment{ID: commentID, IssueID: commentedIssue, Version: 2}

	memory := h.cite(t, "Deploys go out on Tuesdays",
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &issueID},
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceComment, CommentID: &commentID},
	)

	stored := h.stored[memory.ID].Evidence
	if pinned(stored[0].IssueVersion) != 4 || pinned(stored[1].CommentVersion) != 2 || stored[1].IssueVersion != nil {
		t.Fatalf("stored pins %+v", stored)
	}
}

func TestACorrectionKeepsThePinsOfSourcesItStillCites(t *testing.T) {
	h := newHarness(t)
	kept, added := uuid.New(), uuid.New()
	h.visibleIssues = []uuid.UUID{kept, added}

	memory := h.cite(t, "Deploys go out on Tuesdays", entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &kept})

	h.issueVersions[kept], h.issueVersions[added] = 7, 3
	h.sourceStates[kept] = entity.MemoryEvidenceChanged

	h.correct(t, memory, "Deploys go out on Wednesdays",
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &kept},
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &added},
	)

	stored := h.stored[memory.ID].Evidence
	if len(stored) != 2 || pinned(stored[0].IssueVersion) != 1 || pinned(stored[1].IssueVersion) != 3 {
		t.Fatalf("a correction re-pinned its evidence to %+v", stored)
	}
}

func TestACorrectionKeepsEvidenceTheEditorCannotSee(t *testing.T) {
	h := newHarness(t)
	seen, unseen := uuid.New(), uuid.New()
	h.visibleIssues = []uuid.UUID{seen, unseen}

	memory := h.cite(t, "Deploys go out on Tuesdays",
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &seen},
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &unseen},
	)

	h.visibleIssues = []uuid.UUID{seen}

	h.correct(t, memory, "Deploys go out on Wednesdays", entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &seen})

	stored := h.stored[memory.ID].Evidence
	if len(stored) != 2 || *stored[0].IssueID != seen || *stored[1].IssueID != unseen {
		t.Fatalf("the correction left evidence %+v", stored)
	}
}

func TestAnUnavailableSourceIsConcealedEvenFromItsAuthor(t *testing.T) {
	h := newHarness(t)
	gone, changed := uuid.New(), uuid.New()
	h.visibleIssues = []uuid.UUID{gone, changed}

	memory := h.cite(t, "Deploys go out on Tuesdays",
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &gone},
		entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &changed},
	)

	h.sourceStates[gone] = entity.MemoryEvidenceUnavailable
	h.sourceStates[changed] = entity.MemoryEvidenceChanged

	got, err := h.service.Get(context.Background(), h.workspaceID, memory.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	concealed := entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, State: entity.MemoryEvidenceUnavailable}
	if !reflect.DeepEqual(got.Evidence[0], concealed) || got.Evidence[1].State != entity.MemoryEvidenceChanged ||
		*got.Evidence[1].IssueID != changed {
		t.Fatalf("the author sees evidence %+v", got.Evidence)
	}

	if got.Status != memory.Status {
		t.Fatalf("stale evidence moved the memory from %s to %s", memory.Status, got.Status)
	}
}
