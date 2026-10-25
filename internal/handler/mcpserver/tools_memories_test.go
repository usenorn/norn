package mcpserver_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func memoryFor(scene docScene, statement string) entity.Memory {
	agentID, tokenID := uuid.New(), uuid.New()

	return entity.Memory{
		ID:            uuid.New(),
		WorkspaceID:   scene.workspace.ID,
		Kind:          entity.MemoryKindDecision,
		Statement:     statement,
		Rationale:     "the build broke on SQLite",
		Audience:      entity.WorkspaceAudience(),
		Applicability: entity.KnowledgeApplicability{ProjectID: &scene.project.ID, Component: "storage"},
		Status:        entity.MemoryStatusAccepted,
		Basis:         entity.MemoryBasisInference,
		AuthorID:      uuid.New(),
		AuthorName:    "Rae",
		By: entity.KnowledgeAttribution{
			AgentID: &agentID, TokenID: &tokenID, Source: entity.RevisionSourceAgent,
		},
		EffectiveFrom: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC),
		Version:       1,
		UpdatedAt:     time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
	}
}

type memoryResult struct {
	Memory struct {
		ID         string `json:"id"`
		Statement  string `json:"statement"`
		Status     string `json:"status"`
		Basis      string `json:"basis"`
		AuthorName string `json:"authorName"`
		Provenance struct {
			Source  string `json:"source"`
			AgentID string `json:"agentId"`
			TokenID string `json:"tokenId"`
		} `json:"provenance"`
		Evidence []struct {
			Kind    string `json:"kind"`
			IssueID string `json:"issueId"`
			Excerpt string `json:"excerpt"`
		} `json:"evidence"`
		SupersedesID string `json:"supersedesId"`
		Version      int    `json:"version"`
	} `json:"memory"`
	Lineage []struct {
		Statement string `json:"statement"`
		Status    string `json:"status"`
	} `json:"lineage"`
	Reminder string `json:"reminder"`
}

func TestRememberingRecordsEveryFieldTheAgentGave(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	issue := entity.Issue{ID: uuid.New(), WorkspaceID: scene.workspace.ID}
	revisionID := uuid.New()

	h.issues.EXPECT().GetByReference(gomock.Any(), scene.workspace.ID, "ENG-42").Return(issue, nil)
	h.docs.EXPECT().
		Revisions(gomock.Any(), scene.workspace.ID, scene.doc.ID, service.ListDocRevisionsInput{Limit: 1}).
		Return(service.DocHistory{Revisions: []entity.DocRevisionSummary{{ID: revisionID}}}, nil)

	var recorded service.CreateMemoryInput

	h.memories.EXPECT().Create(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.CreateMemoryInput) (service.MemoryWrite, error) {
			recorded = input

			return service.MemoryWrite{Memory: memoryFor(scene, input.Statement)}, nil
		})

	result := callTool(t, h, "norn_remember", map[string]any{
		"workspace":  "acme",
		"kind":       "Decision",
		"statement":  "Use Postgres for storage",
		"rationale":  "the build broke on SQLite",
		"conditions": "unless the service runs offline",
		"applies_to": "atlas",
		"component":  "storage",
		"basis":      "observation",
		"project":    "atlas",
		"evidence": []map[string]any{
			{"issue": "eng-42"},
			{"doc": scene.doc.ID.String()},
			{"excerpt": "SQLite locked under load"},
		},
	})
	if result.IsError {
		t.Fatalf("remember refused: %s", refusalText(t, result))
	}

	content := recorded.MemoryContent
	if content.Kind != entity.MemoryKindDecision || content.Statement != "Use Postgres for storage" ||
		content.Rationale != "the build broke on SQLite" || content.Conditions != "unless the service runs offline" ||
		content.Basis != entity.MemoryBasisObservation {
		t.Fatalf("the memory was recorded as %+v", content)
	}

	if recorded.Audience != entity.ProjectAudience(scene.project.ID) {
		t.Fatalf("the memory was shared with %+v, want the project", recorded.Audience)
	}

	if content.Applicability.ProjectID == nil || *content.Applicability.ProjectID != scene.project.ID ||
		content.Applicability.Component != "storage" {
		t.Fatalf("the memory applies to %+v", content.Applicability)
	}

	evidence := content.Evidence
	if len(evidence) != 3 ||
		evidence[0].Kind != entity.MemoryEvidenceIssue || *evidence[0].IssueID != issue.ID ||
		evidence[1].Kind != entity.MemoryEvidenceDocRevision || *evidence[1].DocRevisionID != revisionID ||
		evidence[2].Kind != entity.MemoryEvidenceExcerpt || evidence[2].Excerpt != "SQLite locked under load" {
		t.Fatalf("the evidence was recorded as %+v", evidence)
	}
}

