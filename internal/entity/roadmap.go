package entity

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	RoadmapTitleMaxLen       = 120
	RoadmapDescriptionMaxLen = 4000
	RoadmapEntriesMax        = 200
	RoadmapProgressMin       = 0
	RoadmapProgressMax       = 100
	RoadmapPublicIDBytes     = 10
)

const (
	RoadmapFieldTitle           = "title"
	RoadmapFieldDescription     = "description"
	RoadmapFieldPublicTitle     = "publicTitle"
	RoadmapFieldPublicDesc      = "publicDescription"
	RoadmapFieldStatus          = "status"
	RoadmapFieldStartOn         = "timeline.startOn"
	RoadmapFieldEndOn           = "timeline.endOn"
	RoadmapFieldProgressMode    = "progressMode"
	RoadmapFieldProgressPercent = "progressPercent"
	RoadmapFieldKind            = "kind"
)

var (
	ErrRoadmapNotFound       = errors.New("roadmap not found")
	ErrRoadmapStale          = errors.New("roadmap changed since it was read")
	ErrRoadmapEntryNotFound  = errors.New("roadmap entry not found")
	ErrRoadmapEntryDuplicate = errors.New("that project or issue is already on the roadmap")
	ErrRoadmapFull           = errors.New("roadmap already holds the most entries it may")
	ErrRoadmapUnreachable    = errors.New("roadmap entry's project or issue is missing or out of reach")
)

var (
	roadmapPublicIDEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)
	roadmapPublicIDPattern  = regexp.MustCompile(`^[a-z2-7]{16}$`)
)

type RoadmapStaleError struct {
	Version int
}

func (e RoadmapStaleError) Error() string {
	return fmt.Sprintf("%s: now at version %d", ErrRoadmapStale, e.Version)
}

func (e RoadmapStaleError) Unwrap() error {
	return ErrRoadmapStale
}

type RoadmapUnreachableError struct {
	EntryID uuid.UUID
}

func (e RoadmapUnreachableError) Error() string {
	return fmt.Sprintf("%s: entry %s", ErrRoadmapUnreachable, e.EntryID)
}

func (e RoadmapUnreachableError) Unwrap() error {
	return ErrRoadmapUnreachable
}

type RoadmapVisibility struct {
	IssueLists          bool
	ProjectDescriptions bool
	IssueDescriptions   bool
	Status              bool
	Timeline            bool
	Progress            bool
}

func (v RoadmapVisibility) Bound(entry RoadmapVisibility) RoadmapVisibility {
	return RoadmapVisibility{
		IssueLists:          v.IssueLists && entry.IssueLists,
		ProjectDescriptions: v.ProjectDescriptions && entry.ProjectDescriptions,
		IssueDescriptions:   v.IssueDescriptions && entry.IssueDescriptions,
		Status:              v.Status && entry.Status,
		Timeline:            v.Timeline && entry.Timeline,
		Progress:            v.Progress && entry.Progress,
	}
}

type RoadmapEntryKind string

const (
	RoadmapEntryProject RoadmapEntryKind = "project"
	RoadmapEntryIssue   RoadmapEntryKind = "issue"
)

func (k RoadmapEntryKind) Valid() bool {
	return k == RoadmapEntryProject || k == RoadmapEntryIssue
}

type PublicStatus string

const (
	PublicStatusPlanned    PublicStatus = "planned"
	PublicStatusInProgress PublicStatus = "in_progress"
	PublicStatusPaused     PublicStatus = "paused"
	PublicStatusCompleted  PublicStatus = "completed"
	PublicStatusCancelled  PublicStatus = "cancelled"
)

func PublicStatuses() []PublicStatus {
	return []PublicStatus{
		PublicStatusPlanned,
		PublicStatusInProgress,
		PublicStatusPaused,
		PublicStatusCompleted,
		PublicStatusCancelled,
	}
}

