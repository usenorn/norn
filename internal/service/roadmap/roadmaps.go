package roadmap

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type roadmapsService struct {
	roadmaps   repository.Roadmap
	projects   repository.Project
	issues     repository.Issue
	authorizer service.Authorizer
	audit      service.Audit
	transactor repository.Transactor
}

func New(
	roadmaps repository.Roadmap,
	projects repository.Project,
	issues repository.Issue,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
) service.Roadmaps {
	return &roadmapsService{
		roadmaps:   roadmaps,
		projects:   projects,
		issues:     issues,
		authorizer: authorizer,
		audit:      audit,
		transactor: transactor,
	}
}

func (s *roadmapsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceRoadmap,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func reachable(
	workspaceID uuid.UUID,
	entry entity.RoadmapEntry,
	source entity.RoadmapSource,
	scope entity.TeamScope,
) error {
	if !entity.RoadmapEntryReachable(workspaceID, entry, source, scope) {
		return entity.RoadmapUnreachableError{EntryID: entry.ID}
	}

	return nil
}

func (s *roadmapsService) view(
	ctx context.Context,
	roadmap entity.Roadmap,
	scope entity.TeamScope,
) (service.RoadmapDraftView, error) {
	entries, err := s.roadmaps.ListEntries(ctx, roadmap.ID)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	sources, err := s.roadmaps.EntrySources(ctx, roadmap.ID)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	published, err := s.roadmaps.ListPublishedEntries(ctx, roadmap.ID)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	intact := make(map[string]bool, len(published))
	for _, entry := range published {
		intact[entry.PublicID] = entry.Intact()
	}

	view := service.RoadmapDraftView{
		Roadmap: roadmap,
		Entries: make([]service.RoadmapEntryView, 0, len(entries)),
	}

	for _, entry := range entries {
		isIntact, isPublished := intact[entry.PublicID]

		view.Entries = append(view.Entries, service.RoadmapEntryView{
			Entry:     entry,
			Reachable: reachable(roadmap.WorkspaceID, entry, sources[entry.ID], scope) == nil,
			Published: isPublished,
			Withheld:  isPublished && !isIntact,
		})
	}

	return view, nil
}

func (s *roadmapsService) Get(ctx context.Context, workspaceID uuid.UUID) (service.RoadmapDraftView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	roadmap, err := s.roadmaps.Ensure(ctx, workspaceID)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	return s.view(ctx, roadmap, decision.Scope)
}

func (s *roadmapsService) edit(
	ctx context.Context,
	workspaceID uuid.UUID,
	expectedVersion int,
	change func(ctx context.Context, roadmap *entity.Roadmap, decision entity.Decision) error,
) (service.RoadmapDraftView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	var view service.RoadmapDraftView

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		roadmap, err := s.lock(ctx, workspaceID, expectedVersion)
		if err != nil {
			return err
		}

		if err := change(ctx, &roadmap, decision); err != nil {
			return err
		}

		roadmap.Version++

		saved, err := s.roadmaps.Save(ctx, roadmap)
		if err != nil {
			return err
		}

		view, err = s.view(ctx, saved, decision.Scope)

		return err
	})
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	return view, nil
}

func (s *roadmapsService) lock(ctx context.Context, workspaceID uuid.UUID, expectedVersion int) (entity.Roadmap, error) {
	if _, err := s.roadmaps.Ensure(ctx, workspaceID); err != nil {
		return entity.Roadmap{}, err
	}

	roadmap, err := s.roadmaps.Lock(ctx, workspaceID)
	if err != nil {
		return entity.Roadmap{}, err
	}

	if roadmap.Version != expectedVersion {
		return entity.Roadmap{}, entity.RoadmapStaleError{Version: roadmap.Version}
	}

	return roadmap, nil
}

func (s *roadmapsService) Update(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.UpdateRoadmapInput,
) (service.RoadmapDraftView, error) {
	if err := entity.NewValidationError(
		entity.ValidateRoadmapDraftTitle(entity.RoadmapFieldTitle, input.Title),
		entity.ValidateRoadmapDescription(entity.RoadmapFieldDescription, input.Description),
	); err != nil {
		return service.RoadmapDraftView{}, err
	}

	return s.edit(ctx, workspaceID, input.ExpectedVersion,
		func(_ context.Context, roadmap *entity.Roadmap, _ entity.Decision) error {
			roadmap.Title = input.Title
			roadmap.Description = input.Description
			roadmap.Visibility = input.Visibility

			return nil
		})
}

