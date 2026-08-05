package event_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	eventstreamrepo "github.com/usenorn/norn/internal/repository/eventstream"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
)

type harness struct {
	stream     *eventstreamrepo.MockEventStream
	authorizer *authorizersvc.MockAuthorizer
	service    service.Events

	workspaceID uuid.UUID
	accountID   uuid.UUID
	openTeam    uuid.UUID
	closedTeam  uuid.UUID

	published chan entity.Event
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		stream:      eventstreamrepo.NewMockEventStream(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		accountID:   uuid.New(),
		openTeam:    uuid.New(),
		closedTeam:  uuid.New(),
		published:   make(chan entity.Event, 32),
	}

	h.service = eventsvc.New(h.stream, h.authorizer)

	h.stream.EXPECT().Latest(gomock.Any(), gomock.Any()).Return("0-0", nil).AnyTimes()
	h.scopedTo(h.openTeam)

	return h
}

func (h *harness) scopedTo(teams ...uuid.UUID) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{AccountID: h.accountID, Kind: entity.ActorKindUser},
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, TeamIDs: teams},
		}, nil).
		AnyTimes()
}

// feed makes the mocked stream hand the hub one batch and then block, the way a real blocking
// XREAD sits idle between changes.
func (h *harness) feed(events ...entity.Event) {
	batch := events
	h.stream.EXPECT().
		Read(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID, cursor string) ([]entity.Event, string, error) {
			if len(batch) == 0 {
				<-ctx.Done()

				return nil, cursor, ctx.Err()
			}

			sending := batch
			batch = nil

			return sending, sending[len(sending)-1].ID, nil
		}).
		AnyTimes()
}

// live hands the hub one batch per send on the returned channel, the way a real blocking XREAD
// returns each change as it lands. feed's single batch is delivered in one uninterrupted loop, so a
// reader cannot drain part way through it and every subscriber looks equally stalled.
func (h *harness) live() chan<- []entity.Event {
	batches := make(chan []entity.Event)

	h.stream.EXPECT().
		Read(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID, cursor string) ([]entity.Event, string, error) {
			select {
			case batch := <-batches:
				return batch, batch[len(batch)-1].ID, nil
			case <-ctx.Done():
				return nil, cursor, ctx.Err()
			}
		}).
		AnyTimes()

	return batches
}

func (h *harness) event(id string, team uuid.UUID) entity.Event {
	return entity.Event{
		ID:          id,
		WorkspaceID: h.workspaceID,
		Kind:        entity.EventIssueUpdated,
		TeamID:      team,
		IssueID:     uuid.New(),
	}
}

func (h *harness) subscribe(t *testing.T) *service.Subscription {
	t.Helper()

	subscription, err := h.service.Subscribe(context.Background(), service.SubscribeInput{
		WorkspaceID:  h.workspaceID,
		Subscription: entity.EventSubscription{Topics: []entity.EventTopic{entity.EventTopicWorkspace}},
	})
	if err != nil {
		t.Fatalf("subscribing failed: %v", err)
	}

	t.Cleanup(subscription.Close)

	return subscription
}

func waitFor(t *testing.T, events <-chan entity.Event) (entity.Event, bool) {
	t.Helper()

	select {
	case event, open := <-events:
		return event, open
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an event")

		return entity.Event{}, false
	}
}

func TestAChangeOnAVisibleTeamReachesTheSubscriber(t *testing.T) {
	h := newHarness(t)
	h.feed(h.event("1-0", h.openTeam))

	subscription := h.subscribe(t)

	event, _ := waitFor(t, subscription.Events)

	if event.ID != "1-0" {
		t.Fatalf("received %q, want the published event", event.ID)
	}
}

func TestAChangeOnATeamTheSubscriberCannotSeeIsNeverDelivered(t *testing.T) {
	h := newHarness(t)
	h.feed(h.event("1-0", h.closedTeam), h.event("1-1", h.openTeam))

	subscription := h.subscribe(t)

	event, _ := waitFor(t, subscription.Events)

	if event.ID == "1-0" {
		t.Fatal(
			"an event for a team outside the subscriber's scope was delivered. This is the one " +
				"guarantee a push channel has to hold: it reads every workspace's changes and is " +
				"the only thing standing between them and somebody who cannot open them.",
		)
	}

	if event.ID != "1-1" {
		t.Fatalf("received %q, want the permitted event", event.ID)
	}
}

func TestAnEventAddressedToSomebodyElseIsNeverDelivered(t *testing.T) {
	h := newHarness(t)

	addressed := h.event("1-0", h.openTeam)
	addressed.Kind = entity.EventNotificationArrived
	addressed.AccountID = uuid.New()

	h.feed(addressed, h.event("1-1", h.openTeam))

	subscription := h.subscribe(t)

	event, _ := waitFor(t, subscription.Events)

	if event.ID != "1-1" {
		t.Fatalf(
			"received %q. A notification carries the account it is for, and a wide team scope "+
				"must not turn somebody else's inbox into yours.",
			event.ID,
		)
	}
}

