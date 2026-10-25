package retrieval_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	aiproviderrepo "github.com/usenorn/norn/internal/repository/aiprovider"
	docrepo "github.com/usenorn/norn/internal/repository/doc"
	docpassagerepo "github.com/usenorn/norn/internal/repository/docpassage"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	knowledgerepo "github.com/usenorn/norn/internal/repository/knowledge"
	memoryrepo "github.com/usenorn/norn/internal/repository/memory"
	memoryrevisionrepo "github.com/usenorn/norn/internal/repository/memoryrevision"
	"github.com/usenorn/norn/internal/service"
	knowledgeauthorizersvc "github.com/usenorn/norn/internal/service/knowledgeauthorizer"
	knowledgeembeddingsvc "github.com/usenorn/norn/internal/service/knowledgeembedding"
	knowledgeindexsvc "github.com/usenorn/norn/internal/service/knowledgeindex"
	retrievalsvc "github.com/usenorn/norn/internal/service/retrieval"
)

var updateBaseline = flag.Bool("update", false, "rewrite testdata/retrieval_baseline.json from this run")

const (
	corpusPath       = "testdata/retrieval_corpus.json"
	baselinePath     = "testdata/retrieval_baseline.json"
	corpusModel      = "concept-lexicon"
	corpusNoiseDims  = 64
	corpusNoiseScale = 0.05
	corpusRankDepth  = 10
	corpusRecallAt   = 5
	corpusTolerance  = 1e-9
)

var errCorpusRollback = errors.New("roll the retrieval corpus back")

type corpus struct {
	Concepts     map[string][]string `json:"concepts"`
	Memories     []corpusMemory      `json:"memories"`
	Docs         []corpusDoc         `json:"docs"`
	Questions    []corpusQuestion    `json:"questions"`
	Enumerations []corpusEnumeration `json:"enumerations"`
}

type corpusMemory struct {
	Key                string `json:"key"`
	Kind               string `json:"kind"`
	Statement          string `json:"statement"`
	Rationale          string `json:"rationale"`
	Visibility         string `json:"visibility"`
	Project            string `json:"project"`
	Applies            string `json:"applies"`
	Component          string `json:"component"`
	Status             string `json:"status"`
	Author             string `json:"author"`
	SupersededBy       string `json:"supersededBy"`
	EffectiveFromDays  *int   `json:"effectiveFromDays"`
	EffectiveUntilDays *int   `json:"effectiveUntilDays"`
}

type corpusDoc struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	Visibility string `json:"visibility"`
	Project    string `json:"project"`
	Author     string `json:"author"`
	Archived   bool   `json:"archived"`
	Markdown   string `json:"markdown"`
}

type corpusQuestion struct {
	Key             string   `json:"key"`
	Type            string   `json:"type"`
	Reader          string   `json:"reader"`
	Query           string   `json:"query"`
	Project         string   `json:"project"`
	Component       string   `json:"component"`
	IncludeProposed bool     `json:"includeProposed"`
	AsOfDays        *int     `json:"asOfDays"`
	Expect          []string `json:"expect"`
	Forbid          []string `json:"forbid"`
	First           string   `json:"first"`
	Proposed        []string `json:"proposed"`
}

type corpusEnumeration struct {
	Key         string   `json:"key"`
	Reader      string   `json:"reader"`
	MemoryKinds []string `json:"memoryKinds"`
	Project     string   `json:"project"`
	Expect      []string `json:"expect"`
}

type rankingMetrics struct {
	Questions int     `json:"questions"`
	RecallAt5 float64 `json:"recallAt5"`
	MRR       float64 `json:"mrr"`
	NDCGAt10  float64 `json:"ndcgAt10"`
	EmptyRate float64 `json:"emptyRate,omitempty"`
}

type corpusBaseline map[string]map[string]rankingMetrics

type conceptEmbedder struct {
	concepts []string
	terms    map[string][]string
}

func newConceptEmbedder(concepts map[string][]string) *conceptEmbedder {
	names := make([]string, 0, len(concepts))
	for name := range concepts {
		names = append(names, name)
	}

	slices.Sort(names)

	return &conceptEmbedder{concepts: names, terms: concepts}
}

