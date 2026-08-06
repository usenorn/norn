package webhook_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

type settlement struct {
	deliveryID uuid.UUID
	state      entity.WebhookDeliveryState
}

func (h *harness) capturesSettlements() *[]settlement {
	settled := new([]settlement)

	h.deliveries.EXPECT().
		Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, deliveryID uuid.UUID, state entity.WebhookDeliveryState, _ time.Time,
		) error {
			*settled = append(*settled, settlement{deliveryID: deliveryID, state: state})

			return nil
		}).
		AnyTimes()

	return settled
}

func (h *harness) capturesReschedules() *[]uuid.UUID {
	rescheduled := new([]uuid.UUID)

	h.deliveries.EXPECT().
		Reschedule(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, deliveryID uuid.UUID, _ time.Time) error {
			*rescheduled = append(*rescheduled, deliveryID)

			return nil
		}).
		AnyTimes()

	return rescheduled
}

func (h *harness) capturesAttempts() *[]entity.WebhookAttempt {
	recorded := new([]entity.WebhookAttempt)

	h.deliveries.EXPECT().
		RecordAttempt(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, attempt entity.WebhookAttempt) error {
			*recorded = append(*recorded, attempt)

			return nil
		}).
		AnyTimes()

	return recorded
}

func (h *harness) capturesFailures(streak int) *[]uuid.UUID {
	counted := new([]uuid.UUID)

	h.webhooks.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, webhookID uuid.UUID) (int, error) {
			*counted = append(*counted, webhookID)

			return streak, nil
		}).
		AnyTimes()

	return counted
}

func (h *harness) capturesSends(response entity.WebhookResponse) *[]entity.WebhookRequest {
	sent := new([]entity.WebhookRequest)

	h.sender.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.WebhookRequest) entity.WebhookResponse {
			*sent = append(*sent, request)

			return response
		}).
		AnyTimes()

	return sent
}

func (h *harness) delivers(hook entity.Webhook, delivery entity.WebhookDelivery, attempt int) {
	h.deliveries.EXPECT().ClaimAttempt(gomock.Any(), delivery.ID, attempt).Return(delivery, nil)
	h.webhooks.EXPECT().GetForDelivery(gomock.Any(), hook.ID).Return(hook, nil)
}

func TestAFailedAttemptShortOfTheLadderIsRescheduledRatherThanGivenUpOn(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, 1)

	h.delivers(hook, delivery, 0)
	h.capturesSends(refusedResponse())
	h.capturesAttempts()

	settled := h.capturesSettlements()
	rescheduled := h.capturesReschedules()
	failures := h.capturesFailures(1)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    0,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*settled) != 0 {
		t.Fatalf(
			"the delivery was settled as %q on its first refusal. A receiver that is briefly down "+
				"must be tried again down the ladder, not written off after one attempt.",
			(*settled)[0].state,
		)
	}

	if len(*rescheduled) != 1 {
		t.Fatalf("the delivery was rescheduled %d times, want exactly one retry", len(*rescheduled))
	}

	if len(*failures) != 0 {
		t.Error(
			"a single refusal counted against the subscription. Only a delivery that has exhausted " +
				"the whole ladder is evidence the destination is broken.",
		)
	}

	if len(h.woken) != 1 {
		t.Fatalf("the retry queued %d jobs, want exactly one", len(h.woken))
	}

	if h.woken[0].payload.Attempt != delivery.Attempt {
		t.Errorf(
			"the queued retry carries attempt %d but the claim consumed attempt %d. A retry that "+
				"replays the attempt it just used would be refused by the claim and vanish.",
			h.woken[0].payload.Attempt, delivery.Attempt,
		)
	}

	if h.woken[0].payload.DeliveryID != delivery.ID {
		t.Errorf("the retry names delivery %s rather than %s", h.woken[0].payload.DeliveryID, delivery.ID)
	}
}

