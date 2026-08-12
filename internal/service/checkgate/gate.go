package checkgate

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

type Gate struct {
	checks    repository.Check
	evidence  repository.CheckEvidence
	codeLinks repository.CodeLink
}

func New(
	checks repository.Check,
	evidence repository.CheckEvidence,
	codeLinks repository.CodeLink,
) *Gate {
	return &Gate{checks: checks, evidence: evidence, codeLinks: codeLinks}
}

func (g *Gate) Blocking(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Check, error) {
	unproven, _, err := g.Obstructing(ctx, workspaceID, issueID)

	return unproven, err
}

func (g *Gate) Obstructing(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Check, []entity.Check, error) {
	checks, err := g.checks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return nil, nil, err
	}

	if len(checks) == 0 {
		return nil, nil, nil
	}

	evidence, err := g.evidence.Digest(ctx, workspaceID, issueID)
	if err != nil {
		return nil, nil, err
	}

	links, err := g.codeLinks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return nil, nil, err
	}

	reports := entity.ReportChecks(checks, evidence, entity.EvidenceHorizon{
		Now:   time.Now().UTC(),
		Heads: entity.HeadsOf(links),
	})

	return entity.BlockingChecks(reports), entity.UnratifiedChecks(reports), nil
}
