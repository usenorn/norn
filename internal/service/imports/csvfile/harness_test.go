package csvfile_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/importrun"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
)

const uploadName = "backlog.csv"

var (
	stagingWorkspace = uuid.MustParse("2c9a1e77-58f4-4a1f-9d0b-1f6c3b7a2e51")
	stagingRun       = uuid.MustParse("7d4b0c19-6e23-4f88-b5a1-90ac2f5e6d34")
)

func uploadKey() string {
	return entity.ImportBlobKey(stagingWorkspace, stagingRun, uploadName)
}

type reading struct {
	body   []byte
	offset int64
	reads  int
	closed bool
}

func (r *reading) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}

	r.reads++

	if r.offset >= int64(len(r.body)) {
		return 0, io.EOF
	}

	read := copy(buffer, r.body[r.offset:])
	r.offset += int64(read)

	return read, nil
}

func (r *reading) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset += offset
	case io.SeekEnd:
		r.offset = int64(len(r.body)) + offset
	default:
		return 0, errors.New("seek from nowhere")
	}

	if r.offset < 0 {
		return 0, errors.New("seek before the start of the object")
	}

	return r.offset, nil
}

func (r *reading) Close() error {
	r.closed = true

	return nil
}

type store struct {
	repository.Blob

	objects map[string][]byte
	opened  []*reading
}

func (s *store) Open(_ context.Context, key string) (io.ReadSeekCloser, error) {
	body, held := s.objects[key]
	if !held {
		return nil, entity.ErrBlobNotFound
	}

	opened := &reading{body: body}
	s.opened = append(s.opened, opened)

	return opened, nil
}

type stand struct {
	t     *testing.T
	blobs *store
}

func standing(t *testing.T, body string) *stand {
	t.Helper()

	return &stand{
		t:     t,
		blobs: &store{objects: map[string][]byte{uploadKey(): []byte(body)}},
	}
}

func (s *stand) source() *csvfile.Source {
	return csvfile.New(s.blobs)
}

func (s *stand) reads() int {
	total := 0

	for _, opened := range s.blobs.opened {
		total += opened.reads
	}

	return total
}

func (s *stand) leftOpen() int {
	open := 0

	for _, opened := range s.blobs.opened {
		if !opened.closed {
			open++
		}
	}

	return open
}

func staging() context.Context {
	return importrun.With(context.Background(), importrun.Run{
		WorkspaceID: stagingWorkspace,
		RunID:       stagingRun,
	})
}

func configured(t *testing.T, held csvfile.Settings) service.ImportSourceConfig {
	t.Helper()

	if held.ObjectKey == "" {
		held.ObjectKey = uploadKey()
	}

	encoded, err := json.Marshal(held)
	if err != nil {
		t.Fatalf("encode the settings this run would be configured with: %v", err)
	}

	return service.ImportSourceConfig{Settings: encoded}
}

func asking(
	t *testing.T,
	resource entity.ImportResource,
	held csvfile.Settings,
) service.ImportFetchRequest {
	t.Helper()

	return service.ImportFetchRequest{Resource: resource, Config: configured(t, held)}
}

func fetched(
	t *testing.T,
	source *csvfile.Source,
	request service.ImportFetchRequest,
) service.ImportFetchPage {
	t.Helper()

	page, err := source.Fetch(staging(), request)
	if err != nil {
		t.Fatalf("fetch %s: %v", request.Resource, err)
	}

	return page
}

func probed(t *testing.T, source *csvfile.Source, held csvfile.Settings) entity.ImportCatalogue {
	t.Helper()

	catalogue, err := source.Probe(context.Background(), configured(t, held))
	if err != nil {
		t.Fatalf("probe the uploaded file: %v", err)
	}

	return catalogue
}

func payloadOf[T any](t *testing.T, record entity.ImportRecord) T {
	t.Helper()

	var held T

	if err := json.Unmarshal(record.Payload, &held); err != nil {
		t.Fatalf("decode the %T staged for %q: %v", held, record.ExternalID, err)
	}

	return held
}

func issuesIn(t *testing.T, page service.ImportFetchPage) map[string]service.ImportIssuePayload {
	t.Helper()

	held := make(map[string]service.ImportIssuePayload, len(page.Records))

	for _, record := range page.Records {
		held[record.ExternalID] = payloadOf[service.ImportIssuePayload](t, record)
	}

	return held
}

func titlesIn(t *testing.T, page service.ImportFetchPage) []string {
	t.Helper()

	titles := make([]string, 0, len(page.Records))

	for _, record := range page.Records {
		titles = append(titles, payloadOf[service.ImportIssuePayload](t, record).Title)
	}

	return titles
}

func externalIDs(page service.ImportFetchPage) []string {
	held := make([]string, 0, len(page.Records))

	for _, record := range page.Records {
		held = append(held, record.ExternalID)
	}

	return held
}

func refusal(t *testing.T, err error) entity.ImportSourceRefusedError {
	t.Helper()

	var refused entity.ImportSourceRefusedError

	if !errors.As(err, &refused) {
		t.Fatalf("the source answered %v, which is not a refusal this run can be told about", err)
	}

	return refused
}

func saying(t *testing.T, said, wanted string) {
	t.Helper()

	if !strings.Contains(strings.ToLower(said), strings.ToLower(wanted)) {
		t.Errorf(
			"the message reads %q and never mentions %q. A refusal a person cannot act on leaves "+
				"them with a run that failed and no idea which file to change.",
			said, wanted,
		)
	}
}
