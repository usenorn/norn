package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
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

func roadmapEdge(t *testing.T) (*roadmapsvc.MockRoadmaps, http.Handler) {
	t.Helper()

	ctrl := gomock.NewController(t)
	roadmaps := roadmapsvc.NewMockRoadmaps(ctrl)

	return roadmaps, newEdge(ctrl, edgeServices{roadmaps: roadmaps})
}

func serve(t *testing.T, routes http.Handler, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, recorder.Code, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return recorder, body
}

func TestTheDraftPreviewIsTheSameAllowlistedUncachedShapeAsThePublicRoadmap(t *testing.T) {
	roadmaps, routes := roadmapEdge(t)
	workspaceID := uuid.New()
	blocked := uuid.New()
	item := entity.PublicRoadmapItem{
		PublicID: "abcdefghijklmnop",
		Kind:     entity.RoadmapEntryProject,
		Title:    "Faster sync",
		Status:   entity.PublicStatusInProgress,
		Timeline: &entity.RoadmapTimeline{EndOn: "2026-12-01"},
		Issues:   []entity.PublicRoadmapItem{},
	}

	roadmaps.EXPECT().Preview(gomock.Any(), workspaceID).Return(service.RoadmapPreview{
		Version: 4,
		Digest:  "digest",
		Roadmap: entity.PublicRoadmap{
			Title:       "Acme roadmap",
			PublishedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			Projects:    []entity.PublicRoadmapItem{item},
			Issues:      []entity.PublicRoadmapItem{},
		},
		Blockers: []entity.RoadmapBlocker{{Code: entity.RoadmapBlockerEntryUnreachable, EntryID: blocked}},
	}, nil)
	roadmaps.EXPECT().PreviewEntry(gomock.Any(), workspaceID, item.PublicID).Return(item, nil)

	recorder, body := serve(t, routes, "/workspaces/"+workspaceID.String()+"/roadmap/preview")

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	if keys := unlistedKeys(body["roadmap"]); len(keys) != 0 {
		t.Fatalf("previewed roadmap carries fields outside the public allowlist: %v", keys)
	}

	blockers, _ := body["blockers"].([]any)
	if body["digest"] != "digest" || len(blockers) != 1 || blockers[0].(map[string]any)["entryId"] != blocked.String() {
		t.Fatalf("preview = %v, want the digest and the blocked entry", body)
	}

	recorder, entry := serve(t, routes, "/workspaces/"+workspaceID.String()+"/roadmap/preview/entries/"+item.PublicID)

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("entry Cache-Control = %q, want no-store", got)
	}

	if keys := unlistedKeys(entry); len(keys) != 0 || entry["title"] != "Faster sync" {
		t.Fatalf("previewed entry = %v, unlisted %v", entry, keys)
	}
}

func TestTheRoadmapCarriesItsReviewAndANullAddressWithoutADomain(t *testing.T) {
	roadmaps, routes := roadmapEdge(t)
	workspaceID := uuid.New()
	entryID := uuid.New()

	roadmaps.EXPECT().Get(gomock.Any(), workspaceID).Return(service.RoadmapDraftView{
		Roadmap: entity.Roadmap{ID: uuid.New(), WorkspaceID: workspaceID, Title: "Acme roadmap", Version: 2},
		Entries: []service.RoadmapEntryView{{
			Entry: entity.RoadmapEntry{
				ID:           entryID,
				PublicID:     "abcdefghijklmnop",
				Kind:         entity.RoadmapEntryIssue,
				PublicTitle:  "Offline edits",
				Status:       entity.PublicStatusPlanned,
				ProgressMode: entity.RoadmapProgressHidden,
			},
			Reachable:       true,
			SourceLabel:     "CRDT merge for drafts",
			SourceReference: "ENG-7",
		}},
		Review: entity.RoadmapReview{
			HasUnpublishedChanges: true,
			Digest:                "digest",
			Entries: []entity.RoadmapEntryReview{{
				EntryID:  entryID,
				PublicID: "abcdefghijklmnop",
				Kind:     entity.RoadmapEntryIssue,
				Title:    "Offline edits",
				Change:   entity.RoadmapEntryAdded,
				Fields: []entity.RoadmapFieldChange{
					{Field: entity.RoadmapReviewTitle, Kind: entity.RoadmapFieldExposed, After: "Offline edits"},
				},
			}},
		},
	}, nil)

	_, body := serve(t, routes, "/workspaces/"+workspaceID.String()+"/roadmap")

	if address, present := body["publicUrl"]; !present || address != nil {
		t.Fatalf("publicUrl = %v (present %v), want an explicit null", address, present)
	}

	review, _ := body["review"].(map[string]any)
	entries, _ := review["entries"].([]any)

	if review["hasUnpublishedChanges"] != true || review["digest"] != "digest" || len(entries) != 1 {
		t.Fatalf("review = %v, want the added entry and its digest", review)
	}

	fields, _ := entries[0].(map[string]any)["fields"].([]any)
	title, _ := fields[0].(map[string]any)

	if _, hasBefore := title["before"]; hasBefore || title["after"] != "Offline edits" || title["kind"] != "exposed" {
		t.Fatalf("title change = %v, want only what will be exposed", title)
	}

	draftEntries, _ := body["entries"].([]any)
	source, _ := draftEntries[0].(map[string]any)["source"].(map[string]any)

	if source["label"] != "CRDT merge for drafts" || source["reference"] != "ENG-7" {
		t.Fatalf("entry source = %v", source)
	}
}

func TestPublishingAReviewThatChangedAnswersAConflictWithItsCode(t *testing.T) {
	roadmaps, routes := roadmapEdge(t)
	workspaceID := uuid.New()

	roadmaps.EXPECT().
		Publish(gomock.Any(), workspaceID, service.PublishRoadmapInput{ExpectedVersion: 3, ExpectedDigest: "seen"}).
		Return(service.RoadmapDraftView{}, entity.ErrRoadmapReviewChanged)

	request := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/roadmap/publish",
		strings.NewReader(`{"expectedVersion":3,"expectedDigest":"seen"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	var problem map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if recorder.Code != http.StatusConflict || problem["code"] != "roadmap_review_changed" {
		t.Fatalf("publish = %d %v, want 409 roadmap_review_changed", recorder.Code, problem)
	}
}