func (s PublicStatus) Valid() bool {
	return slices.Contains(PublicStatuses(), s)
}

func PublicStatusOfProject(state ProjectState) PublicStatus {
	switch state {
	case ProjectStateActive:
		return PublicStatusInProgress
	case ProjectStatePaused:
		return PublicStatusPaused
	case ProjectStateCompleted:
		return PublicStatusCompleted
	case ProjectStateCancelled:
		return PublicStatusCancelled
	default:
		return PublicStatusPlanned
	}
}

func PublicStatusOfIssue(category StateCategory) PublicStatus {
	switch category {
	case StateCategoryActive:
		return PublicStatusInProgress
	case StateCategoryComplete:
		return PublicStatusCompleted
	case StateCategoryAbandoned:
		return PublicStatusCancelled
	default:
		return PublicStatusPlanned
	}
}

type RoadmapProgressMode string

const (
	RoadmapProgressHidden     RoadmapProgressMode = "hidden"
	RoadmapProgressApproved   RoadmapProgressMode = "approved"
	RoadmapProgressCalculated RoadmapProgressMode = "calculated"
)

func (m RoadmapProgressMode) Valid() bool {
	return m == RoadmapProgressHidden || m == RoadmapProgressApproved || m == RoadmapProgressCalculated
}

type RoadmapTimeline struct {
	StartOn string
	EndOn   string
}

func (t RoadmapTimeline) Empty() bool {
	return t.StartOn == "" && t.EndOn == ""
}

type Roadmap struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Title       string
	Description string
	Visibility  RoadmapVisibility
	Version     int
	Publication *RoadmapPublication
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r Roadmap) Published() bool {
	return r.Publication != nil
}

type RoadmapPublication struct {
	Title       string
	Description string
	Visibility  RoadmapVisibility
	Version     int
	PublishedAt time.Time
	PublishedBy uuid.UUID
}

