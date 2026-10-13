package roadmap_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	roadmaprepo "github.com/usenorn/norn/internal/repository/roadmap"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	roadmapsvc "github.com/usenorn/norn/internal/service/roadmap"
)

type harness struct {
	roadmaps   *roadmaprepo.MockRoadmap
	projects   *projectrepo.MockProject
	issues     *issuerepo.MockIssue
	authorizer *authorizersvc.MockAuthorizer
	audit      *auditsvc.MockAudit
	transactor *transactorrepo.MockTransactor
	service    service.Roadmaps

	workspaceID uuid.UUID
	accountID   uuid.UUID
	roadmap     entity.Roadmap
	recorded    []entity.AuditEntry
	publication *entity.RoadmapPublication
	published   []entity.PublishedRoadmapEntry
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	workspaceID := uuid.New()

	h := &harness{
		roadmaps:    roadmaprepo.NewMockRoadmap(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: workspaceID,
		accountID:   uuid.New(),
		roadmap: entity.Roadmap{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			Title:       "What we are building",
			Version:     3,
		},
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		AnyTimes()

	h.audit.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) { h.recorded = append(h.recorded, entry) }).
		AnyTimes()

	h.withDomain("roadmap.example.com")

	return h
}

func (h *harness) withDomain(domain string) {
	h.service = roadmapsvc.New(
		h.roadmaps, h.projects, h.issues, h.authorizer, h.audit, h.transactor,
		config.App{BaseURL: "https://app.example.com"},
		config.Roadmaps{Domain: domain},
	)
}

func (h *harness) actAs(scope entity.TeamScope) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.Resource != entity.ResourceRoadmap {
				return entity.Decision{}, errors.New("a roadmap operation asked about " + string(request.Resource))
			}

			scope.WorkspaceID = h.workspaceID

			return entity.Decision{
				Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
				Role:      entity.MembershipRoleAdmin,
				Workspace: entity.Workspace{ID: h.workspaceID, Slug: "acme"},
				Scope:     scope,
			}, nil
		}).
		AnyTimes()
}

func (h *harness) lockAt(version int) {
	locked := func(context.Context, uuid.UUID) (entity.Roadmap, error) {
		roadmap := h.roadmap
		roadmap.Version = version

		if h.publication != nil {
			roadmap.Publication = h.publication
		}

		return roadmap, nil
	}

	h.roadmaps.EXPECT().Ensure(gomock.Any(), h.workspaceID).DoAndReturn(locked).AnyTimes()
	h.roadmaps.EXPECT().Lock(gomock.Any(), h.workspaceID).DoAndReturn(locked).AnyTimes()
}

func (h *harness) expectDraft(entries []entity.RoadmapEntry, sources map[uuid.UUID]entity.RoadmapSource) {
	h.roadmaps.EXPECT().ListEntries(gomock.Any(), h.roadmap.ID).Return(entries, nil).AnyTimes()
	h.roadmaps.EXPECT().EntrySources(gomock.Any(), h.roadmap.ID).Return(sources, nil).AnyTimes()
	h.roadmaps.EXPECT().ListPublishedEntries(gomock.Any(), h.roadmap.ID).
		DoAndReturn(func(context.Context, uuid.UUID) ([]entity.PublishedRoadmapEntry, error) { return h.published, nil }).
		AnyTimes()
}

func (h *harness) expectSaveAndView(entries []entity.RoadmapEntry, sources map[uuid.UUID]entity.RoadmapSource) {
	h.roadmaps.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, roadmap entity.Roadmap) (entity.Roadmap, error) { return roadmap, nil }).
		AnyTimes()
	h.expectDraft(entries, sources)
	h.projects.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID, gomock.Any()).Return(nil, nil).AnyTimes()
	h.issues.EXPECT().ListVisibleByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
}

func (h *harness) expectPublish() {
	h.roadmaps.EXPECT().Publish(gomock.Any(), h.roadmap.ID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, p entity.RoadmapPublication, entries []entity.PublishedRoadmapEntry) error {
			h.publication, h.published = &p, entries

			return nil
		})
}

