package entity_test

import (
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestARankLandsStrictlyBetweenItsNeighbours(t *testing.T) {
	cases := map[string]struct{ before, after string }{
		"an empty list":                       {"", ""},
		"after everything":                    {"i", ""},
		"before everything":                   {"", "i"},
		"between two neighbours":              {"a", "b"},
		"between adjacent digits":             {"0i", "0j"},
		"before the smallest key":             {"", "01i"},
		"between a key and its own extension": {"i", "ii"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			rank, err := entity.RankBetween(testCase.before, testCase.after)
			if err != nil {
				t.Fatalf("RankBetween(%q, %q): %v", testCase.before, testCase.after, err)
			}

			if testCase.before != "" && rank <= testCase.before {
				t.Fatalf("%q does not sort after %q", rank, testCase.before)
			}

			if testCase.after != "" && rank >= testCase.after {
				t.Fatalf("%q does not sort before %q", rank, testCase.after)
			}

			if !entity.ValidIssueRank(rank) {
				t.Fatalf(
					"%q is not a rank anything can be placed before: a key ending in the "+
						"lowest digit leaves no room underneath it",
					rank,
				)
			}
		})
	}
}

func TestAnyNumberOfDropsBetweenTheSameTwoCardsKeepsWorking(t *testing.T) {
	before, after := "a", "b"

	for drop := range 200 {
		rank, err := entity.RankBetween(before, after)
		if err != nil {
			t.Fatalf("drop %d: %v", drop, err)
		}

		if rank <= before || rank >= after {
			t.Fatalf("drop %d put %q outside (%q, %q)", drop, rank, before, after)
		}

		after = rank
	}
}

func TestAListStaysInTheOrderItWasBuilt(t *testing.T) {
	held := []string{}

	for range 50 {
		last := ""
		if len(held) > 0 {
			last = held[len(held)-1]
		}

		rank, err := entity.RankBetween(last, "")
		if err != nil {
			t.Fatalf("RankBetween(%q, \"\"): %v", last, err)
		}

		held = append(held, rank)
	}

	for i := 1; i < len(held); i++ {
		if held[i-1] >= held[i] {
			t.Fatalf("appending produced %q then %q, which sort the wrong way round", held[i-1], held[i])
		}
	}
}

func TestBoundsInTheWrongOrderAreRefused(t *testing.T) {
	if _, err := entity.RankBetween("b", "a"); !errors.Is(err, entity.ErrIssueRankOutOfOrder) {
		t.Fatalf("RankBetween(\"b\", \"a\") = %v, want a refusal", err)
	}

	if _, err := entity.RankBetween("a", "a"); !errors.Is(err, entity.ErrIssueRankOutOfOrder) {
		t.Fatalf("RankBetween(\"a\", \"a\") = %v, want a refusal", err)
	}
}

func TestARankOutsideTheAlphabetIsRefused(t *testing.T) {
	for _, rank := range []string{"A", "-", "a b", "a0"} {
		if entity.ValidIssueRank(rank) {
			t.Fatalf("%q was accepted as a rank", rank)
		}

		if _, err := entity.RankBetween(rank, ""); err == nil {
			t.Fatalf("RankBetween(%q, \"\") was accepted", rank)
		}
	}
}
