package apitoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolation = "23505"
	nameUniqueIndex = "api_tokens_live_name_key"
)

func toEntity(model *dbpostgres.APIToken) (entity.APIToken, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.APIToken{}, fmt.Errorf("parse api token id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.APIToken{}, fmt.Errorf("parse api token account id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.APIToken{}, fmt.Errorf("parse api token workspace id: %w", err)
	}

	token := entity.APIToken{
		ID:          id,
		AccountID:   accountID,
		WorkspaceID: workspaceID,
		Name:        model.Name,
		TokenHash:   model.TokenHash,
		Scopes:      entity.NewAPIScopeSet(model.Scopes),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.ExpiresAt.Valid {
		expiresAt := model.ExpiresAt.Time
		token.ExpiresAt = &expiresAt
	}

	if model.RevokedAt.Valid {
		revokedAt := model.RevokedAt.Time
		token.RevokedAt = &revokedAt
	}

	if model.LastUsedAt.Valid {
		lastUsedAt := model.LastUsedAt.Time
		token.LastUsedAt = &lastUsedAt
	}

	return token, nil
}

func toModel(token entity.APIToken) *dbpostgres.APIToken {
	model := &dbpostgres.APIToken{
		ID:          token.ID.String(),
		AccountID:   token.AccountID.String(),
		WorkspaceID: token.WorkspaceID.String(),
		Name:        token.Name,
		TokenHash:   token.TokenHash,
		Scopes:      types.StringArray(token.Scopes.Strings()),
		CreatedAt:   token.CreatedAt,
		UpdatedAt:   token.UpdatedAt,
	}

	if token.ExpiresAt != nil {
		model.ExpiresAt = null.TimeFrom(*token.ExpiresAt)
	}

	if token.RevokedAt != nil {
		model.RevokedAt = null.TimeFrom(*token.RevokedAt)
	}

	if token.LastUsedAt != nil {
		model.LastUsedAt = null.TimeFrom(*token.LastUsedAt)
	}

	return model
}

type apiTokenRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.APIToken {
	return &apiTokenRepository{db: db}
}

func (r *apiTokenRepository) Create(ctx context.Context, token entity.APIToken) (entity.APIToken, error) {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	now := time.Now().UTC()
	token.CreatedAt = now
	token.UpdatedAt = now

	model := toModel(token)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameUniqueIndex {
			return entity.APIToken{}, entity.ErrAPITokenNameTaken
		}

		return entity.APIToken{}, fmt.Errorf("insert api token: %w", err)
	}

	return toEntity(model)
}

func (r *apiTokenRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.APIToken, error) {
	model, err := dbpostgres.APITokens(
		dbpostgres.APITokenWhere.TokenHash.EQ(tokenHash),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.APIToken{}, entity.ErrAPITokenNotFound
		}

		return entity.APIToken{}, fmt.Errorf("find api token by hash: %w", err)
	}

	return toEntity(model)
}

func (r *apiTokenRepository) ListByOwner(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) ([]entity.APIToken, error) {
	models, err := dbpostgres.APITokens(
		dbpostgres.APITokenWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.APITokenWhere.AccountID.EQ(accountID.String()),
		qm.Where("revoked_at IS NULL"),
		qm.OrderBy(dbpostgres.APITokenColumns.CreatedAt+" DESC"),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}

	tokens := make([]entity.APIToken, 0, len(models))

	for _, model := range models {
		token, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (r *apiTokenRepository) Revoke(
	ctx context.Context,
	workspaceID, accountID, tokenID uuid.UUID,
	revokedAt time.Time,
) error {
	updated, err := dbpostgres.APITokens(
		dbpostgres.APITokenWhere.ID.EQ(tokenID.String()),
		dbpostgres.APITokenWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.APITokenWhere.AccountID.EQ(accountID.String()),
		qm.Where("revoked_at IS NULL"),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.APITokenColumns.RevokedAt: null.TimeFrom(revokedAt),
		dbpostgres.APITokenColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}

	if updated == 0 {
		return entity.ErrAPITokenNotFound
	}

	return nil
}

func (r *apiTokenRepository) RecordUsage(ctx context.Context, tokenID uuid.UUID, usedAt time.Time) error {
	if _, err := dbpostgres.APITokens(
		dbpostgres.APITokenWhere.ID.EQ(tokenID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.APITokenColumns.LastUsedAt: null.TimeFrom(usedAt),
	}); err != nil {
		return fmt.Errorf("record api token usage: %w", err)
	}

	return nil
}
