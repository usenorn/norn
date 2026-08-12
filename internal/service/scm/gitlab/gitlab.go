package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/service"
)

type Forge struct {
	client   *forge.Client
	endpoint string
	pageSize int
}

func New(client *forge.Client, cfg config.SourceControl) *Forge {
	return &Forge{
		client:   client,
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.GitLabEndpoint), "/"),
		pageSize: cfg.PageSize,
	}
}

func (f *Forge) Provider() entity.SCMProvider {
	return entity.SCMProviderGitLab
}

func (f *Forge) Endpoint() string {
	return f.endpoint
}

func (f *Forge) base(target entity.SCMTarget) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/"); trimmed != "" {
		return trimmed
	}

	return f.endpoint
}

func project(target entity.SCMTarget) string {
	return url.PathEscape(target.Repository)
}

func (f *Forge) call(
	ctx context.Context,
	target entity.SCMTarget,
	method, path string,
	body any,
) (forge.Response, error) {
	address := path
	if !strings.HasPrefix(path, "http") {
		address = f.base(target) + "/api/v4" + path
	}

	request := forge.Request{
		Trust:    target.Trust,
		Provider: entity.SCMProviderGitLab,
		Method:   method,
		URL:      address,
		Header: http.Header{
			"Accept":        {"application/json"},
			"PRIVATE-TOKEN": {target.Token},
		},
	}

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return forge.Response{}, fmt.Errorf("encode request: %w", err)
		}

		request.Body = encoded
		request.Header.Set("Content-Type", "application/json")
	}

	return f.client.Do(ctx, request)
}

func (f *Forge) decode(response forge.Response, target entity.SCMTarget, into any) error {
	if response.Status == http.StatusNotFound {
		return entity.SCMRepositoryUnreachableError{
			Provider:   entity.SCMProviderGitLab,
			Repository: target.Repository,
			Reason:     "the project is not visible to this token",
		}
	}

	if response.Status < 200 || response.Status > 299 {
		return entity.SCMUnavailableError{
			Provider: entity.SCMProviderGitLab,
			Reason:   fmt.Sprintf("answered %d", response.Status),
		}
	}

	if into == nil {
		return nil
	}

	if err := json.Unmarshal(response.Body, into); err != nil {
		return entity.SCMUnavailableError{
			Provider: entity.SCMProviderGitLab,
			Reason:   fmt.Sprintf("answered with something that is not json: %v", err),
			Cause:    err,
		}
	}

	return nil
}

