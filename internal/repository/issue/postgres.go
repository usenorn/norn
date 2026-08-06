package issue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const issueColumns = `
       i.id,
       i.workspace_id,
       i.team_id,
       i.number,
       i.title,
       i.version,
       i.field_versions,
       i.description,
       i.priority,
       coalesce(i.assignee_account_id::text, ''),
       coalesce(i.estimate, 0),
       coalesce(to_char(i.due_on, 'YYYY-MM-DD'), ''),
       i.state_entered_at,
       i.completed_at,
       i.status,
       i.archived_at,
       coalesce(i.parent_issue_id::text, ''),
       coalesce(p.reference_key || '-' || p.number::text, ''),
       i.depth,
       coalesce(i.cycle_id::text, ''),
       coalesce(c.number, 0),
       coalesce(i.project_id::text, ''),
       coalesce(pr.name, ''),
       coalesce(i.created_by_account_id::text, ''),
       coalesce(i.triage_state, ''),
       coalesce(i.triage_source, ''),
       coalesce(i.triage_decided_by_account_id::text, ''),
       coalesce(td.display_name, ''),
       i.triage_decided_at,
       i.created_at,
       i.updated_at,
       t.key,
       i.reference_key,
       s.id,
       s.name,
       s.category,
       s.position`

const issueJoins = `
FROM workspace_issues i
JOIN workspace_teams t ON t.id = i.team_id
JOIN workspace_workflow_states s ON s.id = i.state_id AND s.team_id = i.team_id
LEFT JOIN workspace_issues p ON p.id = i.parent_issue_id
LEFT JOIN workspace_cycles c ON c.id = i.cycle_id
LEFT JOIN workspace_projects pr ON pr.id = i.project_id
LEFT JOIN accounts td ON td.id = i.triage_decided_by_account_id`

const visibleIssueQuery = `
SELECT` + issueColumns + issueJoins + `
WHERE i.id = $1
  AND i.workspace_id = $2
  AND ($3::boolean IS TRUE OR i.team_id = ANY($4::uuid[]))`

const issueByReferenceQuery = `
SELECT` + issueColumns + issueJoins + `
WHERE i.workspace_id = $1
  AND i.reference_key = $2
  AND i.number = $3
  AND ($4::boolean IS TRUE OR i.team_id = ANY($5::uuid[]))`

const insertIssueQuery = `
WITH allocated AS (
    INSERT INTO workspace_issue_numbers (team_id, next_number)
    VALUES ($3, 2)
    ON CONFLICT (team_id) DO UPDATE
        SET next_number = workspace_issue_numbers.next_number + 1
    RETURNING next_number - 1 AS number
), inserted AS (
    INSERT INTO workspace_issues (
        id, workspace_id, team_id, reference_key, number, title, state_id,
        created_by_account_id, description, priority, assignee_account_id,
        estimate, due_on, project_id, triage_state, triage_source, created_at, updated_at
    )
    SELECT $1, $2, $3, t.key, allocated.number, $4, $5, $6,
           $8, $9, nullif($10, '')::uuid, nullif($11, 0), nullif($12, '')::date,
           nullif($15, '')::uuid, nullif($13, ''), nullif($14, ''), $7, $16
    FROM allocated
    JOIN workspace_teams t ON t.id = $3
    RETURNING id, workspace_id, team_id, reference_key, number, title, state_id,
              version, field_versions, description, priority, assignee_account_id,
              estimate, due_on, state_entered_at, completed_at,
              status, archived_at,
              parent_issue_id, depth, cycle_id, project_id,
              created_by_account_id, triage_state, triage_source,
              triage_decided_by_account_id, triage_decided_at, created_at, updated_at
)
SELECT` + issueColumns + `
FROM inserted i
JOIN workspace_teams t ON t.id = i.team_id
JOIN workspace_workflow_states s ON s.id = i.state_id AND s.team_id = i.team_id
LEFT JOIN workspace_issues p ON p.id = i.parent_issue_id
LEFT JOIN workspace_cycles c ON c.id = i.cycle_id
LEFT JOIN workspace_projects pr ON pr.id = i.project_id
LEFT JOIN accounts td ON td.id = i.triage_decided_by_account_id`

const lockIssueQuery = `
SELECT` + issueColumns + issueJoins + `
WHERE i.id = $1
  AND i.workspace_id = $2
  AND ($3::boolean IS TRUE OR i.team_id = ANY($4::uuid[]))
FOR UPDATE OF i`

