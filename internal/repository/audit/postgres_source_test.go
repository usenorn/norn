package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository"
)

func packageSource(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		combined.Write(body)
	}

	if combined.Len() == 0 {
		t.Fatal("this guard found no source, so it is protecting nothing")
	}

	return combined.String()
}

func TestNothingRewritesAnAuditEvent(t *testing.T) {
	rewriting := regexp.MustCompile(`(?is)update\s+audit_events`)

	if rewriting.MatchString(packageSource(t)) {
		t.Error(
			"this package rewrites an audit event. The record of what happened cannot be " +
				"corrected afterwards by anybody, instance administrators included, so the only " +
				"statements this table may see are an insert and the retention sweep.",
		)
	}
}

func TestTheOnlyDeleteIsTheRetentionSweepAndItFiltersOnAgeAlone(t *testing.T) {
	deleting := regexp.MustCompile(`(?is)delete\s+from\s+audit_events`)

	source := packageSource(t)

	if found := len(deleting.FindAllString(source, -1)); found != 1 {
		t.Fatalf(
			"this package has %d statements that remove audit events. There may be exactly one, "+
				"the retention sweep; a second is how somebody eventually removes a particular "+
				"entry.",
			found,
		)
	}

	sweep := dropExpiredQuery

	for _, column := range []string{
		"id =", "workspace_id", "actor_account_id", "action", "resource_id", "actor_kind",
	} {
		if strings.Contains(strings.ToLower(sweep), column) && column != "id =" {
			t.Errorf(
				"the retention sweep filters on %q. It may filter on age and nothing else, or it "+
					"becomes a way to make one inconvenient record disappear.",
				column,
			)
		}
	}

	if !strings.Contains(strings.ToLower(sweep), "occurred_at <") {
		t.Error("the retention sweep does not filter on age at all")
	}
}

func TestTheAuditSurfaceOffersNoWayToChangeWhatHappened(t *testing.T) {
	forbidden := []string{"update", "delete", "remove", "edit", "purge", "set", "redact"}

	surface := reflect.TypeOf((*repository.Audit)(nil)).Elem()

	for i := range surface.NumMethod() {
		name := strings.ToLower(surface.Method(i).Name)

		for _, verb := range forbidden {
			if strings.Contains(name, verb) {
				t.Errorf(
					"repository.Audit exposes %q. Recording and reading are the whole surface; "+
						"the moment it offers a third verb, some caller will find a reason.",
					surface.Method(i).Name,
				)
			}
		}
	}
}

func TestNoGeneratedModelOffersAWayToRewriteTheTable(t *testing.T) {
	generated, err := os.ReadDir(filepath.Join("..", "..", "db", "postgres"))
	if err != nil {
		t.Fatalf("reading the generated models: %v", err)
	}

	for _, entry := range generated {
		if strings.Contains(entry.Name(), "audit_event") {
			t.Fatalf(
				"%s exists, so audit_events has a generated model and every repository can now "+
					"call UpdateAll and DeleteAll on it. Keep the table blacklisted in "+
					"sqlboiler.toml; raw SQL in this package is the only surface the guards cover.",
				entry.Name(),
			)
		}
	}
}

func TestTheAuditInsertWritesEveryColumnItNames(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO audit_events \((.*?)\)`).
		FindStringSubmatch(recordAuditQuery)
	if columns == nil {
		t.Fatal("could not find the audit insert's column list")
	}

	named := strings.Count(columns[1], ",") + 1

	values := regexp.MustCompile(`VALUES \((.*?)\)`).FindStringSubmatch(recordAuditQuery)
	if values == nil {
		t.Fatal("could not find the audit insert's VALUES list")
	}

	if placeholders := strings.Count(values[1], "$"); named != placeholders {
		t.Fatalf(
			"the insert names %d columns but binds %d values, so a fact somebody thought was "+
				"being recorded is silently being dropped.",
			named, placeholders,
		)
	}
}

func TestWhatTheAuditInsertWritesTheAuditReadReturns(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO audit_events \((.*?)\)`).
		FindStringSubmatch(recordAuditQuery)
	if columns == nil {
		t.Fatal("could not find the audit insert's column list")
	}

	for _, raw := range strings.Split(columns[1], ",") {
		column := strings.TrimSpace(raw)

		if column == "" {
			continue
		}

		if !strings.Contains(auditColumns, column) {
			t.Errorf(
				"the insert writes %s but no read returns it, so it is stored and then thrown "+
					"away — and an audit record that cannot be read is not a record.",
				column,
			)
		}
	}
}