func applyContent(entry entity.RoadmapEntry, content service.RoadmapEntryContent) entity.RoadmapEntry {
	entry.PublicTitle = content.PublicTitle
	entry.PublicDescription = content.PublicDescription
	entry.Status = content.Status
	entry.Timeline = content.Timeline
	entry.ProgressMode = content.ProgressMode
	entry.ProgressPercent = content.ProgressPercent
	entry.Visibility = content.Visibility
	entry.Position = content.Position

	return entry
}

type snapshot struct {
	status    entity.PublicStatus
	endOn     string
	projectID uuid.UUID
}

func (s *roadmapsService) snapshot(
	ctx context.Context,
	workspaceID uuid.UUID,
	kind entity.RoadmapEntryKind,
	sourceID uuid.UUID,
	scope entity.TeamScope,
) (snapshot, error) {
	if kind == entity.RoadmapEntryProject {
		project, err := s.projects.GetByID(ctx, workspaceID, sourceID)
		if err != nil {
			return snapshot{}, err
		}

		return snapshot{status: entity.PublicStatusOfProject(project.State), endOn: project.TargetOn}, nil
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, sourceID, scope)
	if err != nil {
		return snapshot{}, err
	}

	if issue.Status != entity.IssueStatusActive {
		return snapshot{}, entity.ErrIssueNotFound
	}

	return snapshot{
		status:    entity.PublicStatusOfIssue(issue.State.Category),
		endOn:     issue.DueOn,
		projectID: issue.ProjectID,
	}, nil
}

func projectEntryFor(entries []entity.RoadmapEntry, projectID uuid.UUID) uuid.UUID {
	if projectID == uuid.Nil {
		return uuid.Nil
	}

	for _, entry := range entries {
		if entry.Kind == entity.RoadmapEntryProject && entry.SourceProjectID == projectID {
			return entry.ID
		}
	}

	return uuid.Nil
}

