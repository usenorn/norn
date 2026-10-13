package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type reviewFixture struct {
	roadmap entity.Roadmap
	team    uuid.UUID
	drafts  []entity.RoadmapEntry
	sources map[uuid.UUID]entity.RoadmapSource
}

func newReviewFixture(visibility entity.RoadmapVisibility) *reviewFixture {
	return &reviewFixture{
		roadmap: entity.Roadmap{
			ID:          uuid.New(),
			WorkspaceID: uuid.New(),
			Title:       "Roadmap",
			Visibility:  visibility,
			Version:     2,
		},
		team:    uuid.New(),
		sources: make(map[uuid.UUID]entity.RoadmapSource),
	}
}

func (f *reviewFixture) add(kind entity.RoadmapEntryKind, title string) int {
	publicID, _ := entity.NewRoadmapPublicID()

	entry := entity.RoadmapEntry{
		ID:           uuid.New(),
		RoadmapID:    f.roadmap.ID,
		WorkspaceID:  f.roadmap.WorkspaceID,
		PublicID:     publicID,
		Kind:         kind,
		PublicTitle:  title,
		Status:       entity.PublicStatusPlanned,
		ProgressMode: entity.RoadmapProgressHidden,
		Visibility:   everything,
		Position:     len(f.drafts),
	}

	source := entity.RoadmapSource{Present: true, WorkspaceID: f.roadmap.WorkspaceID, Active: true}

	if kind == entity.RoadmapEntryProject {
		entry.SourceProjectID = uuid.New()
	} else {
		entry.SourceIssueID = uuid.New()
		source.TeamID = f.team
	}

	f.drafts = append(f.drafts, entry)
	f.sources[entry.ID] = source

	return len(f.drafts) - 1
}

func (f *reviewFixture) project(title string) int {
	return f.add(entity.RoadmapEntryProject, title)
}

func (f *reviewFixture) issue(title string, parent int) int {
	index := f.add(entity.RoadmapEntryIssue, title)
	project := f.drafts[parent]

	f.drafts[index].ParentEntryID = project.ID

	source := f.sources[f.drafts[index].ID]
	source.ProjectID = project.SourceProjectID
	f.sources[f.drafts[index].ID] = source

	return index
}

