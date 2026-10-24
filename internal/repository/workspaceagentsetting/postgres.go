package workspaceagentsetting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

func settingsFromModel(model *dbpostgres.WorkspaceAgentSetting) (entity.WorkspaceAgentSettings, error) {
	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.WorkspaceAgentSettings{}, fmt.Errorf("parse workspace agent settings workspace id: %w", err)
	}

	return entity.WorkspaceAgentSettings{
		WorkspaceID:   workspaceID,
		HoldDocWrites: entity.AgentHold(model.HoldDocWrites),
	}.Normalised(), nil
}

type workspaceAgentSettingRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.WorkspaceAgentSetting {
	return &workspaceAgentSettingRepository{db: db}
}

func (r *workspaceAgentSettingRepository) Settings(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceAgentSettings, error) {
	model, err := dbpostgres.FindWorkspaceAgentSetting(ctx, r.db.Querier(ctx), workspaceID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return entity.WorkspaceAgentSettings{WorkspaceID: workspaceID}.Normalised(), nil
	}

	if err != nil {
		return entity.WorkspaceAgentSettings{}, fmt.Errorf("read workspace agent settings: %w", err)
	}

	return settingsFromModel(model)
}

func (r *workspaceAgentSettingRepository) Upsert(
	ctx context.Context,
	settings entity.WorkspaceAgentSettings,
) (entity.WorkspaceAgentSettings, error) {
	now := time.Now().UTC()

	model := &dbpostgres.WorkspaceAgentSetting{
		WorkspaceID:   settings.WorkspaceID.String(),
		HoldDocWrites: string(settings.HoldDocWrites),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := model.Upsert(
		ctx,
		r.db.Querier(ctx),
		true,
		[]string{dbpostgres.WorkspaceAgentSettingColumns.WorkspaceID},
		boil.Whitelist(
			dbpostgres.WorkspaceAgentSettingColumns.HoldDocWrites,
			dbpostgres.WorkspaceAgentSettingColumns.UpdatedAt,
		),
		boil.Infer(),
	); err != nil {
		return entity.WorkspaceAgentSettings{}, fmt.Errorf("save workspace agent settings: %w", err)
	}

	return settingsFromModel(model)
}
