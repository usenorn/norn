package agent_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestNoLayerExposesAnAgentCountingOperation(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "usage", "meter", "bill"}

	surfaces := map[string]reflect.Type{
		"repository.Agent":         reflect.TypeOf((*repository.Agent)(nil)).Elem(),
		"repository.AgentThrottle": reflect.TypeOf((*repository.AgentThrottle)(nil)).Elem(),
		"repository.AgentProposal": reflect.TypeOf((*repository.AgentProposal)(nil)).Elem(),
		"service.Agents":           reflect.TypeOf((*service.Agents)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ReplaceAll(strings.ToLower(surface.Method(i).Name), "account", "")

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf(
						"%s exposes %q. Nothing about agents may be counted or billed, and an "+
							"operation that totals them is how that starts.",
						name, surface.Method(i).Name,
					)
				}
			}
		}
	}
}

func TestNothingAboutAgentsIsPricedOrLicenced(t *testing.T) {
	words := []string{
		"licence", "license", "tier", "entitlement", "quota",
		"billing", "seat", "upgrade", "paywall", "premium", "plan",
	}

	files := []string{
		"agents.go",
		"approvals.go",
		"../agents.go",
		"../agents_input.go",
		"../../entity/agent.go",
		"../../entity/agent_approval.go",
		"../../repository/agent.go",
		"../../repository/agent_throttle.go",
		"../agenthold/hold.go",
	}

	for _, name := range files {
		source, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		lowered := strings.ToLower(string(source))

		for _, word := range words {
			if strings.Contains(lowered, word) {
				t.Errorf(
					"%s mentions %q. Agents are deliberately not a priced axis, which is the "+
						"opposite of how this is sold elsewhere; the moment one of these words "+
						"appears the feature has started becoming one.",
					name, word,
				)
			}
		}
	}
}

func TestAnAgentIsNotAPersonInTheMembersListing(t *testing.T) {
	member := reflect.TypeOf(entity.WorkspaceMember{})

	if _, ok := member.FieldByName("AccountKind"); !ok {
		t.Fatal(
			"a workspace member does not carry its account kind, so an agent is indistinguishable " +
				"from a person in every listing and picker that reads it — and would be swept " +
				"into any head count added later.",
		)
	}
}