func TestExhaustingTheLadderSettlesTheDeliveryAndCountsAgainstTheSubscription(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, entity.WebhookMaxAttempts)

	h.delivers(hook, delivery, entity.WebhookMaxAttempts-1)
	h.capturesSends(refusedResponse())
	h.capturesAttempts()

	settled := h.capturesSettlements()
	rescheduled := h.capturesReschedules()
	failures := h.capturesFailures(1)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    entity.WebhookMaxAttempts - 1,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*rescheduled) != 0 {
		t.Fatal(
			"the delivery was rescheduled after its last permitted attempt, so it would sit in the " +
				"queue forever being claimed and refused",
		)
	}

	if len(*settled) != 1 || (*settled)[0].state != entity.WebhookDeliveryFailed {
		t.Fatalf(
			"the exhausted delivery settled as %v, want exactly one settlement in state %q so the "+
				"log shows the receiver was given up on",
			*settled, entity.WebhookDeliveryFailed,
		)
	}

	if len(*failures) != 1 || (*failures)[0] != hook.ID {
		t.Fatalf(
			"the exhausted delivery counted %d failures against the subscription, want one for %s. "+
				"Without it a permanently broken destination never accumulates a streak and is "+
				"never disabled.",
			len(*failures), hook.ID,
		)
	}
}

func TestSustainedFailureDisablesTheSubscriptionOnceAndOnlyTheWinnerTellsTheOwner(t *testing.T) {
	for name, won := range map[string]bool{
		"this worker won the race":    true,
		"another worker won the race": false,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
			delivery := claimedDelivery(hook, entity.WebhookMaxAttempts)

			h.delivers(hook, delivery, entity.WebhookMaxAttempts-1)
			h.capturesSends(refusedResponse())
			h.capturesAttempts()
			h.capturesSettlements()
			h.capturesFailures(entity.WebhookFailureLimit)
			h.deliveries.EXPECT().Attempts(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

			h.webhooks.EXPECT().
				Disable(gomock.Any(), hook.ID, entity.WebhookDisabledSustainedFault, gomock.Any()).
				Return(won, nil).
				Times(1)

			if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
				DeliveryID: delivery.ID,
				Attempt:    entity.WebhookMaxAttempts - 1,
			}); err != nil {
				t.Fatalf("Deliver: %v", err)
			}

			if !won {
				if len(h.mailed) != 0 {
					t.Fatalf(
						"%d notices went out although another worker had already disabled the "+
							"subscription. Every worker that loses the race would send its own "+
							"copy, so the owner is told once per concurrent delivery.",
						len(h.mailed),
					)
				}

				return
			}

			if len(h.mailed) != 1 {
				t.Fatalf(
					"disabling the subscription sent %d notices, want exactly one. Norn has stopped "+
						"delivering the owner's events and nothing else tells them.",
					len(h.mailed),
				)
			}

			if h.mailed[0].To != ownerEmail {
				t.Errorf("the notice went to %q rather than the subscription's owner %q", h.mailed[0].To, ownerEmail)
			}

			if !strings.Contains(h.mailed[0].PlainBody, hook.URL) {
				t.Error("the notice does not name the destination that stopped answering, so the owner cannot act on it")
			}
		})
	}
}

func TestASuccessSettlesTheDeliveryAndClearsTheFailureStreak(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, 1)

	h.delivers(hook, delivery, 0)
	h.capturesSends(acceptedResponse())
	h.capturesAttempts()

	settled := h.capturesSettlements()
	failures := h.capturesFailures(1)

	h.webhooks.EXPECT().RecordSuccess(gomock.Any(), hook.ID, gomock.Any()).Return(nil).Times(1)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    0,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*settled) != 1 || (*settled)[0].state != entity.WebhookDeliverySucceeded {
		t.Fatalf("an accepted delivery settled as %v, want one settlement in state %q", *settled, entity.WebhookDeliverySucceeded)
	}

	if len(*failures) != 0 {
		t.Error(
			"an accepted delivery counted against the subscription. A destination that answers is " +
				"healthy however many attempts it took to get there.",
		)
	}
}