func (h *harness) publish(t *testing.T) service.RoadmapDraftView {
	t.Helper()

	preview, err := h.service.Preview(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	view, err := h.service.Publish(context.Background(), h.workspaceID, service.PublishRoadmapInput{
		ExpectedVersion: preview.Version,
		ExpectedDigest:  preview.Digest,
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	return view
}

func reviewed(version int) service.PublishRoadmapInput {
	return service.PublishRoadmapInput{ExpectedVersion: version, ExpectedDigest: "reviewed"}
}

func (h *harness) entry(kind entity.RoadmapEntryKind, title string) entity.RoadmapEntry {
	publicID, _ := entity.NewRoadmapPublicID()

	entry := entity.RoadmapEntry{
		ID:           uuid.New(),
		RoadmapID:    h.roadmap.ID,
		WorkspaceID:  h.workspaceID,
		PublicID:     publicID,
		Kind:         kind,
		PublicTitle:  title,
		Status:       entity.PublicStatusPlanned,
		ProgressMode: entity.RoadmapProgressCalculated,
	}

	if kind == entity.RoadmapEntryProject {
		entry.SourceProjectID = uuid.New()
	} else {
		entry.SourceIssueID = uuid.New()
	}

	return entry
}

func content(title string) service.RoadmapEntryContent {
	return service.RoadmapEntryContent{PublicTitle: title, ProgressMode: entity.RoadmapProgressHidden}
}

func TestOnlyARoadmapGrantPublishesAndProjectEditingNeverImpliesIt(t *testing.T) {
	for _, role := range []entity.MembershipRole{entity.MembershipRoleMember, entity.MembershipRoleViewer} {
		if entity.RoleGrants(role, entity.ResourceRoadmap, entity.ActionManage) {
			t.Fatalf("%s may publish the roadmap; publication is an admin grant", role)
		}

		if entity.RoleGrants(role, entity.ResourceRoadmap, entity.ActionRead) {
			t.Fatalf("%s may read the roadmap draft", role)
		}
	}

	if !entity.RoleGrants(entity.MembershipRoleMember, entity.ResourceProject, entity.ActionManage) {
		t.Fatal("the premise changed: members no longer edit projects")
	}

	if !entity.RoleGrants(entity.MembershipRoleAdmin, entity.ResourceRoadmap, entity.ActionManage) {
		t.Fatal("an admin may not publish the roadmap")
	}

	h := newHarness(t)
	denied := entity.AccessDeniedError{Reason: entity.DenyReasonRoleLacksAction, Resource: entity.ResourceRoadmap}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.Resource != entity.ResourceRoadmap || request.Action != entity.ActionManage {
				t.Fatalf("publishing asked for %s %s, want roadmap manage", request.Resource, request.Action)
			}

			return entity.Decision{}, denied
		}).
		Times(3)

	if _, err := h.service.Publish(context.Background(), h.workspaceID, reviewed(3)); !errors.As(err, &entity.AccessDeniedError{}) {
		t.Fatalf("Publish = %v, want access denied", err)
	}

	if _, err := h.service.Unpublish(context.Background(), h.workspaceID); !errors.As(err, &entity.AccessDeniedError{}) {
		t.Fatalf("Unpublish = %v, want access denied", err)
	}

	if _, err := h.service.AddEntry(context.Background(), h.workspaceID, service.AddRoadmapEntryInput{
		ExpectedVersion: 3,
		Kind:            entity.RoadmapEntryProject,
		SourceID:        uuid.New(),
		Content:         content("Public"),
	}); !errors.As(err, &entity.AccessDeniedError{}) {
		t.Fatalf("AddEntry = %v, want access denied", err)
	}
}

func TestAProjectFromAnotherWorkspaceCannotBeSelected(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)
	h.roadmaps.EXPECT().ListEntries(gomock.Any(), h.roadmap.ID).Return(nil, nil)

	foreign := uuid.New()
	h.projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, foreign).Return(entity.Project{}, entity.ErrProjectNotFound)
	h.roadmaps.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).Times(0)
	h.roadmaps.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.AddEntry(context.Background(), h.workspaceID, service.AddRoadmapEntryInput{
		ExpectedVersion: 3,
		Kind:            entity.RoadmapEntryProject,
		SourceID:        foreign,
		Content:         content("Public"),
	})
	if !errors.Is(err, entity.ErrProjectNotFound) {
		t.Fatalf("AddEntry = %v, want ErrProjectNotFound", err)
	}
}

