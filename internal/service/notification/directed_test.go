package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAgentAsksOnBehalfOfWhoeverOwnsIt(t *testing.T) {
	h := newHarness(t)
	owner := uuid.New()
	recipient := uuid.New()

	h.actingAs(entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      uuid.New(),
		OwnerAccountID: owner,
	})

	var askedFor uuid.UUID

	h.notifications.EXPECT().
		Directed(gomock.Any(), h.workspaceID, gomock.Any(), recipient, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, actorID, _, _ uuid.UUID,
			_ time.Time,
			_ int,
		) ([]entity.DirectedNotice, error) {
			askedFor = actorID

			return nil, nil
		})

	if _, err := h.service.Directed(context.Background(), h.workspaceID, recipient, uuid.Nil, 0, 0); err != nil {
		t.Fatalf("Directed: %v", err)
	}

	if askedFor != owner {
		t.Fatalf(
			"an agent asking what was directed at somebody was answered for %s, want its owner %s "+
				"— otherwise it reports on what the agent sent rather than what its person sent",
			askedFor, owner,
		)
	}
}

func TestAskingWithoutNamingSomebodyIsRefused(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()})

	_, err := h.service.Directed(context.Background(), h.workspaceID, uuid.Nil, uuid.Nil, 0, 0)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("asking with no recipient returned %v, want a validation error", err)
	}
}

func TestAskingAboutOneIssueIgnoresTheWindow(t *testing.T) {
	h := newHarness(t)
	subject := uuid.New()

	h.actingAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()})

	var since time.Time

	h.notifications.EXPECT().
		Directed(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), subject, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _, _, _ uuid.UUID,
			from time.Time,
			_ int,
		) ([]entity.DirectedNotice, error) {
			since = from

			return nil, nil
		})

	if _, err := h.service.Directed(
		context.Background(), h.workspaceID, uuid.New(), subject, 0, 0,
	); err != nil {
		t.Fatalf("Directed: %v", err)
	}

	if !since.IsZero() {
		t.Fatalf(
			"asking about one issue looked back only to %s. A named subject has no window: an "+
				"issue assigned before it would lose its mark on the card even though the "+
				"receipt is still the answer",
			since,
		)
	}
}

func TestAskingWithoutASubjectStopsAtTheWindow(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()})

	var since time.Time

	h.notifications.EXPECT().
		DirectedTally(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), uuid.Nil, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _, _, _ uuid.UUID,
			from time.Time,
		) (entity.DirectedTally, error) {
			since = from

			return entity.DirectedTally{}, nil
		})

	if _, err := h.service.DirectedTally(
		context.Background(), h.workspaceID, uuid.New(), uuid.Nil, 0,
	); err != nil {
		t.Fatalf("DirectedTally: %v", err)
	}

	back := time.Since(since)
	if back < entity.DirectedWindowDefault-time.Minute || back > entity.DirectedWindowDefault+time.Minute {
		t.Fatalf("looked back %s, want the default window of %s", back, entity.DirectedWindowDefault)
	}
}

func TestAWindowWiderThanTheYearIsCutBackToIt(t *testing.T) {
	h := newHarness(t)

	h.actingAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()})

	var since time.Time

	h.notifications.EXPECT().
		DirectedTally(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, _, _, _ uuid.UUID,
			from time.Time,
		) (entity.DirectedTally, error) {
			since = from

			return entity.DirectedTally{}, nil
		})

	if _, err := h.service.DirectedTally(
		context.Background(), h.workspaceID, uuid.New(), uuid.Nil, 10*entity.DirectedWindowMax,
	); err != nil {
		t.Fatalf("DirectedTally: %v", err)
	}

	if back := time.Since(since); back > entity.DirectedWindowMax+time.Minute {
		t.Fatalf("looked back %s, want no more than %s", back, entity.DirectedWindowMax)
	}
}