const updateIssueQuery = `
UPDATE workspace_issues
SET title               = coalesce($3, title),
    state_id            = coalesce($4::uuid, state_id),
    description         = coalesce($5, description),
    priority            = coalesce($6, priority),
    assignee_account_id = CASE WHEN $7::boolean THEN NULL
                               ELSE coalesce($8::uuid, assignee_account_id) END,
    estimate            = CASE WHEN $9::boolean THEN NULL
                               ELSE coalesce($10::integer, estimate) END,
    due_on              = CASE WHEN $11::boolean THEN NULL
                               ELSE coalesce($12::date, due_on) END,
    cycle_id            = CASE WHEN $18::boolean THEN NULL
                               ELSE coalesce($19::uuid, cycle_id) END,
    project_id          = CASE WHEN $20::boolean THEN NULL
                               ELSE coalesce($21::uuid, project_id) END,
    state_entered_at    = coalesce($13::timestamptz, state_entered_at),
    completed_at        = CASE WHEN $14::boolean THEN $15::timestamptz ELSE completed_at END,
    version             = version + 1,
    field_versions      = field_versions || $16::jsonb,
    updated_at          = $17
WHERE id = $1 AND version = $2`

const dropTeamScopedLabelsQuery = `
DELETE FROM workspace_issue_labels
WHERE issue_id = $1 AND label_team_id IS NOT NULL`

const moveIssueTeamQuery = `
UPDATE workspace_issues
SET team_id          = $3,
    state_id         = $4,
    cycle_id         = NULL,
    state_entered_at = $5,
    completed_at     = $6,
    version          = version + 1,
    field_versions   = field_versions || $7::jsonb,
    updated_at       = $8
WHERE id = $1 AND version = $2`

const moveIssuesToCycleQuery = `
UPDATE workspace_issues
SET cycle_id       = $2::uuid,
    version        = version + 1,
    field_versions = field_versions || jsonb_build_object('cycle', version + 1),
    updated_at     = $3
WHERE id = ANY($1::uuid[])`

const reassignStateQuery = `
UPDATE workspace_issues i
SET state_id         = $2,
    state_entered_at = CASE WHEN src.category = dst.category
                            THEN i.state_entered_at ELSE $3 END,
    completed_at     = CASE
                           WHEN dst.category <> 'complete' THEN NULL
                           WHEN src.category = 'complete'  THEN i.completed_at
                           ELSE $3
                       END,
    version          = i.version + 1,
    field_versions   = i.field_versions || jsonb_build_object('state', i.version + 1),
    updated_at       = $3
FROM workspace_workflow_states src, workspace_workflow_states dst
WHERE i.state_id = $1 AND src.id = $1 AND dst.id = $2`

const setIssueStatusQuery = `
UPDATE workspace_issues
SET status                = $3,
    archived_at           = $4,
    deletion_requested_at = $5,
    purge_after           = $6,
    version               = version + 1,
    field_versions        = field_versions || jsonb_build_object('status', version + 1),
    updated_at            = $7
WHERE id = $1 AND version = $2`

const rerootChildrenQuery = `
WITH RECURSIVE subtree AS (
    SELECT id, 1 AS depth
    FROM workspace_issues
    WHERE parent_issue_id = $1
      AND EXISTS (
          SELECT 1 FROM workspace_issues due
          WHERE due.id = $1 AND due.status = 'pending_deletion' AND due.purge_after <= $2
      )
    UNION ALL
    SELECT i.id, s.depth + 1
    FROM workspace_issues i
    JOIN subtree s ON i.parent_issue_id = s.id
    WHERE s.depth < $3
)
UPDATE workspace_issues i
SET parent_issue_id = CASE WHEN i.parent_issue_id = $1 THEN NULL ELSE i.parent_issue_id END,
    depth           = s.depth,
    updated_at      = $2
FROM subtree s
WHERE i.id = s.id`

const purgeIssueQuery = `
DELETE FROM workspace_issues
WHERE id = $1 AND status = 'pending_deletion' AND purge_after <= $2`

const stampLabelsQuery = `
UPDATE workspace_issues
SET version        = version + 1,
    field_versions = field_versions || jsonb_build_object('labels', version + 1),
    updated_at     = $3
WHERE id = $1 AND version = $2`

const ancestorsQuery = `
WITH RECURSIVE ancestry AS (
    SELECT id, parent_issue_id, 1 AS step
    FROM workspace_issues
    WHERE id = $1
    UNION ALL
    SELECT i.id, i.parent_issue_id, a.step + 1
    FROM workspace_issues i
    JOIN ancestry a ON i.id = a.parent_issue_id
    WHERE a.step <= $2
)
SELECT id FROM ancestry WHERE id <> $1`