func TestAnIssueTheSelectorCannotSeeCannotBeSelected(t *testing.T) {
	h := newHarness(t)
	scope := entity.TeamScope{TeamIDs: []uuid.UUID{uuid.New()}}
	h.actAs(scope)
	h.lockAt(3)
	h.roadmaps.EXPECT().ListEntries(gomock.Any(), h.roadmap.ID).Return(nil, nil)

	hidden := uuid.New()
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, hidden, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, got entity.TeamScope) (entity.Issue, error) {
			if got.AllTeams || len(got.TeamIDs) != 1 {
				t.Fatalf("issue looked up with scope %+v, want the selector's own scope", got)
			}

			return entity.Issue{}, entity.ErrIssueNotFound
		})
	h.roadmaps.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.AddEntry(context.Background(), h.workspaceID, service.AddRoadmapEntryInput{
		ExpectedVersion: 3,
		Kind:            entity.RoadmapEntryIssue,
		SourceID:        hidden,
		Content:         content("Public"),
	})
	if !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf("AddEntry = %v, want ErrIssueNotFound", err)
	}
}

func TestAnEntryNeverFallsBackToInternalText(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)
	h.roadmaps.EXPECT().ListEntries(gomock.Any(), h.roadmap.ID).Return(nil, nil)

	project := entity.Project{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Name:        "Internal codename",
		Description: "Internal plan",
		State:       entity.ProjectStateActive,
		TargetOn:    "2026-12-01",
	}
	h.projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, project.ID).Return(project, nil).AnyTimes()
	h.roadmaps.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.AddEntry(context.Background(), h.workspaceID, service.AddRoadmapEntryInput{
		ExpectedVersion: 3,
		Kind:            entity.RoadmapEntryProject,
		SourceID:        project.ID,
		Content:         content(""),
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) || validation.Fields[0].Field != entity.RoadmapFieldPublicTitle {
		t.Fatalf("AddEntry with no public title = %v, want a required public title", err)
	}
}

func TestAddingAProjectSnapshotsItsStatusAndDateButNeverItsCountsOrText(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	project := entity.Project{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		Name:        "Internal codename",
		Description: "Internal plan",
		State:       entity.ProjectStateActive,
		TargetOn:    "2026-12-01",
	}
	h.projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, project.ID).Return(project, nil)
	h.expectSaveAndView(nil, nil)

	var created entity.RoadmapEntry

	h.roadmaps.EXPECT().CreateEntry(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.RoadmapEntry) (entity.RoadmapEntry, error) {
			created = entry

			return entry, nil
		})

	view, err := h.service.AddEntry(context.Background(), h.workspaceID, service.AddRoadmapEntryInput{
		ExpectedVersion: 3,
		Kind:            entity.RoadmapEntryProject,
		SourceID:        project.ID,
		Content:         content("Faster sync"),
	})
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	if created.PublicTitle != "Faster sync" || created.PublicDescription != "" {
		t.Fatalf("public copy = %q / %q, want only what the author wrote", created.PublicTitle, created.PublicDescription)
	}

	if created.Status != entity.PublicStatusInProgress || created.Timeline.EndOn != "2026-12-01" || created.Timeline.StartOn != "" {
		t.Fatalf("snapshot = %s %+v, want in_progress ending on the target date", created.Status, created.Timeline)
	}

	if !entity.ValidRoadmapPublicID(created.PublicID) || created.SourceProjectID != project.ID {
		t.Fatalf("entry identity = %q / %s", created.PublicID, created.SourceProjectID)
	}

	if view.Roadmap.Version != 4 {
		t.Fatalf("draft version = %d, want 4 after one change", view.Roadmap.Version)
	}
}

func TestAConcurrentDraftChangeIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(5)
	h.roadmaps.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := h.service.Update(context.Background(), h.workspaceID, service.UpdateRoadmapInput{
		ExpectedVersion: 4,
		Title:           "Mine",
	})

	var stale entity.RoadmapStaleError
	if !errors.As(err, &stale) || stale.Version != 5 {
		t.Fatalf("Update on an old version = %v, want RoadmapStaleError at version 5", err)
	}

	if _, err := h.service.Publish(context.Background(), h.workspaceID, reviewed(4)); !errors.Is(err, entity.ErrRoadmapStale) {
		t.Fatalf("Publish of an unreviewed version = %v, want ErrRoadmapStale", err)
	}
}

