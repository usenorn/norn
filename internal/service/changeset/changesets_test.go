package changeset_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnIssueShowsOneRowPerRepositoryFromWhicheverRunTouchedItLast(t *testing.T) {
	h := newHarness(t)
	h.reading()

	h.changesets.EXPECT().
		ByIssue(gomock.Any(), h.workspaceID, h.issue.ID).
		Return(entity.IssueChangeSet{
			IssueID: h.issue.ID,
			Changes: []entity.IssueRepositoryChange{
				{
					ExecutionChange: entity.ExecutionChange{
						ExecutionID: "exec-01FIRST",
						Repository:  "backend",
						Branch:      "norn/NORN-38/backend",
					},
					Attempt: 1,
				},
				{
					ExecutionChange: entity.ExecutionChange{
						ExecutionID: "exec-01SECOND",
						Repository:  "frontend",
						Branch:      "norn/NORN-38-r2/frontend",
					},
					Attempt: 2,
				},
			},
		}, nil)

	changeset, err := h.service.ForIssue(context.Background(), h.workspaceID, h.issue.ID)
	if err != nil {
		t.Fatalf("read what the runs on an issue changed: %v", err)
	}

	if len(changeset.Changes) != 2 {
		t.Fatalf(
			"a re-run that touched a second repository produced %d rows; the first attempt's "+
				"still-open pull request has to stay visible on the issue",
			len(changeset.Changes),
		)
	}

	if changeset.Changes[0].Attempt != 1 || changeset.Changes[1].Attempt != 2 {
		t.Fatalf("the rows do not say which attempt each came from: %+v", changeset.Changes)
	}
}

func TestAnIssueInAnotherWorkspaceHandsBackNothing(t *testing.T) {
	h := newHarness(t)

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{}, nil)

	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, gomock.Any(), gomock.Any()).
		Return(entity.Issue{}, entity.ErrIssueNotFound)

	_, err := h.service.ForIssue(context.Background(), h.workspaceID, uuid.New())
	if !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf(
			"an issue the caller cannot see answered %v; a changeset has to be exactly as "+
				"visible as the issue it belongs to",
			err,
		)
	}
}