func (e *conceptEmbedder) vector(text string) []float32 {
	lower := strings.ToLower(text)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})

	vector := make([]float32, 0, len(e.concepts)+corpusNoiseDims)

	for _, concept := range e.concepts {
		var weight float32

		for _, term := range e.terms[concept] {
			if strings.Contains(term, " ") {
				weight += float32(strings.Count(lower, term))

				continue
			}

			for _, token := range tokens {
				if token == term || (len(term) >= 4 && strings.HasPrefix(token, term)) {
					weight++
				}
			}
		}

		vector = append(vector, weight)
	}

	for dimension := range corpusNoiseDims {
		hash := fnv.New64a()
		_, _ = fmt.Fprintf(hash, "%d:%s", dimension, lower)
		vector = append(vector, float32(int64(hash.Sum64()%2001)-1000)/1000*corpusNoiseScale)
	}

	return vector
}

func (e *conceptEmbedder) List(context.Context, entity.AIProviderEndpoint, string) (entity.AIProviderModels, error) {
	return entity.AIProviderModels{Embedding: []string{corpusModel}}, nil
}

func (e *conceptEmbedder) Probe(context.Context, entity.AIProviderEndpoint, string, string) error {
	return nil
}

func (e *conceptEmbedder) Converse(
	context.Context, entity.AIProviderEndpoint, string, entity.AIConversationRequest,
) (entity.AIConversationStep, error) {
	return entity.AIConversationStep{}, errors.New("the corpus embedder does not converse")
}

func (e *conceptEmbedder) Embed(
	_ context.Context, _ entity.AIProviderEndpoint, _, _ string, inputs []string,
) ([][]float32, error) {
	vectors := make([][]float32, len(inputs))
	for index, input := range inputs {
		vectors[index] = e.vector(input)
	}

	return vectors, nil
}

type corpusWorld struct {
	workspaceID uuid.UUID
	accounts    map[string]uuid.UUID
	projects    map[string]uuid.UUID
	keys        map[uuid.UUID]string
	docKeys     map[uuid.UUID]string
	readers     map[string]entity.KnowledgeReader
	now         time.Time
}

func corpusID(kind, key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("norn-retrieval-corpus/"+kind+"/"+key))
}

func (w corpusWorld) day(offset int) time.Time {
	return entity.MemoryDay(w.now.AddDate(0, 0, offset))
}

func loadCorpus(t *testing.T) corpus {
	t.Helper()

	raw, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	var loaded corpus
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}

	return loaded
}

func reachCorpusDatabase(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to run the corpus against")
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

	t.Cleanup(cleanup)

	return client
}

func TestTheRetrievalCorpusReturnsWhatEachQuestionNeeds(t *testing.T) {
	loaded := loadCorpus(t)
	client := reachCorpusDatabase(t)
	embedder := newConceptEmbedder(loaded.Concepts)
	ctrl := gomock.NewController(t)

	var (
		world    corpusWorld
		reader   entity.KnowledgeReader
		embeds   bool
		measured = corpusBaseline{}
	)

	readers := knowledgeauthorizersvc.NewMockKnowledgeAuthorizer(ctrl)
	readers.EXPECT().Reader(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID) (entity.KnowledgeDecision, error) {
			return entity.KnowledgeDecision{Reader: reader}, nil
		}).
		AnyTimes()

	providers := aiproviderrepo.NewMockAIProvider(ctrl)
	providers.EXPECT().Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID uuid.UUID) (entity.AIProviderConnection, error) {
			if !embeds {
				return entity.AIProviderConnection{}, entity.ErrAIProviderNotConfigured
			}

			return entity.AIProviderConnection{WorkspaceID: workspaceID, EmbeddingModel: corpusModel}, nil
		}).
		AnyTimes()
	providers.EXPECT().APIKey(gomock.Any(), gomock.Any()).Return("corpus", nil).AnyTimes()

	indexes := knowledgeindexsvc.New(
		docrepo.New(client), docpassagerepo.New(client), knowledgerepo.NewEmbeddings(client), providers, embedder,
		jobqueuerepo.NewMockJobProducer(ctrl), client,
	)

	cfg := config.Knowledge{QueryTimeout: 5 * time.Second, SemanticMinSimilarity: 0.3}
	retrievals := retrievalsvc.New(
		knowledgerepo.New(client), memoryrepo.New(client), memoryrevisionrepo.New(client),
		knowledgeembeddingsvc.New(providers, embedder, cfg), readers, cfg,
	)

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		var err error

		if world, err = layCorpus(ctx, client, loaded); err != nil {
			return err
		}

		embeds = true

		if err := indexes.Refresh(ctx, world.workspaceID); err != nil {
			return fmt.Errorf("refresh the corpus index: %w", err)
		}

		for _, mode := range []string{"hybrid", "lexical"} {
			embeds = mode == "hybrid"
			measured[mode] = map[string]rankingMetrics{}

			for _, question := range loaded.Questions {
				reader = world.readers[question.Reader]

				ranked, err := ask(ctx, retrievals, world, question)
				if err != nil {
					return fmt.Errorf("%s/%s: %w", mode, question.Key, err)
				}

				judge(t, mode, question, ranked)
				record(measured[mode], question, ranked.keys)
			}

			finish(measured[mode])
		}

		for _, enumeration := range loaded.Enumerations {
			reader = world.readers[enumeration.Reader]

			if err := enumerate(ctx, t, retrievals, world, enumeration); err != nil {
				return fmt.Errorf("enumeration %s: %w", enumeration.Key, err)
			}
		}

		return errCorpusRollback
	})
	if !errors.Is(err, errCorpusRollback) {
		t.Fatalf("corpus: %v", err)
	}

	compareWithBaseline(t, measured)
}