func TestPublishingReverifiesEverySourceAgainstThePublisher(t *testing.T) {
	visibleTeam := uuid.New()

	for _, test := range []struct {
		name   string
		source entity.RoadmapSource
	}{
		{name: "deleted", source: entity.RoadmapSource{}},
		{name: "archived", source: entity.RoadmapSource{Present: true, Active: false, TeamID: visibleTeam}},
		{name: "team out of reach", source: entity.RoadmapSource{Present: true, Active: true, TeamID: uuid.New()}},
		{name: "another workspace", source: entity.RoadmapSource{Present: true, Active: true, TeamID: visibleTeam, WorkspaceID: uuid.New()}},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t)
			h.actAs(entity.TeamScope{TeamIDs: []uuid.UUID{visibleTeam}})
			h.lockAt(3)

			issue := h.entry(entity.RoadmapEntryIssue, "Public")
			source := test.source
			if source.Present && source.WorkspaceID == uuid.Nil {
				source.WorkspaceID = h.workspaceID
			}

			h.roadmaps.EXPECT().ListEntries(gomock.Any(), h.roadmap.ID).Return([]entity.RoadmapEntry{issue}, nil)
			h.roadmaps.EXPECT().EntrySources(gomock.Any(), h.roadmap.ID).
				Return(map[uuid.UUID]entity.RoadmapSource{issue.ID: source}, nil)
			h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			_, err := h.service.Publish(context.Background(), h.workspaceID, reviewed(3))

			var unreachable entity.RoadmapUnreachableError
			if !errors.As(err, &unreachable) || unreachable.EntryID != issue.ID {
				t.Fatalf("Publish = %v, want the entry reported unreachable", err)
			}

			if len(h.recorded) != 0 {
				t.Fatalf("a refused publication was audited: %+v", h.recorded)
			}
		})
	}
}

func TestPublishingCapturesTheApprovedScopeAndNeverRevealsAnUnpublishedProject(t *testing.T) {
	h := newHarness(t)
	team := uuid.New()
	h.actAs(entity.TeamScope{TeamIDs: []uuid.UUID{team}})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Public project")
	grouped := h.entry(entity.RoadmapEntryIssue, "Grouped")
	grouped.ParentEntryID = project.ID
	moved := h.entry(entity.RoadmapEntryIssue, "Moved away")
	moved.ParentEntryID = project.ID
	alone := h.entry(entity.RoadmapEntryIssue, "Alone")
	elsewhere := uuid.New()

	sources := map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
		grouped.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: team, ProjectID: project.SourceProjectID},
		moved.ID:   {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: team, ProjectID: elsewhere},
		alone.ID:   {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: team, TeamPrivate: true, ProjectID: elsewhere},
	}

	h.expectSaveAndView([]entity.RoadmapEntry{project, grouped, moved, alone}, sources)
	h.expectPublish()
	h.publish(t)

	publication, published := *h.publication, h.published

	if publication.Version != 3 || publication.Title != h.roadmap.Title || publication.PublishedBy != h.accountID {
		t.Fatalf("publication = %+v, want the reviewed version 3 by the publisher", publication)
	}

	byTitle := make(map[string]entity.PublishedRoadmapEntry)
	for _, entry := range published {
		byTitle[entry.PublicTitle] = entry
	}

	if got := byTitle["Grouped"].ParentPublicID; got != project.PublicID {
		t.Fatalf("grouped parent = %q, want the published project", got)
	}

	if got := byTitle["Moved away"].ParentPublicID; got != "" {
		t.Fatalf("an issue that left the project still names it: %q", got)
	}

	approval := byTitle["Alone"].Approval
	if approval.TeamID != team || !approval.TeamPrivate || approval.ProjectID != elsewhere {
		t.Fatalf("approval = %+v, want the live team, its privacy and project captured", approval)
	}

	if len(h.recorded) != 1 || h.recorded[0].Action != entity.AuditRoadmapPublished {
		t.Fatalf("audit = %+v, want one publication entry", h.recorded)
	}

	for key, value := range h.recorded[0].Detail {
		if key != "version" && key != "entries" {
			t.Fatalf("publication audit carries %s=%q; it records counts, never copy", key, value)
		}
	}
}

func TestATitlelessDraftPreviewsButCannotBePublished(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.roadmap.Title = ""
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	h.expectDraft([]entity.RoadmapEntry{project}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
	})
	h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	preview, err := h.service.Preview(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(preview.Roadmap.Projects) != 1 || len(preview.Blockers) != 1 || preview.Blockers[0].Code != entity.RoadmapBlockerTitleRequired {
		t.Fatalf("preview = %+v, want the project rendered and the missing title named", preview)
	}

	var validation entity.ValidationError
	if _, err := h.service.Publish(context.Background(), h.workspaceID, service.PublishRoadmapInput{
		ExpectedVersion: preview.Version,
		ExpectedDigest:  preview.Digest,
	}); !errors.As(err, &validation) {
		t.Fatalf("Publish without a title = %v, want a validation error", err)
	}
}

