package notification_test

import (
	"context"
	"errors"
	"testing"

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
		Directed(gomock.Any(), h.workspaceID, gomock.Any(), recipient, gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_, actorID, _, _ uuid.UUID,
			_ int,
		) ([]entity.DirectedNotice, error) {
			askedFor = actorID

			return nil, nil
		})

	if _, err := h.service.Directed(context.Background(), h.workspaceID, recipient, uuid.Nil, 0); err != nil {
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

	_, err := h.service.Directed(context.Background(), h.workspaceID, uuid.Nil, uuid.Nil, 0)

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("asking with no recipient returned %v, want a validation error", err)
	}
}
