package knowledge

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

func guidance(fixture knowledgeFixture, text string) repository.KnowledgeQuery {
	return repository.KnowledgeQuery{
		Query:            entity.ParseSearchQuery(text),
		Statuses:         entity.GuidanceStatuses(false),
		On:               time.Now().UTC(),
		PersonalAuthorID: fixture.authorID,
		Limit:            entity.KnowledgeLaneSize,
	}
}

func ids(candidates []entity.KnowledgeCandidate) []uuid.UUID {
	found := make([]uuid.UUID, len(candidates))
	for index, candidate := range candidates {
		found[index] = candidate.ID
	}

	return found
}

func TestLexicalMemoriesKeepOnlyCurrentGuidanceTheReaderMaySee(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		knowledge := &knowledgeRepository{db: client}
		now := time.Now().UTC()
		yesterday, tomorrow := now.AddDate(0, 0, -1), now.AddDate(0, 0, 1)

		memories := map[string]fixtureMemory{
			"accepted":       {statement: "Deploys go out on Tuesdays"},
			"proposed":       {statement: "Deploys go out on Thursdays", status: "proposed"},
			"withdrawn":      {statement: "Deploys go out on Mondays", status: "withdrawn"},
			"personal":       {statement: "Deploys feel risky to me", visibility: "personal"},
			"others":         {statement: "Deploys need my sign-off", visibility: "personal", authorID: fixture.outsiderID},
			"other project":  {statement: "Deploys freeze during billing runs", visibility: "project", projectID: fixture.otherProjectID},
			"not yet":        {statement: "Deploys move to Fridays", effectiveFrom: tomorrow},
			"expired":        {statement: "Deploys paused for the audit", effectiveFrom: yesterday.AddDate(0, 0, -7), effectiveUntil: &yesterday},
			"applies there":  {statement: "Deploys of billing need a dry run", appliesProjectID: fixture.otherProjectID},
			"component only": {statement: "Deploys of the worker drain queues first", component: "worker"},
		}

		created := map[string]uuid.UUID{}

		for name, memory := range memories {
			id, err := fixture.memory(ctx, client, memory)
			if err != nil {
				return err
			}

			created[name] = id
		}

		found, err := knowledge.LexicalMemories(ctx, fixture.workspaceID, guidance(fixture, "deploys"))
		if err != nil {
			return err
		}

		want := []string{"accepted", "applies there", "component only", "personal"}
		if err := sameNames(created, ids(found), want); err != nil {
			return fmt.Errorf("current guidance for the author: %w", err)
		}

		proposals := guidance(fixture, "deploys")
		proposals.Statuses = entity.GuidanceStatuses(true)

		found, err = knowledge.LexicalMemories(ctx, fixture.workspaceID, proposals)
		if err != nil {
			return err
		}

		if !slices.Contains(ids(found), created["proposed"]) {
			return fmt.Errorf("a proposal is missing when proposals were asked for")
		}

		scoped := guidance(fixture, "deploys")
		scoped.ProjectID = &fixture.projectID
		scoped.Component = "api"

		found, err = knowledge.LexicalMemories(ctx, fixture.workspaceID, scoped)
		if err != nil {
			return err
		}

		if err := sameNames(created, ids(found), []string{"accepted", "personal"}); err != nil {
			return fmt.Errorf("guidance for the platform api: %w", err)
		}

		outsider := guidance(fixture, "deploys")
		outsider.PersonalAuthorID = fixture.outsiderID
		outsider.AllProjects = true

		found, err = knowledge.LexicalMemories(ctx, fixture.workspaceID, outsider)
		if err != nil {
			return err
		}

		if err := sameNames(created, ids(found), []string{"accepted", "applies there", "component only", "other project", "others"}); err != nil {
			return fmt.Errorf("guidance for another reader: %w", err)
		}

		return nil
	})
}

