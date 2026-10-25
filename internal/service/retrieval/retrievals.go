package retrieval

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type retrievals struct {
	knowledge repository.Knowledge
	memories  repository.Memory
	revisions repository.MemoryRevision
	providers repository.AIProvider
	models    repository.AIModel
	readers   service.KnowledgeAuthorizer
	cfg       config.Knowledge
	now       func() time.Time
}

func New(
	knowledge repository.Knowledge,
	memories repository.Memory,
	revisions repository.MemoryRevision,
	providers repository.AIProvider,
	models repository.AIModel,
	readers service.KnowledgeAuthorizer,
	cfg config.Knowledge,
) service.Retrievals {
	return &retrievals{
		knowledge: knowledge,
		memories:  memories,
		revisions: revisions,
		providers: providers,
		models:    models,
		readers:   readers,
		cfg:       cfg,
		now:       time.Now,
	}
}

func (s *retrievals) Retrieve(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.RetrieveInput,
) (entity.KnowledgeRetrieval, error) {
	query := entity.ParseSearchQuery(input.Query)

	if query.Raw == "" {
		return entity.KnowledgeRetrieval{}, entity.NewValidationError(entity.FieldError{
			Field: entity.KnowledgeFieldQuery, Code: entity.ValidationCodeRequired,
		})
	}

	kinds, err := subjectKinds(input.Kinds)
	if err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	if err := validMemoryKinds(input.MemoryKinds); err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	decision, err := s.readers.Reader(ctx, workspaceID)
	if err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	reader := decision.Reader
	lookup := repository.KnowledgeQuery{
		Query:            query,
		MemoryKinds:      input.MemoryKinds,
		Statuses:         entity.GuidanceStatuses(input.IncludeProposed),
		On:               s.now().UTC(),
		ProjectID:        input.ProjectID,
		Component:        strings.TrimSpace(input.Component),
		AllProjects:      reader.AllProjects,
		ProjectIDs:       reader.ProjectIDs,
		PersonalAuthorID: reader.PersonalAuthor(),
		Limit:            entity.KnowledgeLaneSize,
	}

	limit := clamp(input.Limit, entity.KnowledgeRetrieveDefaultSize, entity.KnowledgeRetrieveMaxSize)

	if input.AsOf != nil {
		lookup.On = input.AsOf.UTC()

		return s.history(ctx, workspaceID, reader, kinds, lookup, limit)
	}

	retrieval := entity.KnowledgeRetrieval{Scope: scopeOf(kinds, lookup, false)}

	lanes, err := s.lexical(ctx, workspaceID, kinds, lookup)
	if err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	semantic, state, err := s.semantic(ctx, workspaceID, kinds, lookup, input.Query)
	if err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	retrieval.Semantic = state

	hits := entity.FuseKnowledge(limit, append(lanes, semantic...)...)

	if retrieval.Hits, err = s.hydrate(ctx, workspaceID, reader, hits); err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	return retrieval, nil
}

func (s *retrievals) history(
	ctx context.Context,
	workspaceID uuid.UUID,
	reader entity.KnowledgeReader,
	kinds []entity.KnowledgeSubjectKind,
	lookup repository.KnowledgeQuery,
	limit int,
) (entity.KnowledgeRetrieval, error) {
	historical := slices.DeleteFunc(slices.Clone(kinds), func(kind entity.KnowledgeSubjectKind) bool {
		return kind != entity.KnowledgeSubjectMemory
	})

	retrieval := entity.KnowledgeRetrieval{
		Scope:    scopeOf(historical, lookup, true),
		Semantic: entity.KnowledgeSemanticHistorical,
	}

	if len(historical) == 0 {
		return retrieval, nil
	}

	candidates, err := s.revisions.InEffect(ctx, workspaceID, lookup)
	if err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	hits := entity.FuseKnowledge(limit, entity.KnowledgeLane{Match: entity.KnowledgeMatchLexical, Candidates: candidates})

	if retrieval.Hits, err = s.hydrate(ctx, workspaceID, reader, hits); err != nil {
		return entity.KnowledgeRetrieval{}, err
	}

	return retrieval, nil
}

