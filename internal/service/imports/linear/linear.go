package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/lineargraph"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	sourceKind = "linear"
	pageMost   = 250
)

// Settings is what choosing which teams to import comes down to. Every query this source
// makes is narrowed by it, so a run that has chosen nothing has not chosen everything.
type Settings struct {
	TeamIDs []string `json:"teamIds"`
}

type Source struct {
	client *lineargraph.Client
	blobs  repository.Blob
	files  *http.Client
	cfg    config.Linear
	limits config.Imports
}

func New(
	client *lineargraph.Client,
	blobs repository.Blob,
	cfg config.Linear,
	limits config.Imports,
) *Source {
	return &Source{
		client: client,
		blobs:  blobs,
		files:  &http.Client{Timeout: cfg.RequestTimeout},
		cfg:    cfg,
		limits: limits,
	}
}

func (s *Source) Kind() string { return sourceKind }

// Resources names every phase this source is asked for. Two of the twelve are derived from a
// row that has already arrived rather than fetched, and asking for them would stage a page of
// nothing over the records the staging pass had just derived itself.
func (s *Source) Resources() []entity.ImportResource {
	phases := entity.ImportPhases()
	fetched := make([]entity.ImportResource, 0, len(phases))

	for _, resource := range phases {
		if resource.Fetched() {
			fetched = append(fetched, resource)
		}
	}

	return fetched
}

func (s *Source) Fetch(
	ctx context.Context,
	request service.ImportFetchRequest,
) (service.ImportFetchPage, error) {
	settings, err := s.settings(request.Resource, request.Config)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	switch request.Resource {
	case entity.ImportTeam:
		return s.fetchTeams(ctx, request, settings)
	case entity.ImportWorkflowState:
		return s.fetchWorkflowStates(ctx, request, settings)
	case entity.ImportLabelGroup:
		return s.fetchLabelGroups(ctx, request, settings)
	case entity.ImportLabel:
		return s.fetchLabels(ctx, request, settings)
	case entity.ImportProject:
		return s.fetchProjects(ctx, request, settings)
	case entity.ImportCycle:
		return s.fetchCycles(ctx, request, settings)
	case entity.ImportIssue:
		return s.fetchIssues(ctx, request, settings)
	case entity.ImportIssueRelation:
		return s.fetchIssueRelations(ctx, request, settings)
	case entity.ImportComment:
		return s.fetchComments(ctx, request, settings)
	case entity.ImportAttachment:
		return s.fetchAttachments(ctx, request, settings)
	default:
		return service.ImportFetchPage{}, entity.ImportSourceRefusedError{
			Resource: request.Resource,
			Reason:   "this source holds nothing that answers to " + string(request.Resource),
		}
	}
}

func (s *Source) settings(
	resource entity.ImportResource,
	held service.ImportSourceConfig,
) (Settings, error) {
	if strings.TrimSpace(held.Secret) == "" {
		return Settings{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason:   "this run has no Linear key, so the source has nothing to answer to",
		}
	}

	var settings Settings

	if len(held.Settings) > 0 {
		if err := json.Unmarshal(held.Settings, &settings); err != nil {
			return Settings{}, entity.ImportSourceRefusedError{
				Resource: resource,
				Reason:   "this run's Linear settings cannot be read: " + err.Error(),
				Cause:    err,
			}
		}
	}

	settings.TeamIDs = chosenTeams(settings.TeamIDs)

	if len(settings.TeamIDs) == 0 {
		return Settings{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason: "this run names no Linear team. Every query is narrowed to the teams the " +
				"run chose, and a run that chose none would either read the whole workspace or " +
				"nothing at all — neither of which is what an empty selection asked for",
		}
	}

	return settings, nil
}

func chosenTeams(chosen []string) []string {
	teams := make([]string, 0, len(chosen))
	held := make(map[string]bool, len(chosen))

	for _, team := range chosen {
		trimmed := strings.TrimSpace(team)

		if trimmed == "" || held[trimmed] {
			continue
		}

		held[trimmed] = true

		teams = append(teams, trimmed)
	}

	return teams
}

func (s *Source) first(hint int) int {
	if hint <= 0 {
		hint = s.cfg.PageSize
	}

	switch {
	case hint > pageMost:
		return pageMost
	case hint < 1:
		return 1
	default:
		return hint
	}
}

func query[T any](
	ctx context.Context,
	source *Source,
	request service.ImportFetchRequest,
	settings Settings,
	text string,
) (T, error) {
	var reply T

	variables := map[string]any{
		"teams": settings.TeamIDs,
		"first": source.first(request.PageHint),
	}

	if strings.TrimSpace(request.Cursor) != "" {
		variables["after"] = request.Cursor
	}

	data, err := source.client.Query(ctx, request.Config.Secret, text, variables, request.Resource)
	if err != nil {
		return reply, err
	}

	return decoded[T](data, request.Resource)
}
