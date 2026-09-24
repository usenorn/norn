package project

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

var errProjectRollback = errors.New("roll the project fixture back")

func liveProjectRepository(t *testing.T) (*postgres.Client, func()) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to test")
	}

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        2,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}

	return client, cleanup
}

func TestProjectAgentInstructionsSurviveEveryReadAndEveryUnrelatedWrite(t *testing.T) {
	client, cleanup := liveProjectRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID := uuid.New()

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Project instructions')",
			workspaceID,
			"project-instructions-"+workspaceID.String()[:8],
		); err != nil {
			failure = err

			return errProjectRollback
		}

		projects := New(client)

		created, err := projects.Create(ctx, entity.Project{
			WorkspaceID: workspaceID,
			Slug:        "checkout-rebuild",
			Name:        "Checkout rebuild",
			Description: "The shared reference.",
			State:       entity.ProjectStateActive,
		})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		if created.AgentInstructions != "" {
			failure = errors.New("a fresh project already carried instructions")

			return errProjectRollback
		}

		written := "Touch the ledger only."

		saved, err := projects.UpdateSettings(ctx, created.ID, repository.ProjectSettings{
			Name:              created.Name,
			AgentInstructions: &written,
		})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		byID, err := projects.GetByID(ctx, workspaceID, created.ID)
		if err != nil {
			failure = err

			return errProjectRollback
		}

		bySlug, err := projects.GetBySlug(ctx, workspaceID, created.Slug)
		if err != nil {
			failure = err

			return errProjectRollback
		}

		locked, err := projects.LockByID(ctx, created.ID)
		if err != nil {
			failure = err

			return errProjectRollback
		}

		listed, err := projects.ListByWorkspaceID(ctx, workspaceID, repository.ProjectFilter{})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		if len(listed) != 1 {
			failure = errors.New("the workspace listed a number of projects other than the one it has")

			return errProjectRollback
		}

		finished, err := projects.SetState(ctx, created.ID, entity.ProjectStateCompleted)
		if err != nil {
			failure = err

			return errProjectRollback
		}

		archived, err := projects.Archive(ctx, created.ID, time.Now().UTC())
		if err != nil {
			failure = err

			return errProjectRollback
		}

		unarchived, err := projects.Unarchive(ctx, created.ID)
		if err != nil {
			failure = err

			return errProjectRollback
		}

		for name, read := range map[string]string{
			"the write itself":     saved.AgentInstructions,
			"by id":                byID.AgentInstructions,
			"by address":           bySlug.AgentInstructions,
			"locked":               locked.AgentInstructions,
			"in the list":          listed[0].AgentInstructions,
			"after a state change": finished.AgentInstructions,
			"after archiving":      archived.AgentInstructions,
			"after unarchiving":    unarchived.AgentInstructions,
		} {
			if read != written {
				failure = errors.New("a project read " + name + " returned " + read + ", want " + written)

				return errProjectRollback
			}
		}

		renamed, err := projects.UpdateSettings(ctx, created.ID, repository.ProjectSettings{
			Name: "Checkout rebuild II",
		})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		if renamed.AgentInstructions != written {
			failure = errors.New("renaming the project blanked its instructions")

			return errProjectRollback
		}

		if renamed.Description != created.Description {
			failure = errors.New("an update that said nothing about the description blanked it")

			return errProjectRollback
		}

		empty := ""

		cleared, err := projects.UpdateSettings(ctx, created.ID, repository.ProjectSettings{
			Name:              renamed.Name,
			AgentInstructions: &empty,
		})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		if cleared.AgentInstructions != "" {
			failure = errors.New("instructions could not be cleared")

			return errProjectRollback
		}

		blanked, err := projects.UpdateSettings(ctx, created.ID, repository.ProjectSettings{
			Name:        renamed.Name,
			Description: &empty,
		})
		if err != nil {
			failure = err

			return errProjectRollback
		}

		if blanked.Description != "" {
			failure = errors.New("a description asked for in so many words could not be emptied")
		}

		return errProjectRollback
	})

	if !errors.Is(err, errProjectRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
