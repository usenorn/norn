package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAgentStartsOnARunnerAndKeepsTheExecutionItIsMovedTo(t *testing.T) {
	client, cleanup := liveAgentRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, ownerID, accountID := uuid.New(), uuid.New(), uuid.New()

		if err := layScopeFixture(ctx, client, workspaceID, ownerID, accountID, uuid.New()); err != nil {
			failure = err

			return errAgentRollback
		}

		repository := New(client)

		created, err := repository.Create(ctx, entity.Agent{
			WorkspaceID:    workspaceID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Name:           "hosted-agent",
		})
		if err != nil {
			failure = fmt.Errorf("create agent: %w", err)

			return errAgentRollback
		}

		if created.Execution != entity.AgentExecutionRunner {
			failure = fmt.Errorf("a new agent runs on %q, want runner", created.Execution)

			return errAgentRollback
		}

		if _, err := repository.SetExecution(ctx, workspaceID, created.ID, entity.AgentExecutionHosted); err != nil {
			failure = fmt.Errorf("set agent execution: %w", err)

			return errAgentRollback
		}

		byAccount, err := repository.GetByAccountID(ctx, accountID)
		if err != nil {
			failure = fmt.Errorf("read agent by account: %w", err)

			return errAgentRollback
		}

		if !byAccount.Hosted() {
			failure = fmt.Errorf("read back execution %q, want hosted", byAccount.Execution)

			return errAgentRollback
		}

		if _, err := repository.SetExecution(
			ctx, workspaceID, uuid.New(), entity.AgentExecutionHosted,
		); !errors.Is(err, entity.ErrAgentNotFound) {
			failure = fmt.Errorf("moving an unknown agent = %v, want ErrAgentNotFound", err)
		}

		return errAgentRollback
	})

	if !errors.Is(err, errAgentRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
