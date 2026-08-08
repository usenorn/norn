package scm

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *connections) reads(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *connections) manages(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *connections) Links(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.CodeLink, map[uuid.UUID]entity.CodeReviewers, error) {
	if _, _, err := s.reads(ctx, workspaceID, issueID); err != nil {
		return nil, nil, err
	}

	links, err := s.links.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]uuid.UUID, len(links))
	for i, link := range links {
		ids[i] = link.ID
	}

	reviewers, err := s.links.ListReviewers(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	return links, reviewers, nil
}

func (s *connections) Link(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.LinkIssueCodeInput,
) (entity.CodeLink, error) {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return entity.CodeLink{}, err
	}

	parsed, err := parseCodeURL(input.URL)
	if err != nil {
		return entity.CodeLink{}, err
	}

	stored, err := s.matchRepository(ctx, workspaceID, parsed)
	if err != nil {
		return entity.CodeLink{}, err
	}

	if err := s.reaches(ctx, stored, issue, decision); err != nil {
		return entity.CodeLink{}, err
	}

	link, err := s.links.Upsert(ctx, entity.CodeLink{
		WorkspaceID:    workspaceID,
		IssueID:        issueID,
		RepositoryID:   stored.ID,
		Provider:       stored.Provider,
		RepositoryName: stored.FullName,
		Kind:           parsed.kind,
		ExternalID:     parsed.externalID,
		Number:         parsed.number,
		URL:            input.URL,
		State:          entity.CodeChangeOpen,
		DetectedIn:     "a person linked it",
	})
	if err != nil {
		return entity.CodeLink{}, err
	}

	s.recordLinked(ctx, workspaceID, issue, link, decision)

	return link, nil
}

func (s *connections) Unlink(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) error {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}

	link, err := s.links.Delete(ctx, workspaceID, issueID, linkID)
	if err != nil {
		return err
	}

	s.recordUnlinked(ctx, workspaceID, issue, link, decision)

	return nil
}

func (s *connections) recordLinked(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
) {
	s.recordActivity(ctx, workspaceID, issue, link, decision, entity.ActivityKindCodeLinked)
}

func (s *connections) recordUnlinked(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
) {
	s.recordActivity(ctx, workspaceID, issue, link, decision, entity.ActivityKindCodeUnlinked)
}

func (s *connections) recordActivity(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
	kind entity.ActivityKind,
) {
	if err := s.activity.Record(ctx, entity.Activity{
		WorkspaceID: workspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        kind,
		Field:       string(link.Kind),
		ToValue:     linkLabel(link),
		Version:     issue.Version,
	}); err != nil {
		logWarn(ctx, "recording a code link on the issue feed failed", link.ID, err)
	}
}

func linkLabel(link entity.CodeLink) string {
	if link.Number > 0 {
		return link.RepositoryName + "#" + strconv.Itoa(link.Number)
	}

	return link.RepositoryName + " " + link.ExternalID
}

type codeAddress struct {
	host       string
	repository string
	kind       entity.CodeLinkKind
	externalID string
	number     int
}

func parseCodeURL(raw string) (codeAddress, error) {
	invalid := entity.ValidationError{
		Fields: []entity.FieldError{{Field: "url", Code: entity.ValidationCodeUnsupportedValue}},
	}

	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return codeAddress{}, invalid
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return codeAddress{}, invalid
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	for i, segment := range segments {
		var (
			kind entity.CodeLinkKind
			skip int
		)

		switch segment {
		case "pull", "pulls", "merge_requests":
			kind, skip = entity.CodeLinkChange, 1
		case "commit", "commits":
			kind, skip = entity.CodeLinkCommit, 1
		case "tree":
			kind, skip = entity.CodeLinkBranch, 1
		default:
			continue
		}

		if i+skip >= len(segments) {
			return codeAddress{}, invalid
		}

		value := segments[i+skip]
		repository := strings.Join(trimGitLabMarker(segments[:i]), "/")

		if repository == "" || value == "" {
			return codeAddress{}, invalid
		}

		address := codeAddress{
			host:       parsed.Host,
			repository: repository,
			kind:       kind,
			externalID: value,
		}

		if number, err := strconv.Atoi(value); err == nil {
			address.number = number
		}

		return address, nil
	}

	return codeAddress{}, invalid
}

