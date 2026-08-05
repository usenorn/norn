package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestThePrefixLaneCarriesOnlyTheWordBeingTyped(t *testing.T) {
	query := entity.ParseSearchQuery("payments retr")

	if query.Stemmed != "payments retr" {
		t.Fatalf("the stemmed lane is %q, want the whole phrase", query.Stemmed)
	}

	if query.Prefix != "retr" {
		t.Fatalf(
			"the prefix lane is %q, want the final token. Every earlier word is already "+
				"complete; only the one under the cursor should match on a prefix.",
			query.Prefix,
		)
	}
}

func TestPunctuationNeverReachesTheTsqueryParser(t *testing.T) {
	for _, raw := range []string{
		"foo:bar",
		"R&D",
		"C++",
		"(",
		"!",
		"a | b",
		`"login page"`,
		"deploy   ",
		"<->",
	} {
		prefix := entity.ParseSearchQuery(raw).Prefix

		if strings.ContainsAny(prefix, `:&|!()<>*'" `) {
			t.Errorf(
				"parsing %q produced the prefix token %q, which still carries tsquery syntax. "+
					"That string is concatenated with ':*' and handed to to_tsquery, where an "+
					"operator character is a syntax error and a 500, not a bad search.",
				raw, prefix,
			)
		}
	}
}

func TestAQuoteNeverBecomesAPhraseOperator(t *testing.T) {
	query := entity.ParseSearchQuery(`"retry gateway"`)

	if strings.Contains(query.Stemmed, `"`) {
		t.Fatal(
			"a quote survived into the stemmed lane. websearch_to_tsquery turns quotes into <->, " +
				"and the search document concatenates title and description, so a phrase can " +
				"match across the seam between the last title word and the first body word.",
		)
	}

	if query.Stemmed != "retry gateway" {
		t.Fatalf("the quoted phrase became %q, want the words on their own", query.Stemmed)
	}
}

func TestAReferenceIsFoundWithoutSwallowingTheTextSearch(t *testing.T) {
	query := entity.ParseSearchQuery("ENG-412")

	if query.Reference == nil {
		t.Fatal("ENG-412 did not parse as a reference")
	}

	if query.Reference.Key != "ENG" || query.Reference.Number != 412 {
		t.Fatalf("parsed the reference as %+v", *query.Reference)
	}

	if query.Stemmed == "" {
		t.Fatal(
			"a reference match emptied the text lanes. ParseIssueReference also claims ordinary " +
				"words like re-2 and mac-1, so the reference is an extra result, never a reason " +
				"to stop searching text.",
		)
	}
}

func TestAnOversizedQueryIsCutRatherThanRefused(t *testing.T) {
	query := entity.ParseSearchQuery(strings.Repeat("alpha ", 400))

	if len(strings.Fields(query.Stemmed)) > entity.SearchQueryMaxTokens {
		t.Fatalf(
			"a %d-token query kept %d tokens. An unbounded tsquery is a CPU denial of service, "+
				"and nobody types forty terms on purpose.",
			400, len(strings.Fields(query.Stemmed)),
		)
	}

	if len(entity.ParseSearchQuery(strings.Repeat("x", 5000)).Prefix) > entity.SearchTokenMaxLen {
		t.Fatal("a single enormous token was not cut to the token limit")
	}
}

func TestAnEmptyQueryIsRecognisableRatherThanAWildcard(t *testing.T) {
	for _, raw := range []string{"", "   ", `"""`, "!!!"} {
		if !entity.ParseSearchQuery(raw).Empty() {
			t.Errorf(
				"%q did not parse as empty. An empty query that reaches the repository matches "+
					"every row in the workspace.",
				raw,
			)
		}
	}
}

func TestNonLatinInputSurvivesParsing(t *testing.T) {
	query := entity.ParseSearchQuery("привет мир")

	if query.Stemmed != "привет мир" {
		t.Fatalf("cyrillic input became %q", query.Stemmed)
	}

	if query.Prefix != "мир" {
		t.Fatalf("the cyrillic prefix token is %q, want мир", query.Prefix)
	}
}
