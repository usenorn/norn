package webhook_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnEntryThatCannotSpreadDoesNotTakeTheRestOfTheBatchWithIt(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleMember, teams: nil},
	})

	h.membersEverywhere()

	broken := outboxEntry(workspaceID, uuid.Nil, entity.WebhookIssueCreated)
	healthy := outboxEntry(workspaceID, uuid.Nil, entity.WebhookIssueCreated)

	h.outbox.EXPECT().
		ClaimPending(gomock.Any(), fanOutBatch).
		Return([]entity.WebhookOutboxEntry{broken, healthy}, nil)

	h.webhooks.EXPECT().
		ListSubscribed(gomock.Any(), workspaceID, entity.WebhookIssueCreated).
		Return(nil, errors.New("subscriptions are unreadable"))

	h.webhooks.EXPECT().
		ListSubscribed(gomock.Any(), workspaceID, entity.WebhookIssueCreated).
		Return([]entity.Webhook{hook}, nil)

	captured := h.capturesQueue()

	queued, err := h.fanOut.FanOut(ctx)
	if err == nil {
		t.Fatal("FanOut reported success, want the failed entry reported")
	}

	if queued == 0 || len(*captured) == 0 {
		t.Error("FanOut queued nothing; the claimed batch was abandoned after the first failure")
	}
}
