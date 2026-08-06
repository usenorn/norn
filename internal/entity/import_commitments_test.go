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

// importSlice is every directory the import slice owns, adapters included. goSourceUnder does
// not descend, so an adapter in a sub-package is outside every guard here until it is named.
func importSlice() []string {
	return []string{
		filepath.Join("..", "service", "imports"),
		filepath.Join("..", "service", "imports", "linear"),
		filepath.Join("..", "service", "imports", "csvfile"),
	}
}

func importSource(t *testing.T) map[string]string {
	t.Helper()

	held := make(map[string]string)

	for _, target := range importSlice() {
		for name, body := range goSourceUnder(t, target) {
			held[name] = body
		}
	}

	return held
}

func TestNoLicenceStandsBetweenAWorkspaceAndItsOwnHistory(t *testing.T) {
	sources := importSource(t)

	for name, body := range goSourceUnder(t, filepath.Join("..", "repository", "imports")) {
		sources[name] = body
	}

	for name, body := range sources {
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

func TestEveryConcessionGrantedToAnImportTestsAttributionRatherThanPresence(t *testing.T) {
	granting := []string{
		filepath.Join("..", "service", "issue"),
		filepath.Join("..", "service", "cycle"),
		filepath.Join("..", "service", "project"),
		filepath.Join("..", "service", "issuecomment"),
		filepath.Join("..", "service", "label"),
		filepath.Join("..", "service", "workflowstate"),
		filepath.Join("..", "service", "attachment"),
	}

	for _, target := range granting {
		for name, body := range goSourceUnder(t, target) {
			for _, weak := range []string{"Origin == nil", "Origin != nil"} {
				if strings.Contains(body, weak) {
					t.Errorf(
						"%s decides something on %q. An ImportOrigin decoded from a request body is "+
							"non-nil and inert, so testing the pointer for presence hands the "+
							"concession to any caller who names the field. entity.OriginAttributed "+
							"reads the unexported flag that only the constructor can set, which is "+
							"the whole reason the flag is unexported.",
						name, weak,
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
	}

	for name, body := range importSource(t) {
		for word, because := range forbidden {
			if strings.Contains(body, word) {
				t.Errorf("%s %s", name, because)
			}
		}
	}
}

func TestRunningOutOfStorageCostsTheFileAndNeverTheBacklog(t *testing.T) {
	refusals := map[error]string{
		entity.ErrStorageExhausted: "A workspace that fills up mid-import has a ceiling on bytes " +
			"and none on rows. Recorded as anything but skipped, the first file over the line " +
			"settles its record failed and, once the run has retried its way to the attempt " +
			"limit, abandons every issue behind it — years of a team's work lost to one screenshot.",
		entity.ErrAttachmentTooLarge: "One file larger than this instance accepts says nothing " +
			"about the backlog around it.",
	}

	for refusal, because := range refusals {
		if outcome := entity.OutcomeForImport(refusal); outcome != entity.ImportOutcomeSkipped {
			t.Errorf("OutcomeForImport(%v) = %q, want skipped. %s", refusal, outcome, because)
		}
	}

	for name, body := range importSource(t) {
		for _, named := range []string{"ErrStorageExhausted", "StorageExhaustedError"} {
			if strings.Contains(body, named) {
				t.Errorf(
					"%s names %s. The apply path adopts a stored object and hands whatever comes "+
						"back to OutcomeForImport; code that recognises a full workspace by name is "+
						"code that has an opinion about it, and the only opinions available at that "+
						"point are to abandon the chunk or to swallow the refusal silently.",
					name, named,
				)
			}
		}
	}
}

func TestTheOnlyThingHereThatCanImportAnythingIsTheAdapterThisInstanceDeclared(t *testing.T) {
	declared := map[string]bool{
		filepath.Join("..", "service", "imports", "linear", "linear.go"):   true,
		filepath.Join("..", "service", "imports", "csvfile", "csvfile.go"): true,
	}

	port := regexp.MustCompile(
		`ImportFetchRequest,?\s*\)\s*\(\s*(?:service\.)?ImportFetchPage`,
	)

	found := make([]string, 0)
	standing := make(map[string]bool, len(declared))

	for _, target := range []string{
		filepath.Join("..", "service"),
		filepath.Join("..", "service", "imports"),
		filepath.Join("..", "service", "imports", "linear"),
		filepath.Join("..", "service", "imports", "csvfile"),
		filepath.Join("..", "repository", "imports"),
		filepath.Join("..", "handler", "job"),
	} {
		for name, body := range goSourceUnder(t, target) {
			if strings.HasSuffix(name, filepath.Join("service", "imports.go")) {
				continue
			}

			if !port.MatchString(body) {
				continue
			}

			if declared[name] {
				standing[name] = true

				continue
			}

			found = append(found, name)
		}
	}

	if len(found) > 0 {
		t.Errorf(
			"%s implements the ImportSource port and nothing declared it. Every adapter costs "+
				"this instance the same three things: a credential it now has to store and read "+
				"back, a rate-limit contract it has to honour by parking a cursor rather than "+
				"sleeping on a worker slot, and a mapping surface that every source concept has "+
				"to be answered on. Shipping one is a decision taken in the open — added to the "+
				"list below and to the registry — never a file that appeared.",
			strings.Join(found, ", "),
		)
	}

	for name := range declared {
		if !standing[name] {
			t.Errorf(
				"%s is named here as an adapter and implements nothing. A list that outlives what "+
					"it allows stops being a decision and starts being a hole.",
				name,
			)
		}
	}
}

func TestARunSaysAKeyIsStoredWithoutEverCarryingIt(t *testing.T) {
	run := reflect.TypeOf(entity.ImportRun{})

	told := false

	for index := range run.NumField() {
		field := run.Field(index)

		if !strings.Contains(strings.ToLower(field.Name), "secret") {
			continue
		}

		if field.Name == "SourceSecretSet" && field.Type.Kind() == reflect.Bool {
			told = true

			continue
		}

		t.Errorf(
			"ImportRun carries %s %s. The run is read by every list, report and rescue sweep and "+
				"is the shape a wizard is handed back; a source key on it would travel to all of "+
				"them, and the one caller that needs the key reads it through the staging accessor "+
				"instead.",
			field.Name, field.Type,
		)
	}

	if !told {
		t.Error(
			"nothing on ImportRun says whether a key is stored. Without it a wizard cannot tell " +
				"a run that has been given its credentials from one that never was, and would have " +
				"to ask for the key again on every visit.",
		)
	}
}

func TestASourceIsAskedWhatItHoldsAndNeverForRowsInTheSameBreath(t *testing.T) {
	asked := 0

	for name, body := range importSource(t) {
		if !strings.Contains(body, "Probe") {
			continue
		}

		asked++

		for _, staging := range []string{"records.Stage", "ledger.Record"} {
			if strings.Contains(body, staging) {
				t.Errorf(
					"%s both asks a source what it holds and calls %s. Probing is the one place a "+
						"source is called outside the staging job, and it is bounded because it "+
						"answers with a catalogue: a probe that could also stage rows would be a "+
						"second, unleased import path with no cursor and no ledger behind it.",
					name, staging,
				)
			}
		}
	}

	if asked == 0 {
		t.Error(
			"nothing asks a source what it holds, so this guard is protecting nothing. Choosing " +
				"teams or reading a header row needs an answer before staging has anything to show.",
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

// importedResources are the repositories the apply path writes through. Every collision one of
// them translates is a collision the import produces on ordinary data, and the transaction the
// chunk runs in is aborted by the statement that raised it either way.
func importedResources() []string {
	return []string{
		"team", "workflowstate", "label", "labelgroup", "project", "issue", "issuerelation", "cycle",
	}
}

func collisionSentinels(t *testing.T) map[string]string {
	t.Helper()

	named := regexp.MustCompile(`entity\.(Err\w+)`)
	found := map[string]string{}

	for _, resource := range importedResources() {
		pattern := filepath.Join("..", "repository", resource, "postgres*.go")

		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}

		for _, match := range matches {
			body, err := os.ReadFile(filepath.Clean(match))
			if err != nil {
				t.Fatalf("read %s: %v", match, err)
			}

			lines := strings.Split(string(body), "\n")

			for at, line := range lines {
				if !strings.Contains(line, "uniqueViolation") && !strings.Contains(line, "exclusionViolation") {
					continue
				}

				for _, ahead := range lines[at:min(at+4, len(lines))] {
					if sentinel := named.FindStringSubmatch(ahead); sentinel != nil {
						found[sentinel[1]] = match

						break
					}
				}
			}
		}
	}

	return found
}

// collisionsNoImportCanRaise are translated by a repository the apply path also writes
// through, on a row it never writes. They are named rather than filtered out by file so that a
// new one has to be looked at rather than quietly inherited.
func collisionsNoImportCanRaise() map[string]string {
	return map[string]string{
		"ErrProjectMemberExists": "an import creates a project and never puts anybody on it",
	}
}

func TestEveryCollisionARepositoryTranslatesCostsTheRowRatherThanTheRun(t *testing.T) {
	sentinels := map[string]error{
		"ErrTeamKeyTaken":           entity.ErrTeamKeyTaken,
		"ErrWorkflowStateNameTaken": entity.ErrWorkflowStateNameTaken,
		"ErrLabelNameTaken":         entity.ErrLabelNameTaken,
		"ErrLabelGroupExclusive":    entity.ErrLabelGroupExclusive,
		"ErrLabelGroupNameTaken":    entity.ErrLabelGroupNameTaken,
		"ErrProjectSlugTaken":       entity.ErrProjectSlugTaken,
		"ErrIssueReferenceTaken":    entity.ErrIssueReferenceTaken,
		"ErrIssueRelationExists":    entity.ErrIssueRelationExists,
		"ErrCycleOverlaps":          entity.ErrCycleOverlaps,
	}

	translated := collisionSentinels(t)

	if len(translated) == 0 {
		t.Fatal("no repository the import writes through translates a constraint violation, so this guard is protecting nothing")
	}

	for name, where := range translated {
		if _, beyond := collisionsNoImportCanRaise()[name]; beyond {
			continue
		}

		sentinel, known := sentinels[name]

		if !known {
			t.Errorf(
				"%s translates a constraint violation into %s and nothing here has an opinion "+
					"about it. Postgres aborts the whole transaction on the statement that raised "+
					"it, so the apply path either recognises it as a row the source legitimately "+
					"produced or spends the run's attempts retrying into the same collision.",
				where, name,
			)

			continue
		}

		if outcome := entity.OutcomeForImport(sentinel); outcome != entity.ImportOutcomeSkipped {
			t.Errorf(
				"%s reaches OutcomeForImport as %q, want skipped. %s raises it whenever the "+
					"workspace already holds the row being imported, which is every re-run and "+
					"every import into a workspace anybody has already used.",
				name, outcome, where,
			)
		}
	}
}
