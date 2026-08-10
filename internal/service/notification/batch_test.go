package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

func TestAnEventThatCannotFanOutDoesNotTakeTheRestOfTheBatchWithIt(t *testing.T) {
	h := newHarness(t, config.SMTP{})

	unreadable := uuid.New()
	broken := h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser)
	broken.Subject = entity.NotifyIssue(unreadable)

	h.expectPendingEvents(broken, h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser))

	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, unreadable, gomock.Any()).
		Return(entity.Issue{}, errors.New("issue is unreadable"))

	h.expectIssueOnTeam()
	h.expectFollowers(h.readerID)
	h.expectAudience(h.readerID)
	h.expectSettings()

	captured := h.captureDeliveries()

	delivered, err := h.service.FanOut(context.Background())
	if err == nil {
		t.Fatal("FanOut reported success, want the failed event reported")
	}

	if delivered == 0 {
		t.Fatal("FanOut delivered nothing, want the healthy event still delivered")
	}

	if _, ok := deliveryTo(*captured, h.readerID); !ok {
		t.Errorf("the reader was not delivered to; the claimed batch was abandoned after the first failure")
	}
}
