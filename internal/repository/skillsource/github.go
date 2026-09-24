package skillsource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository"
)

const (
	defaultRef     = "HEAD"
	blobType       = "blob"
	executableMode = "100755"
	regularMode    = "100644"
	shaMediaType   = "application/vnd.github.sha"
)

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type tree struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type githubSource struct {
	client *toolingclient.Client
}

func New(client *toolingclient.Client) repository.SkillSource {
	return &githubSource{client: client}
}

func translate(err error) error {
	if errors.Is(err, toolingclient.ErrNotFound) {
		return entity.ErrAgentSkillSourceNotFound
	}

	return fmt.Errorf("%w: %w", entity.ErrAgentSkillSourceUnreachable, err)
}

func escapedRepository(repository string) string {
	owner, name, _ := strings.Cut(repository, "/")

	return url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func escapedPath(file string) string {
	segments := strings.Split(file, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}

func (s *githubSource) revision(ctx context.Context, repository, ref string) (string, error) {
	if ref == "" {
		ref = defaultRef
	}

	header := s.client.GitHubHeader()
	header.Set("Accept", shaMediaType)

	body, err := s.client.Get(
		ctx,
		s.client.GitHubURL("/repos/"+escapedRepository(repository)+"/commits/"+url.PathEscape(ref)),
		header,
		false,
	)
	if err != nil {
		return "", translate(err)
	}

	sha := strings.TrimSpace(string(body))
	if len(sha) < 40 || strings.ContainsAny(sha, "{}\" ") {
		return "", fmt.Errorf("%w: the commit lookup did not answer with a sha", entity.ErrAgentSkillSourceUnreachable)
	}

	return sha, nil
}

func (s *githubSource) tree(ctx context.Context, repository, revision string) ([]treeEntry, error) {
	var listed tree

	err := s.client.GetJSON(
		ctx,
		s.client.GitHubURL("/repos/"+escapedRepository(repository)+"/git/trees/"+url.PathEscape(revision)+"?recursive=1"),
		s.client.GitHubHeader(),
		&listed,
	)
	if err != nil {
		return nil, translate(err)
	}

	return listed.Tree, nil
}

func (s *githubSource) raw(ctx context.Context, repository, revision, file string) ([]byte, error) {
	body, err := s.client.Get(
		ctx,
		s.client.GitHubRawURL("/"+escapedRepository(repository)+"/"+url.PathEscape(revision)+"/"+escapedPath(file)),
		http.Header{},
		false,
	)
	if err != nil {
		return nil, translate(err)
	}

	return body, nil
}

func within(directory, file string) bool {
	return directory == "" || file == directory || strings.HasPrefix(file, directory+"/")
}

func (s *githubSource) Discover(
	ctx context.Context,
	location entity.AgentSkillLocation,
) (entity.AgentSkillDiscovery, error) {
	revision, err := s.revision(ctx, location.Repository, location.Ref)
	if err != nil {
		return entity.AgentSkillDiscovery{}, err
	}

	entries, err := s.tree(ctx, location.Repository, revision)
	if err != nil {
		return entity.AgentSkillDiscovery{}, err
	}

	var manifests []string

	for _, entry := range entries {
		if entry.Type != blobType || path.Base(entry.Path) != entity.AgentSkillManifestFile {
			continue
		}

		if within(location.Path, entry.Path) {
			manifests = append(manifests, entry.Path)
		}
	}

	slices.Sort(manifests)

	if len(manifests) > entity.AgentSkillCandidatesMax {
		manifests = manifests[:entity.AgentSkillCandidatesMax]
	}

	discovery := entity.AgentSkillDiscovery{Location: location, Revision: revision}

	for _, manifest := range manifests {
		content, err := s.raw(ctx, location.Repository, revision, manifest)
		if err != nil {
			if errors.Is(err, entity.ErrAgentSkillSourceNotFound) {
				continue
			}

			return entity.AgentSkillDiscovery{}, err
		}

		parsed, err := entity.ParseAgentSkillManifest(content)
		if err != nil {
			continue
		}

		directory := path.Dir(manifest)
		if directory == "." {
			directory = ""
		}

		discovery.Candidates = append(discovery.Candidates, entity.AgentSkillCandidate{
			Name:        parsed.Name,
			Description: parsed.Description,
			Path:        directory,
		})
	}

	if len(discovery.Candidates) == 0 {
		return entity.AgentSkillDiscovery{}, entity.ErrAgentSkillSourceNotFound
	}

	return discovery, nil
}

func (s *githubSource) Fetch(
	ctx context.Context,
	repository, revision, directory string,
) (entity.AgentSkillBundle, error) {
	entries, err := s.tree(ctx, repository, revision)
	if err != nil {
		return entity.AgentSkillBundle{}, err
	}

	var (
		wanted []treeEntry
		total  int64
	)

	for _, entry := range entries {
		if entry.Type != blobType || (entry.Mode != regularMode && entry.Mode != executableMode) {
			continue
		}

		if directory != "" && !strings.HasPrefix(entry.Path, directory+"/") {
			continue
		}

		wanted = append(wanted, entry)
		total += entry.Size

		if len(wanted) > entity.AgentSkillBundleMaxFiles || total > entity.AgentSkillBundleMaxBytes {
			return entity.AgentSkillBundle{}, entity.NewValidationError(
				entity.FieldError{Field: "bundle", Code: entity.ValidationCodeTooLong},
			)
		}
	}

	if len(wanted) == 0 {
		return entity.AgentSkillBundle{}, entity.ErrAgentSkillSourceNotFound
	}

	bundle := entity.AgentSkillBundle{Files: make([]entity.AgentSkillFile, 0, len(wanted))}

	for _, entry := range wanted {
		content, err := s.raw(ctx, repository, revision, entry.Path)
		if err != nil {
			return entity.AgentSkillBundle{}, err
		}

		relative := entry.Path
		if directory != "" {
			relative = strings.TrimPrefix(entry.Path, directory+"/")
		}

		bundle.Files = append(bundle.Files, entity.AgentSkillFile{
			Path:       relative,
			Executable: entry.Mode == executableMode,
			Content:    content,
		})
	}

	return bundle, nil
}
