package gitea_test

import (
	"context"
	"errors"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/service/scm/gitea"
)

func adapter(t *testing.T) *gitea.Forge {
	t.Helper()

	client, err := forge.New(config.SourceControl{PageSize: 20})
	if err != nil {
		t.Fatalf("build a forge client: %v", err)
	}

	return gitea.New(client, config.SourceControl{PageSize: 20})
}

func TestATargetWithNoDeploymentsSaysSoRatherThanReturningNothing(t *testing.T) {
	found := adapter(t).Capabilities()

	if found.Has(entity.CapabilityDeployments) {
		t.Fatal(
			"Gitea claimed deployments. It has no such api, so the screen would offer an " +
				"environment that never appears and read as broken rather than unsupported",
		)
	}

	if !found.Has(entity.CapabilityReleases) {
		t.Error("Gitea does serve releases and must claim them")
	}

	_, err := adapter(t).Deployments(context.Background(), entity.SCMTarget{}, 10)

	if !errors.Is(err, entity.ErrSCMCapabilityUnsupported) {
		t.Fatalf(
			"Deployments returned %v; a capability the target does not have must be refused by "+
				"name, not answered with an empty list that reads as \"nothing deployed\"",
			err,
		)
	}
}
