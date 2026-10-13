package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

var everything = entity.RoadmapVisibility{
	IssueLists:          true,
	ProjectDescriptions: true,
	IssueDescriptions:   true,
	Status:              true,
	Timeline:            true,
	Progress:            true,
}

func publishedProject(workspaceID uuid.UUID, publicID string, position int) entity.PublishedRoadmapEntry {
	return entity.PublishedRoadmapEntry{
		RoadmapEntry: entity.RoadmapEntry{
			WorkspaceID:       workspaceID,
			PublicID:          publicID,
			Kind:              entity.RoadmapEntryProject,
			SourceProjectID:   uuid.New(),
			PublicTitle:       "Public " + publicID,
			PublicDescription: "About " + publicID,
			Status:            entity.PublicStatusInProgress,
			Timeline:          entity.RoadmapTimeline{EndOn: "2026-12-01"},
			ProgressMode:      entity.RoadmapProgressCalculated,
			Visibility:        everything,
			Position:          position,
		},
		Source: entity.RoadmapSource{Present: true, WorkspaceID: workspaceID, Active: true},
	}
}

func publishedIssue(
	workspaceID uuid.UUID,
	publicID string,
	parent string,
	status entity.PublicStatus,
	position int,
) entity.PublishedRoadmapEntry {
	teamID := uuid.New()

	return entity.PublishedRoadmapEntry{
		RoadmapEntry: entity.RoadmapEntry{
			WorkspaceID:       workspaceID,
			PublicID:          publicID,
			Kind:              entity.RoadmapEntryIssue,
			SourceIssueID:     uuid.New(),
			PublicTitle:       "Public " + publicID,
			PublicDescription: "About " + publicID,
			Status:            status,
			ProgressMode:      entity.RoadmapProgressHidden,
			Visibility:        everything,
			Position:          position,
		},
		ParentPublicID: parent,
		Approval:       entity.RoadmapApproval{TeamID: teamID},
		Source:         entity.RoadmapSource{Present: true, WorkspaceID: workspaceID, Active: true, TeamID: teamID},
	}
}

func roadmapOf(entries ...entity.PublishedRoadmapEntry) entity.PublishedRoadmap {
	return entity.PublishedRoadmap{
		Publication: entity.RoadmapPublication{
			Title:       "Roadmap",
			Visibility:  everything,
			Version:     1,
			PublishedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
		Entries: entries,
	}
}

func TestARoadmapLevelRestrictionCapsWhatAnEntryMayExpose(t *testing.T) {
	roadmap := entity.RoadmapVisibility{Status: true, Timeline: true}
	entry := entity.RoadmapVisibility{Status: true, Progress: true, IssueDescriptions: true}

	got := roadmap.Bound(entry)
	want := entity.RoadmapVisibility{Status: true}

	if got != want {
		t.Fatalf("Bound = %+v, want %+v. The roadmap setting is an upper bound; an entry can only expose less.", got, want)
	}
}

func TestAPublishedSourceFailsClosedOnceItLeavesTheApprovedScope(t *testing.T) {
	workspaceID := uuid.New()

	for _, test := range []struct {
		name   string
		mutate func(*entity.PublishedRoadmapEntry)
		intact bool
	}{
		{name: "untouched", mutate: func(*entity.PublishedRoadmapEntry) {}, intact: true},
		{name: "deleted", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source = entity.RoadmapSource{} }},
		{name: "another workspace", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source.WorkspaceID = uuid.New() }},
		{name: "archived", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source.Active = false }},
		{name: "moved team", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source.TeamID = uuid.New() }},
		{name: "moved project", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source.ProjectID = uuid.New() }},
		{name: "team made private", mutate: func(e *entity.PublishedRoadmapEntry) { e.Source.TeamPrivate = true }},
		{
			name: "private team approved as private",
			mutate: func(e *entity.PublishedRoadmapEntry) {
				e.Source.TeamPrivate = true
				e.Approval.TeamPrivate = true
			},
			intact: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := publishedIssue(workspaceID, "issue", "", entity.PublicStatusPlanned, 0)
			test.mutate(&entry)

			if got := entry.Intact(); got != test.intact {
				t.Fatalf("Intact() = %v, want %v. A change outside the reviewed scope must hide the entry until it is reviewed again.", got, test.intact)
			}
		})
	}
}

