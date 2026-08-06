package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

func (s *importsService) Stage(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return entity.ImportRun{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if !run.Status.CanTransitionTo(entity.ImportStaging) {
		return entity.ImportRun{}, entity.ErrImportStatusTransition
	}

	if err := s.jobs.EnqueueImportStage(ctx, entity.ImportStagePayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt,
	}, time.Now().UTC()); err != nil {
		return entity.ImportRun{}, err
	}

	return run, nil
}

func (s *importsService) RunStage(ctx context.Context, payload entity.ImportStagePayload) error {
	run, err := s.runs.GetByID(ctx, payload.WorkspaceID, payload.ImportRunID)
	if err != nil {
		return err
	}

	if run.Status.Settled() || run.Status == entity.ImportStaged {
		return nil
	}

	token, _, err := s.lease(ctx, run, entity.ImportDraft, entity.ImportStaging)
	if err != nil {
		if errors.Is(err, entity.ErrImportRunLeased) {
			return nil
		}

		return err
	}

	source, err := s.sources.Lookup(run.SourceKind)
	if err != nil {
		return s.fail(ctx, run.ID, err)
	}

	pause, err := s.fetchWhileLeased(ctx, run, token, source)
	if err != nil {
		return s.settleStaging(ctx, run, token, err)
	}

	if pause == nil {
		return s.runs.Settle(ctx, run.ID, entity.ImportStaged, "", time.Now().UTC())
	}

	return s.resumeStaging(ctx, run, token, *pause)
}

type stagingPause struct {
	resource    entity.ImportResource
	resumeAt    time.Time
	rateLimited bool
}

func (s *importsService) fetchWhileLeased(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	source service.ImportSource,
) (*stagingPause, error) {
	reached, err := s.cursors.List(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	progress := make(map[entity.ImportResource]entity.ImportCursor, len(reached))

	for _, cursor := range reached {
		progress[cursor.Resource] = cursor
	}

	started := time.Now()

	for _, resource := range source.Resources() {
		if !resource.Fetched() {
			continue
		}

		cursor := progress[resource]
		cursor.RunID = run.ID
		cursor.Resource = resource

		if cursor.Exhausted {
			continue
		}

		if cursor.Parked(time.Now().UTC()) {
			return &stagingPause{resource: resource, resumeAt: *cursor.RetryAfter, rateLimited: true}, nil
		}

		for !cursor.Exhausted {
			if time.Since(started) >= s.cfg.SliceBudget {
				return &stagingPause{resource: resource, resumeAt: time.Now().UTC()}, nil
			}

			if err := s.beat(ctx, run.ID, token); err != nil {
				return nil, err
			}

			page, err := source.Fetch(ctx, service.ImportFetchRequest{
				Resource: resource,
				Cursor:   cursor.Cursor,
				PageHint: s.cfg.PageSize,
			})
			if err != nil {
				var limited entity.ImportRateLimitedError

				if errors.As(err, &limited) {
					waited := entity.ClampImportBackoff(limited.RetryAfter, s.cfg.MinBackoff, s.cfg.MaxBackoff)

					return &stagingPause{
						resource:    resource,
						resumeAt:    time.Now().UTC().Add(waited),
						rateLimited: true,
					}, nil
				}

				return nil, err
			}

			cursor.Cursor = page.NextCursor
			cursor.Exhausted = page.Done
			cursor.Fetched += len(page.Records)
			cursor.RetryAfter = nil
			cursor.UpdatedAt = time.Now().UTC()

			staging, err := addressed(run.ID, resource, page.Records)
			if err != nil {
				return nil, err
			}

			linked, err := parentLinks(run.ID, staging)
			if err != nil {
				return nil, err
			}

			if err := s.recordPage(ctx, run, cursor, append(staging, linked...)); err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}

func (s *importsService) settleStaging(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	cause error,
) error {
	var refused entity.ImportSourceRefusedError

	if errors.As(cause, &refused) || run.Attempt >= s.cfg.MaxAttempts {
		return s.fail(ctx, run.ID, cause)
	}

	if err := s.runs.Release(ctx, run.ID, token, time.Now().UTC()); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}

func (s *importsService) resumeStaging(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	pause stagingPause,
) error {
	if pause.rateLimited {
		if err := s.cursors.Park(ctx, run.ID, pause.resource, pause.resumeAt); err != nil {
			return err
		}

		logging.From(ctx).InfoContext(ctx, "import staging is waiting on a source rate limit",
			"import_run_id", run.ID.String(),
			"resource", string(pause.resource),
			"resume_at", pause.resumeAt.Format(time.RFC3339),
		)
	}

	if err := s.runs.Release(ctx, run.ID, token, time.Now().UTC()); err != nil {
		return err
	}

	return s.jobs.EnqueueImportStage(ctx, entity.ImportStagePayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt + 1,
	}, pause.resumeAt)
}

func (s *importsService) recordPage(
	ctx context.Context,
	run entity.ImportRun,
	cursor entity.ImportCursor,
	records []entity.ImportRecord,
) error {
	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		staged, err := s.records.Stage(ctx, run.ID, records)
		if err != nil {
			return fmt.Errorf("stage import records: %w", err)
		}

		if err := s.cursors.Save(ctx, cursor); err != nil {
			return err
		}

		return s.runs.Advance(ctx, run.ID, staged, 0)
	})
}

// parentLinks turns the parent every issue names into a record of its own, so that the
// phase a revert unpicks first exists without an adapter having to send the same link
// twice. The child names one parent at most, which is why its own identifier addresses
// the link: a page read again upserts onto the row it wrote before rather than a second.
func parentLinks(runID uuid.UUID, records []entity.ImportRecord) ([]entity.ImportRecord, error) {
	links := make([]entity.ImportRecord, 0, len(records))

	for _, record := range records {
		if record.Resource != entity.ImportIssue || strings.TrimSpace(record.ParentExternalID) == "" {
			continue
		}

		payload, err := json.Marshal(service.ImportIssueParentPayload{
			Issue:  record.ExternalID,
			Parent: record.ParentExternalID,
		})
		if err != nil {
			return nil, fmt.Errorf("encode parent link for %q: %w", record.ExternalID, err)
		}

		links = append(links, entity.ImportRecord{
			RunID:           runID,
			Resource:        entity.ImportIssueParent,
			ExternalID:      record.ExternalID,
			Payload:         payload,
			SourceCreatedAt: record.SourceCreatedAt,
			SourceUpdatedAt: record.SourceUpdatedAt,
			State:           entity.ImportRecordStaged,
		})
	}

	return links, nil
}

func addressed(
	runID uuid.UUID,
	resource entity.ImportResource,
	records []entity.ImportRecord,
) ([]entity.ImportRecord, error) {
	staging := make([]entity.ImportRecord, 0, len(records))

	for position, record := range records {
		if record.ExternalID == "" {
			return nil, fmt.Errorf(
				"import source returned a %s at position %d with no identifier of its own; "+
					"preserving original identifiers is what makes a re-run safe and a revert possible, "+
					"so a row that cannot be named cannot be carried",
				resource, position,
			)
		}

		record.RunID = runID
		record.Resource = resource
		record.State = entity.ImportRecordStaged

		staging = append(staging, record)
	}

	return staging, nil
}