func (s *retrievals) lexical(
	ctx context.Context,
	workspaceID uuid.UUID,
	kinds []entity.KnowledgeSubjectKind,
	lookup repository.KnowledgeQuery,
) ([]entity.KnowledgeLane, error) {
	var lanes []entity.KnowledgeLane

	if slices.Contains(kinds, entity.KnowledgeSubjectMemory) {
		candidates, err := s.knowledge.LexicalMemories(ctx, workspaceID, lookup)
		if err != nil {
			return nil, err
		}

		lanes = append(lanes, entity.KnowledgeLane{Match: entity.KnowledgeMatchLexical, Candidates: candidates})
	}

	if slices.Contains(kinds, entity.KnowledgeSubjectDocPassage) {
		candidates, err := s.knowledge.LexicalPassages(ctx, workspaceID, lookup)
		if err != nil {
			return nil, err
		}

		lanes = append(lanes, entity.KnowledgeLane{Match: entity.KnowledgeMatchLexical, Candidates: candidates})
	}

	return lanes, nil
}

func (s *retrievals) semantic(
	ctx context.Context,
	workspaceID uuid.UUID,
	kinds []entity.KnowledgeSubjectKind,
	lookup repository.KnowledgeQuery,
	question string,
) ([]entity.KnowledgeLane, entity.KnowledgeSemantic, error) {
	connection, err := s.providers.Get(ctx, workspaceID)
	if errors.Is(err, entity.ErrAIProviderNotConfigured) {
		return nil, entity.KnowledgeSemanticUnconfigured, nil
	}

	if err != nil {
		return nil, "", err
	}

	if !connection.Embeds() {
		return nil, entity.KnowledgeSemanticUnconfigured, nil
	}

	vector, embedded := s.embed(ctx, connection, strings.TrimSpace(question))
	if !embedded {
		return nil, entity.KnowledgeSemanticUnavailable, nil
	}

	nearest := repository.KnowledgeVector{
		Model:         connection.EmbeddingModel,
		Vector:        vector,
		MinSimilarity: s.cfg.SemanticMinSimilarity,
	}

	var lanes []entity.KnowledgeLane

	if slices.Contains(kinds, entity.KnowledgeSubjectMemory) {
		candidates, err := s.knowledge.SemanticMemories(ctx, workspaceID, lookup, nearest)
		if err != nil {
			return nil, "", err
		}

		lanes = append(lanes, entity.KnowledgeLane{Match: entity.KnowledgeMatchSemantic, Candidates: candidates})
	}

	if slices.Contains(kinds, entity.KnowledgeSubjectDocPassage) {
		candidates, err := s.knowledge.SemanticPassages(ctx, workspaceID, lookup, nearest)
		if err != nil {
			return nil, "", err
		}

		lanes = append(lanes, entity.KnowledgeLane{Match: entity.KnowledgeMatchSemantic, Candidates: candidates})
	}

	return lanes, entity.KnowledgeSemanticUsed, nil
}

func (s *retrievals) embed(
	ctx context.Context,
	connection entity.AIProviderConnection,
	question string,
) ([]float32, bool) {
	apiKey, err := s.providers.APIKey(ctx, connection.WorkspaceID)
	if err != nil {
		return nil, false
	}

	bounded, cancel := context.WithTimeout(ctx, s.cfg.QueryTimeout)
	defer cancel()

	vectors, err := s.models.Embed(bounded, connection.Endpoint, apiKey, connection.EmbeddingModel, []string{question})
	if err != nil || len(vectors) != 1 {
		return nil, false
	}

	return vectors[0], true
}

func (s *retrievals) hydrate(
	ctx context.Context,
	workspaceID uuid.UUID,
	reader entity.KnowledgeReader,
	hits []entity.KnowledgeHit,
) ([]entity.KnowledgeHit, error) {
	var missing []uuid.UUID

	for _, hit := range hits {
		if hit.Kind == entity.KnowledgeSubjectMemory && hit.Memory == nil {
			missing = append(missing, hit.ID)
		}
	}

	loaded, err := s.memories.ListByIDs(ctx, workspaceID, missing)
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]entity.Memory, len(loaded))
	for _, memory := range loaded {
		byID[memory.ID] = memory
	}

	shown := make([]entity.KnowledgeHit, 0, len(hits))

	for _, hit := range hits {
		if hit.Kind == entity.KnowledgeSubjectMemory && hit.Memory == nil {
			memory, found := byID[hit.ID]
			if !found {
				continue
			}

			hit.Memory = &memory
		}

		if !visible(reader, hit) {
			continue
		}

		if hit.Memory != nil {
			hit.Memory.Evidence = nil
			hit.Proposed = hit.Memory.Status == entity.MemoryStatusProposed
		}

		shown = append(shown, hit)
	}

	return shown, nil
}

