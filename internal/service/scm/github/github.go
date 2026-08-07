package github

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
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.GitHubEndpoint), "/"),
		pageSize: cfg.PageSize,
	}
}

func (f *Forge) Provider() entity.SCMProvider {
	return entity.SCMProviderGitHub
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

func (f *Forge) call(
	ctx context.Context,
	target entity.SCMTarget,
	method, path string,
	body any,
) (forge.Response, error) {
	address := path
	if !strings.HasPrefix(path, "http") {
		address = f.base(target) + path
	}

	request := forge.Request{
		Trust:    target.Trust,
		Provider: entity.SCMProviderGitHub,
		Method:   method,
		URL:      address,
		Header: http.Header{
			"Accept":               {"application/vnd.github+json"},
			"X-GitHub-Api-Version": {"2022-11-28"},
			"Authorization":        {"Bearer " + target.Token},
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
			Provider:   entity.SCMProviderGitHub,
			Repository: target.Repository,
			Reason:     "the repository is not visible to this token",
		}
	}

	if response.Status < 200 || response.Status > 299 {
		return entity.SCMUnavailableError{
			Provider: entity.SCMProviderGitHub,
			Reason:   fmt.Sprintf("answered %d", response.Status),
		}
	}

	if into == nil {
		return nil
	}

	if err := json.Unmarshal(response.Body, into); err != nil {
		return entity.SCMUnavailableError{
			Provider: entity.SCMProviderGitHub,
			Reason:   fmt.Sprintf("answered with something that is not json: %v", err),
			Cause:    err,
		}
	}

	return nil
}

type repositoryBody struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Permissions   struct {
		Admin bool `json:"admin"`
	} `json:"permissions"`
}

func (f *Forge) Repository(
	ctx context.Context,
	target entity.SCMTarget,
) (entity.SCMRemoteRepository, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/repos/"+target.Repository, nil)
	if err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	var body repositoryBody

	if err := f.decode(response, target, &body); err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	return entity.SCMRemoteRepository{
		ExternalID:    strconv.FormatInt(body.ID, 10),
		FullName:      body.FullName,
		URL:           body.HTMLURL,
		DefaultBranch: body.DefaultBranch,
		Private:       body.Private,
		CanAdmin:      body.Permissions.Admin,
	}, nil
}

