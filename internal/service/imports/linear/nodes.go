package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type connection[T any] struct {
	Nodes    []T      `json:"nodes"`
	PageInfo pageInfo `json:"pageInfo"`
}

type reference struct {
	ID string `json:"id"`
}

func (r *reference) key() string {
	if r == nil {
		return ""
	}

	return r.ID
}

type person struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

func (p *person) imported() service.ImportUser {
	if p == nil || strings.TrimSpace(p.ID) == "" {
		return service.ImportUser{}
	}

	return service.ImportUser{Key: p.ID, Name: named(p.Name, p.DisplayName), Email: p.Email}
}

type teamNode struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type workflowStateNode struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
	Team      *reference `json:"team"`
}

type labelNode struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Color     string     `json:"color"`
	IsGroup   bool       `json:"isGroup"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
	Team      *reference `json:"team"`
	Parent    *reference `json:"parent"`
}

type projectNode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	SlugID      string  `json:"slugId"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	Lead        *person `json:"lead"`
}

type cycleNode struct {
	ID          string     `json:"id"`
	Number      int        `json:"number"`
	Name        string     `json:"name"`
	StartsAt    string     `json:"startsAt"`
	EndsAt      string     `json:"endsAt"`
	CompletedAt string     `json:"completedAt"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
	Team        *reference `json:"team"`
}

type issueNode struct {
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Priority    int                        `json:"priority"`
	Estimate    float64                    `json:"estimate"`
	DueDate     string                     `json:"dueDate"`
	CreatedAt   string                     `json:"createdAt"`
	UpdatedAt   string                     `json:"updatedAt"`
	Team        *reference                 `json:"team"`
	State       *reference                 `json:"state"`
	Project     *reference                 `json:"project"`
	Cycle       *reference                 `json:"cycle"`
	Parent      *reference                 `json:"parent"`
	Assignee    *person                    `json:"assignee"`
	Creator     *person                    `json:"creator"`
	Labels      connection[reference]      `json:"labels"`
	Relations   connection[relationNode]   `json:"relations"`
	Inverse     connection[relationNode]   `json:"inverseRelations"`
	Attachments connection[attachmentNode] `json:"attachments"`
	Comments    connection[commentNode]    `json:"comments"`
}

type relationNode struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	CreatedAt    string     `json:"createdAt"`
	UpdatedAt    string     `json:"updatedAt"`
	Issue        *reference `json:"issue"`
	RelatedIssue *reference `json:"relatedIssue"`
}

type attachmentNode struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type commentNode struct {
	ID        string     `json:"id"`
	Body      string     `json:"body"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
	Issue     *reference `json:"issue"`
	Parent    *reference `json:"parent"`
	User      *person    `json:"user"`
}

type teamsReply struct {
	Teams connection[teamNode] `json:"teams"`
}

type workflowStatesReply struct {
	WorkflowStates connection[workflowStateNode] `json:"workflowStates"`
}

type labelsReply struct {
	IssueLabels connection[labelNode] `json:"issueLabels"`
}

type projectsReply struct {
	Projects connection[projectNode] `json:"projects"`
}

type cyclesReply struct {
	Cycles connection[cycleNode] `json:"cycles"`
}

type issuesReply struct {
	Issues connection[issueNode] `json:"issues"`
}

type commentsReply struct {
	Comments connection[commentNode] `json:"comments"`
}

func decoded[T any](data json.RawMessage, resource entity.ImportResource) (T, error) {
	var reply T

	if err := json.Unmarshal(data, &reply); err != nil {
		return reply, entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   "the source answered in a shape this import does not recognise: " + err.Error(),
			Cause:    err,
		}
	}

	return reply, nil
}

func recordOf(externalID, parentExternalID, created, updated string, payload any) (entity.ImportRecord, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return entity.ImportRecord{}, fmt.Errorf("encode %T for %q: %w", payload, externalID, err)
	}

	return entity.ImportRecord{
		ExternalID:       externalID,
		ParentExternalID: parentExternalID,
		Payload:          encoded,
		SourceCreatedAt:  at(created),
		SourceUpdatedAt:  at(updated),
	}, nil
}

func pageOf(records []entity.ImportRecord, info pageInfo) service.ImportFetchPage {
	return service.ImportFetchPage{
		Records:    records,
		NextCursor: info.EndCursor,
		Done:       !info.HasNextPage,
	}
}

func at(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil
	}

	moment := parsed.UTC()

	return &moment
}

func named(name, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}

	return fallback
}

// truncated says whether a nested connection had more than the fixed page this adapter asks
// for. Linear nests labels, relations, attachments and comments inside the issue that holds
// them, and a nested connection cannot be resumed from the run's cursor: the cursor addresses
// the issue walk, so paging inside one issue would need a second, nested position the port
// has nowhere to put. Rather than lose the remainder in silence, the walk says so.
func truncated[T any](held connection[T]) bool {
	return held.PageInfo.HasNextPage
}

func warnTruncated[T any](ctx context.Context, what, issue string, held connection[T]) {
	if !truncated(held) {
		return
	}

	logging.From(ctx).WarnContext(ctx, "a linear issue held more than one page of nested records",
		"issue", issue,
		"nested", what,
		"carried", len(held.Nodes),
	)
}
