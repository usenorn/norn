package scm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// declaredStatements reads the package's own source for every `const xQuery = ...`. Listing
// them by hand in the schema test would rot the moment somebody added one, and a statement
// missing from that list is a statement nothing checks.
func declaredStatements() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	set := token.NewFileSet()
	names := make([]string, 0, 64)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			return nil, err
		}

		names = append(names, queryConstants(file)...)
	}

	return names, nil
}

func queryConstants(file *ast.File) []string {
	found := make([]string, 0, 8)

	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}

		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for _, declared := range value.Names {
				if strings.HasSuffix(declared.Name, "Query") {
					found = append(found, declared.Name)
				}
			}
		}
	}

	return found
}