type rankedAnswer struct {
	keys      []string
	proposed  []string
	retrieval entity.KnowledgeRetrieval
}

func ask(
	ctx context.Context,
	retrievals service.Retrievals,
	world corpusWorld,
	question corpusQuestion,
) (rankedAnswer, error) {
	input := service.RetrieveInput{
		Query:           question.Query,
		Component:       question.Component,
		IncludeProposed: question.IncludeProposed,
		Limit:           corpusRankDepth,
	}

	if question.Project != "" {
		projectID := world.projects[question.Project]
		input.ProjectID = &projectID
	}

	if question.AsOfDays != nil {
		at := world.now.AddDate(0, 0, *question.AsOfDays)
		input.AsOf = &at
	}

	retrieval, err := retrievals.Retrieve(ctx, world.workspaceID, input)
	if err != nil {
		return rankedAnswer{}, err
	}

	answer := rankedAnswer{retrieval: retrieval}

	for _, hit := range retrieval.Hits {
		key := world.keys[hit.ID]
		if hit.Passage != nil {
			key = world.docKeys[hit.Passage.DocID]
		}

		if key == "" {
			return rankedAnswer{}, fmt.Errorf("hit %s is not part of the corpus", hit.ID)
		}

		if hit.Proposed {
			answer.proposed = append(answer.proposed, key)
		}

		if !slices.Contains(answer.keys, key) {
			answer.keys = append(answer.keys, key)
		}
	}

	return answer, nil
}

func judge(t *testing.T, mode string, question corpusQuestion, answer rankedAnswer) {
	t.Helper()

	label := mode + "/" + question.Key

	for _, forbidden := range question.Forbid {
		if slices.Contains(answer.keys, forbidden) {
			t.Errorf("%s: returned %q, which this reader must never receive here (got %q)", label, forbidden, answer.keys)
		}
	}

	if question.Type == "no_evidence" && !answer.retrieval.Empty() {
		t.Errorf("%s: returned %q for a question nothing in the corpus answers", label, answer.keys)
	}

	if question.First != "" && (len(answer.keys) == 0 || answer.keys[0] != question.First) {
		t.Errorf("%s: ranked %q, want %q first as an exact match", label, answer.keys, question.First)
	}

	for _, proposed := range question.Proposed {
		if !slices.Contains(answer.proposed, proposed) {
			t.Errorf("%s: %q came back without its proposed label", label, proposed)
		}
	}

	if question.AsOfDays != nil && answer.retrieval.Semantic != entity.KnowledgeSemanticHistorical {
		t.Errorf("%s: semantic state %q for a historical question", label, answer.retrieval.Semantic)
	}

	if mode != "hybrid" && question.AsOfDays == nil {
		return
	}

	for _, expected := range question.Expect {
		if !slices.Contains(answer.keys, expected) {
			t.Errorf("%s: ranked %q, want %q among the first %d", label, answer.keys, expected, corpusRankDepth)
		}
	}
}

