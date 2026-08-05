package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/usenorn/norn/internal/entity"
)

var filterColumns = map[entity.IssueFilterField]string{
	entity.IssueFilterFieldTeam:          "i.team_id",
	entity.IssueFilterFieldState:         "i.state_id",
	entity.IssueFilterFieldStateCategory: "s.category",
	entity.IssueFilterFieldPriority:      "i.priority",
	entity.IssueFilterFieldStatus:        "i.status",
	entity.IssueFilterFieldAssignee:      "i.assignee_account_id",
	entity.IssueFilterFieldCreator:       "i.created_by_account_id",
	entity.IssueFilterFieldProject:       "i.project_id",
	entity.IssueFilterFieldCycle:         "i.cycle_id",
	entity.IssueFilterFieldCreatedAt:     "i.created_at",
	entity.IssueFilterFieldUpdatedAt:     "i.updated_at",
	entity.IssueFilterFieldDueOn:         "i.due_on",
	entity.IssueFilterFieldCompletedAt:   "i.completed_at",
	entity.IssueFilterFieldEstimate:      "i.estimate",
}

var sortColumns = map[entity.IssueSortField]string{
	entity.IssueSortFieldCreatedAt: "i.created_at",
	entity.IssueSortFieldUpdatedAt: "i.updated_at",
	entity.IssueSortFieldPriority:  priorityRank,
	entity.IssueSortFieldDueOn:     "i.due_on",
	entity.IssueSortFieldState:     "s.position",
	entity.IssueSortFieldEstimate:  "i.estimate",
}

var nullableSorts = map[entity.IssueSortField]bool{
	entity.IssueSortFieldDueOn:    true,
	entity.IssueSortFieldEstimate: true,
}

const priorityRank = `array_position(ARRAY['urgent','high','medium','low','none'], i.priority)`

var groupKeys = map[entity.IssueGroupBy]string{
	entity.IssueGroupByState:         "i.state_id::text",
	entity.IssueGroupByStateCategory: "s.category",
	entity.IssueGroupByPriority:      "i.priority",
	entity.IssueGroupByAssignee:      "coalesce(i.assignee_account_id::text, '')",
	entity.IssueGroupByTeam:          "i.team_id::text",
	entity.IssueGroupByProject:       "coalesce(i.project_id::text, '')",
	entity.IssueGroupByCycle:         "coalesce(i.cycle_id::text, '')",
	entity.IssueGroupByLabel:         "coalesce(il.label_id::text, '')",
}

const groupLabelJoin = `
LEFT JOIN workspace_issue_labels il ON il.issue_id = i.id`

type builder struct {
	args []any
}

func (b *builder) bind(value any) string {
	b.args = append(b.args, value)

	return fmt.Sprintf("$%d", len(b.args))
}

func (b *builder) scope(scope entity.TeamScope, waiting bool) string {
	triage := decided
	if waiting {
		triage = stillWaiting
	}

	return fmt.Sprintf(
		"i.workspace_id = %s AND (%s::boolean IS TRUE OR i.team_id = ANY(%s::uuid[])) AND %s",
		b.bind(scope.WorkspaceID.String()),
		b.bind(scope.AllTeams),
		b.bind(teamIDs(scope)),
		triage,
	)
}

const (
	decided      = `i.triage_state IS DISTINCT FROM 'waiting'`
	stillWaiting = `i.triage_state = 'waiting'`
)

func (b *builder) where(scope entity.TeamScope, page entity.IssuePage, filter *entity.IssueFilter) string {
	clause := b.scope(scope, page.Waiting) + b.text(page.Text)

	if filter == nil || filter.Empty() {
		return clause
	}

	return clause + " AND " + b.node(*filter)
}

const matchesText = `
  AND ((%s <> '' AND i.search_document @@ websearch_to_tsquery('english', %[1]s))
       OR (%s <> '' AND i.search_document @@ to_tsquery('simple', %[3]s || ':*')))`

