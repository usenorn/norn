package issue

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func scopeOf(teamIDs ...uuid.UUID) entity.TeamScope {
	return entity.TeamScope{WorkspaceID: uuid.New(), TeamIDs: teamIDs}
}

func TestAValueNeverReachesTheStatementItIsOnlyEverAnArgument(t *testing.T) {
	hostile := "'; DROP TABLE workspace_issues; --"

	b := &builder{}
	where := b.where(scopeOf(uuid.New()), entity.IssuePage{}, &entity.IssueFilter{
		Field:  entity.IssueFilterFieldPriority,
		Op:     entity.IssueFilterOpIs,
		Values: []string{hostile},
	})

	if strings.Contains(where, hostile) {
		t.Fatalf(
			"the filter value was written into the statement:\n%s\nA value must only ever reach "+
				"the database as an argument, or a filter expression becomes a way to run SQL.",
			where,
		)
	}

	found := false

	for _, arg := range b.args {
		if text, ok := arg.(string); ok && text == hostile {
			found = true
		}
	}

	if !found {
		t.Fatal("the filter value reached neither the statement nor the arguments, so it was silently dropped")
	}
}

func TestEveryCompiledStatementCarriesTheCallersScope(t *testing.T) {
	filters := map[string]*entity.IssueFilter{
		"no filter": nil,
		"empty":     {},
		"a leaf":    {Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}},
		"a disjunction the caller controls": {Any: []entity.IssueFilter{
			{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}},
			{Field: entity.IssueFilterFieldStatus, Op: entity.IssueFilterOpIs, Values: []string{"active"}},
		}},
		"a negation": {Not: &entity.IssueFilter{
			Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue,
		}},
	}

	for name, filter := range filters {
		t.Run(name, func(t *testing.T) {
			b := &builder{}
			where := b.where(scopeOf(uuid.New()), entity.IssuePage{}, filter)

			if !strings.Contains(where, "i.team_id = ANY(") {
				t.Fatalf(
					"the compiled statement has no team scope:\n%s\nPermission filtering has to "+
						"happen inside the query; a filtered list that scopes afterwards reports "+
						"counts and pages over issues the caller cannot see.",
					where,
				)
			}

			if !strings.HasPrefix(where, "i.workspace_id = ") {
				t.Errorf("the workspace predicate is not the first thing in:\n%s", where)
			}
		})
	}
}

func TestACallersDisjunctionCannotReachAroundTheScope(t *testing.T) {
	b := &builder{}
	where := b.where(scopeOf(uuid.New()), entity.IssuePage{}, &entity.IssueFilter{Any: []entity.IssueFilter{
		{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}},
		{Field: entity.IssueFilterFieldStatus, Op: entity.IssueFilterOpIs, Values: []string{"active"}},
	}})

	scope, expression, split := strings.Cut(where, " AND ")
	if !split {
		t.Fatalf("expected the scope and the expression to be joined by AND: %s", where)
	}

	if strings.Contains(scope, " OR ") && !strings.Contains(scope, "IS TRUE OR i.team_id") {
		t.Errorf("the scope clause contains a disjunction that is not its own: %s", scope)
	}

	if !strings.HasPrefix(expression, "(") || !strings.HasSuffix(expression, ")") {
		t.Fatalf(
			"the caller's expression is not parenthesised: %s\nAn unparenthesised OR would bind "+
				"looser than the scope AND and hand back every issue in the workspace.",
			expression,
		)
	}
}

func TestSortingAlwaysEndsInATotalOrder(t *testing.T) {
	for name, sort := range map[string][]entity.IssueSort{
		"the default":   entity.DefaultIssueSort(),
		"a single key":  {{Field: entity.IssueSortFieldPriority}},
		"mixed keys":    {{Field: entity.IssueSortFieldPriority}, {Field: entity.IssueSortFieldDueOn, Descending: true}},
		"nullable only": {{Field: entity.IssueSortFieldEstimate}},
	} {
		t.Run(name, func(t *testing.T) {
			clause := orderBy(sort)

			if !strings.HasSuffix(clause, "i.id DESC") && !strings.HasSuffix(clause, "i.id ASC") {
				t.Fatalf(
					"%q does not end in the id tiebreak. Without a total order two rows can sort "+
						"either way between requests, and a cursor cannot name a page boundary.",
					clause,
				)
			}
		})
	}
}

