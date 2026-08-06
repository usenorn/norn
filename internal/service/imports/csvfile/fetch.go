package csvfile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type cursor struct {
	Pass   string `json:"pass"`
	Offset int64  `json:"offset"`
	Row    int    `json:"row"`
}

func (c cursor) encode() string {
	held, err := json.Marshal(c)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(held)
}

func decodeCursor(resource entity.ImportResource, text string) (cursor, error) {
	if strings.TrimSpace(text) == "" {
		return cursor{Pass: string(resource)}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return cursor{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason:   "this run resumes from a cursor this source did not write",
			Cause:    err,
		}
	}

	var at cursor

	if err := json.Unmarshal(raw, &at); err != nil {
		return cursor{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason:   "this run resumes from a cursor this source did not write",
			Cause:    err,
		}
	}

	if at.Pass != string(resource) {
		return cursor{}, entity.ImportSourceRefusedError{
			Resource: resource,
			Reason: fmt.Sprintf(
				"this run resumes %s from a cursor the %s pass wrote, which addresses a row in "+
					"neither of them",
				resource, at.Pass,
			),
		}
	}

	return at, nil
}

func teamPage(settings Settings) (service.ImportFetchPage, error) {
	record, err := recordOf(teamExternalID, "", nil, settings.team())
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	return service.ImportFetchPage{Records: []entity.ImportRecord{record}, Done: true}, nil
}

func (s *Source) walking(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (*scan, error) {
	from, err := decodeCursor(request.Resource, request.Cursor)
	if err != nil {
		return nil, err
	}

	file, err := s.open(ctx, request.Resource, settings)
	if err != nil {
		return nil, err
	}

	held, err := newScan(file, settings, request.Resource, from)
	if err != nil {
		_ = file.Close()

		return nil, err
	}

	return held, nil
}

func (s *Source) fetchLabels(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	held, err := s.walking(ctx, request, settings)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	defer held.close()

	records := make([]entity.ImportRecord, 0)
	seen := make(map[string]bool)

	for range pageRows(request.PageHint) {
		read, more, err := held.next()
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		if !more {
			return service.ImportFetchPage{Records: records, Done: true}, nil
		}

		if read.defect != "" {
			continue
		}

		for _, value := range labelsIn(held.bound.cell(read.fields, targetLabels)) {
			if seen[value] {
				continue
			}

			seen[value] = true

			record, err := recordOf(labelKey(value), "", nil, service.ImportLabelPayload{Name: value})
			if err != nil {
				return service.ImportFetchPage{}, err
			}

			records = append(records, record)
		}
	}

	return held.paged(records), nil
}

func (s *Source) fetchIssues(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	held, err := s.walking(ctx, request, settings)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	defer held.close()

	if !held.bound.holds(targetTitle) {
		return service.ImportFetchPage{}, entity.ImportSourceRefusedError{
			Resource: request.Resource,
			Reason: "no column in this file is read as an issue title. Every row would arrive " +
				"nameless and be refused one at a time, so the column is asked for before the " +
				"first of them is staged",
		}
	}

	records := make([]entity.ImportRecord, 0)
	taken := make(map[string]bool)

	for range pageRows(request.PageHint) {
		read, more, err := held.next()
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		if !more {
			return service.ImportFetchPage{Records: records, Done: true}, nil
		}

		record, err := issueRecord(held.bound, read, taken)
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return held.paged(records), nil
}

func (s *scan) paged(records []entity.ImportRecord) service.ImportFetchPage {
	return service.ImportFetchPage{
		Records: records,
		NextCursor: cursor{
			Pass:   string(s.resource),
			Offset: s.offset(),
			Row:    s.number,
		}.encode(),
	}
}

// issueRecord stages a row it could not read rather than failing on it: an error out of Fetch
// ends the whole run, and one row of a backlog is worth exactly itself. The framework skips a
// record carrying a defect and reports the sentence with it.
func issueRecord(at binding, read row, taken map[string]bool) (entity.ImportRecord, error) {
	position := rowPrefix + strconv.Itoa(read.number)

	if read.defect != "" {
		return recordOf(position, "", nil, service.ImportIssuePayload{Defect: read.defect})
	}

	// Two rows carrying the same identifier would be one statement naming the same record
	// twice, which Postgres refuses outright, so the second of them is addressed by where it
	// sits instead and the page still carries both.
	external := at.cell(read.fields, targetExternalID)
	if external == "" || taken[external] {
		external = position
	}

	taken[external] = true

	return recordOf(
		external,
		at.cell(read.fields, targetParent),
		moment(at.cell(read.fields, targetCreated)),
		service.ImportIssuePayload{
			Title:       at.cell(read.fields, targetTitle),
			Description: at.cell(read.fields, targetDescription),
			Team:        teamExternalID,
			State:       at.cell(read.fields, targetState),
			Priority:    at.cell(read.fields, targetPriority),
			Labels:      labelKeys(labelsIn(at.cell(read.fields, targetLabels))),
			Assignee:    person(at.cell(read.fields, targetAssignee)),
			Estimate:    number(at.cell(read.fields, targetEstimate)),
			DueOn:       day(at.cell(read.fields, targetDue)),
		},
	)
}

func recordOf(
	externalID, parentExternalID string,
	created *time.Time,
	payload any,
) (entity.ImportRecord, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return entity.ImportRecord{}, fmt.Errorf("encode %T for %q: %w", payload, externalID, err)
	}

	return entity.ImportRecord{
		ExternalID:       externalID,
		ParentExternalID: parentExternalID,
		Payload:          encoded,
		SourceCreatedAt:  created,
	}, nil
}

func labelsIn(cell string) []string {
	values := make([]string, 0)

	for _, part := range strings.FieldsFunc(cell, func(symbol rune) bool {
		return symbol == ',' || symbol == ';'
	}) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}

func labelKeys(values []string) []string {
	keys := make([]string, 0, len(values))

	for _, value := range values {
		keys = append(keys, labelKey(value))
	}

	return keys
}

func labelKey(value string) string { return labelPrefix + value }

// person carries the cell itself as the key the mapping plan is keyed on, because a file names
// whoever it names and nothing here can turn that into an account. An address doubles as the
// email so that a member of this workspace with the same one can be suggested.
func person(value string) service.ImportUser {
	if value == "" {
		return service.ImportUser{}
	}

	held := service.ImportUser{Key: value, Name: value}

	if strings.Contains(value, "@") {
		held.Email = value
	}

	return held
}

func number(value string) int {
	if whole, err := strconv.Atoi(value); err == nil {
		return whole
	}

	if fraction, err := strconv.ParseFloat(value, 64); err == nil {
		return int(math.Round(fraction))
	}

	return 0
}

func day(value string) string {
	at, read := parsed(value)
	if !read {
		return ""
	}

	return entity.FormatCalendarDate(at)
}

func moment(value string) *time.Time {
	at, read := parsed(value)
	if !read {
		return nil
	}

	return &at
}

func layouts() []string {
	return []string{
		time.DateOnly,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
}

func parsed(value string) (time.Time, bool) {
	for _, layout := range layouts() {
		if read, err := time.Parse(layout, value); err == nil {
			return read.UTC(), true
		}
	}

	return time.Time{}, false
}