func TestRefreshUpdatesTheDraftSnapshotButNeverItsPublicCopy(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	entry := h.entry(entity.RoadmapEntryProject, "Faster sync")
	entry.PublicDescription = "Written for customers"
	entry.Timeline = entity.RoadmapTimeline{StartOn: "2026-10-01", EndOn: "2026-11-01"}

	h.roadmaps.EXPECT().GetEntry(gomock.Any(), h.roadmap.ID, entry.ID).Return(entry, nil)
	h.projects.EXPECT().GetByID(gomock.Any(), h.workspaceID, entry.SourceProjectID).Return(entity.Project{
		ID:          entry.SourceProjectID,
		WorkspaceID: h.workspaceID,
		Name:        "Renamed internally",
		Description: "Edited internally after publication",
		State:       entity.ProjectStateCompleted,
		TargetOn:    "2026-12-15",
	}, nil)
	h.expectSaveAndView([]entity.RoadmapEntry{entry}, nil)

	var saved entity.RoadmapEntry

	h.roadmaps.EXPECT().UpdateEntry(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, updated entity.RoadmapEntry) (entity.RoadmapEntry, error) {
			saved = updated

			return updated, nil
		})
	h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.RefreshEntry(context.Background(), h.workspaceID, entry.ID, 3); err != nil {
		t.Fatalf("RefreshEntry: %v", err)
	}

	if saved.PublicTitle != "Faster sync" || saved.PublicDescription != "Written for customers" {
		t.Fatalf("refresh rewrote public copy to %q / %q", saved.PublicTitle, saved.PublicDescription)
	}

	if saved.Status != entity.PublicStatusCompleted || saved.Timeline.EndOn != "2026-12-15" || saved.Timeline.StartOn != "2026-10-01" {
		t.Fatalf("snapshot = %s %+v, want the refreshed status and date in the draft", saved.Status, saved.Timeline)
	}
}

func TestEditingAnEntryWithoutAStatusKeepsItsSnapshot(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	entry := h.entry(entity.RoadmapEntryIssue, "Offline edits")
	entry.Status = entity.PublicStatusInProgress

	h.roadmaps.EXPECT().GetEntry(gomock.Any(), h.roadmap.ID, entry.ID).Return(entry, nil)
	h.expectSaveAndView([]entity.RoadmapEntry{entry}, nil)

	var saved entity.RoadmapEntry

	h.roadmaps.EXPECT().UpdateEntry(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, updated entity.RoadmapEntry) (entity.RoadmapEntry, error) {
			saved = updated

			return updated, nil
		})

	if _, err := h.service.UpdateEntry(context.Background(), h.workspaceID, entry.ID, service.UpdateRoadmapEntryInput{
		ExpectedVersion: 3,
		Content:         content("Edits that survive going offline"),
	}); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	if saved.PublicTitle != "Edits that survive going offline" || saved.Status != entity.PublicStatusInProgress {
		t.Fatalf("saved = %q %s, want the new title over the existing in_progress snapshot", saved.PublicTitle, saved.Status)
	}
}

func TestRemovingAnEntryRevokesItsPublicCopyImmediately(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	entry := h.entry(entity.RoadmapEntryIssue, "Gone soon")
	entry.PublicDescription = "Sensitive public description"

	h.roadmaps.EXPECT().GetEntry(gomock.Any(), h.roadmap.ID, entry.ID).Return(entry, nil)
	h.roadmaps.EXPECT().DeleteEntry(gomock.Any(), h.roadmap.ID, entry.ID).Return(nil)
	h.roadmaps.EXPECT().DeletePublishedEntry(gomock.Any(), h.roadmap.ID, entry.PublicID).Return(nil)
	h.expectSaveAndView(nil, nil)

	if _, err := h.service.RemoveEntry(context.Background(), h.workspaceID, entry.ID, 3); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}

	if len(h.recorded) != 1 || h.recorded[0].Action != entity.AuditRoadmapEntryRemoved {
		t.Fatalf("audit = %+v, want the removal recorded", h.recorded)
	}

	for _, value := range h.recorded[0].Detail {
		if strings.Contains(value, "Sensitive") || strings.Contains(value, "Gone soon") {
			t.Fatalf("the removal audit recorded public copy: %+v", h.recorded[0].Detail)
		}
	}
}