func record(metrics map[string]rankingMetrics, question corpusQuestion, keys []string) {
	if question.Type == "no_evidence" {
		for _, group := range []string{question.Type, "overall"} {
			current := metrics[group]
			current.Questions++

			if len(keys) == 0 {
				current.EmptyRate++
			}

			metrics[group] = current
		}

		return
	}

	if len(question.Expect) == 0 {
		return
	}

	var found, reciprocal, gain, ideal float64

	for rank, key := range keys {
		if !slices.Contains(question.Expect, key) {
			continue
		}

		if rank < corpusRecallAt {
			found++
		}

		if reciprocal == 0 {
			reciprocal = 1 / float64(rank+1)
		}

		if rank < corpusRankDepth {
			gain += 1 / math.Log2(float64(rank+2))
		}
	}

	for rank := range min(len(question.Expect), corpusRankDepth) {
		ideal += 1 / math.Log2(float64(rank+2))
	}

	for _, group := range []string{question.Type, "overall"} {
		current := metrics[group]
		current.Questions++
		current.RecallAt5 += found / float64(len(question.Expect))
		current.MRR += reciprocal
		current.NDCGAt10 += gain / ideal
		metrics[group] = current
	}
}

func finish(metrics map[string]rankingMetrics) {
	for group, current := range metrics {
		ranked := float64(current.Questions)
		if group == "no_evidence" {
			current.EmptyRate = round(current.EmptyRate / ranked)
			metrics[group] = current

			continue
		}

		if group == "overall" {
			emptyQuestions := float64(metrics["no_evidence"].Questions)
			ranked -= emptyQuestions
			current.EmptyRate = round(current.EmptyRate / math.Max(emptyQuestions, 1))
		}

		current.RecallAt5 = round(current.RecallAt5 / ranked)
		current.MRR = round(current.MRR / ranked)
		current.NDCGAt10 = round(current.NDCGAt10 / ranked)
		metrics[group] = current
	}
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func compareWithBaseline(t *testing.T, measured corpusBaseline) {
	t.Helper()

	if *updateBaseline {
		encoded, err := json.MarshalIndent(measured, "", "  ")
		if err != nil {
			t.Fatalf("encode baseline: %v", err)
		}

		if err := os.WriteFile(filepath.Clean(baselinePath), append(encoded, '\n'), 0o644); err != nil {
			t.Fatalf("write baseline: %v", err)
		}

		return
	}

	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v; run the test with -update to record one", err)
	}

	var recorded corpusBaseline
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatalf("decode baseline: %v", err)
	}

	for mode, groups := range recorded {
		for group, want := range groups {
			got := measured[mode][group]

			for _, metric := range []struct {
				name      string
				got, want float64
			}{
				{"recall@5", got.RecallAt5, want.RecallAt5},
				{"mrr", got.MRR, want.MRR},
				{"ndcg@10", got.NDCGAt10, want.NDCGAt10},
				{"empty rate", got.EmptyRate, want.EmptyRate},
			} {
				switch {
				case metric.got < metric.want-corpusTolerance:
					t.Errorf("%s/%s %s fell to %.4f from the recorded %.4f", mode, group, metric.name, metric.got, metric.want)
				case metric.got > metric.want+corpusTolerance:
					t.Logf("%s/%s %s rose to %.4f from %.4f; record it with -update", mode, group, metric.name, metric.got, metric.want)
				}
			}

			if got.Questions != want.Questions {
				t.Errorf("%s/%s measured %d questions, the baseline %d; record it with -update", mode, group, got.Questions, want.Questions)
			}
		}
	}
}

