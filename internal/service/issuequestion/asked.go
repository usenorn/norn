package issuequestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	sweepBatch    = 200
	dismissedNote = "nobody is going to answer the question this run stopped on"
	expiredNote   = "nobody answered the question this run stopped on, and it has run out of time"
)

func (s *questionsService) Asked(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.executions.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.Question

	if err := json.Unmarshal(message.Payload, &incoming); err != nil {
		return entity.ErrChannelEnvelopeInvalid
	}

	if err := entity.NewValidationError(
		entity.ValidateQuestionRef("ref", incoming.Ref),
	); err != nil {
		return err
	}

	target := asking{
		workspaceID: execution.WorkspaceID,
		issueID:     execution.IssueID,
		teamID:      execution.TeamID,
	}

	_, err = s.ask(ctx, target, asker(ctx), entity.ActorKindAgent, service.AskQuestionInput{
		Question:      incoming.Message,
		Default:       incoming.Default,
		Wait:          waitFor(incoming),
		Kind:          entity.QuestionKind(incoming.Kind),
		Blocking:      incoming.Blocking,
		Options:       incoming.Options,
		AllowFreeText: incoming.AllowFreeText,
		Context: entity.QuestionContext{
			Preview:   incoming.Context.Preview,
			Files:     incoming.Context.Files,
			Artifacts: incoming.Context.Artifacts,
		},
		ExecutionID: execution.ID,
		Ref:         incoming.Ref,
	})

	// The machine replayed the message after a reconnect. It is asking nothing new.
	if errors.Is(err, entity.ErrIssueQuestionRecorded) {
		return nil
	}

	return err
}

// waitFor gives a run that stopped the long deadline, because a person has to notice the question
// before the run can go anywhere, and the shorter wait a working agent declares would strand it.
func waitFor(incoming channelv1.Question) time.Duration {
	if incoming.Blocking {
		return entity.QuestionParkedWait
	}

	if incoming.Wait <= 0 {
		return entity.QuestionWaitDefault
	}

	return time.Duration(incoming.Wait) * time.Second
}

// asker reads the machine's identity out of the context the channel put it in, which is the same
// account the run's other writes are attributed to.
func asker(ctx context.Context) entity.ActivityAttribution {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return entity.ActivityAttribution{Kind: entity.ActorKindSystem}
	}

	return entity.ActivityAttribution{AccountID: actor.AccountID, Kind: entity.ActorKindAgent}
}

func (s *questionsService) SweepExpired(ctx context.Context) error {
	now := time.Now().UTC()

	lapsed, err := s.questions.Lapsed(ctx, now, sweepBatch)
	if err != nil {
		return err
	}

	for _, question := range lapsed {
		settled, err := s.questions.Settle(ctx, question.WorkspaceID, repository.QuestionSettlement{
			QuestionID: question.ID,
			State:      entity.QuestionExpired,
			SettledAt:  now,
		})
		if err != nil {
			if errors.Is(err, entity.ErrIssueQuestionAnswered) ||
				errors.Is(err, entity.ErrIssueQuestionSettled) {
				continue
			}

			return err
		}

		if !settled.Blocking || settled.ExecutionID == "" {
			continue
		}

		if err := s.executions.Unanswerable(ctx, settled, expiredNote); err != nil {
			return fmt.Errorf("stop a run nobody answered: %w", err)
		}

		logging.From(ctx).InfoContext(
			ctx,
			"a run was stopped because the question it waited on ran out of time",
			slog.String("execution_id", settled.ExecutionID),
			slog.String("question_id", settled.ID.String()),
		)
	}

	return nil
}