func (b *builder) text(query entity.SearchQuery) string {
	if query.Empty() {
		return ""
	}

	stemmed := b.bind(query.Stemmed)
	prefix := b.bind(query.Prefix)

	return fmt.Sprintf(matchesText, stemmed, stemmed, prefix, prefix)
}

func (b *builder) node(filter entity.IssueFilter) string {
	switch {
	case len(filter.All) > 0:
		return b.join(filter.All, " AND ")
	case len(filter.Any) > 0:
		return b.join(filter.Any, " OR ")
	case filter.Not != nil:
		return "NOT " + b.node(*filter.Not)
	case filter.Leaf():
		return b.leaf(filter)
	default:
		return "TRUE"
	}
}

func (b *builder) join(branches []entity.IssueFilter, operator string) string {
	parts := make([]string, 0, len(branches))

	for _, branch := range branches {
		parts = append(parts, b.node(branch))
	}

	return "(" + strings.Join(parts, operator) + ")"
}

func (b *builder) leaf(filter entity.IssueFilter) string {
	switch filter.Field {
	case entity.IssueFilterFieldLabel:
		return b.labels(filter)
	case entity.IssueFilterFieldBlocked:
		return b.truth(blockedExists, filter.Op)
	case entity.IssueFilterFieldHasChildren:
		return b.truth(childrenExists, filter.Op)
	case entity.IssueFilterFieldIsChild:
		return b.truth("i.parent_issue_id IS NOT NULL", filter.Op)
	}

	column, known := filterColumns[filter.Field]
	if !known {
		return "FALSE"
	}

	return b.comparison(column, filter)
}

const blockedExists = `EXISTS (
    SELECT 1 FROM workspace_issue_relations r
    JOIN workspace_issues b ON b.id = r.source_issue_id
    JOIN workspace_workflow_states bs ON bs.id = b.state_id AND bs.team_id = b.team_id
    WHERE r.target_issue_id = i.id AND r.kind = 'blocks'
      AND b.status = 'active' AND bs.category IN ('not_started', 'active')
)`

const childrenExists = `EXISTS (
    SELECT 1 FROM workspace_issues ch WHERE ch.parent_issue_id = i.id AND ch.status = 'active'
)`

func (b *builder) truth(predicate string, op entity.IssueFilterOp) string {
	if op == entity.IssueFilterOpIsFalse {
		return "NOT (" + predicate + ")"
	}

	return "(" + predicate + ")"
}

func (b *builder) labels(filter entity.IssueFilter) string {
	exists := func(value string) string {
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM workspace_issue_labels il WHERE il.issue_id = i.id AND il.label_id = %s::uuid)",
			b.bind(value),
		)
	}

	switch filter.Op {
	case entity.IssueFilterOpHasAll:
		parts := make([]string, 0, len(filter.Values))

		for _, value := range filter.Values {
			parts = append(parts, exists(value))
		}

		return "(" + strings.Join(parts, " AND ") + ")"

	case entity.IssueFilterOpHasNone:
		return fmt.Sprintf(
			"NOT EXISTS (SELECT 1 FROM workspace_issue_labels il WHERE il.issue_id = i.id AND il.label_id = ANY(%s::uuid[]))",
			b.bind(filter.Values),
		)

	default:
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM workspace_issue_labels il WHERE il.issue_id = i.id AND il.label_id = ANY(%s::uuid[]))",
			b.bind(filter.Values),
		)
	}
}

