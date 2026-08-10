package entity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

type commitment struct {
	name  string
	words []string
	built bool
}

func neverGated() []commitment {
	return []commitment{
		{name: "the member count", words: []string{"member", "membership", "people", "headcount"}, built: true},
		{name: "the issue count", words: []string{"issue"}, built: true},
		{name: "the project count", words: []string{"project"}, built: true},
		{name: "the team count", words: []string{"team"}, built: true},
		{name: "the agent count", words: []string{"agent"}, built: true},
		{name: "single sign-on by SAML", words: []string{"saml"}, built: true},
		{name: "single sign-on by OIDC", words: []string{"oidc", "openid"}, built: true},
		{name: "the API", words: []string{"api", "token"}, built: true},
		{name: "data export", words: []string{"export"}, built: true},
		{name: "the MCP server", words: []string{"mcp"}, built: true},
		{name: "webhooks", words: []string{"webhook"}, built: true},
	}
}

func TestNoneOfTheFreeForeverFeaturesCanBecomeAFeatureFlag(t *testing.T) {
	for _, promised := range neverGated() {
		for _, feature := range entity.Features() {
			named := strings.ToLower(string(feature))

			for _, word := range promised.words {
				if strings.Contains(named, word) {
					t.Errorf(
						"the licence names a feature %q, which reads as %s. That is free forever on "+
							"every tier including self-hosted with no licence, and this is a product "+
							"commitment rather than an implementation detail.",
						feature, promised.name,
					)
				}
			}
		}
	}
}

func TestTheUnbuiltCommitmentsAreNamedSoTheyCannotArrivePriced(t *testing.T) {
	unbuilt := make([]string, 0, 2)

	for _, promised := range neverGated() {
		if !promised.built {
			unbuilt = append(unbuilt, promised.name)
		}
	}

	if len(unbuilt) == 0 {
		return
	}

	t.Logf(
		"these are promised free but do not exist yet, so nothing here can exercise them: %s. "+
			"They are named in the commitment list above so that whichever slice builds them "+
			"cannot quietly gate them on arrival.",
		strings.Join(unbuilt, ", "),
	)
}

func TestTheGatedSurfaceIsExactlyTheTwoFeaturesWeSaidItWas(t *testing.T) {
	wanted := map[entity.Feature]bool{
		entity.FeatureAudit:     true,
		entity.FeatureDirectory: true,
	}

	for _, feature := range entity.Features() {
		if !wanted[feature] {
			t.Errorf(
				"the licence gates %q. At this stage exactly two things are paid — the audit log "+
					"and directory synchronization — and adding a third is a product decision, not "+
					"a refactor.",
				feature,
			)
		}

		delete(wanted, feature)
	}

	for feature := range wanted {
		t.Errorf("the licence no longer gates %q, which was a paid feature", feature)
	}
}

func TestNothingInTheLicenceCountsAnything(t *testing.T) {
	forbidden := []string{"count", "seat", "quota", "usage", "meter", "bill", "tier", "plan"}

	paths := []string{"licence.go", filepath.Join("..", "service", "licensing")}

	for _, target := range paths {
		for name, body := range goSourceUnder(t, target) {
			lowered := strings.ToLower(body)

			for _, word := range forbidden {
				if strings.Contains(lowered, word) {
					t.Errorf(
						"%s mentions %q. Provisioning any number of members, issues, projects, "+
							"teams or agents is free on every tier, so the licensing path must "+
							"never learn to count.",
						name, word,
					)
				}
			}
		}
	}
}

func TestTheThingsPromisedFreeConsultNoLicence(t *testing.T) {
	paths := []string{
		filepath.Join("..", "handler", "mcpserver"),
		filepath.Join("..", "service", "agent"),
		filepath.Join("..", "service", "ssoconnection"),
		filepath.Join("..", "service", "apitoken"),
		filepath.Join("..", "service", "webhook"),
	}

	for _, target := range paths {
		for name, body := range goSourceUnder(t, target) {
			lowered := strings.ToLower(body)

			for _, word := range []string{"licensing", "unlicensed", "entity.feature"} {
				if strings.Contains(lowered, word) {
					t.Errorf(
						"%s consults the licence through %q. The MCP server, single sign-on and "+
							"the API are free forever on every tier, so no licence may stand "+
							"between a self-hoster and any of them.",
						name, word,
					)
				}
			}
		}
	}
}

func TestLicenceValidationCannotReachTheNetwork(t *testing.T) {
	reaching := []string{"net/http", "\"net\"", "url.", "dial", "http.get", "http.post", "http.client"}

	paths := []string{
		"licence.go",
		filepath.Join("..", "pkg", "licence"),
		filepath.Join("..", "service", "licensing"),
	}

	for _, target := range paths {
		for name, body := range goSourceUnder(t, target) {
			lowered := strings.ToLower(body)

			for _, word := range reaching {
				if strings.Contains(lowered, word) {
					t.Errorf(
						"%s reaches for %q. A self-hosted instance must validate its licence with "+
							"no network at all, and a licence check must never carry workspace "+
							"contents or usage detail off the box.",
						name, word,
					)
				}
			}
		}
	}
}

func TestAnInstanceWithNoLicenceIsUnremarkable(t *testing.T) {
	absent := entity.Licence{}
	now := time.Now()
	grace := 30 * 24 * time.Hour

	if status := absent.Status(now, grace); status != entity.LicenceAbsent {
		t.Errorf("status with no licence = %q, want absent", status)
	}

	if absent.Present() {
		t.Error("a zero licence reports itself as present")
	}

	if !absent.GraceEndsAt(grace).IsZero() {
		t.Error("a zero licence has a grace period, which would imply it once ran out")
	}

	for _, feature := range entity.Features() {
		if absent.Permits(now, grace, feature) {
			t.Errorf("an absent licence permits %q", feature)
		}

		if absent.Enables(feature) {
			t.Errorf("an absent licence enables %q", feature)
		}
	}
}

func goSourceUnder(t *testing.T, target string) map[string]string {
	t.Helper()

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat %s: %v", target, err)
	}

	sources := make(map[string]string)

	if !info.IsDir() {
		body, err := os.ReadFile(filepath.Clean(target))
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}

		sources[target] = string(body)

		return sources
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}

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

		sources[filepath.Join(target, name)] = string(body)
	}

	if len(sources) == 0 {
		t.Fatalf("%s has no source to guard", target)
	}

	return sources
}
