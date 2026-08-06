package webhook_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func settledDelivery(hook entity.Webhook, state entity.WebhookDeliveryState) entity.WebhookDelivery {
	settledAt := time.Now().UTC()

	delivery := claimedDelivery(hook, entity.WebhookMaxAttempts)
	delivery.State = state
	delivery.TeamID = uuid.New()
	delivery.Body = []byte(`{"identifier":"NOR-14","title":"Ship the relay"}`)

	if state.Settled() {
		delivery.SettledAt = &settledAt
	}

	return delivery
}

func TestADeliveryStillInFlightCannotBeReplayed(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	pending := settledDelivery(hook, entity.WebhookDeliveryPending)

	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.webhooks.EXPECT().Get(gomock.Any(), hook.WorkspaceID, hook.ID).Return(hook, nil)
	h.deliveries.EXPECT().Get(gomock.Any(), hook.ID, pending.ID).Return(pending, nil)

	queued := h.capturesQueue()

	_, err := h.log.Replay(ctx, hook.WorkspaceID, hook.ID, pending.ID)

	if !errors.Is(err, entity.ErrWebhookDeliveryNotReplayable) {
		t.Fatalf(
			"Replay error = %v, want ErrWebhookDeliveryNotReplayable. A delivery still working its "+
				"way down the retry ladder will land on its own; replaying it now sends the same "+
				"event twice.",
			err,
		)
	}

	if len(*queued) != 0 {
		t.Fatal("a delivery in flight was replayed anyway")
	}
}

func TestReplayingAFailedDeliveryResendsTheOriginalBodyAndSaysWhatItReplays(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	failed := settledDelivery(hook, entity.WebhookDeliveryFailed)

	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.webhooks.EXPECT().Get(gomock.Any(), hook.WorkspaceID, hook.ID).Return(hook, nil)
	h.deliveries.EXPECT().Get(gomock.Any(), hook.ID, failed.ID).Return(failed, nil)

	queued := h.capturesQueue()

	replay, err := h.log.Replay(ctx, hook.WorkspaceID, hook.ID, failed.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if len(*queued) != 1 {
		t.Fatalf("the replay queued %d deliveries, want one", len(*queued))
	}

	queuedReplay := (*queued)[0]

	if queuedReplay.ID == failed.ID {
		t.Fatal(
			"the replay reuses the original delivery's identifier, so its attempts would be filed " +
				"against the run that already failed and the log would stop being a record",
		)
	}

	if queuedReplay.ReplayOf != failed.ID {
		t.Fatalf(
			"the replay names %s as its original rather than %s. Without it the log shows a second "+
				"delivery of the same event with nothing explaining why.",
			queuedReplay.ReplayOf, failed.ID,
		)
	}

	if !bytes.Equal(queuedReplay.Body, failed.Body) {
		t.Fatalf(
			"the replay carries %q but the original carried %q. A replay exists to send what was "+
				"missed, not what the workspace looks like now.",
			queuedReplay.Body, failed.Body,
		)
	}

	if queuedReplay.Event != failed.Event || queuedReplay.SubjectID != failed.SubjectID ||
		queuedReplay.TeamID != failed.TeamID {
		t.Errorf(
			"the replay describes event %q / subject %s / team %s, want %q / %s / %s",
			queuedReplay.Event, queuedReplay.SubjectID, queuedReplay.TeamID,
			failed.Event, failed.SubjectID, failed.TeamID,
		)
	}

	if replay.State != entity.WebhookDeliveryPending {
		t.Errorf("the replay was reported as %q rather than pending, so a caller cannot poll it", replay.State)
	}

	if len(h.woken) != 1 || h.woken[0].payload.DeliveryID != queuedReplay.ID {
		t.Errorf("the replay woke %d jobs, want one naming the new delivery", len(h.woken))
	}
}

func TestOnlyAnAdministratorMayReadOrDriveTheDeliveryLog(t *testing.T) {
	workspaceID, webhookID, deliveryID := uuid.New(), uuid.New(), uuid.New()

	reads := map[string]func(context.Context, *harness) error{
		"List": func(ctx context.Context, h *harness) error {
			_, err := h.log.List(ctx, workspaceID, webhookID, service.ListWebhookDeliveriesInput{})

			return err
		},
		"Get": func(ctx context.Context, h *harness) error {
			_, err := h.log.Get(ctx, workspaceID, webhookID, deliveryID)

			return err
		},
		"Test": func(ctx context.Context, h *harness) error {
			_, err := h.log.Test(ctx, workspaceID, webhookID)

			return err
		},
		"Replay": func(ctx context.Context, h *harness) error {
			_, err := h.log.Replay(ctx, workspaceID, webhookID, deliveryID)

			return err
		},
	}

	for name, call := range reads {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			ctx := h.actingAs(entity.MembershipRoleMember)

			queued := h.capturesQueue()

			if err := call(ctx, h); !errors.Is(err, entity.ErrWebhookNotPermitted) {
				t.Fatalf(
					"%s error = %v, want ErrWebhookNotPermitted. The delivery log holds the bodies "+
						"of every event the subscription carried, so reading it is as privileged "+
						"as registering one.",
					name, err,
				)
			}

			if len(*queued) != 0 {
				t.Errorf("%s queued a delivery for a caller who was refused", name)
			}
		})
	}
}