func TestAUniformSortPagesByComparingOneRowAgainstAnother(t *testing.T) {
	for name, sort := range map[string][]entity.IssueSort{
		"the default":      entity.DefaultIssueSort(),
		"ascending":        {{Field: entity.IssueSortFieldCreatedAt}},
		"two keys one way": {{Field: entity.IssueSortFieldUpdatedAt, Descending: true}, {Field: entity.IssueSortFieldCreatedAt, Descending: true}},
	} {
		t.Run(name, func(t *testing.T) {
			b := &builder{}
			cursor := entity.IssueQueryCursor{IssueID: uuid.New()}

			for range sort {
				cursor.Keys = append(cursor.Keys, "2026-08-04T09:00:00Z")
			}

			clause := b.keyset(sort, &cursor)

			if strings.Contains(clause, " OR ") {
				t.Fatalf(
					"the keyset for a sort that runs one way is a disjunction:\n%s\nPostgres cannot "+
						"turn an OR into an index condition, so every deep page walks the rows before "+
						"it. A row comparison is the same question asked in a form the index answers.",
					clause,
				)
			}

			if !strings.Contains(clause, "i.id") {
				t.Fatalf("the keyset does not carry the tiebreak, so it cannot name a unique boundary:\n%s", clause)
			}
		})
	}
}

func TestAMixedSortStillPagesCorrectlyEvenThoughItCannotCollapse(t *testing.T) {
	b := &builder{}
	sort := []entity.IssueSort{
		{Field: entity.IssueSortFieldPriority},
		{Field: entity.IssueSortFieldDueOn, Descending: true},
	}
	cursor := entity.IssueQueryCursor{Keys: []string{"2", ""}, IssueID: uuid.New()}

	clause := b.keyset(sort, &cursor)

	if !strings.Contains(clause, "i.due_on IS NULL") {
		t.Fatalf(
			"the boundary row has no due date and the keyset does not say so:\n%s\nUnder NULLS LAST "+
				"the rows after it are the other undated ones, which only an IS NULL equality reaches.",
			clause,
		)
	}

	if !strings.Contains(clause, "i.id") {
		t.Fatalf("the expanded keyset dropped the tiebreak:\n%s", clause)
	}
}

func TestANullableKeyRanksItsNullsWithoutWrappingTheColumn(t *testing.T) {
	for _, field := range entity.IssueSortFields() {
		for _, descending := range []bool{false, true} {
			t.Run(string(field), func(t *testing.T) {
				clause := orderBy([]entity.IssueSort{{Field: field, Descending: descending}})

				if nullableSorts[field] && !strings.Contains(clause, "NULLS LAST") {
					t.Fatalf(
						"%q leaves the NULL position to the direction's default, so issues with no "+
							"value sit at the top when the sort is reversed",
						clause,
					)
				}

				if strings.Contains(clause, "IS NULL") {
					t.Fatalf(
						"%q ranks NULLs with an expression rather than NULLS LAST. A leading "+
							"expression term is not a column, so no index can satisfy the ordering "+
							"and every page scans and sorts the whole table.",
						clause,
					)
				}
			})
		}
	}
}

func TestAGroupKeyIsNeverTakenFromTheCaller(t *testing.T) {
	if _, known := groupKeys[entity.IssueGroupBy("i.id; DROP TABLE workspace_issues")]; known {
		t.Fatal("an arbitrary string resolved to a group key")
	}

	for _, group := range entity.IssueGroupBys() {
		if _, known := groupKeys[group]; !known {
			t.Errorf("%q is offered by the domain but has no column, so grouping by it would fail at runtime", group)
		}
	}
}

func TestEverySortableFieldHasAColumn(t *testing.T) {
	for _, field := range entity.IssueSortFields() {
		if _, known := sortColumns[field]; !known {
			t.Errorf("%q is offered as a sort key but has no column", field)
		}
	}
}

