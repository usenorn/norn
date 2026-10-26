package entity

import (
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	KnowledgeRetrieveDefaultSize  = 10
	KnowledgeRetrieveMaxSize      = 50
	KnowledgeBriefingSize         = 5
	KnowledgeEnumerateDefaultSize = 50
	KnowledgeEnumerateMaxSize     = 200
	KnowledgeLaneSize             = 50
	KnowledgeFusionConstant       = 60
	KnowledgeExcerptRunes         = 280

	KnowledgeFieldQuery      = "query"
	KnowledgeFieldKinds      = "kinds"
	KnowledgeFieldMemoryKind = "memoryKinds"
)

var ErrKnowledgeCursorInvalid = errors.New("knowledge cursor is invalid")

type KnowledgeSubjectKind string

const (
	KnowledgeSubjectMemory     KnowledgeSubjectKind = "memory"
	KnowledgeSubjectDocPassage KnowledgeSubjectKind = "doc_passage"
)

func KnowledgeSubjectKinds() []KnowledgeSubjectKind {
	return []KnowledgeSubjectKind{KnowledgeSubjectMemory, KnowledgeSubjectDocPassage}
}

func (k KnowledgeSubjectKind) Valid() bool {
	return slices.Contains(KnowledgeSubjectKinds(), k)
}

type KnowledgeMatch string

const (
	KnowledgeMatchExact    KnowledgeMatch = "exact"
	KnowledgeMatchLexical  KnowledgeMatch = "lexical"
	KnowledgeMatchSemantic KnowledgeMatch = "semantic"
)

type KnowledgeSemantic string

const (
	KnowledgeSemanticUsed         KnowledgeSemantic = "used"
	KnowledgeSemanticUnconfigured KnowledgeSemantic = "unconfigured"
	KnowledgeSemanticUnavailable  KnowledgeSemantic = "unavailable"
	KnowledgeSemanticHistorical   KnowledgeSemantic = "historical"
)

func GuidanceStatuses(includeProposed bool) []MemoryStatus {
	if includeProposed {
		return []MemoryStatus{MemoryStatusAccepted, MemoryStatusProposed}
	}

	return []MemoryStatus{MemoryStatusAccepted}
}

type KnowledgePassage struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	DocID       uuid.UUID
	DocTitle    string
	Heading     string
	Excerpt     string
	Position    int
	Audience    KnowledgeAudience
	AuthorID    uuid.UUID
	UpdatedAt   time.Time
}

func (p KnowledgePassage) Access() KnowledgeAccess {
	return DocSummary{WorkspaceID: p.WorkspaceID, AuthorID: p.AuthorID, Audience: p.Audience}.Access()
}

type KnowledgeCandidate struct {
	Kind    KnowledgeSubjectKind
	ID      uuid.UUID
	Exact   bool
	Memory  *Memory
	Passage *KnowledgePassage
}

type KnowledgeLane struct {
	Match      KnowledgeMatch
	Candidates []KnowledgeCandidate
}

type KnowledgeHit struct {
	Kind     KnowledgeSubjectKind
	ID       uuid.UUID
	Memory   *Memory
	Passage  *KnowledgePassage
	Matches  []KnowledgeMatch
	Score    float64
	Proposed bool
}

func (h KnowledgeHit) Exact() bool {
	return slices.Contains(h.Matches, KnowledgeMatchExact)
}

func (h KnowledgeHit) order() string {
	if h.Passage != nil {
		return fmt.Sprintf("%s/%s/%08d", h.Kind, h.Passage.DocID, h.Passage.Position)
	}

	return fmt.Sprintf("%s/%s", h.Kind, h.ID)
}

type KnowledgeScope struct {
	Kinds       []KnowledgeSubjectKind
	MemoryKinds []MemoryKind
	Statuses    []MemoryStatus
	ProjectID   *uuid.UUID
	Component   string
	On          time.Time
	Historical  bool
}

type KnowledgeRetrieval struct {
	Hits     []KnowledgeHit
	Scope    KnowledgeScope
	Semantic KnowledgeSemantic
}

func (r KnowledgeRetrieval) Empty() bool {
	return len(r.Hits) == 0
}

func FuseKnowledge(limit int, lanes ...KnowledgeLane) []KnowledgeHit {
	type key struct {
		kind KnowledgeSubjectKind
		id   uuid.UUID
	}

	var (
		order []key
		hits  = map[key]*KnowledgeHit{}
	)

	for _, lane := range lanes {
		for rank, candidate := range lane.Candidates {
			at := key{kind: candidate.Kind, id: candidate.ID}

			hit, seen := hits[at]
			if !seen {
				hit = &KnowledgeHit{Kind: candidate.Kind, ID: candidate.ID}
				hits[at] = hit
				order = append(order, at)
			}

			if hit.Memory == nil {
				hit.Memory = candidate.Memory
			}

			if hit.Passage == nil {
				hit.Passage = candidate.Passage
			}

			hit.Score += 1 / float64(KnowledgeFusionConstant+rank+1)

			if !slices.Contains(hit.Matches, lane.Match) {
				hit.Matches = append(hit.Matches, lane.Match)
			}

			if candidate.Exact && !hit.Exact() {
				hit.Matches = append([]KnowledgeMatch{KnowledgeMatchExact}, hit.Matches...)
			}
		}
	}

	fused := make([]KnowledgeHit, 0, len(order))
	for _, at := range order {
		fused = append(fused, *hits[at])
	}

	slices.SortStableFunc(fused, func(a, b KnowledgeHit) int {
		if a.Exact() != b.Exact() {
			if a.Exact() {
				return -1
			}

			return 1
		}

		if a.Score != b.Score {
			return cmp.Compare(b.Score, a.Score)
		}

		return cmp.Compare(a.order(), b.order())
	})

	if len(fused) > limit {
		fused = fused[:limit]
	}

	return fused
}

type KnowledgeCursor struct {
	MemoryID uuid.UUID
}

func (c KnowledgeCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(c.MemoryID[:])
}

func DecodeKnowledgeCursor(raw string) (KnowledgeCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return KnowledgeCursor{}, ErrKnowledgeCursorInvalid
	}

	memoryID, err := uuid.FromBytes(decoded)
	if err != nil {
		return KnowledgeCursor{}, ErrKnowledgeCursorInvalid
	}

	return KnowledgeCursor{MemoryID: memoryID}, nil
}
