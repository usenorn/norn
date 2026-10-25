package entity

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MemoryRelatedMax        = 5
	MemoryRelatedLaneSize   = 20
	MemoryRelatedOverlapMin = 0.5

	memoryWordMinRunes = 3

	MemoryFieldResolution = "resolution"
	MemoryFieldTarget     = "target"
)

var ErrMemoryNotPending = errors.New("memory is not awaiting a decision")

var memoryFillerWords = []string{
	"and", "are", "for", "from", "has", "have", "into", "its", "that", "the", "this", "was", "were", "with",
}

type MemoryRelation string

const (
	MemoryRelationDuplicate   MemoryRelation = "duplicate"
	MemoryRelationConditional MemoryRelation = "conditional"
	MemoryRelationConflict    MemoryRelation = "conflict"
)

func MemoryRelations() []MemoryRelation {
	return []MemoryRelation{MemoryRelationDuplicate, MemoryRelationConditional, MemoryRelationConflict}
}

func (r MemoryRelation) Valid() bool {
	return slices.Contains(MemoryRelations(), r)
}

type MemoryResolution string

const (
	MemoryResolutionIndependent MemoryResolution = "independent"
	MemoryResolutionUpdate      MemoryResolution = "update"
	MemoryResolutionSupersede   MemoryResolution = "supersede"
	MemoryResolutionNoChange    MemoryResolution = "no_change"
)

func MemoryResolutions() []MemoryResolution {
	return []MemoryResolution{
		MemoryResolutionIndependent,
		MemoryResolutionUpdate,
		MemoryResolutionSupersede,
		MemoryResolutionNoChange,
	}
}

func (r MemoryResolution) Valid() bool {
	return slices.Contains(MemoryResolutions(), r)
}

func (r MemoryResolution) Targeted() bool {
	return r == MemoryResolutionUpdate || r == MemoryResolutionSupersede
}

type RelatedMemory struct {
	Memory   Memory
	Relation MemoryRelation
}

type MemoryChoice struct {
	Resolution MemoryResolution
	TargetID   *uuid.UUID
}

func MemoryRelatableStatuses() []MemoryStatus {
	return []MemoryStatus{MemoryStatusProposed, MemoryStatusAccepted, MemoryStatusDisputed}
}

func (m Memory) Pending() bool {
	return slices.Contains(MemoryPendingStatuses(), m.Status)
}

func (m Memory) RelationTo(other Memory, semantic bool) (MemoryRelation, bool) {
	if !m.sharesScopeWith(other) {
		return "", false
	}

	statement, otherStatement := memoryWords(m.Statement), memoryWords(other.Statement)

	if !semantic && memoryOverlap(statement, otherStatement) < MemoryRelatedOverlapMin {
		return "", false
	}

	switch {
	case !slices.Equal(memoryWords(m.Conditions), memoryWords(other.Conditions)):
		return MemoryRelationConditional, true
	case slices.Equal(statement, otherStatement):
		return MemoryRelationDuplicate, true
	case m.Kind == MemoryKindDecision && other.Kind == MemoryKindDecision && other.Status == MemoryStatusAccepted:
		return MemoryRelationConflict, true
	default:
		return MemoryRelationDuplicate, true
	}
}

func (m Memory) sharesScopeWith(other Memory) bool {
	ours, theirs := m.Applicability, other.Applicability

	if ours.ProjectID != nil && theirs.ProjectID != nil && *ours.ProjectID != *theirs.ProjectID {
		return false
	}

	return ours.Component == "" || theirs.Component == "" || strings.EqualFold(ours.Component, theirs.Component)
}

func (m Memory) StatusAmong(related []RelatedMemory, settled bool) MemoryStatus {
	switch {
	case slices.ContainsFunc(related, func(r RelatedMemory) bool { return r.Relation == MemoryRelationConflict }):
		return MemoryStatusDisputed
	case len(related) == 0 && settled:
		return MemoryStatusAccepted
	default:
		return MemoryStatusProposed
	}
}

func MemoryChoices(related []RelatedMemory) []MemoryChoice {
	if len(related) == 0 {
		return nil
	}

	choices := []MemoryChoice{{Resolution: MemoryResolutionIndependent}}

	for _, candidate := range related {
		target := candidate.Memory
		if !target.Viewer.Edits || target.Status.Terminal() {
			continue
		}

		choices = append(choices, MemoryChoice{Resolution: MemoryResolutionUpdate, TargetID: &target.ID})

		if target.Status.CanBecome(MemoryStatusSuperseded) {
			choices = append(choices, MemoryChoice{Resolution: MemoryResolutionSupersede, TargetID: &target.ID})
		}
	}

	return append(choices, MemoryChoice{Resolution: MemoryResolutionNoChange})
}

func memoryWords(text string) []string {
	var words []string

	for _, field := range strings.Fields(strings.ToLower(text)) {
		word := strings.TrimFunc(field, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })
		if word != "" {
			words = append(words, word)
		}
	}

	return words
}

func memoryOverlap(a, b []string) float64 {
	ours, theirs := memoryTerms(a), memoryTerms(b)
	if len(ours) == 0 || len(theirs) == 0 {
		return 0
	}

	shared := 0
	for _, term := range ours {
		if slices.Contains(theirs, term) {
			shared++
		}
	}

	return float64(shared) / float64(min(len(ours), len(theirs)))
}

func memoryTerms(words []string) []string {
	var terms []string

	for _, word := range words {
		if utf8.RuneCountInString(word) < memoryWordMinRunes || slices.Contains(memoryFillerWords, word) {
			continue
		}

		if !slices.Contains(terms, word) {
			terms = append(terms, word)
		}
	}

	return terms
}