func (s *roadmapsService) AddEntry(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.AddRoadmapEntryInput,
) (service.RoadmapDraftView, error) {
	if !input.Kind.Valid() {
		return service.RoadmapDraftView{}, entity.NewValidationError(entity.FieldError{
			Field: entity.RoadmapFieldKind,
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return s.edit(ctx, workspaceID, input.ExpectedVersion,
		func(ctx context.Context, roadmap *entity.Roadmap, decision entity.Decision) error {
			entries, err := s.roadmaps.ListEntries(ctx, roadmap.ID)
			if err != nil {
				return err
			}

			if len(entries) >= entity.RoadmapEntriesMax {
				return entity.ErrRoadmapFull
			}

			source, err := s.snapshot(ctx, workspaceID, input.Kind, input.SourceID, decision.Scope)
			if err != nil {
				return err
			}

			publicID, err := entity.NewRoadmapPublicID()
			if err != nil {
				return err
			}

			content := input.Content
			if content.Status == "" {
				content.Status = source.status
			}

			if content.Timeline.Empty() {
				content.Timeline.EndOn = source.endOn
			}

			entry := applyContent(entity.RoadmapEntry{
				ID:            uuid.New(),
				RoadmapID:     roadmap.ID,
				WorkspaceID:   workspaceID,
				PublicID:      publicID,
				Kind:          input.Kind,
				ParentEntryID: projectEntryFor(entries, source.projectID),
			}, content)

			if input.Kind == entity.RoadmapEntryProject {
				entry.SourceProjectID = input.SourceID
			} else {
				entry.SourceIssueID = input.SourceID
			}

			if err := entity.ValidateRoadmapEntry(entry); err != nil {
				return err
			}

			created, err := s.roadmaps.CreateEntry(ctx, entry)
			if err != nil {
				return err
			}

			if created.Kind == entity.RoadmapEntryProject {
				return s.adoptIssues(ctx, roadmap.ID, created, entries)
			}

			return nil
		})
}

func (s *roadmapsService) adoptIssues(
	ctx context.Context,
	roadmapID uuid.UUID,
	project entity.RoadmapEntry,
	entries []entity.RoadmapEntry,
) error {
	sources, err := s.roadmaps.EntrySources(ctx, roadmapID)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Kind != entity.RoadmapEntryIssue || entry.ParentEntryID != uuid.Nil {
			continue
		}

		if sources[entry.ID].ProjectID != project.SourceProjectID {
			continue
		}

		entry.ParentEntryID = project.ID

		if _, err := s.roadmaps.UpdateEntry(ctx, entry); err != nil {
			return err
		}
	}

	return nil
}

func (s *roadmapsService) UpdateEntry(
	ctx context.Context,
	workspaceID, entryID uuid.UUID,
	input service.UpdateRoadmapEntryInput,
) (service.RoadmapDraftView, error) {
	return s.edit(ctx, workspaceID, input.ExpectedVersion,
		func(ctx context.Context, roadmap *entity.Roadmap, _ entity.Decision) error {
			entry, err := s.roadmaps.GetEntry(ctx, roadmap.ID, entryID)
			if err != nil {
				return err
			}

			if input.Content.Status == "" {
				input.Content.Status = entry.Status
			}

			entry = applyContent(entry, input.Content)

			if err := entity.ValidateRoadmapEntry(entry); err != nil {
				return err
			}

			_, err = s.roadmaps.UpdateEntry(ctx, entry)

			return err
		})
}

func (s *roadmapsService) RefreshEntry(
	ctx context.Context,
	workspaceID, entryID uuid.UUID,
	expectedVersion int,
) (service.RoadmapDraftView, error) {
	return s.edit(ctx, workspaceID, expectedVersion,
		func(ctx context.Context, roadmap *entity.Roadmap, decision entity.Decision) error {
			entry, err := s.roadmaps.GetEntry(ctx, roadmap.ID, entryID)
			if err != nil {
				return err
			}

			sourceID := entry.SourceProjectID
			if entry.Kind == entity.RoadmapEntryIssue {
				sourceID = entry.SourceIssueID
			}

			if sourceID == uuid.Nil {
				return entity.RoadmapUnreachableError{EntryID: entry.ID}
			}

			source, err := s.snapshot(ctx, workspaceID, entry.Kind, sourceID, decision.Scope)
			if err != nil {
				return err
			}

			entries, err := s.roadmaps.ListEntries(ctx, roadmap.ID)
			if err != nil {
				return err
			}

			entry.Status = source.status
			if source.endOn != "" && (entry.Timeline.StartOn == "" || source.endOn >= entry.Timeline.StartOn) {
				entry.Timeline.EndOn = source.endOn
			}

			if entry.Kind == entity.RoadmapEntryIssue {
				entry.ParentEntryID = projectEntryFor(entries, source.projectID)
			}

			_, err = s.roadmaps.UpdateEntry(ctx, entry)

			return err
		})
}

func (s *roadmapsService) RemoveEntry(
	ctx context.Context,
	workspaceID, entryID uuid.UUID,
	expectedVersion int,
) (service.RoadmapDraftView, error) {
	var removed entity.RoadmapEntry

	view, err := s.edit(ctx, workspaceID, expectedVersion,
		func(ctx context.Context, roadmap *entity.Roadmap, _ entity.Decision) error {
			entry, err := s.roadmaps.GetEntry(ctx, roadmap.ID, entryID)
			if err != nil {
				return err
			}

			if err := s.roadmaps.DeleteEntry(ctx, roadmap.ID, entryID); err != nil {
				return err
			}

			if err := s.roadmaps.DeletePublishedEntry(ctx, roadmap.ID, entry.PublicID); err != nil {
				return err
			}

			removed = entry

			return nil
		})
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditRoadmapEntryRemoved,
		ResourceKind: string(entity.ResourceRoadmap),
		ResourceID:   view.Roadmap.ID,
		Detail: map[string]string{
			"entry": removed.PublicID,
			"kind":  string(removed.Kind),
		},
	})

	return view, nil
}

