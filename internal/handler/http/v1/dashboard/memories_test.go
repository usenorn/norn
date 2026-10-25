package dashboard_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	memorysvc "github.com/usenorn/norn/internal/service/memory"
)

func memoryEdge(t *testing.T) (*memorysvc.MockMemories, http.Handler) {
	t.Helper()

	ctrl := gomock.NewController(t)
	memories := memorysvc.NewMockMemories(ctrl)

	return memories, newEdge(ctrl, edgeServices{memories: memories})
}

func TestAMemoryIsRecordedFromItsContentAndReadsOutWithItsProvenance(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, projectID, issueID, agentID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	until := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)

	memories.EXPECT().Create(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ any, _ uuid.UUID, input service.CreateMemoryInput) (service.MemoryWrite, error) {
			if input.Audience != entity.WorkspaceAudience() || input.Kind != entity.MemoryKindConstraint ||
				input.Conditions != "local prototypes only" || input.Applicability.Component != "storage" ||
				input.Applicability.ProjectID == nil || *input.Applicability.ProjectID != projectID {
				t.Fatalf("the create request reached the service as %+v", input)
			}

			if input.EffectiveUntil == nil || !input.EffectiveUntil.Equal(until) || input.EffectiveFrom != nil {
				t.Fatalf("the effective dates reached the service as %v to %v", input.EffectiveFrom, input.EffectiveUntil)
			}

			if len(input.Evidence) != 2 || *input.Evidence[0].IssueID != issueID || input.Evidence[1].Excerpt != "it broke" {
				t.Fatalf("the evidence reached the service as %+v", input.Evidence)
			}

			return service.MemoryWrite{Memory: entity.Memory{
				ID:            uuid.New(),
				WorkspaceID:   workspaceID,
				Kind:          input.Kind,
				Statement:     input.Statement,
				Conditions:    input.Conditions,
				Audience:      input.Audience,
				Applicability: input.Applicability,
				Status:        entity.MemoryStatusProposed,
				Basis:         entity.MemoryBasisInference,
				Evidence:      input.Evidence,
				AuthorID:      uuid.New(),
				By:            entity.KnowledgeAttribution{AgentID: &agentID, Source: entity.RevisionSourceAgent},
				EffectiveFrom: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC),
				Version:       1,
			}}, nil
		})

	status, body := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/memories",
		`{"kind":"constraint","statement":"Use SQLite","conditions":"local prototypes only",
		  "audience":{"visibility":"workspace"},
		  "appliesTo":{"projectId":"`+projectID.String()+`","component":"storage"},
		  "effectiveUntil":"2027-01-31",
		  "evidence":[{"kind":"issue","issueId":"`+issueID.String()+`"},{"kind":"excerpt","excerpt":"it broke"}]}`,
	)

	if status != http.StatusCreated {
		t.Fatalf("create = %d %v", status, body)
	}

	memory, _ := body["memory"].(map[string]any)

	if memory["status"] != "proposed" || memory["basis"] != "inference" || memory["source"] != "agent" ||
		memory["agentId"] != agentID.String() || memory["effectiveFrom"] != "2026-10-24" {
		t.Fatalf("the memory read out as %v", body)
	}

	if evidence, ok := memory["evidence"].([]any); !ok || len(evidence) != 2 {
		t.Fatalf("the evidence read out as %v", memory["evidence"])
	}

	if contributors, ok := memory["contributorIds"].([]any); !ok || len(contributors) != 0 {
		t.Fatalf("contributors read out as %v, want an empty list", memory["contributorIds"])
	}

	related, relatedOK := body["related"].([]any)
	choices, choicesOK := body["choices"].([]any)

	if !relatedOK || len(related) != 0 || !choicesOK || len(choices) != 0 {
		t.Fatalf("a memory with nothing related read out with %v and %v, want empty lists", body["related"], body["choices"])
	}
}

