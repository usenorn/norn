package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAConnectionThroughAnApplicationDeliversCentrally(t *testing.T) {
	cases := []struct {
		name    string
		kind    entity.SCMAuthKind
		central bool
	}{
		{name: "an installation", kind: entity.SCMAuthApp, central: true},
		{name: "a pasted token", kind: entity.SCMAuthToken, central: false},
		{name: "an unset kind", kind: "", central: false},
	}

	for _, held := range cases {
		t.Run(held.name, func(t *testing.T) {
			connection := entity.SCMConnection{AuthKind: held.kind}

			if connection.DeliversCentrally() != held.central {
				t.Fatalf(
					"DeliversCentrally() = %t for %s, want %t. It decides whether a repository is "+
						"given a webhook of its own: wrong one way the sweep keeps installing a hook "+
						"the application already has and the screen calls a healthy repository broken, "+
						"wrong the other way nothing is ever installed and no delivery arrives",
					connection.DeliversCentrally(), held.name, held.central,
				)
			}
		})
	}
}
