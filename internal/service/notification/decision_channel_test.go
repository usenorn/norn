package notification_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) actingAsReader() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.readerID}}, nil).
		AnyTimes()
}

func TestADecisionChannelIsSavedForTheCallerOnly(t *testing.T) {
	h := newHarness(t)
	h.actingAsReader()
	h.settings.EXPECT().
		SaveDecisionChannel(gomock.Any(), h.workspaceID, h.readerID, entity.DecisionChannelTelegram).
		Return(nil)

	saved, err := h.service.SetDecisionChannel(context.Background(), h.workspaceID, entity.DecisionChannelTelegram)
	if err != nil {
		t.Fatalf("save the decision channel: %v", err)
	}

	if saved != entity.DecisionChannelTelegram {
		t.Fatalf("saved %q, want telegram", saved)
	}
}

func TestAnUnknownDecisionChannelIsRefusedBeforeAnythingIsSaved(t *testing.T) {
	h := newHarness(t)
	h.actingAsReader()

	_, err := h.service.SetDecisionChannel(context.Background(), h.workspaceID, entity.DecisionChannel("email"))

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("an unknown channel came back %v, want a validation error", err)
	}
}