func (s *roadmapsService) Publish(
	ctx context.Context,
	workspaceID uuid.UUID,
	expectedVersion int,
) (service.RoadmapDraftView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	var (
		view    service.RoadmapDraftView
		entries []entity.PublishedRoadmapEntry
	)

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		roadmap, err := s.lock(ctx, workspaceID, expectedVersion)
		if err != nil {
			return err
		}

		if err := entity.NewValidationError(
			entity.ValidateRoadmapTitle(entity.RoadmapFieldTitle, roadmap.Title),
		); err != nil {
			return err
		}

		entries, err = s.reviewed(ctx, roadmap, decision.Scope)
		if err != nil {
			return err
		}

		if err := s.roadmaps.Publish(ctx, roadmap.ID, entity.RoadmapPublication{
			Title:       roadmap.Title,
			Description: roadmap.Description,
			Visibility:  roadmap.Visibility,
			Version:     roadmap.Version,
			PublishedAt: time.Now().UTC(),
			PublishedBy: decision.Actor.AccountID,
		}, entries); err != nil {
			return err
		}

		published, err := s.roadmaps.Lock(ctx, workspaceID)
		if err != nil {
			return err
		}

		view, err = s.view(ctx, published, decision.Scope)

		return err
	})
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditRoadmapPublished,
		ResourceKind: string(entity.ResourceRoadmap),
		ResourceID:   view.Roadmap.ID,
		Detail: map[string]string{
			"version": strconv.Itoa(view.Roadmap.Version),
			"entries": strconv.Itoa(len(entries)),
		},
	})

	return view, nil
}

func (s *roadmapsService) reviewed(
	ctx context.Context,
	roadmap entity.Roadmap,
	scope entity.TeamScope,
) ([]entity.PublishedRoadmapEntry, error) {
	drafts, err := s.roadmaps.ListEntries(ctx, roadmap.ID)
	if err != nil {
		return nil, err
	}

	sources, err := s.roadmaps.EntrySources(ctx, roadmap.ID)
	if err != nil {
		return nil, err
	}

	projects := make(map[uuid.UUID]entity.RoadmapEntry, len(drafts))

	for _, draft := range drafts {
		if err := reachable(roadmap.WorkspaceID, draft, sources[draft.ID], scope); err != nil {
			return nil, err
		}

		if err := entity.ValidateRoadmapEntry(draft); err != nil {
			return nil, err
		}

		if draft.Kind == entity.RoadmapEntryProject {
			projects[draft.ID] = draft
		}
	}

	reviewed := make([]entity.PublishedRoadmapEntry, 0, len(drafts))

	for _, draft := range drafts {
		source := sources[draft.ID]
		entry := entity.PublishedRoadmapEntry{RoadmapEntry: draft}

		if draft.Kind == entity.RoadmapEntryIssue {
			entry.Approval = entity.RoadmapApproval{
				TeamID:      source.TeamID,
				TeamPrivate: source.TeamPrivate,
				ProjectID:   source.ProjectID,
			}

			if parent, ok := projects[draft.ParentEntryID]; ok && parent.SourceProjectID == source.ProjectID {
				entry.ParentPublicID = parent.PublicID
			}
		}

		reviewed = append(reviewed, entry)
	}

	return reviewed, nil
}

func (s *roadmapsService) Unpublish(ctx context.Context, workspaceID uuid.UUID) (service.RoadmapDraftView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	var (
		view      service.RoadmapDraftView
		withdrawn bool
	)

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.roadmaps.Ensure(ctx, workspaceID); err != nil {
			return err
		}

		roadmap, err := s.roadmaps.Lock(ctx, workspaceID)
		if err != nil {
			return err
		}

		if roadmap.Published() {
			if err := s.roadmaps.Unpublish(ctx, roadmap.ID); err != nil {
				return err
			}

			roadmap.Publication = nil
			withdrawn = true
		}

		view, err = s.view(ctx, roadmap, decision.Scope)

		return err
	})
	if err != nil {
		return service.RoadmapDraftView{}, err
	}

	if withdrawn {
		s.audit.Record(ctx, entity.AuditEntry{
			WorkspaceID:  workspaceID,
			Action:       entity.AuditRoadmapUnpublished,
			ResourceKind: string(entity.ResourceRoadmap),
			ResourceID:   view.Roadmap.ID,
		})
	}

	return view, nil
}