func (f *Forge) Repository(
	ctx context.Context,
	target entity.SCMTarget,
) (entity.SCMRemoteRepository, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/projects/"+project(target), nil)
	if err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	var body struct {
		ID                int64  `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
		DefaultBranch     string `json:"default_branch"`
		Visibility        string `json:"visibility"`
		Permissions       struct {
			ProjectAccess struct {
				AccessLevel int `json:"access_level"`
			} `json:"project_access"`
		} `json:"permissions"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	const maintainer = 40

	return entity.SCMRemoteRepository{
		ExternalID:    strconv.FormatInt(body.ID, 10),
		FullName:      body.PathWithNamespace,
		URL:           body.WebURL,
		DefaultBranch: body.DefaultBranch,
		Private:       !strings.EqualFold(body.Visibility, "public"),
		CanAdmin:      body.Permissions.ProjectAccess.AccessLevel >= maintainer,
	}, nil
}

func (f *Forge) Identity(ctx context.Context, target entity.SCMTarget) (string, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}

	var body struct {
		Username string `json:"username"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return "", err
	}

	return body.Username, nil
}

func (f *Forge) InstallHook(
	ctx context.Context,
	request service.ForgeHookRequest,
) (string, error) {
	payload := map[string]any{
		"url":                     request.CallbackURL,
		"token":                   request.Secret,
		"push_events":             true,
		"merge_requests_events":   true,
		"issues_events":           true,
		"note_events":             true,
		"pipeline_events":         true,
		"enable_ssl_verification": true,
	}

	response, err := f.call(
		ctx,
		request.Target,
		http.MethodPost,
		"/projects/"+project(request.Target)+"/hooks",
		payload,
	)
	if err != nil {
		return "", err
	}

	var body struct {
		ID int64 `json:"id"`
	}

	if err := f.decode(response, request.Target, &body); err != nil {
		return "", err
	}

	return strconv.FormatInt(body.ID, 10), nil
}

func (f *Forge) RemoveHook(ctx context.Context, target entity.SCMTarget, hookID string) error {
	response, err := f.call(
		ctx,
		target,
		http.MethodDelete,
		"/projects/"+project(target)+"/hooks/"+hookID,
		nil,
	)
	if err != nil {
		return err
	}

	if response.Status == http.StatusNotFound {
		return nil
	}

	return f.decode(response, target, nil)
}

type changeBody struct {
	ID             int64      `json:"id"`
	IID            int        `json:"iid"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	WebURL         string     `json:"web_url"`
	State          string     `json:"state"`
	Draft          bool       `json:"draft"`
	MergeStatus    string     `json:"merge_status"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
	DetailedMerge  string     `json:"detailed_merge_status"`
	SourceBranch   string     `json:"source_branch"`
	SHA            string     `json:"sha"`
	TargetBranch   string     `json:"target_branch"`
	UpdatedAt      time.Time  `json:"updated_at"`
	MergedAt       *time.Time `json:"merged_at"`
	ClosedAt       *time.Time `json:"closed_at"`
	Author         struct {
		Username string `json:"username"`
	} `json:"author"`
	Reviewers []struct {
		Username string `json:"username"`
	} `json:"reviewers"`
}

func (f *Forge) Changes(
	ctx context.Context,
	target entity.SCMTarget,
	since time.Time,
	cursor string,
) (service.ForgeChangePage, error) {
	path := cursor
	if path == "" {
		query := url.Values{}
		query.Set("state", "all")
		query.Set("order_by", "updated_at")
		query.Set("sort", "desc")
		query.Set("per_page", strconv.Itoa(f.pageSize))

		if !since.IsZero() {
			query.Set("updated_after", since.UTC().Format(time.RFC3339))
		}

		path = "/projects/" + project(target) + "/merge_requests?" + query.Encode()
	}

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return service.ForgeChangePage{}, err
	}

	var body []changeBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeChangePage{}, err
	}

	page := service.ForgeChangePage{
		Changes: make([]service.ForgeChange, 0, len(body)),
		Cursor:  response.Link("next"),
	}

	for _, change := range body {
		page.Changes = append(page.Changes, service.ForgeChange{
			ExternalID: strconv.FormatInt(change.ID, 10),
			Number:     change.IID,
			Title:      change.Title,
			Body:       change.Description,
			URL:        change.WebURL,
			State: changeState(
				change.Draft,
				change.State,
				change.MergeStatus,
				change.DetailedMerge,
			),
			MergeCommitSHA: change.MergeCommitSHA,
			ReviewsMoved:   true,
			Author:         change.Author.Username,
			HeadBranch:     change.SourceBranch,
			HeadSHA:        change.SHA,
			BaseBranch:     change.TargetBranch,
			UpdatedAt:      change.UpdatedAt,
			MergedAt:       change.MergedAt,
			ClosedAt:       change.ClosedAt,
		})
	}

	return page, nil
}

type issueBody struct {
	ID          int64     `json:"id"`
	IID         int       `json:"iid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	WebURL      string    `json:"web_url"`
	State       string    `json:"state"`
	Labels      []string  `json:"labels"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Author      struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (b issueBody) forgeIssue() service.ForgeIssue {
	return service.ForgeIssue{
		ExternalID: strconv.FormatInt(b.ID, 10),
		Number:     b.IID,
		Title:      b.Title,
		Body:       b.Description,
		URL:        b.WebURL,
		State:      b.State,
		Author:     b.Author.Username,
		Labels:     b.Labels,
		CreatedAt:  b.CreatedAt,
		UpdatedAt:  b.UpdatedAt,
	}
}

func (f *Forge) Issues(
	ctx context.Context,
	target entity.SCMTarget,
	label string,
	since time.Time,
	cursor string,
) (service.ForgeIssuePage, error) {
	path := cursor
	if path == "" {
		query := url.Values{}
		query.Set("scope", "all")
		query.Set("order_by", "updated_at")
		query.Set("sort", "desc")
		query.Set("per_page", strconv.Itoa(f.pageSize))

		if label != "" {
			query.Set("labels", label)
		}

		if !since.IsZero() {
			query.Set("updated_after", since.UTC().Format(time.RFC3339))
		}

		path = "/projects/" + project(target) + "/issues?" + query.Encode()
	}

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return service.ForgeIssuePage{}, err
	}

	var body []issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssuePage{}, err
	}

	page := service.ForgeIssuePage{
		Issues: make([]service.ForgeIssue, 0, len(body)),
		Cursor: response.Link("next"),
	}

	for _, issue := range body {
		page.Issues = append(page.Issues, issue.forgeIssue())
	}

	return page, nil
}

func (f *Forge) PostChangeComment(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	body string,
) error {
	_, err := f.call(
		ctx,
		target,
		http.MethodPost,
		"/projects/"+project(target)+"/merge_requests/"+strconv.Itoa(number)+"/notes",
		map[string]any{"body": body},
	)

	return err
}

func (f *Forge) issuePath(target entity.SCMTarget, number int) string {
	return "/projects/" + project(target) + "/issues/" + strconv.Itoa(number)
}

func (f *Forge) Issue(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) (service.ForgeIssue, error) {
	response, err := f.call(ctx, target, http.MethodGet, f.issuePath(target, number), nil)
	if err != nil {
		return service.ForgeIssue{}, err
	}

	var body issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssue{}, err
	}

	return body.forgeIssue(), nil
}

func (f *Forge) AmendIssue(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	patch service.ForgeIssuePatch,
) (service.ForgeIssue, error) {
	payload := map[string]any{}

	if patch.Title != nil {
		payload["title"] = *patch.Title
	}

	if patch.Body != nil {
		payload["description"] = *patch.Body
	}

	if patch.Assignee != nil {
		if id, found := f.userID(ctx, target, *patch.Assignee); found {
			payload["assignee_ids"] = []int64{id}
		}
	}

	if len(patch.Labels) > 0 {
		payload["labels"] = strings.Join(patch.Labels, ",")
	}

	if patch.Closed != nil {
		payload["state_event"] = "reopen"
		if *patch.Closed {
			payload["state_event"] = "close"
		}
	}

	response, err := f.call(ctx, target, http.MethodPut, f.issuePath(target, number), payload)
	if err != nil {
		return service.ForgeIssue{}, err
	}

	var body issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssue{}, err
	}

	return body.forgeIssue(), nil
}

type noteBody struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	System    bool      `json:"system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (f *Forge) Comments(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	since time.Time,
) ([]service.ForgeComment, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(f.pageSize))
	query.Set("sort", "asc")

	path := f.issuePath(target, number) + "/notes?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []noteBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	comments := make([]service.ForgeComment, 0, len(body))

	for _, note := range body {
		if note.System || (!since.IsZero() && note.UpdatedAt.Before(since)) {
			continue
		}

		comments = append(comments, service.ForgeComment{
			ExternalID: strconv.FormatInt(note.ID, 10),
			Body:       note.Body,
			Author:     note.Author.Username,
			CreatedAt:  note.CreatedAt,
			UpdatedAt:  note.UpdatedAt,
		})
	}

	return comments, nil
}

