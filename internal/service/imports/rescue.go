package imports

import (
	"context"
	"fmt"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *importsService) Rescue(ctx context.Context) (int, error) {
	now := time.Now().UTC()

	due, err := s.runs.ListLeaseExpired(ctx, now, now.Add(-s.cfg.LeaseTTL), s.cfg.RescueBatch)
	if err != nil {
		return 0, err
	}

	rescued := 0
	failures := 0

	for _, run := range due {
		parked, err := s.parked(ctx, run, now)
		if err != nil {
			failures++

			s.warnRescue(ctx, run, err)

			continue
		}

		if parked {
			continue
		}

		if err := s.redrive(ctx, run, now); err != nil {
			failures++

			s.warnRescue(ctx, run, err)

			continue
		}

		rescued++
	}

	if failures > 0 {
		return rescued, fmt.Errorf("rescue %d of %d import runs", failures, len(due))
	}

	return rescued, nil
}

func (s *importsService) parked(
	ctx context.Context,
	run entity.ImportRun,
	now time.Time,
) (bool, error) {
	cursors, err := s.cursors.List(ctx, run.ID)
	if err != nil {
		return false, err
	}

	for _, cursor := range cursors {
		if cursor.Parked(now) {
			return true, nil
		}
	}

	return false, nil
}

func (s *importsService) redrive(ctx context.Context, run entity.ImportRun, now time.Time) error {
	switch run.Status {
	case entity.ImportStaging:
		return s.jobs.EnqueueImportStage(ctx, entity.ImportStagePayload{
			ImportRunID: run.ID,
			WorkspaceID: run.WorkspaceID,
			Attempt:     run.Attempt + 1,
		}, now)

	case entity.ImportExecuting:
		return s.jobs.EnqueueImportExecute(ctx, entity.ImportExecutePayload{
			ImportRunID: run.ID,
			WorkspaceID: run.WorkspaceID,
			Attempt:     run.Attempt + 1,
		}, now)

	case entity.ImportReverting:
		return s.jobs.EnqueueImportRevert(ctx, entity.ImportRevertPayload{
			ImportRunID: run.ID,
			WorkspaceID: run.WorkspaceID,
			Attempt:     run.Attempt + 1,
		}, now)

	default:
		return nil
	}
}

func (s *importsService) warnRescue(ctx context.Context, run entity.ImportRun, cause error) {
	logging.From(ctx).WarnContext(ctx, "re-driving a stalled import failed",
		"import_run_id", run.ID.String(),
		"status", string(run.Status),
		"error", cause.Error(),
	)
}

func (s *importsService) Sweep(ctx context.Context) (int, error) {
	return s.records.Purge(ctx, time.Now().UTC().Add(-s.cfg.RecordRetention), s.cfg.RescueBatch)
}