func (f *reviewFixture) candidate() entity.RoadmapCandidate {
	scope := entity.TeamScope{WorkspaceID: f.roadmap.WorkspaceID, TeamIDs: []uuid.UUID{f.team}}

	return entity.NewRoadmapCandidate(f.roadmap, f.drafts, f.sources, scope, time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
}

func (f *reviewFixture) published() *entity.PublishedRoadmap {
	published := f.candidate().Published

	return &published
}

func (f *reviewFixture) review(current *entity.PublishedRoadmap) entity.RoadmapReview {
	ids := make(map[string]uuid.UUID, len(f.drafts))
	for _, draft := range f.drafts {
		ids[draft.PublicID] = draft.ID
	}

	return entity.ReviewRoadmap(current, f.candidate(), ids)
}

func fieldsByName(fields []entity.RoadmapFieldChange) map[entity.RoadmapReviewField]entity.RoadmapFieldChange {
	byName := make(map[entity.RoadmapReviewField]entity.RoadmapFieldChange, len(fields))
	for _, field := range fields {
		byName[field.Field] = field
	}

	return byName
}

func TestAFirstPublicationReviewsEveryVisibleFieldAsExposed(t *testing.T) {
	f := newReviewFixture(entity.RoadmapVisibility{Status: true, Timeline: true})
	project := f.project("Faster sync")
	f.drafts[project].PublicDescription = "Hidden by the roadmap"
	f.drafts[project].Timeline = entity.RoadmapTimeline{EndOn: "2026-12-01"}

	review := f.review(nil)

	if !review.HasUnpublishedChanges {
		t.Fatal("a first publication reads as having nothing to publish")
	}

	if title := fieldsByName(review.Roadmap)[entity.RoadmapReviewTitle]; title.Kind != entity.RoadmapFieldExposed || title.After != "Roadmap" {
		t.Fatalf("roadmap title change = %+v, want the title exposed", title)
	}

	if len(review.Entries) != 1 || review.Entries[0].Change != entity.RoadmapEntryAdded || review.Entries[0].EntryID != f.drafts[project].ID {
		t.Fatalf("entries = %+v, want the project added", review.Entries)
	}

	fields := fieldsByName(review.Entries[0].Fields)
	want := map[entity.RoadmapReviewField]string{
		entity.RoadmapReviewTitle:  "Faster sync",
		entity.RoadmapReviewStatus: string(entity.PublicStatusPlanned),
		entity.RoadmapReviewEndOn:  "2026-12-01",
	}

	if len(fields) != len(want) {
		t.Fatalf("fields = %+v, want exactly %v; a field the roadmap hides is never listed", review.Entries[0].Fields, want)
	}

	for name, value := range want {
		if fields[name].Kind != entity.RoadmapFieldExposed || fields[name].After != value {
			t.Fatalf("%s = %+v, want %q exposed", name, fields[name], value)
		}
	}
}

func TestTheJustPublishedDraftHasNothingLeftToReview(t *testing.T) {
	f := newReviewFixture(everything)
	project := f.project("Faster sync")
	f.issue("Offline edits", project)

	current := f.published()
	review := f.review(current)

	if review.HasUnpublishedChanges || len(review.Entries) != 0 || len(review.Roadmap) != 0 {
		t.Fatalf("review = %+v, want nothing to publish", review)
	}
}

func TestAnEditedTitleShowsWhatItWasAndWhatItWillBe(t *testing.T) {
	f := newReviewFixture(everything)
	project := f.project("Faster sync")
	current := f.published()

	f.roadmap.Title = "What we build next"
	f.drafts[project].PublicTitle = "Instant sync"

	review := f.review(current)

	roadmapTitle := fieldsByName(review.Roadmap)[entity.RoadmapReviewTitle]
	if roadmapTitle.Kind != entity.RoadmapFieldChanged || roadmapTitle.Before != "Roadmap" || roadmapTitle.After != "What we build next" {
		t.Fatalf("roadmap title = %+v", roadmapTitle)
	}

	if len(review.Entries) != 1 || review.Entries[0].Change != entity.RoadmapEntryChanged {
		t.Fatalf("entries = %+v, want the project changed", review.Entries)
	}

	title := fieldsByName(review.Entries[0].Fields)[entity.RoadmapReviewTitle]
	if title.Kind != entity.RoadmapFieldChanged || title.Before != "Faster sync" || title.After != "Instant sync" {
		t.Fatalf("entry title = %+v, want Faster sync -> Instant sync", title)
	}
}

func TestACopyEditNobodyCanSeeIsNotAChange(t *testing.T) {
	f := newReviewFixture(entity.RoadmapVisibility{Status: true})
	project := f.project("Faster sync")
	current := f.published()

	f.drafts[project].PublicDescription = "Rewritten while hidden"
	f.drafts[project].Visibility.Progress = false

	if review := f.review(current); review.HasUnpublishedChanges {
		t.Fatalf("review = %+v, want no change: nothing visible moved", review)
	}
}

func TestShowingTheTimelineDisclosesExactlyTheStoredDates(t *testing.T) {
	f := newReviewFixture(entity.RoadmapVisibility{Status: true})
	project := f.project("Faster sync")
	f.drafts[project].Timeline = entity.RoadmapTimeline{StartOn: "2026-10-01", EndOn: "2026-12-01"}
	current := f.published()

	f.roadmap.Visibility.Timeline = true

	review := f.review(current)
	if len(review.Entries) != 1 {
		t.Fatalf("entries = %+v, want the project changed", review.Entries)
	}

	fields := fieldsByName(review.Entries[0].Fields)

	if fields[entity.RoadmapReviewStartOn].Kind != entity.RoadmapFieldExposed || fields[entity.RoadmapReviewStartOn].After != "2026-10-01" {
		t.Fatalf("startOn = %+v, want 2026-10-01 exposed", fields[entity.RoadmapReviewStartOn])
	}

	if fields[entity.RoadmapReviewEndOn].Kind != entity.RoadmapFieldExposed || fields[entity.RoadmapReviewEndOn].After != "2026-12-01" {
		t.Fatalf("endOn = %+v, want 2026-12-01 exposed", fields[entity.RoadmapReviewEndOn])
	}
}

func TestShowingIssueListsPlacesIssuesUnderTheirProject(t *testing.T) {
	f := newReviewFixture(entity.RoadmapVisibility{Status: true})
	project := f.project("Faster sync")
	issue := f.issue("Offline edits", project)
	current := f.published()

	f.roadmap.Visibility.IssueLists = true

	review := f.review(current)
	byID := make(map[uuid.UUID]entity.RoadmapEntryReview, len(review.Entries))

	for _, entry := range review.Entries {
		byID[entry.EntryID] = entry
	}

	issues := fieldsByName(byID[f.drafts[project].ID].Fields)[entity.RoadmapReviewIssues]
	if issues.Kind != entity.RoadmapFieldExposed || issues.After != "1" {
		t.Fatalf("project issues = %+v, want a list of one exposed", issues)
	}

	placement := fieldsByName(byID[f.drafts[issue].ID].Fields)[entity.RoadmapReviewPlacement]
	if placement.Kind != entity.RoadmapFieldExposed || placement.After != "Faster sync" {
		t.Fatalf("issue placement = %+v, want it listed under Faster sync", placement)
	}
}

func TestAnEntryThatCannotBePublishedIsLeftOutAndBlocksPublication(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(f *reviewFixture, index int)
		code   entity.RoadmapBlockerCode
		check  func(error) bool
	}{
		{
			name: "out of reach",
			mutate: func(f *reviewFixture, index int) {
				source := f.sources[f.drafts[index].ID]
				source.TeamID = uuid.New()
				f.sources[f.drafts[index].ID] = source
			},
			code:  entity.RoadmapBlockerEntryUnreachable,
			check: func(err error) bool { return errors.As(err, &entity.RoadmapUnreachableError{}) },
		},
		{
			name:   "deleted",
			mutate: func(f *reviewFixture, index int) { delete(f.sources, f.drafts[index].ID) },
			code:   entity.RoadmapBlockerEntryUnreachable,
			check:  func(err error) bool { return errors.Is(err, entity.ErrRoadmapUnreachable) },
		},
		{
			name:   "invalid copy",
			mutate: func(f *reviewFixture, index int) { f.drafts[index].PublicTitle = " " },
			code:   entity.RoadmapBlockerEntryInvalid,
			check:  func(err error) bool { return errors.As(err, &entity.ValidationError{}) },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newReviewFixture(everything)
			f.project("Faster sync")
			blocked := f.add(entity.RoadmapEntryIssue, "Offline edits")
			test.mutate(f, blocked)

			candidate := f.candidate()

			if public := candidate.Public(); len(public.Projects) != 1 || len(public.Issues) != 0 {
				t.Fatalf("preview = %+v, want only the project", public)
			}

			if len(candidate.Blockers) != 1 || candidate.Blockers[0].Code != test.code || candidate.Blockers[0].EntryID != f.drafts[blocked].ID {
				t.Fatalf("blockers = %+v, want %s on the entry", candidate.Blockers, test.code)
			}

			if err := candidate.Err(); !test.check(err) {
				t.Fatalf("Err() = %v, want the same refusal publishing has always given", err)
			}
		})
	}
}