func TestAHeldMemoryAnswersWithItsRelatedMemoriesAndChoices(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, draftID, existingID := uuid.New(), uuid.New(), uuid.New()

	memories.EXPECT().Related(gomock.Any(), workspaceID, draftID).Return(service.MemoryWrite{
		Memory: entity.Memory{ID: draftID, Kind: entity.MemoryKindDecision, Statement: "Production uses MySQL", Status: entity.MemoryStatusDisputed},
		Related: []entity.RelatedMemory{{
			Memory:   entity.Memory{ID: existingID, Kind: entity.MemoryKindDecision, Statement: "Production uses PostgreSQL", Status: entity.MemoryStatusAccepted},
			Relation: entity.MemoryRelationConflict,
		}},
		Choices: []entity.MemoryChoice{
			{Resolution: entity.MemoryResolutionIndependent},
			{Resolution: entity.MemoryResolutionSupersede, TargetID: &existingID},
			{Resolution: entity.MemoryResolutionNoChange},
		},
	}, nil)

	status, body := sendDoc(t, routes, http.MethodGet,
		"/workspaces/"+workspaceID.String()+"/memories/"+draftID.String()+"/related", "",
	)

	if status != http.StatusOK {
		t.Fatalf("related = %d %v", status, body)
	}

	related, _ := body["related"].([]any)
	first, _ := related[0].(map[string]any)
	firstMemory, _ := first["memory"].(map[string]any)

	if len(related) != 1 || first["relation"] != "conflict" || firstMemory["id"] != existingID.String() {
		t.Fatalf("related read out as %v", body["related"])
	}

	choices, _ := body["choices"].([]any)
	supersede, _ := choices[1].(map[string]any)

	if len(choices) != 3 || supersede["resolution"] != "supersede" || supersede["targetId"] != existingID.String() {
		t.Fatalf("choices read out as %v", body["choices"])
	}
}

func TestReconcilingReachesTheServiceWithItsTargetAndAnswersWithBoth(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, draftID, existingID := uuid.New(), uuid.New(), uuid.New()

	memories.EXPECT().
		Reconcile(gomock.Any(), workspaceID, draftID, service.ReconcileMemoryInput{
			Resolution:      entity.MemoryResolutionSupersede,
			ExpectedVersion: 1,
			Target:          &service.MemoryTarget{ID: existingID, ExpectedVersion: 4},
		}).
		Return(service.MemoryReconciliation{
			Memory: entity.Memory{ID: draftID, Status: entity.MemoryStatusAccepted, SupersedesID: &existingID, Version: 2},
			Target: &entity.Memory{ID: existingID, Status: entity.MemoryStatusSuperseded, SupersededByID: &draftID, Version: 5},
		}, nil)

	status, body := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/memories/"+draftID.String()+"/reconcile",
		`{"resolution":"supersede","expectedVersion":1,"target":{"memoryId":"`+existingID.String()+`","expectedVersion":4}}`,
	)

	memory, _ := body["memory"].(map[string]any)
	target, _ := body["target"].(map[string]any)

	if status != http.StatusOK || memory["supersedesId"] != existingID.String() || target["supersededById"] != draftID.String() {
		t.Fatalf("reconcile = %d %v", status, body)
	}
}

func TestReconcilingASettledMemoryIsAConflict(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, memoryID := uuid.New(), uuid.New()

	memories.EXPECT().Reconcile(gomock.Any(), workspaceID, memoryID, gomock.Any()).
		Return(service.MemoryReconciliation{}, entity.ErrMemoryNotPending)

	status, problem := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/memories/"+memoryID.String()+"/reconcile",
		`{"resolution":"independent","expectedVersion":2}`,
	)

	if status != http.StatusConflict || problem["code"] != "memory_not_pending" {
		t.Fatalf("reconcile = %d %v, want 409 memory_not_pending", status, problem)
	}
}