func TestAMembershipChangeStopsDeliveryUntilTheScopeIsResolvedAgain(t *testing.T) {
	h := newHarness(t)

	membership := h.event("1-0", uuid.Nil)
	membership.Kind = entity.EventMembershipChanged
	membership.AccountID = h.accountID

	h.feed(membership, h.event("1-1", h.openTeam))

	subscription := h.subscribe(t)

	select {
	case event := <-subscription.Events:
		t.Fatalf(
			"event %q was delivered while the scope was being re-resolved. A membership change "+
				"can only narrow what somebody may see, so the gap has to fail closed rather than "+
				"keep using the scope that change just invalidated.",
			event.ID,
		)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAStalledSubscriberIsClosedRatherThanBufferedForever(t *testing.T) {
	h := newHarness(t)

	flood := make([]entity.Event, 0, entity.EventSubscriberBuffer*2)
	for i := range cap(flood) {
		flood = append(flood, h.event(string(rune('a'+i%26))+"-0", h.openTeam))
	}

	h.feed(flood...)

	subscription := h.subscribe(t)

	// Read nothing at all while the flood lands, which is what a stalled client looks like from
	// the server's side, then drain and expect to reach a closed channel.
	time.Sleep(300 * time.Millisecond)

	for read := 0; read <= entity.EventSubscriberBuffer; read++ {
		select {
		case _, open := <-subscription.Events:
			if !open {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("draining a closed subscriber blocked")
		}
	}

	t.Fatal(
		"a subscriber that never drained was still connected after twice its buffer was " +
			"published. Waiting for it would let one stalled reader hold up every other " +
			"reader on the workspace.",
	)
}

func TestOneStalledSubscriberDoesNotStarveAHealthyOne(t *testing.T) {
	h := newHarness(t)
	batches := h.live()

	stalled := h.subscribe(t)
	healthy := h.subscribe(t)

	received := make(chan entity.Event)

	go func() {
		defer close(received)

		for event := range healthy.Events {
			received <- event
		}
	}()

	// Enough to bury the stalled subscriber's buffer twice over, published one change at a time and
	// only once the healthy subscriber has taken the previous one, which is what keeping up means.
	total := entity.EventSubscriberBuffer * 2

	for sent := range total {
		batches <- []entity.Event{h.event(fmt.Sprintf("%d-0", sent+1), h.openTeam)}

		select {
		case _, open := <-received:
			if !open {
				t.Fatalf(
					"the healthy subscriber was closed after %d of %d events. Dropping a reader "+
						"that never drained must not cost the reader beside it anything.",
					sent, total,
				)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("the healthy subscriber stalled behind one that never drained at event %d", sent+1)
		}
	}

	for read := 0; read <= entity.EventSubscriberBuffer; read++ {
		select {
		case _, open := <-stalled.Events:
			if !open {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("draining the stalled subscriber blocked")
		}
	}

	t.Fatal("the subscriber that never drained was still connected after twice its buffer was published")
}

func TestResumingPastTheWindowReportsALapseRatherThanAPartialReplay(t *testing.T) {
	h := newHarness(t)
	h.feed()

	h.stream.EXPECT().
		Since(gomock.Any(), gomock.Any(), "1-0").
		Return(nil, entity.ErrEventStreamLapsed)

	subscription, err := h.service.Subscribe(context.Background(), service.SubscribeInput{
		WorkspaceID: h.workspaceID,
		Cursor:      "1-0",
	})
	if err != nil {
		t.Fatalf("subscribing failed: %v", err)
	}

	t.Cleanup(subscription.Close)

	if !subscription.Lapsed {
		t.Fatal(
			"a cursor older than the stream window was not reported as lapsed. XRANGE returns " +
				"rows from a trimmed id without complaint, so the client would treat a partial " +
				"replay as a complete one and never learn what it missed.",
		)
	}
}

func TestAResumedSubscriberOnlyReceivesMissedEventsItMaySee(t *testing.T) {
	h := newHarness(t)
	h.feed()

	h.stream.EXPECT().
		Since(gomock.Any(), gomock.Any(), "1-0").
		Return([]entity.Event{
			h.event("1-1", h.closedTeam),
			h.event("1-2", h.openTeam),
		}, nil)

	subscription, err := h.service.Subscribe(context.Background(), service.SubscribeInput{
		WorkspaceID: h.workspaceID,
		Cursor:      "1-0",
	})
	if err != nil {
		t.Fatalf("subscribing failed: %v", err)
	}

	t.Cleanup(subscription.Close)

	if len(subscription.Missed) != 1 || subscription.Missed[0].ID != "1-2" {
		t.Fatalf(
			"catch-up returned %d events. Replay reads the same stream the live tail does, so it "+
				"needs the same permission check; skipping it would make reconnection the way to "+
				"see a private team's work.",
			len(subscription.Missed),
		)
	}
}
