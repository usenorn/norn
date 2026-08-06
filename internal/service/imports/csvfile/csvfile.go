package csvfile

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	sourceKind = "csv"

	teamExternalID = "csv:team"
	labelPrefix    = "label:"
	rowPrefix      = "row:"

	defaultTeamKey  = "CSV"
	defaultTeamName = "Imported"

	rowsDefault = 500
	rowsMost    = 2000
)

// Settings is everything a file cannot say about itself. ObjectKey addresses the upload,
// Delimiter and Header override what the probe read off the first line, Columns is the
// answer to the proposal the catalogue made, and the team is the one a CSV has no way of
// naming.
type Settings struct {
	ObjectKey string   `json:"objectKey"`
	Delimiter string   `json:"delimiter"`
	Header    *bool    `json:"header"`
	TeamKey   string   `json:"teamKey"`
	TeamName  string   `json:"teamName"`
	Columns   []Column `json:"columns"`
}

type Column struct {
	Index  int    `json:"index"`
	Target string `json:"target"`
}

type Source struct {
	blobs repository.Blob
}

func New(blobs repository.Blob) *Source { return &Source{blobs: blobs} }

func (s *Source) Kind() string { return sourceKind }

// Resources is three phases and the team is not one that could be dropped. An issue resolves
// its team through the mapping plan, a blank team key resolves to nothing, and a row with no
// team is a row the apply pass skips: a file that named no team at all would import as nothing
// at all. One team stands in for the whole file and the run maps it onto a real one.
func (s *Source) Resources() []entity.ImportResource {
	return []entity.ImportResource{entity.ImportTeam, entity.ImportLabel, entity.ImportIssue}
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
		return teamPage(settings)
	case entity.ImportLabel:
		return s.fetchLabels(ctx, request, settings)
	case entity.ImportIssue:
		return s.fetchIssues(ctx, request, settings)
	default:
		return service.ImportFetchPage{}, entity.ImportSourceRefusedError{
			Resource: request.Resource,
			Reason:   "a file of rows holds nothing that answers to " + string(request.Resource),
		}
	}
}

func (s *Source) settings(
	resource entity.ImportResource,
	held service.ImportSourceConfig,
) (Settings, error) {
	var settings Settings

	if len(held.Settings) > 0 {
		if err := json.Unmarshal(held.Settings, &settings); err != nil {
			return Settings{}, entity.ImportSourceRefusedError{
				Resource: resource,
				Reason:   "this run's file settings cannot be read: " + err.Error(),
				Cause:    err,
			}
		}
	}

	if strings.TrimSpace(settings.ObjectKey) == "" {
		return Settings{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason:   "this run names no uploaded file, so there is nothing to read rows from",
		}
	}

	return settings, nil
}

func (s Settings) team() service.ImportTeamPayload {
	return service.ImportTeamPayload{
		Key:  named(entity.NormalizeTeamKey(s.TeamKey), defaultTeamKey),
		Name: named(strings.TrimSpace(s.TeamName), defaultTeamName),
	}
}

func pageRows(hint int) int {
	switch {
	case hint <= 0:
		return rowsDefault
	case hint > rowsMost:
		return rowsMost
	default:
		return hint
	}
}

func named(value, fallback string) string {
	if value != "" {
		return value
	}

	return fallback
}
