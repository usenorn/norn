package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordReportQuery = `
INSERT INTO workspace_import_report_lines (
    id, run_id, phase, resource, subject, external_id, outcome, detail, recorded_at
)
SELECT unnest($1::uuid[]), $2, unnest($3::text[]), unnest($4::text[]), unnest($5::text[]),
       unnest($6::text[]), unnest($7::text[]), unnest($8::jsonb[]), unnest($9::timestamptz[])`

const reportLinesQuery = `
SELECT id, run_id, phase, resource, subject, external_id, outcome,
       coalesce(detail::text, ''), recorded_at
FROM workspace_import_report_lines
WHERE run_id = $1
  AND phase = $2
  AND ($3::boolean IS NOT TRUE
       OR (recorded_at, id) > ($4::timestamptz, $5::uuid))
ORDER BY recorded_at, id
LIMIT $6`

type reportRepository struct {
	db *postgres.Client
}

func NewImportReport(db *postgres.Client) repository.ImportReport {
	return &reportRepository{db: db}
}

func (r *reportRepository) Record(
	ctx context.Context,
	runID uuid.UUID,
	lines []entity.ImportReportLine,
) error {
	if len(lines) == 0 {
		return nil
	}

	ids := make([]string, 0, len(lines))
	phases := make([]string, 0, len(lines))
	resources := make([]string, 0, len(lines))
	subjects := make([]string, 0, len(lines))
	externalIDs := make([]string, 0, len(lines))
	outcomes := make([]string, 0, len(lines))
	details := make([]*string, 0, len(lines))
	recorded := make([]time.Time, 0, len(lines))

	for _, line := range lines {
		if line.ID == uuid.Nil {
			line.ID = uuid.New()
		}

		if line.RecordedAt.IsZero() {
			line.RecordedAt = time.Now().UTC()
		}

		detail, err := encodeDetail(line.Detail)
		if err != nil {
			return err
		}

		ids = append(ids, line.ID.String())
		phases = append(phases, string(line.Phase))
		resources = append(resources, string(line.Resource))
		subjects = append(subjects, line.Subject)
		externalIDs = append(externalIDs, line.ExternalID)
		outcomes = append(outcomes, string(line.Outcome))
		details = append(details, detail)
		recorded = append(recorded, line.RecordedAt)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordReportQuery,
		ids,
		runID.String(),
		phases, resources, subjects, externalIDs, outcomes, details, recorded,
	); err != nil {
		return fmt.Errorf("record import report lines: %w", err)
	}

	return nil
}

func (r *reportRepository) List(
	ctx context.Context,
	runID uuid.UUID,
	phase entity.ImportPhase,
	after *entity.ImportReportCursor,
	limit int,
) ([]entity.ImportReportLine, error) {
	recordedAt := time.Time{}
	cursorID := uuid.Nil.String()

	if after != nil {
		recordedAt = after.RecordedAt
		cursorID = after.ID.String()
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, reportLinesQuery, runID.String(), string(phase),
		after != nil, recordedAt, cursorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list import report lines: %w", err)
	}

	defer func() { _ = rows.Close() }()

	lines := make([]entity.ImportReportLine, 0, limit)

	for rows.Next() {
		var (
			line            entity.ImportReportLine
			rawID, rawRun   string
			phase, resource string
			outcome, detail string
		)

		if err := rows.Scan(
			&rawID, &rawRun, &phase, &resource, &line.Subject, &line.ExternalID,
			&outcome, &detail, &line.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan import report line: %w", err)
		}

		line.ID = parseID(rawID)
		line.RunID = parseID(rawRun)
		line.Phase = entity.ImportPhase(phase)
		line.Resource = entity.ImportResource(resource)
		line.Outcome = entity.ImportOutcome(outcome)

		if detail != "" {
			if err := json.Unmarshal([]byte(detail), &line.Detail); err != nil {
				return nil, fmt.Errorf("decode import report detail: %w", err)
			}
		}

		lines = append(lines, line)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read import report lines: %w", err)
	}

	return lines, nil
}

func encodeDetail(detail map[string]string) (*string, error) {
	if len(detail) == 0 {
		return nil, nil
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("encode import report detail: %w", err)
	}

	value := string(encoded)

	return &value, nil
}
