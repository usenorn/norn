package scm_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

// A repository somebody set one way must never touch the other. This pins the decision
// itself rather than the plumbing around it, because the plumbing is what changes.
func TestADirectionIsRespectedInBothHalvesOfTheSync(t *testing.T) {
	cases := []struct {
		name          string
		repository    entity.SCMRepository
		pulls, pushes bool
	}{
		{
			name:       "read-only takes issues but writes nothing",
			repository: entity.SCMRepository{SyncDirection: entity.MirrorInbound},
			pulls:      true,
			pushes:     false,
		},
		{
			name:       "write-only pushes but brings nothing across",
			repository: entity.SCMRepository{SyncDirection: entity.MirrorOutbound},
			pulls:      false,
			pushes:     true,
		},
		{
			name:       "a repository connected before direction existed keeps working both ways",
			repository: entity.SCMRepository{},
			pulls:      true,
			pushes:     true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			direction := testCase.repository.Direction()

			if direction.Pulls() != testCase.pulls {
				t.Errorf("Pulls = %t, want %t", direction.Pulls(), testCase.pulls)
			}

			if direction.Pushes() != testCase.pushes {
				t.Errorf("Pushes = %t, want %t", direction.Pushes(), testCase.pushes)
			}
		})
	}
}
