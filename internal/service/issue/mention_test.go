package issue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type mention struct {
	subject uuid.UUID
	text    string
}

func (h *harness) expectDescriptionWrites() {
	h.revisions.EXPECT().
		Latest(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.IssueDescriptionRevision{}, entity.ErrIssueRevisionNotFound).
		AnyTimes()
	h.revisions.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.followers.EXPECT().Follow(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()
}

func TestANewIssueRelatesTheIssuesItsDescriptionMentions(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()

	h.expectRaising(workspaceID, teamID)
	h.expectDescriptionWrites()

	created, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Retry the webhook",
		Description: "Follows on from MOB-7.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(h.mentioned) != 1 || h.mentioned[0].subject != created.ID || h.mentioned[0].text != "Follows on from MOB-7." {
		t.Fatalf("mentions related %+v, want the new issue's description once", h.mentioned)
	}
}

func TestAnImportedIssueLeavesItsMentionsToTheImportedRelationsEvenWithoutSourceDates(t *testing.T) {
	h := newHarness(t)

	workspaceID, teamID := uuid.New(), uuid.New()
	origin := entity.NewImportOrigin(time.Time{}, time.Time{}, uuid.Nil)

	h.expectRaising(workspaceID, teamID)
	h.expectDescriptionWrites()

	if _, err := h.service.Create(context.Background(), service.CreateIssueInput{
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		Title:       "Retry the webhook",
		Description: "Blocked by PROJ-2.",
		Origin:      &origin,
		Imported:    true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(h.mentioned) != 0 {
		t.Fatalf("an import related %+v; its relations arrive as their own records", h.mentioned)
	}
}

func (h *harness) describable(t *testing.T) (uuid.UUID, entity.Issue) {
	t.Helper()

	workspaceID, issueID, issue := h.editable(t)

	h.actor = entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}
	h.expectDescriptionWrites()
	h.expectStateWrite(issueID)
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return workspaceID, issue
}

func TestRewritingADescriptionRelatesTheIssuesItNowMentions(t *testing.T) {
	h := newHarness(t)
	workspaceID, issue := h.describable(t)

	rewritten := "Retries drop the key, see MOB-7."

	if _, err := h.service.Update(context.Background(), workspaceID, issue.ID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		Description:     &rewritten,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(h.mentioned) != 1 || h.mentioned[0].subject != issue.ID || h.mentioned[0].text != rewritten {
		t.Fatalf("mentions related %+v, want the rewritten description once", h.mentioned)
	}
}

func TestAnImportRewritingADescriptionRelatesNothing(t *testing.T) {
	h := newHarness(t)
	workspaceID, issue := h.describable(t)

	rewritten := "Blocked by PROJ-2, pictured at /files/hub.png."

	if _, err := h.service.Update(context.Background(), workspaceID, issue.ID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		Description:     &rewritten,
		Imported:        true,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(h.mentioned) != 0 {
		t.Fatalf("an import's rewrite related %+v; its relations arrive as their own records", h.mentioned)
	}
}

func TestAnEditThatLeavesTheDescriptionAloneRelatesNothing(t *testing.T) {
	h := newHarness(t)
	workspaceID, issue := h.describable(t)

	renamed := "Retries drop the idempotency key, see MOB-7"

	if _, err := h.service.Update(context.Background(), workspaceID, issue.ID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		Title:           &renamed,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(h.mentioned) != 0 {
		t.Fatalf("a rename related %+v; only the description is read for mentions", h.mentioned)
	}
}

func TestAHeldDescriptionRelatesNothingUntilItIsApproved(t *testing.T) {
	h := newHarness(t)
	workspaceID, issueID, _ := h.editable(t)

	h.holding(entity.AgentSettings{HoldIssueEdits: entity.AgentHoldAlways})
	h.expectHeld(t)

	rewritten := "Retries drop the key, see MOB-7."

	_, err := h.service.Update(context.Background(), workspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: 1,
		Description:     &rewritten,
	})

	var waiting entity.AgentActionHeldError
	if !errors.As(err, &waiting) {
		t.Fatalf("Update returned %v, want the edit held", err)
	}

	if len(h.mentioned) != 0 {
		t.Fatalf("a held edit related %+v before anybody approved it", h.mentioned)
	}
}