func TestAPageIsTrimmedToItsLimitAndOnlyCarriesACursorWhenMoreRemain(t *testing.T) {
	for name, probe := range map[string]struct {
		rows   int
		cursor bool
	}{
		"a full page with more behind it": {rows: 6, cursor: true},
		"the last page":                   {rows: 5, cursor: false},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			const limit = 5

			hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
			ctx := h.actingAs(entity.MembershipRoleAdmin)

			h.webhooks.EXPECT().Get(gomock.Any(), hook.WorkspaceID, hook.ID).Return(hook, nil)

			found := make([]entity.WebhookDelivery, 0, probe.rows)
			for range probe.rows {
				found = append(found, settledDelivery(hook, entity.WebhookDeliveryFailed))
			}

			h.deliveries.EXPECT().
				List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(
					_ context.Context, filter entity.WebhookDeliveryFilter,
				) ([]entity.WebhookDelivery, error) {
					if filter.Limit != limit+1 {
						t.Errorf(
							"the log asked the store for %d rows on a page of %d. It has to read "+
								"one past the page to know whether a cursor is owed, or the last "+
								"page hands out a cursor onto nothing.",
							filter.Limit, limit,
						)
					}

					return found, nil
				})

			page, err := h.log.List(ctx, hook.WorkspaceID, hook.ID, service.ListWebhookDeliveriesInput{Limit: limit})
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			want := min(probe.rows, limit)

			if len(page.Deliveries) != want {
				t.Fatalf(
					"the page carries %d deliveries, want %d. The lookahead row is how the log "+
						"knows more remain; returning it would show the caller a row twice.",
					len(page.Deliveries), want,
				)
			}

			if (page.NextCursor != "") != probe.cursor {
				t.Fatalf(
					"the page reports a next cursor = %v, want %v. A cursor on the last page walks "+
						"the caller into an empty request; none on a full page hides the rest.",
					page.NextCursor != "", probe.cursor,
				)
			}

			if !probe.cursor {
				return
			}

			cursor, err := entity.DecodeWebhookDeliveryCursor(page.NextCursor)
			if err != nil {
				t.Fatalf("the page's own cursor does not decode: %v", err)
			}

			if cursor.ID != page.Deliveries[len(page.Deliveries)-1].ID {
				t.Errorf(
					"the cursor points at %s rather than the last row returned, so the next page "+
						"skips or repeats work",
					cursor.ID,
				)
			}
		})
	}
}

func TestATestDeliveryIsSentAsTheTestEventAndNothingElse(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	ctx := h.actingAs(entity.MembershipRoleAdmin)

	h.webhooks.EXPECT().Get(gomock.Any(), hook.WorkspaceID, hook.ID).Return(hook, nil)

	queued := h.capturesQueue()

	delivery, err := h.log.Test(ctx, hook.WorkspaceID, hook.ID)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}

	if len(*queued) != 1 {
		t.Fatalf("the test queued %d deliveries, want one", len(*queued))
	}

	if (*queued)[0].Event != entity.WebhookTest {
		t.Fatalf(
			"the test delivery went out as %q. It carries no workspace change, so sending it under "+
				"a real event name would make a receiver act on something that never happened.",
			(*queued)[0].Event,
		)
	}

	if delivery.WebhookID != hook.ID || delivery.WorkspaceID != hook.WorkspaceID {
		t.Errorf("the test delivery names webhook %s in workspace %s, want %s in %s",
			delivery.WebhookID, delivery.WorkspaceID, hook.ID, hook.WorkspaceID)
	}

	if len(h.woken) != 1 {
		t.Errorf(
			"the test delivery woke %d jobs, want one. A test that waits for the rescue sweep is "+
				"useless to somebody watching the screen.",
			len(h.woken),
		)
	}
}
