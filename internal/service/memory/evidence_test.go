package memory_test

import (
	"context"
	"errors"
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

func (h *harness) personalDoc() entity.MemoryEvidence {
	docID, revisionID := uuid.New(), uuid.New()
	h.docs[docID] = entity.Doc{DocSummary: entity.DocSummary{
		ID: docID, WorkspaceID: h.workspaceID, AuthorID: h.accountID, Audience: entity.PersonalAudience(),
	}}
	h.docRevisions[revisionID] = docID

	return entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, DocID: &docID, DocRevisionID: &revisionID}
}

func TestOnlyAnExplicitRedactionSharesAMemoryWiderThanItsDoc(t *testing.T) {
	h := newHarness(t)
	input := content("Use SQLite")
	input.Evidence = []entity.MemoryEvidence{h.personalDoc()}

	_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("an unredacted workspace memory from a personal doc got %v, want ErrKnowledgeNotShareable", err)
	}

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
		Redacted:      true,
	})
	if err != nil {
		t.Fatalf("a redacted workspace memory from a personal doc: %v", err)
	}

	if !written.Memory.Redacted || !h.revisionsOf(written.Memory.ID)[0].Redacted {
		t.Fatalf("the redaction was not recorded on %+v", written.Memory)
	}

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleMember)

	got, err := h.service.Get(context.Background(), h.workspaceID, written.Memory.ID)
	if err != nil {
		t.Fatalf("a member reads the redacted memory: %v", err)
	}

	concealed := entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, State: entity.MemoryEvidenceUnavailable}
	if len(got.Evidence) != 1 || !reflect.DeepEqual(got.Evidence[0], concealed) {
		t.Fatalf("a member sees the personal doc behind a redacted memory as %+v", got.Evidence)
	}
}

func TestARedactionThatNoLongerWidensIsCleared(t *testing.T) {
	h := newHarness(t)
	input := content("Use SQLite")
	input.Evidence = []entity.MemoryEvidence{h.personalDoc()}

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
		Redacted:      true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	reshared, err := h.service.Reshare(context.Background(), h.workspaceID, written.Memory.ID, service.ReshareMemoryInput{
		ExpectedVersion: written.Memory.Version,
		Audience:        entity.PersonalAudience(),
		Redacted:        true,
	})
	if err != nil {
		t.Fatalf("narrow to personal: %v", err)
	}

	if reshared.Redacted || reshared.Audience != entity.PersonalAudience() {
		t.Fatalf("a personal memory citing a personal doc stayed redacted: %+v", reshared)
	}

	_, err = h.service.Reshare(context.Background(), h.workspaceID, written.Memory.ID, service.ReshareMemoryInput{
		ExpectedVersion: reshared.Version,
		Audience:        entity.WorkspaceAudience(),
	})
	if !errors.Is(err, entity.ErrKnowledgeNotShareable) {
		t.Fatalf("widening again without redaction got %v, want ErrKnowledgeNotShareable", err)
	}
}

func TestAnAgentCannotRedactAMemoryWiderThanItsSources(t *testing.T) {
	h := newHarness(t)
	h.actAsAgent(entity.AgentScopeMember)

	input := content("Use SQLite")
	input.Evidence = []entity.MemoryEvidence{h.personalDoc()}

	_, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
		Redacted:      true,
	})
	if !errors.Is(err, entity.ErrMemoryRedactionForbidden) {
		t.Fatalf("an agent's redaction got %v, want ErrMemoryRedactionForbidden", err)
	}
}

func TestAnAgentCannotAddANarrowerSourceToARedactedMemory(t *testing.T) {
	h := newHarness(t)
	first := h.personalDoc()
	input := content("Use SQLite")
	input.Evidence = []entity.MemoryEvidence{first}

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: input,
		Audience:      entity.WorkspaceAudience(),
		Redacted:      true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	h.actAsAgent(entity.AgentScopeMember)

	h.correct(t, written.Memory, "Use SQLite for prototypes", first)

	corrected, err := h.service.Get(context.Background(), h.workspaceID, written.Memory.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	input = content("Use SQLite everywhere")
	input.Evidence = []entity.MemoryEvidence{first, h.personalDoc()}

	_, err = h.service.Correct(context.Background(), h.workspaceID, written.Memory.ID, service.CorrectMemoryInput{
		MemoryContent:   input,
		ExpectedVersion: corrected.Version,
	})
	if !errors.Is(err, entity.ErrMemoryRedactionForbidden) {
		t.Fatalf("an agent citing a new personal doc got %v, want ErrMemoryRedactionForbidden", err)
	}
}