func TestMemoryRefusalsAnswerWithTheirConflict(t *testing.T) {
	for name, testCase := range map[string]struct {
		err     error
		code    string
		version any
	}{
		"a stale write":             {err: entity.MemoryStaleError{Version: 4}, code: "memory_stale", version: float64(4)},
		"a withdrawn memory":        {err: entity.ErrMemoryClosed, code: "memory_closed"},
		"a proposal being replaced": {err: entity.MemoryTransitionError{From: entity.MemoryStatusProposed, To: entity.MemoryStatusSuperseded}, code: "memory_transition_invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			memories, routes := memoryEdge(t)
			workspaceID, memoryID := uuid.New(), uuid.New()

			memories.EXPECT().Withdraw(gomock.Any(), workspaceID, memoryID, 3).Return(entity.Memory{}, testCase.err)

			status, problem := sendDoc(t, routes, http.MethodPost,
				"/workspaces/"+workspaceID.String()+"/memories/"+memoryID.String()+"/withdraw",
				`{"expectedVersion":3}`,
			)

			if status != http.StatusConflict || problem["code"] != testCase.code || problem["version"] != testCase.version {
				t.Fatalf("withdraw = %d %v, want 409 %s", status, problem, testCase.code)
			}
		})
	}
}

func TestAMemoryOutsideTheReadersAudienceIsNotFound(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, memoryID := uuid.New(), uuid.New()

	memories.EXPECT().Get(gomock.Any(), workspaceID, memoryID).Return(entity.Memory{}, entity.ErrMemoryNotFound)

	status, problem := sendDoc(t, routes, http.MethodGet,
		"/workspaces/"+workspaceID.String()+"/memories/"+memoryID.String(), "",
	)

	if status != http.StatusNotFound {
		t.Fatalf("get = %d %v, want 404", status, problem)
	}
}

func TestACorrectionByAnOutsiderIsForbiddenWithItsReason(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, memoryID := uuid.New(), uuid.New()

	memories.EXPECT().
		Correct(gomock.Any(), workspaceID, memoryID, service.CorrectMemoryInput{
			MemoryContent:   service.MemoryContent{Kind: entity.MemoryKindDecision, Statement: "Ship weekly"},
			ExpectedVersion: 2,
		}).
		Return(entity.Memory{}, entity.ErrKnowledgeNotEditable)

	status, problem := sendDoc(t, routes, http.MethodPut,
		"/workspaces/"+workspaceID.String()+"/memories/"+memoryID.String(),
		`{"expectedVersion":2,"kind":"decision","statement":"Ship weekly"}`,
	)

	if status != http.StatusForbidden || problem["reason"] != "knowledge_not_editable" {
		t.Fatalf("correct = %d %v, want 403 knowledge_not_editable", status, problem)
	}
}

func TestSupersedingAnswersWithTheReplacement(t *testing.T) {
	memories, routes := memoryEdge(t)
	workspaceID, memoryID, replacementID := uuid.New(), uuid.New(), uuid.New()

	memories.EXPECT().Supersede(gomock.Any(), workspaceID, memoryID, gomock.Any()).
		DoAndReturn(func(_ any, _, _ uuid.UUID, input service.SupersedeMemoryInput) (entity.Memory, error) {
			if input.ExpectedVersion != 3 || input.Replacement.Statement != "Use Postgres" ||
				input.Replacement.Audience != entity.WorkspaceAudience() {
				t.Fatalf("the supersede request reached the service as %+v", input)
			}

			return entity.Memory{
				ID:           replacementID,
				Kind:         entity.MemoryKindDecision,
				Statement:    "Use Postgres",
				Audience:     entity.WorkspaceAudience(),
				Status:       entity.MemoryStatusAccepted,
				Basis:        entity.MemoryBasisObservation,
				SupersedesID: &memoryID,
				Version:      1,
			}, nil
		})

	status, body := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/memories/"+memoryID.String()+"/supersede",
		`{"expectedVersion":3,"replacement":{"kind":"decision","statement":"Use Postgres","audience":{"visibility":"workspace"}}}`,
	)

	if status != http.StatusCreated || body["id"] != replacementID.String() || body["supersedesId"] != memoryID.String() {
		t.Fatalf("supersede = %d %v", status, body)
	}
}