func TestUnpublishingWithdrawsTheRoadmapAndAuditsOnce(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.roadmap.Publication = &entity.RoadmapPublication{Title: "Live", Version: 3, PublishedAt: time.Now()}
	h.lockAt(3)
	h.roadmaps.EXPECT().Unpublish(gomock.Any(), h.roadmap.ID).Return(nil)
	h.expectSaveAndView(nil, nil)

	view, err := h.service.Unpublish(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Unpublish: %v", err)
	}

	if view.Roadmap.Published() {
		t.Fatal("the roadmap still reads as published")
	}

	if len(h.recorded) != 1 || h.recorded[0].Action != entity.AuditRoadmapUnpublished {
		t.Fatalf("audit = %+v, want one unpublish entry", h.recorded)
	}
}

func zeroed(roadmap entity.PublicRoadmap) entity.PublicRoadmap {
	roadmap.PublishedAt = time.Time{}

	return roadmap
}

func TestThePreviewIsExactlyWhatPublishingWrites(t *testing.T) {
	h := newHarness(t)
	team := uuid.New()
	h.actAs(entity.TeamScope{TeamIDs: []uuid.UUID{team}})
	h.roadmap.Visibility = entity.RoadmapVisibility{IssueLists: true, Status: true, Progress: true}
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	done := h.entry(entity.RoadmapEntryIssue, "Offline edits")
	done.ParentEntryID = project.ID
	done.Status = entity.PublicStatusCompleted
	open := h.entry(entity.RoadmapEntryIssue, "Conflict hints")
	open.ParentEntryID = project.ID
	project.Visibility = entity.RoadmapVisibility{IssueLists: true, Status: true, Progress: true}

	live := func(projectID uuid.UUID) entity.RoadmapSource {
		return entity.RoadmapSource{Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: team, ProjectID: projectID}
	}

	h.expectSaveAndView([]entity.RoadmapEntry{project, done, open}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
		done.ID:    live(project.SourceProjectID),
		open.ID:    live(project.SourceProjectID),
	})
	h.expectPublish()

	preview, err := h.service.Preview(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.Version != 3 || len(preview.Blockers) != 0 {
		t.Fatalf("preview = version %d, blockers %+v; want version 3 and nothing blocking", preview.Version, preview.Blockers)
	}

	if progress := preview.Roadmap.Projects[0].Progress; progress == nil || *progress != 50 {
		t.Fatalf("previewed progress = %v, want 50", progress)
	}

	view := h.publish(t)

	written := entity.NewPublicRoadmap(entity.PublishedRoadmap{
		WorkspaceID: h.workspaceID,
		Publication: *h.publication,
		Entries:     h.published,
	})

	if !reflect.DeepEqual(zeroed(written), zeroed(preview.Roadmap)) {
		t.Fatalf("published %+v, previewed %+v; the preview must be what publishing writes", written, preview.Roadmap)
	}

	if view.Review.HasUnpublishedChanges || len(view.Review.Entries) != 0 {
		t.Fatalf("review right after publishing = %+v, want nothing left", view.Review)
	}

	for _, entry := range view.Entries {
		if entry.Entry.ID == project.ID && entry.ProgressCoverage != 2 {
			t.Fatalf("progress coverage = %d, want the 2 selected issues", entry.ProgressCoverage)
		}
	}
}

func TestAnUnreachableEntryIsLeftOutOfThePreviewAndStillRefusesPublication(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{TeamIDs: []uuid.UUID{uuid.New()}})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	hidden := h.entry(entity.RoadmapEntryIssue, "Out of reach")

	h.expectDraft([]entity.RoadmapEntry{project, hidden}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
		hidden.ID:  {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: uuid.New()},
	})
	h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	preview, err := h.service.Preview(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(preview.Roadmap.Projects) != 1 || len(preview.Roadmap.Issues) != 0 {
		t.Fatalf("preview = %+v, want only the reachable project", preview.Roadmap)
	}

	if len(preview.Blockers) != 1 || preview.Blockers[0].EntryID != hidden.ID || preview.Blockers[0].Code != entity.RoadmapBlockerEntryUnreachable {
		t.Fatalf("blockers = %+v, want the unreachable issue named", preview.Blockers)
	}

	var unreachable entity.RoadmapUnreachableError
	if _, err := h.service.Publish(context.Background(), h.workspaceID, service.PublishRoadmapInput{
		ExpectedVersion: preview.Version,
		ExpectedDigest:  preview.Digest,
	}); !errors.As(err, &unreachable) || unreachable.EntryID != hidden.ID {
		t.Fatalf("Publish = %v, want the entry reported unreachable", err)
	}
}

