package issuefilterreference

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type issueFilterReferenceRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueFilterReference {
	return &issueFilterReferenceRepository{db: db}
}

const (
	kindTeam    = "team"
	kindState   = "state"
	kindLabel   = "label"
	kindProject = "project"
	kindCycle   = "cycle"
	kindAccount = "account"
)

const resolveReferencesQuery = `
SELECT 'team' AS kind, t.id::text AS id,
       CASE WHEN $3::boolean IS TRUE OR t.id = ANY($4::uuid[]) THEN t.name ELSE '' END AS name,
       ($3::boolean IS TRUE OR t.id = ANY($4::uuid[])) AS visible
FROM workspace_teams t
WHERE t.workspace_id = $1 AND t.id = ANY($2::uuid[])

UNION ALL
SELECT 'state', s.id::text,
       CASE WHEN $3::boolean IS TRUE OR s.team_id = ANY($4::uuid[]) THEN s.name ELSE '' END,
       ($3::boolean IS TRUE OR s.team_id = ANY($4::uuid[]))
FROM workspace_workflow_states s
WHERE s.workspace_id = $1 AND s.id = ANY($2::uuid[])

UNION ALL
SELECT 'label', l.id::text,
       CASE WHEN $3::boolean IS TRUE OR l.team_id IS NULL OR l.team_id = ANY($4::uuid[])
            THEN l.name ELSE '' END,
       ($3::boolean IS TRUE OR l.team_id IS NULL OR l.team_id = ANY($4::uuid[]))
FROM workspace_labels l
WHERE l.workspace_id = $1 AND l.id = ANY($2::uuid[])

UNION ALL
SELECT 'cycle', c.id::text,
       CASE WHEN $3::boolean IS TRUE OR c.team_id = ANY($4::uuid[])
            THEN ct.key || ' ' || c.number::text ELSE '' END,
       ($3::boolean IS TRUE OR c.team_id = ANY($4::uuid[]))
FROM workspace_cycles c
JOIN workspace_teams ct ON ct.id = c.team_id
WHERE c.workspace_id = $1 AND c.id = ANY($2::uuid[])

UNION ALL
SELECT 'project', p.id::text, p.name, TRUE
FROM workspace_projects p
WHERE p.workspace_id = $1 AND p.id = ANY($2::uuid[])

UNION ALL
SELECT 'account', m.account_id::text, coalesce(a.display_name, ''), TRUE
FROM workspace_memberships m
JOIN accounts a ON a.id = m.account_id
WHERE m.workspace_id = $1 AND m.account_id = ANY($2::uuid[])`

var referenceKinds = map[entity.IssueFilterField]string{
	entity.IssueFilterFieldTeam:     kindTeam,
	entity.IssueFilterFieldState:    kindState,
	entity.IssueFilterFieldLabel:    kindLabel,
	entity.IssueFilterFieldProject:  kindProject,
	entity.IssueFilterFieldCycle:    kindCycle,
	entity.IssueFilterFieldAssignee: kindAccount,
	entity.IssueFilterFieldCreator:  kindAccount,
}

type resolution struct {
	name    string
	visible bool
}

func (r *issueFilterReferenceRepository) Resolve(
	ctx context.Context,
	workspaceID uuid.UUID,
	scope entity.TeamScope,
	wanted []entity.IssueFilterReference,
) ([]entity.IssueFilterReference, error) {
	if len(wanted) == 0 {
		return wanted, nil
	}

	ids := make([]string, 0, len(wanted))
	seen := make(map[string]bool, len(wanted))

	for _, reference := range wanted {
		if seen[reference.Value] {
			continue
		}

		seen[reference.Value] = true

		ids = append(ids, reference.Value)
	}

	teams := make([]string, 0, len(scope.TeamIDs))
	for _, id := range scope.TeamIDs {
		teams = append(teams, id.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, resolveReferencesQuery, workspaceID.String(), ids, scope.AllTeams, teams,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve filter references: %w", err)
	}

	defer func() { _ = rows.Close() }()

	found := make(map[string]resolution, len(ids))

	for rows.Next() {
		var (
			kind, id string
			resolved resolution
		)

		if err := rows.Scan(&kind, &id, &resolved.name, &resolved.visible); err != nil {
			return nil, fmt.Errorf("scan filter reference: %w", err)
		}

		found[kind+"\x00"+id] = resolved
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read filter references: %w", err)
	}

	resolvedReferences := make([]entity.IssueFilterReference, 0, len(wanted))

	for _, reference := range wanted {
		match, ok := found[referenceKinds[reference.Field]+"\x00"+reference.Value]

		switch {
		case !ok:
			reference.State = entity.IssueFilterReferenceMissing
		case match.visible:
			reference.State = entity.IssueFilterReferenceResolved
			reference.Name = match.name
		default:
			reference.State = entity.IssueFilterReferenceRestricted
		}

		resolvedReferences = append(resolvedReferences, reference)
	}

	return resolvedReferences, nil
}