func TestAPublicRoadmapNeverCarriesWhatItsSettingsHide(t *testing.T) {
	workspaceID := uuid.New()
	project := publishedProject(workspaceID, "project", 0)
	issue := publishedIssue(workspaceID, "issue", "project", entity.PublicStatusCompleted, 1)
	issue.Timeline = entity.RoadmapTimeline{StartOn: "2026-10-01", EndOn: "2026-11-01"}

	published := roadmapOf(project, issue)
	published.Publication.Visibility = entity.RoadmapVisibility{IssueLists: true}

	public := entity.NewPublicRoadmap(published)

	if len(public.Projects) != 1 || len(public.Projects[0].Issues) != 1 {
		t.Fatalf("roadmap = %+v, want the project with its one issue", public)
	}

	for _, item := range []entity.PublicRoadmapItem{public.Projects[0], public.Projects[0].Issues[0]} {
		if item.Description != "" || item.Status != "" || item.Timeline != nil || item.Progress != nil {
			t.Fatalf("item %+v carries a field the roadmap hides", item)
		}
	}
}

func TestAnIssueListHiddenOnTheProjectLeavesItsIssuesUngrouped(t *testing.T) {
	workspaceID := uuid.New()
	project := publishedProject(workspaceID, "project", 0)
	project.Visibility.IssueLists = false
	issue := publishedIssue(workspaceID, "issue", "project", entity.PublicStatusPlanned, 1)

	public := entity.NewPublicRoadmap(roadmapOf(project, issue))

	if public.Projects[0].Issues != nil {
		t.Fatalf("project issues = %+v, want none listed", public.Projects[0].Issues)
	}

	if len(public.Issues) != 1 || public.Issues[0].PublicID != "issue" {
		t.Fatalf("issues = %+v, want the issue on its own", public.Issues)
	}
}

func TestAnIssueUnderAnUnpublishedProjectNeverRevealsIt(t *testing.T) {
	workspaceID := uuid.New()
	issue := publishedIssue(workspaceID, "issue", "", entity.PublicStatusPlanned, 0)

	public := entity.NewPublicRoadmap(roadmapOf(issue))

	if len(public.Projects) != 0 || len(public.Issues) != 1 {
		t.Fatalf("roadmap = %+v, want the issue alone with no project", public)
	}

	hidden := publishedProject(workspaceID, "project", 0)
	hidden.Source = entity.RoadmapSource{}
	grouped := publishedIssue(workspaceID, "grouped", "project", entity.PublicStatusPlanned, 1)

	public = entity.NewPublicRoadmap(roadmapOf(hidden, grouped))

	if len(public.Projects) != 0 || len(public.Issues) != 1 || public.Issues[0].PublicID != "grouped" {
		t.Fatalf("roadmap = %+v, want the issue alone once its project fails closed", public)
	}
}

func TestCalculatedProgressCountsOnlyPublishedVisibleIssues(t *testing.T) {
	workspaceID := uuid.New()
	project := publishedProject(workspaceID, "project", 0)
	done := publishedIssue(workspaceID, "done", "project", entity.PublicStatusCompleted, 1)
	open := publishedIssue(workspaceID, "open", "project", entity.PublicStatusInProgress, 2)
	dropped := publishedIssue(workspaceID, "dropped", "project", entity.PublicStatusCancelled, 3)
	gone := publishedIssue(workspaceID, "gone", "project", entity.PublicStatusInProgress, 4)
	gone.Source.TeamPrivate = true

	public := entity.NewPublicRoadmap(roadmapOf(project, done, open, dropped, gone))

	progress := public.Projects[0].Progress
	if progress == nil || *progress != 50 {
		t.Fatalf("progress = %v, want 50: one of the two counted issues is done, cancelled and failed-closed issues never count", progress)
	}

	if len(public.Projects[0].Issues) != 3 {
		t.Fatalf("issues = %d, want 3; the failed-closed issue must not be listed", len(public.Projects[0].Issues))
	}
}