func TestEvidenceNamingTwoSourcesAtOnceIsRefused(t *testing.T) {
	h := newHarness(t)
	docSceneFor(h)

	result := callTool(t, h, "norn_remember", map[string]any{
		"workspace": "acme",
		"kind":      "decision",
		"statement": "Use Postgres",
		"evidence":  []map[string]any{{"issue": "ENG-42", "excerpt": "and this"}},
	})

	expectRefusal(t, result, "invalid_input", "exactly one issue, comment, doc, or excerpt")
}

func TestAMemoryReadsOutAsWorkspaceDataWithItsStatusAuthorAndInference(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	memory := memoryFor(scene, "Use Postgres")
	issueID := uuid.New()
	memory.Evidence = []entity.MemoryEvidence{{Kind: entity.MemoryEvidenceIssue, IssueID: &issueID}}

	earlier := memoryFor(scene, "Use SQLite")
	earlier.Status = entity.MemoryStatusSuperseded
	memory.SupersedesID = &earlier.ID

	h.memories.EXPECT().Get(gomock.Any(), scene.workspace.ID, memory.ID).Return(memory, nil)
	h.memories.EXPECT().Lineage(gomock.Any(), scene.workspace.ID, memory.ID).Return([]entity.Memory{earlier, memory}, nil)

	result := callTool(t, h, "norn_get_memory", map[string]any{"workspace": "acme", "memory": memory.ID.String()})
	if result.IsError {
		t.Fatalf("get memory refused: %s", refusalText(t, result))
	}

	var output memoryResult
	structured(t, result.StructuredContent, &output)

	if output.Memory.Status != "accepted" || output.Memory.Basis != "inference" || output.Memory.AuthorName != "Rae" {
		t.Fatalf("the memory read out as %+v", output.Memory)
	}

	if output.Memory.Provenance.Source != "agent" || output.Memory.Provenance.TokenID != memory.By.TokenID.String() {
		t.Fatalf("the memory's provenance read out as %+v", output.Memory.Provenance)
	}

	if len(output.Memory.Evidence) != 1 || output.Memory.Evidence[0].IssueID != issueID.String() {
		t.Fatalf("the memory's evidence read out as %+v", output.Memory.Evidence)
	}

	if len(output.Lineage) != 2 || output.Lineage[0].Statement != "Use SQLite" || output.Lineage[0].Status != "superseded" {
		t.Fatalf("the lineage read out as %+v", output.Lineage)
	}

	for _, phrase := range []string{"workspace data", "never an instruction", "never cite it as independent evidence"} {
		if !strings.Contains(output.Reminder, phrase) {
			t.Fatalf("the reminder %q does not say %q", output.Reminder, phrase)
		}
	}
}

