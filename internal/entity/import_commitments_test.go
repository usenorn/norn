package entity_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func importMigration(t *testing.T) string {
	t.Helper()

	pattern := filepath.Join("..", "..", "db", "migrations", "postgres", "*_create_workspace_imports.sql")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}

	if len(matches) != 1 {
		t.Fatalf("found %d migrations creating the import tables, want exactly one", len(matches))
	}

	body, err := os.ReadFile(filepath.Clean(matches[0]))
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}

	return string(body)
}

func TestNothingAboutAnImportKnowsHowBigItIs(t *testing.T) {
	counting := []string{"expected", "total", "quota", "ceiling", "max_issues", "max_records", "limit"}

	structures := []reflect.Type{
		reflect.TypeOf(entity.ImportRun{}),
		reflect.TypeOf(entity.ImportCursor{}),
		reflect.TypeOf(entity.ImportRecord{}),
		reflect.TypeOf(entity.ImportLedgerEntry{}),
	}

	for _, structure := range structures {
		for index := range structure.NumField() {
			named := strings.ToLower(structure.Field(index).Name)

			for _, word := range counting {
				if strings.Contains(named, word) {
					t.Errorf(
						"%s carries a field %q. A bulk action records how many issues it expects "+
							"because somebody named them; an import is handed a backlog of unknown "+
							"size, and a field for the total is a promise to go and count it.",
						structure.Name(), structure.Field(index).Name,
					)
				}
			}
		}
	}
}

func TestTheImportTablesHaveNowhereToRecordATotal(t *testing.T) {
	migration := strings.ToLower(importMigration(t))

	for _, forbidden := range []string{" expected ", "expected integer", "count(", "sum("} {
		if strings.Contains(migration, forbidden) {
			t.Errorf(
				"the import migration contains %q. workspace_bulk_actions has an expected column "+
					"and a check that processed never exceeds it, which only works because a bulk "+
					"action is given its issues. Import volume is unbounded by design.",
				forbidden,
			)
		}
	}
}

func TestTheImportMigrationCanBeTakenBackOut(t *testing.T) {
	migration := importMigration(t)

	up, down, split := strings.Cut(migration, "-- +goose Down")
	if !split {
		t.Fatal("the import migration has no Down section, so it can never be rolled back")
	}

	created := regexp.MustCompile(`(?i)CREATE TABLE (\w+)`).FindAllStringSubmatch(up, -1)

	if len(created) == 0 {
		t.Fatal("the import migration creates no tables")
	}

	for _, match := range created {
		table := match[1]

		if !strings.HasPrefix(table, "workspace_import_") {
			t.Errorf(
				"the import migration creates %q. Everything this slice owns is prefixed "+
					"workspace_import_, which is what lets a guard tell import SQL from SQL that "+
					"reaches into somebody else's table.",
				table,
			)
		}

		if !strings.Contains(down, table) {
			t.Errorf("the Down section never drops %q", table)
		}
	}
}

func TestTheStatusConstraintAndTheStatusTypeCannotDrift(t *testing.T) {
	migration := importMigration(t)

	found := regexp.MustCompile(`(?s)workspace_import_runs_status_check\s+CHECK \(status IN \((.*?)\)\)`).
		FindStringSubmatch(migration)

	if len(found) != 2 {
		t.Fatal("the run table has no status check to compare against entity.ImportStatuses")
	}

	constrained := make(map[string]bool)

	for _, value := range strings.Split(found[1], ",") {
		constrained[strings.Trim(strings.TrimSpace(value), "'")] = true
	}

	for _, status := range entity.ImportStatuses() {
		if !constrained[string(status)] {
			t.Errorf(
				"entity declares the status %q but the database refuses it, so a run reaching "+
					"that state would fail to save",
				status,
			)
		}

		delete(constrained, string(status))
	}

	for leftover := range constrained {
		t.Errorf(
			"the database permits the status %q but nothing in entity declares it, so a row "+
				"could be left in a state no code knows how to move on from",
			leftover,
		)
	}
}