func (f *Forge) Identity(ctx context.Context, target entity.SCMTarget) (string, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}

	var body struct {
		Login string `json:"login"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return "", err
	}

	return body.Login, nil
}

func (f *Forge) InstallHook(
	ctx context.Context,
	request service.ForgeHookRequest,
) (string, error) {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": hookEvents,
		"config": map[string]any{
			"url":          request.CallbackURL,
			"content_type": "json",
			"secret":       request.Secret,
			"insecure_ssl": "0",
		},
	}

	response, err := f.call(
		ctx,
		request.Target,
		http.MethodPost,
		"/repos/"+request.Target.Repository+"/hooks",
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

func (f *Forge) RemoveHook(
	ctx context.Context,
	target entity.SCMTarget,
	hookID string,
) error {
	response, err := f.call(
		ctx,
		target,
		http.MethodDelete,
		"/repos/"+target.Repository+"/hooks/"+hookID,
		nil,
	)
	if err != nil {
		return err
	}

	// A hook already gone is the state this was asking for, so disconnecting a repository
	// somebody tidied up by hand must not fail.
	if response.Status == http.StatusNotFound {
		return nil
	}

	return f.decode(response, target, nil)
}

type changeBody struct {
	ID                 int64      `json:"id"`
	Number             int        `json:"number"`
	Title              string     `json:"title"`
	Body               string     `json:"body"`
	HTMLURL            string     `json:"html_url"`
	State              string     `json:"state"`
	Draft              bool       `json:"draft"`
	MergeableState     string     `json:"mergeable_state"`
	MergedAt           *time.Time `json:"merged_at"`
	ClosedAt           *time.Time `json:"closed_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	RequestedReviewers []struct {
		Login string `json:"login"`
	} `json:"requested_reviewers"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// Changes asks for every state, not the default. GitHub lists open pull requests unless told
// otherwise, so a sweep healing a missed delivery would never see the merge it exists to
// catch — the change is closed by then and invisible to the default query.
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
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		query.Set("per_page", strconv.Itoa(f.pageSize))

		path = "/repos/" + target.Repository + "/pulls?" + query.Encode()
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
		if change.UpdatedAt.Before(since) {
			// The listing is newest first, so the first change older than the watermark ends
			// the walk and stops the cursor rather than paging to the beginning of time.
			page.Cursor = ""

			break
		}

		page.Changes = append(page.Changes, service.ForgeChange{
			ExternalID: strconv.FormatInt(change.ID, 10),
			Number:     change.Number,
			Title:      change.Title,
			Body:       change.Body,
			URL:        change.HTMLURL,
			State: changeState(
				change.Draft,
				change.State,
				change.MergedAt,
				change.MergeableState,
			),
			// The listing carries who was asked to review but not what anybody answered. The
			// sweep asks for the set to be read rather than rebuilding it from what it has.
			ReviewsMoved: true,
			Author:       change.User.Login,
			HeadBranch:   change.Head.Ref,
			BaseBranch:   change.Base.Ref,
			UpdatedAt:    change.UpdatedAt,
			MergedAt:     change.MergedAt,
			ClosedAt:     change.ClosedAt,
		})
	}

	return page, nil
}

type issueBody struct {
	ID          int64     `json:"id"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (b issueBody) forgeIssue() service.ForgeIssue {
	labels := make([]string, 0, len(b.Labels))
	for _, label := range b.Labels {
		labels = append(labels, label.Name)
	}

	return service.ForgeIssue{
		ExternalID: strconv.FormatInt(b.ID, 10),
		Number:     b.Number,
		Title:      b.Title,
		Body:       b.Body,
		URL:        b.HTMLURL,
		State:      b.State,
		Author:     b.User.Login,
		Labels:     labels,
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
		query.Set("state", "all")
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		query.Set("per_page", strconv.Itoa(f.pageSize))

		if label != "" {
			query.Set("labels", label)
		}

		if !since.IsZero() {
			query.Set("since", since.UTC().Format(time.RFC3339))
		}

		path = "/repos/" + target.Repository + "/issues?" + query.Encode()
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
		if issue.PullRequest != nil {
			continue
		}

		page.Issues = append(page.Issues, issue.forgeIssue())
	}

	return page, nil
}

// issuePath addresses an issue by the number its repository counts with. The id carried in a
// payload is global and is what a mirror is stored under; it is not an address.
func (f *Forge) issuePath(target entity.SCMTarget, number int) string {
	return "/repos/" + target.Repository + "/issues/" + strconv.Itoa(number)
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
		payload["body"] = *patch.Body
	}

	if patch.Assignee != nil {
		payload["assignees"] = []string{*patch.Assignee}
	}

	if len(patch.Labels) > 0 {
		payload["labels"] = patch.Labels
	}

	if patch.Closed != nil {
		payload["state"] = "open"
		if *patch.Closed {
			payload["state"] = "closed"
		}
	}

	response, err := f.call(
		ctx,
		target,
		http.MethodPatch,
		f.issuePath(target, number),
		payload,
	)
	if err != nil {
		return service.ForgeIssue{}, err
	}

	var body issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssue{}, err
	}

	return body.forgeIssue(), nil
}

type commentBody struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (b commentBody) forgeComment() service.ForgeComment {
	return service.ForgeComment{
		ExternalID: strconv.FormatInt(b.ID, 10),
		Body:       b.Body,
		Author:     b.User.Login,
		URL:        b.HTMLURL,
		CreatedAt:  b.CreatedAt,
		UpdatedAt:  b.UpdatedAt,
	}
}

func (f *Forge) Comments(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	since time.Time,
) ([]service.ForgeComment, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(f.pageSize))

	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}

	path := f.issuePath(target, number) + "/comments?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []commentBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	comments := make([]service.ForgeComment, 0, len(body))
	for _, comment := range body {
		comments = append(comments, comment.forgeComment())
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
		f.issuePath(target, number)+"/comments",
		map[string]any{"body": body},
	)
	if err != nil {
		return service.ForgeComment{}, err
	}

	var posted commentBody

	if err := f.decode(response, target, &posted); err != nil {
		return service.ForgeComment{}, err
	}

	return posted.forgeComment(), nil
}

type changedFileBody struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
}

