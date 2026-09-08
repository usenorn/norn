package cycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordResultsQuery = `
INSERT INTO workspace_cycle_results (
    cycle_id, issue_id, team_id, state_category, state_name, decision
)
VALUES %s
ON CONFLICT (cycle_id, issue_id) DO UPDATE
SET team_id = excluded.team_id,
    state_category = excluded.state_category,
    state_name = excluded.state_name,
    decision = excluded.decision,
    recorded_at = now()`

const resultsByCycleQuery = `
SELECT cycle_id, issue_id, team_id, state_category, state_name, decision, recorded_at
FROM workspace_cycle_results
WHERE cycle_id = $1
ORDER BY recorded_at, issue_id`

const recordingFromQuery = `SELECT recording_at FROM cycle_history_coverage LIMIT 1`

type resultRepository struct {
	db *postgres.Client
}

func NewResult(db *postgres.Client) repository.CycleResult {
	return &resultRepository{db: db}
}

func (r *resultRepository) Record(ctx context.Context, results []entity.CycleResult) error {
	if len(results) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(results))
	arguments := make([]any, 0, len(results)*6)

	for index, result := range results {
		at := index * 6
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d)",
			at+1, at+2, at+3, at+4, at+5, at+6,
		))
		arguments = append(arguments,
			result.CycleID.String(),
			result.IssueID.String(),
			result.TeamID.String(),
			string(result.Category),
			result.StateName,
			string(result.Decision),
		)
	}

	query := fmt.Sprintf(recordResultsQuery, strings.Join(placeholders, ", "))

	if _, err := r.db.Querier(ctx).ExecContext(ctx, query, arguments...); err != nil {
		return fmt.Errorf("record cycle results: %w", err)
	}

	return nil
}

func (r *resultRepository) ListByCycleID(
	ctx context.Context,
	cycleID uuid.UUID,
) ([]entity.CycleResult, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, resultsByCycleQuery, cycleID.String())
	if err != nil {
		return nil, fmt.Errorf("list cycle results: %w", err)
	}

	defer func() { _ = rows.Close() }()

	results := make([]entity.CycleResult, 0)

	for rows.Next() {
		var (
			result                   entity.CycleResult
			cycleID, issueID, teamID string
			category, decision       string
		)

		if err := rows.Scan(
			&cycleID, &issueID, &teamID,
			&category, &result.StateName, &decision, &result.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cycle result: %w", err)
		}

		for target, raw := range map[*uuid.UUID]string{
			&result.CycleID: cycleID,
			&result.IssueID: issueID,
			&result.TeamID:  teamID,
		} {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse cycle result id: %w", err)
			}

			*target = parsed
		}

		result.Category = entity.StateCategory(category)
		result.Decision = entity.CycleRollover(decision)

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read cycle results: %w", err)
	}

	return results, nil
}

func (r *resultRepository) RecordingFrom(ctx context.Context) (time.Time, error) {
	var from time.Time

	if err := r.db.Querier(ctx).QueryRowContext(ctx, recordingFromQuery).Scan(&from); err != nil {
		return time.Time{}, fmt.Errorf("read the history coverage boundary: %w", err)
	}

	return from, nil
}