func (b *builder) comparison(column string, filter entity.IssueFilter) string {
	cast := ""
	if kind, _ := filter.Field.Kind(); kind == "id" {
		cast = "::uuid"
	}

	switch filter.Op {
	case entity.IssueFilterOpIsSet:
		return column + " IS NOT NULL"

	case entity.IssueFilterOpIsNotSet:
		return column + " IS NULL"

	case entity.IssueFilterOpIs:
		return fmt.Sprintf("%s = %s%s", column, b.bind(filter.Values[0]), cast)

	case entity.IssueFilterOpIsNot:
		return fmt.Sprintf("(%s IS NULL OR %s <> %s%s)", column, column, b.bind(filter.Values[0]), cast)

	case entity.IssueFilterOpIn:
		return fmt.Sprintf("%s = ANY(%s%s)", column, b.bind(filter.Values), arrayCast(cast))

	case entity.IssueFilterOpNotIn:
		return fmt.Sprintf(
			"(%s IS NULL OR NOT (%s = ANY(%s%s)))",
			column, column, b.bind(filter.Values), arrayCast(cast),
		)

	case entity.IssueFilterOpBefore:
		return fmt.Sprintf("%s < %s::date", column, b.bind(filter.Values[0]))

	case entity.IssueFilterOpAfter:
		return fmt.Sprintf("%s > %s::date", column, b.bind(filter.Values[0]))

	case entity.IssueFilterOpOn:
		return fmt.Sprintf("%s::date = %s::date", column, b.bind(filter.Values[0]))

	case entity.IssueFilterOpEq:
		return fmt.Sprintf("%s = %s::integer", column, b.bind(filter.Values[0]))

	case entity.IssueFilterOpLt:
		return fmt.Sprintf("%s < %s::integer", column, b.bind(filter.Values[0]))

	case entity.IssueFilterOpGt:
		return fmt.Sprintf("%s > %s::integer", column, b.bind(filter.Values[0]))

	default:
		return "FALSE"
	}
}

func arrayCast(cast string) string {
	if cast == "" {
		return "::text[]"
	}

	return "::uuid[]"
}

func orderBy(sort []entity.IssueSort) string {
	terms := make([]string, 0, len(sort)+1)

	for _, key := range sort {
		column := sortColumns[key.Field]
		direction := "ASC"

		if key.Descending {
			direction = "DESC"
		}

		if nullableSorts[key.Field] {
			direction += " NULLS LAST"
		}

		terms = append(terms, column+" "+direction)
	}

	terms = append(terms, "i.id "+tiebreak(sort))

	return "ORDER BY " + strings.Join(terms, ", ")
}

func tiebreak(sort []entity.IssueSort) string {
	if len(sort) > 0 && !sort[0].Descending {
		return "ASC"
	}

	return "DESC"
}

func (b *builder) keyset(sort []entity.IssueSort, cursor *entity.IssueQueryCursor) string {
	if cursor == nil {
		return ""
	}

	if len(cursor.Keys) != len(sort) {
		return ""
	}

	if row := b.rowComparison(sort, cursor); row != "" {
		return row
	}

	rungs := make([]string, 0, len(sort)+1)
	equalities := make([]string, 0, len(sort))

	for i, key := range sort {
		column := sortColumns[key.Field]
		value := cursor.Keys[i]

		comparison := ">"
		if key.Descending {
			comparison = "<"
		}

		rungs = append(rungs, strings.Join(append(
			append([]string{}, equalities...),
			b.step(column, value, comparison, nullableSorts[key.Field]),
		), " AND "))

		equalities = append(equalities, b.sameAs(column, value))
	}

	rungs = append(rungs, strings.Join(append(
		append([]string{}, equalities...),
		fmt.Sprintf("i.id %s %s::uuid", stepComparison(tiebreak(sort)), b.bind(cursor.IssueID.String())),
	), " AND "))

	return "(" + strings.Join(rungs, ") OR (") + ")"
}

