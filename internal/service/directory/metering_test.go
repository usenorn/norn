package directory_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestNoDirectorySurfaceCountsThePeopleItProvisions(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "usage", "meter", "bill", "licence"}

	surfaces := map[string]reflect.Type{
		"service.Directories":      reflect.TypeOf((*service.Directories)(nil)).Elem(),
		"repository.Directory":     reflect.TypeOf((*repository.Directory)(nil)).Elem(),
		"repository.DirectorySync": reflect.TypeOf((*repository.DirectorySync)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ReplaceAll(strings.ToLower(surface.Method(i).Name), "account", "")

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf(
						"%s exposes %q. Provisioning any number of people is free in every tier, "+
							"so no directory surface may tally the people it manages — the moment "+
							"one does, somebody will price it.",
						name, surface.Method(i).Name,
					)
				}
			}
		}
	}
}

func TestNoDirectoryQueryTalliesMembership(t *testing.T) {
	packages := []string{".", "../../repository/directory"}

	for _, target := range packages {
		entries, err := os.ReadDir(target)
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}

		scanned := 0

		for _, entry := range entries {
			name := entry.Name()

			if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
				strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "mock_") {
				continue
			}

			body, err := os.ReadFile(filepath.Join(target, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}

			scanned++

			lowered := strings.ToLower(string(body))

			for _, phrase := range []string{
				"count(*) from workspace_memberships",
				"count(*) from directory_users",
				"count(*) from accounts",
			} {
				if strings.Contains(lowered, phrase) {
					t.Errorf(
						"%s/%s tallies provisioned people with %q. Directory synchronization "+
							"never counts the people it manages.",
						target, name, phrase,
					)
				}
			}
		}

		if scanned == 0 {
			t.Fatalf("%s has no source to guard", target)
		}
	}
}

func TestTheDirectoryFeatureNeverGatesSigningIn(t *testing.T) {
	features := reflect.TypeOf(entity.LicenceFeatures{})

	if _, ok := features.FieldByName("Directory"); !ok {
		t.Fatal("LicenceFeatures has no Directory field, so nothing gates directory synchronization")
	}

	unlicensed := entity.Licence{}

	if unlicensed.Permits(nowForTest(), 0, entity.FeatureDirectory) {
		t.Error("an absent licence permits directory synchronization")
	}

	authentication := []string{
		"../../entity/sso.go",
		"../../entity/oidc.go",
		"../../entity/saml.go",
		"../../service/ssoconnection/connections.go",
	}

	for _, path := range authentication {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		if strings.Contains(strings.ToLower(string(body)), "directoryfeature") {
			t.Errorf(
				"%s consults the directory feature. Signing in through a provider is free; "+
					"only automated provisioning is licensed.",
				path,
			)
		}
	}
}

func nowForTest() time.Time {
	return time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
}
