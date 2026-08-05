package entity_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestEveryAuditActionIsInTheAllowList(t *testing.T) {
	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(fileSet, "audit.go", nil, 0)
	if err != nil {
		t.Fatalf("parse audit.go: %v", err)
	}

	declared := make([]string, 0, 40)

	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}

		named, ok := spec.Type.(*ast.Ident)
		if !ok || named.Name != "AuditAction" {
			return true
		}

		for _, value := range spec.Values {
			literal, ok := value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}

			declared = append(declared, strings.Trim(literal.Value, `"`))
		}

		return true
	})

	if len(declared) == 0 {
		t.Fatal("found no AuditAction constants, so this guard is protecting nothing")
	}

	for _, action := range declared {
		if !entity.AuditAction(action).Valid() {
			t.Errorf(
				"the action %q is declared but missing from AuditActions(), so Valid() rejects it "+
					"and anything filtering by it silently returns nothing. The list is maintained "+
					"by hand; every new action must be added twice.",
				action,
			)
		}
	}

	if len(entity.AuditActions()) != len(declared) {
		t.Errorf(
			"AuditActions() lists %d actions but %d are declared",
			len(entity.AuditActions()), len(declared),
		)
	}
}
