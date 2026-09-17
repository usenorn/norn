package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func relatablePair(h *harness, scene issueScene) (entity.Issue, entity.Issue) {
	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 2, Title: "Retry the webhook", Version: 1,
	}
	other := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 1, Title: "Webhooks time out", Version: 1,
		Status: entity.IssueStatusActive,
		State:  entity.IssueState{ID: uuid.New(), Name: "Todo", Category: entity.StateCategoryNotStarted},
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, reference string) (entity.Issue, error) {
			if reference == other.Reference() {
				return other, nil
			}

			return issue, nil
		}).
		AnyTimes()

	return issue, other
}

func TestLinkingTwoIssuesRecordsTheKindAsSeenFromTheFirst(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	issue, other := relatablePair(h, scene)

	var added service.AddIssueRelationInput

	h.relations.EXPECT().
		Add(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.AddIssueRelationInput) (entity.IssueRelation, error) {
			added = in

			return entity.IssueRelation{ID: uuid.New(), Kind: in.Kind, Issue: other}, nil
		})

	result := callTool(t, h, "norn_link_issues", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "related": "gam-1", "kind": "Blocked_By",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if added.Kind != entity.IssueRelationViewBlockedBy || added.CounterpartID != other.ID {
		t.Fatalf("the relation was added as %+v, want GAM-2 blocked by GAM-1", added)
	}

	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	var output struct {
		Relation struct {
			Kind  string `json:"kind"`
			Issue struct {
				Reference string `json:"reference"`
			} `json:"issue"`
		} `json:"relation"`
	}

	if err := json.Unmarshal(payload, &output); err != nil {
		t.Fatalf("unmarshal %s: %v", payload, err)
	}

	if output.Relation.Kind != "blocked_by" || output.Relation.Issue.Reference != "GAM-1" {
		t.Fatalf("the tool answered %s, want blocked_by GAM-1", payload)
	}
}

func TestLinkingRefusesAKindItDoesNotKnowBeforeTouchingAnything(t *testing.T) {
	h := newHarness(t)

	expectRefusal(t, callTool(t, h, "norn_link_issues", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "related": "GAM-1", "kind": "depends_on",
	}), "relation_kind_invalid")
}

func TestLinkingAPairAlreadyRelatedSaysHow(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	issue, _ := relatablePair(h, scene)

	h.relations.EXPECT().
		Add(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		Return(entity.IssueRelation{}, entity.IssueRelationExistsError{
			Kind: entity.IssueRelationViewRelatesTo, Reference: "GAM-1",
		})

	expectRefusal(t, callTool(t, h, "norn_link_issues", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "related": "GAM-1", "kind": "blocks",
	}), "relation_exists", "related to GAM-1 as relates_to", "norn_unlink_issues")
}

func TestUnlinkingRemovesTheRelationHeldWithTheNamedIssue(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	issue, other := relatablePair(h, scene)

	bystander := entity.IssueRelation{ID: uuid.New(), Kind: entity.IssueRelationViewRelatesTo, Issue: entity.Issue{ID: uuid.New()}}
	held := entity.IssueRelation{ID: uuid.New(), Kind: entity.IssueRelationViewBlockedBy, Issue: other}

	h.related = []entity.IssueRelationGroup{
		{Kind: entity.IssueRelationViewBlockedBy, Relations: []entity.IssueRelation{held}},
		{Kind: entity.IssueRelationViewRelatesTo, Relations: []entity.IssueRelation{bystander}},
	}

	h.relations.EXPECT().Remove(gomock.Any(), scene.workspace.ID, issue.ID, held.ID).Return(nil)

	result := callTool(t, h, "norn_unlink_issues", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "related": "GAM-1",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}
}

func TestUnlinkingIssuesThatAreNotRelatedIsRefused(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	relatablePair(h, scene)

	expectRefusal(t, callTool(t, h, "norn_unlink_issues", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "related": "GAM-1",
	}), "relation_not_found")
}

func TestAnIssueIsReadWithTheIssuesItIsRelatedTo(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	_, other := relatablePair(h, scene)

	h.related = []entity.IssueRelationGroup{{
		Kind:      entity.IssueRelationViewBlockedBy,
		Relations: []entity.IssueRelation{{ID: uuid.New(), Kind: entity.IssueRelationViewBlockedBy, Issue: other}},
	}}

	result := callTool(t, h, "norn_get_issue", map[string]any{"workspace": "acme", "issue": "GAM-2"})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	var output struct {
		Relations []struct {
			Kind  string `json:"kind"`
			Issue struct {
				ID        string `json:"id"`
				Reference string `json:"reference"`
				Title     string `json:"title"`
				State     struct {
					Name string `json:"name"`
				} `json:"state"`
			} `json:"issue"`
		} `json:"relations"`
	}

	if err := json.Unmarshal(payload, &output); err != nil {
		t.Fatalf("unmarshal %s: %v", payload, err)
	}

	if len(output.Relations) != 1 {
		t.Fatalf("read %s, want the one relation GAM-2 holds", payload)
	}

	got := output.Relations[0]
	if got.Kind != "blocked_by" || got.Issue.ID != other.ID.String() || got.Issue.Reference != "GAM-1" ||
		got.Issue.Title != other.Title || got.Issue.State.Name != "Todo" {
		t.Fatalf("read %s, want GAM-2 blocked by GAM-1 with its title and state", payload)
	}
}

func TestAnIssueWhoseRelationsCannotBeReadIsNotReturnedWithoutThem(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)
	relatablePair(h, scene)

	h.relationsFail = errors.New("connection reset")

	expectRefusal(t, callTool(t, h, "norn_get_issue", map[string]any{
		"workspace": "acme", "issue": "GAM-2",
	}), "operation_failed")
}
