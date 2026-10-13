package roadmap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	checkViolationCode  = "23514"

	projectUniqueIndex = "workspace_roadmap_entries_project_key"
	issueUniqueIndex   = "workspace_roadmap_entries_issue_key"
	titleCheck         = "workspace_roadmap_entries_title_check"
)

const ensureRoadmapQuery = `
INSERT INTO workspace_roadmaps (workspace_id) VALUES ($1)
ON CONFLICT (workspace_id) DO NOTHING`

const liveSourceColumns = `
       (p.id IS NOT NULL OR i.id IS NOT NULL),
       coalesce(p.workspace_id, i.workspace_id)::text,
       coalesce(p.id IS NOT NULL
                OR (i.status = 'active' AND i.triage_state IS DISTINCT FROM 'waiting' AND t.status = 'active'), false),
       coalesce(i.team_id::text, ''),
       coalesce(t.visibility = 'private', false),
       coalesce(i.project_id::text, '')`

const liveSourceJoins = `
LEFT JOIN workspace_projects p ON p.id = e.source_project_id
LEFT JOIN workspace_issues i ON i.id = e.source_issue_id
LEFT JOIN workspace_teams t ON t.id = i.team_id`

const entrySourcesQuery = `
SELECT e.id::text,` + liveSourceColumns + `
FROM workspace_roadmap_entries e` + liveSourceJoins + `
WHERE e.roadmap_id = $1`

const publishedEntryColumns = `
       e.workspace_id::text, e.public_id, e.kind,
       coalesce(e.source_project_id::text, ''), coalesce(e.source_issue_id::text, ''),
       coalesce(e.parent_public_id, ''), e.public_title, e.public_description, e.status,
       coalesce(to_char(e.start_on, 'YYYY-MM-DD'), ''), coalesce(to_char(e.end_on, 'YYYY-MM-DD'), ''),
       e.progress_mode, e.progress_percent,
       e.show_issue_lists, e.show_project_descriptions, e.show_issue_descriptions,
       e.show_status, e.show_timeline, e.show_progress, e.position,
       coalesce(e.approved_team_id::text, ''), e.approved_team_private,
       coalesce(e.approved_project_id::text, ''),` + liveSourceColumns

const publishedEntriesQuery = `
SELECT` + publishedEntryColumns + `
FROM workspace_roadmap_published_entries e` + liveSourceJoins + `
WHERE e.roadmap_id = $1`

const publicationColumns = `
       r.workspace_id::text, r.published_title, r.published_description,
       r.published_show_issue_lists, r.published_show_project_descriptions,
       r.published_show_issue_descriptions, r.published_show_status,
       r.published_show_timeline, r.published_show_progress,
       r.published_version, r.published_at, coalesce(r.published_by_account_id::text, '')`

const publishedRoadmapJoin = `
FROM workspaces w
JOIN workspace_roadmaps r ON r.workspace_id = w.id AND r.published_version IS NOT NULL`

const publishedRoadmapBySlug = `
WHERE w.slug = $1 AND w.status = 'active'`

const publishedBySlugQuery = `
SELECT` + publicationColumns + `,` + publishedEntryColumns + publishedRoadmapJoin + `
JOIN workspace_roadmap_published_entries e ON e.roadmap_id = r.id` + liveSourceJoins + publishedRoadmapBySlug

const publicationBySlugQuery = `
SELECT` + publicationColumns + publishedRoadmapJoin + publishedRoadmapBySlug

const detachPublishedChildrenQuery = `
UPDATE workspace_roadmap_published_entries SET parent_public_id = NULL
WHERE roadmap_id = $1 AND parent_public_id = $2`

type scanner interface {
	Scan(dest ...any) error
}

func parseOptionalUUID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value)
}

func optionalUUID(id uuid.UUID) null.String {
	if id == uuid.Nil {
		return null.String{}
	}

	return null.StringFrom(id.String())
}

func optionalString(value string) null.String {
	if value == "" {
		return null.String{}
	}

	return null.StringFrom(value)
}

func optionalDate(value string) (null.Time, error) {
	if value == "" {
		return null.Time{}, nil
	}

	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return null.Time{}, fmt.Errorf("parse roadmap date: %w", err)
	}

	return null.TimeFrom(parsed), nil
}

func formatDate(value null.Time) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format(time.DateOnly)
}

