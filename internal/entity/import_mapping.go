package entity

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ImportMappingKind string

const (
	ImportMapUser     ImportMappingKind = "user"
	ImportMapState    ImportMappingKind = "state"
	ImportMapPriority ImportMappingKind = "priority"
	ImportMapLabel    ImportMappingKind = "label"
	ImportMapProject  ImportMappingKind = "project"
	ImportMapTeam     ImportMappingKind = "team"
)

func ImportMappingKinds() []ImportMappingKind {
	return []ImportMappingKind{
		ImportMapUser,
		ImportMapState,
		ImportMapPriority,
		ImportMapLabel,
		ImportMapProject,
		ImportMapTeam,
	}
}

func (k ImportMappingKind) Valid() bool {
	return slices.Contains(ImportMappingKinds(), k)
}

func (k ImportMappingKind) TargetsAnAccount() bool {
	return k == ImportMapUser
}

type ImportDecision string

const (
	ImportDecisionMap          ImportDecision = "map"
	ImportDecisionCreate       ImportDecision = "create"
	ImportDecisionUnattributed ImportDecision = "unattributed"
	ImportDecisionSkip         ImportDecision = "skip"
)

func ImportDecisions() []ImportDecision {
	return []ImportDecision{
		ImportDecisionMap,
		ImportDecisionCreate,
		ImportDecisionUnattributed,
		ImportDecisionSkip,
	}
}

func (d ImportDecision) Valid() bool {
	return slices.Contains(ImportDecisions(), d)
}

func (d ImportDecision) Decided() bool {
	return d.Valid()
}

func (d ImportDecision) NeedsTarget() bool {
	return d == ImportDecisionMap
}

type ImportMapping struct {
	RunID             uuid.UUID
	Kind              ImportMappingKind
	SourceKey         string
	SourceLabel       string
	SourceEmail       string
	Decision          ImportDecision
	TargetID          uuid.UUID
	TargetValue       string
	SuggestedTargetID uuid.UUID
	SuggestedReason   string
	DecidedByAccount  uuid.UUID
	DecidedAt         *time.Time
}

func (m ImportMapping) Validate() error {
	switch {
	case !m.Kind.Valid():
		return ErrImportMappingUnknownKind
	case !m.Decision.Valid():
		return NewValidationError(FieldError{Field: "decision", Code: ValidationCodeUnsupportedValue})
	case m.Decision.NeedsTarget() && m.TargetID == uuid.Nil && strings.TrimSpace(m.TargetValue) == "":
		return NewValidationError(FieldError{Field: "target", Code: ValidationCodeRequired})
	case m.Decision == ImportDecisionUnattributed && m.Kind != ImportMapUser:
		return NewValidationError(FieldError{Field: "decision", Code: ValidationCodeUnsupportedValue})
	default:
		return nil
	}
}

func (m ImportMapping) Undecided() bool {
	return !m.Decision.Valid()
}

type MappingPlan struct {
	Mappings []ImportMapping
}

func (p MappingPlan) Complete() bool {
	return len(p.Undecided()) == 0
}

func (p MappingPlan) Undecided() []ImportMapping {
	pending := make([]ImportMapping, 0)

	for _, mapping := range p.Mappings {
		if mapping.Undecided() {
			pending = append(pending, mapping)
		}
	}

	return pending
}

func (p MappingPlan) Unattributed() []ImportMapping {
	unattributed := make([]ImportMapping, 0)

	for _, mapping := range p.Mappings {
		if mapping.Kind == ImportMapUser && mapping.Decision == ImportDecisionUnattributed {
			unattributed = append(unattributed, mapping)
		}
	}

	return unattributed
}

func (p MappingPlan) Lookup(kind ImportMappingKind, sourceKey string) (ImportMapping, bool) {
	for _, mapping := range p.Mappings {
		if mapping.Kind == kind && mapping.SourceKey == sourceKey {
			return mapping, true
		}
	}

	return ImportMapping{}, false
}

func (p MappingPlan) Target(kind ImportMappingKind, sourceKey string) (uuid.UUID, bool) {
	mapping, found := p.Lookup(kind, sourceKey)
	if !found || mapping.Decision != ImportDecisionMap {
		return uuid.Nil, false
	}

	return mapping.TargetID, mapping.TargetID != uuid.Nil
}

func (p MappingPlan) Skips(kind ImportMappingKind, sourceKey string) bool {
	mapping, found := p.Lookup(kind, sourceKey)

	return found && mapping.Decision == ImportDecisionSkip
}

const (
	SuggestedByEmail = "the source address matches a member of this workspace"
	SuggestedByName  = "the source name matches a member of this workspace"
)

type ImportCandidate struct {
	AccountID   uuid.UUID
	Email       string
	DisplayName string
}

func SuggestAccount(mapping ImportMapping, candidates []ImportCandidate) ImportMapping {
	email := strings.ToLower(strings.TrimSpace(mapping.SourceEmail))
	name := strings.ToLower(strings.TrimSpace(mapping.SourceLabel))

	for _, candidate := range candidates {
		if email != "" && strings.EqualFold(strings.TrimSpace(candidate.Email), email) {
			mapping.SuggestedTargetID = candidate.AccountID
			mapping.SuggestedReason = SuggestedByEmail

			return mapping
		}
	}

	for _, candidate := range candidates {
		if name != "" && strings.EqualFold(strings.TrimSpace(candidate.DisplayName), name) {
			mapping.SuggestedTargetID = candidate.AccountID
			mapping.SuggestedReason = SuggestedByName

			return mapping
		}
	}

	return mapping
}

type labelSwatch struct {
	color LabelColor
	r     int
	g     int
	b     int
}

func labelSwatches() []labelSwatch {
	return []labelSwatch{
		{LabelColorNeutral, 0x8a, 0x93, 0xa6},
		{LabelColorCyan, 0x58, 0xb4, 0xc4},
		{LabelColorBlue, 0x6b, 0x93, 0xd6},
		{LabelColorViolet, 0x8e, 0x86, 0xd9},
		{LabelColorOrchid, 0xb0, 0x7a, 0xd0},
		{LabelColorMagenta, 0xcb, 0x77, 0xb4},
	}
}

// NearestLabelColor maps an arbitrary source colour onto the six Norn offers. Anything
// unreadable becomes neutral rather than an error: a label is worth importing even when
// its colour is not.
func NearestLabelColor(value string) LabelColor {
	red, green, blue, ok := parseHex(value)
	if !ok {
		return LabelColorNeutral
	}

	nearest := LabelColorNeutral
	best := -1

	for _, swatch := range labelSwatches() {
		distance := square(red-swatch.r) + square(green-swatch.g) + square(blue-swatch.b)
		if best < 0 || distance < best {
			best = distance
			nearest = swatch.color
		}
	}

	return nearest
}

func square(value int) int {
	return value * value
}

func parseHex(value string) (int, int, int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")

	if len(trimmed) == 3 {
		expanded := make([]byte, 0, 6)
		for index := range len(trimmed) {
			expanded = append(expanded, trimmed[index], trimmed[index])
		}

		trimmed = string(expanded)
	}

	if len(trimmed) != 6 {
		return 0, 0, 0, false
	}

	channels := make([]int, 0, 3)

	for index := 0; index < 6; index += 2 {
		channel, err := strconv.ParseUint(trimmed[index:index+2], 16, 8)
		if err != nil {
			return 0, 0, 0, false
		}

		channels = append(channels, int(channel))
	}

	return channels[0], channels[1], channels[2], true
}