const subtreeHeightQuery = `
WITH RECURSIVE subtree AS (
    SELECT id, 0 AS height
    FROM workspace_issues
    WHERE id = $1
    UNION ALL
    SELECT i.id, s.height + 1
    FROM workspace_issues i
    JOIN subtree s ON i.parent_issue_id = s.id
    WHERE s.height <= $2
)
SELECT max(height) FROM subtree`

const setParentQuery = `
UPDATE workspace_issues
SET parent_issue_id = $3::uuid,
    depth           = $4,
    version         = version + 1,
    field_versions  = field_versions || jsonb_build_object('parent', version + 1),
    updated_at      = $5
WHERE id = $1 AND version = $2`

const rewriteSubtreeDepthQuery = `
WITH RECURSIVE subtree AS (
    SELECT id, $2::integer + 1 AS depth
    FROM workspace_issues
    WHERE parent_issue_id = $1
    UNION ALL
    SELECT i.id, s.depth + 1
    FROM workspace_issues i
    JOIN subtree s ON i.parent_issue_id = s.id
    WHERE s.depth < $3
)
UPDATE workspace_issues i
SET depth = s.depth, updated_at = $4
FROM subtree s
WHERE i.id = s.id AND i.depth <> s.depth`

const childrenQuery = `
SELECT` + issueColumns + issueJoins + `
WHERE i.parent_issue_id = $1
  AND i.workspace_id = $2
  AND ($3::boolean IS TRUE OR i.team_id = ANY($4::uuid[]))
ORDER BY i.created_at, i.id`

const blockedQuery = `
SELECT DISTINCT r.target_issue_id
FROM workspace_issue_relations r
JOIN workspace_issues b ON b.id = r.source_issue_id
JOIN workspace_workflow_states bs ON bs.id = b.state_id AND bs.team_id = b.team_id
WHERE r.kind = 'blocks'
  AND r.target_issue_id = ANY($1::uuid[])
  AND b.status = 'active'
  AND bs.category IN ('not_started', 'active')`

const progressQuery = `
SELECT coalesce(i.parent_issue_id::text, ''), s.category, count(*)
FROM workspace_issues i
JOIN workspace_workflow_states s ON s.id = i.state_id AND s.team_id = i.team_id
WHERE i.workspace_id = $1
  AND ($2::boolean IS TRUE OR i.team_id = ANY($3::uuid[]))
  AND ($4::boolean IS NOT TRUE OR i.team_id = $5::uuid)
  AND ($6::boolean IS NOT TRUE OR i.parent_issue_id = ANY($7::uuid[]))
  AND ($8::boolean IS NOT TRUE OR i.cycle_id = $9::uuid)
  AND ($10::boolean IS NOT TRUE OR i.project_id = $11::uuid)
  AND i.status = 'active'
  AND i.triage_state IS DISTINCT FROM 'waiting'
GROUP BY i.parent_issue_id, s.category`

const issueLabelsQuery = `
SELECT il.issue_id,
       l.id,
       l.workspace_id,
       coalesce(l.team_id::text, ''),
       coalesce(l.group_id::text, ''),
       l.name,
       l.color
FROM workspace_issue_labels il
JOIN workspace_labels l ON l.id = il.label_id
WHERE il.issue_id = ANY($1::uuid[])
ORDER BY il.issue_id, lower(l.name), l.id`

const (
	uniqueViolationCode  = "23505"
	referenceUniqueIndex = "workspace_issues_reference_key"
)

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == referenceUniqueIndex {
		return entity.ErrIssueReferenceTaken
	}

	return err
}

type issueRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Issue {
	return &issueRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIssue(row scanner) (entity.Issue, error) {
	var (
		issue     entity.Issue
		id        string
		workspace string
		team      string
		createdBy string
		stateID   string
		category  string
		priority  string
		assignee  string

		status        string
		parent        string
		cycle         string
		project       string
		completedAt   sql.NullTime
		archivedAt    sql.NullTime
		fieldVersions []byte

		triageState   string
		triageSource  string
		triageDecider string
		triageAt      sql.NullTime
	)

	if err := row.Scan(
		&id,
		&workspace,
		&team,
		&issue.Number,
		&issue.Title,
		&issue.Version,
		&fieldVersions,
		&issue.Description,
		&priority,
		&assignee,
		&issue.Estimate,
		&issue.DueOn,
		&issue.StateEnteredAt,
		&completedAt,
		&status,
		&archivedAt,
		&parent,
		&issue.ParentReference,
		&issue.Depth,
		&cycle,
		&issue.CycleNumber,
		&project,
		&issue.ProjectName,
		&createdBy,
		&triageState,
		&triageSource,
		&triageDecider,
		&issue.TriageDecidedName,
		&triageAt,
		&issue.CreatedAt,
		&issue.UpdatedAt,
		&issue.TeamKey,
		&issue.ReferenceKey,
		&stateID,
		&issue.State.Name,
		&category,
		&issue.State.Position,
	); err != nil {
		return entity.Issue{}, err
	}

	issue.State.Category = entity.StateCategory(category)
	issue.Priority = entity.IssuePriority(priority)
	issue.Status = entity.IssueStatus(status)
	issue.TriageState = entity.TriageState(triageState)
	issue.TriageSource = entity.ActorKind(triageSource)

	if triageAt.Valid {
		decided := triageAt.Time
		issue.TriageDecidedAt = &decided
	}

	if archivedAt.Valid {
		shelved := archivedAt.Time
		issue.ArchivedAt = &shelved
	}

	if completedAt.Valid {
		finished := completedAt.Time
		issue.CompletedAt = &finished
	}

	if len(fieldVersions) > 0 {
		if err := json.Unmarshal(fieldVersions, &issue.FieldVersions); err != nil {
			return entity.Issue{}, fmt.Errorf("decode issue field versions: %w", err)
		}
	}

	if issue.FieldVersions == nil {
		issue.FieldVersions = map[string]int{}
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Issue{}, fmt.Errorf("parse issue id: %w", err)
	}

	issue.ID = parsed

	if issue.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Issue{}, fmt.Errorf("parse issue workspace id: %w", err)
	}

	if issue.TeamID, err = uuid.Parse(team); err != nil {
		return entity.Issue{}, fmt.Errorf("parse issue team id: %w", err)
	}

	if issue.State.ID, err = uuid.Parse(stateID); err != nil {
		return entity.Issue{}, fmt.Errorf("parse issue state id: %w", err)
	}

	if createdBy != "" {
		if issue.CreatedByAccountID, err = uuid.Parse(createdBy); err != nil {
			return entity.Issue{}, fmt.Errorf("parse issue author id: %w", err)
		}
	}

	if triageDecider != "" {
		if issue.TriageDecidedBy, err = uuid.Parse(triageDecider); err != nil {
			return entity.Issue{}, fmt.Errorf("parse triage decider id: %w", err)
		}
	}

	if assignee != "" {
		if issue.AssigneeAccountID, err = uuid.Parse(assignee); err != nil {
			return entity.Issue{}, fmt.Errorf("parse issue assignee id: %w", err)
		}
	}

	if parent != "" {
		if issue.ParentIssueID, err = uuid.Parse(parent); err != nil {
			return entity.Issue{}, fmt.Errorf("parse issue parent id: %w", err)
		}
	}

	if cycle != "" {
		if issue.CycleID, err = uuid.Parse(cycle); err != nil {
			return entity.Issue{}, fmt.Errorf("parse issue cycle id: %w", err)
		}
	}

	if project != "" {
		if issue.ProjectID, err = uuid.Parse(project); err != nil {
			return entity.Issue{}, fmt.Errorf("parse issue project id: %w", err)
		}
	}

	return issue, nil
}

func statusNames(statuses []entity.IssueStatus) []string {
	names := make([]string, 0, len(statuses))

	for _, status := range statuses {
		names = append(names, string(status))
	}

	return names
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *issueRepository) Create(ctx context.Context, issue entity.Issue) (entity.Issue, error) {
	if issue.ID == uuid.Nil {
		issue.ID = uuid.New()
	}

	createdAt, updatedAt := entity.OriginStamp(issue.Origin, time.Now().UTC())

	var author any

	if issue.CreatedByAccountID != uuid.Nil {
		author = issue.CreatedByAccountID.String()
	}

	created, err := scanIssue(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertIssueQuery,
		issue.ID.String(),
		issue.WorkspaceID.String(),
		issue.TeamID.String(),
		issue.Title,
		issue.State.ID.String(),
		author,
		createdAt,
		issue.Description,
		string(issue.Priority),
		text(issue.AssigneeAccountID),
		issue.Estimate,
		issue.DueOn,
		string(issue.TriageState),
		string(issue.TriageSource),
		text(issue.ProjectID),
		updatedAt,
	))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Issue{}, translated
		}

		return entity.Issue{}, fmt.Errorf("insert issue: %w", err)
	}

	return created, nil
}

