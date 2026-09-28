package telegrambot

import (
	"bytes"
	"context"
	"errors"
	"testing"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
)

const plaintextToken = "7000000001:AAHroundtripsecretroundtripsecretWXYZ"

func (f fixture) bot(botUserID int64) entity.TelegramBot {
	return entity.TelegramBot{
		WorkspaceID: f.workspaceID,
		AgentID:     f.agentID,
		BotUserID:   botUserID,
		Username:    "ada_bot",
		Name:        "Ada",
		ConnectedBy: f.accountID,
	}
}

func TestATokenIsSealedAtRestAndOpensBackForItsBot(t *testing.T) {
	db := reach(t)
	bots := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)

		saved, err := bots.Save(ctx, f.bot(7000000001), plaintextToken, []byte("secret-hash"))
		if err != nil {
			return err
		}

		if saved.TokenHint != "WXYZ" || saved.ConnectedByName != "Rae" || saved.AgentID != f.agentID {
			t.Errorf("saved = %+v", saved)
		}

		row, err := dbpostgres.FindWorkspaceTelegramBot(ctx, db.Querier(ctx), saved.ID.String())
		if err != nil {
			return err
		}

		if bytes.Contains(row.TokenSealed, []byte(plaintextToken)) {
			t.Error("the stored column holds the token in the clear")
		}

		token, err := bots.Token(ctx, saved.ID)
		if err != nil {
			return err
		}

		if token != plaintextToken {
			t.Errorf("opened token %q, want the saved token", token)
		}

		hash, err := bots.SecretHash(ctx, saved.ID)
		if err != nil {
			return err
		}

		if string(hash) != "secret-hash" {
			t.Errorf("secret hash = %q", hash)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestOneTelegramBotServesOneAgent(t *testing.T) {
	db := reach(t)
	bots := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		first := seed(ctx, t, db)
		second := seed(ctx, t, db)

		if _, err := bots.Save(ctx, first.bot(7000000002), plaintextToken, []byte("a")); err != nil {
			return err
		}

		holder, err := bots.GetByBotUser(ctx, 7000000002)
		if err != nil {
			return err
		}

		if holder.AgentID != first.agentID {
			t.Errorf("holder agent = %s, want %s", holder.AgentID, first.agentID)
		}

		err = db.WithSavepoint(ctx, func(ctx context.Context) error {
			_, err := bots.Save(ctx, second.bot(7000000002), plaintextToken, []byte("b"))

			return err
		})
		if !errors.Is(err, entity.ErrTelegramBotTaken) {
			t.Errorf("second agent: err = %v, want ErrTelegramBotTaken", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestRekeyingKeepsTheBotAndDeleteRemovesIt(t *testing.T) {
	db := reach(t)
	bots := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)

		saved, err := bots.Save(ctx, f.bot(7000000003), plaintextToken, []byte("a"))
		if err != nil {
			return err
		}

		saved.Username = "ada_renamed_bot"

		rekeyed, err := bots.Rekey(ctx, saved, "7000000003:AAHrotatedrotatedrotatedrotated1234", []byte("b"))
		if err != nil {
			return err
		}

		if rekeyed.ID != saved.ID || rekeyed.TokenHint != "1234" || rekeyed.Username != "ada_renamed_bot" {
			t.Errorf("rekeyed = %+v", rekeyed)
		}

		if err := bots.Delete(ctx, f.workspaceID, f.agentID); err != nil {
			return err
		}

		if _, err := bots.Get(ctx, f.workspaceID, f.agentID); !errors.Is(err, entity.ErrTelegramBotNotFound) {
			t.Errorf("after delete: err = %v, want ErrTelegramBotNotFound", err)
		}

		if err := bots.Delete(ctx, f.workspaceID, f.agentID); !errors.Is(err, entity.ErrTelegramBotNotFound) {
			t.Errorf("second delete: err = %v, want ErrTelegramBotNotFound", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