func TestADraftWithoutAUsableTitleStillPreviewsButCannotPublish(t *testing.T) {
	long := make([]rune, entity.RoadmapTitleMaxLen+1)
	for i := range long {
		long[i] = 'a'
	}

	for title, code := range map[string]entity.RoadmapBlockerCode{
		"":           entity.RoadmapBlockerTitleRequired,
		string(long): entity.RoadmapBlockerTitleTooLong,
	} {
		f := newReviewFixture(everything)
		f.roadmap.Title = title
		f.project("Faster sync")
		unreachable := f.add(entity.RoadmapEntryIssue, "Offline edits")
		delete(f.sources, f.drafts[unreachable].ID)

		candidate := f.candidate()

		if public := candidate.Public(); len(public.Projects) != 1 {
			t.Fatalf("preview = %+v, want the project rendered", public)
		}

		if len(candidate.Blockers) != 2 || candidate.Blockers[0].Code != code {
			t.Fatalf("blockers = %+v, want %s first", candidate.Blockers, code)
		}

		var validation entity.ValidationError
		if err := candidate.Err(); !errors.As(err, &validation) || validation.Fields[0].Field != entity.RoadmapFieldTitle {
			t.Fatalf("Err() = %v, want the title refused before any entry", err)
		}
	}
}

func TestAWithheldEntryThatIsBackInReachReviewsAsAdded(t *testing.T) {
	f := newReviewFixture(everything)
	issue := f.add(entity.RoadmapEntryIssue, "Offline edits")
	current := f.published()

	current.Entries[0].Source.TeamPrivate = true

	review := f.review(current)

	if len(review.Entries) != 1 || review.Entries[0].Change != entity.RoadmapEntryAdded || review.Entries[0].EntryID != f.drafts[issue].ID {
		t.Fatalf("entries = %+v, want the withheld issue added again", review.Entries)
	}
}