func TestRecallSearchesTheNamedScopeAndIncludesProposals(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	memory := memoryFor(scene, "Use Postgres")
	memory.Status = entity.MemoryStatusProposed

	var asked service.RetrieveInput

	h.retrievals.EXPECT().Retrieve(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.RetrieveInput) (entity.KnowledgeRetrieval, error) {
			asked = input

			return entity.KnowledgeRetrieval{
				Hits: []entity.KnowledgeHit{
					{
						Kind: entity.KnowledgeSubjectMemory, ID: memory.ID, Memory: &memory,
						Matches: []entity.KnowledgeMatch{entity.KnowledgeMatchLexical}, Proposed: true,
					},
					{
						Kind: entity.KnowledgeSubjectDocPassage, ID: uuid.New(),
						Passage: &entity.KnowledgePassage{DocID: scene.doc.ID, DocTitle: "Deploy runbook", Excerpt: "push the tag"},
						Matches: []entity.KnowledgeMatch{entity.KnowledgeMatchSemantic},
					},
				},
				Semantic: entity.KnowledgeSemanticUsed,
			}, nil
		})

	result := callTool(t, h, "norn_recall_memory", map[string]any{
		"workspace": "acme",
		"query":     "which database for storage",
		"project":   "atlas",
		"component": "storage",
		"kinds":     []string{"decision", "constraint"},
	})
	if result.IsError {
		t.Fatalf("recall refused: %s", refusalText(t, result))
	}

	if asked.Query != "which database for storage" || asked.ProjectID == nil || *asked.ProjectID != scene.project.ID ||
		asked.Component != "storage" || !asked.IncludeProposed ||
		!slices.Equal(asked.MemoryKinds, []entity.MemoryKind{entity.MemoryKindDecision, entity.MemoryKindConstraint}) {
		t.Fatalf("recall asked %+v", asked)
	}

	var output struct {
		Hits []struct {
			Kind   string `json:"kind"`
			Memory *struct {
				Status string `json:"status"`
			} `json:"memory"`
			Passage *struct {
				DocTitle string `json:"docTitle"`
			} `json:"passage"`
		} `json:"hits"`
		Semantic string `json:"semantic"`
		Reminder string `json:"reminder"`
	}
	structured(t, result.StructuredContent, &output)

	if len(output.Hits) != 2 || output.Hits[0].Memory == nil || output.Hits[0].Memory.Status != "proposed" ||
		output.Hits[1].Passage == nil || output.Hits[1].Passage.DocTitle != "Deploy runbook" {
		t.Fatalf("recall returned %+v", output.Hits)
	}

	if output.Semantic != "used" || !strings.Contains(output.Reminder, "proposed or disputed one is still open") {
		t.Fatalf("recall said semantic %q and reminded %q", output.Semantic, output.Reminder)
	}
}

func TestListingActiveMemoriesCanLeaveOutProposals(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	var asked service.EnumerateInput

	h.retrievals.EXPECT().Enumerate(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.EnumerateInput) (service.KnowledgeList, error) {
			asked = input

			return service.KnowledgeList{Memories: []entity.Memory{memoryFor(scene, "Use Postgres")}, NextCursor: "next"}, nil
		})

	result := callTool(t, h, "norn_list_memories", map[string]any{
		"workspace":     "acme",
		"project":       "atlas",
		"accepted_only": true,
		"cursor":        "page-two",
	})
	if result.IsError {
		t.Fatalf("list memories refused: %s", refusalText(t, result))
	}

	if asked.IncludeProposed || asked.Cursor != "page-two" || asked.ProjectID == nil || *asked.ProjectID != scene.project.ID {
		t.Fatalf("listing asked %+v", asked)
	}

	var output struct {
		Memories   []struct{ Statement string } `json:"memories"`
		NextCursor string                       `json:"nextCursor"`
	}
	structured(t, result.StructuredContent, &output)

	if len(output.Memories) != 1 || output.NextCursor != "next" {
		t.Fatalf("listing returned %+v", output)
	}
}

func TestSupersedingRetiresTheNamedMemoryAtItsVersion(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	replacedID := uuid.New()

	var asked service.SupersedeMemoryInput

	h.memories.EXPECT().Supersede(gomock.Any(), scene.workspace.ID, replacedID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, input service.SupersedeMemoryInput) (entity.Memory, error) {
			asked = input
			replacement := memoryFor(scene, input.Replacement.Statement)
			replacement.SupersedesID = &replacedID

			return replacement, nil
		})

	result := callTool(t, h, "norn_supersede_memory", map[string]any{
		"workspace":        "acme",
		"memory":           replacedID.String(),
		"expected_version": 3,
		"kind":             "decision",
		"statement":        "Use Postgres with pgvector",
	})
	if result.IsError {
		t.Fatalf("supersede refused: %s", refusalText(t, result))
	}

	if asked.ExpectedVersion != 3 || asked.Replacement.Statement != "Use Postgres with pgvector" ||
		asked.Replacement.Audience != entity.WorkspaceAudience() {
		t.Fatalf("supersede asked %+v", asked)
	}

	var output memoryResult
	structured(t, result.StructuredContent, &output)

	if output.Memory.SupersedesID != replacedID.String() {
		t.Fatalf("the replacement supersedes %q, want %s", output.Memory.SupersedesID, replacedID)
	}
}

