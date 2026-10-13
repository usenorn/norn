package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	roadmapsvc "github.com/usenorn/norn/internal/service/roadmap"
)

var publicRoadmapKeys = map[string]bool{
	"title":       true,
	"description": true,
	"publishedAt": true,
	"projects":    true,
	"issues":      true,
	"id":          true,
	"kind":        true,
	"status":      true,
	"timeline":    true,
	"startOn":     true,
	"endOn":       true,
	"progress":    true,
}

func unlistedKeys(value any) []string {
	var found []string

	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if !publicRoadmapKeys[key] {
				found = append(found, key)
			}

			found = append(found, unlistedKeys(nested)...)
		}
	case []any:
		for _, nested := range typed {
			found = append(found, unlistedKeys(nested)...)
		}
	}

	return found
}

func publicRoadmapEdge(t *testing.T) (*roadmapsvc.MockPublicRoadmaps, http.Handler) {
	t.Helper()

	ctrl := gomock.NewController(t)
	public := roadmapsvc.NewMockPublicRoadmaps(ctrl)
	routes := middleware.ClientCapture(config.HTTP{})(newEdge(ctrl, edgeServices{publicRoadmaps: public}))

	return public, routes
}

func TestThePublicRoadmapIsAnAllowlistedUncachedResponseForItsHost(t *testing.T) {
	public, routes := publicRoadmapEdge(t)
	progress := 40

	public.EXPECT().Get(gomock.Any(), "acme.roadmap.example.com").Return(entity.PublicRoadmap{
		Title:       "Acme roadmap",
		PublishedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Projects: []entity.PublicRoadmapItem{{
			PublicID: "abcdefghijklmnop",
			Kind:     entity.RoadmapEntryProject,
			Title:    "Faster sync",
			Status:   entity.PublicStatusInProgress,
			Timeline: &entity.RoadmapTimeline{EndOn: "2026-12-01"},
			Progress: &progress,
			Issues: []entity.PublicRoadmapItem{{
				PublicID: "qrstuvwxyz234567",
				Kind:     entity.RoadmapEntryIssue,
				Title:    "Offline edits",
			}},
		}},
		Issues: []entity.PublicRoadmapItem{},
	}, nil)

	request := httptest.NewRequest(http.MethodGet, "/public/roadmap", nil)
	request.Host = "acme.roadmap.example.com"

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /public/roadmap = %d %s", recorder.Code, recorder.Body.String())
	}

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store so a withdrawn entry is never served from a cache", got)
	}

	var body any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if keys := unlistedKeys(body); len(keys) != 0 {
		t.Fatalf("public roadmap carries fields outside the allowlist: %v", keys)
	}
}

func TestAnUnknownPublicEntryAnswersNotFound(t *testing.T) {
	public, routes := publicRoadmapEdge(t)

	public.EXPECT().
		Entry(gomock.Any(), gomock.Any(), "a6f1b2c4-0000-4000-8000-000000000000").
		DoAndReturn(func(context.Context, string, string) (entity.PublicRoadmapItem, error) {
			return entity.PublicRoadmapItem{}, entity.ErrRoadmapEntryNotFound
		})

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/public/roadmap/entries/a6f1b2c4-0000-4000-8000-000000000000", nil,
	))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("guessed entry = %d %s, want 404", recorder.Code, recorder.Body.String())
	}
}