func roadmapToEntity(model *dbpostgres.WorkspaceRoadmap) (entity.Roadmap, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Roadmap{}, fmt.Errorf("parse roadmap id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Roadmap{}, fmt.Errorf("parse roadmap workspace id: %w", err)
	}

	roadmap := entity.Roadmap{
		ID:          id,
		WorkspaceID: workspaceID,
		Title:       model.Title,
		Description: model.Description,
		Visibility: entity.RoadmapVisibility{
			IssueLists:          model.ShowIssueLists,
			ProjectDescriptions: model.ShowProjectDescriptions,
			IssueDescriptions:   model.ShowIssueDescriptions,
			Status:              model.ShowStatus,
			Timeline:            model.ShowTimeline,
			Progress:            model.ShowProgress,
		},
		Version:   model.Version,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	if model.PublishedVersion.Valid {
		publishedBy, err := parseOptionalUUID(model.PublishedByAccountID.String)
		if err != nil {
			return entity.Roadmap{}, fmt.Errorf("parse roadmap publisher id: %w", err)
		}

		roadmap.Publication = &entity.RoadmapPublication{
			Title:       model.PublishedTitle.String,
			Description: model.PublishedDescription.String,
			Visibility: entity.RoadmapVisibility{
				IssueLists:          model.PublishedShowIssueLists.Bool,
				ProjectDescriptions: model.PublishedShowProjectDescriptions.Bool,
				IssueDescriptions:   model.PublishedShowIssueDescriptions.Bool,
				Status:              model.PublishedShowStatus.Bool,
				Timeline:            model.PublishedShowTimeline.Bool,
				Progress:            model.PublishedShowProgress.Bool,
			},
			Version:     model.PublishedVersion.Int,
			PublishedAt: model.PublishedAt.Time,
			PublishedBy: publishedBy,
		}
	}

	return roadmap, nil
}

func entryToEntity(model *dbpostgres.WorkspaceRoadmapEntry) (entity.RoadmapEntry, error) {
	ids := make([]uuid.UUID, 0, 6)

	for _, value := range []string{
		model.ID,
		model.RoadmapID,
		model.WorkspaceID,
		model.SourceProjectID.String,
		model.SourceIssueID.String,
		model.ParentEntryID.String,
	} {
		id, err := parseOptionalUUID(value)
		if err != nil {
			return entity.RoadmapEntry{}, fmt.Errorf("parse roadmap entry id: %w", err)
		}

		ids = append(ids, id)
	}

	return entity.RoadmapEntry{
		ID:                ids[0],
		RoadmapID:         ids[1],
		WorkspaceID:       ids[2],
		PublicID:          model.PublicID,
		Kind:              entity.RoadmapEntryKind(model.Kind),
		SourceProjectID:   ids[3],
		SourceIssueID:     ids[4],
		ParentEntryID:     ids[5],
		PublicTitle:       model.PublicTitle,
		PublicDescription: model.PublicDescription,
		Status:            entity.PublicStatus(model.Status),
		Timeline:          entity.RoadmapTimeline{StartOn: formatDate(model.StartOn), EndOn: formatDate(model.EndOn)},
		ProgressMode:      entity.RoadmapProgressMode(model.ProgressMode),
		ProgressPercent:   model.ProgressPercent,
		Visibility: entity.RoadmapVisibility{
			IssueLists:          model.ShowIssueLists,
			ProjectDescriptions: model.ShowProjectDescriptions,
			IssueDescriptions:   model.ShowIssueDescriptions,
			Status:              model.ShowStatus,
			Timeline:            model.ShowTimeline,
			Progress:            model.ShowProgress,
		},
		Position:  model.Position,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}, nil
}

func entryToModel(entry entity.RoadmapEntry) (*dbpostgres.WorkspaceRoadmapEntry, error) {
	startOn, err := optionalDate(entry.Timeline.StartOn)
	if err != nil {
		return nil, err
	}

	endOn, err := optionalDate(entry.Timeline.EndOn)
	if err != nil {
		return nil, err
	}

	return &dbpostgres.WorkspaceRoadmapEntry{
		ID:                      entry.ID.String(),
		RoadmapID:               entry.RoadmapID.String(),
		WorkspaceID:             entry.WorkspaceID.String(),
		PublicID:                entry.PublicID,
		Kind:                    string(entry.Kind),
		SourceProjectID:         optionalUUID(entry.SourceProjectID),
		SourceIssueID:           optionalUUID(entry.SourceIssueID),
		ParentEntryID:           optionalUUID(entry.ParentEntryID),
		PublicTitle:             entry.PublicTitle,
		PublicDescription:       entry.PublicDescription,
		Status:                  string(entry.Status),
		StartOn:                 startOn,
		EndOn:                   endOn,
		ProgressMode:            string(entry.ProgressMode),
		ProgressPercent:         entry.ProgressPercent,
		ShowIssueLists:          entry.Visibility.IssueLists,
		ShowProjectDescriptions: entry.Visibility.ProjectDescriptions,
		ShowIssueDescriptions:   entry.Visibility.IssueDescriptions,
		ShowStatus:              entry.Visibility.Status,
		ShowTimeline:            entry.Visibility.Timeline,
		ShowProgress:            entry.Visibility.Progress,
		Position:                entry.Position,
		CreatedAt:               entry.CreatedAt,
		UpdatedAt:               entry.UpdatedAt,
	}, nil
}

func publishedToModel(
	roadmapID uuid.UUID,
	entry entity.PublishedRoadmapEntry,
) (*dbpostgres.WorkspaceRoadmapPublishedEntry, error) {
	startOn, err := optionalDate(entry.Timeline.StartOn)
	if err != nil {
		return nil, err
	}

	endOn, err := optionalDate(entry.Timeline.EndOn)
	if err != nil {
		return nil, err
	}

	return &dbpostgres.WorkspaceRoadmapPublishedEntry{
		RoadmapID:               roadmapID.String(),
		WorkspaceID:             entry.WorkspaceID.String(),
		PublicID:                entry.PublicID,
		Kind:                    string(entry.Kind),
		SourceProjectID:         optionalUUID(entry.SourceProjectID),
		SourceIssueID:           optionalUUID(entry.SourceIssueID),
		ParentPublicID:          optionalString(entry.ParentPublicID),
		PublicTitle:             entry.PublicTitle,
		PublicDescription:       entry.PublicDescription,
		Status:                  string(entry.Status),
		StartOn:                 startOn,
		EndOn:                   endOn,
		ProgressMode:            string(entry.ProgressMode),
		ProgressPercent:         entry.ProgressPercent,
		ShowIssueLists:          entry.Visibility.IssueLists,
		ShowProjectDescriptions: entry.Visibility.ProjectDescriptions,
		ShowIssueDescriptions:   entry.Visibility.IssueDescriptions,
		ShowStatus:              entry.Visibility.Status,
		ShowTimeline:            entry.Visibility.Timeline,
		ShowProgress:            entry.Visibility.Progress,
		Position:                entry.Position,
		ApprovedTeamID:          optionalUUID(entry.Approval.TeamID),
		ApprovedTeamPrivate:     entry.Approval.TeamPrivate,
		ApprovedProjectID:       optionalUUID(entry.Approval.ProjectID),
	}, nil
}

func scanSource(row scanner, extra ...any) (entity.RoadmapSource, error) {
	var (
		source                         entity.RoadmapSource
		workspaceID, teamID, projectID sql.NullString
		present, active, teamPrivate   bool
	)

	if err := row.Scan(append(extra, &present, &workspaceID, &active, &teamID, &teamPrivate, &projectID)...); err != nil {
		return entity.RoadmapSource{}, err
	}

	parsed := make([]uuid.UUID, 0, 3)

	for _, value := range []sql.NullString{workspaceID, teamID, projectID} {
		id, err := parseOptionalUUID(value.String)
		if err != nil {
			return entity.RoadmapSource{}, fmt.Errorf("parse roadmap source id: %w", err)
		}

		parsed = append(parsed, id)
	}

	source.Present = present
	source.WorkspaceID = parsed[0]
	source.Active = active
	source.TeamID = parsed[1]
	source.TeamPrivate = teamPrivate
	source.ProjectID = parsed[2]

	return source, nil
}

type publishedRow struct {
	workspaceID, kind, sourceProjectID, sourceIssueID string
	approvedTeamID, approvedProjectID                 string
	entry                                             entity.PublishedRoadmapEntry
}

type publicationRow struct {
	workspaceID, publishedBy string
	publication              entity.RoadmapPublication
}

func (h *publicationRow) targets() []any {
	return []any{
		&h.workspaceID, &h.publication.Title, &h.publication.Description,
		&h.publication.Visibility.IssueLists, &h.publication.Visibility.ProjectDescriptions,
		&h.publication.Visibility.IssueDescriptions, &h.publication.Visibility.Status,
		&h.publication.Visibility.Timeline, &h.publication.Visibility.Progress,
		&h.publication.Version, &h.publication.PublishedAt, &h.publishedBy,
	}
}

func (h *publicationRow) resolve() (entity.PublishedRoadmap, error) {
	workspaceID, err := uuid.Parse(h.workspaceID)
	if err != nil {
		return entity.PublishedRoadmap{}, fmt.Errorf("parse roadmap workspace id: %w", err)
	}

	publication := h.publication

	publication.PublishedBy, err = parseOptionalUUID(h.publishedBy)
	if err != nil {
		return entity.PublishedRoadmap{}, fmt.Errorf("parse roadmap publisher id: %w", err)
	}

	return entity.PublishedRoadmap{WorkspaceID: workspaceID, Publication: publication}, nil
}

func (p *publishedRow) targets() []any {
	return []any{
		&p.workspaceID, &p.entry.PublicID, &p.kind,
		&p.sourceProjectID, &p.sourceIssueID,
		&p.entry.ParentPublicID, &p.entry.PublicTitle, &p.entry.PublicDescription, &p.entry.Status,
		&p.entry.Timeline.StartOn, &p.entry.Timeline.EndOn,
		&p.entry.ProgressMode, &p.entry.ProgressPercent,
		&p.entry.Visibility.IssueLists, &p.entry.Visibility.ProjectDescriptions, &p.entry.Visibility.IssueDescriptions,
		&p.entry.Visibility.Status, &p.entry.Visibility.Timeline, &p.entry.Visibility.Progress, &p.entry.Position,
		&p.approvedTeamID, &p.entry.Approval.TeamPrivate, &p.approvedProjectID,
	}
}

func (p *publishedRow) resolve(source entity.RoadmapSource) (entity.PublishedRoadmapEntry, error) {
	parsed := make([]uuid.UUID, 0, 5)

	for _, value := range []string{p.workspaceID, p.sourceProjectID, p.sourceIssueID, p.approvedTeamID, p.approvedProjectID} {
		id, err := parseOptionalUUID(value)
		if err != nil {
			return entity.PublishedRoadmapEntry{}, fmt.Errorf("parse published roadmap entry id: %w", err)
		}

		parsed = append(parsed, id)
	}

	entry := p.entry
	entry.WorkspaceID = parsed[0]
	entry.Kind = entity.RoadmapEntryKind(p.kind)
	entry.SourceProjectID = parsed[1]
	entry.SourceIssueID = parsed[2]
	entry.Approval.TeamID = parsed[3]
	entry.Approval.ProjectID = parsed[4]
	entry.Source = source

	return entry, nil
}

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolationCode &&
		(pgErr.ConstraintName == projectUniqueIndex || pgErr.ConstraintName == issueUniqueIndex):
		return entity.ErrRoadmapEntryDuplicate
	case pgErr.Code == checkViolationCode && pgErr.ConstraintName == titleCheck:
		return entity.NewValidationError(entity.FieldError{
			Field: entity.RoadmapFieldPublicTitle,
			Code:  entity.ValidationCodeRequired,
		})
	default:
		return err
	}
}

type roadmapRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Roadmap {
	return &roadmapRepository{db: db}
}

func (r *roadmapRepository) Ensure(ctx context.Context, workspaceID uuid.UUID) (entity.Roadmap, error) {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, ensureRoadmapQuery, workspaceID.String()); err != nil {
		return entity.Roadmap{}, fmt.Errorf("ensure roadmap: %w", err)
	}

	return r.find(ctx, workspaceID)
}

func (r *roadmapRepository) Lock(ctx context.Context, workspaceID uuid.UUID) (entity.Roadmap, error) {
	return r.find(ctx, workspaceID, qm.For("UPDATE"))
}

func (r *roadmapRepository) find(ctx context.Context, workspaceID uuid.UUID, mods ...qm.QueryMod) (entity.Roadmap, error) {
	model, err := dbpostgres.WorkspaceRoadmaps(
		append([]qm.QueryMod{dbpostgres.WorkspaceRoadmapWhere.WorkspaceID.EQ(workspaceID.String())}, mods...)...,
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Roadmap{}, entity.ErrRoadmapNotFound
		}

		return entity.Roadmap{}, fmt.Errorf("find roadmap: %w", err)
	}

	return roadmapToEntity(model)
}

func (r *roadmapRepository) Save(ctx context.Context, roadmap entity.Roadmap) (entity.Roadmap, error) {
	updated, err := dbpostgres.WorkspaceRoadmaps(
		dbpostgres.WorkspaceRoadmapWhere.ID.EQ(roadmap.ID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceRoadmapColumns.Title:                   roadmap.Title,
		dbpostgres.WorkspaceRoadmapColumns.Description:             roadmap.Description,
		dbpostgres.WorkspaceRoadmapColumns.ShowIssueLists:          roadmap.Visibility.IssueLists,
		dbpostgres.WorkspaceRoadmapColumns.ShowProjectDescriptions: roadmap.Visibility.ProjectDescriptions,
		dbpostgres.WorkspaceRoadmapColumns.ShowIssueDescriptions:   roadmap.Visibility.IssueDescriptions,
		dbpostgres.WorkspaceRoadmapColumns.ShowStatus:              roadmap.Visibility.Status,
		dbpostgres.WorkspaceRoadmapColumns.ShowTimeline:            roadmap.Visibility.Timeline,
		dbpostgres.WorkspaceRoadmapColumns.ShowProgress:            roadmap.Visibility.Progress,
		dbpostgres.WorkspaceRoadmapColumns.Version:                 roadmap.Version,
		dbpostgres.WorkspaceRoadmapColumns.UpdatedAt:               time.Now().UTC(),
	})
	if err != nil {
		return entity.Roadmap{}, fmt.Errorf("save roadmap: %w", err)
	}

	if updated == 0 {
		return entity.Roadmap{}, entity.ErrRoadmapNotFound
	}

	return r.find(ctx, roadmap.WorkspaceID)
}

