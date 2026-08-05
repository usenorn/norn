package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

var configured = config.SMTP{Host: "localhost", FromAddress: "norn@example.test"}

func (h *harness) recipient() entity.NotificationDigestClaim {
	return entity.NotificationDigestClaim{
		WorkspaceID: h.workspaceID,
		AccountID:   h.readerID,
		Window:      entity.NotificationDigestWindowAt(time.Now().UTC()),
		Email:       "reader@example.test",
		Timezone:    "UTC",
	}
}

func TestNothingIsMailedOnAnInstanceWithNoSMTP(t *testing.T) {
	h := newHarness(t, config.SMTP{})

	if err := h.service.Digest(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("the digest failed rather than standing down: %v", err)
	}
}

func TestADigestIsClaimedBeforeItIsSent(t *testing.T) {
	h := newHarness(t, configured)
	recipient := h.recipient()

	gomock.InOrder(
		h.notifications.EXPECT().DigestRecipients(gomock.Any(), gomock.Any()).
			Return([]entity.NotificationDigestClaim{recipient}, nil),
		h.notifications.EXPECT().ClaimDigest(gomock.Any(), recipient).Return(true, nil),
		h.notifications.EXPECT().DigestEntries(gomock.Any(), h.workspaceID, h.readerID, gomock.Any()).
			Return([]entity.Notification{{
				Subject:     entity.NotifyIssue(h.issueID),
				Kind:        entity.NotificationKindCommented,
				Reason:      entity.NotificationReasonFollowing,
				Title:       "Payments retry twice",
				Reference:   "ENG-4",
				ActorName:   "Rae",
				UnreadCount: 3,
			}}, nil),
		h.workspaces.EXPECT().GetByID(gomock.Any(), h.workspaceID).
			Return(entity.Workspace{Name: "Northwind", Slug: "northwind"}, nil),
		h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil),
		h.notifications.EXPECT().RecordDigestOutcome(gomock.Any(), recipient, nil).Return(nil),
	)

	if err := h.service.Digest(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("sending the digest failed: %v", err)
	}
}

func TestARecipientAnotherWorkerAlreadyClaimedIsSkipped(t *testing.T) {
	h := newHarness(t, configured)
	recipient := h.recipient()

	h.notifications.EXPECT().DigestRecipients(gomock.Any(), gomock.Any()).
		Return([]entity.NotificationDigestClaim{recipient}, nil)
	h.notifications.EXPECT().ClaimDigest(gomock.Any(), recipient).Return(false, nil)

	if err := h.service.Digest(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("the digest failed: %v", err)
	}
}

func TestAFailedSendIsRecordedAndReturnedRatherThanSwallowed(t *testing.T) {
	h := newHarness(t, configured)
	recipient := h.recipient()
	refused := errors.New("dial tcp: connection refused")

	h.notifications.EXPECT().DigestRecipients(gomock.Any(), gomock.Any()).
		Return([]entity.NotificationDigestClaim{recipient}, nil)
	h.notifications.EXPECT().ClaimDigest(gomock.Any(), recipient).Return(true, nil)
	h.notifications.EXPECT().DigestEntries(gomock.Any(), h.workspaceID, h.readerID, gomock.Any()).
		Return([]entity.Notification{{
			Subject:   entity.NotifyIssue(h.issueID),
			Kind:      entity.NotificationKindCommented,
			Reference: "ENG-4",
		}}, nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), h.workspaceID).
		Return(entity.Workspace{Name: "Northwind", Slug: "northwind"}, nil)
	h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(refused)
	h.notifications.EXPECT().RecordDigestOutcome(gomock.Any(), recipient, refused).Return(nil)

	err := h.service.Digest(context.Background(), time.Now().UTC())
	if !errors.Is(err, refused) {
		t.Fatalf(
			"a failed send returned %v. Mail that cannot be delivered has to fail where "+
				"somebody sees it, not disappear into a log line while the task reports success.",
			err,
		)
	}
}