type RoadmapEntry struct {
	ID                uuid.UUID
	RoadmapID         uuid.UUID
	WorkspaceID       uuid.UUID
	PublicID          string
	Kind              RoadmapEntryKind
	SourceProjectID   uuid.UUID
	SourceIssueID     uuid.UUID
	ParentEntryID     uuid.UUID
	PublicTitle       string
	PublicDescription string
	Status            PublicStatus
	Timeline          RoadmapTimeline
	ProgressMode      RoadmapProgressMode
	ProgressPercent   int
	Visibility        RoadmapVisibility
	Position          int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type RoadmapApproval struct {
	TeamID      uuid.UUID
	TeamPrivate bool
	ProjectID   uuid.UUID
}

type PublishedRoadmapEntry struct {
	RoadmapEntry
	ParentPublicID string
	Approval       RoadmapApproval
	Source         RoadmapSource
}

type RoadmapSource struct {
	Present     bool
	WorkspaceID uuid.UUID
	Active      bool
	TeamID      uuid.UUID
	TeamPrivate bool
	ProjectID   uuid.UUID
}

func (e PublishedRoadmapEntry) Intact() bool {
	source := e.Source

	if !source.Present || source.WorkspaceID != e.WorkspaceID {
		return false
	}

	if e.Kind == RoadmapEntryProject {
		return true
	}

	switch {
	case !source.Active:
		return false
	case source.TeamID != e.Approval.TeamID:
		return false
	case source.TeamPrivate && !e.Approval.TeamPrivate:
		return false
	default:
		return source.ProjectID == e.Approval.ProjectID
	}
}

type PublishedRoadmap struct {
	WorkspaceID uuid.UUID
	Publication RoadmapPublication
	Entries     []PublishedRoadmapEntry
}

type PublicRoadmap struct {
	Title       string
	Description string
	PublishedAt time.Time
	Projects    []PublicRoadmapItem
	Issues      []PublicRoadmapItem
}

type PublicRoadmapItem struct {
	PublicID    string
	Kind        RoadmapEntryKind
	Title       string
	Description string
	Status      PublicStatus
	Timeline    *RoadmapTimeline
	Progress    *int
	Issues      []PublicRoadmapItem
}

func (r PublicRoadmap) Item(publicID string) (PublicRoadmapItem, bool) {
	for _, project := range r.Projects {
		if project.PublicID == publicID {
			return project, true
		}

		for _, issue := range project.Issues {
			if issue.PublicID == publicID {
				return issue, true
			}
		}
	}

	for _, issue := range r.Issues {
		if issue.PublicID == publicID {
			return issue, true
		}
	}

	return PublicRoadmapItem{}, false
}

func NewPublicRoadmap(published PublishedRoadmap) PublicRoadmap {
	entries := slices.DeleteFunc(slices.Clone(published.Entries), func(entry PublishedRoadmapEntry) bool {
		return !entry.Intact()
	})

	slices.SortStableFunc(entries, func(a, b PublishedRoadmapEntry) int {
		if a.Position != b.Position {
			return a.Position - b.Position
		}

		return strings.Compare(a.PublicID, b.PublicID)
	})

	bound := published.Publication.Visibility
	projects := make(map[string]PublishedRoadmapEntry)

	for _, entry := range entries {
		if entry.Kind == RoadmapEntryProject {
			projects[entry.PublicID] = entry
		}
	}

	grouped := make(map[string][]PublishedRoadmapEntry)
	public := PublicRoadmap{
		Title:       published.Publication.Title,
		Description: published.Publication.Description,
		PublishedAt: published.Publication.PublishedAt,
		Projects:    []PublicRoadmapItem{},
		Issues:      []PublicRoadmapItem{},
	}

	for _, entry := range entries {
		if entry.Kind != RoadmapEntryIssue {
			continue
		}

		if parent, ok := projects[entry.ParentPublicID]; ok {
			grouped[parent.PublicID] = append(grouped[parent.PublicID], entry)

			if bound.Bound(parent.Visibility).IssueLists {
				continue
			}
		}

		public.Issues = append(public.Issues, publicRoadmapItem(entry, bound.Bound(entry.Visibility)))
	}

	for _, entry := range entries {
		if entry.Kind != RoadmapEntryProject {
			continue
		}

		visible := bound.Bound(entry.Visibility)
		item := publicRoadmapItem(entry, visible)

		if visible.Progress {
			item.Progress = roadmapProgress(entry, grouped[entry.PublicID])
		}

		if visible.IssueLists {
			item.Issues = make([]PublicRoadmapItem, 0, len(grouped[entry.PublicID]))

			for _, issue := range grouped[entry.PublicID] {
				item.Issues = append(item.Issues, publicRoadmapItem(issue, bound.Bound(issue.Visibility)))
			}
		}

		public.Projects = append(public.Projects, item)
	}

	return public
}

func publicRoadmapItem(entry PublishedRoadmapEntry, visible RoadmapVisibility) PublicRoadmapItem {
	item := PublicRoadmapItem{
		PublicID: entry.PublicID,
		Kind:     entry.Kind,
		Title:    entry.PublicTitle,
	}

	describes := visible.IssueDescriptions
	if entry.Kind == RoadmapEntryProject {
		describes = visible.ProjectDescriptions
	}

	if describes {
		item.Description = entry.PublicDescription
	}

	if visible.Status {
		item.Status = entry.Status
	}

	if visible.Timeline && !entry.Timeline.Empty() {
		timeline := entry.Timeline
		item.Timeline = &timeline
	}

	return item
}

func roadmapProgress(project PublishedRoadmapEntry, issues []PublishedRoadmapEntry) *int {
	switch project.ProgressMode {
	case RoadmapProgressApproved:
		percent := project.ProgressPercent

		return &percent
	case RoadmapProgressCalculated:
		statuses := make([]PublicStatus, 0, len(issues))
		for _, issue := range issues {
			statuses = append(statuses, issue.Status)
		}

		percent, ok := CalculatedRoadmapProgress(statuses)
		if !ok {
			return nil
		}

		return &percent
	default:
		return nil
	}
}

func CalculatedRoadmapProgress(statuses []PublicStatus) (int, bool) {
	counted, completed := 0, 0

	for _, status := range statuses {
		switch status {
		case PublicStatusCancelled:
			continue
		case PublicStatusCompleted:
			completed++
		}

		counted++
	}

	if counted == 0 {
		return 0, false
	}

	return completed * RoadmapProgressMax / counted, true
}

func NewRoadmapPublicID() (string, error) {
	raw := make([]byte, RoadmapPublicIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate roadmap public id: %w", err)
	}

	return roadmapPublicIDEncoding.EncodeToString(raw), nil
}

func ValidRoadmapPublicID(publicID string) bool {
	return roadmapPublicIDPattern.MatchString(publicID)
}

func RoadmapSlugFromHost(host, domain string) (string, bool) {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" {
		return "", false
	}

	host = strings.ToLower(strings.TrimSpace(host))
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}

	host = strings.TrimSuffix(host, ".")

	slug, found := strings.CutSuffix(host, "."+domain)
	if !found || strings.Contains(slug, ".") {
		return "", false
	}

	if ValidateWorkspaceSlug("", slug).Code != "" {
		return "", false
	}

	return slug, true
}

