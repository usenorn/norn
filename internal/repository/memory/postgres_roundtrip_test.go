package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

var errMemoryRollback = errors.New("roll the memory fixture back")

const memoryFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Memory fixture')
    RETURNING id
), author AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC')
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'platform', 'Platform' FROM workspace
    RETURNING id
), doc AS (
    INSERT INTO workspace_docs (id, workspace_id, title, content, visibility, author_account_id)
    SELECT $5, workspace.id, 'Storage notes', '{"type":"doc"}', 'personal', author.id FROM workspace, author
    RETURNING id, workspace_id
), revision AS (
    INSERT INTO workspace_doc_revisions (id, workspace_id, doc_id, version, action, title, content, visibility, source)
    SELECT $6, workspace_id, id, 1, 'created', 'Storage notes', '{"type":"doc"}', 'personal', 'person' FROM doc
    RETURNING id
)
SELECT project.id FROM project, revision`

const tokenFixtureQuery = `
INSERT INTO api_tokens (id, account_id, name, token_hash, scopes)
VALUES ($1, $2, 'Memory agent', $3, '{knowledge:manage}')`

func layToken(ctx context.Context, client *postgres.Client, accountID uuid.UUID) (*uuid.UUID, error) {
	tokenID := uuid.New()
	if _, err := client.Querier(ctx).ExecContext(ctx, tokenFixtureQuery, tokenID, accountID, tokenID[:]); err != nil {
		return nil, fmt.Errorf("lay token fixture: %w", err)
	}

	return &tokenID, nil
}

type memoryFixture struct {
	workspaceID, authorID, projectID, docID, docRevisionID uuid.UUID
}

func withMemoryFixture(
	t *testing.T,
	run func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error,
) {
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
	defer cleanup()

	var failure error

	err = client.WithTx(context.Background(), func(ctx context.Context) error {
		fixture := memoryFixture{
			workspaceID:   uuid.New(),
			authorID:      uuid.New(),
			docID:         uuid.New(),
			docRevisionID: uuid.New(),
		}

		if err := client.Querier(ctx).QueryRowContext(
			ctx,
			memoryFixtureQuery,
			fixture.workspaceID,
			"memories-"+fixture.workspaceID.String()[:8],
			fixture.authorID,
			fixture.authorID.String()+"@example.test",
			fixture.docID,
			fixture.docRevisionID,
		).Scan(&fixture.projectID); err != nil {
			failure = fmt.Errorf("lay memory fixture: %w", err)

			return errMemoryRollback
		}

		failure = run(ctx, client, fixture)

		return errMemoryRollback
	})
	if !errors.Is(err, errMemoryRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func draftMemory(fixture memoryFixture, statement string, audience entity.KnowledgeAudience, at time.Time) entity.Memory {
	return entity.Memory{
		ID:            uuid.New(),
		WorkspaceID:   fixture.workspaceID,
		Kind:          entity.MemoryKindConstraint,
		Statement:     statement,
		Conditions:    "local prototypes only",
		Audience:      audience,
		Status:        entity.MemoryStatusAccepted,
		Basis:         entity.MemoryBasisObservation,
		AuthorID:      fixture.authorID,
		By:            entity.KnowledgeAttribution{AccountID: fixture.authorID, Source: entity.RevisionSourcePerson},
		EffectiveFrom: entity.MemoryDay(at),
		Version:       1,
		CreatedAt:     at,
		UpdatedAt:     at,
	}
}

func TestAMemoryReadsBackWithItsEvidenceAndRefusesAStaleWrite(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
		until := entity.MemoryDay(at.AddDate(0, 3, 0))
		agentID := uuid.New()

		draft := draftMemory(fixture, "Use SQLite", entity.ProjectAudience(fixture.projectID), at)
		draft.Applicability = entity.KnowledgeApplicability{ProjectID: &fixture.projectID, Component: "storage"}
		draft.EffectiveUntil = &until
		draft.By.AgentID = &agentID

		tokenID, err := layToken(ctx, client, fixture.authorID)
		if err != nil {
			return err
		}

		draft.By.TokenID = tokenID
		draft.Evidence = []entity.MemoryEvidence{
			{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "the build broke on SQLite"},
			{Kind: entity.MemoryEvidenceDocRevision, DocID: &fixture.docID, DocRevisionID: &fixture.docRevisionID},
		}

		if err := memories.Create(ctx, draft); err != nil {
			return fmt.Errorf("create memory: %w", err)
		}

		created, err := memories.GetByID(ctx, fixture.workspaceID, draft.ID)
		if err != nil {
			return fmt.Errorf("read memory: %w", err)
		}

		if created.Statement != "Use SQLite" || created.Applicability.Component != "storage" ||
			created.Applicability.ProjectID == nil || *created.Applicability.ProjectID != fixture.projectID {
			return fmt.Errorf("memory read back as %+v", created)
		}

		if created.EffectiveUntil == nil || !created.EffectiveUntil.Equal(until) || !created.EffectiveFrom.Equal(entity.MemoryDay(at)) {
			return fmt.Errorf("effective dates read back as %v to %v", created.EffectiveFrom, created.EffectiveUntil)
		}

		if created.By.AgentID == nil || *created.By.AgentID != agentID || created.AuthorName != "Rae" ||
			created.By.TokenID == nil || *created.By.TokenID != *draft.By.TokenID {
			return fmt.Errorf("provenance read back as %+v by %q", created.By, created.AuthorName)
		}

		if len(created.Evidence) != 2 || created.Evidence[0].Excerpt != "the build broke on SQLite" {
			return fmt.Errorf("evidence read back as %+v", created.Evidence)
		}

		cited := created.Evidence[1]
		if cited.DocAudience.Visibility != entity.KnowledgeVisibilityPersonal || cited.DocAuthorID != fixture.authorID {
			return fmt.Errorf("a cited doc revision lost its doc's audience: %+v", cited)
		}

		corrected := created
		corrected.Version, corrected.Statement = 2, "Use SQLite for local prototypes"
		corrected.Evidence = created.Evidence[1:]

		if err := memories.Update(ctx, corrected, 1); err != nil {
			return fmt.Errorf("correct memory: %w", err)
		}

		if err := memories.Update(ctx, corrected, 1); !errors.Is(err, entity.ErrMemoryStale) {
			return fmt.Errorf("a write against version 1 after version 2 got %v, want ErrMemoryStale", err)
		}

		read, err := memories.GetByID(ctx, fixture.workspaceID, draft.ID)
		if err != nil {
			return fmt.Errorf("re-read memory: %w", err)
		}

		if read.Version != 2 || len(read.Evidence) != 1 || read.Evidence[0].Kind != entity.MemoryEvidenceDocRevision {
			return fmt.Errorf("the correction read back as version %d with evidence %+v", read.Version, read.Evidence)
		}

		return nil
	})
}

func TestTheMemoryListHonoursTheReaderAndItsFilters(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

		shared := draftMemory(fixture, "Ship weekly", entity.WorkspaceAudience(), at)
		project := draftMemory(fixture, "Freeze on Fridays", entity.ProjectAudience(fixture.projectID), at.Add(time.Minute))
		personal := draftMemory(fixture, "I prefer tabs", entity.PersonalAudience(), at.Add(2*time.Minute))
		personal.Kind = entity.MemoryKindPreference
		proposed := draftMemory(fixture, "Cache everything", entity.WorkspaceAudience(), at.Add(3*time.Minute))
		proposed.Status = entity.MemoryStatusProposed

		for _, memory := range []entity.Memory{shared, project, personal, proposed} {
			if err := memories.Create(ctx, memory); err != nil {
				return fmt.Errorf("create %q: %w", memory.Statement, err)
			}
		}

		page := entity.MemoryPage{Limit: 10}
		preference := entity.MemoryKindPreference

		for name, testCase := range map[string]struct {
			filter repository.MemoryFilter
			want   []string
		}{
			"a stranger to the project": {
				filter: repository.MemoryFilter{},
				want:   []string{"Cache everything", "Ship weekly"},
			},
			"the author on the project": {
				filter: repository.MemoryFilter{ProjectIDs: []uuid.UUID{fixture.projectID}, PersonalAuthorID: fixture.authorID},
				want:   []string{"Cache everything", "I prefer tabs", "Freeze on Fridays", "Ship weekly"},
			},
			"accepted only": {
				filter: repository.MemoryFilter{AllProjects: true, Statuses: []entity.MemoryStatus{entity.MemoryStatusAccepted}},
				want:   []string{"Freeze on Fridays", "Ship weekly"},
			},
			"preferences only": {
				filter: repository.MemoryFilter{Kinds: []entity.MemoryKind{preference}, PersonalAuthorID: fixture.authorID},
				want:   []string{"I prefer tabs"},
			},
		} {
			listed, err := memories.List(ctx, fixture.workspaceID, testCase.filter, page)
			if err != nil {
				return fmt.Errorf("%s: list: %w", name, err)
			}

			got := make([]string, 0, len(listed))
			for _, memory := range listed {
				got = append(got, memory.Statement)
			}

			if strings.Join(got, "|") != strings.Join(testCase.want, "|") {
				return fmt.Errorf("%s: listed %v, want %v", name, got, testCase.want)
			}
		}

		return nil
	})
}

func TestTheMemoryListNarrowsToAScopeAndToWhatAReviewerMaintains(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

		workspaceProposal := draftMemory(fixture, "Cache everything", entity.WorkspaceAudience(), at)
		workspaceProposal.Status = entity.MemoryStatusProposed
		projectProposal := draftMemory(fixture, "Freeze on Fridays", entity.ProjectAudience(fixture.projectID), at.Add(time.Minute))
		projectProposal.Status = entity.MemoryStatusDisputed
		personal := draftMemory(fixture, "I prefer tabs", entity.PersonalAudience(), at.Add(2*time.Minute))
		personal.Status = entity.MemoryStatusProposed

		for _, memory := range []entity.Memory{workspaceProposal, projectProposal, personal} {
			if err := memories.Create(ctx, memory); err != nil {
				return fmt.Errorf("create %q: %w", memory.Statement, err)
			}
		}

		reader := repository.MemoryFilter{AllProjects: true, PersonalAuthorID: fixture.authorID}
		narrowed := func(change func(*repository.MemoryFilter)) repository.MemoryFilter {
			filter := reader
			change(&filter)

			return filter
		}

		for name, testCase := range map[string]struct {
			filter repository.MemoryFilter
			want   []string
		}{
			"project and personal scopes": {
				filter: narrowed(func(f *repository.MemoryFilter) {
					f.Visibilities = []entity.KnowledgeVisibility{entity.KnowledgeVisibilityProject, entity.KnowledgeVisibilityPersonal}
				}),
				want: []string{"I prefer tabs", "Freeze on Fridays"},
			},
			"a workspace admin reviewing": {
				filter: narrowed(func(f *repository.MemoryFilter) {
					f.ReviewedBy = &entity.KnowledgeReview{Workspace: true}
				}),
				want: []string{"Freeze on Fridays", "Cache everything"},
			},
			"the project lead reviewing": {
				filter: narrowed(func(f *repository.MemoryFilter) {
					f.ReviewedBy = &entity.KnowledgeReview{LedProjectIDs: []uuid.UUID{fixture.projectID}}
				}),
				want: []string{"Freeze on Fridays"},
			},
			"the lead of another project reviewing": {
				filter: narrowed(func(f *repository.MemoryFilter) {
					f.ReviewedBy = &entity.KnowledgeReview{LedProjectIDs: []uuid.UUID{uuid.New()}}
				}),
				want: nil,
			},
		} {
			listed, err := memories.List(ctx, fixture.workspaceID, testCase.filter, entity.MemoryPage{Limit: 10})
			if err != nil {
				return fmt.Errorf("%s: list: %w", name, err)
			}

			got := make([]string, 0, len(listed))
			for _, memory := range listed {
				got = append(got, memory.Statement)
			}

			if strings.Join(got, "|") != strings.Join(testCase.want, "|") {
				return fmt.Errorf("%s: listed %v, want %v", name, got, testCase.want)
			}
		}

		return nil
	})
}

func TestEnumerationPagesThroughEveryInEffectMemoryInAStableOrder(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Now().UTC()
		tomorrow := entity.MemoryDay(at.AddDate(0, 0, 1))

		var want []uuid.UUID

		for index := range 7 {
			memory := draftMemory(fixture, fmt.Sprintf("Constraint %d", index), entity.WorkspaceAudience(), at)

			switch index {
			case 5:
				memory.EffectiveFrom = tomorrow
			case 6:
				memory.Kind = entity.MemoryKindLesson
			default:
				want = append(want, memory.ID)
			}

			if err := memories.Create(ctx, memory); err != nil {
				return err
			}
		}

		slices.SortFunc(want, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

		filter := repository.MemoryFilter{
			Kinds:      []entity.MemoryKind{entity.MemoryKindConstraint},
			Statuses:   []entity.MemoryStatus{entity.MemoryStatusAccepted},
			InEffectOn: &at,
		}

		var (
			got   []uuid.UUID
			after *uuid.UUID
		)

		for range len(want) {
			page, err := memories.Enumerate(ctx, fixture.workspaceID, filter, after, 2)
			if err != nil {
				return err
			}

			if len(page) == 0 {
				break
			}

			for _, memory := range page {
				got = append(got, memory.ID)
			}

			last := page[len(page)-1].ID
			after = &last
		}

		if !slices.Equal(got, want) {
			return fmt.Errorf("enumerated %v, want every in-effect constraint once, in id order: %v", got, want)
		}

		hydrated, err := memories.ListByIDs(ctx, fixture.workspaceID, []uuid.UUID{want[0], uuid.New()})
		if err != nil {
			return err
		}

		if len(hydrated) != 1 || hydrated[0].ID != want[0] || hydrated[0].Statement == "" {
			return fmt.Errorf("hydrated %+v, want only the memory that exists", hydrated)
		}

		return nil
	})
}

func TestLineageWalksBothWaysAlongSupersessionOldestFirst(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

		var chain []entity.Memory

		for _, statement := range []string{"Use SQLite", "Use Postgres", "Use Postgres with pgvector"} {
			memory := draftMemory(fixture, statement, entity.WorkspaceAudience(), at)

			if len(chain) > 0 {
				replaced := chain[len(chain)-1]
				memory.SupersedesID = &replaced.ID

				if err := memories.Create(ctx, memory); err != nil {
					return fmt.Errorf("create %q: %w", statement, err)
				}

				replaced.Status, replaced.SupersededByID, replaced.Version = entity.MemoryStatusSuperseded, &memory.ID, 2
				if err := memories.Update(ctx, replaced, 1); err != nil {
					return fmt.Errorf("supersede %q: %w", replaced.Statement, err)
				}

				chain[len(chain)-1] = replaced
			} else if err := memories.Create(ctx, memory); err != nil {
				return fmt.Errorf("create %q: %w", statement, err)
			}

			chain = append(chain, memory)
		}

		lineage, err := memories.Lineage(ctx, fixture.workspaceID, chain[1].ID, entity.MemoryLineageMax)
		if err != nil {
			return fmt.Errorf("trace lineage: %w", err)
		}

		var statements []string
		for _, memory := range lineage {
			statements = append(statements, memory.Statement)
		}

		if !slices.Equal(statements, []string{"Use SQLite", "Use Postgres", "Use Postgres with pgvector"}) {
			return fmt.Errorf("the lineage reads %q, want oldest first", statements)
		}

		capped, err := memories.Lineage(ctx, fixture.workspaceID, chain[0].ID, 1)
		if err != nil {
			return fmt.Errorf("trace a capped lineage: %w", err)
		}

		if len(capped) != 2 || capped[1].ID != chain[1].ID {
			return fmt.Errorf("a lineage capped at one step returned %d memories", len(capped))
		}

		return nil
	})
}

func TestAProposalLinkedOnReconciliationJoinsTheLineage(t *testing.T) {
	withMemoryFixture(t, func(ctx context.Context, client *postgres.Client, fixture memoryFixture) error {
		memories := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

		existing := draftMemory(fixture, "Production uses PostgreSQL", entity.WorkspaceAudience(), at)
		proposal := draftMemory(fixture, "Production uses MySQL", entity.WorkspaceAudience(), at.Add(time.Minute))
		proposal.Status = entity.MemoryStatusDisputed

		for _, memory := range []entity.Memory{existing, proposal} {
			if err := memories.Create(ctx, memory); err != nil {
				return fmt.Errorf("create %q: %w", memory.Statement, err)
			}
		}

		existing.Status, existing.SupersededByID, existing.Version = entity.MemoryStatusSuperseded, &proposal.ID, 2
		if err := memories.Update(ctx, existing, 1); err != nil {
			return fmt.Errorf("supersede the existing decision: %w", err)
		}

		proposal.Status, proposal.SupersedesID, proposal.Version = entity.MemoryStatusAccepted, &existing.ID, 2
		if err := memories.Update(ctx, proposal, 1); err != nil {
			return fmt.Errorf("accept the proposal: %w", err)
		}

		stored, err := memories.GetByID(ctx, fixture.workspaceID, proposal.ID)
		if err != nil {
			return fmt.Errorf("read the proposal: %w", err)
		}

		if stored.SupersedesID == nil || *stored.SupersedesID != existing.ID {
			return fmt.Errorf("the proposal supersedes %v, want %s", stored.SupersedesID, existing.ID)
		}

		lineage, err := memories.Lineage(ctx, fixture.workspaceID, existing.ID, entity.MemoryLineageMax)
		if err != nil {
			return fmt.Errorf("trace lineage: %w", err)
		}

		if len(lineage) != 2 || lineage[0].ID != existing.ID || lineage[1].ID != proposal.ID {
			return fmt.Errorf("the lineage holds %d memories, want the decision then its replacement", len(lineage))
		}

		return nil
	})
}
