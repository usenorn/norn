package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAServerBoundMessageTypeCanBeMintedForARunner(t *testing.T) {
	for _, kind := range entity.ChannelRunnerMessages() {
		if _, err := entity.NewServerMessage(kind, "", nil, time.Now().UTC()); !errors.Is(
			err, entity.ErrChannelTypeNotOutbound,
		) {
			t.Errorf("minting %q for a runner returned %v, want %v",
				kind, err, entity.ErrChannelTypeNotOutbound)
		}
	}

	for _, kind := range entity.ChannelServerMessages() {
		if _, err := entity.NewServerMessage(kind, "", nil, time.Now().UTC()); err != nil {
			t.Errorf("minting %q for a runner returned %v", kind, err)
		}
	}
}

func TestEveryMintedMessageCarriesSomethingToDedupeOn(t *testing.T) {
	issued := time.Now().UTC()

	first, err := entity.NewServerMessage(entity.ChannelExecutionCancel, "exec-1", nil, issued)
	if err != nil {
		t.Fatalf("mint a cancellation: %v", err)
	}

	second, err := entity.NewServerMessage(entity.ChannelExecutionCancel, "exec-1", nil, issued)
	if err != nil {
		t.Fatalf("mint a second cancellation: %v", err)
	}

	switch {
	case first.ID == "":
		t.Fatal("a minted message has no id, so a runner cannot tell a redelivery from a new one")
	case first.ID == second.ID:
		t.Fatal("two messages were minted with the same id, so one would be dropped as a repeat")
	case !first.IssuedAt.Equal(issued):
		t.Fatalf("a minted message is stamped %v, want %v", first.IssuedAt, issued)
	case first.ExecutionID != "exec-1":
		t.Fatalf("a minted message names execution %q, want exec-1", first.ExecutionID)
	}
}
