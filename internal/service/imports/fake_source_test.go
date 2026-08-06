package imports_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	sourceTeam   = "team-core"
	sourceHidden = "team-hidden"

	sourceStateTodo  = "state-todo"
	sourceStateDoing = "state-doing"
	sourceStateDone  = "state-done"

	sourceGroup     = "group-area"
	sourceLabelBug  = "label-bug"
	sourceLabelWork = "label-chore"
	sourceLabelOps  = "label-ops"

	sourceProject = "project-atlas"

	sourceIssueHub     = "issue-hub"
	sourceIssueChild   = "issue-child"
	sourceIssueKin     = "issue-kin"
	sourceIssueDrift   = "issue-drift"
	sourceIssueCadence = "issue-cadence"
	sourceIssueLoose   = "issue-loose"

	sourceRelation = "rel-kin"

	sourceComment = "comment-first"
	sourceReply   = "comment-reply"

	sourcePriority = "P2"
	sourceCycle    = "cycle-7"
	sourceOffPalet = "#ff0044"

	hostileTeamless  = "issue-hidden"
	hostileDuplicate = "issue-dupe"
	hostileAnchor    = "issue-anchor"
	hostileVast      = "issue-vast"
	hostileGhost     = "ghost-issue"
	hostileOrphanKin = "rel-orphan"
	hostileOrphanSay = "comment-orphan"
	hostileStranger  = "comment-stranger"
	hostileLabel     = "label-hot"
)

var (
	rae  = service.ImportUser{Key: "u-rae", Name: "Rae Fenn", Email: "rae@northwind.co"}
	ida  = service.ImportUser{Key: "u-ida", Name: "Ida Vance", Email: "ida@northwind.co"}
	otto = service.ImportUser{Key: "u-otto", Name: "Otto Kell", Email: "otto@elsewhere.test"}
	sam  = service.ImportUser{Key: "u-sam", Name: "Sam Pryce", Email: "sam@elsewhere.test"}
)

func sourceCreatedAt() time.Time {
	return time.Date(2024, time.March, 1, 9, 0, 0, 0, time.UTC)
}

func sourceUpdatedAt() time.Time {
	return time.Date(2024, time.March, 2, 10, 30, 0, 0, time.UTC)
}

type staticSource struct {
	kind string
	held map[entity.ImportResource][]entity.ImportRecord
}

func (s *staticSource) Kind() string { return s.kind }

func (s *staticSource) Resources() []entity.ImportResource { return entity.ImportPhases() }

func (s *staticSource) Fetch(
	_ context.Context,
	request service.ImportFetchRequest,
) (service.ImportFetchPage, error) {
	return pageOf(s.held[request.Resource], request), nil
}

func pageOf(
	held []entity.ImportRecord,
	request service.ImportFetchRequest,
) service.ImportFetchPage {
	offset := 0

	if request.Cursor != "" {
		offset, _ = strconv.Atoi(request.Cursor)
	}

	if offset >= len(held) {
		return service.ImportFetchPage{NextCursor: request.Cursor, Done: true}
	}

	size := request.PageHint
	if size <= 0 {
		size = len(held)
	}

	end := min(offset+size, len(held))

	return service.ImportFetchPage{
		Records:    slices.Clone(held[offset:end]),
		NextCursor: strconv.Itoa(end),
	}
}

func fetched(
	t *testing.T,
	externalID, parentExternalID string,
	payload any,
) entity.ImportRecord {
	t.Helper()

	created := sourceCreatedAt()
	updated := sourceUpdatedAt()

	return entity.ImportRecord{
		ExternalID:       externalID,
		ParentExternalID: parentExternalID,
		Payload:          payloadOf(t, payload),
		SourceCreatedAt:  &created,
		SourceUpdatedAt:  &updated,
	}
}

