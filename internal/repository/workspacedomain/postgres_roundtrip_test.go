package workspacedomain

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errRollback = errors.New("roll the fixture back")

func reach(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no schema to write to")
	}

	db, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        8,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("reach postgres: %v", err)
	}

	t.Cleanup(cleanup)

	return db
}

func fixtureWorkspace(ctx context.Context, t *testing.T, db *postgres.Client) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	model := &dbpostgres.Workspace{
		ID:           id.String(),
		Slug:         "domain-" + id.String()[:8],
		Name:         "Domain fixture",
		Status:       string(entity.WorkspaceStatusActive),
		Timezone:     "UTC",
		WeekStartsOn: string(entity.WeekDayMonday),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := model.Insert(ctx, db.Querier(ctx), boil.Infer()); err != nil {
		t.Fatalf("insert workspace fixture: %v", err)
	}

	return id
}

func claim(workspaceID uuid.UUID, domain string) entity.WorkspaceDomain {
	return entity.WorkspaceDomain{ID: uuid.New(), WorkspaceID: workspaceID, Domain: domain, Token: "token-" + domain}
}

func TestOnlyVerifiedClaimsRouteADomainAndSeveralWorkspacesMayHoldIt(t *testing.T) {
	db := reach(t)
	domains := New(db)
	domain := "rt-" + uuid.NewString()[:8] + ".example"

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		first := fixtureWorkspace(ctx, t, db)
		second := fixtureWorkspace(ctx, t, db)
		pending := fixtureWorkspace(ctx, t, db)

		firstClaim, err := domains.Add(ctx, claim(first, domain))
		if err != nil {
			return err
		}

		secondClaim, err := domains.Add(ctx, claim(second, domain))
		if err != nil {
			return err
		}

		if _, err := domains.Add(ctx, claim(pending, domain)); err != nil {
			return err
		}

		routed, err := domains.VerifiedWorkspaceIDs(ctx, domain)
		if err != nil {
			return err
		}

		if len(routed) != 0 {
			t.Errorf("unverified claims routed %v", routed)
		}

		now := time.Now().UTC()

		checked, err := domains.MarkChecked(ctx, first, firstClaim.ID, false, now)
		if err != nil {
			return err
		}

		if checked.Verified() || checked.CheckedAt == nil {
			t.Errorf("a failed check: verified %v checkedAt %v", checked.Verified(), checked.CheckedAt)
		}

		if _, err := domains.MarkChecked(ctx, first, firstClaim.ID, true, now); err != nil {
			return err
		}

		if _, err := domains.MarkChecked(ctx, second, secondClaim.ID, true, now.Add(time.Second)); err != nil {
			return err
		}

		routed, err = domains.VerifiedWorkspaceIDs(ctx, domain)
		if err != nil {
			return err
		}

		if len(routed) != 2 || routed[0] != first || routed[1] != second {
			t.Errorf("routed %v, want [%s %s]", routed, first, second)
		}

		if _, err := domains.MarkChecked(ctx, pending, firstClaim.ID, true, now); !errors.Is(err, entity.ErrWorkspaceDomainNotFound) {
			t.Errorf("checking another workspace's claim: err = %v, want ErrWorkspaceDomainNotFound", err)
		}

		if err := domains.Delete(ctx, pending, secondClaim.ID); !errors.Is(err, entity.ErrWorkspaceDomainNotFound) {
			t.Errorf("deleting another workspace's claim: err = %v, want ErrWorkspaceDomainNotFound", err)
		}

		if err := domains.Delete(ctx, second, secondClaim.ID); err != nil {
			return err
		}

		routed, err = domains.VerifiedWorkspaceIDs(ctx, domain)
		if err != nil {
			return err
		}

		if len(routed) != 1 || routed[0] != first {
			t.Errorf("after removal routed %v, want [%s]", routed, first)
		}

		listed, err := domains.List(ctx, first)
		if err != nil {
			return err
		}

		if len(listed) != 1 || listed[0].Domain != domain || !listed[0].Verified() {
			t.Errorf("listed %+v", listed)
		}

		if _, err := domains.Add(ctx, claim(first, domain)); !errors.Is(err, entity.ErrWorkspaceDomainTaken) {
			t.Errorf("claiming twice: err = %v, want ErrWorkspaceDomainTaken", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