func (r *issueRepository) GetVisible(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	scope entity.TeamScope,
) (entity.Issue, error) {
	issue, err := scanIssue(r.db.Querier(ctx).QueryRowContext(
		ctx,
		visibleIssueQuery,
		issueID.String(),
		workspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Issue{}, entity.ErrIssueNotFound
		}

		return entity.Issue{}, fmt.Errorf("find visible issue: %w", err)
	}

	hydrated := []entity.Issue{issue}
	if err := r.hydrate(ctx, scope, hydrated); err != nil {
		return entity.Issue{}, err
	}

	return hydrated[0], nil
}

func (r *issueRepository) GetVisibleByReference(
	ctx context.Context,
	workspaceID uuid.UUID,
	reference entity.IssueReference,
	scope entity.TeamScope,
) (entity.Issue, error) {
	issue, err := scanIssue(r.db.Querier(ctx).QueryRowContext(
		ctx,
		issueByReferenceQuery,
		workspaceID.String(),
		reference.Key,
		reference.Number,
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Issue{}, entity.ErrIssueNotFound
		}

		return entity.Issue{}, fmt.Errorf("find issue by reference: %w", err)
	}

	hydrated := []entity.Issue{issue}
	if err := r.hydrate(ctx, scope, hydrated); err != nil {
		return entity.Issue{}, err
	}

	return hydrated[0], nil
}

func (r *issueRepository) ListVisible(
	ctx context.Context,
	scope entity.TeamScope,
	page entity.IssuePage,
) ([]entity.Issue, error) {
	statement, args := r.pageStatement(scope, page)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list visible issues: %w", err)
	}

	defer func() { _ = rows.Close() }()

	issues := make([]entity.Issue, 0, page.Limit)

	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}

		issues = append(issues, issue)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issues: %w", err)
	}

	if err := r.hydrate(ctx, scope, issues); err != nil {
		return nil, err
	}

	return issues, nil
}

func (r *issueRepository) hydrate(
	ctx context.Context,
	scope entity.TeamScope,
	issues []entity.Issue,
) error {
	if err := r.hydrateLabels(ctx, issues); err != nil {
		return err
	}

	if err := r.hydrateBlocked(ctx, issues); err != nil {
		return err
	}

	return r.hydrateChildProgress(ctx, scope, issues)
}

func (r *issueRepository) hydrateBlocked(ctx context.Context, issues []entity.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, blockedQuery, ids)
	if err != nil {
		return fmt.Errorf("find blocked issues: %w", err)
	}

	defer func() { _ = rows.Close() }()

	blocked := map[uuid.UUID]bool{}

	for rows.Next() {
		var raw string

		if err := rows.Scan(&raw); err != nil {
			return fmt.Errorf("scan blocked issue id: %w", err)
		}

		id, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("parse blocked issue id: %w", err)
		}

		blocked[id] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate blocked issues: %w", err)
	}

	for i := range issues {
		issues[i].Blocked = blocked[issues[i].ID]
	}

	return nil
}

func (r *issueRepository) hydrateLabels(ctx context.Context, issues []entity.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, issueLabelsQuery, ids)
	if err != nil {
		return fmt.Errorf("list issue labels: %w", err)
	}

	defer func() { _ = rows.Close() }()

	byIssue := make(map[uuid.UUID][]entity.Label, len(issues))

	for rows.Next() {
		var (
			label     entity.Label
			issueID   string
			id        string
			workspace string
			team      string
			group     string
			color     string
		)

		if err := rows.Scan(&issueID, &id, &workspace, &team, &group, &label.Name, &color); err != nil {
			return fmt.Errorf("scan issue label: %w", err)
		}

		issue, err := uuid.Parse(issueID)
		if err != nil {
			return fmt.Errorf("parse labelled issue id: %w", err)
		}

		if label.ID, err = uuid.Parse(id); err != nil {
			return fmt.Errorf("parse issue label id: %w", err)
		}

		if label.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return fmt.Errorf("parse issue label workspace id: %w", err)
		}

		if team != "" {
			if label.TeamID, err = uuid.Parse(team); err != nil {
				return fmt.Errorf("parse issue label team id: %w", err)
			}
		}

		if group != "" {
			if label.GroupID, err = uuid.Parse(group); err != nil {
				return fmt.Errorf("parse issue label group id: %w", err)
			}
		}

		label.Color = entity.LabelColor(color)
		byIssue[issue] = append(byIssue[issue], label)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate issue labels: %w", err)
	}

	for i := range issues {
		issues[i].Labels = byIssue[issues[i].ID]
	}

	return nil
}

