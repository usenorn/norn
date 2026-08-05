package search

import (
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func statements(t *testing.T) map[string]string {
	t.Helper()

	found := map[string]string{"fuzzyIssueResults": fuzzyIssueResults}

	for _, kind := range entity.SearchKinds() {
		statement, _ := statementFor(kind)
		found[string(kind)] = statement
	}

	return found
}

func TestEveryTeamOwnedLaneHonoursTheActorsNarrowedScope(t *testing.T) {
	for _, kind := range []entity.SearchKind{
		entity.SearchKindIssue, entity.SearchKindComment, entity.SearchKindTeam,
	} {
		statement, scoped := statementFor(kind)

		if !scoped {
			t.Errorf(
				"%s is not marked scoped, so its query never receives the actor's team scope and "+
					"a narrowed credential would search beyond the teams it was granted",
				kind,
			)
		}

		if !strings.Contains(statement, "::uuid[]") {
			t.Errorf("%s never binds the scope's team list", kind)
		}
	}

	if !strings.Contains(fuzzyIssueResults, "::uuid[]") {
		t.Error("the fuzzy lane never binds the scope's team list")
	}
}

func TestEveryKindNarrowsToTheSearchersOwnWorkspaceMembership(t *testing.T) {
	for name, query := range statements(t) {
		if !strings.Contains(query, "$1") || !strings.Contains(query, "$2") {
			t.Errorf(
				"%s does not bind both the workspace and the searcher. Every row search can "+
					"reach has to be reached through the searcher's own membership; a query that "+
					"forgets one of them returns another workspace's rows or another person's view.",
				name,
			)
		}

		if !strings.Contains(query, "workspace_memberships m") {
			t.Errorf("%s never joins the searcher's membership", name)
		}
	}
}

func TestEveryQueryOverTeamOwnedRowsAppliesTheSameThreeVisibilityRules(t *testing.T) {
	rules := []string{
		"m.role = 'admin'",
		"t.visibility = 'public'",
		"tm.account_id IS NOT NULL",
	}

	for _, name := range []string{"issue", "comment", "team", "fuzzyIssueResults"} {
		query := statements(t)[name]

		for _, rule := range rules {
			if !strings.Contains(query, rule) {
				t.Errorf(
					"%s is missing %q. A private team's work is invisible everywhere else in the "+
						"product; search is the one surface that reads every table at once, so a "+
						"rule missing here hands back exactly what the rest of the app hides.",
					name, rule,
				)
			}
		}
	}
}

func TestNothingArchivedOrDeletedIsSearchable(t *testing.T) {
	for name, required := range map[string][]string{
		"issue":             {"i.status = 'active'", "t.status = 'active'"},
		"comment":           {"i.status = 'active'", "t.status = 'active'", "c.deleted_at IS NULL"},
		"project":           {"p.archived_at IS NULL"},
		"team":              {"t.status = 'active'"},
		"fuzzyIssueResults": {"i.status = 'active'", "t.status = 'active'"},
	} {
		query := statements(t)[name]

		for _, rule := range required {
			if !strings.Contains(query, rule) {
				t.Errorf(
					"%s does not require %q. Archiving is how people take work out of circulation, "+
						"and an issue in pending_deletion has already been deleted as far as its "+
						"author is concerned; handing either back through search undoes that.",
					name, rule,
				)
			}
		}
	}
}

func TestPeopleSearchNeverMatchesAnEmailAddressOrReturnsAnAgent(t *testing.T) {
	people := statements(t)["person"]

	if strings.Contains(people, "a.email") {
		t.Fatal(
			"people search matches on email. The members settings screen does that deliberately " +
				"because it is an admin surface; a palette every member can open would let anyone " +
				"recover a colleague's address by probing substrings.",
		)
	}

	for _, rule := range []string{"a.status = 'active'", "a.kind <> 'agent'"} {
		if !strings.Contains(people, rule) {
			t.Errorf("people search is missing %q", rule)
		}
	}
}

func TestNoSearchQueryCountsOrAggregates(t *testing.T) {
	aggregating := regexp.MustCompile(`(?i)\b(count|sum|avg|min|max|string_agg|array_agg)\s*\(`)

	for name, query := range statements(t) {
		if aggregating.MatchString(query) {
			t.Errorf(
				"%s aggregates. A result total is a fact about how much matching work exists, "+
					"and the honest bounded version still costs a full sort of the candidate set "+
					"on every keystroke. The group either has more results or it does not.",
				name,
			)
		}
	}
}

var textSearchCall = regexp.MustCompile(`(?:websearch_to_tsquery|to_tsquery)\('[a-z]+',\s*([^)]*)\)`)

func TestTheSearchersTextIsAlwaysBoundNeverInterpolated(t *testing.T) {
	for name, query := range statements(t) {
		for _, verb := range []string{"%s", "%v", "%[1]s", "%d"} {
			if strings.Contains(query, verb) {
				t.Errorf("%s still carries the unresolved format verb %q after assembly", name, verb)
			}
		}

		for _, call := range textSearchCall.FindAllStringSubmatch(query, -1) {
			argument := strings.TrimSpace(call[1])

			if !strings.HasPrefix(argument, "$") {
				t.Errorf(
					"%s builds a tsquery from %q rather than a bound parameter. A tsquery is a "+
						"little language: an unescaped ! or | changes what the query means, and an "+
						"unbalanced paren is a syntax error the searcher sees as a 500.",
					name, argument,
				)
			}
		}
	}
}

func TestTheCandidateSetIsBoundedBeforeItIsSorted(t *testing.T) {
	for _, name := range []string{"issue", "comment", "project", "team", "person"} {
		query := statements(t)[name]

		if !strings.Contains(query, "WITH candidates AS (") {
			t.Errorf("%s does not bound its candidates before sorting", name)

			continue
		}

		if strings.Index(query, "LIMIT $5") > strings.Index(query, "ORDER BY title_hit") {
			t.Errorf(
				"%s sorts before it bounds. GIN cannot return rows in order, so an unbounded "+
					"match set is sorted in full on every keystroke, and work_mem is 4MB.",
				name,
			)
		}
	}
}

func TestATitleMatchIsDistinguishedFromABodyMatchOnBothLanes(t *testing.T) {
	issues := statements(t)["issue"]

	if strings.Count(issues, "ts_rank_cd(ARRAY[0, 0, 0, 1]") != 2 {
		t.Fatal(
			"the weight-A test does not cover both lanes. Prefix matches arrive through the " +
				"unstemmed lane, so testing only the stemmed one ranks a half-typed title match " +
				"below a body match — the ordering inverts while the reader is still typing.",
		)
	}
}

func TestFuzzyMatchingPinsItsThresholdRatherThanInheritingTheSession(t *testing.T) {
	if !strings.HasPrefix(pinSimilarity, "SET LOCAL ") {
		t.Fatal(
			"the similarity threshold is not set with SET LOCAL. pg_trgm reads it from a session " +
				"GUC, and connections are pooled, so a plain SET leaks the value to whichever " +
				"request picks that connection up next.",
		)
	}

	if !strings.Contains(fuzzyIssueResults, "$3 <% i.title") {
		t.Fatal(
			"fuzzy matching does not use the word-similarity operator. Plain similarity() " +
				"normalises against the whole title, so a mistyped word scores 0.13 against a " +
				"six-word title and never clears any usable threshold; and similarity() is not " +
				"index-accelerated, so it turns every keystroke into a sequential scan.",
		)
	}

	if !strings.Contains(pinSimilarity, "word_similarity_threshold") {
		t.Fatal(
			"the pinned GUC is not the one <% reads. pg_trgm keeps similarity_threshold and " +
				"word_similarity_threshold separately, and setting the wrong one leaves <% on its " +
				"default of 0.6, which rejects ordinary typos.",
		)
	}
}