func TestProgressModesPublishOnlyWhatWasApproved(t *testing.T) {
	workspaceID := uuid.New()

	approved := publishedProject(workspaceID, "approved", 0)
	approved.ProgressMode = entity.RoadmapProgressApproved
	approved.ProgressPercent = 70

	hidden := publishedProject(workspaceID, "hidden", 1)
	hidden.ProgressMode = entity.RoadmapProgressHidden

	empty := publishedProject(workspaceID, "empty", 2)

	public := entity.NewPublicRoadmap(roadmapOf(approved, hidden, empty))

	if got := public.Projects[0].Progress; got == nil || *got != 70 {
		t.Fatalf("approved progress = %v, want 70", got)
	}

	if got := public.Projects[1].Progress; got != nil {
		t.Fatalf("hidden progress = %v, want none", *got)
	}

	if got := public.Projects[2].Progress; got != nil {
		t.Fatalf("calculated progress with no published issues = %v, want none", *got)
	}
}

func TestPublicOrderFollowsTheAuthoredPositions(t *testing.T) {
	workspaceID := uuid.New()

	public := entity.NewPublicRoadmap(roadmapOf(
		publishedIssue(workspaceID, "second", "", entity.PublicStatusPlanned, 2),
		publishedIssue(workspaceID, "first", "", entity.PublicStatusPlanned, 1),
	))

	if public.Issues[0].PublicID != "first" || public.Issues[1].PublicID != "second" {
		t.Fatalf("order = %s, %s; want first, second", public.Issues[0].PublicID, public.Issues[1].PublicID)
	}
}

func TestAPublicItemIsFoundOnlyByItsPublicID(t *testing.T) {
	workspaceID := uuid.New()
	project := publishedProject(workspaceID, "project", 0)
	issue := publishedIssue(workspaceID, "issue", "project", entity.PublicStatusPlanned, 1)

	public := entity.NewPublicRoadmap(roadmapOf(project, issue))

	if _, ok := public.Item("issue"); !ok {
		t.Fatal("Item(issue) not found")
	}

	if _, ok := public.Item(issue.SourceIssueID.String()); ok {
		t.Fatal("an internal id resolved to a public item")
	}
}

func TestStatusSnapshotsMapOntoThePublicVocabulary(t *testing.T) {
	for state, want := range map[entity.ProjectState]entity.PublicStatus{
		entity.ProjectStatePlanned:   entity.PublicStatusPlanned,
		entity.ProjectStateActive:    entity.PublicStatusInProgress,
		entity.ProjectStatePaused:    entity.PublicStatusPaused,
		entity.ProjectStateCompleted: entity.PublicStatusCompleted,
		entity.ProjectStateCancelled: entity.PublicStatusCancelled,
	} {
		if got := entity.PublicStatusOfProject(state); got != want {
			t.Errorf("PublicStatusOfProject(%s) = %s, want %s", state, got, want)
		}
	}

	for category, want := range map[entity.StateCategory]entity.PublicStatus{
		entity.StateCategoryNotStarted: entity.PublicStatusPlanned,
		entity.StateCategoryActive:     entity.PublicStatusInProgress,
		entity.StateCategoryComplete:   entity.PublicStatusCompleted,
		entity.StateCategoryAbandoned:  entity.PublicStatusCancelled,
	} {
		if got := entity.PublicStatusOfIssue(category); got != want {
			t.Errorf("PublicStatusOfIssue(%s) = %s, want %s", category, got, want)
		}
	}
}