func (r *roadmapRepository) ListEntries(ctx context.Context, roadmapID uuid.UUID) ([]entity.RoadmapEntry, error) {
	models, err := dbpostgres.WorkspaceRoadmapEntries(
		dbpostgres.WorkspaceRoadmapEntryWhere.RoadmapID.EQ(roadmapID.String()),
		qm.OrderBy(dbpostgres.WorkspaceRoadmapEntryColumns.Position+", "+dbpostgres.WorkspaceRoadmapEntryColumns.CreatedAt),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list roadmap entries: %w", err)
	}

	entries := make([]entity.RoadmapEntry, 0, len(models))

	for _, model := range models {
		entry, err := entryToEntity(model)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *roadmapRepository) GetEntry(ctx context.Context, roadmapID, entryID uuid.UUID) (entity.RoadmapEntry, error) {
	model, err := dbpostgres.WorkspaceRoadmapEntries(
		dbpostgres.WorkspaceRoadmapEntryWhere.ID.EQ(entryID.String()),
		dbpostgres.WorkspaceRoadmapEntryWhere.RoadmapID.EQ(roadmapID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.RoadmapEntry{}, entity.ErrRoadmapEntryNotFound
		}

		return entity.RoadmapEntry{}, fmt.Errorf("find roadmap entry: %w", err)
	}

	return entryToEntity(model)
}

func (r *roadmapRepository) CreateEntry(ctx context.Context, entry entity.RoadmapEntry) (entity.RoadmapEntry, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	now := time.Now().UTC()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	model, err := entryToModel(entry)
	if err != nil {
		return entity.RoadmapEntry{}, err
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.RoadmapEntry{}, translated
		}

		return entity.RoadmapEntry{}, fmt.Errorf("insert roadmap entry: %w", err)
	}

	return entryToEntity(model)
}

