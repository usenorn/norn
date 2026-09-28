package telegramconversation

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

const questionFixture = `
WITH team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    VALUES ($1, 'TG', 'Telegram')
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT team.workspace_id, team.id, 1, 'Ship it', state.id, 'TG', 'n'
    FROM team, state
    RETURNING id, workspace_id
)
INSERT INTO workspace_issue_questions (workspace_id, issue_id, question, default_answer, deadline)
SELECT workspace_id, id, 'Ship it?', 'Yes', now() + interval '1 hour' FROM issue
RETURNING id`

func question(ctx context.Context, t *testing.T, db *postgres.Client, f fixture) uuid.UUID {
	t.Helper()

	var id string
	if err := db.Querier(ctx).QueryRowContext(ctx, questionFixture, f.workspaceID.String()).Scan(&id); err != nil {
		t.Fatalf("insert question fixture: %v", err)
	}

	return uuid.MustParse(id)
}

func TestAQuestionMessageIsRememberedOncePerChatAndResolvesBack(t *testing.T) {
	db := reach(t)
	conversation := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		asked := question(ctx, t, db, f)

		first := entity.TelegramQuestionMessage{BotID: f.botID, ChatID: 7, MessageID: 70, QuestionID: asked}
		if err := conversation.Remember(ctx, first); err != nil {
			return err
		}

		if err := conversation.Remember(ctx, entity.TelegramQuestionMessage{BotID: f.botID, ChatID: 7, MessageID: 71, QuestionID: asked}); err != nil {
			return err
		}

		posted, err := conversation.Posted(ctx, asked)
		if err != nil {
			return err
		}

		if len(posted) != 1 || posted[0].MessageID != 70 {
			t.Errorf("posted = %+v, want only the first message", posted)
		}

		resolved, err := conversation.QuestionAt(ctx, f.botID, 7, 70)
		if err != nil {
			return err
		}

		if resolved != asked {
			t.Errorf("resolved %s, want %s", resolved, asked)
		}

		if _, err := conversation.QuestionAt(ctx, f.botID, 8, 70); !errors.Is(err, entity.ErrIssueQuestionNotFound) {
			t.Errorf("another chat: err = %v, want ErrIssueQuestionNotFound", err)
		}

		if err := conversation.MarkSettled(ctx, first, time.Now().UTC()); err != nil {
			return err
		}

		settled, err := conversation.Posted(ctx, asked)
		if err != nil {
			return err
		}

		if !settled[0].Settled {
			t.Error("the message was not marked settled")
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestHistoryReturnsTheLatestTurnsInOrderAndPruneKeepsThem(t *testing.T) {
	db := reach(t)
	conversation := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)

		for _, text := range []string{"one", "two", "three", "four"} {
			if err := conversation.Append(ctx, f.botID, 7, entity.AgentTurn{Role: entity.AgentTurnUser, Text: text}); err != nil {
				return err
			}
		}

		if err := conversation.Append(ctx, f.botID, 8, entity.AgentTurn{Role: entity.AgentTurnUser, Text: "elsewhere"}); err != nil {
			return err
		}

		latest, err := conversation.History(ctx, f.botID, 7, 2)
		if err != nil {
			return err
		}

		if texts := texts(latest); !slices.Equal(texts, []string{"three", "four"}) {
			t.Errorf("history = %q", texts)
		}

		if err := conversation.Prune(ctx, f.botID, 7, 3); err != nil {
			return err
		}

		kept, err := conversation.History(ctx, f.botID, 7, 10)
		if err != nil {
			return err
		}

		if texts := texts(kept); !slices.Equal(texts, []string{"two", "three", "four"}) {
			t.Errorf("after prune = %q", texts)
		}

		other, err := conversation.History(ctx, f.botID, 8, 10)
		if err != nil {
			return err
		}

		if len(other) != 1 {
			t.Errorf("another chat's history was touched: %q", texts(other))
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func texts(turns []entity.AgentTurn) []string {
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		out = append(out, turn.Text)
	}

	return out
}