func TestAnInstanceWithNoEncryptionKeyDoesNotBlameTheReceiver(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, 1)

	h.deliveries.EXPECT().ClaimAttempt(gomock.Any(), delivery.ID, 0).Return(delivery, nil)
	h.webhooks.EXPECT().
		GetForDelivery(gomock.Any(), hook.ID).
		Return(entity.Webhook{}, entity.ErrWebhookEncryptionKeyMissing)

	sent := h.capturesSends(acceptedResponse())
	recorded := h.capturesAttempts()
	settled := h.capturesSettlements()
	rescheduled := h.capturesReschedules()
	failures := h.capturesFailures(1)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    0,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*sent) != 0 {
		t.Fatal(
			"an unsignable delivery was sent anyway. A receiver that cannot verify the signature " +
				"has to treat the request as forged, so sending it is worse than not sending it.",
		)
	}

	if len(*recorded) != 1 {
		t.Fatalf("the unsignable delivery recorded %d attempts, want one so the log explains the silence", len(*recorded))
	}

	if (*recorded)[0].Outcome != entity.WebhookAttemptRefused {
		t.Errorf(
			"the attempt was recorded as %q. The instance has no key; recording it as anything the "+
				"receiver did would send an administrator to debug the wrong system.",
			(*recorded)[0].Outcome,
		)
	}

	if len(*settled)+len(*rescheduled) != 1 {
		t.Errorf(
			"the unsignable delivery was neither settled nor rescheduled exactly once, so it is " +
				"left claimed and nothing will pick it up again",
		)
	}

	if len(*failures) != 0 {
		t.Error(
			"a missing instance key counted against the subscription. That failure is ours, and " +
				"twenty of them would disable a destination that never did anything wrong.",
		)
	}
}

func TestRerunningAnAlreadyClaimedAttemptChangesNothing(t *testing.T) {
	h := newHarness(t)

	deliveryID := uuid.New()

	h.deliveries.EXPECT().
		ClaimAttempt(gomock.Any(), deliveryID, 2).
		Return(entity.WebhookDelivery{}, entity.ErrWebhookDeliveryNotFound)

	sent := h.capturesSends(acceptedResponse())
	recorded := h.capturesAttempts()
	settled := h.capturesSettlements()

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: deliveryID,
		Attempt:    2,
	}); err != nil {
		t.Fatalf(
			"Deliver error = %v, want nil. The job queue redelivers, so a second run of the same "+
				"attempt must be a no-op rather than a failure that is retried forever.",
			err,
		)
	}

	if len(*sent)+len(*recorded)+len(*settled)+len(h.woken) != 0 {
		t.Error("a losing claim still sent, recorded, settled or requeued something, so a redelivery duplicates the event")
	}
}

func TestADisabledSubscriptionSettlesItsDeliveryWithoutSending(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	hook.Enabled = false
	delivery := claimedDelivery(hook, 1)

	h.delivers(hook, delivery, 0)

	sent := h.capturesSends(acceptedResponse())
	settled := h.capturesSettlements()

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    0,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*sent) != 0 {
		t.Fatal(
			"a disabled subscription still received a request. Disabling is how an owner or Norn " +
				"stops the traffic, and a queue drained after the fact would ignore it.",
		)
	}

	if len(*settled) != 1 || (*settled)[0].state != entity.WebhookDeliveryFailed {
		t.Fatalf(
			"the delivery for a disabled subscription settled as %v, want one settlement in state "+
				"%q; left pending it would be retried by the rescue sweep for ever",
			*settled, entity.WebhookDeliveryFailed,
		)
	}
}

func TestTheOutgoingRequestIsSignedAndSelfDescribing(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, 3)

	h.delivers(hook, delivery, 2)
	h.capturesAttempts()
	h.capturesSettlements()

	sent := h.capturesSends(acceptedResponse())

	h.webhooks.EXPECT().RecordSuccess(gomock.Any(), hook.ID, gomock.Any()).Return(nil)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    2,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(*sent) != 1 {
		t.Fatalf("the delivery produced %d requests, want one", len(*sent))
	}

	request := (*sent)[0]

	if request.URL != hook.URL {
		t.Errorf("the request went to %q rather than the registered destination %q", request.URL, hook.URL)
	}

	timestamp, err := strconv.ParseInt(request.Headers["Norn-Timestamp"], 10, 64)
	if err != nil {
		t.Fatalf(
			"Norn-Timestamp %q is not a unix second, so a receiver cannot reject a replayed "+
				"request on age: %v",
			request.Headers["Norn-Timestamp"], err,
		)
	}

	if request.Headers["Norn-Delivery"] != delivery.ID.String() {
		t.Fatalf(
			"Norn-Delivery is %q rather than %s, so a receiver cannot recognise a redelivery of "+
				"work it has already done",
			request.Headers["Norn-Delivery"], delivery.ID,
		)
	}

	want := entity.SignWebhook(hook.Secret, time.Unix(timestamp, 0), delivery.ID, request.Body)

	if request.Headers["Norn-Signature"] != want {
		t.Fatalf(
			"Norn-Signature is %q but signing the body with the webhook's own secret, the sent "+
				"timestamp and the sent delivery id gives %q. A receiver following the "+
				"documentation would reject every delivery.",
			request.Headers["Norn-Signature"], want,
		)
	}

	if request.Headers["Norn-Event"] != string(delivery.Event) {
		t.Errorf(
			"Norn-Event is %q rather than %q, so a receiver has to parse the body before it can "+
				"route the request",
			request.Headers["Norn-Event"], delivery.Event,
		)
	}

	if request.Headers["Norn-Attempt"] != strconv.Itoa(delivery.Attempt) {
		t.Errorf(
			"Norn-Attempt is %q rather than %d, so a receiver cannot tell a retry from a new event",
			request.Headers["Norn-Attempt"], delivery.Attempt,
		)
	}
}

