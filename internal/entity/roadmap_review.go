package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var ErrRoadmapReviewChanged = errors.New("roadmap changed since it was reviewed")

type RoadmapBlockerCode string

const (
	RoadmapBlockerTitleRequired    RoadmapBlockerCode = "roadmap_title_required"
	RoadmapBlockerTitleTooLong     RoadmapBlockerCode = "roadmap_title_too_long"
	RoadmapBlockerEntryUnreachable RoadmapBlockerCode = "entry_unreachable"
	RoadmapBlockerEntryInvalid     RoadmapBlockerCode = "entry_invalid"
)

type RoadmapBlocker struct {
	Code    RoadmapBlockerCode
	EntryID uuid.UUID
	cause   error
}

type RoadmapReviewField string

const (
	RoadmapReviewTitle       RoadmapReviewField = "title"
	RoadmapReviewDescription RoadmapReviewField = "description"
	RoadmapReviewStatus      RoadmapReviewField = "status"
	RoadmapReviewStartOn     RoadmapReviewField = "startOn"
	RoadmapReviewEndOn       RoadmapReviewField = "endOn"
	RoadmapReviewProgress    RoadmapReviewField = "progress"
	RoadmapReviewIssues      RoadmapReviewField = "issues"
	RoadmapReviewPlacement   RoadmapReviewField = "placement"
)

type RoadmapFieldChangeKind string

const (
	RoadmapFieldExposed   RoadmapFieldChangeKind = "exposed"
	RoadmapFieldChanged   RoadmapFieldChangeKind = "changed"
	RoadmapFieldConcealed RoadmapFieldChangeKind = "concealed"
)

type RoadmapEntryChange string

const (
	RoadmapEntryAdded   RoadmapEntryChange = "added"
	RoadmapEntryRemoved RoadmapEntryChange = "removed"
	RoadmapEntryChanged RoadmapEntryChange = "changed"
)

type RoadmapFieldChange struct {
	Field  RoadmapReviewField
	Kind   RoadmapFieldChangeKind
	Before string
	After  string
}

type RoadmapEntryReview struct {
	EntryID  uuid.UUID
	PublicID string
	Kind     RoadmapEntryKind
	Title    string
	Change   RoadmapEntryChange
	Fields   []RoadmapFieldChange
}

type RoadmapReview struct {
	HasUnpublishedChanges bool
	Digest                string
	Roadmap               []RoadmapFieldChange
	Entries               []RoadmapEntryReview
	Blockers              []RoadmapBlocker
}

type RoadmapCandidate struct {
	Published PublishedRoadmap
	Blockers  []RoadmapBlocker
}

func RoadmapEntryReachable(
	workspaceID uuid.UUID,
	entry RoadmapEntry,
	source RoadmapSource,
	scope TeamScope,
) bool {
	if !source.Present || source.WorkspaceID != workspaceID || !source.Active {
		return false
	}

	return entry.Kind != RoadmapEntryIssue || scope.Covers(source.TeamID)
}

