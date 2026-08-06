package imports

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *importsService) Catalogue(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (entity.ImportCatalogue, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return entity.ImportCatalogue{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	source, err := s.sources.Lookup(run.SourceKind)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	probe, answers := source.(service.ImportSourceProbe)
	if !answers {
		return entity.ImportCatalogue{}, nil
	}

	config, err := s.sourceConfig(ctx, run.ID)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	return probe.Probe(ctx, config)
}
