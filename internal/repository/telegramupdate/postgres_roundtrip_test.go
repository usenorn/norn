package telegramupdate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestARedeliveredUpdateIsRefusedAndSettlingDropsItsBody(t *testing.T) {
	db := reach(t)
	updates := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)

		id, err := updates.Record(ctx, entity.TelegramUpdate{BotID: f.botID, UpdateID: 900, Payload: []byte(`{"update_id":900}`)})
		if err != nil {
			return err
		}

		if _, err := updates.Record(ctx, entity.TelegramUpdate{BotID: f.botID, UpdateID: 900, Payload: []byte(`{}`)}); !errors.Is(err, entity.ErrTelegramUpdateDuplicate) {
			t.Errorf("redelivery: err = %v, want ErrTelegramUpdateDuplicate", err)
		}

		stored, err := updates.UpdateOf(ctx, f.botID, 900)
		if err != nil {
			return err
		}

		if stored.ID != id || stored.Processed() {
			t.Errorf("stored = %+v", stored)
		}

		if err := updates.Settle(ctx, id, entity.TelegramUpdateApplied, time.Now().UTC()); err != nil {
			return err
		}

		settled, err := updates.Lock(ctx, id)
		if err != nil {
			return err
		}

		if !settled.Processed() || settled.Outcome != entity.TelegramUpdateApplied || settled.Payload != nil {
			t.Errorf("settled = %+v", settled)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestTheSweepLeavesUnprocessedUpdatesAlone(t *testing.T) {
	db := reach(t)
	updates := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		old := time.Now().UTC().Add(-48 * time.Hour)

		done, err := updates.Record(ctx, entity.TelegramUpdate{BotID: f.botID, UpdateID: 1, Payload: []byte(`{}`), ReceivedAt: old})
		if err != nil {
			return err
		}

		pending, err := updates.Record(ctx, entity.TelegramUpdate{BotID: f.botID, UpdateID: 2, Payload: []byte(`{}`), ReceivedAt: old})
		if err != nil {
			return err
		}

		if err := updates.Settle(ctx, done, entity.TelegramUpdateIgnored, old); err != nil {
			return err
		}

		if _, err := updates.Sweep(ctx, time.Now().UTC().Add(-time.Hour), 100); err != nil {
			return err
		}

		if _, err := updates.Lock(ctx, done); !errors.Is(err, entity.ErrTelegramUpdateNotFound) {
			t.Errorf("processed update survived the sweep: err = %v", err)
		}

		if _, err := updates.Lock(ctx, pending); err != nil {
			t.Errorf("unprocessed update was swept: %v", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
