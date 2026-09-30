package changeset

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/postgres"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *changeSetsService) Published(
	ctx context.Context,
	runner entity.Runner,
	message entity.ChannelMessage,
) error {
	execution, err := s.executions.Held(ctx, runner, message.ExecutionID)
	if err != nil {
		return err
	}

	var incoming channelv1.Publication

	if err := decode(message.Payload, &incoming); err != nil {
		return err
	}

	if len(incoming.Repos) > entity.ExecutionChangesMax {
		return entity.ErrChannelEnvelopeInvalid
	}

	approved, err := s.snapshots.Latest(ctx, execution.ID)
	if err != nil {
		return err
	}

	published, err := publicationsOf(approved, incoming, reportedAt(message))
	if errors.Is(err, entity.ErrPublicationStale) || errors.Is(err, entity.ErrPublicationUnapproved) {
		logging.From(ctx).WarnContext(
			ctx,
			"a runner reported a publication that does not match the approved revision",
			slog.String("execution_id", execution.ID),
			slog.Int("approved_revision", approved.Revision),
			slog.Int("reported_revision", incoming.Revision),
			slog.String("reason", err.Error()),
		)

		return nil
	}

	if err != nil {
		return err
	}

	saved := make([]entity.ExecutionChange, 0, len(published))

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		saved = saved[:0]

		for _, repository := range published {
			stored, err := s.changesets.SavePublication(ctx, execution.ID, repository)
			if err != nil {
				return err
			}

			saved = append(saved, stored)
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

func publicationsOf(
	approved entity.ExecutionSnapshot,
	incoming channelv1.Publication,
	reported time.Time,
) ([]entity.RepositoryPublication, error) {
	if incoming.Revision != approved.Revision {
		return nil, entity.ErrPublicationStale
	}

	heads := approved.Heads()
	published := make([]entity.RepositoryPublication, 0, len(incoming.Repos))

	for index, repo := range incoming.Repos {
		repository := entity.RepositoryPublication{
			Repository:     repo.Repository,
			PullRequestURL: repo.PullRequest,
			Publication: entity.ExecutionPublication{
				State:    entity.PublicationState(repo.State),
				Step:     entity.PublicationStep(repo.Step),
				Error:    repo.Failure,
				SHA:      repo.SHA,
				Revision: incoming.Revision,
			},
		}

		if err := entity.ValidateRepositoryPublication(
			indexed("repos", index), repository,
		); err != nil {
			return nil, err
		}

		if head, ok := heads[repo.Repository]; !ok || head != repo.SHA {
			return nil, entity.ErrPublicationUnapproved
		}

		if repository.Publication.State == entity.PublicationPushed ||
			repository.Publication.State == entity.PublicationPublished {
			repository.Publication.PublishedAt = &reported
		}

		published = append(published, repository)
	}

	return published, nil
}