func TestAnExactStatementRanksFirstAndAnyWordOfTheQuestionCanMatch(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		knowledge := &knowledgeRepository{db: client}

		exact, err := fixture.memory(ctx, client, fixtureMemory{statement: "Use Postgres"})
		if err != nil {
			return err
		}

		wordy, err := fixture.memory(ctx, client, fixtureMemory{
			statement: "Use Postgres for every service that stores state, Postgres first",
		})
		if err != nil {
			return err
		}

		partial, err := fixture.memory(ctx, client, fixtureMemory{statement: "Queues run on Valkey"})
		if err != nil {
			return err
		}

		found, err := knowledge.LexicalMemories(ctx, fixture.workspaceID, guidance(fixture, "use postgres"))
		if err != nil {
			return err
		}

		if len(found) < 2 || found[0].ID != exact || !found[0].Exact || found[1].ID != wordy || found[1].Exact {
			return fmt.Errorf("found %+v, want the exact statement first and flagged", found)
		}

		found, err = knowledge.LexicalMemories(ctx, fixture.workspaceID, guidance(fixture, "where do queues and postgres live"))
		if err != nil {
			return err
		}

		if !slices.Contains(ids(found), partial) || !slices.Contains(ids(found), wordy) {
			return fmt.Errorf("found %v, want every memory that shares a word with the question", ids(found))
		}

		byID, err := knowledge.LexicalMemories(ctx, fixture.workspaceID, guidance(fixture, partial.String()))
		if err != nil {
			return err
		}

		if len(byID) != 1 || byID[0].ID != partial || !byID[0].Exact {
			return fmt.Errorf("asking by id found %+v, want only that memory as an exact match", byID)
		}

		return nil
	})
}

func TestLexicalPassagesFilterDocsByAudienceAndExcerptTheMatch(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		knowledge := &knowledgeRepository{db: client}

		runbook, err := fixture.doc(ctx, client, "Runbook", "workspace", uuid.Nil, fixture.authorID,
			[2]string{"Runbook", "Start here."},
			[2]string{"Runbook › Rollback", "Revert the release tag and redeploy the previous image."},
		)
		if err != nil {
			return err
		}

		if _, err := fixture.doc(ctx, client, "Billing rollback", "project", fixture.otherProjectID, fixture.authorID,
			[2]string{"Billing rollback", "Rollback billing by replaying the ledger."}); err != nil {
			return err
		}

		if _, err := fixture.doc(ctx, client, "My rollback notes", "personal", uuid.Nil, fixture.outsiderID,
			[2]string{"My rollback notes", "Rollback scares me."}); err != nil {
			return err
		}

		found, err := knowledge.LexicalPassages(ctx, fixture.workspaceID, guidance(fixture, "how do I roll back a release"))
		if err != nil {
			return err
		}

		if len(found) != 1 || found[0].Passage.DocID != runbook || found[0].Passage.Heading != "Runbook › Rollback" {
			return fmt.Errorf("found %+v, want only the workspace runbook's rollback passage", found)
		}

		if passage := found[0].Passage; !strings.Contains(passage.Excerpt, "Revert the release tag") ||
			passage.Audience.Visibility != entity.KnowledgeVisibilityWorkspace || passage.WorkspaceID != fixture.workspaceID {
			return fmt.Errorf("passage = %+v", passage)
		}

		titled, err := knowledge.LexicalPassages(ctx, fixture.workspaceID, guidance(fixture, "runbook"))
		if err != nil {
			return err
		}

		if len(titled) == 0 || !titled[0].Exact || titled[0].Passage.Position != 0 {
			return fmt.Errorf("asking for the title found %+v, want its first passage as an exact match", titled)
		}

		member := guidance(fixture, "rollback")
		member.ProjectIDs = []uuid.UUID{fixture.otherProjectID}

		found, err = knowledge.LexicalPassages(ctx, fixture.workspaceID, member)
		if err != nil {
			return err
		}

		var titles []string
		for _, candidate := range found {
			titles = append(titles, candidate.Passage.DocTitle)
		}

		slices.Sort(titles)

		if !slices.Equal(titles, []string{"Billing rollback", "Runbook"}) {
			return fmt.Errorf("a billing member found %q", titles)
		}

		return nil
	})
}

