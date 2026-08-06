package csvfile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/importrun"
)

const (
	readBuffer  = 1 << 20
	sniffAhead  = 64 << 10
	probeWindow = 64 << 10
	stallMost   = 2

	headerNamed   = "the header"
	firstRowNamed = "the first row"
)

var (
	utf8Mark    = []byte{0xEF, 0xBB, 0xBF}
	utf16LEMark = []byte{0xFF, 0xFE}
	utf16BEMark = []byte{0xFE, 0xFF}
)

func delimiters() []rune { return []rune{',', ';', '\t', '|'} }

// open holds the key settings carry to the storage an import owns. The key is chosen by
// whoever configured the run, and handed to blob storage unchecked it addresses any object in
// the bucket. A staging pass knows its run and is held to that run's own prefix; a probe runs
// before a pass exists and can only insist on the import area, which is addressed by two
// opaque identifiers rather than by anything guessable.
func (s *Source) open(
	ctx context.Context,
	resource entity.ImportResource,
	settings Settings,
) (io.ReadSeekCloser, error) {
	key := strings.TrimSpace(settings.ObjectKey)

	if !strings.HasPrefix(key, reachable(ctx)) {
		return nil, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason: "this run's file sits outside the storage an import may read, so it is left " +
				"where it is; upload the file to this run and configure it again",
		}
	}

	file, err := s.blobs.Open(ctx, key)
	if err != nil {
		if errors.Is(err, entity.ErrBlobNotFound) {
			return nil, entity.ImportSourceRefusedError{
				Resource: resource,
				Reason:   "the file this run was configured with is no longer in storage",
				Cause:    err,
			}
		}

		return nil, entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   "the uploaded file could not be opened: " + err.Error(),
			Cause:    err,
		}
	}

	return file, nil
}

func reachable(ctx context.Context) string {
	run, addressed := importrun.From(ctx)
	if !addressed {
		return entity.ImportKeyPrefix + "/"
	}

	return entity.ImportBlobPrefix(run.WorkspaceID) + "/" + run.RunID.String() + "/"
}

// rowReader counts fields itself so that a short row is reported in this application's own
// words rather than as "wrong number of fields", and refuses a broken quote rather than
// guessing at it: a file whose quoting is wrong is worth telling somebody about, while a
// lazily repaired one silently merges two columns into one.
func rowReader(source io.Reader, comma rune) *csv.Reader {
	reader := csv.NewReader(source)

	reader.Comma = comma
	reader.FieldsPerRecord = -1

	return reader
}

// buffered wraps the file in a megabyte before encoding/csv sees it. The reader underneath is
// a blob range reader that issues one GET per Read, and csv's own 4 KB buffer would turn a
// 25 MB upload into roughly 6,400 requests.
func buffered(file io.Reader) *bufio.Reader {
	return bufio.NewReaderSize(file, readBuffer)
}

func preamble(head *bufio.Reader, resource entity.ImportResource) (int64, error) {
	mark, err := head.Peek(len(utf8Mark))
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, unreadable(resource, err)
	}

	if bytes.HasPrefix(mark, utf16LEMark) || bytes.HasPrefix(mark, utf16BEMark) {
		return 0, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason: "this file is UTF-16. Every row of it would read as one column of interleaved " +
				"nulls, so it is refused rather than imported as nonsense: open it in whatever wrote " +
				"it, save it as UTF-8, and upload it again",
		}
	}

	if !bytes.HasPrefix(mark, utf8Mark) {
		return 0, nil
	}

	if _, err := head.Discard(len(utf8Mark)); err != nil {
		return 0, unreadable(resource, err)
	}

	return int64(len(utf8Mark)), nil
}

func separator(
	head *bufio.Reader,
	settings Settings,
	resource entity.ImportResource,
) (rune, bool, error) {
	if settings.Delimiter != "" {
		comma, width := utf8.DecodeRuneInString(settings.Delimiter)

		if width != len(settings.Delimiter) || comma == utf8.RuneError ||
			comma == '"' || comma == '\r' || comma == '\n' {
			return 0, false, entity.ImportSourceRefusedError{
				Resource: resource,
				Reason: fmt.Sprintf(
					"this run names %q as the character between its columns, which no file can be read with",
					settings.Delimiter,
				),
			}
		}

		return comma, false, nil
	}

	line, err := head.Peek(sniffAhead)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return 0, false, unreadable(resource, err)
	}

	return sniffed(line), true, nil
}

// sniffed reads the separator off the first line. A comma is the name of the format and the
// answer when nothing else shows, but a spreadsheet in a locale where the comma is the decimal
// point exports semicolons, and reading one of those as a single column is the most ordinary
// way a CSV import fails.
func sniffed(head []byte) rune {
	line := head

	if cut := bytes.IndexByte(head, '\n'); cut >= 0 {
		line = head[:cut]
	}

	best, most := ',', 0

	for _, candidate := range delimiters() {
		if count := outsideQuotes(line, candidate); count > most {
			best, most = candidate, count
		}
	}

	return best
}