func TestEditsAfterPublicationReadAsUnpublishedChanges(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	sources := map[uuid.UUID]entity.RoadmapSource{project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true}}

	h.expectSaveAndView([]entity.RoadmapEntry{project}, sources)
	h.publication = &entity.RoadmapPublication{Title: h.roadmap.Title, Version: 2, PublishedAt: time.Now()}
	h.published = []entity.PublishedRoadmapEntry{{
		RoadmapEntry: func() entity.RoadmapEntry {
			before := project
			before.PublicTitle = "Sync, but faster"

			return before
		}(),
		Source: sources[project.ID],
	}}

	view, err := h.service.Get(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !view.Review.HasUnpublishedChanges || len(view.Review.Entries) != 1 {
		t.Fatalf("review = %+v, want the edited entry listed", view.Review)
	}

	title := view.Review.Entries[0].Fields[0]
	if title.Field != entity.RoadmapReviewTitle || title.Before != "Sync, but faster" || title.After != "Faster sync" {
		t.Fatalf("title change = %+v, want what is public and what will be", title)
	}
}

func TestAReviewThatNoLongerDescribesTheDraftIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	h.expectDraft([]entity.RoadmapEntry{project}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
	})
	h.roadmaps.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.Publish(context.Background(), h.workspaceID, reviewed(3)); !errors.Is(err, entity.ErrRoadmapReviewChanged) {
		t.Fatalf("Publish with a digest of another review = %v, want ErrRoadmapReviewChanged", err)
	}

	if len(h.recorded) != 0 {
		t.Fatalf("a refused publication was audited: %+v", h.recorded)
	}
}

func TestThePublicAddressNeedsARoadmapDomain(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)
	h.expectSaveAndView(nil, nil)

	view, err := h.service.Get(context.Background(), h.workspaceID)
	if err != nil || view.PublicURL != "https://acme.roadmap.example.com" {
		t.Fatalf("Get = %q, %v; want the workspace's roadmap address", view.PublicURL, err)
	}

	h.withDomain("")

	view, err = h.service.Get(context.Background(), h.workspaceID)
	if err != nil || view.PublicURL != "" {
		t.Fatalf("Get without a domain = %q, %v; want no address", view.PublicURL, err)
	}
}

func TestThePreviewAsksForTheRoadmapGrant(t *testing.T) {
	h := newHarness(t)
	denied := entity.AccessDeniedError{Reason: entity.DenyReasonRoleLacksAction, Resource: entity.ResourceRoadmap}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.Resource != entity.ResourceRoadmap || request.Action != entity.ActionRead {
				t.Fatalf("preview asked for %s %s, want roadmap read", request.Resource, request.Action)
			}

			return entity.Decision{}, denied
		}).
		Times(2)
	h.roadmaps.EXPECT().Ensure(gomock.Any(), gomock.Any()).Times(0)

	if _, err := h.service.Preview(context.Background(), h.workspaceID); !errors.As(err, &entity.AccessDeniedError{}) {
		t.Fatalf("Preview = %v, want access denied", err)
	}

	if _, err := h.service.PreviewEntry(context.Background(), h.workspaceID, "abcdefghijklmnop"); !errors.As(err, &entity.AccessDeniedError{}) {
		t.Fatalf("PreviewEntry = %v, want access denied", err)
	}
}

func TestAPreviewEntryIsFoundOnlyByItsPublicID(t *testing.T) {
	h := newHarness(t)
	h.actAs(entity.TeamScope{AllTeams: true})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	h.expectDraft([]entity.RoadmapEntry{project}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
	})

	if item, err := h.service.PreviewEntry(context.Background(), h.workspaceID, project.PublicID); err != nil || item.Title != "Faster sync" {
		t.Fatalf("PreviewEntry(public id) = %+v, %v", item, err)
	}

	for _, guess := range []string{project.ID.String(), project.SourceProjectID.String(), ""} {
		if _, err := h.service.PreviewEntry(context.Background(), h.workspaceID, guess); !errors.Is(err, entity.ErrRoadmapEntryNotFound) {
			t.Fatalf("PreviewEntry(%q) = %v, want not found", guess, err)
		}
	}
}

