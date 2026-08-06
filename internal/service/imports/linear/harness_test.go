package linear_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/importrun"
	"github.com/usenorn/norn/internal/pkg/lineargraph"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/linear"
)

const (
	sourceKey = "lin_api_key"

	uploadHost           = "uploads.linear.app"
	defaultPageSize      = 50
	defaultAttachmentCap = 1 << 20
)

var (
	stagingWorkspace = uuid.MustParse("6f4d2f5a-3f2b-4c1e-9a7d-2b8f1c0e5a44")
	stagingRun       = uuid.MustParse("b31e9c47-0a55-4d6f-8f2a-77c9e5d10b23")
)

type graphCall struct {
	Operation string
	Query     string
	Variables map[string]any
}

type storedObject struct {
	Key         string
	ContentType string
	Size        int64
	Bytes       []byte
}

type blobStore struct {
	repository.Blob

	mu     sync.Mutex
	stored []storedObject
	refuse error
}

func (b *blobStore) Put(_ context.Context, key, contentType string, body io.Reader, size int64) error {
	read, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.refuse != nil {
		return b.refuse
	}

	b.stored = append(b.stored, storedObject{
		Key:         key,
		ContentType: contentType,
		Size:        size,
		Bytes:       read,
	})

	return nil
}

func (b *blobStore) objects() []storedObject {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]storedObject{}, b.stored...)
}

type stand struct {
	t        *testing.T
	endpoint string
	blobs    *blobStore
	limit    int64
	page     int

	mu     sync.Mutex
	calls  []graphCall
	answer func(graphCall) string
	serve  http.HandlerFunc
}

func standing(t *testing.T) *stand {
	t.Helper()

	held := &stand{
		t:      t,
		blobs:  &blobStore{},
		limit:  defaultAttachmentCap,
		page:   defaultPageSize,
		answer: func(graphCall) string { return `{"data":{}}` },
		serve: func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
		},
	}

	graph := httptest.NewServer(http.HandlerFunc(held.graphql))
	t.Cleanup(graph.Close)

	held.endpoint = graph.URL

	uploads := httptest.NewTLSServer(http.HandlerFunc(held.file))
	t.Cleanup(uploads.Close)

	held.resolveUploadsTo(uploads.Listener.Addr().String())

	return held
}

func (s *stand) resolveUploadsTo(addr string) {
	base, ours := http.DefaultTransport.(*http.Transport)
	if !ours {
		s.t.Fatalf("the default transport is not one this harness can redirect")
	}

	swapped := base.Clone()
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	swapped.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	swapped.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.HasPrefix(address, uploadHost+":") {
			address = addr
		}

		return dialer.DialContext(ctx, network, address)
	}

	http.DefaultTransport = swapped

	s.t.Cleanup(func() {
		swapped.CloseIdleConnections()

		http.DefaultTransport = base
	})
}

func (s *stand) graphql(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		s.t.Errorf("read the query the adapter sent: %v", err)
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	var sent struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}

	if err := json.Unmarshal(body, &sent); err != nil {
		s.t.Errorf("the adapter sent %s, which is not a GraphQL request: %v", body, err)
		writer.WriteHeader(http.StatusBadRequest)

		return
	}

	call := graphCall{Operation: operationOf(sent.Query), Query: sent.Query, Variables: sent.Variables}

	s.mu.Lock()
	s.calls = append(s.calls, call)
	answer := s.answer
	s.mu.Unlock()

	writer.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(writer, answer(call))
}

func (s *stand) file(writer http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	serve := s.serve
	s.mu.Unlock()

	serve(writer, request)
}

func operationOf(query string) string {
	rest := strings.TrimPrefix(strings.TrimSpace(query), "query ")

	if cut := strings.IndexAny(rest, "({ "); cut >= 0 {
		rest = rest[:cut]
	}

	return rest
}

