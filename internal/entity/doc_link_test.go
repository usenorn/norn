package entity_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestADocLinkPointsAtExactlyOneTarget(t *testing.T) {
	issue, project := uuid.New(), uuid.New()

	for name, testCase := range map[string]struct {
		issueID, projectID uuid.UUID
		target             entity.DocLinkTarget
		field, code        string
	}{
		"an issue":     {issueID: issue, target: entity.DocLinkTargetIssue},
		"a project":    {projectID: project, target: entity.DocLinkTargetProject},
		"neither":      {field: entity.DocLinkFieldIssueID, code: entity.ValidationCodeRequired},
		"both at once": {issueID: issue, projectID: project, field: entity.DocLinkFieldProjectID, code: entity.ValidationCodeUnsupportedValue},
	} {
		t.Run(name, func(t *testing.T) {
			target, err := entity.DocLinkTargetOf(testCase.issueID, testCase.projectID)

			if testCase.code == "" {
				if err != nil || target != testCase.target {
					t.Fatalf("got %q, %v; want %q", target, err, testCase.target)
				}

				return
			}

			var invalid entity.ValidationError
			if !errors.As(err, &invalid) || len(invalid.Fields) != 1 ||
				invalid.Fields[0] != (entity.FieldError{Field: testCase.field, Code: testCase.code}) {
				t.Fatalf("got %v, want %s: %s", err, testCase.field, testCase.code)
			}
		})
	}
}

func TestAStoredDocLinkNamesItsTarget(t *testing.T) {
	if got := (entity.StoredDocLink{IssueID: uuid.New()}).Target(); got != entity.DocLinkTargetIssue {
		t.Fatalf("issue link reported %q", got)
	}

	if got := (entity.StoredDocLink{ProjectID: uuid.New()}).Target(); got != entity.DocLinkTargetProject {
		t.Fatalf("project link reported %q", got)
	}
}