func newStaticSource(t *testing.T) *staticSource {
	t.Helper()

	return &staticSource{
		kind: "fake",
		held: map[entity.ImportResource][]entity.ImportRecord{
			entity.ImportTeam: {
				fetched(t, sourceTeam, "", service.ImportTeamPayload{Key: "CORE", Name: "Core"}),
			},
			entity.ImportWorkflowState: {
				fetched(t, sourceStateTodo, "", service.ImportWorkflowStatePayload{
					Name: "Todo", Category: string(entity.StateCategoryNotStarted), Team: sourceTeam,
				}),
				fetched(t, sourceStateDoing, "", service.ImportWorkflowStatePayload{
					Name: "In flight", Category: "wip", Team: sourceTeam,
				}),
				fetched(t, sourceStateDone, "", service.ImportWorkflowStatePayload{
					Name: "Done", Category: string(entity.StateCategoryComplete), Team: sourceTeam,
				}),
			},
			entity.ImportLabelGroup: {
				fetched(t, sourceGroup, "", service.ImportLabelGroupPayload{Name: "Area"}),
			},
			entity.ImportLabel: {
				fetched(t, sourceLabelBug, "", service.ImportLabelPayload{
					Name: "Bug", Color: sourceOffPalet, Group: sourceGroup, Team: sourceTeam,
				}),
				fetched(t, sourceLabelWork, "", service.ImportLabelPayload{
					Name: "Chore", Group: sourceGroup, Team: sourceTeam,
				}),
				fetched(t, sourceLabelOps, "", service.ImportLabelPayload{Name: "Ops"}),
			},
			entity.ImportProject: {
				fetched(t, sourceProject, "", service.ImportProjectPayload{
					Slug: "atlas", Name: "Atlas", Description: "The rewrite", Lead: rae,
				}),
			},
			entity.ImportIssue: {
				fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
					Title: "Rework the hub", Description: "It sags in the middle",
					Team: sourceTeam, State: sourceStateTodo, Priority: sourcePriority,
					Project: sourceProject, Labels: []string{sourceLabelBug},
					Assignee: rae, Author: rae, Estimate: 3, DueOn: "2024-06-01",
				}),
				fetched(t, sourceIssueChild, sourceIssueHub, service.ImportIssuePayload{
					Title: "Rework the hub header", Team: sourceTeam, State: sourceStateTodo,
					Priority: sourcePriority, Author: ida,
				}),
				fetched(t, sourceIssueKin, "", service.ImportIssuePayload{
					Title: "Retire the old hub", Team: sourceTeam, State: sourceStateDone,
					Priority: sourcePriority, Author: ida,
				}),
				fetched(t, sourceIssueDrift, "", service.ImportIssuePayload{
					Title: "Chase the drift", Team: sourceTeam, State: sourceStateDoing,
					Priority: sourcePriority, Assignee: otto, Author: otto,
				}),
				fetched(t, sourceIssueCadence, "", service.ImportIssuePayload{
					Title: "Set the cadence", Team: sourceTeam, State: sourceStateTodo,
					Priority: sourcePriority, Author: sam, Cycle: sourceCycle,
				}),
				fetched(t, sourceIssueLoose, "", service.ImportIssuePayload{
					Title: "Tidy the loose ends", Team: sourceTeam, State: sourceStateTodo,
					Priority: sourcePriority, Author: sam,
				}),
			},
			entity.ImportIssueRelation: {
				fetched(t, sourceRelation, "", service.ImportIssueRelationPayload{
					Issue: sourceIssueKin, Related: sourceIssueHub,
					Kind: string(entity.IssueRelationViewRelatesTo),
				}),
			},
			entity.ImportComment: {
				fetched(t, sourceComment, "", service.ImportCommentPayload{
					Issue: sourceIssueHub, Body: "The header is the worst of it", Author: ida,
				}),
				fetched(t, sourceReply, "", service.ImportCommentPayload{
					Issue: sourceIssueHub, Body: "Agreed, starting there",
					Author: rae, Parent: sourceComment,
				}),
			},
		},
	}
}

func (s *staticSource) countOf(resource entity.ImportResource) int {
	return len(s.held[resource])
}

func (s *staticSource) fetchedCount() int {
	total := 0

	for resource, held := range s.held {
		if resource.Fetched() {
			total += len(held)
		}
	}

	return total
}

func (s *staticSource) parentedCount() int {
	parented := 0

	for _, record := range s.held[entity.ImportIssue] {
		if record.ParentExternalID != "" {
			parented++
		}
	}

	return parented
}

func sourceOfIssues(records ...entity.ImportRecord) *staticSource {
	return &staticSource{
		kind: "issues",
		held: map[entity.ImportResource][]entity.ImportRecord{entity.ImportIssue: records},
	}
}

