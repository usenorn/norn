package dashboard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func TestAgentAuthorityAlwaysSerializesItsCollectionsAsArrays(t *testing.T) {
	dto := workspaceAgentDTO(service.OwnedAgent{Authority: service.AgentAuthority{AllTeams: true}})

	if dto.Authority.Scopes == nil {
		t.Fatal("authority scopes are nil, want an empty array")
	}

	if dto.Authority.TeamIds == nil {
		t.Fatal("authority teamIds are nil, want an empty array")
	}

	encoded, err := json.Marshal(dto.Authority)
	if err != nil {
		t.Fatalf("marshal authority: %v", err)
	}

	if strings.Contains(string(encoded), "null") {
		t.Fatalf("authority JSON = %s, want arrays", encoded)
	}
}

func TestAgentLifecycleConflictsUseTypedProblemCodes(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code api.AgentUnusableProblemCode
	}{
		{
			name: "already active",
			err:  entity.ErrAgentActive,
			code: api.AgentUnusableProblemCodeAgentActive,
		},
		{
			name: "authority missing",
			err:  entity.ErrAgentAuthorityMissing,
			code: api.AgentUnusableProblemCodeAgentAuthorityMissing,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			problem, ok := problemFor(test.err)
			if !ok {
				t.Fatal("lifecycle conflict was not mapped")
			}

			body, ok := problem.body.(api.AgentUnusableProblem)
			if !ok {
				t.Fatalf("problem body = %T, want AgentUnusableProblem", problem.body)
			}

			if body.Code != test.code {
				t.Fatalf("problem code = %q, want %q", body.Code, test.code)
			}
		})
	}
}