// ChangedPaths reads the files a pull request touches. Nothing in a webhook payload carries
// them, and routing a change to the team that owns its area cannot be decided without them.
// A rename is reported under both names, because either one may be what a route matches.
func (f *Forge) ChangedPaths(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]string, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(f.pageSize))

	path := "/repos/" + target.Repository + "/pulls/" + strconv.Itoa(number) +
		"/files?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []changedFileBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(body))

	for _, file := range body {
		if file.Filename != "" {
			paths = append(paths, file.Filename)
		}

		if file.PreviousFilename != "" {
			paths = append(paths, file.PreviousFilename)
		}
	}

	return paths, nil
}

// hookEvents is what Norn asks GitHub to send. Adding one here is not enough on its own: a
// hook installed before the addition keeps its old list, which is what installedEvents and
// the sweep's repair exist for.
var hookEvents = []string{
	"push",
	"pull_request",
	"pull_request_review",
	"check_suite",
	"issues",
	"issue_comment",
}

type reviewBody struct {
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	SubmittedAt time.Time `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

// Reviews reads the answers a change currently has, keeping the latest one per person. A
// reviewer who comments and later approves has one verdict, not two, and GitHub returns
// every review ever submitted in order.
func (f *Forge) Reviews(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]service.ForgeReviewer, error) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(f.pageSize))

	path := "/repos/" + target.Repository + "/pulls/" + strconv.Itoa(number) +
		"/reviews?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []reviewBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	at := make(map[string]int, len(body))
	reviewers := make([]service.ForgeReviewer, 0, len(body))

	for _, review := range body {
		verdict, known := reviewVerdict(review.State)
		if !known || review.User.Login == "" {
			continue
		}

		submitted := review.SubmittedAt
		reviewer := service.ForgeReviewer{
			Login:      review.User.Login,
			Verdict:    verdict,
			URL:        review.HTMLURL,
			ReviewedAt: &submitted,
		}

		if index, seen := at[review.User.Login]; seen {
			reviewers[index] = reviewer

			continue
		}

		at[review.User.Login] = len(reviewers)
		reviewers = append(reviewers, reviewer)
	}

	requested, err := f.requestedReviewers(ctx, target, number)
	if err != nil {
		return nil, err
	}

	for _, login := range requested {
		if _, answered := at[login]; answered {
			continue
		}

		reviewers = append(reviewers, service.ForgeReviewer{
			Login:   login,
			Verdict: entity.ReviewRequested,
		})
	}

	return reviewers, nil
}

func (f *Forge) requestedReviewers(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]string, error) {
	path := "/repos/" + target.Repository + "/pulls/" + strconv.Itoa(number) +
		"/requested_reviewers"

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body struct {
		Users []struct {
			Login string `json:"login"`
		} `json:"users"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	logins := make([]string, 0, len(body.Users))
	for _, user := range body.Users {
		if user.Login != "" {
			logins = append(logins, user.Login)
		}
	}

	return logins, nil
}

// RepairHook rewrites the event list only when it is actually short. A pointless write on
// every sweep would spend a rate limit on every repository for nothing.
func (f *Forge) RepairHook(
	ctx context.Context,
	request service.ForgeHookRequest,
	hookID string,
) (bool, error) {
	path := "/repos/" + request.Target.Repository + "/hooks/" + hookID

	response, err := f.call(ctx, request.Target, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}

	var installed struct {
		Events []string `json:"events"`
	}

	if err := f.decode(response, request.Target, &installed); err != nil {
		return false, err
	}

	if covers(installed.Events, hookEvents) {
		return false, nil
	}

	patched, err := f.call(ctx, request.Target, http.MethodPatch, path, map[string]any{
		"active": true,
		"events": hookEvents,
		"config": map[string]any{
			"url":          request.CallbackURL,
			"content_type": "json",
			"secret":       request.Secret,
			"insecure_ssl": "0",
		},
	})
	if err != nil {
		return false, err
	}

	return true, f.decode(patched, request.Target, nil)
}

func covers(installed, wanted []string) bool {
	held := make(map[string]bool, len(installed))
	for _, event := range installed {
		held[event] = true
	}

	if held["*"] {
		return true
	}

	for _, event := range wanted {
		if !held[event] {
			return false
		}
	}

	return true
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
	}
}
