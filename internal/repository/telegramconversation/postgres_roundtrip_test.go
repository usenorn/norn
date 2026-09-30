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

const executionFixture = `
WITH team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    VALUES ($1, 'TX', 'Runs')
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT team.workspace_id, team.id, 1, 'Run it', state.id, 'TX', 'n'
    FROM team, state
    RETURNING id, workspace_id
), delegation AS (
    INSERT INTO workspace_issue_delegations (workspace_id, issue_id, agent_id)
    SELECT workspace_id, id, $2 FROM issue
    RETURNING id, workspace_id, issue_id
)
INSERT INTO workspace_executions (id, workspace_id, issue_id, delegation_id, agent_id, attempt)
SELECT 'exec-' || $2::text, workspace_id, issue_id, id, $2, 1 FROM delegation
RETURNING id`

func execution(ctx context.Context, t *testing.T, db *postgres.Client, f fixture) string {
	t.Helper()

	var id string
	if err := db.Querier(ctx).QueryRowContext(
		ctx, executionFixture, f.workspaceID.String(), f.agentID.String(),
	).Scan(&id); err != nil {
		t.Fatalf("insert execution fixture: %v", err)
	}

	return id
}

func TestAQuestionMessageIsRememberedOncePerChatAndResolvesBack(t *testing.T) {
	db := reach(t)
	conversation := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		asked := question(ctx, t, db, f)
		decision := entity.TelegramDecision{WorkspaceID: f.workspaceID, Kind: entity.TelegramDecisionQuestion, QuestionID: asked}

		first := entity.TelegramDecisionMessage{
			BotID: f.botID, ChatID: 7, MessageID: 70, Kind: entity.TelegramDecisionQuestion, QuestionID: asked,
		}
		if err := conversation.Remember(ctx, first); err != nil {
			return err
		}

		again := first
		again.MessageID = 71

		if err := conversation.Remember(ctx, again); err != nil {
			return err
		}

		posted, err := conversation.Posted(ctx, decision)
		if err != nil {
			return err
		}

		if len(posted) != 1 || posted[0].MessageID != 70 {
			t.Errorf("posted = %+v, want only the first message", posted)
		}

		resolved, err := conversation.DecisionAt(ctx, f.botID, 7, 70)
		if err != nil {
			return err
		}

		if resolved.Kind != entity.TelegramDecisionQuestion || resolved.QuestionID != asked {
			t.Errorf("resolved %+v, want question %s", resolved, asked)
		}

		if _, err := conversation.DecisionAt(ctx, f.botID, 8, 70); !errors.Is(err, entity.ErrTelegramDecisionNotFound) {
			t.Errorf("another chat: err = %v, want ErrTelegramDecisionNotFound", err)
		}

		if err := conversation.MarkSettled(ctx, first, time.Now().UTC()); err != nil {
			return err
		}

		settled, err := conversation.Posted(ctx, decision)
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

func TestPlanReviewAndPublicationMessagesKeepWhatTheyWereSentAbout(t *testing.T) {
	db := reach(t)
	conversation := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db)
		run := execution(ctx, t, db, f)
		heads := entity.ReviewHeads{"api": "abc123", "web": "def456"}

		for _, message := range []entity.TelegramDecisionMessage{
			{BotID: f.botID, ChatID: 7, MessageID: 80, Kind: entity.TelegramDecisionPlan, ExecutionID: run, PlanRevision: 1},
			{BotID: f.botID, ChatID: 7, MessageID: 81, Kind: entity.TelegramDecisionPlan, ExecutionID: run, PlanRevision: 2},
			{BotID: f.botID, ChatID: 7, MessageID: 82, Kind: entity.TelegramDecisionReview, ExecutionID: run, ReviewHeads: heads},
			{BotID: f.botID, ChatID: 7, MessageID: 83, Kind: entity.TelegramDecisionPublication, ExecutionID: run, Round: "2.1"},
		} {
			if err := conversation.Remember(ctx, message); err != nil {
				return err
			}
		}

		plans, err := conversation.Posted(ctx, entity.TelegramDecision{Kind: entity.TelegramDecisionPlan, ExecutionID: run})
		if err != nil {
			return err
		}

		if len(plans) != 2 {
			t.Errorf("posted plans = %+v, want both revisions", plans)
		}

		review, err := conversation.DecisionAt(ctx, f.botID, 7, 82)
		if err != nil {
			return err
		}

		if review.Kind != entity.TelegramDecisionReview || !review.ReviewHeads.Matches(heads) {
			t.Errorf("review message resolved to %+v, want the heads it was sent for", review)
		}

		plan, err := conversation.DecisionAt(ctx, f.botID, 7, 81)
		if err != nil {
			return err
		}

		if plan.PlanRevision != 2 || plan.ExecutionID != run {
			t.Errorf("plan message resolved to %+v, want revision 2 of %s", plan, run)
		}

		publication, err := conversation.DecisionAt(ctx, f.botID, 7, 83)
		if err != nil {
			return err
		}

		if publication.Kind != entity.TelegramDecisionPublication || publication.Round != "2.1" {
			t.Errorf("publication message resolved to %+v, want the attempt it reported", publication)
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