func TestAnEntryThatDropsOutReviewsAsRemoved(t *testing.T) {
	f := newReviewFixture(everything)
	issue := f.add(entity.RoadmapEntryIssue, "Offline edits")
	current := f.published()

	delete(f.sources, f.drafts[issue].ID)

	review := f.review(current)

	if len(review.Entries) != 1 || review.Entries[0].Change != entity.RoadmapEntryRemoved || review.Entries[0].Title != "Offline edits" {
		t.Fatalf("entries = %+v, want the issue removed", review.Entries)
	}

	if title := fieldsByName(review.Entries[0].Fields)[entity.RoadmapReviewTitle]; title.Kind != entity.RoadmapFieldConcealed {
		t.Fatalf("title = %+v, want it concealed", title)
	}

	if len(review.Blockers) != 1 || review.Blockers[0].Code != entity.RoadmapBlockerEntryUnreachable {
		t.Fatalf("blockers = %+v, want the unreachable issue named", review.Blockers)
	}
}

func TestProgressCoverageCountsOnlyTheSelectedIssuesItCalculatesFrom(t *testing.T) {
	f := newReviewFixture(everything)
	project := f.project("Faster sync")
	other := f.project("Billing")
	f.issue("Done", project)
	f.issue("Open", project)
	dropped := f.issue("Dropped", project)
	f.drafts[dropped].Status = entity.PublicStatusCancelled
	moved := f.issue("Moved", project)
	source := f.sources[f.drafts[moved].ID]
	source.ProjectID = f.drafts[other].SourceProjectID
	f.sources[f.drafts[moved].ID] = source

	coverage := entity.RoadmapProgressCoverage(f.candidate().Published)

	if got := coverage[f.drafts[project].PublicID]; got != 2 {
		t.Fatalf("coverage = %d, want 2: cancelled issues and issues now in another project are not counted", got)
	}

	if got, ok := coverage[f.drafts[other].PublicID]; !ok || got != 0 {
		t.Fatalf("other coverage = %d, %v; want 0 for a project with no selected issues", got, ok)
	}
}

func TestTheDigestIgnoresWhenItWasTakenButNotWhatIsShown(t *testing.T) {
	f := newReviewFixture(everything)
	f.project("Faster sync")

	candidate := f.candidate()
	later := candidate
	later.Published.Publication.PublishedAt = candidate.Published.Publication.PublishedAt.Add(time.Hour)

	if entity.RoadmapReviewDigest(nil, candidate) != entity.RoadmapReviewDigest(nil, later) {
		t.Fatal("the digest changed with the publication time")
	}

	f.drafts[0].PublicTitle = "Instant sync"

	if entity.RoadmapReviewDigest(nil, candidate) == entity.RoadmapReviewDigest(nil, f.candidate()) {
		t.Fatal("the digest did not change with a visible title")
	}
}

func TestTheDigestChangesWithWhatIsPublishedNow(t *testing.T) {
	f := newReviewFixture(everything)
	f.project("Faster sync")

	current := f.published()

	if entity.RoadmapReviewDigest(current, f.candidate()) == entity.RoadmapReviewDigest(nil, f.candidate()) {
		t.Fatal("the digest of a review of a live roadmap matches the digest of a review after it was withdrawn")
	}
}

func TestTheDigestChangesWhenAnIssueTeamTurnsPrivate(t *testing.T) {
	f := newReviewFixture(everything)
	project := f.project("Faster sync")
	issue := f.issue("Offline edits", project)

	reviewed := entity.RoadmapReviewDigest(nil, f.candidate())

	source := f.sources[f.drafts[issue].ID]
	source.TeamPrivate = true
	f.sources[f.drafts[issue].ID] = source

	if entity.RoadmapReviewDigest(nil, f.candidate()) == reviewed {
		t.Fatal("the digest did not change when the approval it would record became private")
	}
}

func TestThePublicAddressFollowsTheAppSchemeAndPort(t *testing.T) {
	for _, test := range []struct {
		base   string
		domain string
		slug   string
		want   string
	}{
		{base: "https://app.norn.dev", domain: "roadmap.norn.dev", slug: "acme", want: "https://acme.roadmap.norn.dev"},
		{base: "http://localhost:5173", domain: "Roadmap.Localhost.", slug: "acme", want: "http://acme.roadmap.localhost:5173"},
		{base: "", domain: "roadmap.norn.dev", slug: "acme", want: "https://acme.roadmap.norn.dev"},
		{base: "https://app.norn.dev", domain: "", slug: "acme", want: ""},
		{base: "https://app.norn.dev", domain: "roadmap.norn.dev", slug: "", want: ""},
	} {
		if got := entity.RoadmapPublicURL(test.base, test.domain, test.slug); got != test.want {
			t.Errorf("RoadmapPublicURL(%q, %q, %q) = %q, want %q", test.base, test.domain, test.slug, got, test.want)
		}
	}
}
