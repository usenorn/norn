package scm

import (
	"context"
	"errors"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *sync) refreshReleases(ctx context.Context, from source, target entity.SCMTarget) error {
	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	if !from.connection.Capabilities.Has(entity.CapabilityReleases) {
		return nil
	}

	found, err := forge.Releases(ctx, target, s.cfg.ReconcileBatch)
	if err != nil {
		if errors.Is(err, entity.ErrSCMCapabilityUnsupported) {
			return nil
		}

		return s.handleForgeError(ctx, from, err)
	}

	stored := make(entity.SCMReleases, 0, len(found))

	for _, release := range found {
		one, err := s.releases.Upsert(ctx, entity.SCMRelease{
			RepositoryID: from.repository.ID,
			WorkspaceID:  from.workspaceID(),
			ExternalID:   release.ExternalID,
			Tag:          release.Tag,
			Name:         release.Name,
			URL:          release.URL,
			CommitSHA:    release.CommitSHA,
			Prerelease:   release.Prerelease,
			PublishedAt:  release.PublishedAt,
		})
		if err != nil {
			return err
		}

		stored = append(stored, one)
	}

	for _, release := range stored {
		if err := s.recordShipped(ctx, from, target, forge, stored, release); err != nil {
			return err
		}
	}

	return nil
}

func (s *sync) recordShipped(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
	forge service.Forge,
	all entity.SCMReleases,
	release entity.SCMRelease,
) error {
	previous, found := all.Previous(release)
	if !found {
		return nil
	}

	commits, err := forge.ReleaseCommits(ctx, target, previous.Tag, release.Tag)
	if err != nil {
		if errors.Is(err, entity.ErrSCMCapabilityUnsupported) {
			return nil
		}

		return s.handleForgeError(ctx, from, err)
	}

	if len(commits) == 0 {
		return nil
	}

	links, err := s.links.ListByRepository(ctx, from.repository.ID)
	if err != nil {
		return err
	}

	shipped := entity.MatchReleaseCommits(commits, links)
	if len(shipped) == 0 {
		return nil
	}

	return s.releases.LinkChanges(ctx, release.ID, shipped)
}

func (s *sync) refreshDeployments(
	ctx context.Context,
	from source,
	target entity.SCMTarget,
) error {
	if !from.connection.Capabilities.Has(entity.CapabilityDeployments) {
		return nil
	}

	forge, err := s.forges.Lookup(from.connection.Provider)
	if err != nil {
		return err
	}

	found, err := forge.Deployments(ctx, target, s.cfg.ReconcileBatch)
	if err != nil {
		if errors.Is(err, entity.ErrSCMCapabilityUnsupported) {
			return nil
		}

		return s.handleForgeError(ctx, from, err)
	}

	for _, deployment := range found {
		if err := s.deployments.Upsert(ctx, entity.SCMDeployment{
			RepositoryID: from.repository.ID,
			WorkspaceID:  from.workspaceID(),
			ExternalID:   deployment.ExternalID,
			Environment:  deployment.Environment,
			State:        deployment.State,
			URL:          deployment.URL,
			CommitSHA:    deployment.CommitSHA,
			OccurredAt:   deployment.OccurredAt,
		}); err != nil {
			return err
		}
	}

	return nil
}
