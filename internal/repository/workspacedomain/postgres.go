package workspacedomain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	workspaceDomainKey  = "workspace_domains_workspace_domain_key"
)

type domainRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.WorkspaceDomain {
	return &domainRepository{db: db}
}

func toEntity(model *dbpostgres.WorkspaceDomain) (entity.WorkspaceDomain, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.WorkspaceDomain{}, fmt.Errorf("parse workspace domain id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.WorkspaceDomain{}, fmt.Errorf("parse workspace domain workspace id: %w", err)
	}

	return entity.WorkspaceDomain{
		ID:          id,
		WorkspaceID: workspaceID,
		Domain:      model.Domain,
		Token:       model.Token,
		VerifiedAt:  model.VerifiedAt.Ptr(),
		CheckedAt:   model.CheckedAt.Ptr(),
		CreatedAt:   model.CreatedAt,
	}, nil
}

func domainTaken(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == workspaceDomainKey
}

func (r *domainRepository) find(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (*dbpostgres.WorkspaceDomain, error) {
	model, err := dbpostgres.WorkspaceDomains(
		dbpostgres.WorkspaceDomainWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceDomainWhere.WorkspaceID.EQ(workspaceID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrWorkspaceDomainNotFound
		}

		return nil, fmt.Errorf("read workspace domain: %w", err)
	}

	return model, nil
}

func (r *domainRepository) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.WorkspaceDomain, error) {
	models, err := dbpostgres.WorkspaceDomains(
		dbpostgres.WorkspaceDomainWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy(dbpostgres.WorkspaceDomainColumns.Domain),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list workspace domains: %w", err)
	}

	domains := make([]entity.WorkspaceDomain, 0, len(models))

	for _, model := range models {
		domain, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		domains = append(domains, domain)
	}

	return domains, nil
}

func (r *domainRepository) Get(ctx context.Context, workspaceID, id uuid.UUID) (entity.WorkspaceDomain, error) {
	model, err := r.find(ctx, workspaceID, id)
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	return toEntity(model)
}

func (r *domainRepository) Add(ctx context.Context, domain entity.WorkspaceDomain) (entity.WorkspaceDomain, error) {
	model := &dbpostgres.WorkspaceDomain{
		ID:          domain.ID.String(),
		WorkspaceID: domain.WorkspaceID.String(),
		Domain:      domain.Domain,
		Token:       domain.Token,
		CreatedAt:   time.Now().UTC(),
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if domainTaken(err) {
			return entity.WorkspaceDomain{}, entity.ErrWorkspaceDomainTaken
		}

		return entity.WorkspaceDomain{}, fmt.Errorf("insert workspace domain: %w", err)
	}

	return toEntity(model)
}

func (r *domainRepository) MarkChecked(
	ctx context.Context,
	workspaceID, id uuid.UUID,
	verified bool,
	at time.Time,
) (entity.WorkspaceDomain, error) {
	model, err := r.find(ctx, workspaceID, id)
	if err != nil {
		return entity.WorkspaceDomain{}, err
	}

	model.CheckedAt = null.TimeFrom(at)
	columns := []string{dbpostgres.WorkspaceDomainColumns.CheckedAt}

	if verified && !model.VerifiedAt.Valid {
		model.VerifiedAt = null.TimeFrom(at)
		columns = append(columns, dbpostgres.WorkspaceDomainColumns.VerifiedAt)
	}

	if _, err := model.Update(ctx, r.db.Querier(ctx), boil.Whitelist(columns...)); err != nil {
		return entity.WorkspaceDomain{}, fmt.Errorf("record workspace domain check: %w", err)
	}

	return toEntity(model)
}

func (r *domainRepository) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	model, err := r.find(ctx, workspaceID, id)
	if err != nil {
		return err
	}

	if _, err := model.Delete(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete workspace domain: %w", err)
	}

	return nil
}

func (r *domainRepository) VerifiedWorkspaceIDs(ctx context.Context, domain string) ([]uuid.UUID, error) {
	models, err := dbpostgres.WorkspaceDomains(
		dbpostgres.WorkspaceDomainWhere.Domain.EQ(domain),
		dbpostgres.WorkspaceDomainWhere.VerifiedAt.IsNotNull(),
		qm.OrderBy(dbpostgres.WorkspaceDomainColumns.VerifiedAt),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list workspaces verifying a domain: %w", err)
	}

	ids := make([]uuid.UUID, 0, len(models))

	for _, model := range models {
		id, err := uuid.Parse(model.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("parse verifying workspace id: %w", err)
		}

		ids = append(ids, id)
	}

	return ids, nil
}
