package imports

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const stageRecordsQuery = `
INSERT INTO workspace_import_records (
    run_id, resource, external_id, parent_external_id, payload,
    source_created_at, source_updated_at
)
SELECT $1, unnest($2::text[]), unnest($3::text[]), unnest($4::text[]), unnest($5::jsonb[]),
       unnest($6::timestamptz[]), unnest($7::timestamptz[])
ON CONFLICT (run_id, resource, external_id) DO UPDATE SET
    payload = excluded.payload,
    parent_external_id = excluded.parent_external_id,
    source_created_at = excluded.source_created_at,
    source_updated_at = excluded.source_updated_at
RETURNING xmax = 0`

const walkRecordsQuery = `
SELECT run_id, resource, external_id, parent_external_id, payload,
       source_created_at, source_updated_at, state, outcome_detail,
       coalesce(created_id::text, ''), seq
FROM workspace_import_records
WHERE run_id = $1 AND resource = $2 AND state = 'staged' AND seq > $3
ORDER BY seq
LIMIT $4`

const settleRecordsQuery = `
UPDATE workspace_import_records r
SET state = $3,
    created_id = d.created_id,
    outcome_detail = d.detail
FROM (
    SELECT unnest($4::text[]) AS external_id,
           unnest($5::uuid[]) AS created_id,
           unnest($6::text[]) AS detail
) d
WHERE r.run_id = $1
  AND r.resource = $2
  AND r.external_id = d.external_id
  AND r.state = 'staged'
RETURNING r.external_id`

const purgeRecordsQuery = `
DELETE FROM workspace_import_records
WHERE ctid IN (
    SELECT r.ctid
    FROM workspace_import_records r
    JOIN workspace_import_runs u ON u.id = r.run_id
    WHERE u.status IN ('imported', 'reverted', 'failed')
      AND coalesce(u.reverted_at, u.finished_at, u.updated_at) < $1
    LIMIT $2
)`

type recordRepository struct {
	db *postgres.Client
}

func NewImportRecord(db *postgres.Client) repository.ImportRecord {
	return &recordRepository{db: db}
}

func (r *recordRepository) Stage(
	ctx context.Context,
	runID uuid.UUID,
	records []entity.ImportRecord,
) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	resources := make([]string, 0, len(records))
	externalIDs := make([]string, 0, len(records))
	parents := make([]string, 0, len(records))
	payloads := make([]string, 0, len(records))
	sourceCreated := make([]*time.Time, 0, len(records))
	sourceUpdated := make([]*time.Time, 0, len(records))

	for _, record := range records {
		resources = append(resources, string(record.Resource))
		externalIDs = append(externalIDs, record.ExternalID)
		parents = append(parents, record.ParentExternalID)
		payloads = append(payloads, string(record.Payload))
		sourceCreated = append(sourceCreated, record.SourceCreatedAt)
		sourceUpdated = append(sourceUpdated, record.SourceUpdatedAt)
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		stageRecordsQuery,
		runID.String(),
		resources,
		externalIDs,
		parents,
		payloads,
		sourceCreated,
		sourceUpdated,
	)
	if err != nil {
		return 0, fmt.Errorf("stage import records: %w", err)
	}

	defer func() { _ = rows.Close() }()

	staged := 0

	for rows.Next() {
		var inserted bool

		if err := rows.Scan(&inserted); err != nil {
			return 0, fmt.Errorf("scan staged import record: %w", err)
		}

		if inserted {
			staged++
		}
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("stage import records: %w", err)
	}

	return staged, nil
}

func (r *recordRepository) Walk(
	ctx context.Context,
	runID uuid.UUID,
	walk entity.ImportWalk,
) ([]entity.ImportRecord, error) {
	walk = walk.Normalized()

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		walkRecordsQuery,
		runID.String(),
		string(walk.Resource),
		walk.After,
		walk.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("walk import records: %w", err)
	}

	defer func() { _ = rows.Close() }()

	records := make([]entity.ImportRecord, 0, walk.Limit)

	for rows.Next() {
		var (
			record            entity.ImportRecord
			rawRun, resource  string
			state, rawCreated string
			sourceCreated     sql.NullTime
			sourceUpdated     sql.NullTime
		)

		if err := rows.Scan(
			&rawRun, &resource, &record.ExternalID, &record.ParentExternalID, &record.Payload,
			&sourceCreated, &sourceUpdated, &state, &record.OutcomeDetail,
			&rawCreated, &record.Seq,
		); err != nil {
			return nil, fmt.Errorf("scan import record: %w", err)
		}

		record.RunID = parseID(rawRun)
		record.Resource = entity.ImportResource(resource)
		record.State = entity.ImportRecordState(state)
		record.CreatedID = parseID(rawCreated)
		record.SourceCreatedAt = optionalTime(sourceCreated)
		record.SourceUpdatedAt = optionalTime(sourceUpdated)

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import records: %w", err)
	}

	return records, nil
}

func (r *recordRepository) Settle(
	ctx context.Context,
	runID uuid.UUID,
	resource entity.ImportResource,
	state entity.ImportRecordState,
	outcomes []entity.ImportRecord,
) ([]string, error) {
	if len(outcomes) == 0 {
		return nil, nil
	}

	externalIDs := make([]string, 0, len(outcomes))
	created := make([]*string, 0, len(outcomes))
	details := make([]string, 0, len(outcomes))

	for _, outcome := range outcomes {
		externalIDs = append(externalIDs, outcome.ExternalID)
		created = append(created, optionalIdentifier(outcome.CreatedID))
		details = append(details, outcome.OutcomeDetail)
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		settleRecordsQuery,
		runID.String(),
		string(resource),
		string(state),
		externalIDs,
		created,
		details,
	)
	if err != nil {
		return nil, fmt.Errorf("settle import records: %w", err)
	}

	defer func() { _ = rows.Close() }()

	settled := make([]string, 0, len(outcomes))

	for rows.Next() {
		var externalID string

		if err := rows.Scan(&externalID); err != nil {
			return nil, fmt.Errorf("scan settled import record: %w", err)
		}

		settled = append(settled, externalID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settle import records: %w", err)
	}

	return settled, nil
}

func (r *recordRepository) Purge(ctx context.Context, before time.Time, limit int) (int, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, purgeRecordsQuery, before, limit)
	if err != nil {
		return 0, fmt.Errorf("purge import records: %w", err)
	}

	purged, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge import records: %w", err)
	}

	return int(purged), nil
}