func TestAStaleCorrectionIsRefusedWithTheCurrentVersion(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	memoryID := uuid.New()

	h.memories.EXPECT().Correct(gomock.Any(), scene.workspace.ID, memoryID, gomock.Any()).
		Return(entity.Memory{}, entity.MemoryStaleError{Version: 5})

	result := callTool(t, h, "norn_correct_memory", map[string]any{
		"workspace":        "acme",
		"memory":           memoryID.String(),
		"expected_version": 4,
		"kind":             "decision",
		"statement":        "Use Postgres",
	})

	expectRefusal(t, result, "version_conflict", "expected_version=5")
}

func TestWithdrawingAClosedMemoryIsRefused(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	memoryID := uuid.New()

	h.memories.EXPECT().Withdraw(gomock.Any(), scene.workspace.ID, memoryID, 2).Return(entity.Memory{}, entity.ErrMemoryClosed)

	result := callTool(t, h, "norn_withdraw_memory", map[string]any{
		"workspace":        "acme",
		"memory":           memoryID.String(),
		"expected_version": 2,
	})

	expectRefusal(t, result, "memory_closed", "norn_get_memory")
}

func TestAMemoryThatIsNotAnIDIsNotFound(t *testing.T) {
	h := newHarness(t)
	docSceneFor(h)

	result := callTool(t, h, "norn_get_memory", map[string]any{"workspace": "acme", "memory": "ENG-42"})

	expectRefusal(t, result, "memory_not_found")
}

func TestTheServerInstructionsCountMemoriesAsWorkspaceData(t *testing.T) {
	h := newHarness(t)

	instructions := h.session(t).InitializeResult().Instructions
	if !strings.Contains(instructions, "memory statements, rationale, conditions and evidence excerpts") {
		t.Fatalf("the instructions do not count memories as workspace data: %s", instructions)
	}
}