func NewRoadmapCandidate(
	roadmap Roadmap,
	drafts []RoadmapEntry,
	sources map[uuid.UUID]RoadmapSource,
	scope TeamScope,
	at time.Time,
) RoadmapCandidate {
	candidate := RoadmapCandidate{
		Published: PublishedRoadmap{
			WorkspaceID: roadmap.WorkspaceID,
			Publication: RoadmapPublication{
				Title:       roadmap.Title,
				Description: roadmap.Description,
				Visibility:  roadmap.Visibility,
				Version:     roadmap.Version,
				PublishedAt: at,
			},
			Entries: make([]PublishedRoadmapEntry, 0, len(drafts)),
		},
	}

	if field := ValidateRoadmapTitle(RoadmapFieldTitle, roadmap.Title); field.Code != "" {
		code := RoadmapBlockerTitleRequired
		if field.Code == ValidationCodeTooLong {
			code = RoadmapBlockerTitleTooLong
		}

		candidate.Blockers = append(candidate.Blockers, RoadmapBlocker{Code: code, cause: NewValidationError(field)})
	}

	accepted := make([]RoadmapEntry, 0, len(drafts))
	projects := make(map[uuid.UUID]RoadmapEntry, len(drafts))

	for _, draft := range drafts {
		if !RoadmapEntryReachable(roadmap.WorkspaceID, draft, sources[draft.ID], scope) {
			candidate.Blockers = append(candidate.Blockers, RoadmapBlocker{
				Code:    RoadmapBlockerEntryUnreachable,
				EntryID: draft.ID,
				cause:   RoadmapUnreachableError{EntryID: draft.ID},
			})

			continue
		}

		if err := ValidateRoadmapEntry(draft); err != nil {
			candidate.Blockers = append(candidate.Blockers, RoadmapBlocker{
				Code:    RoadmapBlockerEntryInvalid,
				EntryID: draft.ID,
				cause:   err,
			})

			continue
		}

		accepted = append(accepted, draft)

		if draft.Kind == RoadmapEntryProject {
			projects[draft.ID] = draft
		}
	}

	for _, draft := range accepted {
		source := sources[draft.ID]
		entry := PublishedRoadmapEntry{RoadmapEntry: draft, Source: source}

		if draft.Kind == RoadmapEntryIssue {
			entry.Approval = RoadmapApproval{
				TeamID:      source.TeamID,
				TeamPrivate: source.TeamPrivate,
				ProjectID:   source.ProjectID,
			}

			if parent, ok := projects[draft.ParentEntryID]; ok && parent.SourceProjectID == source.ProjectID {
				entry.ParentPublicID = parent.PublicID
			}
		}

		candidate.Published.Entries = append(candidate.Published.Entries, entry)
	}

	return candidate
}

func (c RoadmapCandidate) Err() error {
	if len(c.Blockers) == 0 {
		return nil
	}

	return c.Blockers[0].cause
}

func (c RoadmapCandidate) Public() PublicRoadmap {
	return NewPublicRoadmap(c.Published)
}

func RoadmapDigest(public PublicRoadmap) string {
	public.PublishedAt = time.Time{}

	encoded, err := json.Marshal(public)
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(encoded)

	return hex.EncodeToString(sum[:])
}

func RoadmapProgressCoverage(published PublishedRoadmap) map[string]int {
	coverage := make(map[string]int)

	for _, entry := range published.Entries {
		if entry.Kind == RoadmapEntryProject && entry.Intact() {
			coverage[entry.PublicID] = 0
		}
	}

	for _, entry := range published.Entries {
		if entry.Kind != RoadmapEntryIssue || !entry.Intact() || entry.Status == PublicStatusCancelled {
			continue
		}

		if counted, ok := coverage[entry.ParentPublicID]; ok {
			coverage[entry.ParentPublicID] = counted + 1
		}
	}

	return coverage
}

func RoadmapPublicURL(baseURL, domain, slug string) string {
	domain = roadmapDomain(domain)
	if domain == "" || slug == "" {
		return ""
	}

	scheme, host := "https", slug+"."+domain

	if base, err := url.Parse(baseURL); err == nil && base.Scheme != "" && base.Host != "" {
		scheme = base.Scheme

		if port := base.Port(); port != "" {
			host = net.JoinHostPort(host, port)
		}
	}

	return (&url.URL{Scheme: scheme, Host: host}).String()
}

type roadmapReviewItem struct {
	item      PublicRoadmapItem
	placement string
}

type roadmapReviewValue struct {
	field RoadmapReviewField
	value string
}