func (r *roadmapRepository) UpdateEntry(ctx context.Context, entry entity.RoadmapEntry) (entity.RoadmapEntry, error) {
	entry.UpdatedAt = time.Now().UTC()

	model, err := entryToModel(entry)
	if err != nil {
		return entity.RoadmapEntry{}, err
	}

	updated, err := model.Update(ctx, r.db.Querier(ctx), boil.Blacklist(
		dbpostgres.WorkspaceRoadmapEntryColumns.ID,
		dbpostgres.WorkspaceRoadmapEntryColumns.RoadmapID,
		dbpostgres.WorkspaceRoadmapEntryColumns.WorkspaceID,
		dbpostgres.WorkspaceRoadmapEntryColumns.PublicID,
		dbpostgres.WorkspaceRoadmapEntryColumns.Kind,
		dbpostgres.WorkspaceRoadmapEntryColumns.SourceProjectID,
		dbpostgres.WorkspaceRoadmapEntryColumns.SourceIssueID,
		dbpostgres.WorkspaceRoadmapEntryColumns.CreatedAt,
	))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.RoadmapEntry{}, translated
		}

		return entity.RoadmapEntry{}, fmt.Errorf("update roadmap entry: %w", err)
	}

	if updated == 0 {
		return entity.RoadmapEntry{}, entity.ErrRoadmapEntryNotFound
	}

	return r.GetEntry(ctx, entry.RoadmapID, entry.ID)
}