func (r *issueRepository) LockByID(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	scope entity.TeamScope,
) (entity.Issue, error) {
	issue, err := scanIssue(r.db.Querier(ctx).QueryRowContext(
		ctx,
		lockIssueQuery,
		issueID.String(),
		workspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Issue{}, entity.ErrIssueNotFound
		}

		return entity.Issue{}, fmt.Errorf("lock issue: %w", err)
	}

	return issue, nil
}

func (r *issueRepository) Update(
	ctx context.Context,
	issueID uuid.UUID,
	expectedVersion int,
	change entity.IssueChange,
	timestamps *entity.StateTimestamps,
	changedAt time.Time,
) error {
	delta, err := json.Marshal(change.FieldVersions(expectedVersion + 1))
	if err != nil {
		return fmt.Errorf("encode issue field versions: %w", err)
	}

	var (
		stateID        any
		priority       any
		assignee       any
		estimate       any
		dueOn          any
		stateEnteredAt any
		completedAt    any
		touchCompleted bool
	)

	if change.StateID != nil {
		stateID = change.StateID.String()
	}

	if change.Priority != nil {
		priority = string(*change.Priority)
	}

	if change.Assignee != nil {
		assignee = change.Assignee.String()
	}

	if change.Estimate != nil {
		estimate = *change.Estimate
	}

	if change.DueOn != nil {
		dueOn = *change.DueOn
	}

	var cycleID any

	if change.CycleID != nil {
		cycleID = change.CycleID.String()
	}

	var projectID any

	if change.ProjectID != nil {
		projectID = change.ProjectID.String()
	}

	if timestamps != nil {
		stateEnteredAt = timestamps.StateEnteredAt
		touchCompleted = true

		if timestamps.CompletedAt != nil {
			completedAt = *timestamps.CompletedAt
		}
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		updateIssueQuery,
		issueID.String(),
		expectedVersion,
		change.Title,
		stateID,
		change.Description,
		priority,
		change.ClearAssignee,
		assignee,
		change.ClearEstimate,
		estimate,
		change.ClearDueOn,
		dueOn,
		stateEnteredAt,
		touchCompleted,
		completedAt,
		delta,
		changedAt,
		change.ClearCycle,
		cycleID,
		change.ClearProject,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated issue count: %w", err)
	}

	if updated == 0 {
		return entity.ErrIssueStale
	}

	return nil
}

func (r *issueRepository) MoveToTeam(
	ctx context.Context,
	issueID uuid.UUID,
	expectedVersion int,
	teamID, stateID uuid.UUID,
	timestamps entity.StateTimestamps,
	changedAt time.Time,
) error {
	delta, err := json.Marshal(map[string]int{
		entity.IssueFieldTeam:   expectedVersion + 1,
		entity.IssueFieldState:  expectedVersion + 1,
		entity.IssueFieldLabels: expectedVersion + 1,
		entity.IssueFieldCycle:  expectedVersion + 1,
	})
	if err != nil {
		return fmt.Errorf("encode issue field versions: %w", err)
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, dropTeamScopedLabelsQuery, issueID.String()); err != nil {
		return fmt.Errorf("drop team-scoped issue labels: %w", err)
	}

	var completedAt any

	if timestamps.CompletedAt != nil {
		completedAt = *timestamps.CompletedAt
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		moveIssueTeamQuery,
		issueID.String(),
		expectedVersion,
		teamID.String(),
		stateID.String(),
		timestamps.StateEnteredAt,
		completedAt,
		delta,
		changedAt,
	)
	if err != nil {
		return fmt.Errorf("move issue to another team: %w", err)
	}

	moved, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read moved issue count: %w", err)
	}

	if moved == 0 {
		return entity.ErrIssueStale
	}

	return nil
}

func (r *issueRepository) MoveIssuesToCycle(
	ctx context.Context,
	issueIDs []uuid.UUID,
	cycleID *uuid.UUID,
	changedAt time.Time,
) error {
	if len(issueIDs) == 0 {
		return nil
	}

	ids := make([]string, 0, len(issueIDs))
	for _, id := range issueIDs {
		ids = append(ids, id.String())
	}

	var destination any

	if cycleID != nil {
		destination = cycleID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		moveIssuesToCycleQuery,
		ids,
		destination,
		changedAt,
	); err != nil {
		return fmt.Errorf("move issues to another cycle: %w", err)
	}

	return nil
}