func TestASourceIsNamedOnlyWhileItIsInReach(t *testing.T) {
	h := newHarness(t)
	team := uuid.New()
	h.actAs(entity.TeamScope{TeamIDs: []uuid.UUID{team}})
	h.lockAt(3)

	project := h.entry(entity.RoadmapEntryProject, "Faster sync")
	visible := h.entry(entity.RoadmapEntryIssue, "Offline edits")
	hidden := h.entry(entity.RoadmapEntryIssue, "Out of reach")

	h.expectDraft([]entity.RoadmapEntry{project, visible, hidden}, map[uuid.UUID]entity.RoadmapSource{
		project.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true},
		visible.ID: {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: team},
		hidden.ID:  {Present: true, WorkspaceID: h.workspaceID, Active: true, TeamID: uuid.New()},
	})
	h.projects.EXPECT().ListByWorkspaceID(gomock.Any(), h.workspaceID, gomock.Any()).
		Return([]entity.Project{{ID: project.SourceProjectID, Name: "Sync engine v2"}}, nil)
	h.issues.EXPECT().ListVisibleByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, ids []uuid.UUID) ([]entity.Issue, error) {
			if len(ids) != 1 || ids[0] != visible.SourceIssueID {
				t.Fatalf("looked up issues %v, want only the one in reach", ids)
			}

			return []entity.Issue{{ID: visible.SourceIssueID, Title: "CRDT merge for drafts", ReferenceKey: "ENG", Number: 7}}, nil
		})

	view, err := h.service.Get(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	byID := make(map[uuid.UUID]service.RoadmapEntryView, len(view.Entries))
	for _, entry := range view.Entries {
		byID[entry.Entry.ID] = entry
	}

	if got := byID[project.ID]; got.SourceLabel != "Sync engine v2" || got.SourceReference != "" {
		t.Fatalf("project source = %q %q", got.SourceLabel, got.SourceReference)
	}

	if got := byID[visible.ID]; got.SourceLabel != "CRDT merge for drafts" || got.SourceReference != "ENG-7" {
		t.Fatalf("issue source = %q %q, want its title and reference", got.SourceLabel, got.SourceReference)
	}

	if got := byID[hidden.ID]; got.Reachable || got.SourceLabel != "" || got.SourceReference != "" {
		t.Fatalf("unreachable source = %+v, want it unnamed", got)
	}
}

func publicService(t *testing.T, domain string) (*roadmaprepo.MockRoadmap, service.PublicRoadmaps) {
	t.Helper()

	roadmaps := roadmaprepo.NewMockRoadmap(gomock.NewController(t))

	return roadmaps, roadmapsvc.NewPublic(roadmaps, config.Roadmaps{Domain: domain})
}

func TestGuessedPublicIdentifiersResolveToNothing(t *testing.T) {
	roadmaps, public := publicService(t, "roadmap.example.com")
	workspaceID := uuid.New()

	issueID := uuid.New()
	publicID, _ := entity.NewRoadmapPublicID()

	roadmaps.EXPECT().PublishedBySlug(gomock.Any(), "acme").Return(entity.PublishedRoadmap{
		WorkspaceID: workspaceID,
		Publication: entity.RoadmapPublication{Title: "Acme", Visibility: entity.RoadmapVisibility{Status: true}},
		Entries: []entity.PublishedRoadmapEntry{{
			RoadmapEntry: entity.RoadmapEntry{
				WorkspaceID:   workspaceID,
				PublicID:      publicID,
				Kind:          entity.RoadmapEntryIssue,
				SourceIssueID: issueID,
				PublicTitle:   "Public",
				Status:        entity.PublicStatusPlanned,
			},
			Source: entity.RoadmapSource{Present: true, WorkspaceID: workspaceID, Active: true},
		}},
	}, nil).AnyTimes()

	for _, guess := range []string{issueID.String(), "ACME-1", "aaaaaaaaaaaaaaaa", ""} {
		if _, err := public.Entry(context.Background(), "acme.roadmap.example.com", guess); !errors.Is(err, entity.ErrRoadmapEntryNotFound) {
			t.Fatalf("Entry(%q) = %v, want not found", guess, err)
		}
	}

	if item, err := public.Entry(context.Background(), "acme.roadmap.example.com", publicID); err != nil || item.Title != "Public" {
		t.Fatalf("Entry(public id) = %+v, %v", item, err)
	}

	if _, err := public.Get(context.Background(), "acme.other.example.com"); !errors.Is(err, entity.ErrRoadmapNotFound) {
		t.Fatalf("Get on a foreign host = %v, want not found", err)
	}
}

func TestNoRoadmapResolvesWithoutAConfiguredDomain(t *testing.T) {
	roadmaps, public := publicService(t, "")
	roadmaps.EXPECT().PublishedBySlug(gomock.Any(), gomock.Any()).Times(0)

	if _, err := public.Get(context.Background(), "acme.roadmap.example.com"); !errors.Is(err, entity.ErrRoadmapNotFound) {
		t.Fatalf("Get = %v, want not found", err)
	}
}
