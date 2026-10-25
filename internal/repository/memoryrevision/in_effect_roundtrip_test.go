package memoryrevision

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
)

func TestWhatWasInEffectFollowsCorrectionsAndSupersession(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := memoryrepo.New(client)
		revisions := New(client)
		by := entity.KnowledgeAttribution{AccountID: fixture.authorID, Source: entity.RevisionSourcePerson}
		start := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

		old := draftMemory(fixture, "Deploy with Capistrano", entity.WorkspaceAudience(), start)
		replacement := draftMemory(fixture, "Deploy with Argo CD", entity.WorkspaceAudience(), start.AddDate(0, 3, 0))
		replacement.SupersedesID = &old.ID
		hidden := draftMemory(fixture, "Deploy from my laptop", entity.PersonalAudience(), start)
		hidden.AuthorID = uuid.New()

		if _, err := client.Querier(ctx).ExecContext(ctx, `
INSERT INTO accounts (id, status, kind, email, display_name, timezone)
VALUES ($1, 'active', 'person', $2, 'Sol', 'UTC')`, hidden.AuthorID, hidden.AuthorID.String()+"@example.test"); err != nil {
			return err
		}

		hidden.By.AccountID = hidden.AuthorID

		for _, memory := range []entity.Memory{old, replacement, hidden} {
			if err := memories.Create(ctx, memory); err != nil {
				return fmt.Errorf("create %q: %w", memory.Statement, err)
			}

			if err := revisions.Record(ctx, memory.Revision(entity.MemoryRevisionCreated, memory.By)); err != nil {
				return err
			}
		}

		corrected := old
		corrected.Version, corrected.Statement = 2, "Deploy with Capistrano from CI"
		corrected.UpdatedAt = start.AddDate(0, 1, 0)

		superseded := corrected
		superseded.Version, superseded.Status = 3, entity.MemoryStatusSuperseded
		superseded.SupersededByID = &replacement.ID
		superseded.UpdatedAt = replacement.CreatedAt

		for _, step := range []struct {
			memory entity.Memory
			action entity.MemoryRevisionAction
		}{{corrected, entity.MemoryRevisionCorrected}, {superseded, entity.MemoryRevisionSuperseded}} {
			if err := memories.Update(ctx, step.memory, step.memory.Version-1); err != nil {
				return fmt.Errorf("update to version %d: %w", step.memory.Version, err)
			}

			if err := revisions.Record(ctx, step.memory.Revision(step.action, by)); err != nil {
				return err
			}
		}

		for name, testCase := range map[string]struct {
			at   time.Time
			want []string
		}{
			"before anything was decided": {at: start.AddDate(0, 0, -1)},
			"as first written":            {at: start.AddDate(0, 0, 7), want: []string{"Deploy with Capistrano"}},
			"after the correction":        {at: start.AddDate(0, 2, 0), want: []string{"Deploy with Capistrano from CI"}},
			"after the supersession":      {at: start.AddDate(0, 4, 0), want: []string{"Deploy with Argo CD"}},
		} {
			found, err := revisions.InEffect(ctx, fixture.workspaceID, repository.KnowledgeQuery{
				Query:            entity.ParseSearchQuery("how do we deploy"),
				Statuses:         entity.GuidanceStatuses(false),
				On:               testCase.at,
				PersonalAuthorID: fixture.authorID,
				Limit:            entity.KnowledgeLaneSize,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}

			var statements []string
			for _, candidate := range found {
				statements = append(statements, candidate.Memory.Statement)
			}

			if !slices.Equal(statements, testCase.want) {
				return fmt.Errorf("%s: in effect %q, want %q", name, statements, testCase.want)
			}
		}

		return nil
	})
}

func TestARejectedProposalWasNeverInEffect(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := memoryrepo.New(client)
		revisions := New(client)
		by := entity.KnowledgeAttribution{AccountID: fixture.authorID, Source: entity.RevisionSourceAgent}
		start := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

		proposal := draftMemory(fixture, "Deploy from a laptop", entity.WorkspaceAudience(), start)
		proposal.Status = entity.MemoryStatusProposed

		if err := memories.Create(ctx, proposal); err != nil {
			return fmt.Errorf("create the proposal: %w", err)
		}

		if err := revisions.Record(ctx, proposal.Revision(entity.MemoryRevisionCreated, by)); err != nil {
			return err
		}

		rejected := proposal
		rejected.Version, rejected.Status, rejected.UpdatedAt = 2, entity.MemoryStatusRejected, start.AddDate(0, 1, 0)

		if err := memories.Update(ctx, rejected, 1); err != nil {
			return fmt.Errorf("reject the proposal: %w", err)
		}

		if err := revisions.Record(ctx, rejected.Rejection("unsafe", by)); err != nil {
			return err
		}

		found, err := revisions.InEffect(ctx, fixture.workspaceID, repository.KnowledgeQuery{
			Query:            entity.ParseSearchQuery("how do we deploy"),
			Statuses:         entity.GuidanceStatuses(true),
			On:               start.AddDate(0, 0, 7),
			PersonalAuthorID: fixture.authorID,
			Limit:            entity.KnowledgeLaneSize,
		})
		if err != nil {
			return err
		}

		if len(found) != 0 {
			return fmt.Errorf("a rejected proposal was in effect before its rejection: %+v", found)
		}

		return nil
	})
}
