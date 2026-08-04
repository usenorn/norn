package module_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/usenorn/norn"

func packagesUnder(t *testing.T, root string) map[string][]string {
	t.Helper()

	repo := repoRoot(t)
	imports := make(map[string][]string)

	err := filepath.Walk(filepath.Join(repo, root), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		pkg, err := build.ImportDir(path, 0)
		if err != nil {
			return nil
		}

		relative, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}

		imports[filepath.ToSlash(relative)] = pkg.Imports

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return imports
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}

		dir = parent
	}
}

func internal(path string) string {
	return strings.TrimPrefix(path, modulePath+"/")
}

func TestTheAuthorizerDependsOnNoOtherService(t *testing.T) {
	for pkg, imports := range packagesUnder(t, "internal/service/authorizer") {
		for _, imported := range imports {
			if !strings.HasPrefix(imported, modulePath) {
				continue
			}

			path := internal(imported)

			if path == "internal/service" {
				continue
			}

			if strings.HasPrefix(path, "internal/service/") {
				t.Errorf(
					"%s imports %s; the authorizer must stay a sink so it can never join a service cycle",
					pkg,
					path,
				)
			}
		}
	}
}

func TestEntityDependsOnNoOtherInternalPackage(t *testing.T) {
	for pkg, imports := range packagesUnder(t, "internal/entity") {
		for _, imported := range imports {
			if !strings.HasPrefix(imported, modulePath) {
				continue
			}

			t.Errorf("%s imports %s; the domain layer must depend on nothing", pkg, internal(imported))
		}
	}
}

func TestHandlersNeverReachPastTheServiceLayer(t *testing.T) {
	for pkg, imports := range packagesUnder(t, "internal/handler") {
		for _, imported := range imports {
			if !strings.HasPrefix(imported, modulePath) {
				continue
			}

			path := internal(imported)

			switch {
			case path == "internal/repository":
			case strings.HasPrefix(path, "internal/repository/"):
				t.Errorf("%s imports %s; edges call business logic, never a repository implementation", pkg, path)
			case strings.Count(path, "/") > 1 && strings.HasPrefix(path, "internal/service/"):
				t.Errorf("%s imports %s; edges depend on service interfaces, not implementations", pkg, path)
			}
		}
	}
}

func TestConfigDependsOnNoOtherInternalPackage(t *testing.T) {
	for pkg, imports := range packagesUnder(t, "internal/config") {
		for _, imported := range imports {
			if !strings.HasPrefix(imported, modulePath) {
				continue
			}

			t.Errorf("%s imports %s; configuration sits at the bottom of the graph", pkg, internal(imported))
		}
	}
}

func TestGeneratedPersistenceModelsAreReachedOnlyByRepositories(t *testing.T) {
	const models = "internal/db/postgres"

	for _, root := range []string{"internal/service", "internal/handler", "internal/entity"} {
		for pkg, imports := range packagesUnder(t, root) {
			for _, imported := range imports {
				if internal(imported) == models {
					t.Errorf("%s imports %s; persistence models never leave the data-access layer", pkg, models)
				}
			}
		}
	}
}