func TestSemanticLanesRankByMeaningWithinTheReadersAudience(t *testing.T) {
	withKnowledgeFixture(t, func(ctx context.Context, client *postgres.Client, fixture knowledgeFixture) error {
		knowledge := &knowledgeRepository{db: client}
		embeddings := &embeddingRepository{db: client}

		near, err := fixture.memory(ctx, client, fixtureMemory{statement: "Ship on Tuesdays"})
		if err != nil {
			return err
		}

		farther, err := fixture.memory(ctx, client, fixtureMemory{statement: "Releases need a changelog"})
		if err != nil {
			return err
		}

		if _, err := fixture.memory(ctx, client, fixtureMemory{statement: "Lunch is at noon"}); err != nil {
			return err
		}

		if _, err := fixture.memory(ctx, client, fixtureMemory{
			statement: "Ship billing on Wednesdays", visibility: "project", projectID: fixture.otherProjectID,
		}); err != nil {
			return err
		}

		if _, err := fixture.doc(ctx, client, "Release guide", "workspace", uuid.Nil, fixture.authorID,
			[2]string{"Release guide", "Cut the release branch on Monday."}); err != nil {
			return err
		}

		vectors := map[string][]float32{
			"Ship on Tuesdays\n\n":                             {1, 0, 0},
			"Releases need a changelog\n\n":                    {0.8, 0.6, 0},
			"Lunch is at noon\n\n":                             {0, 0, 1},
			"Ship billing on Wednesdays\n\n":                   {1, 0, 0},
			"Release guide\nCut the release branch on Monday.": {0.9, 0.1, 0},
		}

		stale, err := embeddings.Stale(ctx, fixture.workspaceID, embeddingModel, entity.KnowledgeStaleBatchSize)
		if err != nil {
			return err
		}

		var saved []entity.KnowledgeEmbedding
		for _, text := range stale {
			vector, known := vectors[text.Text]
			if !known {
				return fmt.Errorf("no vector for %q", text.Text)
			}

			saved = append(saved, entity.KnowledgeEmbedding{ContentHash: text.ContentHash, Vector: vector})
		}

		if err := embeddings.Save(ctx, fixture.workspaceID, embeddingModel, saved); err != nil {
			return err
		}

		question := repository.KnowledgeVector{Model: embeddingModel, Vector: []float32{1, 0.05, 0}, MinSimilarity: 0.5}

		found, err := knowledge.SemanticMemories(ctx, fixture.workspaceID, guidance(fixture, "when do we ship"), question)
		if err != nil {
			return err
		}

		if !slices.Equal(ids(found), []uuid.UUID{near, farther}) {
			return fmt.Errorf("found %v, want the nearest first, the unrelated one below the floor and the billing one hidden", ids(found))
		}

		passages, err := knowledge.SemanticPassages(ctx, fixture.workspaceID, guidance(fixture, "when do we ship"), question)
		if err != nil {
			return err
		}

		if len(passages) != 1 || passages[0].Passage.Excerpt != "Cut the release branch on Monday." {
			return fmt.Errorf("passages = %+v, want the release guide with its body as the excerpt", passages)
		}

		other := question
		other.Model = "another-model"

		if found, err := knowledge.SemanticMemories(ctx, fixture.workspaceID, guidance(fixture, "ship"), other); err != nil || len(found) != 0 {
			return fmt.Errorf("another model found %v, %v; vectors from one model are never compared with another", found, err)
		}

		return nil
	})
}

func sameNames(created map[string]uuid.UUID, found []uuid.UUID, want []string) error {
	names := make([]string, 0, len(found))

	for _, id := range found {
		for name, createdID := range created {
			if createdID == id {
				names = append(names, name)
			}
		}
	}

	slices.Sort(names)

	if !slices.Equal(names, want) {
		return fmt.Errorf("found %q, want %q", names, want)
	}

	return nil
}
