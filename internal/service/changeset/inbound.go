package changeset

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const linkedByRun = "a run opened it"

func (s *changeSetsService) Updated(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.executions.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.ChangeSet

	if err := decode(message.Payload, &incoming); err != nil {
		return err
	}

	return s.record(ctx, execution, incoming, entity.ExecutionResult{}, false, reportedAt(message))
}

func (s *changeSetsService) Resulted(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.executions.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.Result

	if err := decode(message.Payload, &incoming); err != nil {
		return err
	}

	if err := entity.NewValidationError(
		entity.ValidateExecutionSummary("summary", incoming.Summary),
	); err != nil {
		return err
	}

	reported := reportedAt(message)
	if !incoming.Reported.IsZero() {
		reported = incoming.Reported.UTC()
	}

	result := entity.ExecutionResult{
		ExecutionID: execution.ID,
		WorkspaceID: execution.WorkspaceID,
		Summary:     incoming.Summary,
		ReportedAt:  reported,
	}

	return s.record(ctx, execution, incoming.ChangeSet, result, true, reported)
}

func (s *changeSetsService) record(
	ctx context.Context,
	execution entity.Execution,
	incoming channelv1.ChangeSet,
	result entity.ExecutionResult,
	final bool,
	reported time.Time,
) error {
	changes, validations, err := convert(execution, incoming, reported)
	if err != nil {
		return err
	}

	var saved []entity.ExecutionChange

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if final {
			if _, err := s.changesets.SaveResult(ctx, result); err != nil {
				return err
			}
		}

		saved = saved[:0]

		for _, change := range changes {
			stored, err := s.changesets.SaveChange(ctx, change)
			if err != nil {
				return err
			}

			saved = append(saved, stored)
		}

		for _, validation := range validations {
			if _, err := s.changesets.SaveValidation(ctx, validation); err != nil {
				return err
			}
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) { s.announce(ctx, execution) })

		return nil
	})
	if err != nil {
		return err
	}

	s.link(ctx, execution, saved)

	return nil
}

func convert(
	execution entity.Execution,
	incoming channelv1.ChangeSet,
	reported time.Time,
) ([]entity.ExecutionChange, []entity.ExecutionValidation, error) {
	if len(incoming.Repos) > entity.ExecutionChangesMax ||
		len(incoming.Validation) > entity.ExecutionValidationsMax {
		return nil, nil, entity.ErrChannelEnvelopeInvalid
	}

	changes := make([]entity.ExecutionChange, 0, len(incoming.Repos))

	for index, repo := range incoming.Repos {
		artifactID, err := uuid.Parse(repo.Diff)
		if repo.Diff != "" && err != nil {
			return nil, nil, entity.NewValidationError(entity.FieldError{
				Field: field("repos", index, "diffArtifactId"),
				Code:  entity.ValidationCodeMalformed,
			})
		}

		change := entity.ExecutionChange{
			ExecutionID:    execution.ID,
			WorkspaceID:    execution.WorkspaceID,
			Repository:     repo.Repository,
			Branch:         repo.Branch,
			BaseSHA:        repo.BaseSHA,
			HeadSHA:        repo.HeadSHA,
			Commits:        repo.Commits,
			Additions:      repo.Additions,
			Deletions:      repo.Deletions,
			FilesChanged:   repo.Files,
			DiffArtifactID: artifactID,
			PullRequestURL: repo.PullRequest,
			ReportedAt:     reported,
		}

		if err := entity.ValidateExecutionChange(indexed("repos", index), change); err != nil {
			return nil, nil, err
		}

		changes = append(changes, change)
	}

	validations := make([]entity.ExecutionValidation, 0, len(incoming.Validation))

	for index, reported := range incoming.Validation {
		artifactID, err := uuid.Parse(reported.Artifact)
		if reported.Artifact != "" && err != nil {
			return nil, nil, entity.NewValidationError(entity.FieldError{
				Field: field("validation", index, "artifactId"),
				Code:  entity.ValidationCodeMalformed,
			})
		}

		validation := entity.ExecutionValidation{
			ExecutionID: execution.ID,
			WorkspaceID: execution.WorkspaceID,
			Check:       reported.Check,
			Status:      entity.ValidationStatus(reported.Status),
			Detail:      reported.Detail,
			ArtifactID:  artifactID,
		}

		if err := entity.ValidateExecutionValidation(
			indexed("validation", index), validation,
		); err != nil {
			return nil, nil, err
		}

		validations = append(validations, validation)
	}

	return changes, stamp(validations, reported), nil
}

func stamp(
	validations []entity.ExecutionValidation,
	reported time.Time,
) []entity.ExecutionValidation {
	for index := range validations {
		validations[index].ReportedAt = reported
	}

	return validations
}

func (s *changeSetsService) link(
	ctx context.Context,
	execution entity.Execution,
	changes []entity.ExecutionChange,
) {
	for _, change := range changes {
		if change.PullRequestURL == "" || change.CodeLinkID != uuid.Nil {
			continue
		}

		linked, err := s.source.Link(
			ctx,
			execution.WorkspaceID,
			execution.IssueID,
			service.LinkIssueCodeInput{URL: change.PullRequestURL, DetectedIn: linkedByRun},
		)
		if err != nil {
			s.unlinkable(ctx, execution, change, err)

			continue
		}

		if err := s.changesets.LinkChange(ctx, change.ID, linked.ID); err != nil {
			s.unlinkable(ctx, execution, change, err)
		}
	}
}

func (s *changeSetsService) unlinkable(
	ctx context.Context,
	execution entity.Execution,
	change entity.ExecutionChange,
	cause error,
) {
	level := slog.LevelWarn
	if unreachable(cause) {
		level = slog.LevelInfo
	}

	logging.From(ctx).Log(
		ctx,
		level,
		"a pull request a run opened is recorded by its address alone, because norn does not "+
			"follow that repository",
		slog.String("execution_id", execution.ID),
		slog.String("repository", change.Repository),
		slog.String("url", change.PullRequestURL),
		slog.String("error", cause.Error()),
	)
}

func unreachable(cause error) bool {
	var (
		invalid entity.ValidationError
		denied  entity.AccessDeniedError
	)

	return errors.As(cause, &invalid) ||
		errors.As(cause, &denied) ||
		errors.Is(cause, entity.ErrSCMTeamOutsideConnection) ||
		errors.Is(cause, entity.ErrIssueNotFound)
}

func reportedAt(message entity.ChannelMessage) time.Time {
	if message.IssuedAt.IsZero() {
		return time.Now().UTC()
	}

	return message.IssuedAt.UTC()
}

func indexed(section string, index int) string {
	return section + "[" + strconv.Itoa(index) + "]"
}

func field(section string, index int, name string) string {
	return indexed(section, index) + "." + name
}
