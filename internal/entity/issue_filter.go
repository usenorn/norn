package entity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	IssueFilterMaxDepth = 8
	IssueFilterMaxNodes = 128
	IssueSortMaxKeys    = 3
)

var (
	ErrIssueFilterTooComplex = errors.New("filter expression is too complex")
	ErrIssueFilterAmbiguous  = errors.New("filter node combines more than one kind of condition")
	ErrIssueGroupUnknown     = errors.New("issues cannot be grouped by that")
)

type IssueFilterField string

const (
	IssueFilterFieldTeam          IssueFilterField = "team"
	IssueFilterFieldState         IssueFilterField = "state"
	IssueFilterFieldStateCategory IssueFilterField = "stateCategory"
	IssueFilterFieldPriority      IssueFilterField = "priority"
	IssueFilterFieldStatus        IssueFilterField = "status"
	IssueFilterFieldAssignee      IssueFilterField = "assignee"
	IssueFilterFieldCreator       IssueFilterField = "creator"
	IssueFilterFieldLabel         IssueFilterField = "label"
	IssueFilterFieldProject       IssueFilterField = "project"
	IssueFilterFieldCycle         IssueFilterField = "cycle"
	IssueFilterFieldCreatedAt     IssueFilterField = "createdAt"
	IssueFilterFieldUpdatedAt     IssueFilterField = "updatedAt"
	IssueFilterFieldDueOn         IssueFilterField = "dueOn"
	IssueFilterFieldCompletedAt   IssueFilterField = "completedAt"
	IssueFilterFieldEstimate      IssueFilterField = "estimate"
	IssueFilterFieldBlocked       IssueFilterField = "blocked"
	IssueFilterFieldHasChildren   IssueFilterField = "hasChildren"
	IssueFilterFieldIsChild       IssueFilterField = "isChild"
)

type IssueFilterKind string

const (
	issueFilterKindID       IssueFilterKind = "id"
	issueFilterKindEnum     IssueFilterKind = "enum"
	issueFilterKindLabelSet IssueFilterKind = "labelSet"
	issueFilterKindDate     IssueFilterKind = "date"
	issueFilterKindNumber   IssueFilterKind = "number"
	issueFilterKindBool     IssueFilterKind = "bool"
)

type IssueFilterOp string

const (
	IssueFilterOpIs       IssueFilterOp = "is"
	IssueFilterOpIsNot    IssueFilterOp = "is_not"
	IssueFilterOpIn       IssueFilterOp = "in"
	IssueFilterOpNotIn    IssueFilterOp = "not_in"
	IssueFilterOpIsSet    IssueFilterOp = "is_set"
	IssueFilterOpIsNotSet IssueFilterOp = "is_not_set"
	IssueFilterOpHasAny   IssueFilterOp = "has_any"
	IssueFilterOpHasAll   IssueFilterOp = "has_all"
	IssueFilterOpHasNone  IssueFilterOp = "has_none"
	IssueFilterOpBefore   IssueFilterOp = "before"
	IssueFilterOpAfter    IssueFilterOp = "after"
	IssueFilterOpOn       IssueFilterOp = "on"
	IssueFilterOpEq       IssueFilterOp = "eq"
	IssueFilterOpLt       IssueFilterOp = "lt"
	IssueFilterOpGt       IssueFilterOp = "gt"
	IssueFilterOpIsTrue   IssueFilterOp = "is_true"
	IssueFilterOpIsFalse  IssueFilterOp = "is_false"
)

type issueFilterSpec struct {
	kind IssueFilterKind
	ops  []IssueFilterOp
}

