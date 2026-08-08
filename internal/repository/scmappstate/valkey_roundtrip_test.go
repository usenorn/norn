package scmappstate

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

func store(t *testing.T) repository.SCMAppState {
	t.Helper()

	addr := strings.TrimSpace(os.Getenv("NORN_VALKEY_ADDR"))
	if addr == "" {
		t.Skip("NORN_VALKEY_ADDR is unset, so there is nowhere to write to")
	}

	client, cleanup, err := valkey.New(config.Valkey{
		Addr:         addr,
		Password:     os.Getenv("NORN_VALKEY_PASSWORD"),
		PoolSize:     4,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("reach valkey: %v", err)
	}

	t.Cleanup(cleanup)

	return New(client, config.SourceControl{AppStateTTL: time.Minute})
}

func TestEveryFieldOfAnExchangeSurvivesBeingStored(t *testing.T) {
	held := store(t)
	ctx := context.Background()

	written := entity.SCMAppState{
		Purpose:       entity.SCMAppRegister,
		Provider:      entity.SCMProviderGitHub,
		WorkspaceID:   uuid.New(),
		WorkspaceSlug: "northwind",
		AccountID:     uuid.New(),
		Organization:  "flagroll",
		Installations: []entity.SCMInstallation{
			{ExternalID: "884411", AccountLogin: "flagroll", AccountKind: "organization"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	state := "roundtrip-" + written.WorkspaceID.String()

	if err := held.Put(ctx, state, written); err != nil {
		t.Fatalf("Put: %v", err)
	}

	read, err := held.Read(ctx, state)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if read.WorkspaceSlug != written.WorkspaceSlug {
		t.Errorf(
			"the workspace came back %q, want %q. It is the only thing that says where to send "+
				"the browser back to, and without it every exchange ends on a page that does not "+
				"exist",
			read.WorkspaceSlug, written.WorkspaceSlug,
		)
	}

	if read.Purpose != written.Purpose || read.Provider != written.Provider {
		t.Errorf("purpose or provider came back as %q/%q", read.Purpose, read.Provider)
	}

	if read.WorkspaceID != written.WorkspaceID || read.AccountID != written.AccountID {
		t.Errorf("the workspace or account identifier did not survive: %+v", read)
	}

	if read.Organization != written.Organization {
		t.Errorf("the organization came back %q, want %q", read.Organization, written.Organization)
	}

	if len(read.Installations) != 1 || read.Installations[0].ExternalID != "884411" {
		t.Errorf("the installations did not survive: %+v", read.Installations)
	}

	if !read.CreatedAt.Equal(written.CreatedAt) {
		t.Errorf("the time it was made came back %s, want %s", read.CreatedAt, written.CreatedAt)
	}
}

func TestReadingLeavesTheExchangeAndTakingSpendsIt(t *testing.T) {
	held := store(t)
	ctx := context.Background()

	state := "roundtrip-spend-" + uuid.NewString()

	if err := held.Put(ctx, state, entity.SCMAppState{
		Purpose:       entity.SCMAppChosen,
		Provider:      entity.SCMProviderGitHub,
		WorkspaceSlug: "northwind",
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := held.Read(ctx, state); err != nil {
		t.Fatalf("the first read: %v", err)
	}

	if _, err := held.Read(ctx, state); err != nil {
		t.Fatalf(
			"the second read: %v. Listing the installations must not spend the handle, or the "+
				"screen that lists them could never connect one",
			err,
		)
	}

	if _, err := held.Take(ctx, state); err != nil {
		t.Fatalf("Take: %v", err)
	}

	if _, err := held.Take(ctx, state); err == nil {
		t.Fatal(
			"an exchange was spent twice. A one-shot token that survives its use is not one-shot, " +
				"and a replayed redirect would be honoured",
		)
	}
}
