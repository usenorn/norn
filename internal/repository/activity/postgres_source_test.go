package activity

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository"
)

var reads = activityColumns + oldestLeaders + newestLeaders + operationRows

func TestTheActivityInsertWritesEveryColumnItNames(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO workspace_activity \((.*?)\)`).
		FindStringSubmatch(recordActivityQuery)
	if columns == nil {
		t.Fatal("could not find the activity insert's column list")
	}

	named := strings.Count(columns[1], ",") + 1

	values := regexp.MustCompile(`VALUES \((.*?)\)`).FindStringSubmatch(recordActivityQuery)
	if values == nil {
		t.Fatal("could not find the activity insert's VALUES list")
	}

	placeholders := strings.Count(values[1], "$")

	if named != placeholders {
		t.Fatalf(
			"the insert names %d columns but binds %d values. A column added to the table and to "+
				"the entity but not here is written as its default, so the field silently never "+
				"persists and nothing fails loudly.",
			named, placeholders,
		)
	}
}

func TestWhatTheActivityInsertWritesTheActivityReadReturns(t *testing.T) {
	columns := regexp.MustCompile(`(?s)INSERT INTO workspace_activity \((.*?)\)`).
		FindStringSubmatch(recordActivityQuery)
	if columns == nil {
		t.Fatal("could not find the activity insert's column list")
	}

	for _, raw := range strings.Split(columns[1], ",") {
		column := strings.TrimSpace(raw)

		if column == "" {
			continue
		}

		if !strings.Contains(reads, column) {
			t.Errorf(
				"the insert writes %s but no read query selects it, so the value is stored and "+
					"then thrown away on every read",
				column,
			)
		}
	}
}

func TestNothingInThisPackageRewritesOrRemovesActivity(t *testing.T) {
	mutating := regexp.MustCompile(`(?is)(update|delete\s+from)\s+workspace_activity`)

	for name, query := range map[string]string{
		"recordActivityQuery": recordActivityQuery,
		"activityColumns":     activityColumns,
		"oldestLeaders":       oldestLeaders,
		"newestLeaders":       newestLeaders,
		"operationRows":       operationRows,
	} {
		if mutating.MatchString(query) {
			t.Errorf(
				"%s rewrites or removes activity. Activity is immutable — not editable and not "+
					"deletable by anyone, administrators included — so the only statement this "+
					"table may ever see from the application is an insert.",
				name,
			)
		}
	}
}

func TestTheActivitySurfaceOffersNoWayToChangeWhatHappened(t *testing.T) {
	forbidden := []string{"update", "delete", "remove", "edit", "purge", "set"}

	surface := reflect.TypeOf((*repository.Activity)(nil)).Elem()

	for i := range surface.NumMethod() {
		name := strings.ToLower(surface.Method(i).Name)

		for _, verb := range forbidden {
			if strings.Contains(name, verb) {
				t.Errorf(
					"repository.Activity exposes %q. A record of what happened cannot be "+
						"corrected after the fact; the moment the surface offers a way, some "+
						"caller will find a reason.",
					surface.Method(i).Name,
				)
			}
		}
	}
}

func TestEveryReadIsConfinedToOneSubject(t *testing.T) {
	for name, query := range map[string]string{
		"oldestLeaders": oldestLeaders,
		"newestLeaders": newestLeaders,
		"operationRows": operationRows,
	} {
		if !strings.Contains(query, "WHERE %s") {
			t.Errorf(
				"%s does not take a subject predicate. An operation may span two issues — "+
					"relations write one row on each side — so fetching its rows without a "+
					"subject filter would put a counterpart issue's reference into the feed of "+
					"an issue whose team the caller may not be scoped to.",
				name,
			)
		}
	}
}

func TestThePageIsTakenOverLeadersSoAnEventIsNeverSplit(t *testing.T) {
	for name, query := range map[string]string{
		"oldestLeaders": oldestLeaders,
		"newestLeaders": newestLeaders,
	} {
		if !strings.Contains(query, "a.id = a.operation_id") {
			t.Fatalf(
				"%s pages over rows rather than over the first row of each operation. A page "+
					"boundary would then land in the middle of one edit, and its cursor would "+
					"point at a change rather than at an event.",
				name,
			)
		}
	}

	if !strings.Contains(oldestLeaders, "(a.created_at, a.id) > ") {
		t.Fatal("the oldest-first window does not compare forward")
	}

	if !strings.Contains(newestLeaders, "(a.created_at, a.id) < ") {
		t.Fatal("the newest-first window does not compare backward")
	}
}