var issueFilterFields = map[IssueFilterField]issueFilterSpec{
	IssueFilterFieldTeam:          {issueFilterKindID, idOps},
	IssueFilterFieldState:         {issueFilterKindID, idOps},
	IssueFilterFieldAssignee:      {issueFilterKindID, idOps},
	IssueFilterFieldCreator:       {issueFilterKindID, idOps},
	IssueFilterFieldProject:       {issueFilterKindID, idOps},
	IssueFilterFieldCycle:         {issueFilterKindID, idOps},
	IssueFilterFieldStateCategory: {issueFilterKindEnum, enumOps},
	IssueFilterFieldPriority:      {issueFilterKindEnum, enumOps},
	IssueFilterFieldStatus:        {issueFilterKindEnum, enumOps},
	IssueFilterFieldLabel:         {issueFilterKindLabelSet, labelOps},
	IssueFilterFieldCreatedAt:     {issueFilterKindDate, dateOps},
	IssueFilterFieldUpdatedAt:     {issueFilterKindDate, dateOps},
	IssueFilterFieldDueOn:         {issueFilterKindDate, dateOps},
	IssueFilterFieldCompletedAt:   {issueFilterKindDate, dateOps},
	IssueFilterFieldEstimate:      {issueFilterKindNumber, numberOps},
	IssueFilterFieldBlocked:       {issueFilterKindBool, boolOps},
	IssueFilterFieldHasChildren:   {issueFilterKindBool, boolOps},
	IssueFilterFieldIsChild:       {issueFilterKindBool, boolOps},
}

var (
	idOps = []IssueFilterOp{
		IssueFilterOpIs, IssueFilterOpIsNot, IssueFilterOpIn, IssueFilterOpNotIn,
		IssueFilterOpIsSet, IssueFilterOpIsNotSet,
	}
	enumOps = []IssueFilterOp{
		IssueFilterOpIs, IssueFilterOpIsNot, IssueFilterOpIn, IssueFilterOpNotIn,
	}
	labelOps  = []IssueFilterOp{IssueFilterOpHasAny, IssueFilterOpHasAll, IssueFilterOpHasNone}
	dateOps   = []IssueFilterOp{IssueFilterOpBefore, IssueFilterOpAfter, IssueFilterOpOn, IssueFilterOpIsSet, IssueFilterOpIsNotSet}
	numberOps = []IssueFilterOp{IssueFilterOpEq, IssueFilterOpLt, IssueFilterOpGt, IssueFilterOpIsSet, IssueFilterOpIsNotSet}
	boolOps   = []IssueFilterOp{IssueFilterOpIsTrue, IssueFilterOpIsFalse}
)

func IssueFilterFields() []IssueFilterField {
	fields := make([]IssueFilterField, 0, len(issueFilterFields))

	for field := range issueFilterFields {
		fields = append(fields, field)
	}

	slices.Sort(fields)

	return fields
}

func (f IssueFilterField) Kind() (IssueFilterKind, bool) {
	spec, known := issueFilterFields[f]

	return spec.kind, known
}

type IssueFilter struct {
	All    []IssueFilter    `json:"all,omitempty"`
	Any    []IssueFilter    `json:"any,omitempty"`
	Not    *IssueFilter     `json:"not,omitempty"`
	Field  IssueFilterField `json:"field,omitempty"`
	Op     IssueFilterOp    `json:"op,omitempty"`
	Values []string         `json:"values,omitempty"`
}

func (f IssueFilter) Leaf() bool {
	return f.Field != ""
}

func (f IssueFilter) Empty() bool {
	return !f.Leaf() && len(f.All) == 0 && len(f.Any) == 0 && f.Not == nil
}

func (f IssueFilter) Validate() error {
	nodes := 0

	if err := f.validate(1, &nodes); err != nil {
		return err
	}

	return nil
}

