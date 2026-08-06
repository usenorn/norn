package imports

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *importsService) Execute(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
	input service.ExecuteImportInput,
) (entity.ImportRun, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return entity.ImportRun{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if !run.Status.CanTransitionTo(entity.ImportExecuting) {
		return entity.ImportRun{}, entity.ErrImportStatusTransition
	}

	saved, err := s.mappings.List(ctx, run.ID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if !(entity.MappingPlan{Mappings: saved}).Complete() {
		return entity.ImportRun{}, entity.ErrImportUnmapped
	}

	preview, err := s.Preview(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if strings.TrimSpace(input.PreviewDigest) == "" {
		return entity.ImportRun{}, entity.ErrImportPreviewRequired
	}

	digest := preview.Digest()

	if input.PreviewDigest != digest {
		return entity.ImportRun{}, entity.ErrImportPreviewStale
	}

	if preview.WouldTriage() && !input.AcknowledgeTriage {
		return entity.ImportRun{}, entity.ImportWouldTriageError{Teams: preview.TriageTeams}
	}

	now := time.Now().UTC()

	if err := s.runs.AcceptPreview(ctx, run.ID, digest, input.AcknowledgeTriage, now); err != nil {
		return entity.ImportRun{}, err
	}

	if err := s.jobs.EnqueueImportExecute(ctx, entity.ImportExecutePayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt,
	}, now); err != nil {
		return entity.ImportRun{}, err
	}

	run.PreviewDigest = digest
	run.AcknowledgeTriage = input.AcknowledgeTriage

	return run, nil
}

func (s *importsService) RunExecute(ctx context.Context, payload entity.ImportExecutePayload) error {
	run, err := s.runs.GetByID(ctx, payload.WorkspaceID, payload.ImportRunID)
	if err != nil {
		return err
	}

	if run.Status.Settled() {
		return nil
	}

	token, _, err := s.lease(ctx, run, entity.ImportMapped, entity.ImportExecuting)
	if err != nil {
		if errors.Is(err, entity.ErrImportRunLeased) {
			return nil
		}

		return err
	}

	ctx = identity.WithActor(ctx, run.Requester())

	decision, err := s.decide(ctx, run.WorkspaceID)
	if err != nil {
		return s.fail(ctx, run.ID, err)
	}

	saved, err := s.mappings.List(ctx, run.ID)
	if err != nil {
		return s.settleSlice(ctx, run, token, err)
	}

	plan := entity.MappingPlan{Mappings: saved}

	if !plan.Complete() {
		return s.fail(ctx, run.ID, entity.ErrImportUnmapped)
	}

	done, err := s.applyWhileLeased(ctx, run, token, decision, plan)
	if err != nil {
		return s.settleSlice(ctx, run, token, err)
	}

	if done {
		return s.runs.Settle(ctx, run.ID, entity.ImportImported, "", time.Now().UTC())
	}

	return s.resumeExecuting(ctx, run, token)
}

func (s *importsService) applyWhileLeased(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	decision entity.Decision,
	plan entity.MappingPlan,
) (bool, error) {
	started := time.Now()
	log := newRecorder(entity.ImportPhaseExecute)

	for _, resource := range entity.ImportPhases() {
		after := int64(0)

		for {
			if time.Since(started) >= s.cfg.SliceBudget {
				return false, nil
			}

			if err := s.beat(ctx, run.ID, token); err != nil {
				return false, err
			}

			records, err := s.records.Walk(ctx, run.ID, entity.ImportWalk{
				Resource: resource,
				After:    after,
				Limit:    s.cfg.ChunkSize,
			})
			if err != nil {
				return false, err
			}

			if len(records) == 0 {
				break
			}

			if err := s.applyChunk(ctx, run, decision, plan, resource, records, log); err != nil {
				return false, err
			}

			after = records[len(records)-1].Seq
		}
	}

	return true, nil
}

func (s *importsService) settleSlice(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
	cause error,
) error {
	if run.Attempt >= s.cfg.MaxAttempts {
		return s.fail(ctx, run.ID, cause)
	}

	if err := s.runs.Release(ctx, run.ID, token, time.Now().UTC()); err != nil {
		return errors.Join(cause, err)
	}

	return cause
}

func (s *importsService) resumeExecuting(
	ctx context.Context,
	run entity.ImportRun,
	token uuid.UUID,
) error {
	if err := s.runs.Release(ctx, run.ID, token, time.Now().UTC()); err != nil {
		return err
	}

	return s.jobs.EnqueueImportExecute(ctx, entity.ImportExecutePayload{
		ImportRunID: run.ID,
		WorkspaceID: run.WorkspaceID,
		Attempt:     run.Attempt + 1,
	}, time.Now().UTC())
}