func (f *Forge) PostComment(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	body string,
) (service.ForgeComment, error) {
	response, err := f.call(
		ctx,
		target,
		http.MethodPost,
		f.issuePath(target, number)+"/notes",
		map[string]any{"body": body},
	)
	if err != nil {
		return service.ForgeComment{}, err
	}

	var posted noteBody

	if err := f.decode(response, target, &posted); err != nil {
		return service.ForgeComment{}, err
	}

	return service.ForgeComment{
		ExternalID: strconv.FormatInt(posted.ID, 10),
		Body:       posted.Body,
		Author:     posted.Author.Username,
		CreatedAt:  posted.CreatedAt,
		UpdatedAt:  posted.UpdatedAt,
	}, nil
}

type changesBody struct {
	Changes []struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
	} `json:"changes"`
}

func (f *Forge) ChangedPaths(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]string, error) {
	path := "/projects/" + project(target) + "/merge_requests/" + strconv.Itoa(number) + "/changes"

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body changesBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(body.Changes))

	for _, change := range body.Changes {
		if change.NewPath != "" {
			paths = append(paths, change.NewPath)
		}

		if change.OldPath != "" && change.OldPath != change.NewPath {
			paths = append(paths, change.OldPath)
		}
	}

	return paths, nil
}

func (f *Forge) Reviews(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]service.ForgeReviewer, error) {
	path := "/projects/" + project(target) + "/merge_requests/" + strconv.Itoa(number)

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var change changeBody

	if err := f.decode(response, target, &change); err != nil {
		return nil, err
	}

	approvals, err := f.call(ctx, target, http.MethodGet, path+"/approvals", nil)
	if err != nil {
		return nil, err
	}

	var approved struct {
		ApprovedBy []struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"approved_by"`
	}

	if err := f.decode(approvals, target, &approved); err != nil {
		return nil, err
	}

	total := len(approved.ApprovedBy) + len(change.Reviewers)
	at := make(map[string]int, total)
	reviewers := make([]service.ForgeReviewer, 0, total)

	for _, one := range approved.ApprovedBy {
		if one.User.Username == "" {
			continue
		}

		at[one.User.Username] = len(reviewers)
		reviewers = append(reviewers, service.ForgeReviewer{
			Login:   one.User.Username,
			Verdict: entity.ReviewApproved,
		})
	}

	for _, one := range change.Reviewers {
		if one.Username == "" {
			continue
		}

		if _, approved := at[one.Username]; approved {
			continue
		}

		at[one.Username] = len(reviewers)
		reviewers = append(reviewers, service.ForgeReviewer{
			Login:   one.Username,
			Verdict: entity.ReviewRequested,
		})
	}

	return reviewers, nil
}

func (f *Forge) RepairHook(
	ctx context.Context,
	request service.ForgeHookRequest,
	hookID string,
) (bool, error) {
	path := "/projects/" + project(request.Target) + "/hooks/" + hookID

	response, err := f.call(ctx, request.Target, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}

	var installed struct {
		PushEvents          bool `json:"push_events"`
		MergeRequestsEvents bool `json:"merge_requests_events"`
		IssuesEvents        bool `json:"issues_events"`
		NoteEvents          bool `json:"note_events"`
		PipelineEvents      bool `json:"pipeline_events"`
	}

	if err := f.decode(response, request.Target, &installed); err != nil {
		return false, err
	}

	if installed.PushEvents && installed.MergeRequestsEvents && installed.IssuesEvents &&
		installed.NoteEvents && installed.PipelineEvents {
		return false, nil
	}

	patched, err := f.call(ctx, request.Target, http.MethodPut, path, map[string]any{
		"url":                     request.CallbackURL,
		"token":                   request.Secret,
		"push_events":             true,
		"merge_requests_events":   true,
		"issues_events":           true,
		"note_events":             true,
		"pipeline_events":         true,
		"enable_ssl_verification": true,
	})
	if err != nil {
		return false, err
	}

	return true, f.decode(patched, request.Target, nil)
}

func (f *Forge) userID(ctx context.Context, target entity.SCMTarget, login string) (int64, bool) {
	query := url.Values{}
	query.Set("username", login)

	response, err := f.call(ctx, target, http.MethodGet, "/users?"+query.Encode(), nil)
	if err != nil {
		return 0, false
	}

	var users []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}

	if err := f.decode(response, target, &users); err != nil {
		return 0, false
	}

	for _, user := range users {
		if strings.EqualFold(user.Username, login) {
			return user.ID, true
		}
	}

	return 0, false
}

func (f *Forge) Capabilities() entity.SCMCapabilitySet {
	return entity.SCMCapabilitySet{
		entity.CapabilityWebhooks,
		entity.CapabilityReviews,
		entity.CapabilityChecks,
		entity.CapabilityChangedPaths,
		entity.CapabilityIssues,
		entity.CapabilityLabels,
		entity.CapabilityAssignees,
		entity.CapabilityReleases,
		entity.CapabilityDeployments,
	}
}

func (f *Forge) Releases(
	ctx context.Context,
	target entity.SCMTarget,
	limit int,
) ([]service.ForgeRelease, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(limit))

	path := "/projects/" + project(target) + "/releases?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []struct {
		TagName     string     `json:"tag_name"`
		Name        string     `json:"name"`
		ReleasedAt  *time.Time `json:"released_at"`
		UpcomingRel bool       `json:"upcoming_release"`
		Commit      struct {
			ID string `json:"id"`
		} `json:"commit"`
		Links struct {
			Self string `json:"self"`
		} `json:"_links"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	releases := make([]service.ForgeRelease, 0, len(body))

	for _, release := range body {
		releases = append(releases, service.ForgeRelease{
			ExternalID:  release.TagName,
			Tag:         release.TagName,
			Name:        release.Name,
			URL:         release.Links.Self,
			CommitSHA:   release.Commit.ID,
			Prerelease:  release.UpcomingRel,
			PublishedAt: release.ReleasedAt,
		})
	}

	return releases, nil
}

func (f *Forge) ReleaseCommits(
	ctx context.Context,
	target entity.SCMTarget,
	from, to string,
) ([]string, error) {
	if from == "" || to == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("from", from)
	query.Set("to", to)

	path := "/projects/" + project(target) + "/repository/compare?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body struct {
		Commits []struct {
			ID string `json:"id"`
		} `json:"commits"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	commits := make([]string, 0, len(body.Commits))
	for _, commit := range body.Commits {
		if commit.ID != "" {
			commits = append(commits, commit.ID)
		}
	}

	return commits, nil
}

func (f *Forge) Deployments(
	ctx context.Context,
	target entity.SCMTarget,
	limit int,
) ([]service.ForgeDeployment, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(limit))
	query.Set("order_by", "created_at")
	query.Set("sort", "desc")

	path := "/projects/" + project(target) + "/deployments?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []struct {
		ID          int64      `json:"id"`
		Status      string     `json:"status"`
		CreatedAt   *time.Time `json:"created_at"`
		Environment struct {
			Name string `json:"name"`
		} `json:"environment"`
		Deployable struct {
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
		} `json:"deployable"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	deployments := make([]service.ForgeDeployment, 0, len(body))

	for _, deployment := range body {
		if deployment.Environment.Name == "" {
			continue
		}

		deployments = append(deployments, service.ForgeDeployment{
			ExternalID:  strconv.FormatInt(deployment.ID, 10),
			Environment: deployment.Environment.Name,
			State:       deploymentState(deployment.Status),
			CommitSHA:   deployment.Deployable.Commit.ID,
			OccurredAt:  deployment.CreatedAt,
		})
	}

	return deployments, nil
}

func deploymentState(status string) entity.DeploymentState {
	switch strings.ToLower(status) {
	case "success":
		return entity.DeploymentSucceeded
	case "failed", "canceled", "cancelled":
		return entity.DeploymentFailed
	case "running":
		return entity.DeploymentRunning
	case "blocked", "skipped":
		return entity.DeploymentInactive
	default:
		return entity.DeploymentPending
	}
}