type flakySource struct {
	*staticSource

	issueFetches int
}

func newFlakySource(t *testing.T) *flakySource {
	t.Helper()

	return &flakySource{staticSource: newStaticSource(t)}
}

func (s *flakySource) Fetch(
	ctx context.Context,
	request service.ImportFetchRequest,
) (service.ImportFetchPage, error) {
	if request.Resource != entity.ImportIssue {
		return s.staticSource.Fetch(ctx, request)
	}

	s.issueFetches++

	switch s.issueFetches {
	case 2:
		return service.ImportFetchPage{}, entity.ImportRateLimitedError{
			Resource:   entity.ImportIssue,
			RetryAfter: 30 * time.Second,
		}
	case 3:
		return service.ImportFetchPage{}, entity.ImportSourceUnavailableError{
			Resource: entity.ImportIssue,
			Reason:   "connection reset by peer",
		}
	default:
		return s.staticSource.Fetch(ctx, request)
	}
}

type hostileSource struct {
	*staticSource
}

func newHostileSource(t *testing.T) *hostileSource {
	t.Helper()

	stranger := service.ImportUser{
		Key:   "u-stranger",
		Name:  "Wren Halloway",
		Email: "wren@nowhere.test",
	}

	duplicate := service.ImportIssuePayload{
		Title: "Filed twice by the export", Team: sourceTeam, State: sourceStateTodo,
	}

	return &hostileSource{staticSource: &staticSource{
		kind: "hostile",
		held: map[entity.ImportResource][]entity.ImportRecord{
			entity.ImportTeam: {
				fetched(t, sourceTeam, "", service.ImportTeamPayload{Key: "CORE", Name: "Core"}),
				fetched(t, sourceHidden, "", service.ImportTeamPayload{Key: "HID", Name: "Hidden"}),
			},
			entity.ImportWorkflowState: {
				fetched(t, sourceStateTodo, "", service.ImportWorkflowStatePayload{
					Name: "Todo", Category: string(entity.StateCategoryNotStarted), Team: sourceTeam,
				}),
			},
			entity.ImportLabel: {
				fetched(t, hostileLabel, "", service.ImportLabelPayload{
					Name: "Hot", Color: sourceOffPalet,
				}),
			},
			entity.ImportIssue: {
				fetched(t, hostileDuplicate, "", duplicate),
				fetched(t, hostileDuplicate, "", duplicate),
				fetched(t, hostileAnchor, hostileGhost, service.ImportIssuePayload{
					Title: "Filed under an issue the export left behind",
					Team:  sourceTeam, State: sourceStateTodo,
				}),
				fetched(t, hostileTeamless, "", service.ImportIssuePayload{
					Title: "Filed on a team nobody here can see",
					Team:  sourceHidden, State: sourceStateTodo,
				}),
				fetched(t, hostileVast, "", service.ImportIssuePayload{
					Title:       "Pasted the whole log in",
					Description: strings.Repeat("log line\n", 4<<20/9),
					Team:        sourceTeam, State: sourceStateTodo,
				}),
			},
			entity.ImportIssueRelation: {
				fetched(t, hostileOrphanKin, "", service.ImportIssueRelationPayload{
					Issue: hostileAnchor, Related: hostileGhost,
					Kind: string(entity.IssueRelationViewRelatesTo),
				}),
			},
			entity.ImportComment: {
				fetched(t, hostileOrphanSay, "", service.ImportCommentPayload{
					Issue: hostileGhost, Body: "Said on an issue that never came over",
				}),
				fetched(t, hostileStranger, "", service.ImportCommentPayload{
					Issue: hostileAnchor, Body: "Said by somebody who was never here",
					Author: stranger,
				}),
			},
		},
	}}
}

type scriptedSource struct {
	answer func(service.ImportFetchRequest) (service.ImportFetchPage, error)
}

func (s *scriptedSource) Kind() string { return "scripted" }

func (s *scriptedSource) Resources() []entity.ImportResource {
	return []entity.ImportResource{entity.ImportIssue}
}

func (s *scriptedSource) Fetch(
	_ context.Context,
	request service.ImportFetchRequest,
) (service.ImportFetchPage, error) {
	return s.answer(request)
}
