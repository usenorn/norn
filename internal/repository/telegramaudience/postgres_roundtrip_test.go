package telegramaudience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestALinkCodeStaysUntilSpentAndOnlyBeforeItExpires(t *testing.T) {
	db := reach(t)
	audience := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		now := time.Now().UTC()

		_, fresh, err := entity.NewTelegramLinkCode()
		if err != nil {
			return err
		}

		_, stale, err := entity.NewTelegramLinkCode()
		if err != nil {
			return err
		}

		if err := audience.IssueCode(ctx, entity.TelegramLinkCode{
			BotID: f.botID, AccountID: f.accountID, Purpose: entity.TelegramLinkGroup, ExpiresAt: now.Add(time.Minute),
		}, fresh); err != nil {
			return err
		}

		if err := audience.IssueCode(ctx, entity.TelegramLinkCode{
			BotID: f.botID, AccountID: f.accountID, Purpose: entity.TelegramLinkPrivate, ExpiresAt: now.Add(time.Second),
		}, stale); err != nil {
			return err
		}

		for range 2 {
			found, err := audience.LinkCode(ctx, f.botID, fresh, now)
			if err != nil {
				return err
			}

			if found.AccountID != f.accountID || found.Purpose != entity.TelegramLinkGroup {
				t.Errorf("found = %+v", found)
			}
		}

		if err := audience.SpendCode(ctx, f.botID, fresh); err != nil {
			return err
		}

		if _, err := audience.LinkCode(ctx, f.botID, fresh, now); !errors.Is(err, entity.ErrTelegramLinkCodeInvalid) {
			t.Errorf("spent code: err = %v, want ErrTelegramLinkCodeInvalid", err)
		}

		if _, err := audience.LinkCode(ctx, f.botID, stale, now.Add(time.Minute)); !errors.Is(err, entity.ErrTelegramLinkCodeInvalid) {
			t.Errorf("expired code: err = %v, want ErrTelegramLinkCodeInvalid", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestLinkingATelegramUserToAnotherAccountReleasesTheOldLink(t *testing.T) {
	db := reach(t)
	audience := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		other := account(ctx, t, db, entity.AccountKindPerson, "Bo")

		if err := audience.Link(ctx, entity.TelegramAccount{
			BotID: f.botID, AccountID: f.accountID, TelegramUserID: 7000000001, ChatID: 7000000001, Username: "rae",
		}); err != nil {
			return err
		}

		if err := audience.Link(ctx, entity.TelegramAccount{
			BotID: f.botID, AccountID: other, TelegramUserID: 7000000001, ChatID: 7000000001,
		}); err != nil {
			return err
		}

		if _, err := audience.AccountFor(ctx, f.botID, f.accountID); !errors.Is(err, entity.ErrTelegramAccountNotLinked) {
			t.Errorf("old link: err = %v, want ErrTelegramAccountNotLinked", err)
		}

		linked, err := audience.AccountOf(ctx, f.botID, 7000000001)
		if err != nil {
			return err
		}

		if linked.AccountID != other || linked.AccountName != "Bo" {
			t.Errorf("linked = %+v", linked)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestAGroupIsUnboundOnlyThroughItsOwnBotAndFollowsAMigration(t *testing.T) {
	db := reach(t)
	audience := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		neighbour := seed(ctx, t, db)

		group, err := audience.Bind(ctx, entity.TelegramGroup{
			BotID: f.botID, ChatID: -4001, Title: "Release", BoundBy: f.accountID,
		})
		if err != nil {
			return err
		}

		if group.BoundByName != "Rae" || group.ChatID != -4001 {
			t.Errorf("bound = %+v", group)
		}

		if _, err := audience.Unbind(ctx, neighbour.botID, group.ID); !errors.Is(err, entity.ErrTelegramGroupNotFound) {
			t.Errorf("foreign unbind: err = %v, want ErrTelegramGroupNotFound", err)
		}

		if err := audience.MoveGroup(ctx, f.botID, -4001, -1009000000000001); err != nil {
			return err
		}

		moved, err := audience.Group(ctx, f.botID, -1009000000000001)
		if err != nil {
			return err
		}

		if moved.ID != group.ID {
			t.Errorf("moved group id %s, want %s", moved.ID, group.ID)
		}

		if _, err := audience.Unbind(ctx, f.botID, group.ID); err != nil {
			return err
		}

		groups, err := audience.Groups(ctx, f.botID)
		if err != nil {
			return err
		}

		if len(groups) != 0 {
			t.Errorf("groups after unbind = %+v", groups)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