func ValidateRoadmapTitle(field, title string) FieldError {
	switch length := utf8.RuneCountInString(strings.TrimSpace(title)); {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > RoadmapTitleMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateRoadmapDraftTitle(field, title string) FieldError {
	if utf8.RuneCountInString(strings.TrimSpace(title)) > RoadmapTitleMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateRoadmapDescription(field, description string) FieldError {
	if utf8.RuneCountInString(description) > RoadmapDescriptionMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	return FieldError{}
}

func ValidateRoadmapTimeline(timeline RoadmapTimeline) []FieldError {
	start := ValidateTargetDate(RoadmapFieldStartOn, timeline.StartOn)
	end := ValidateTargetDate(RoadmapFieldEndOn, timeline.EndOn)

	if start.Code != "" || end.Code != "" {
		return []FieldError{start, end}
	}

	if timeline.StartOn != "" && timeline.EndOn == "" {
		return []FieldError{{Field: RoadmapFieldEndOn, Code: ValidationCodeRequired}}
	}

	if timeline.StartOn != "" && timeline.EndOn < timeline.StartOn {
		return []FieldError{{Field: RoadmapFieldEndOn, Code: ValidationCodeOutOfRange}}
	}

	return nil
}

func ValidateRoadmapEntry(entry RoadmapEntry) error {
	fields := []FieldError{
		ValidateRoadmapTitle(RoadmapFieldPublicTitle, entry.PublicTitle),
		ValidateRoadmapDescription(RoadmapFieldPublicDesc, entry.PublicDescription),
	}

	if !entry.Kind.Valid() {
		fields = append(fields, FieldError{Field: RoadmapFieldKind, Code: ValidationCodeUnsupportedValue})
	}

	if !entry.Status.Valid() {
		fields = append(fields, FieldError{Field: RoadmapFieldStatus, Code: ValidationCodeUnsupportedValue})
	}

	if !entry.ProgressMode.Valid() {
		fields = append(fields, FieldError{Field: RoadmapFieldProgressMode, Code: ValidationCodeUnsupportedValue})
	}

	if entry.ProgressPercent < RoadmapProgressMin || entry.ProgressPercent > RoadmapProgressMax {
		fields = append(fields, FieldError{Field: RoadmapFieldProgressPercent, Code: ValidationCodeOutOfRange})
	}

	fields = append(fields, ValidateRoadmapTimeline(entry.Timeline)...)

	return NewValidationError(fields...)
}