func TestARoadmapHostResolvesOnlyAWorkspaceSubdomain(t *testing.T) {
	const domain = "roadmap.example.com"

	for _, test := range []struct {
		host string
		slug string
		ok   bool
	}{
		{host: "acme.roadmap.example.com", slug: "acme", ok: true},
		{host: "ACME.Roadmap.Example.com:443", slug: "acme", ok: true},
		{host: "acme.roadmap.example.com.", slug: "acme", ok: true},
		{host: "roadmap.example.com"},
		{host: "a.b.roadmap.example.com"},
		{host: "acme.example.com"},
		{host: "acme.roadmap.example.com.evil.test"},
		{host: "-acme.roadmap.example.com"},
		{host: "a.roadmap.example.com"},
		{host: "127.0.0.1:8080"},
	} {
		t.Run(test.host, func(t *testing.T) {
			slug, ok := entity.RoadmapSlugFromHost(test.host, domain)

			if ok != test.ok || slug != test.slug {
				t.Fatalf("RoadmapSlugFromHost(%q) = %q, %v; want %q, %v", test.host, slug, ok, test.slug, test.ok)
			}
		})
	}

	if _, ok := entity.RoadmapSlugFromHost("acme.roadmap.example.com", ""); ok {
		t.Fatal("a host resolved with no roadmap domain configured")
	}
}

func TestPublicIDsAreRandomAndShapeChecked(t *testing.T) {
	first, err := entity.NewRoadmapPublicID()
	if err != nil {
		t.Fatalf("NewRoadmapPublicID: %v", err)
	}

	second, err := entity.NewRoadmapPublicID()
	if err != nil {
		t.Fatalf("NewRoadmapPublicID: %v", err)
	}

	if first == second {
		t.Fatal("two public ids collided")
	}

	for _, id := range []string{first, second} {
		if !entity.ValidRoadmapPublicID(id) {
			t.Fatalf("ValidRoadmapPublicID(%q) = false", id)
		}
	}

	for _, guess := range []string{"", uuid.NewString(), "NORN-240", "aaaaaaaaaaaaaaa1"} {
		if entity.ValidRoadmapPublicID(guess) {
			t.Fatalf("ValidRoadmapPublicID(%q) = true", guess)
		}
	}
}

func TestAnEntryNeedsItsOwnPublicCopy(t *testing.T) {
	valid := entity.RoadmapEntry{
		Kind:         entity.RoadmapEntryProject,
		PublicTitle:  "Faster sync",
		Status:       entity.PublicStatusPlanned,
		ProgressMode: entity.RoadmapProgressHidden,
	}

	if err := entity.ValidateRoadmapEntry(valid); err != nil {
		t.Fatalf("ValidateRoadmapEntry(valid) = %v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*entity.RoadmapEntry)
		field  string
		code   string
	}{
		{name: "blank title", mutate: func(e *entity.RoadmapEntry) { e.PublicTitle = "   " }, field: entity.RoadmapFieldPublicTitle, code: entity.ValidationCodeRequired},
		{name: "percent above 100", mutate: func(e *entity.RoadmapEntry) { e.ProgressPercent = 101 }, field: entity.RoadmapFieldProgressPercent, code: entity.ValidationCodeOutOfRange},
		{name: "end before start", mutate: func(e *entity.RoadmapEntry) {
			e.Timeline = entity.RoadmapTimeline{StartOn: "2026-11-01", EndOn: "2026-10-01"}
		}, field: entity.RoadmapFieldEndOn, code: entity.ValidationCodeOutOfRange},
		{name: "start without end", mutate: func(e *entity.RoadmapEntry) {
			e.Timeline = entity.RoadmapTimeline{StartOn: "2026-11-01"}
		}, field: entity.RoadmapFieldEndOn, code: entity.ValidationCodeRequired},
		{name: "malformed date", mutate: func(e *entity.RoadmapEntry) {
			e.Timeline = entity.RoadmapTimeline{EndOn: "soon"}
		}, field: entity.RoadmapFieldEndOn, code: entity.ValidationCodeMalformed},
		{name: "unknown status", mutate: func(e *entity.RoadmapEntry) { e.Status = "shipped" }, field: entity.RoadmapFieldStatus, code: entity.ValidationCodeUnsupportedValue},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := valid
			test.mutate(&entry)

			var validation entity.ValidationError
			if !errors.As(entity.ValidateRoadmapEntry(entry), &validation) {
				t.Fatalf("ValidateRoadmapEntry = nil, want a validation error on %s", test.field)
			}

			for _, field := range validation.Fields {
				if field.Field == test.field && field.Code == test.code {
					return
				}
			}

			t.Fatalf("fields = %+v, want %s %s", validation.Fields, test.field, test.code)
		})
	}
}