func (r *roadmapRepository) DeleteEntry(ctx context.Context, roadmapID, entryID uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceRoadmapEntries(
		dbpostgres.WorkspaceRoadmapEntryWhere.ID.EQ(entryID.String()),
		dbpostgres.WorkspaceRoadmapEntryWhere.RoadmapID.EQ(roadmapID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete roadmap entry: %w", err)
	}

	if deleted == 0 {
		return entity.ErrRoadmapEntryNotFound
	}

	return nil
}

func (r *roadmapRepository) EntrySources(
	ctx context.Context,
	roadmapID uuid.UUID,
) (map[uuid.UUID]entity.RoadmapSource, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, entrySourcesQuery, roadmapID.String())
	if err != nil {
		return nil, fmt.Errorf("list roadmap entry sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sources := make(map[uuid.UUID]entity.RoadmapSource)

	for rows.Next() {
		var entryID string

		source, err := scanSource(rows, &entryID)
		if err != nil {
			return nil, fmt.Errorf("scan roadmap entry source: %w", err)
		}

		id, err := uuid.Parse(entryID)
		if err != nil {
			return nil, fmt.Errorf("parse roadmap entry id: %w", err)
		}

		sources[id] = source
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roadmap entry sources: %w", err)
	}

	return sources, nil
}

func (r *roadmapRepository) ListPublishedEntries(
	ctx context.Context,
	roadmapID uuid.UUID,
) ([]entity.PublishedRoadmapEntry, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, publishedEntriesQuery, roadmapID.String())
	if err != nil {
		return nil, fmt.Errorf("list published roadmap entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]entity.PublishedRoadmapEntry, 0)

	for rows.Next() {
		var row publishedRow

		source, err := scanSource(rows, row.targets()...)
		if err != nil {
			return nil, fmt.Errorf("scan published roadmap entry: %w", err)
		}

		entry, err := row.resolve(source)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published roadmap entries: %w", err)
	}

	return entries, nil
}

func (r *roadmapRepository) Publish(
	ctx context.Context,
	roadmapID uuid.UUID,
	publication entity.RoadmapPublication,
	entries []entity.PublishedRoadmapEntry,
) error {
	executor := r.db.Querier(ctx)

	if _, err := dbpostgres.WorkspaceRoadmapPublishedEntries(
		dbpostgres.WorkspaceRoadmapPublishedEntryWhere.RoadmapID.EQ(roadmapID.String()),
	).DeleteAll(ctx, executor); err != nil {
		return fmt.Errorf("clear published roadmap entries: %w", err)
	}

	for _, entry := range entries {
		model, err := publishedToModel(roadmapID, entry)
		if err != nil {
			return err
		}

		if err := model.Insert(ctx, executor, boil.Infer()); err != nil {
			return fmt.Errorf("insert published roadmap entry: %w", err)
		}
	}

	updated, err := dbpostgres.WorkspaceRoadmaps(
		dbpostgres.WorkspaceRoadmapWhere.ID.EQ(roadmapID.String()),
	).UpdateAll(ctx, executor, dbpostgres.M{
		dbpostgres.WorkspaceRoadmapColumns.PublishedVersion:                 publication.Version,
		dbpostgres.WorkspaceRoadmapColumns.PublishedTitle:                   publication.Title,
		dbpostgres.WorkspaceRoadmapColumns.PublishedDescription:             publication.Description,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowIssueLists:          publication.Visibility.IssueLists,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowProjectDescriptions: publication.Visibility.ProjectDescriptions,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowIssueDescriptions:   publication.Visibility.IssueDescriptions,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowStatus:              publication.Visibility.Status,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowTimeline:            publication.Visibility.Timeline,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowProgress:            publication.Visibility.Progress,
		dbpostgres.WorkspaceRoadmapColumns.PublishedAt:                      publication.PublishedAt,
		dbpostgres.WorkspaceRoadmapColumns.PublishedByAccountID:             optionalUUID(publication.PublishedBy),
	})
	if err != nil {
		return fmt.Errorf("publish roadmap: %w", err)
	}

	if updated == 0 {
		return entity.ErrRoadmapNotFound
	}

	return nil
}

func (r *roadmapRepository) Unpublish(ctx context.Context, roadmapID uuid.UUID) error {
	executor := r.db.Querier(ctx)

	if _, err := dbpostgres.WorkspaceRoadmapPublishedEntries(
		dbpostgres.WorkspaceRoadmapPublishedEntryWhere.RoadmapID.EQ(roadmapID.String()),
	).DeleteAll(ctx, executor); err != nil {
		return fmt.Errorf("clear published roadmap entries: %w", err)
	}

	updated, err := dbpostgres.WorkspaceRoadmaps(
		dbpostgres.WorkspaceRoadmapWhere.ID.EQ(roadmapID.String()),
	).UpdateAll(ctx, executor, dbpostgres.M{
		dbpostgres.WorkspaceRoadmapColumns.PublishedVersion:                 nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedTitle:                   nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedDescription:             nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowIssueLists:          nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowProjectDescriptions: nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowIssueDescriptions:   nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowStatus:              nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowTimeline:            nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedShowProgress:            nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedAt:                      nil,
		dbpostgres.WorkspaceRoadmapColumns.PublishedByAccountID:             nil,
	})
	if err != nil {
		return fmt.Errorf("unpublish roadmap: %w", err)
	}

	if updated == 0 {
		return entity.ErrRoadmapNotFound
	}

	return nil
}

func (r *roadmapRepository) DeletePublishedEntry(ctx context.Context, roadmapID uuid.UUID, publicID string) error {
	executor := r.db.Querier(ctx)

	if _, err := dbpostgres.WorkspaceRoadmapPublishedEntries(
		dbpostgres.WorkspaceRoadmapPublishedEntryWhere.RoadmapID.EQ(roadmapID.String()),
		dbpostgres.WorkspaceRoadmapPublishedEntryWhere.PublicID.EQ(publicID),
	).DeleteAll(ctx, executor); err != nil {
		return fmt.Errorf("delete published roadmap entry: %w", err)
	}

	if _, err := executor.ExecContext(ctx, detachPublishedChildrenQuery, roadmapID.String(), publicID); err != nil {
		return fmt.Errorf("detach published roadmap entries: %w", err)
	}

	return nil
}

func (r *roadmapRepository) PublishedBySlug(ctx context.Context, workspaceSlug string) (entity.PublishedRoadmap, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, publishedBySlugQuery, workspaceSlug)
	if err != nil {
		return entity.PublishedRoadmap{}, fmt.Errorf("read published roadmap: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		published entity.PublishedRoadmap
		found     bool
	)

	for rows.Next() {
		var (
			header publicationRow
			row    publishedRow
		)

		source, err := scanSource(rows, append(header.targets(), row.targets()...)...)
		if err != nil {
			return entity.PublishedRoadmap{}, fmt.Errorf("scan published roadmap: %w", err)
		}

		if !found {
			published, err = header.resolve()
			if err != nil {
				return entity.PublishedRoadmap{}, err
			}

			found = true
		}

		entry, err := row.resolve(source)
		if err != nil {
			return entity.PublishedRoadmap{}, err
		}

		published.Entries = append(published.Entries, entry)
	}

	if err := rows.Err(); err != nil {
		return entity.PublishedRoadmap{}, fmt.Errorf("iterate published roadmap: %w", err)
	}

	if found {
		return published, nil
	}

	var header publicationRow

	if err := r.db.Querier(ctx).QueryRowContext(ctx, publicationBySlugQuery, workspaceSlug).Scan(header.targets()...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PublishedRoadmap{}, entity.ErrRoadmapNotFound
		}

		return entity.PublishedRoadmap{}, fmt.Errorf("read roadmap publication: %w", err)
	}

	return header.resolve()
}
