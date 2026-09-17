package workspace

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
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

func uniqueSlug(prefix string) string {
	return prefix + "-" + uuid.NewString()[:8]
}

func committedWorkspace(t *testing.T, db *postgres.Client, workspaces repository.Workspace) entity.Workspace {
	t.Helper()

	ctx := context.Background()

	created, err := workspaces.Create(ctx, entity.Workspace{Slug: uniqueSlug("race"), Name: "Race"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	t.Cleanup(func() {
		if err := workspaces.Purge(context.Background(), created.ID); err != nil {
			t.Errorf("purge workspace %s: %v", created.ID, err)
		}
	})

	return created
}

func TestEveryWorkspaceSettingSurvivesBeingReadBack(t *testing.T) {
	db := reach(t)
	workspaces := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		created, err := workspaces.Create(ctx, entity.Workspace{Slug: uniqueSlug("roundtrip"), Name: "Roundtrip"})
		if err != nil {
			return err
		}

		if created.WeekStartsOn != entity.WeekDayMonday || created.LogoObjectKey != "" {
			t.Errorf("fresh workspace week start %q logo %q, want monday and none", created.WeekStartsOn, created.LogoObjectKey)
		}

		settings := repository.WorkspaceSettings{
			Slug:         uniqueSlug("renamed"),
			Name:         "Renamed",
			Timezone:     "Asia/Tokyo",
			WeekStartsOn: entity.WeekDaySunday,
		}

		if _, err := workspaces.UpdateSettings(ctx, created.ID, settings); err != nil {
			return err
		}

		logoKey := entity.WorkspaceLogoKey(created.ID, ".png")

		if _, err := workspaces.SetLogo(ctx, created.ID, logoKey); err != nil {
			return err
		}

		read, err := workspaces.GetByID(ctx, created.ID)
		if err != nil {
			return err
		}

		if read.Slug != settings.Slug || read.Name != settings.Name || read.Timezone != settings.Timezone ||
			read.WeekStartsOn != settings.WeekStartsOn || read.LogoObjectKey != logoKey || read.DefaultTeamID != nil {
			t.Errorf("read back %+v, want %+v with logo %q", read, settings, logoKey)
		}

		cleared, err := workspaces.SetLogo(ctx, created.ID, "")
		if err != nil {
			return err
		}

		if cleared.LogoObjectKey != "" {
			t.Errorf("logo after removal = %q, want none", cleared.LogoObjectKey)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("the fixture was not rolled back: %v", err)
	}
}

func TestAFormerAddressIsHeldForItsWorkspaceUntilItExpires(t *testing.T) {
	db := reach(t)
	workspaces := New(db)
	now := time.Now().UTC()

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		owner, err := workspaces.Create(ctx, entity.Workspace{Slug: uniqueSlug("owner"), Name: "Owner"})
		if err != nil {
			return err
		}

		other, err := workspaces.Create(ctx, entity.Workspace{Slug: uniqueSlug("other"), Name: "Other"})
		if err != nil {
			return err
		}

		held := uniqueSlug("held")
		lapsed := uniqueSlug("lapsed")

		if err := workspaces.RecordSlugRedirect(ctx, held, owner.ID, now.Add(entity.WorkspaceSlugRedirectTTL)); err != nil {
			return err
		}

		if err := workspaces.RecordSlugRedirect(ctx, lapsed, owner.ID, now.Add(-time.Minute)); err != nil {
			return err
		}

		if resolved, err := workspaces.ResolveSlugRedirect(ctx, held, now); err != nil || resolved != owner.ID {
			t.Errorf("resolve held address = %s, %v; want %s", resolved, err, owner.ID)
		}

		if _, err := workspaces.ResolveSlugRedirect(ctx, lapsed, now); !errors.Is(err, entity.ErrWorkspaceNotFound) {
			t.Errorf("resolve lapsed address error = %v, want %v", err, entity.ErrWorkspaceNotFound)
		}

		if err := workspaces.ReserveSlug(ctx, held, other.ID, now); !errors.Is(err, entity.ErrWorkspaceSlugTaken) {
			t.Errorf("another workspace reserving a held address = %v, want %v", err, entity.ErrWorkspaceSlugTaken)
		}

		if err := workspaces.ReserveSlug(ctx, owner.Slug, other.ID, now); !errors.Is(err, entity.ErrWorkspaceSlugTaken) {
			t.Errorf("reserving a live address = %v, want %v", err, entity.ErrWorkspaceSlugTaken)
		}

		if err := workspaces.ReserveSlug(ctx, lapsed, other.ID, now); err != nil {
			t.Errorf("reserving a lapsed address = %v, want it free", err)
		}

		if err := workspaces.ReserveSlug(ctx, held, owner.ID, now); err != nil {
			t.Errorf("the owner taking its former address back = %v, want it allowed", err)
		}

		if _, err := workspaces.ResolveSlugRedirect(ctx, held, now); !errors.Is(err, entity.ErrWorkspaceNotFound) {
			t.Errorf("address taken back still redirects: %v", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("the fixture was not rolled back: %v", err)
	}
}

func TestTwoClaimsOnOneAddressAtOnceLeaveExactlyOneWinner(t *testing.T) {
	db := reach(t)
	workspaces := New(db)

	first := committedWorkspace(t, db, workspaces)
	second := committedWorkspace(t, db, workspaces)

	cases := []struct {
		name  string
		claim func(ctx context.Context, slug string, index int) error
	}{
		{
			name: "two renames",
			claim: func(ctx context.Context, slug string, index int) error {
				renaming := []entity.Workspace{first, second}[index]

				if err := workspaces.ReserveSlug(ctx, slug, renaming.ID, time.Now().UTC()); err != nil {
					return err
				}

				time.Sleep(150 * time.Millisecond)

				_, err := workspaces.UpdateSettings(ctx, renaming.ID, repository.WorkspaceSettings{
					Slug:         slug,
					Name:         renaming.Name,
					Timezone:     renaming.Timezone,
					WeekStartsOn: renaming.WeekStartsOn,
				})

				return err
			},
		},
		{
			name: "a rename and a new workspace",
			claim: func(ctx context.Context, slug string, index int) error {
				if index == 0 {
					if err := workspaces.ReserveSlug(ctx, slug, first.ID, time.Now().UTC()); err != nil {
						return err
					}

					time.Sleep(150 * time.Millisecond)

					_, err := workspaces.UpdateSettings(ctx, first.ID, repository.WorkspaceSettings{
						Slug:         slug,
						Name:         first.Name,
						Timezone:     first.Timezone,
						WeekStartsOn: first.WeekStartsOn,
					})

					return err
				}

				id := uuid.New()

				if err := workspaces.ReserveSlug(ctx, slug, id, time.Now().UTC()); err != nil {
					return err
				}

				time.Sleep(150 * time.Millisecond)

				created, err := workspaces.Create(ctx, entity.Workspace{ID: id, Slug: slug, Name: "Newcomer"})
				if err == nil {
					t.Cleanup(func() { _ = workspaces.Purge(context.Background(), created.ID) })
				}

				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slug := uniqueSlug("contested")
			results := make([]error, 2)
			start := make(chan struct{})

			var wg sync.WaitGroup

			for index := range results {
				wg.Add(1)

				go func() {
					defer wg.Done()

					<-start

					results[index] = db.WithTx(context.Background(), func(ctx context.Context) error {
						return tc.claim(ctx, slug, index)
					})
				}()
			}

			close(start)
			wg.Wait()

			won, lost := 0, 0

			for _, err := range results {
				switch {
				case err == nil:
					won++
				case errors.Is(err, entity.ErrWorkspaceSlugTaken):
					lost++
				default:
					t.Errorf("claim failed for another reason: %v", err)
				}
			}

			if won != 1 || lost != 1 {
				t.Fatalf("won %d, lost %d, want exactly one of each: %v", won, lost, results)
			}
		})
	}
}

func TestAnAddressCannotBeTakenWhileItsHolderIsLeavingIt(t *testing.T) {
	db := reach(t)
	workspaces := New(db)

	leaving := committedWorkspace(t, db, workspaces)
	taking := committedWorkspace(t, db, workspaces)

	address := leaving.Slug
	destination := uniqueSlug("moved")

	var leaveErr, takeErr error

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		leaveErr = db.WithTx(context.Background(), func(ctx context.Context) error {
			now := time.Now().UTC()

			if err := workspaces.ReserveSlug(ctx, destination, leaving.ID, now); err != nil {
				return err
			}

			if _, err := workspaces.UpdateSettings(ctx, leaving.ID, repository.WorkspaceSettings{
				Slug:         destination,
				Name:         leaving.Name,
				Timezone:     leaving.Timezone,
				WeekStartsOn: leaving.WeekStartsOn,
			}); err != nil {
				return err
			}

			if err := workspaces.RecordSlugRedirect(ctx, address, leaving.ID, now.Add(entity.WorkspaceSlugRedirectTTL)); err != nil {
				return err
			}

			time.Sleep(200 * time.Millisecond)

			return nil
		})
	}()

	go func() {
		defer wg.Done()

		time.Sleep(50 * time.Millisecond)

		takeErr = db.WithTx(context.Background(), func(ctx context.Context) error {
			if err := workspaces.ReserveSlug(ctx, address, taking.ID, time.Now().UTC()); err != nil {
				return err
			}

			_, err := workspaces.UpdateSettings(ctx, taking.ID, repository.WorkspaceSettings{
				Slug:         address,
				Name:         taking.Name,
				Timezone:     taking.Timezone,
				WeekStartsOn: taking.WeekStartsOn,
			})

			return err
		})
	}()

	wg.Wait()

	if leaveErr != nil {
		t.Fatalf("leaving the address failed: %v", leaveErr)
	}

	if !errors.Is(takeErr, entity.ErrWorkspaceSlugTaken) {
		t.Fatalf("taking the address being left = %v, want %v", takeErr, entity.ErrWorkspaceSlugTaken)
	}

	resolved, err := workspaces.ResolveSlugRedirect(context.Background(), address, time.Now().UTC())
	if err != nil || resolved != leaving.ID {
		t.Fatalf("former address resolves to %s, %v; want %s", resolved, err, leaving.ID)
	}
}
