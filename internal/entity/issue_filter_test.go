package entity_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAHostileFilterExpressionIsRefusedRatherThanCompiled(t *testing.T) {
	deep := entity.IssueFilter{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}}
	for range entity.IssueFilterMaxDepth + 2 {
		deep = entity.IssueFilter{All: []entity.IssueFilter{deep}}
	}

	for name, filter := range map[string]entity.IssueFilter{
		"a field nobody defined": {
			Field: "workspace_id); DROP TABLE workspace_issues; --", Op: entity.IssueFilterOpIs, Values: []string{"x"},
		},
		"an operator the field does not support": {
			Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpHasAll, Values: []string{"high"},
		},
		"a value that is not the field's type": {
			Field: entity.IssueFilterFieldAssignee, Op: entity.IssueFilterOpIs, Values: []string{"not-a-uuid"},
		},
		"an enum value outside the enum": {
			Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"catastrophic"},
		},
		"a date that is not a date": {
			Field: entity.IssueFilterFieldDueOn, Op: entity.IssueFilterOpBefore, Values: []string{"soon"},
		},
		"two conditions in one node": {
			All: []entity.IssueFilter{{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}}},
			Not: &entity.IssueFilter{Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue},
		},
		"nested past the depth limit": deep,
		"a comparison given several values": {
			Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high", "low"},
		},
		"a value where none belongs": {
			Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue, Values: []string{"yes"},
		},
		"a set operator with nothing in the set": {
			Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIn,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := filter.Validate(); err == nil {
				t.Fatal(
					"the expression was accepted. A filter arrives from clients, tokens and agents, " +
						"so anything the validator waves through reaches the database.",
				)
			}
		})
	}
}

func TestAWellFormedFilterIsAccepted(t *testing.T) {
	filter := entity.IssueFilter{All: []entity.IssueFilter{
		{Field: entity.IssueFilterFieldStateCategory, Op: entity.IssueFilterOpIn, Values: []string{"not_started", "active"}},
		{Any: []entity.IssueFilter{
			{Field: entity.IssueFilterFieldAssignee, Op: entity.IssueFilterOpIs, Values: []string{uuid.New().String()}},
			{Field: entity.IssueFilterFieldAssignee, Op: entity.IssueFilterOpIsNotSet},
		}},
		{Not: &entity.IssueFilter{Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue}},
		{Field: entity.IssueFilterFieldLabel, Op: entity.IssueFilterOpHasAny, Values: []string{uuid.New().String()}},
		{Field: entity.IssueFilterFieldDueOn, Op: entity.IssueFilterOpBefore, Values: []string{"2026-09-01"}},
	}}

	if err := filter.Validate(); err != nil {
		t.Fatalf("a filter using every combining form was refused: %v", err)
	}
}

func TestAFilterSurvivesBeingStoredAndReadBackUnchanged(t *testing.T) {
	filter := entity.IssueFilter{Any: []entity.IssueFilter{
		{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIn, Values: []string{"urgent", "high"}},
		{Not: &entity.IssueFilter{Field: entity.IssueFilterFieldHasChildren, Op: entity.IssueFilterOpIsFalse}},
	}}

	stored, err := json.Marshal(filter)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored entity.IssueFilter

	if err := json.Unmarshal(stored, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(filter, restored) {
		t.Fatalf(
			"the expression changed on the way through storage:\nbefore %+v\nafter  %+v\n"+
				"A saved view is only its expression; if that does not round-trip exactly, the "+
				"view means something different tomorrow.",
			filter, restored,
		)
	}

	if err := restored.Validate(); err != nil {
		t.Fatalf("the restored expression no longer validates: %v", err)
	}
}

func TestSortingIsAlwaysBoundedAndTotal(t *testing.T) {
	if _, err := entity.NormalizedIssueSort([]entity.IssueSort{
		{Field: entity.IssueSortFieldCreatedAt},
		{Field: entity.IssueSortFieldUpdatedAt},
		{Field: entity.IssueSortFieldPriority},
		{Field: entity.IssueSortFieldDueOn},
	}); !errors.Is(err, entity.ErrIssueFilterTooComplex) {
		t.Errorf("sorting on more keys than the limit returned %v, want ErrIssueFilterTooComplex", err)
	}

	if _, err := entity.NormalizedIssueSort([]entity.IssueSort{
		{Field: entity.IssueSortFieldCreatedAt},
		{Field: entity.IssueSortFieldCreatedAt},
	}); err == nil {
		t.Error("the same sort key twice was accepted, which makes the second key meaningless")
	}

	if _, err := entity.NormalizedIssueSort([]entity.IssueSort{{Field: "title; DROP TABLE"}}); err == nil {
		t.Error("an unknown sort key was accepted")
	}

	sort, err := entity.NormalizedIssueSort(nil)
	if err != nil || len(sort) != 1 || sort[0].Field != entity.IssueSortFieldCreatedAt {
		t.Errorf("no sort did not fall back to the default: %v %v", sort, err)
	}
}

func TestAQueryCursorSurvivesEncoding(t *testing.T) {
	cursor := entity.IssueQueryCursor{Keys: []string{"2026-08-04T09:00:00Z", ""}, IssueID: uuid.New()}

	restored, err := entity.DecodeIssueQueryCursor(cursor.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !reflect.DeepEqual(cursor, restored) {
		t.Fatalf("the cursor changed: before %+v after %+v", cursor, restored)
	}

	for name, raw := range map[string]string{
		"not base64":    "!!!!",
		"not a cursor":  "aGVsbG8",
		"without an id": entity.IssueQueryCursor{Keys: []string{"a"}}.Encode(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := entity.DecodeIssueQueryCursor(raw); !errors.Is(err, entity.ErrIssueCursorInvalid) {
				t.Errorf("decoding %q returned %v, want ErrIssueCursorInvalid", raw, err)
			}
		})
	}
}

func TestAnEmptyKeyStandsForAMissingSortValue(t *testing.T) {
	keys := entity.IssueSortKeys(
		entity.Issue{},
		[]entity.IssueSort{{Field: entity.IssueSortFieldDueOn}, {Field: entity.IssueSortFieldEstimate}},
	)

	for i, key := range keys {
		if key != "" {
			t.Errorf(
				"key %d of an issue with no due date and no estimate is %q, want the empty string; "+
					"the empty key is what tells the keyset comparison the row sorted with the NULLs",
				i, key,
			)
		}
	}
}