func TestEveryFilterableFieldCompiles(t *testing.T) {
	special := map[entity.IssueFilterField]bool{
		entity.IssueFilterFieldLabel:       true,
		entity.IssueFilterFieldBlocked:     true,
		entity.IssueFilterFieldHasChildren: true,
		entity.IssueFilterFieldIsChild:     true,
	}

	for _, field := range entity.IssueFilterFields() {
		if special[field] {
			continue
		}

		if _, known := filterColumns[field]; !known {
			t.Errorf("%q is offered by the domain but has no column, so filtering on it would silently match nothing", field)
		}
	}
}

func TestEveryTallyIsBuiltInsideTheCallersScope(t *testing.T) {
	for _, group := range entity.IssueGroupBys() {
		t.Run(string(group), func(t *testing.T) {
			statement, _ := tallyStatement(
				scopeOf(uuid.New()),
				entity.IssuePage{Filter: &entity.IssueFilter{
					Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"},
				}},
				groupKeys[group],
				group,
			)

			if !strings.Contains(statement, tallyExpression) {
				t.Fatalf("the %s tally does not count anything:\n%s", group, statement)
			}

			if !strings.Contains(statement, "i.team_id = ANY(") {
				t.Fatalf(
					"the %s tally counts without the caller's team scope:\n%s\n"+
						"A grouped count that reaches past the scope discloses, by subtraction, "+
						"the contents of a team the caller cannot see.",
					group, statement,
				)
			}
		})
	}
}

func TestThePageStatementIsAlwaysScopedAndTotallyOrdered(t *testing.T) {
	statement, args := (&issueRepository{}).pageStatement(scopeOf(uuid.New()), entity.IssuePage{
		Limit:  50,
		Filter: &entity.IssueFilter{Field: entity.IssueFilterFieldBlocked, Op: entity.IssueFilterOpIsTrue},
		Sort:   []entity.IssueSort{{Field: entity.IssueSortFieldDueOn}},
	})

	if !strings.Contains(statement, "i.team_id = ANY(") {
		t.Fatalf("the page statement is unscoped:\n%s", statement)
	}

	if !strings.Contains(statement, "ORDER BY") || !strings.Contains(statement, "i.id ") {
		t.Fatalf("the page statement has no total order:\n%s", statement)
	}

	if len(args) == 0 {
		t.Fatal("the page statement bound no arguments, so something was inlined")
	}
}

func TestEveryCompiledStatementDecidesAboutTriage(t *testing.T) {
	filters := map[string]*entity.IssueFilter{
		"no filter": nil,
		"a leaf":    {Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}},
		"a disjunction": {Any: []entity.IssueFilter{
			{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"}},
			{Field: entity.IssueFilterFieldStatus, Op: entity.IssueFilterOpIs, Values: []string{"active"}},
		}},
	}

	for name, filter := range filters {
		t.Run(name, func(t *testing.T) {
			b := &builder{}

			if where := b.where(scopeOf(uuid.New()), entity.IssuePage{}, filter); !strings.Contains(where, decided) {
				t.Fatalf(
					"the compiled statement admits issues still waiting in triage:\n%s\n"+
						"A backlog that counts work nobody has agreed to do is the thing triage "+
						"exists to prevent, so the predicate belongs beside the scope where no "+
						"caller can forget it.",
					where,
				)
			}

			queue := &builder{}

			if where := queue.where(scopeOf(uuid.New()), entity.IssuePage{Waiting: true}, filter); !strings.Contains(where, stillWaiting) {
				t.Fatalf(
					"the queue's own statement does not ask for waiting work:\n%s\nThe backlog and "+
						"the queue are the same engine asking opposite questions; if the flag is "+
						"ignored the queue silently shows the backlog.",
					where,
				)
			}
		})
	}

	for _, group := range entity.IssueGroupBys() {
		statement, _ := tallyStatement(scopeOf(uuid.New()), entity.IssuePage{}, groupKeys[group], group)

		if !strings.Contains(statement, decided) {
			t.Errorf("the %s tally counts issues still waiting in triage", group)
		}
	}
}