func (s *stand) answering(replies map[string]string) *stand {
	return s.replying(func(call graphCall) string {
		data, held := replies[call.Operation]
		if !held {
			return `{"data":{}}`
		}

		return `{"data":` + data + `}`
	})
}

func (s *stand) replying(answer func(graphCall) string) *stand {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.answer = answer

	return s
}

func (s *stand) holding(serve http.HandlerFunc) *stand {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.serve = serve

	return s
}

func (s *stand) capping(bytes int64) *stand {
	s.limit = bytes

	return s
}

func (s *stand) paging(size int) *stand {
	s.page = size

	return s
}

func (s *stand) source() *linear.Source {
	cfg := config.Linear{
		Endpoint:        s.endpoint,
		RequestTimeout:  10 * time.Second,
		MaxResponseSize: 8 << 20,
		PageSize:        s.page,
	}

	return linear.New(
		lineargraph.New(cfg), s.blobs, cfg, config.Imports{MaxAttachmentBytes: s.limit},
	)
}

func (s *stand) seen() []graphCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]graphCall{}, s.calls...)
}

func (s *stand) only() graphCall {
	s.t.Helper()

	calls := s.seen()

	if len(calls) != 1 {
		s.t.Fatalf("the adapter made %d calls to the source, want exactly one", len(calls))
	}

	return calls[0]
}

func asking(resource entity.ImportResource) service.ImportFetchRequest {
	return service.ImportFetchRequest{
		Resource: resource,
		Config: service.ImportSourceConfig{
			Secret:   sourceKey,
			Settings: json.RawMessage(`{"teamIds":["` + engineeringTeam + `","` + operationsTeam + `"]}`),
		},
	}
}

func staging() context.Context {
	return importrun.With(context.Background(), importrun.Run{
		WorkspaceID: stagingWorkspace,
		RunID:       stagingRun,
	})
}

func noted() (context.Context, *bytes.Buffer) {
	said := &bytes.Buffer{}

	return logging.Into(staging(), slog.New(slog.NewJSONHandler(said, nil))), said
}

func fetched(
	t *testing.T,
	ctx context.Context,
	source *linear.Source,
	request service.ImportFetchRequest,
) service.ImportFetchPage {
	t.Helper()

	page, err := source.Fetch(ctx, request)
	if err != nil {
		t.Fatalf("fetch %s: %v", request.Resource, err)
	}

	return page
}

func payloadOf[T any](t *testing.T, record entity.ImportRecord) T {
	t.Helper()

	var held T

	if err := json.Unmarshal(record.Payload, &held); err != nil {
		t.Fatalf("decode the %T staged for %q: %v", held, record.ExternalID, err)
	}

	return held
}

func recordNamed(t *testing.T, page service.ImportFetchPage, externalID string) entity.ImportRecord {
	t.Helper()

	for _, record := range page.Records {
		if record.ExternalID == externalID {
			return record
		}
	}

	t.Fatalf("no record for %q; the page holds %v", externalID, externalIDs(page))

	return entity.ImportRecord{}
}

func externalIDs(page service.ImportFetchPage) []string {
	held := make([]string, 0, len(page.Records))

	for _, record := range page.Records {
		held = append(held, record.ExternalID)
	}

	return held
}

func teamsIn(t *testing.T, call graphCall) []string {
	t.Helper()

	sent, held := call.Variables["teams"].([]any)
	if !held {
		return nil
	}

	teams := make([]string, 0, len(sent))

	for _, team := range sent {
		named, text := team.(string)
		if !text {
			t.Fatalf("%s was asked for team %v, which is not an identifier", call.Operation, team)
		}

		teams = append(teams, named)
	}

	return teams
}

func numberIn(t *testing.T, call graphCall, name string) int {
	t.Helper()

	held, sent := call.Variables[name].(float64)
	if !sent {
		t.Fatalf("%s carried no %q variable; it sent %v", call.Operation, name, call.Variables)
	}

	return int(held)
}