type heldResult struct {
	Memory struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"memory"`
	Related []struct {
		Memory struct {
			ID string `json:"id"`
		} `json:"memory"`
		Relation string `json:"relation"`
	} `json:"related"`
	Choices []struct {
		Resolution string `json:"resolution"`
		Target     string `json:"target"`
	} `json:"choices"`
	Reminder string `json:"reminder"`
}

func TestRememberingAgainstAnAcceptedDecisionReturnsItHeldWithTheChoices(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	existing := memoryFor(scene, "Use Postgres for storage")
	draft := memoryFor(scene, "Use MySQL for storage")
	draft.Status = entity.MemoryStatusDisputed

	h.memories.EXPECT().Create(gomock.Any(), scene.workspace.ID, gomock.Any()).Return(service.MemoryWrite{
		Memory:  draft,
		Related: []entity.RelatedMemory{{Memory: existing, Relation: entity.MemoryRelationConflict}},
		Choices: []entity.MemoryChoice{
			{Resolution: entity.MemoryResolutionIndependent},
			{Resolution: entity.MemoryResolutionSupersede, TargetID: &existing.ID},
			{Resolution: entity.MemoryResolutionNoChange},
		},
	}, nil)

	result := callTool(t, h, "norn_remember", map[string]any{
		"workspace": "acme",
		"kind":      "decision",
		"statement": "Use MySQL for storage",
	})
	if result.IsError {
		t.Fatalf("remember refused: %s", refusalText(t, result))
	}

	var output heldResult
	structured(t, result.StructuredContent, &output)

	if output.Memory.Status != "disputed" || len(output.Related) != 1 ||
		output.Related[0].Memory.ID != existing.ID.String() || output.Related[0].Relation != "conflict" {
		t.Fatalf("remember returned %+v", output)
	}

	if len(output.Choices) != 3 || output.Choices[1].Resolution != "supersede" || output.Choices[1].Target != existing.ID.String() {
		t.Fatalf("remember offered %+v", output.Choices)
	}

	if !strings.Contains(output.Reminder, "held as disputed") || !strings.Contains(output.Reminder, "norn_reconcile_memory") {
		t.Fatalf("the reminder %q does not explain the hold", output.Reminder)
	}
}

func TestRememberingWithNothingRelatedSaysNothingAboutAHold(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	h.memories.EXPECT().Create(gomock.Any(), scene.workspace.ID, gomock.Any()).
		Return(service.MemoryWrite{Memory: memoryFor(scene, "Use Postgres")}, nil)

	result := callTool(t, h, "norn_remember", map[string]any{"workspace": "acme", "kind": "decision", "statement": "Use Postgres"})

	var output heldResult
	structured(t, result.StructuredContent, &output)

	if output.Related != nil || output.Choices != nil || strings.Contains(output.Reminder, "held as") {
		t.Fatalf("an accepted memory came back with %+v", output)
	}
}

func TestReconcilingPassesTheChoiceAndItsTargetThrough(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	existing := memoryFor(scene, "Use Postgres for storage")
	draft := memoryFor(scene, "Use MySQL for storage")

	h.memories.EXPECT().
		Reconcile(gomock.Any(), scene.workspace.ID, draft.ID, service.ReconcileMemoryInput{
			Resolution:      entity.MemoryResolutionSupersede,
			ExpectedVersion: 1,
			Target:          &service.MemoryTarget{ID: existing.ID, ExpectedVersion: 3},
		}).
		DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID, service.ReconcileMemoryInput) (service.MemoryReconciliation, error) {
			replacement, replaced := draft, existing
			replacement.SupersedesID = &existing.ID
			replaced.Status, replaced.SupersededByID = entity.MemoryStatusSuperseded, &draft.ID

			return service.MemoryReconciliation{Memory: replacement, Target: &replaced}, nil
		})

	result := callTool(t, h, "norn_reconcile_memory", map[string]any{
		"workspace":               "acme",
		"memory":                  draft.ID.String(),
		"expected_version":        1,
		"resolution":              "Supersede",
		"target":                  existing.ID.String(),
		"target_expected_version": 3,
	})
	if result.IsError {
		t.Fatalf("reconcile refused: %s", refusalText(t, result))
	}

	var output struct {
		Memory struct {
			SupersedesID string `json:"supersedesId"`
		} `json:"memory"`
		Target *struct {
			Status         string `json:"status"`
			SupersededByID string `json:"supersededById"`
		} `json:"target"`
	}
	structured(t, result.StructuredContent, &output)

	if output.Memory.SupersedesID != existing.ID.String() || output.Target == nil ||
		output.Target.Status != "superseded" || output.Target.SupersededByID != draft.ID.String() {
		t.Fatalf("reconcile returned %+v", output)
	}
}

func TestReconcilingASettledMemoryExplainsWhy(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	memoryID := uuid.New()

	h.memories.EXPECT().Reconcile(gomock.Any(), scene.workspace.ID, memoryID, gomock.Any()).
		Return(service.MemoryReconciliation{}, entity.ErrMemoryNotPending)

	result := callTool(t, h, "norn_reconcile_memory", map[string]any{
		"workspace":        "acme",
		"memory":           memoryID.String(),
		"expected_version": 2,
		"resolution":       "independent",
	})

	if !result.IsError || !strings.Contains(refusalText(t, result), "memory_not_pending") {
		t.Fatalf("reconcile answered %+v", result)
	}
}

func TestReadingAHeldMemoryShowsWhatItIsHeldAgainst(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	existing := memoryFor(scene, "Use Postgres for storage")
	draft := memoryFor(scene, "Use Postgres for storage.")
	draft.Status = entity.MemoryStatusProposed

	h.memories.EXPECT().Get(gomock.Any(), scene.workspace.ID, draft.ID).Return(draft, nil)
	h.memories.EXPECT().Lineage(gomock.Any(), scene.workspace.ID, draft.ID).Return([]entity.Memory{draft}, nil)
	h.memories.EXPECT().Related(gomock.Any(), scene.workspace.ID, draft.ID).Return(service.MemoryWrite{
		Memory:  draft,
		Related: []entity.RelatedMemory{{Memory: existing, Relation: entity.MemoryRelationDuplicate}},
		Choices: []entity.MemoryChoice{{Resolution: entity.MemoryResolutionNoChange}},
	}, nil)

	result := callTool(t, h, "norn_get_memory", map[string]any{"workspace": "acme", "memory": draft.ID.String()})
	if result.IsError {
		t.Fatalf("get refused: %s", refusalText(t, result))
	}

	var output heldResult
	structured(t, result.StructuredContent, &output)

	if len(output.Related) != 1 || output.Related[0].Relation != "duplicate" || len(output.Choices) != 1 ||
		!strings.Contains(output.Reminder, "held as proposed") {
		t.Fatalf("get returned %+v", output)
	}
}
