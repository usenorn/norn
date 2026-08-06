package imports_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func importSources(t *testing.T) map[string]string {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list import service sources: %v", err)
	}

	sources := map[string]string{}

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "mock_") {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		sources[name] = string(body)
	}

	if len(sources) == 0 {
		t.Fatal("this guard found no import service sources, so it is protecting nothing")
	}

	return sources
}

func bodyOf(t *testing.T, name, signature string) string {
	t.Helper()

	body := importSources(t)[name]

	at := strings.Index(body, signature)
	if at < 0 {
		t.Fatalf("%s no longer holds %s, so this guard is protecting nothing", name, signature)
	}

	body = body[at+len(signature):]

	if end := strings.Index(body, "\nfunc "); end >= 0 {
		body = body[:end]
	}

	return body
}

func caseLabelFor(resource entity.ImportResource) string {
	label := "case entity.Import"

	for _, part := range strings.Split(string(resource), "_") {
		label += strings.ToUpper(part[:1]) + part[1:]
	}

	return label + ":"
}

func TestAResourceNoRevertNamesIsSilentlyUnrevertible(t *testing.T) {
	body := bodyOf(t, "revert.go", "func (s *importsService) unwind(")

	for _, resource := range entity.ImportPhases() {
		if !strings.Contains(body, caseLabelFor(resource)) {
			t.Errorf(
				"unwind has no arm for %q, so its default arm answers retained and no error. The "+
					"revert reports every row of that resource as left standing on purpose, the run "+
					"settles as reverted, and nothing anywhere says the workspace still holds what "+
					"the import made.",
				resource,
			)
		}
	}
}

func TestAResourceNoPreviewNamesIsSilentlyInvisible(t *testing.T) {
	body := bodyOf(t, "preview.go", "func (w *previewWalk) consider(")

	for _, resource := range entity.ImportPhases() {
		if !strings.Contains(body, caseLabelFor(resource)) {
			t.Errorf(
				"consider has no arm for %q, so its default arm returns nil. The preview shows "+
					"nothing of that resource, the requester accepts an import smaller than the one "+
					"that runs, and the first they hear of it is rows appearing that no page ever "+
					"named.",
				resource,
			)
		}
	}
}

func TestApplyingAChunkOpensExactlyOneTransaction(t *testing.T) {
	sources := importSources(t)

	body, present := sources["apply.go"]
	if !present {
		t.Fatal("apply.go is gone, so the chunk boundary this guard protects has moved somewhere unguarded")
	}

	if opened := strings.Count(body, "WithTx("); opened != 1 {
		t.Fatalf(
			"apply.go opens %d transactions. The creates, the ledger, the record settlement, the "+
				"report and the progress advance are one chunk: split across two transactions, a "+
				"worker dying between them leaves rows created that nothing in the ledger admits to.",
			opened,
		)
	}
}

func TestPreviewingWritesNothing(t *testing.T) {
	body := importSources(t)["preview.go"]

	if body == "" {
		t.Fatal("preview.go is gone, so nothing is checking that a preview is free to run")
	}

	for _, write := range []string{"WithTx(", ".Create(", ".Post(", ".Record(", ".Settle(", ".Save("} {
		if strings.Contains(body, write) {
			t.Errorf(
				"preview.go calls %s. A preview is what the requester is shown before deciding, "+
					"and Execute recomputes it to compare digests — a preview that writes would "+
					"change the workspace every time somebody looked at it.",
				write,
			)
		}
	}
}

func TestOnlyTheApplyPathCanAttributeWorkToSomebodyElse(t *testing.T) {
	for name, body := range importSources(t) {
		if name == "apply.go" {
			continue
		}

		for _, attributing := range []string{"entity.NewImportOrigin", "Origin:"} {
			if strings.Contains(body, attributing) {
				t.Errorf(
					"%s uses %s. Back-dating a row and crediting it to another account is the one "+
						"power an import has that no request does; keeping it in a single file is "+
						"what makes it reviewable.",
					name, attributing,
				)
			}
		}
	}
}