func (b *builder) rowComparison(sort []entity.IssueSort, cursor *entity.IssueQueryCursor) string {
	descending := sort[0].Descending

	for i, key := range sort {
		if nullableSorts[key.Field] || key.Descending != descending || cursor.Keys[i] == "" {
			return ""
		}
	}

	columns := make([]string, 0, len(sort)+1)
	values := make([]string, 0, len(sort)+1)

	for i, key := range sort {
		columns = append(columns, sortColumns[key.Field])
		values = append(values, b.bind(cursor.Keys[i]))
	}

	columns = append(columns, "i.id")
	values = append(values, b.bind(cursor.IssueID.String())+"::uuid")

	return fmt.Sprintf(
		"(%s) %s (%s)",
		strings.Join(columns, ", "),
		stepComparison(tiebreak(sort)),
		strings.Join(values, ", "),
	)
}

func stepComparison(direction string) string {
	if direction == "ASC" {
		return ">"
	}

	return "<"
}

func (b *builder) step(column, value, comparison string, nullable bool) string {
	if value == "" {
		return "FALSE"
	}

	compared := fmt.Sprintf("%s %s %s", column, comparison, b.bind(value))

	if !nullable {
		return "(" + compared + ")"
	}

	return "(" + compared + " OR " + column + " IS NULL)"
}

func (b *builder) sameAs(column, value string) string {
	if value == "" {
		return fmt.Sprintf("%s IS NULL", column)
	}

	return fmt.Sprintf("%s = %s", column, b.bind(value))
}

func (r *issueRepository) pageStatement(
	scope entity.TeamScope,
	page entity.IssuePage,
) (string, []any) {
	b := &builder{}

	sort, err := entity.NormalizedIssueSort(page.Sort)
	if err != nil {
		sort = entity.DefaultIssueSort()
	}

	where := b.where(scope, page, page.Filter) + b.legacy(page)

	if keyset := b.keyset(sort, page.QueryCursor); keyset != "" {
		where += " AND (" + keyset + ")"
	}

	statement := "SELECT" + issueColumns + issueJoins +
		"\nWHERE " + where +
		"\n" + orderBy(sort) +
		"\nLIMIT " + b.bind(page.Limit)

	return statement, b.args
}

func (b *builder) legacy(page entity.IssuePage) string {
	clause := ""

	if len(page.Statuses) > 0 {
		clause += fmt.Sprintf(" AND i.status = ANY(%s::text[])", b.bind(statusNames(page.Statuses)))
	}

	if page.CycleID != nil {
		clause += fmt.Sprintf(" AND i.cycle_id = %s::uuid", b.bind(page.CycleID.String()))
	}

	if page.ProjectID != nil {
		clause += fmt.Sprintf(" AND i.project_id = %s::uuid", b.bind(page.ProjectID.String()))
	}

	return clause
}

func tallyStatement(
	scope entity.TeamScope,
	page entity.IssuePage,
	key string,
	groupBy entity.IssueGroupBy,
) (string, []any) {
	b := &builder{}
	where := b.where(scope, page, page.Filter) + b.legacy(page)

	joins := issueJoins
	if groupBy == entity.IssueGroupByLabel {
		joins += groupLabelJoin
	}

	return "SELECT " + key + ", " + tallyExpression + joins + "\nWHERE " + where + "\nGROUP BY 1", b.args
}

const tallyExpression = "count(*)"

func (r *issueRepository) TallyByGroup(
	ctx context.Context,
	scope entity.TeamScope,
	page entity.IssuePage,
	groupBy entity.IssueGroupBy,
) ([]entity.IssueGroupTally, error) {
	key, known := groupKeys[groupBy]
	if !known {
		return nil, entity.ErrIssueGroupUnknown
	}

	statement, args := tallyStatement(scope, page, key, groupBy)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("tally issues by group: %w", err)
	}

	defer func() { _ = rows.Close() }()

	tallies := make([]entity.IssueGroupTally, 0)

	for rows.Next() {
		var tally entity.IssueGroupTally

		if err := rows.Scan(&tally.Key, &tally.Issues); err != nil {
			return nil, fmt.Errorf("scan group tally: %w", err)
		}

		tallies = append(tallies, tally)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group tallies: %w", err)
	}

	return tallies, nil
}
