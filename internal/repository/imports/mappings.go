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

const mappingsByRunQuery = `
SELECT run_id, kind, source_key, source_label, source_email, decision,
       coalesce(target_id::text, ''), target_value,
       coalesce(suggested_target_id::text, ''), suggested_reason,
       coalesce(decided_by_account_id::text, ''), decided_at
FROM workspace_import_mappings
WHERE run_id = $1
ORDER BY kind, source_key`

const saveMappingsQuery = `
INSERT INTO workspace_import_mappings (
    run_id, kind, source_key, source_label, source_email, decision,
    target_id, target_value, suggested_target_id, suggested_reason
)
SELECT $1, unnest($2::text[]), unnest($3::text[]), unnest($4::text[]), unnest($5::text[]),
       $6, unnest($7::uuid[]), unnest($8::text[]), unnest($9::uuid[]), unnest($10::text[])
ON CONFLICT (run_id, kind, source_key) DO UPDATE SET
    source_label = excluded.source_label,
    source_email = excluded.source_email,
    suggested_target_id = excluded.suggested_target_id,
    suggested_reason = excluded.suggested_reason
WHERE workspace_import_mappings.decided_at IS NULL`

const decideMappingsQuery = `
INSERT INTO workspace_import_mappings (
    run_id, kind, source_key, decision, target_id, target_value,
    decided_by_account_id, decided_at
)
SELECT $1, unnest($2::text[]), unnest($3::text[]), unnest($4::text[]), unnest($5::uuid[]),
       unnest($6::text[]), $7, $8
ON CONFLICT (run_id, kind, source_key) DO UPDATE SET
    decision = excluded.decision,
    target_id = excluded.target_id,
    target_value = excluded.target_value,
    decided_by_account_id = excluded.decided_by_account_id,
    decided_at = excluded.decided_at`

type mappingRepository struct {
	db *postgres.Client
}

func NewImportMapping(db *postgres.Client) repository.ImportMapping {
	return &mappingRepository{db: db}
}

func (r *mappingRepository) List(
	ctx context.Context,
	runID uuid.UUID,
) ([]entity.ImportMapping, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, mappingsByRunQuery, runID.String())
	if err != nil {
		return nil, fmt.Errorf("list import mappings: %w", err)
	}

	defer func() { _ = rows.Close() }()

	mappings := make([]entity.ImportMapping, 0)

	for rows.Next() {
		var (
			mapping             entity.ImportMapping
			rawRun, kind        string
			decision, rawTarget string
			rawSuggested        string
			rawDecidedBy        string
			decidedAt           sql.NullTime
		)

		if err := rows.Scan(
			&rawRun, &kind, &mapping.SourceKey, &mapping.SourceLabel, &mapping.SourceEmail,
			&decision, &rawTarget, &mapping.TargetValue,
			&rawSuggested, &mapping.SuggestedReason, &rawDecidedBy, &decidedAt,
		); err != nil {
			return nil, fmt.Errorf("scan import mapping: %w", err)
		}

		mapping.RunID = parseID(rawRun)
		mapping.Kind = entity.ImportMappingKind(kind)
		mapping.TargetID = parseID(rawTarget)
		mapping.SuggestedTargetID = parseID(rawSuggested)
		mapping.DecidedByAccount = parseID(rawDecidedBy)
		mapping.DecidedAt = optionalTime(decidedAt)

		if decidedAt.Valid {
			mapping.Decision = entity.ImportDecision(decision)
		}

		mappings = append(mappings, mapping)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import mappings: %w", err)
	}

	return mappings, nil
}

func (r *mappingRepository) Save(
	ctx context.Context,
	runID uuid.UUID,
	mappings []entity.ImportMapping,
) error {
	if len(mappings) == 0 {
		return nil
	}

	kinds := make([]string, 0, len(mappings))
	keys := make([]string, 0, len(mappings))
	labels := make([]string, 0, len(mappings))
	emails := make([]string, 0, len(mappings))
	targets := make([]*string, 0, len(mappings))
	values := make([]string, 0, len(mappings))
	suggested := make([]*string, 0, len(mappings))
	reasons := make([]string, 0, len(mappings))

	for _, mapping := range mappings {
		kinds = append(kinds, string(mapping.Kind))
		keys = append(keys, mapping.SourceKey)
		labels = append(labels, mapping.SourceLabel)
		emails = append(emails, mapping.SourceEmail)
		targets = append(targets, optionalIdentifier(mapping.TargetID))
		values = append(values, mapping.TargetValue)
		suggested = append(suggested, optionalIdentifier(mapping.SuggestedTargetID))
		reasons = append(reasons, mapping.SuggestedReason)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		saveMappingsQuery,
		runID.String(),
		kinds, keys, labels, emails,
		string(entity.ImportDecisionSkip),
		targets, values, suggested, reasons,
	); err != nil {
		return fmt.Errorf("save import mappings: %w", err)
	}

	return nil
}

func (r *mappingRepository) Decide(
	ctx context.Context,
	runID uuid.UUID,
	decisions []entity.ImportMapping,
	by uuid.UUID,
	at time.Time,
) error {
	if len(decisions) == 0 {
		return nil
	}

	kinds := make([]string, 0, len(decisions))
	keys := make([]string, 0, len(decisions))
	decided := make([]string, 0, len(decisions))
	targets := make([]*string, 0, len(decisions))
	values := make([]string, 0, len(decisions))

	for _, decision := range decisions {
		kinds = append(kinds, string(decision.Kind))
		keys = append(keys, decision.SourceKey)
		decided = append(decided, string(decision.Decision))
		targets = append(targets, optionalIdentifier(decision.TargetID))
		values = append(values, decision.TargetValue)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		decideMappingsQuery,
		runID.String(),
		kinds, keys, decided, targets, values,
		optionalID(by),
		at,
	); err != nil {
		return fmt.Errorf("decide import mappings: %w", err)
	}

	return nil
}