func (f IssueFilter) validate(depth int, nodes *int) error {
	if depth > IssueFilterMaxDepth {
		return ErrIssueFilterTooComplex
	}

	*nodes++

	if *nodes > IssueFilterMaxNodes {
		return ErrIssueFilterTooComplex
	}

	forms := 0

	if len(f.All) > 0 {
		forms++
	}

	if len(f.Any) > 0 {
		forms++
	}

	if f.Not != nil {
		forms++
	}

	if f.Leaf() {
		forms++
	}

	switch {
	case forms == 0:
		return nil
	case forms > 1:
		return ErrIssueFilterAmbiguous
	}

	if f.Not != nil {
		return f.Not.validate(depth+1, nodes)
	}

	for _, branch := range append(append([]IssueFilter{}, f.All...), f.Any...) {
		if err := branch.validate(depth+1, nodes); err != nil {
			return err
		}
	}

	if !f.Leaf() {
		return nil
	}

	return f.validateLeaf()
}

func (f IssueFilter) validateLeaf() error {
	spec, known := issueFilterFields[f.Field]
	if !known {
		return NewValidationError(FieldError{Field: "filter.field", Code: ValidationCodeUnsupportedValue})
	}

	if !slices.Contains(spec.ops, f.Op) {
		return NewValidationError(FieldError{Field: "filter.op", Code: ValidationCodeUnsupportedValue})
	}

	switch f.Op {
	case IssueFilterOpIsSet, IssueFilterOpIsNotSet, IssueFilterOpIsTrue, IssueFilterOpIsFalse:
		if len(f.Values) > 0 {
			return NewValidationError(FieldError{Field: "filter.values", Code: ValidationCodeUnsupportedValue})
		}

		return nil
	}

	if len(f.Values) == 0 {
		return NewValidationError(FieldError{Field: "filter.values", Code: ValidationCodeRequired})
	}

	switch f.Op {
	case IssueFilterOpIs, IssueFilterOpIsNot, IssueFilterOpBefore, IssueFilterOpAfter,
		IssueFilterOpOn, IssueFilterOpEq, IssueFilterOpLt, IssueFilterOpGt:
		if len(f.Values) != 1 {
			return NewValidationError(FieldError{Field: "filter.values", Code: ValidationCodeUnsupportedValue})
		}
	}

	for _, value := range f.Values {
		if err := spec.kind.parse(f.Field, value); err != nil {
			return err
		}
	}

	return nil
}

func (k IssueFilterKind) parse(field IssueFilterField, value string) error {
	malformed := NewValidationError(FieldError{Field: "filter.values", Code: ValidationCodeMalformed})

	switch k {
	case issueFilterKindID, issueFilterKindLabelSet:
		if _, err := uuid.Parse(value); err != nil {
			return malformed
		}
	case issueFilterKindDate:
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return malformed
		}
	case issueFilterKindNumber:
		if _, err := strconv.Atoi(value); err != nil {
			return malformed
		}
	case issueFilterKindEnum:
		if !knownEnumValue(field, value) {
			return malformed
		}
	}

	return nil
}

func knownEnumValue(field IssueFilterField, value string) bool {
	switch field {
	case IssueFilterFieldStateCategory:
		return StateCategory(value).Valid()
	case IssueFilterFieldPriority:
		return IssuePriority(value).Valid()
	case IssueFilterFieldStatus:
		return IssueStatus(value).Valid()
	default:
		return false
	}
}

type IssueSortField string

const (
	IssueSortFieldCreatedAt IssueSortField = "createdAt"
	IssueSortFieldUpdatedAt IssueSortField = "updatedAt"
	IssueSortFieldPriority  IssueSortField = "priority"
	IssueSortFieldDueOn     IssueSortField = "dueOn"
	IssueSortFieldState     IssueSortField = "state"
	IssueSortFieldEstimate  IssueSortField = "estimate"
)

func IssueSortFields() []IssueSortField {
	return []IssueSortField{
		IssueSortFieldCreatedAt,
		IssueSortFieldUpdatedAt,
		IssueSortFieldPriority,
		IssueSortFieldDueOn,
		IssueSortFieldState,
		IssueSortFieldEstimate,
	}
}

func (f IssueSortField) Valid() bool {
	return slices.Contains(IssueSortFields(), f)
}

type IssueSort struct {
	Field      IssueSortField `json:"field"`
	Descending bool           `json:"descending,omitempty"`
}