func trimGitLabMarker(segments []string) []string {
	kept := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "-" {
			continue
		}

		kept = append(kept, segment)
	}

	return kept
}

func (s *connections) matchRepository(
	ctx context.Context,
	workspaceID uuid.UUID,
	address codeAddress,
) (entity.SCMRepository, error) {
	unknown := entity.ValidationError{
		Fields: []entity.FieldError{{Field: "url", Code: entity.ValidationCodeUnsupportedValue}},
	}

	available, err := s.repositories.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return entity.SCMRepository{}, err
	}

	for _, one := range available {
		if !strings.EqualFold(one.FullName, address.repository) {
			continue
		}

		connection, err := s.connections.GetByID(ctx, workspaceID, one.ConnectionID)
		if err != nil {
			return entity.SCMRepository{}, err
		}

		if !sameHost(connection.BaseURL, address.host) {
			continue
		}

		return one, nil
	}

	return entity.SCMRepository{}, unknown
}

func sameHost(baseURL, host string) bool {
	if baseURL == "" {
		return true
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Host, host)
}

func (s *connections) reaches(
	ctx context.Context,
	stored entity.SCMRepository,
	issue entity.Issue,
	decision entity.Decision,
) error {
	if !decision.Scope.Covers(issue.TeamID) {
		return entity.ErrSCMTeamOutsideConnection
	}

	routes, err := s.routes.ListByRepository(ctx, stored.ID)
	if err != nil {
		return err
	}

	if !routes.Reaches(issue.TeamID) {
		return entity.ErrSCMTeamOutsideConnection
	}

	return nil
}

func (s *connections) Mirrors(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueMirror, error) {
	if _, _, err := s.reads(ctx, workspaceID, issueID); err != nil {
		return nil, err
	}

	return s.mirrors.ListByIssue(ctx, workspaceID, issueID)
}

func (s *connections) Mirror(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.MirrorIssueInput,
) (entity.IssueMirror, error) {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	stored, err := s.repositories.GetByID(ctx, workspaceID, input.RepositoryID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	if err := s.reaches(ctx, stored, issue, decision); err != nil {
		return entity.IssueMirror{}, err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, stored.ConnectionID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	number, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(input.Reference), "#"))
	if err != nil || number < 1 {
		return entity.IssueMirror{}, entity.ValidationError{
			Fields: []entity.FieldError{{
				Field: "reference",
				Code:  entity.ValidationCodeUnsupportedValue,
			}},
		}
	}

	token, err := s.credentials.token(ctx, connection)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	found, err := forge.Issue(ctx, connection.Target(stored.FullName, token), number)
	if err != nil {
		s.breakOn(ctx, connection, err)

		return entity.IssueMirror{}, err
	}

	mirror, err := s.mirrors.Create(ctx, entity.IssueMirror{
		WorkspaceID:    workspaceID,
		IssueID:        issueID,
		RepositoryID:   stored.ID,
		Provider:       stored.Provider,
		RepositoryName: stored.FullName,
		ExternalID:     found.ExternalID,
		ExternalNumber: found.Number,
		URL:            found.URL,
		Origin:         entity.MirrorOriginNorn,
		Direction:      entity.MirrorBoth,
	})
	if err != nil {
		return entity.IssueMirror{}, err
	}

	hashes := entity.HashesOf(found.Title, found.Body, found.State)

	if err := s.mirrors.RecordPull(
		ctx,
		mirror.ID,
		hashes,
		found.UpdatedAt,
		issue.Version,
		time.Now().UTC(),
	); err != nil {
		return entity.IssueMirror{}, err
	}

	return s.mirrors.GetByExternalID(
		ctx, workspaceID, stored.Provider, stored.FullName, found.ExternalID,
	)
}

func (s *connections) Unmirror(
	ctx context.Context,
	workspaceID, issueID, mirrorID uuid.UUID,
) error {
	if _, _, err := s.manages(ctx, workspaceID, issueID); err != nil {
		return err
	}

	return s.mirrors.Delete(ctx, workspaceID, issueID, mirrorID)
}
