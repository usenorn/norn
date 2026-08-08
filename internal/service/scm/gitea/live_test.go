package gitea_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/service/scm/gitea"
)

type live struct {
	target  entity.SCMTarget
	adapter *gitea.Forge
}

func liveTarget(t *testing.T) live {
	t.Helper()

	address := strings.TrimSpace(os.Getenv("NORN_TEST_GITEA_URL"))
	if address == "" {
		t.Skip("NORN_TEST_GITEA_URL is unset, so there is no instance to reach")
	}

	token := strings.TrimSpace(os.Getenv("NORN_TEST_GITEA_TOKEN"))
	repository := strings.TrimSpace(os.Getenv("NORN_TEST_GITEA_REPOSITORY"))

	if token == "" || repository == "" {
		t.Fatal("NORN_TEST_GITEA_TOKEN and NORN_TEST_GITEA_REPOSITORY are both required")
	}

	authority := ""

	if path := strings.TrimSpace(os.Getenv("NORN_TEST_GITEA_CA")); path != "" {
		pem, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read the certificate authority at %s: %v", path, err)
		}

		authority = string(pem)
	}

	cfg := config.SourceControl{
		PageSize:        20,
		RequestTimeout:  20 * time.Second,
		DialTimeout:     5 * time.Second,
		MaxResponseSize: 8 << 20,
	}

	client, err := forge.New(cfg)
	if err != nil {
		t.Fatalf("build a forge client: %v", err)
	}

	return live{
		adapter: gitea.New(client, cfg),
		target: entity.SCMTarget{
			Provider:   entity.SCMProviderGitea,
			BaseURL:    address,
			Repository: repository,
			Token:      token,
			Trust: entity.SCMTrust{
				AllowPrivateAddress: true,
				CACertificate:       authority,
			},
		},
	}
}

func TestAnInstanceOnAPrivateAddressIsRefusedWithoutTheException(t *testing.T) {
	held := liveTarget(t)

	held.target.Trust.AllowPrivateAddress = false

	_, err := held.adapter.Identity(context.Background(), held.target)

	var refused entity.SCMDestinationRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf(
			"reaching a private address without the exception returned %v; the exception is the "+
				"only thing that opens it, and a guard that lets everything through protects nothing",
			err,
		)
	}
}

func TestAPrivateAuthorityIsRefusedUntilItIsSupplied(t *testing.T) {
	held := liveTarget(t)

	if held.target.Trust.CACertificate == "" {
		t.Skip("NORN_TEST_GITEA_CA is unset, so the instance is not on a private authority")
	}

	held.target.Trust.CACertificate = ""

	if _, err := held.adapter.Identity(context.Background(), held.target); err == nil {
		t.Fatal(
			"a certificate no public authority signed was accepted without the authority being " +
				"supplied, which means the supplied one is not what made the call work",
		)
	}
}

func TestTheAdapterReadsARealInstance(t *testing.T) {
	held := liveTarget(t)
	ctx := context.Background()

	login, err := held.adapter.Identity(ctx, held.target)
	if err != nil {
		t.Fatalf("Identity: %v", err)
	}

	if login == "" {
		t.Fatal("Identity returned nobody, so no connection could name who it acts as")
	}

	found, err := held.adapter.Repository(ctx, held.target)
	if err != nil {
		t.Fatalf("Repository: %v", err)
	}

	if !strings.EqualFold(found.FullName, held.target.Repository) {
		t.Fatalf("Repository returned %q, want %q", found.FullName, held.target.Repository)
	}

	if found.DefaultBranch == "" {
		t.Error("the repository reported no default branch")
	}

	page, err := held.adapter.Changes(ctx, held.target, time.Time{}, "")
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	if len(page.Changes) == 0 {
		t.Fatal("Changes found nothing; the fixture repository is expected to hold one open change")
	}

	change := page.Changes[0]

	if change.Number <= 0 || change.ExternalID == "" {
		t.Fatalf("a change came back without a number or an id: %+v", change)
	}

	if change.HeadBranch == "" || change.Title == "" {
		t.Errorf("a change came back with nothing to render: %+v", change)
	}

	if !change.ReviewsMoved {
		t.Error("a change read from the listing must ask for its reviews to be read")
	}

	paths, err := held.adapter.ChangedPaths(ctx, held.target, change.Number)
	if err != nil {
		t.Fatalf("ChangedPaths: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal(
			"ChangedPaths found no files. Routing decides which team a change belongs to from " +
				"exactly this, so an empty answer sends every change to the default route",
		)
	}

	if _, err := held.adapter.Reviews(ctx, held.target, change.Number); err != nil {
		t.Fatalf("Reviews: %v", err)
	}
}

func TestWhatThisInstanceCannotDoIsRefusedByName(t *testing.T) {
	held := liveTarget(t)

	_, err := held.adapter.Deployments(context.Background(), held.target, 10)

	if !errors.Is(err, entity.ErrSCMCapabilityUnsupported) {
		t.Fatalf(
			"Deployments returned %v; a capability the target does not have must be refused by "+
				"name rather than answered with an empty list that reads as nothing deployed",
			err,
		)
	}
}
