package aiprovider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type aiProviderRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.AIProvider {
	return &aiProviderRepository{db: db, crypter: sealer}
}

func toEntity(model *dbpostgres.WorkspaceAiProvider) (entity.AIProviderConnection, error) {
	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.AIProviderConnection{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.AIProviderConnection{
		WorkspaceID: workspaceID,
		Provider:    entity.AIProviderKind(model.Provider),
		Endpoint: entity.AIProviderEndpoint{
			BaseURL:             model.BaseURL,
			AllowPrivateAddress: model.AllowPrivateAddress,
		},
		KeyHint:      model.APIKeyHint,
		DefaultModel: model.DefaultModel,
		VerifiedAt:   model.VerifiedAt.Ptr(),
		FailedAt:     model.FailedAt.Ptr(),
		Failure:      entity.AIProviderFailure(model.Failure),
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}, nil
}

func (r *aiProviderRepository) seal(apiKey string) ([]byte, error) {
	sealed, err := r.crypter.Seal([]byte(apiKey))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrAIProviderEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal ai provider key: %w", err)
	}

	return sealed, nil
}

func (r *aiProviderRepository) open(sealed []byte) (string, error) {
	apiKey, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrAIProviderEncryptionKeyMissing
		}

		return "", fmt.Errorf("open stored ai provider key: %w", err)
	}

	return string(apiKey), nil
}

func (r *aiProviderRepository) find(
	ctx context.Context,
	workspaceID uuid.UUID,
) (*dbpostgres.WorkspaceAiProvider, error) {
	model, err := dbpostgres.FindWorkspaceAiProvider(ctx, r.db.Querier(ctx), workspaceID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrAIProviderNotConfigured
		}

		return nil, fmt.Errorf("read ai provider: %w", err)
	}

	return model, nil
}

func (r *aiProviderRepository) Get(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.AIProviderConnection, error) {
	model, err := r.find(ctx, workspaceID)
	if err != nil {
		return entity.AIProviderConnection{}, err
	}

	return toEntity(model)
}

func (r *aiProviderRepository) APIKey(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	model, err := r.find(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	return r.open(model.APIKeySealed)
}

func (r *aiProviderRepository) Save(
	ctx context.Context,
	connection entity.AIProviderConnection,
	apiKey string,
) (entity.AIProviderConnection, error) {
	sealed, err := r.seal(apiKey)
	if err != nil {
		return entity.AIProviderConnection{}, err
	}

	now := time.Now().UTC()

	model := &dbpostgres.WorkspaceAiProvider{
		WorkspaceID:         connection.WorkspaceID.String(),
		Provider:            string(connection.Provider),
		BaseURL:             connection.Endpoint.BaseURL,
		AllowPrivateAddress: connection.Endpoint.AllowPrivateAddress,
		APIKeySealed:        sealed,
		APIKeyHint:          entity.AIProviderKeyHint(apiKey),
		DefaultModel:        connection.DefaultModel,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := model.Upsert(
		ctx,
		r.db.Querier(ctx),
		true,
		[]string{dbpostgres.WorkspaceAiProviderColumns.WorkspaceID},
		boil.Whitelist(
			dbpostgres.WorkspaceAiProviderColumns.Provider,
			dbpostgres.WorkspaceAiProviderColumns.BaseURL,
			dbpostgres.WorkspaceAiProviderColumns.AllowPrivateAddress,
			dbpostgres.WorkspaceAiProviderColumns.APIKeySealed,
			dbpostgres.WorkspaceAiProviderColumns.APIKeyHint,
			dbpostgres.WorkspaceAiProviderColumns.DefaultModel,
			dbpostgres.WorkspaceAiProviderColumns.VerifiedAt,
			dbpostgres.WorkspaceAiProviderColumns.FailedAt,
			dbpostgres.WorkspaceAiProviderColumns.Failure,
			dbpostgres.WorkspaceAiProviderColumns.UpdatedAt,
		),
		boil.Infer(),
	); err != nil {
		return entity.AIProviderConnection{}, fmt.Errorf("save ai provider: %w", err)
	}

	return r.Get(ctx, connection.WorkspaceID)
}

func (r *aiProviderRepository) update(
	ctx context.Context,
	workspaceID uuid.UUID,
	columns dbpostgres.M,
) error {
	columns[dbpostgres.WorkspaceAiProviderColumns.UpdatedAt] = time.Now().UTC()

	rows, err := dbpostgres.WorkspaceAiProviders(
		dbpostgres.WorkspaceAiProviderWhere.WorkspaceID.EQ(workspaceID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), columns)
	if err != nil {
		return fmt.Errorf("update ai provider: %w", err)
	}

	if rows == 0 {
		return entity.ErrAIProviderNotConfigured
	}

	return nil
}

func (r *aiProviderRepository) Update(
	ctx context.Context,
	connection entity.AIProviderConnection,
) (entity.AIProviderConnection, error) {
	if err := r.update(ctx, connection.WorkspaceID, dbpostgres.M{
		dbpostgres.WorkspaceAiProviderColumns.Provider:            string(connection.Provider),
		dbpostgres.WorkspaceAiProviderColumns.BaseURL:             connection.Endpoint.BaseURL,
		dbpostgres.WorkspaceAiProviderColumns.AllowPrivateAddress: connection.Endpoint.AllowPrivateAddress,
		dbpostgres.WorkspaceAiProviderColumns.DefaultModel:        connection.DefaultModel,
		dbpostgres.WorkspaceAiProviderColumns.VerifiedAt:          null.Time{},
		dbpostgres.WorkspaceAiProviderColumns.FailedAt:            null.Time{},
		dbpostgres.WorkspaceAiProviderColumns.Failure:             "",
	}); err != nil {
		return entity.AIProviderConnection{}, err
	}

	return r.Get(ctx, connection.WorkspaceID)
}

func (r *aiProviderRepository) MarkVerified(
	ctx context.Context,
	workspaceID uuid.UUID,
	at time.Time,
) error {
	return r.update(ctx, workspaceID, dbpostgres.M{
		dbpostgres.WorkspaceAiProviderColumns.VerifiedAt: null.TimeFrom(at),
		dbpostgres.WorkspaceAiProviderColumns.FailedAt:   null.Time{},
		dbpostgres.WorkspaceAiProviderColumns.Failure:    "",
	})
}

func (r *aiProviderRepository) MarkFailed(
	ctx context.Context,
	workspaceID uuid.UUID,
	failure entity.AIProviderFailure,
	at time.Time,
) error {
	return r.update(ctx, workspaceID, dbpostgres.M{
		dbpostgres.WorkspaceAiProviderColumns.FailedAt: null.TimeFrom(at),
		dbpostgres.WorkspaceAiProviderColumns.Failure:  string(failure),
	})
}

func (r *aiProviderRepository) Delete(ctx context.Context, workspaceID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAiProviders(
		dbpostgres.WorkspaceAiProviderWhere.WorkspaceID.EQ(workspaceID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete ai provider: %w", err)
	}

	if rows == 0 {
		return entity.ErrAIProviderNotConfigured
	}

	return nil
}