func (r *issueRepository) SetStatus(
	ctx context.Context,
	issueID uuid.UUID,
	expectedVersion int,
	lifecycle entity.IssueLifecycle,
	changedAt time.Time,
) error {
	var (
		archivedAt any
		requested  any
		purgeAfter any
	)

	if lifecycle.ArchivedAt != nil {
		archivedAt = *lifecycle.ArchivedAt
	}

	if lifecycle.DeletionRequestedAt != nil {
		requested = *lifecycle.DeletionRequestedAt
	}

	if lifecycle.PurgeAfter != nil {
		purgeAfter = *lifecycle.PurgeAfter
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		setIssueStatusQuery,
		issueID.String(),
		expectedVersion,
		string(lifecycle.Status),
		archivedAt,
		requested,
		purgeAfter,
		changedAt,
	)
	if err != nil {
		return fmt.Errorf("set issue status: %w", err)
	}

	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read restatused issue count: %w", err)
	}

	if changed == 0 {
		return entity.ErrIssueStale
	}

	return nil
}

func (r *issueRepository) Purge(ctx context.Context, issueID uuid.UUID, due time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		rerootChildrenQuery,
		issueID.String(),
		due,
		entity.IssueMaxDepth,
	); err != nil {
		return fmt.Errorf("re-root the children of a purged issue: %w", err)
	}

	result, err := r.db.Querier(ctx).ExecContext(ctx, purgeIssueQuery, issueID.String(), due)
	if err != nil {
		return fmt.Errorf("purge issue: %w", err)
	}

	purged, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read purged issue count: %w", err)
	}

	if purged == 0 {
		return entity.ErrIssuePurgeNotDue
	}

	return nil
}

func (r *issueRepository) StampLabels(
	ctx context.Context,
	issueID uuid.UUID,
	expectedVersion int,
	changedAt time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		stampLabelsQuery,
		issueID.String(),
		expectedVersion,
		changedAt,
	)
	if err != nil {
		return fmt.Errorf("stamp issue labels: %w", err)
	}

	stamped, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read relabelled issue count: %w", err)
	}

	if stamped == 0 {
		return entity.ErrIssueStale
	}

	return nil
}

func (r *issueRepository) ReassignState(ctx context.Context, fromStateID, toStateID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		reassignStateQuery,
		fromStateID.String(),
		toStateID.String(),
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("reassign issues to another state: %w", err)
	}

	return nil
}

func (r *issueRepository) Ancestors(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, ancestorsQuery, issueID.String(), entity.IssueMaxDepth)
	if err != nil {
		return nil, fmt.Errorf("walk issue ancestry: %w", err)
	}

	defer func() { _ = rows.Close() }()

	ancestors := make([]uuid.UUID, 0, entity.IssueMaxDepth)

	for rows.Next() {
		var raw string

		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan ancestor id: %w", err)
		}

		ancestor, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse ancestor id: %w", err)
		}

		ancestors = append(ancestors, ancestor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue ancestry: %w", err)
	}

	return ancestors, nil
}

func (r *issueRepository) SubtreeHeight(ctx context.Context, issueID uuid.UUID) (int, error) {
	var height sql.NullInt64

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		subtreeHeightQuery,
		issueID.String(),
		entity.IssueMaxDepth,
	).Scan(&height); err != nil {
		return 0, fmt.Errorf("measure issue sub-tree: %w", err)
	}

	if !height.Valid {
		return 0, nil
	}

	return int(height.Int64), nil
}

func (r *issueRepository) SetParent(
	ctx context.Context,
	issueID uuid.UUID,
	expectedVersion int,
	parentID *uuid.UUID,
	depth int,
	changedAt time.Time,
) error {
	var parent any

	if parentID != nil {
		parent = parentID.String()
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		setParentQuery,
		issueID.String(),
		expectedVersion,
		parent,
		depth,
		changedAt,
	)
	if err != nil {
		return fmt.Errorf("set issue parent: %w", err)
	}

	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read reparented issue count: %w", err)
	}

	if changed == 0 {
		return entity.ErrIssueStale
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		rewriteSubtreeDepthQuery,
		issueID.String(),
		depth,
		entity.IssueMaxDepth,
		changedAt,
	); err != nil {
		return fmt.Errorf("rewrite issue sub-tree depth: %w", err)
	}

	return nil
}