func TestAWebhookInsideItsRotationGraceSignsWithBothSecrets(t *testing.T) {
	h := newHarness(t)

	expires := time.Now().UTC().Add(time.Hour)
	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	hook.PreviousSecret = "nrnwhs_previous"
	hook.SecretExpiresAt = &expires

	delivery := claimedDelivery(hook, 1)

	h.delivers(hook, delivery, 0)
	h.capturesAttempts()
	h.capturesSettlements()

	sent := h.capturesSends(acceptedResponse())

	h.webhooks.EXPECT().RecordSuccess(gomock.Any(), hook.ID, gomock.Any()).Return(nil)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    0,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	request := (*sent)[0]
	signatures := strings.Split(request.Headers["Norn-Signature"], " ")

	if len(signatures) != 2 {
		t.Fatalf(
			"a webhook inside its rotation grace sent %d signatures, want 2. A receiver that has "+
				"not yet switched to the new secret would drop every delivery until the grace ends.",
			len(signatures),
		)
	}

	timestamp, err := strconv.ParseInt(request.Headers["Norn-Timestamp"], 10, 64)
	if err != nil {
		t.Fatalf("parse Norn-Timestamp %q: %v", request.Headers["Norn-Timestamp"], err)
	}

	for _, secret := range []string{hook.Secret, hook.PreviousSecret} {
		want := entity.SignWebhook(secret, time.Unix(timestamp, 0), delivery.ID, request.Body)

		if signatures[0] != want && signatures[1] != want {
			t.Errorf(
				"neither sent signature verifies against the secret %q, so a receiver holding it "+
					"is locked out mid-rotation",
				secret,
			)
		}
	}
}

func TestTheDisableNoticeNamesWhatTheReceiverActuallySaid(t *testing.T) {
	h := newHarness(t)

	hook := enabledWebhook(uuid.New(), uuid.New(), entity.WebhookIssueCreated)
	delivery := claimedDelivery(hook, entity.WebhookMaxAttempts)

	h.delivers(hook, delivery, entity.WebhookMaxAttempts-1)
	h.capturesSends(refusedResponse())
	h.capturesAttempts()
	h.capturesSettlements()
	h.capturesFailures(entity.WebhookFailureLimit)

	h.webhooks.EXPECT().
		Disable(gomock.Any(), hook.ID, entity.WebhookDisabledSustainedFault, gomock.Any()).
		Return(true, nil)

	h.deliveries.EXPECT().
		Attempts(gomock.Any(), delivery.ID).
		Return([]entity.WebhookAttempt{
			{Attempt: 1, Error: "an earlier error"},
			{Attempt: 2, Error: "connection reset by peer"},
		}, nil)

	if err := h.deliverer.Deliver(context.Background(), entity.WebhookDeliverPayload{
		DeliveryID: delivery.ID,
		Attempt:    entity.WebhookMaxAttempts - 1,
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(h.mailed) != 1 {
		t.Fatalf("%d notices went out, want exactly one", len(h.mailed))
	}

	if !strings.Contains(h.mailed[0].PlainBody, "connection reset by peer") {
		t.Fatalf(
			"the notice does not name the last error the receiver gave. Being told a "+
				"subscription was disabled without being told why leaves the owner to go "+
				"digging, which is the whole reason the delivery log exists:\n%s",
			h.mailed[0].PlainBody,
		)
	}
}