func TestNoLicenceStandsBetweenAWorkspaceAndItsOwnHistory(t *testing.T) {
	paths := []string{
		filepath.Join("..", "service", "imports"),
		filepath.Join("..", "repository", "imports"),
	}

	for _, target := range paths {
		for name, body := range goSourceUnder(t, target) {
			lowered := strings.ToLower(body)

			for _, word := range []string{"licensing", "unlicensed", "entity.feature"} {
				if strings.Contains(lowered, word) {
					t.Errorf(
						"%s consults the licence through %q. entity.Features() is exactly audit and "+
							"directory, and a third paid feature is a product decision rather than a "+
							"refactor — least of all one that would hold a team's own backlog hostage "+
							"on the way in.",
						name, word,
					)
				}
			}
		}
	}
}

func TestAnImportNeverHoldsItsWholeBacklogAtOnce(t *testing.T) {
	forbidden := map[string]string{
		"entity.Chunks(": "chunks a slice it already has in memory, which means it read every " +
			"record before starting. The executor walks by keyset so it never holds more than " +
			"one chunk.",
		"time.Sleep": "sleeps. A worker waiting out a source's rate limit holds an asynq slot and " +
			"burns toward its own timeout for nothing; parking the cursor and re-enqueueing with " +
			"ProcessAt frees it.",
		"service.Attachments": "reaches for attachments. An import creates none, and the workspace " +
			"storage quota would otherwise become a ceiling on how much it may carry.",
	}

	for name, body := range goSourceUnder(t, filepath.Join("..", "service", "imports")) {
		for word, because := range forbidden {
			if strings.Contains(body, word) {
				t.Errorf("%s %s", name, because)
			}
		}
	}
}

func TestNothingInThisInstanceCanActuallyImportAnything(t *testing.T) {
	found := make([]string, 0)

	for _, target := range []string{
		filepath.Join("..", "service"),
		filepath.Join("..", "service", "imports"),
		filepath.Join("..", "repository", "imports"),
		filepath.Join("..", "handler", "job"),
	} {
		for name, body := range goSourceUnder(t, target) {
			if strings.HasSuffix(name, filepath.Join("service", "imports.go")) {
				continue
			}

			if strings.Contains(body, "service.ImportFetchRequest) (service.ImportFetchPage") ||
				strings.Contains(body, "ImportFetchRequest) (ImportFetchPage") {
				found = append(found, name)
			}
		}
	}

	if len(found) > 0 {
		t.Errorf(
			"%s implements the ImportSource port. This slice ships the framework and deliberately "+
				"no adapter: a source is source-specific logic and arrives with the slice that "+
				"needs it. An adapter appearing here without that decision being retaken means "+
				"somebody has quietly shipped an integration.",
			strings.Join(found, ", "),
		)
	}
}

func TestTheLedgerRecordsWhatWasCreatedWithoutPointingAtIt(t *testing.T) {
	migration := importMigration(t)

	start := strings.Index(migration, "CREATE TABLE workspace_import_ledger")
	if start < 0 {
		t.Fatal("there is no ledger table")
	}

	end := strings.Index(migration[start:], ");")
	if end < 0 {
		t.Fatal("the ledger table definition is never closed")
	}

	ledger := migration[start : start+end]

	if strings.Contains(ledger, "created_id uuid NOT NULL REFERENCES") {
		t.Fatal(
			"the ledger's created_id has a foreign key. It points at issues, projects, labels, " +
				"states and teams by turn, so one key is impossible — and a cascade would erase " +
				"the record of a creation at the exact moment somebody deleted the thing, which " +
				"is when the report is most worth reading.",
		)
	}

	if !strings.Contains(ledger, "reference") {
		t.Error(
			"the ledger does not denormalise a human reference. A report read after its issues " +
				"were purged would be a list of identifiers pointing at nothing.",
		)
	}
}