func (r *issueRepository) ListChildren(
	ctx context.Context,
	workspaceID, parentID uuid.UUID,
	scope entity.TeamScope,
) ([]entity.Issue, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		childrenQuery,
		parentID.String(),
		workspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
	)
	if err != nil {
		return nil, fmt.Errorf("list issue children: %w", err)
	}

	defer func() { _ = rows.Close() }()

	children := make([]entity.Issue, 0)

	for rows.Next() {
		child, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue child: %w", err)
		}

		children = append(children, child)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue children: %w", err)
	}

	if err := r.hydrate(ctx, scope, children); err != nil {
		return nil, err
	}

	return children, nil
}

func (r *issueRepository) hydrateChildProgress(
	ctx context.Context,
	scope entity.TeamScope,
	issues []entity.Issue,
) error {
	if len(issues) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}

	byParent, err := r.ProgressByParent(ctx, scope, ids)
	if err != nil {
		return err
	}

	for i := range issues {
		issues[i].Children = byParent[issues[i].ID]
	}

	return nil
}

func (r *issueRepository) ProgressByCategory(
	ctx context.Context,
	scope entity.TeamScope,
	teamID *uuid.UUID,
) (entity.IssueProgress, error) {
	return r.summed(ctx, scope, teamID, nil, nil)
}

func (r *issueRepository) ProgressByCycle(
	ctx context.Context,
	scope entity.TeamScope,
	cycleID uuid.UUID,
) (entity.IssueProgress, error) {
	return r.summed(ctx, scope, nil, &cycleID, nil)
}

func (r *issueRepository) ProgressByProject(
	ctx context.Context,
	scope entity.TeamScope,
	projectID uuid.UUID,
) (entity.IssueProgress, error) {
	return r.summed(ctx, scope, nil, nil, &projectID)
}

func (r *issueRepository) summed(
	ctx context.Context,
	scope entity.TeamScope,
	teamID, cycleID, projectID *uuid.UUID,
) (entity.IssueProgress, error) {
	byParent, err := r.tally(ctx, scope, teamID, cycleID, projectID, nil)
	if err != nil {
		return entity.IssueProgress{}, err
	}

	var progress entity.IssueProgress

	for _, group := range byParent {
		progress.NotStarted += group.NotStarted
		progress.Active += group.Active
		progress.Complete += group.Complete
		progress.Abandoned += group.Abandoned
	}

	return progress, nil
}

func (r *issueRepository) ProgressByParent(
	ctx context.Context,
	scope entity.TeamScope,
	parentIDs []uuid.UUID,
) (map[uuid.UUID]entity.IssueProgress, error) {
	if len(parentIDs) == 0 {
		return map[uuid.UUID]entity.IssueProgress{}, nil
	}

	return r.tally(ctx, scope, nil, nil, nil, parentIDs)
}

func (r *issueRepository) tally(
	ctx context.Context,
	scope entity.TeamScope,
	teamID, cycleID, projectID *uuid.UUID,
	parentIDs []uuid.UUID,
) (map[uuid.UUID]entity.IssueProgress, error) {
	team := uuid.Nil.String()

	if teamID != nil {
		team = teamID.String()
	}

	cycle := uuid.Nil.String()

	if cycleID != nil {
		cycle = cycleID.String()
	}

	project := uuid.Nil.String()

	if projectID != nil {
		project = projectID.String()
	}

	parents := make([]string, 0, len(parentIDs))
	for _, id := range parentIDs {
		parents = append(parents, id.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		progressQuery,
		scope.WorkspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
		teamID != nil,
		team,
		len(parents) > 0,
		parents,
		cycleID != nil,
		cycle,
		projectID != nil,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("tally issue progress: %w", err)
	}

	defer func() { _ = rows.Close() }()

	byParent := map[uuid.UUID]entity.IssueProgress{}

	for rows.Next() {
		var (
			parent   string
			category string
			issues   int
		)

		if err := rows.Scan(&parent, &category, &issues); err != nil {
			return nil, fmt.Errorf("scan issue progress: %w", err)
		}

		key := uuid.Nil

		if parent != "" {
			if key, err = uuid.Parse(parent); err != nil {
				return nil, fmt.Errorf("parse progress parent id: %w", err)
			}
		}

		progress := byParent[key]

		if !progress.Add(entity.StateCategory(category), issues) {
			return nil, fmt.Errorf("unknown workflow state category %q", category)
		}

		byParent[key] = progress
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue progress: %w", err)
	}

	return byParent, nil
}
