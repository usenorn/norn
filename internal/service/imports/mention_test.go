package imports_test

import (
	"context"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestAnUndatedImportedIssueIsMarkedImportedSoItsMentionsRelateNothing(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.offering(&staticSource{
		kind: "undated",
		held: map[entity.ImportResource][]entity.ImportRecord{
			entity.ImportIssue: {
				undated(t, sourceIssueHub, service.ImportIssuePayload{
					Title: "Rework the hub", Description: "Blocked by PROJ-2.", Team: sourceTeam,
				}),
			},
		},
	})

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)
	executing(t, h)

	if len(made.issues) != 1 {
		t.Fatalf("created %d issues, want 1", len(made.issues))
	}

	created := made.issues[0]

	if entity.OriginAttributed(created.Origin) {
		t.Fatal("the undated record arrived attributed, so this no longer covers an import without source dates")
	}

	if !created.Imported {
		t.Fatal("the issue was created without being marked imported, so its description's mentions would be related")
	}
}

func TestAnImportRewritingADescriptionMarksTheRewriteImported(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Core")

	h.scopedTo(team.ID)

	made := illustrated(t, h, team)

	executing(t, h)

	if len(made.rewrites) == 0 {
		t.Fatal("the import rewrote no description, so this covers nothing")
	}

	for _, rewrite := range made.rewrites {
		if !rewrite.Imported {
			t.Fatalf("a description rewrite %+v was not marked imported", rewrite)
		}
	}
}
