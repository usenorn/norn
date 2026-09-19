package membership

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errMembershipRollback = errors.New("roll back the membership fixture")

type seededMember struct {
	accountID uuid.UUID
	kind      entity.AccountKind
	name      string
}

func liveClient(t *testing.T) (*postgres.Client, func()) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to test")
	}

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        2,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}

	return client, cleanup
}

func seedWorkspaceMembers(
	ctx context.Context,
	client *postgres.Client,
	repository *membershipRepository,
) (uuid.UUID, []seededMember, error) {
	workspaceID := uuid.New()

	if _, err := client.Querier(ctx).ExecContext(
		ctx,
		"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Meridian')",
		workspaceID,
		"meridian-"+workspaceID.String()[:8],
	); err != nil {
		return uuid.Nil, nil, fmt.Errorf("create workspace: %w", err)
	}

	seeded := []seededMember{
		{accountID: uuid.New(), kind: entity.AccountKindPerson, name: "Ana Person"},
		{accountID: uuid.New(), kind: entity.AccountKindAgent, name: "Bo Agent"},
		{accountID: uuid.New(), kind: entity.AccountKindIntegration, name: "Cy Integration"},
		{accountID: uuid.New(), kind: entity.AccountKindPerson, name: "Di Person"},
	}

	for _, member := range seeded {
		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO accounts (id, status, kind, display_name, email, timezone)
             VALUES ($1, 'active', $2, $3, $4, 'UTC')`,
			member.accountID,
			string(member.kind),
			member.name,
			member.accountID.String()+"@meridian.co",
		); err != nil {
			return uuid.Nil, nil, fmt.Errorf("create account: %w", err)
		}

		if _, err := repository.Create(ctx, entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   member.accountID,
			Role:        entity.MembershipRoleMember,
			Source:      entity.MembershipSourceManual,
		}); err != nil {
			return uuid.Nil, nil, fmt.Errorf("create membership: %w", err)
		}
	}

	return workspaceID, seeded, nil
}

func kindsOf(members []entity.WorkspaceMember) []entity.AccountKind {
	kinds := make([]entity.AccountKind, 0, len(members))
	for _, member := range members {
		kinds = append(kinds, member.AccountKind)
	}

	return kinds
}

func countOf(members []entity.WorkspaceMember, kind entity.AccountKind) int {
	count := 0

	for _, member := range members {
		if member.AccountKind == kind {
			count++
		}
	}

	return count
}

func TestAnAbsentKindFilterAnswersEveryKind(t *testing.T) {
	client, cleanup := liveClient(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		repository := &membershipRepository{db: client}

		workspaceID, seeded, err := seedWorkspaceMembers(ctx, client, repository)
		if err != nil {
			failure = err

			return errMembershipRollback
		}

		members, err := repository.ListPageByWorkspaceID(ctx, workspaceID, entity.MembershipPage{Limit: 10})
		if err != nil {
			failure = fmt.Errorf("list members: %w", err)

			return errMembershipRollback
		}

		if len(members) != len(seeded) {
			failure = fmt.Errorf("members = %v, want every seeded kind", kindsOf(members))
		}

		return errMembershipRollback
	})

	if !errors.Is(err, errMembershipRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func TestAPersonFilterAnswersPeopleOnly(t *testing.T) {
	client, cleanup := liveClient(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		repository := &membershipRepository{db: client}

		workspaceID, _, err := seedWorkspaceMembers(ctx, client, repository)
		if err != nil {
			failure = err

			return errMembershipRollback
		}

		members, err := repository.ListPageByWorkspaceID(ctx, workspaceID, entity.MembershipPage{
			Kinds: []entity.AccountKind{entity.AccountKindPerson},
			Limit: 10,
		})
		if err != nil {
			failure = fmt.Errorf("list members: %w", err)

			return errMembershipRollback
		}

		if len(members) != 2 || countOf(members, entity.AccountKindPerson) != 2 {
			failure = fmt.Errorf("members = %v, want the two people only", kindsOf(members))
		}

		return errMembershipRollback
	})

	if !errors.Is(err, errMembershipRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func TestAMachineFilterAnswersAgentsAndIntegrationsAndNoPerson(t *testing.T) {
	client, cleanup := liveClient(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		repository := &membershipRepository{db: client}

		workspaceID, _, err := seedWorkspaceMembers(ctx, client, repository)
		if err != nil {
			failure = err

			return errMembershipRollback
		}

		members, err := repository.ListPageByWorkspaceID(ctx, workspaceID, entity.MembershipPage{
			Kinds: []entity.AccountKind{entity.AccountKindAgent, entity.AccountKindIntegration},
			Limit: 10,
		})
		if err != nil {
			failure = fmt.Errorf("list members: %w", err)

			return errMembershipRollback
		}

		agents := countOf(members, entity.AccountKindAgent)
		integrations := countOf(members, entity.AccountKindIntegration)
		people := countOf(members, entity.AccountKindPerson)

		if agents != 1 || integrations != 1 || people != 0 {
			failure = fmt.Errorf("members = %v, want one agent, one integration and no person", kindsOf(members))
		}

		return errMembershipRollback
	})

	if !errors.Is(err, errMembershipRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func TestPagingAFilteredListNeverMixesInAnotherKind(t *testing.T) {
	client, cleanup := liveClient(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		repository := &membershipRepository{db: client}

		workspaceID, _, err := seedWorkspaceMembers(ctx, client, repository)
		if err != nil {
			failure = err

			return errMembershipRollback
		}

		page := entity.MembershipPage{
			Kinds: []entity.AccountKind{entity.AccountKindPerson},
			Limit: 1,
		}

		first, err := repository.ListPageByWorkspaceID(ctx, workspaceID, page)
		if err != nil {
			failure = fmt.Errorf("list first page: %w", err)

			return errMembershipRollback
		}

		if len(first) != 1 || first[0].AccountKind != entity.AccountKindPerson {
			failure = fmt.Errorf("first page = %v, want one person", kindsOf(first))

			return errMembershipRollback
		}

		cursor := first[0].Cursor()
		page.Cursor = &cursor

		second, err := repository.ListPageByWorkspaceID(ctx, workspaceID, page)
		if err != nil {
			failure = fmt.Errorf("list second page: %w", err)

			return errMembershipRollback
		}

		if len(second) != 1 || second[0].AccountKind != entity.AccountKindPerson {
			failure = fmt.Errorf("second page = %v, want the other person", kindsOf(second))

			return errMembershipRollback
		}

		if second[0].Membership.AccountID == first[0].Membership.AccountID {
			failure = errors.New("the second page repeated the first row instead of moving forward")
		}

		return errMembershipRollback
	})

	if !errors.Is(err, errMembershipRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