func outsideQuotes(line []byte, delimiter rune) int {
	count, quoted := 0, false

	for _, symbol := range string(line) {
		switch {
		case symbol == '"':
			quoted = !quoted
		case symbol == delimiter && !quoted:
			count++
		}
	}

	return count
}

func unreadable(resource entity.ImportResource, err error) error {
	return entity.ImportSourceUnavailableError{
		Resource: resource,
		Reason:   "the uploaded file could not be read: " + err.Error(),
		Cause:    err,
	}
}

type row struct {
	number int
	fields []string
	defect string
}

type scan struct {
	file     io.ReadSeekCloser
	rows     *csv.Reader
	resource entity.ImportResource
	bound    binding
	header   []string
	width    int
	against  string
	base     int64
	number   int
	pending  []string
	stalled  int
}

func newScan(
	file io.ReadSeekCloser,
	settings Settings,
	resource entity.ImportResource,
	from cursor,
) (*scan, error) {
	head := buffered(file)

	mark, err := preamble(head, resource)
	if err != nil {
		return nil, err
	}

	comma, _, err := separator(head, settings, resource)
	if err != nil {
		return nil, err
	}

	reader := rowReader(head, comma)

	first, err := reader.Read()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason:   "the first row of this file cannot be read: " + err.Error(),
			Cause:    err,
		}
	}

	held := &scan{
		file:     file,
		rows:     reader,
		resource: resource,
		base:     mark,
		number:   1,
		width:    len(first),
		against:  firstRowNamed,
	}

	if len(first) > 0 {
		if headerRow(settings, first) {
			held.header = first
			held.against = headerNamed
		} else {
			held.pending = first
		}
	}

	held.bound, err = bound(settings, held.header, resource)
	if err != nil {
		return nil, err
	}

	if from.Offset > 0 {
		if err := held.resume(comma, from); err != nil {
			return nil, err
		}
	}

	return held, nil
}

// resume starts again on a record boundary. The offset a page ends on is the reader's own
// input offset after a completed record, so the seek lands between two rows rather than inside
// a quoted field that would then read as the tail of one row and the head of the next.
func (s *scan) resume(comma rune, from cursor) error {
	if _, err := s.file.Seek(from.Offset, io.SeekStart); err != nil {
		return entity.ImportSourceUnavailableError{
			Resource: s.resource,
			Reason: "the uploaded file could not be read from where the last page stopped: " +
				err.Error(),
			Cause: err,
		}
	}

	s.rows = rowReader(buffered(s.file), comma)
	s.base = from.Offset
	s.number = from.Row
	s.pending = nil

	return nil
}

func (s *scan) next() (row, bool, error) {
	if s.pending != nil {
		fields := s.pending
		s.pending = nil

		return row{number: s.number, fields: fields}, true, nil
	}

	for {
		before := s.rows.InputOffset()

		fields, err := s.rows.Read()

		switch {
		case errors.Is(err, io.EOF):
			return row{}, false, nil

		case err != nil:
			broken, malformed := brokenRow(err)
			if !malformed {
				return row{}, false, unreadable(s.resource, err)
			}

			if s.rows.InputOffset() == before {
				if s.stalled++; s.stalled >= stallMost {
					return row{}, false, s.stuck(before, broken, err)
				}

				continue
			}

			s.stalled = 0
			s.number++

			return row{number: s.number, defect: broken}, true, nil

		default:
			s.stalled = 0
			s.number++

			return s.measured(fields), true, nil
		}
	}
}

func (s *scan) measured(fields []string) row {
	if len(fields) == s.width {
		return row{number: s.number, fields: fields}
	}

	return row{
		number: s.number,
		defect: fmt.Sprintf(
			"row %d has %d fields, %s has %d", s.number, len(fields), s.against, s.width,
		),
	}
}

func (s *scan) stuck(at int64, broken string, cause error) error {
	return entity.ImportSourceRefusedError{
		Resource: s.resource,
		Reason: fmt.Sprintf(
			"this file stops being readable %d bytes in, and reading it again makes no progress, "+
				"so the rest of it is left alone rather than walked forever: %s",
			s.base+at, broken,
		),
		Cause: cause,
	}
}

func brokenRow(err error) (string, bool) {
	var broken *csv.ParseError

	if !errors.As(err, &broken) {
		return "", false
	}

	return "this row cannot be read: " + broken.Err.Error(), true
}

func (s *scan) offset() int64 { return s.base + s.rows.InputOffset() }

func (s *scan) close() { _ = s.file.Close() }