func visible(reader entity.KnowledgeReader, hit entity.KnowledgeHit) bool {
	switch {
	case hit.Memory != nil:
		return reader.Sees(hit.Memory.Access())
	case hit.Passage != nil:
		return reader.Sees(hit.Passage.Access())
	default:
		return false
	}
}

func (s *retrievals) Enumerate(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.EnumerateInput,
) (service.KnowledgeList, error) {
	if err := validMemoryKinds(input.MemoryKinds); err != nil {
		return service.KnowledgeList{}, err
	}

	var after *uuid.UUID

	if input.Cursor != "" {
		cursor, err := entity.DecodeKnowledgeCursor(input.Cursor)
		if err != nil {
			return service.KnowledgeList{}, err
		}

		after = &cursor.MemoryID
	}

	decision, err := s.readers.Reader(ctx, workspaceID)
	if err != nil {
		return service.KnowledgeList{}, err
	}

	reader := decision.Reader
	now := s.now().UTC()
	limit := clamp(input.Limit, entity.KnowledgeEnumerateDefaultSize, entity.KnowledgeEnumerateMaxSize)

	filter := repository.MemoryFilter{
		Kinds:            input.MemoryKinds,
		Statuses:         entity.GuidanceStatuses(input.IncludeProposed),
		AppliesProjectID: input.ProjectID,
		Component:        strings.TrimSpace(input.Component),
		WorkspaceWide:    true,
		InEffectOn:       &now,
		AllProjects:      reader.AllProjects,
		ProjectIDs:       reader.ProjectIDs,
		PersonalAuthorID: reader.PersonalAuthor(),
	}

	memories, err := s.memories.Enumerate(ctx, workspaceID, filter, after, limit+1)
	if err != nil {
		return service.KnowledgeList{}, err
	}

	list := service.KnowledgeList{
		Scope: entity.KnowledgeScope{
			Kinds:       []entity.KnowledgeSubjectKind{entity.KnowledgeSubjectMemory},
			MemoryKinds: input.MemoryKinds,
			Statuses:    filter.Statuses,
			ProjectID:   input.ProjectID,
			Component:   filter.Component,
			On:          now,
		},
	}

	if len(memories) > limit {
		memories = memories[:limit]
		list.NextCursor = entity.KnowledgeCursor{MemoryID: memories[limit-1].ID}.Encode()
	}

	list.Memories = make([]entity.Memory, 0, len(memories))

	for _, memory := range memories {
		if reader.Sees(memory.Access()) {
			memory.Evidence = nil
			list.Memories = append(list.Memories, memory)
		}
	}

	return list, nil
}

func subjectKinds(requested []entity.KnowledgeSubjectKind) ([]entity.KnowledgeSubjectKind, error) {
	if len(requested) == 0 {
		return entity.KnowledgeSubjectKinds(), nil
	}

	for _, kind := range requested {
		if !kind.Valid() {
			return nil, entity.NewValidationError(entity.FieldError{
				Field: entity.KnowledgeFieldKinds, Code: entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	return requested, nil
}

func validMemoryKinds(requested []entity.MemoryKind) error {
	for _, kind := range requested {
		if !kind.Valid() {
			return entity.NewValidationError(entity.FieldError{
				Field: entity.KnowledgeFieldMemoryKind, Code: entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	return nil
}

func scopeOf(
	kinds []entity.KnowledgeSubjectKind,
	lookup repository.KnowledgeQuery,
	historical bool,
) entity.KnowledgeScope {
	return entity.KnowledgeScope{
		Kinds:       kinds,
		MemoryKinds: lookup.MemoryKinds,
		Statuses:    lookup.Statuses,
		ProjectID:   lookup.ProjectID,
		Component:   lookup.Component,
		On:          lookup.On,
		Historical:  historical,
	}
}

func clamp(requested, fallback, ceiling int) int {
	switch {
	case requested <= 0:
		return fallback
	case requested > ceiling:
		return ceiling
	default:
		return requested
	}
}
