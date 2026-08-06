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

const recordLedgerQuery = `
INSERT INTO workspace_import_ledger (
    run_id, workspace_id, resource, created_id, external_id, reference, version_at_create
)
SELECT $1, unnest($2::uuid[]), unnest($3::text[]), unnest($4::uuid[]), unnest($5::text[]),
       unnest($6::text[]), unnest($7::integer[])
ON CONFLICT (run_id, resource, created_id) DO NOTHING`

const walkLedgerQuery = `
SELECT run_id, workspace_id, resource, created_id, external_id, reference,
       version_at_create, created_at_recorded, order_seq, reverted_at, revert_outcome
FROM workspace_import_ledger
WHERE run_id = $1 AND reverted_at IS NULL AND order_seq < $2
ORDER BY order_seq DESC
LIMIT $3`

const resolveLedgerQuery = `
SELECT external_id, created_id::text
FROM workspace_import_ledger
WHERE run_id = $1 AND resource = $2 AND external_id = ANY($3::text[])`

const restampLedgerQuery = `
UPDATE workspace_import_ledger l
SET version_at_create = d.version
FROM (
    SELECT unnest($2::text[]) AS resource,
           unnest($3::uuid[]) AS created_id,
           unnest($4::integer[]) AS version
) d
WHERE l.run_id = $1
  AND l.resource = d.resource
  AND l.created_id = d.created_id
  AND l.reverted_at IS NULL`

const settleLedgerQuery = `
UPDATE workspace_import_ledger l
SET reverted_at = $2, revert_outcome = d.outcome
FROM (
    SELECT unnest($3::text[]) AS resource,
           unnest($4::uuid[]) AS created_id,
           unnest($5::text[]) AS outcome
) d
WHERE l.run_id = $1
  AND l.resource = d.resource
  AND l.created_id = d.created_id
  AND l.reverted_at IS NULL`

type ledgerRepository struct {
	db *postgres.Client
}

func NewImportLedger(db *postgres.Client) repository.ImportLedger {
	return &ledgerRepository{db: db}
}

func (r *ledgerRepository) Record(
	ctx context.Context,
	runID uuid.UUID,
	entries []entity.ImportLedgerEntry,
) error {
	if len(entries) == 0 {
		return nil
	}

	workspaces := make([]string, 0, len(entries))
	resources := make([]string, 0, len(entries))
	created := make([]string, 0, len(entries))
	externalIDs := make([]string, 0, len(entries))
	references := make([]string, 0, len(entries))
	versions := make([]int, 0, len(entries))

	for _, entry := range entries {
		workspaces = append(workspaces, entry.WorkspaceID.String())
		resources = append(resources, string(entry.Resource))
		created = append(created, entry.CreatedID.String())
		externalIDs = append(externalIDs, entry.ExternalID)
		references = append(references, entry.Reference)
		versions = append(versions, entry.VersionAtCreate)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordLedgerQuery,
		runID.String(),
		workspaces, resources, created, externalIDs, references, versions,
	); err != nil {
		return fmt.Errorf("record import ledger entries: %w", err)
	}

	return nil
}

func (r *ledgerRepository) Walk(
	ctx context.Context,
	runID uuid.UUID,
	walk entity.ImportLedgerWalk,
) ([]entity.ImportLedgerEntry, error) {
	walk = walk.Normalized()

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, walkLedgerQuery, runID.String(), walk.Before, walk.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("walk import ledger: %w", err)
	}

	defer func() { _ = rows.Close() }()

	entries := make([]entity.ImportLedgerEntry, 0, walk.Limit)

	for rows.Next() {
		var (
			entry            entity.ImportLedgerEntry
			rawRun, rawSpace string
			resource, rawID  string
			outcome          string
			revertedAt       sql.NullTime
		)

		if err := rows.Scan(
			&rawRun, &rawSpace, &resource, &rawID, &entry.ExternalID, &entry.Reference,
			&entry.VersionAtCreate, &entry.CreatedAtRecorded, &entry.OrderSeq,
			&revertedAt, &outcome,
		); err != nil {
			return nil, fmt.Errorf("scan import ledger entry: %w", err)
		}

		entry.RunID = parseID(rawRun)
		entry.WorkspaceID = parseID(rawSpace)
		entry.Resource = entity.ImportResource(resource)
		entry.CreatedID = parseID(rawID)
		entry.RevertedAt = optionalTime(revertedAt)
		entry.RevertOutcome = entity.ImportOutcome(outcome)

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import ledger: %w", err)
	}

	return entries, nil
}

func (r *ledgerRepository) Resolve(
	ctx context.Context,
	runID uuid.UUID,
	resource entity.ImportResource,
	externalIDs []string,
) (map[string]uuid.UUID, error) {
	resolved := make(map[string]uuid.UUID, len(externalIDs))

	if len(externalIDs) == 0 {
		return resolved, nil
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, resolveLedgerQuery, runID.String(), string(resource), externalIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve import ledger entries: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var externalID, rawID string

		if err := rows.Scan(&externalID, &rawID); err != nil {
			return nil, fmt.Errorf("scan resolved import ledger entry: %w", err)
		}

		resolved[externalID] = parseID(rawID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read resolved import ledger entries: %w", err)
	}

	return resolved, nil
}

func (r *ledgerRepository) Restamp(
	ctx context.Context,
	runID uuid.UUID,
	entries []entity.ImportLedgerEntry,
) error {
	if len(entries) == 0 {
		return nil
	}

	resources := make([]string, 0, len(entries))
	created := make([]string, 0, len(entries))
	versions := make([]int, 0, len(entries))

	for _, entry := range entries {
		resources = append(resources, string(entry.Resource))
		created = append(created, entry.CreatedID.String())
		versions = append(versions, entry.VersionAtCreate)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, restampLedgerQuery, runID.String(), resources, created, versions,
	); err != nil {
		return fmt.Errorf("restamp import ledger entries: %w", err)
	}

	return nil
}

func (r *ledgerRepository) Settle(
	ctx context.Context,
	runID uuid.UUID,
	entries []entity.ImportLedgerEntry,
	at time.Time,
) error {
	if len(entries) == 0 {
		return nil
	}

	resources := make([]string, 0, len(entries))
	created := make([]string, 0, len(entries))
	outcomes := make([]string, 0, len(entries))

	for _, entry := range entries {
		resources = append(resources, string(entry.Resource))
		created = append(created, entry.CreatedID.String())
		outcomes = append(outcomes, string(entry.RevertOutcome))
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, settleLedgerQuery, runID.String(), at, resources, created, outcomes,
	); err != nil {
		return fmt.Errorf("settle import ledger entries: %w", err)
	}

	return nil
}
