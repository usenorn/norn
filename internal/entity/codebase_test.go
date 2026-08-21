package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func repository(name, path, hash string) entity.CodebaseRepository {
	return entity.CodebaseRepository{
		Name:          name,
		RelPath:       path,
		DefaultBranch: "main",
		Remote:        entity.RemoteFingerprint{Hash: hash, Host: "github.com", PathTail: "usenorn/" + name},
	}
}

func TestDriftIsAboutWhichRepositoriesAreThereRatherThanTheirOrder(t *testing.T) {
	api := repository("api", "api", "aaa")
	web := repository("web", "web", "bbb")

	cases := map[string]struct {
		before, after []entity.CodebaseRepository
		same          bool
	}{
		"identical":            {[]entity.CodebaseRepository{api, web}, []entity.CodebaseRepository{api, web}, true},
		"reordered":            {[]entity.CodebaseRepository{api, web}, []entity.CodebaseRepository{web, api}, true},
		"one added":            {[]entity.CodebaseRepository{api}, []entity.CodebaseRepository{api, web}, false},
		"one removed":          {[]entity.CodebaseRepository{api, web}, []entity.CodebaseRepository{api}, false},
		"both empty":           {nil, nil, true},
		"branch changed":       {[]entity.CodebaseRepository{api}, []entity.CodebaseRepository{{Name: api.Name, RelPath: api.RelPath, DefaultBranch: "trunk", Remote: api.Remote}}, false},
		"remote changed":       {[]entity.CodebaseRepository{api}, []entity.CodebaseRepository{repository("api", "api", "ccc")}, false},
		"moved to a new path":  {[]entity.CodebaseRepository{api}, []entity.CodebaseRepository{repository("api", "services/api", "aaa")}, false},
		"renamed in the place": {[]entity.CodebaseRepository{api}, []entity.CodebaseRepository{repository("gateway", "api", "aaa")}, false},
	}

	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			if got := entity.SameRepositorySet(expected.before, expected.after); got != expected.same {
				t.Fatalf("comparing the %s sets reported same=%v, want %v", name, got, expected.same)
			}
		})
	}
}

func TestOnlyTheThreeKnownCodebaseStatesAreValid(t *testing.T) {
	for _, state := range entity.CodebaseStates() {
		if !state.Valid() {
			t.Errorf("%q is listed as a codebase state but does not validate", state)
		}
	}

	for _, state := range []entity.CodebaseState{"", "connected", "Active", "drifted"} {
		if state.Valid() {
			t.Errorf("%q validates as a codebase state and should not", state)
		}
	}
}

func TestOnlyTheThreeKnownRuntimesAreValid(t *testing.T) {
	for _, runtime := range entity.CodebaseRuntimes() {
		if !runtime.Valid() {
			t.Errorf("%q is listed as a runtime but does not validate", runtime)
		}
	}

	for _, runtime := range []entity.CodebaseRuntime{"", "podman", "Docker"} {
		if runtime.Valid() {
			t.Errorf("%q validates as a runtime and should not", runtime)
		}
	}
}

func TestACodebaseMustNameItselfAndSayWhereItIs(t *testing.T) {
	cases := map[string]struct {
		field string
		value string
		code  string
	}{
		"blank name":      {"name", "   ", entity.ValidationCodeRequired},
		"overlong name":   {"name", strings.Repeat("n", entity.CodebaseNameMaxLen+1), entity.ValidationCodeTooLong},
		"blank root":      {"rootPath", "", entity.ValidationCodeRequired},
		"overlong root":   {"rootPath", strings.Repeat("p", entity.CodebaseRootPathMaxLen+1), entity.ValidationCodeTooLong},
		"acceptable name": {"name", "norn", ""},
		"acceptable root": {"rootPath", "/Users/vlad/projects/norn", ""},
	}

	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			var got entity.FieldError

			if expected.field == "name" {
				got = entity.ValidateCodebaseName("name", expected.value)
			} else {
				got = entity.ValidateCodebaseRootPath("rootPath", expected.value)
			}

			if got.Code != expected.code {
				t.Fatalf("validating %s gave code %q, want %q", expected.field, got.Code, expected.code)
			}
		})
	}
}

func TestTwoRepositoriesCannotShareOnePath(t *testing.T) {
	fields := entity.ValidateCodebaseRepositories("repositories", []entity.CodebaseRepository{
		repository("api", "services", "aaa"),
		repository("web", "services", "bbb"),
	})

	if err := entity.NewValidationError(fields...); err == nil {
		t.Fatal("two repositories at the same relative path were accepted, so the unique index would reject the write instead")
	}
}

func TestAnUnknownRuntimeIsRefusedRatherThanStored(t *testing.T) {
	fields := entity.ValidateCodebaseRuntimes("runtimes", []entity.CodebaseRuntime{
		entity.CodebaseRuntimeProcess,
		"podman",
	})

	if err := entity.NewValidationError(fields...); err == nil {
		t.Fatal("an unknown runtime was accepted, so the runner could teach the server capabilities it cannot honour")
	}
}
