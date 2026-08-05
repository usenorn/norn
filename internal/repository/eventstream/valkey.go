package eventstream

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
)

const keyPrefix = "norn:events:"

const (
	fieldKind    = "kind"
	fieldTeam    = "team"
	fieldSubject = "subject"
	fieldIssue   = "issue"
	fieldAccount = "account"
	fieldActor   = "actor"
	fieldPayload = "payload"
)

type eventStream struct {
	writer *valkey.Client
	reader *valkey.Client
}

// New takes two clients because a blocking XREAD outlives the shared client's read timeout by
// design. Reads go to a connection whose timeout is disabled; everything else uses the pool.
func New(writer *valkey.Client, reader *ReadClient) repository.EventStream {
	return &eventStream{writer: writer, reader: reader.Client}
}

func key(workspaceID uuid.UUID) string {
	return keyPrefix + workspaceID.String()
}

func identifier(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (s *eventStream) Publish(ctx context.Context, event entity.Event) error {
	if err := s.writer.XAdd(ctx, &redis.XAddArgs{
		Stream: key(event.WorkspaceID),
		MaxLen: entity.EventStreamWindow,
		Approx: true,
		Values: map[string]any{
			fieldKind:    string(event.Kind),
			fieldTeam:    identifier(event.TeamID),
			fieldSubject: identifier(event.SubjectID),
			fieldIssue:   identifier(event.IssueID),
			fieldAccount: identifier(event.AccountID),
			fieldActor:   identifier(event.ActorID),
			fieldPayload: string(event.Payload),
		},
	}).Err(); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}

func (s *eventStream) Since(
	ctx context.Context,
	workspaceID uuid.UUID,
	cursor string,
) ([]entity.Event, error) {
	oldest, err := s.writer.XRangeN(ctx, key(workspaceID), "-", "+", 1).Result()
	if err != nil {
		return nil, fmt.Errorf("read oldest event: %w", err)
	}

	// A cursor older than anything still in the stream returns rows without complaint, so the
	// gap has to be detected here or the client believes a partial replay was a complete one.
	// The oldest surviving entry being newer than the cursor is exactly that gap.
	if len(oldest) > 0 && after(oldest[0].ID, cursor) {
		return nil, entity.ErrEventStreamLapsed
	}

	messages, err := s.writer.XRangeN(
		ctx, key(workspaceID), "("+cursor, "+", entity.EventStreamWindow,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("read missed events: %w", err)
	}

	return decode(workspaceID, messages), nil
}

func after(candidate, cursor string) bool {
	candidateTime, candidateSeq := split(candidate)
	cursorTime, cursorSeq := split(cursor)

	if candidateTime != cursorTime {
		return candidateTime > cursorTime
	}

	return candidateSeq > cursorSeq
}

func split(id string) (int64, int64) {
	milliseconds, sequence, _ := strings.Cut(id, "-")

	return number(milliseconds), number(sequence)
}

func number(raw string) int64 {
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return parsed
}

func (s *eventStream) Read(
	ctx context.Context,
	workspaceID uuid.UUID,
	cursor string,
) ([]entity.Event, string, error) {
	streams, err := s.reader.XRead(ctx, &redis.XReadArgs{
		Streams: []string{key(workspaceID), cursor},
		Block:   entity.EventBlockSeconds * time.Second,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, cursor, nil
		}

		return nil, cursor, fmt.Errorf("read events: %w", err)
	}

	events := make([]entity.Event, 0)
	next := cursor

	for _, stream := range streams {
		events = append(events, decode(workspaceID, stream.Messages)...)

		if len(stream.Messages) > 0 {
			next = stream.Messages[len(stream.Messages)-1].ID
		}
	}

	return events, next, nil
}

func (s *eventStream) Latest(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	messages, err := s.writer.XRevRangeN(ctx, key(workspaceID), "+", "-", 1).Result()
	if err != nil {
		return "", fmt.Errorf("read latest event: %w", err)
	}

	if len(messages) == 0 {
		return "0-0", nil
	}

	return messages[0].ID, nil
}

func decode(workspaceID uuid.UUID, messages []redis.XMessage) []entity.Event {
	events := make([]entity.Event, 0, len(messages))

	for _, message := range messages {
		events = append(events, entity.Event{
			ID:          message.ID,
			WorkspaceID: workspaceID,
			Kind:        entity.EventKind(text(message.Values[fieldKind])),
			TeamID:      parse(message.Values[fieldTeam]),
			SubjectID:   parse(message.Values[fieldSubject]),
			IssueID:     parse(message.Values[fieldIssue]),
			AccountID:   parse(message.Values[fieldAccount]),
			ActorID:     parse(message.Values[fieldActor]),
			Payload:     []byte(text(message.Values[fieldPayload])),
		})
	}

	return events
}

func text(value any) string {
	found, ok := value.(string)
	if !ok {
		return ""
	}

	return found
}

func parse(value any) uuid.UUID {
	parsed, err := uuid.Parse(text(value))
	if err != nil {
		return uuid.Nil
	}

	return parsed
}