func DefaultIssueSort() []IssueSort {
	return []IssueSort{{Field: IssueSortFieldCreatedAt, Descending: true}}
}

func NormalizedIssueSort(sort []IssueSort) ([]IssueSort, error) {
	if len(sort) == 0 {
		return DefaultIssueSort(), nil
	}

	if len(sort) > IssueSortMaxKeys {
		return nil, ErrIssueFilterTooComplex
	}

	seen := make(map[IssueSortField]bool, len(sort))

	for _, key := range sort {
		if !key.Field.Valid() {
			return nil, NewValidationError(FieldError{Field: "sort", Code: ValidationCodeUnsupportedValue})
		}

		if seen[key.Field] {
			return nil, NewValidationError(FieldError{Field: "sort", Code: ValidationCodeUnsupportedValue})
		}

		seen[key.Field] = true
	}

	return slices.Clone(sort), nil
}

type IssueGroupBy string

const (
	IssueGroupByState         IssueGroupBy = "state"
	IssueGroupByStateCategory IssueGroupBy = "stateCategory"
	IssueGroupByPriority      IssueGroupBy = "priority"
	IssueGroupByAssignee      IssueGroupBy = "assignee"
	IssueGroupByTeam          IssueGroupBy = "team"
	IssueGroupByProject       IssueGroupBy = "project"
	IssueGroupByCycle         IssueGroupBy = "cycle"
	IssueGroupByLabel         IssueGroupBy = "label"
)

func IssueGroupBys() []IssueGroupBy {
	return []IssueGroupBy{
		IssueGroupByState,
		IssueGroupByStateCategory,
		IssueGroupByPriority,
		IssueGroupByAssignee,
		IssueGroupByTeam,
		IssueGroupByProject,
		IssueGroupByCycle,
		IssueGroupByLabel,
	}
}

func (g IssueGroupBy) Valid() bool {
	return slices.Contains(IssueGroupBys(), g)
}

func (g IssueGroupBy) Overlaps() bool {
	return g == IssueGroupByLabel
}

type IssueGroupTally struct {
	Key        string
	Issues     int
	NextCursor string
}

type IssueGroupSlice struct {
	Key    string
	Issues []Issue
}

type IssueQueryCursor struct {
	Keys    []string  `json:"k"`
	IssueID uuid.UUID `json:"id"`
}

func (c IssueQueryCursor) Encode() string {
	encoded, err := json.Marshal(c)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(encoded)
}

func DecodeIssueQueryCursor(raw string) (IssueQueryCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return IssueQueryCursor{}, ErrIssueCursorInvalid
	}

	var cursor IssueQueryCursor

	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return IssueQueryCursor{}, ErrIssueCursorInvalid
	}

	if cursor.IssueID == uuid.Nil {
		return IssueQueryCursor{}, ErrIssueCursorInvalid
	}

	return cursor, nil
}

func IssueSortKeys(issue Issue, sort []IssueSort) []string {
	keys := make([]string, 0, len(sort))

	for _, key := range sort {
		keys = append(keys, issueSortKey(issue, key.Field))
	}

	return keys
}

func issueSortKey(issue Issue, field IssueSortField) string {
	switch field {
	case IssueSortFieldCreatedAt:
		return issue.CreatedAt.UTC().Format(time.RFC3339Nano)
	case IssueSortFieldUpdatedAt:
		return issue.UpdatedAt.UTC().Format(time.RFC3339Nano)
	case IssueSortFieldPriority:
		return strconv.Itoa(slices.Index(IssuePriorities(), issue.Priority) + 1)
	case IssueSortFieldDueOn:
		return issue.DueOn
	case IssueSortFieldState:
		return strconv.Itoa(issue.State.Position)
	case IssueSortFieldEstimate:
		if issue.Estimate == 0 {
			return ""
		}

		return strconv.Itoa(issue.Estimate)
	default:
		return ""
	}
}