func enumerate(
	ctx context.Context,
	t *testing.T,
	retrievals service.Retrievals,
	world corpusWorld,
	enumeration corpusEnumeration,
) error {
	input := service.EnumerateInput{Limit: 3}

	for _, kind := range enumeration.MemoryKinds {
		input.MemoryKinds = append(input.MemoryKinds, entity.MemoryKind(kind))
	}

	if enumeration.Project != "" {
		projectID := world.projects[enumeration.Project]
		input.ProjectID = &projectID
	}

	var got []string

	for pages := 0; ; pages++ {
		if pages > len(enumeration.Expect) {
			return errors.New("enumeration never reached its last page")
		}

		list, err := retrievals.Enumerate(ctx, world.workspaceID, input)
		if err != nil {
			return err
		}

		for _, memory := range list.Memories {
			key := world.keys[memory.ID]
			if slices.Contains(got, key) {
				t.Errorf("enumeration %s listed %q twice", enumeration.Key, key)
			}

			got = append(got, key)
		}

		if list.NextCursor == "" {
			break
		}

		input.Cursor = list.NextCursor
	}

	want := slices.Clone(enumeration.Expect)
	slices.Sort(want)
	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Errorf("enumeration %s listed %q, want exactly %q", enumeration.Key, got, want)
	}

	return nil
}

const corpusWorldQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Retrieval corpus') RETURNING id
), people AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC'), ($5, 'active', 'person', $6, 'Sol', 'UTC')
    RETURNING id
), platform AS (
    INSERT INTO workspace_projects (workspace_id, slug, name) SELECT id, 'platform', 'Platform' FROM workspace RETURNING id
), billing AS (
    INSERT INTO workspace_projects (workspace_id, slug, name) SELECT id, 'billing', 'Billing' FROM workspace RETURNING id
)
SELECT platform.id, billing.id FROM platform, billing, (SELECT count(*) FROM people) AS laid`

const corpusDocQuery = `
INSERT INTO workspace_docs (id, workspace_id, title, content, markdown, visibility, project_id, author_account_id,
                            status, archived_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

func layCorpus(ctx context.Context, client *postgres.Client, loaded corpus) (corpusWorld, error) {
	world := corpusWorld{
		workspaceID: uuid.New(),
		accounts:    map[string]uuid.UUID{"rae": uuid.New(), "sol": uuid.New()},
		projects:    map[string]uuid.UUID{},
		keys:        map[uuid.UUID]string{},
		docKeys:     map[uuid.UUID]string{},
		now:         time.Now().UTC(),
	}

	var platform, billing uuid.UUID

	if err := client.Querier(ctx).QueryRowContext(
		ctx, corpusWorldQuery,
		world.workspaceID, "corpus-"+world.workspaceID.String()[:8],
		world.accounts["rae"], world.accounts["rae"].String()+"@example.test",
		world.accounts["sol"], world.accounts["sol"].String()+"@example.test",
	).Scan(&platform, &billing); err != nil {
		return corpusWorld{}, fmt.Errorf("lay the corpus workspace: %w", err)
	}

	world.projects["platform"], world.projects["billing"] = platform, billing
	world.readers = map[string]entity.KnowledgeReader{
		"rae": {
			WorkspaceID: world.workspaceID,
			Identity:    entity.KnowledgeIdentity{Principal: world.accounts["rae"], Personal: true},
			ProjectIDs:  []uuid.UUID{platform},
		},
		"sol": {
			WorkspaceID: world.workspaceID,
			Identity:    entity.KnowledgeIdentity{Principal: world.accounts["sol"], Personal: true},
			ProjectIDs:  []uuid.UUID{billing},
		},
		"agent": {
			WorkspaceID: world.workspaceID,
			Identity:    entity.KnowledgeIdentity{Principal: uuid.New(), Shared: true},
		},
	}

	if err := layMemories(ctx, client, &world, loaded.Memories); err != nil {
		return corpusWorld{}, err
	}

	for _, doc := range loaded.Docs {
		if err := layDoc(ctx, client, &world, doc); err != nil {
			return corpusWorld{}, err
		}
	}

	return world, nil
}

func layMemories(ctx context.Context, client *postgres.Client, world *corpusWorld, memories []corpusMemory) error {
	store := memoryrepo.New(client)
	revisions := memoryrevisionrepo.New(client)
	written := world.now.AddDate(0, 0, -120)
	replaced := world.now.AddDate(0, 0, -60)

	replacements := map[string]bool{}
	for _, memory := range memories {
		if memory.SupersededBy != "" {
			replacements[memory.SupersededBy] = true
		}
	}

	byKey := map[string]entity.Memory{}

	for _, laid := range memories {
		at := written
		if replacements[laid.Key] {
			at = replaced
		}

		memory := world.memory(laid, at)
		memory.ID = corpusID("memory", laid.Key)

		if err := store.Create(ctx, memory); err != nil {
			return fmt.Errorf("create memory %s: %w", laid.Key, err)
		}

		if err := revisions.Record(ctx, memory.Revision(entity.MemoryRevisionCreated, memory.By)); err != nil {
			return fmt.Errorf("record memory %s: %w", laid.Key, err)
		}

		world.keys[memory.ID] = laid.Key
		byKey[laid.Key] = memory
	}

	for _, laid := range memories {
		if laid.SupersededBy == "" {
			continue
		}

		memory := byKey[laid.Key]
		replacementID := byKey[laid.SupersededBy].ID

		if err := memory.SupersededBy(replacementID); err != nil {
			return fmt.Errorf("supersede memory %s: %w", laid.Key, err)
		}

		memory.Version++
		memory.UpdatedAt = replaced

		if err := store.Update(ctx, memory, memory.Version-1); err != nil {
			return fmt.Errorf("save superseded memory %s: %w", laid.Key, err)
		}

		if err := revisions.Record(ctx, memory.Revision(entity.MemoryRevisionSuperseded, memory.By)); err != nil {
			return fmt.Errorf("record superseded memory %s: %w", laid.Key, err)
		}
	}

	return nil
}

func (w corpusWorld) memory(laid corpusMemory, at time.Time) entity.Memory {
	author := w.accounts["rae"]
	if laid.Author != "" {
		author = w.accounts[laid.Author]
	}

	memory := entity.Memory{
		ID:            uuid.New(),
		WorkspaceID:   w.workspaceID,
		Kind:          entity.MemoryKind(laid.Kind),
		Statement:     laid.Statement,
		Rationale:     laid.Rationale,
		Audience:      entity.WorkspaceAudience(),
		Status:        entity.MemoryStatusAccepted,
		Basis:         entity.MemoryBasisObservation,
		AuthorID:      author,
		By:            entity.KnowledgeAttribution{AccountID: author, Source: entity.RevisionSourcePerson},
		EffectiveFrom: entity.MemoryDay(at),
		Version:       1,
		CreatedAt:     at,
		UpdatedAt:     at,
	}

	switch laid.Visibility {
	case "personal":
		memory.Audience = entity.PersonalAudience()
	case "project":
		memory.Audience = entity.ProjectAudience(w.projects[laid.Project])
	}

	if laid.Status != "" {
		memory.Status = entity.MemoryStatus(laid.Status)
	}

	if laid.Applies != "" {
		projectID := w.projects[laid.Applies]
		memory.Applicability.ProjectID = &projectID
	}

	memory.Applicability.Component = laid.Component

	if laid.EffectiveFromDays != nil {
		memory.EffectiveFrom = w.day(*laid.EffectiveFromDays)
	}

	if laid.EffectiveUntilDays != nil {
		until := w.day(*laid.EffectiveUntilDays)
		memory.EffectiveUntil = &until
	}

	return memory
}

func layDoc(ctx context.Context, client *postgres.Client, world *corpusWorld, laid corpusDoc) error {
	document := entity.DocumentFromMarkdown(laid.Markdown)

	content, err := document.Encode()
	if err != nil {
		return fmt.Errorf("encode doc %s: %w", laid.Key, err)
	}

	visibility := laid.Visibility
	if visibility == "" {
		visibility = string(entity.KnowledgeVisibilityWorkspace)
	}

	var project any
	if laid.Project != "" {
		project = world.projects[laid.Project]
	}

	author := world.accounts["rae"]
	if laid.Author != "" {
		author = world.accounts[laid.Author]
	}

	status, archivedAt := string(entity.DocStatusActive), any(nil)
	if laid.Archived {
		status, archivedAt = string(entity.DocStatusArchived), world.now
	}

	docID := corpusID("doc", laid.Key)

	if _, err := client.Querier(ctx).ExecContext(
		ctx, corpusDocQuery,
		docID, world.workspaceID, laid.Title, string(content), document.Markdown(), visibility, project, author,
		status, archivedAt,
	); err != nil {
		return fmt.Errorf("insert doc %s: %w", laid.Key, err)
	}

	world.docKeys[docID] = laid.Key

	return nil
}