func ReviewRoadmap(
	current *PublishedRoadmap,
	candidate RoadmapCandidate,
	idsByPublicID map[string]uuid.UUID,
) RoadmapReview {
	var before PublicRoadmap
	if current != nil {
		before = NewPublicRoadmap(*current)
	}

	after := candidate.Public()
	review := RoadmapReview{
		Digest: RoadmapDigest(after),
		Roadmap: roadmapFieldChanges(
			[]roadmapReviewValue{{RoadmapReviewTitle, before.Title}, {RoadmapReviewDescription, before.Description}},
			[]roadmapReviewValue{{RoadmapReviewTitle, after.Title}, {RoadmapReviewDescription, after.Description}},
		),
		Entries:  []RoadmapEntryReview{},
		Blockers: candidate.Blockers,
	}

	beforeOrder, beforeItems := roadmapReviewItems(before)
	afterOrder, afterItems := roadmapReviewItems(after)

	for _, publicID := range afterOrder {
		previous, existed := beforeItems[publicID]
		next := afterItems[publicID]
		fields := roadmapFieldChanges(roadmapReviewValues(previous), roadmapReviewValues(next))

		change := RoadmapEntryChanged
		if !existed {
			change = RoadmapEntryAdded
		} else if len(fields) == 0 {
			continue
		}

		review.Entries = append(review.Entries, RoadmapEntryReview{
			EntryID:  idsByPublicID[publicID],
			PublicID: publicID,
			Kind:     next.item.Kind,
			Title:    next.item.Title,
			Change:   change,
			Fields:   fields,
		})
	}

	for _, publicID := range beforeOrder {
		if _, kept := afterItems[publicID]; kept {
			continue
		}

		previous := beforeItems[publicID]

		review.Entries = append(review.Entries, RoadmapEntryReview{
			EntryID:  idsByPublicID[publicID],
			PublicID: publicID,
			Kind:     previous.item.Kind,
			Title:    previous.item.Title,
			Change:   RoadmapEntryRemoved,
			Fields:   roadmapFieldChanges(roadmapReviewValues(previous), roadmapReviewValues(roadmapReviewItem{})),
		})
	}

	review.HasUnpublishedChanges = len(review.Roadmap) > 0 || len(review.Entries) > 0

	return review
}

func roadmapReviewItems(public PublicRoadmap) ([]string, map[string]roadmapReviewItem) {
	order := make([]string, 0, len(public.Projects)+len(public.Issues))
	items := make(map[string]roadmapReviewItem, cap(order))

	add := func(item PublicRoadmapItem, placement string) {
		order = append(order, item.PublicID)
		items[item.PublicID] = roadmapReviewItem{item: item, placement: placement}
	}

	for _, project := range public.Projects {
		add(project, "")

		for _, issue := range project.Issues {
			add(issue, project.Title)
		}
	}

	for _, issue := range public.Issues {
		add(issue, "")
	}

	return order, items
}

func roadmapReviewValues(entry roadmapReviewItem) []roadmapReviewValue {
	item := entry.item

	var timeline RoadmapTimeline
	if item.Timeline != nil {
		timeline = *item.Timeline
	}

	var progress string
	if item.Progress != nil {
		progress = strconv.Itoa(*item.Progress)
	}

	var issues string
	if item.Issues != nil {
		issues = strconv.Itoa(len(item.Issues))
	}

	return []roadmapReviewValue{
		{RoadmapReviewTitle, item.Title},
		{RoadmapReviewDescription, item.Description},
		{RoadmapReviewStatus, string(item.Status)},
		{RoadmapReviewStartOn, timeline.StartOn},
		{RoadmapReviewEndOn, timeline.EndOn},
		{RoadmapReviewProgress, progress},
		{RoadmapReviewIssues, issues},
		{RoadmapReviewPlacement, entry.placement},
	}
}

func roadmapFieldChanges(before, after []roadmapReviewValue) []RoadmapFieldChange {
	changes := make([]RoadmapFieldChange, 0, len(after))

	for i, next := range after {
		previous := before[i].value

		switch {
		case previous == next.value:
			continue
		case previous == "":
			changes = append(changes, RoadmapFieldChange{Field: next.field, Kind: RoadmapFieldExposed, After: next.value})
		case next.value == "":
			changes = append(changes, RoadmapFieldChange{Field: next.field, Kind: RoadmapFieldConcealed, Before: previous})
		default:
			changes = append(changes, RoadmapFieldChange{
				Field:  next.field,
				Kind:   RoadmapFieldChanged,
				Before: previous,
				After:  next.value,
			})
		}
	}

	return changes
}
