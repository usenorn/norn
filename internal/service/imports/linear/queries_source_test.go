package linear_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func queriesSource(t *testing.T) string {
	t.Helper()

	body, err := os.ReadFile("queries.go")
	if err != nil {
		t.Fatalf("read queries.go: %v", err)
	}

	return string(body)
}

func TestEveryNestedConnectionSaysWhetherItHeldMore(t *testing.T) {
	source := queriesSource(t)

	nested := regexp.MustCompile(`(?m)^\s+(\w+)\(first: \d+\)\s*\{`).FindAllStringSubmatchIndex(source, -1)

	if len(nested) == 0 {
		t.Fatal("no nested connection found in queries.go, so this guard is watching nothing")
	}

	for _, found := range nested {
		name := source[found[2]:found[3]]

		depth, end := 0, -1

		for index := found[1] - 1; index < len(source); index++ {
			switch source[index] {
			case '{':
				depth++
			case '}':
				depth--

				if depth == 0 {
					end = index
				}
			}

			if end >= 0 {
				break
			}
		}

		if end < 0 {
			t.Fatalf("the nested connection %q is never closed", name)
		}

		if !strings.Contains(source[found[1]:end], "pageInfo") {
			t.Errorf(
				"the nested connection %q asks for a fixed page and never asks whether there was "+
					"more. A nested connection cannot be resumed from the run's cursor, which "+
					"addresses the issue walk, so the remainder is not carried — and without "+
					"pageInfo nothing can even say so. An issue with more comments than the page "+
					"holds would arrive quietly incomplete.",
				name,
			)
		}
	}
}
