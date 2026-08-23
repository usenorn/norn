package runnerchannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/eventstream"
)

const (
	spoolPrefix    = "runner-out:"
	cursorPrefix   = "runner-cursor:"
	presencePrefix = "runner-conn:"
	seenPrefix     = "runner-seen:"

	fieldID      = "id"
	fieldType    = "type"
	fieldExec    = "exec"
	fieldIssued  = "ts"
	fieldPayload = "payload"

	seen = "1"

	beginning = "0-0"
)

type presenceRecord struct {
	Epoch        string    `json:"epoch"`
	SeenAt       time.Time `json:"seenAt"`
	Capacity     int       `json:"capacity,omitempty"`
	Used         int       `json:"used,omitempty"`
	Paused       bool      `json:"paused,omitempty"`
	DiskPressure bool      `json:"diskPressure,omitempty"`
	CPUPressure  bool      `json:"cpuPressure,omitempty"`
}

type channelRepository struct {
	writer *valkey.Client
	reader *valkey.Client
}

func New(writer *valkey.Client, reader *eventstream.ReadClient) repository.RunnerChannel {
	return &channelRepository{writer: writer, reader: reader.Client}
}

func spoolKey(runnerID uuid.UUID) string { return spoolPrefix + runnerID.String() }

func cursorKey(runnerID uuid.UUID) string { return cursorPrefix + runnerID.String() }

func presenceKey(runnerID uuid.UUID) string { return presencePrefix + runnerID.String() }

func seenKey(runnerID uuid.UUID, messageID string) string {
	return seenPrefix + runnerID.String() + ":" + messageID
}

func (r *channelRepository) Append(
	ctx context.Context,
	runnerID uuid.UUID,
	message entity.ChannelMessage,
) (string, error) {
	key := spoolKey(runnerID)

	cursor, err := r.writer.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: entity.ChannelSpoolWindow,
		Approx: true,
		Values: map[string]any{
			fieldID:      message.ID,
			fieldType:    string(message.Type),
			fieldExec:    message.ExecutionID,
			fieldIssued:  message.IssuedAt.UTC().Format(time.RFC3339Nano),
			fieldPayload: string(message.Payload),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("spool runner message: %w", err)
	}

	if err := r.writer.Expire(ctx, key, entity.ChannelSpoolTTL).Err(); err != nil {
		return "", fmt.Errorf("bound the runner spool: %w", err)
	}

	return cursor, nil
}

func (r *channelRepository) Read(
	ctx context.Context,
	runnerID uuid.UUID,
	cursor string,
) ([]entity.SpooledMessage, string, error) {
	streams, err := r.reader.XRead(ctx, &redis.XReadArgs{
		Streams: []string{spoolKey(runnerID), cursor},
		Block:   entity.ChannelReadBlock,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, cursor, nil
		}

		return nil, cursor, fmt.Errorf("read the runner spool: %w", err)
	}

	messages := make([]entity.SpooledMessage, 0)
	next := cursor

	for _, stream := range streams {
		for _, message := range stream.Messages {
			messages = append(messages, decode(message))
			next = message.ID
		}
	}

	return messages, next, nil
}

func (r *channelRepository) Cursor(ctx context.Context, runnerID uuid.UUID) (string, error) {
	cursor, err := r.writer.Get(ctx, cursorKey(runnerID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return beginning, nil
		}

		return "", fmt.Errorf("read the runner cursor: %w", err)
	}

	return cursor, nil
}

func (r *channelRepository) Acknowledge(
	ctx context.Context,
	runnerID uuid.UUID,
	cursor string,
) error {
	if err := r.writer.Set(
		ctx, cursorKey(runnerID), cursor, entity.ChannelSpoolTTL,
	).Err(); err != nil {
		return fmt.Errorf("record the runner cursor: %w", err)
	}

	return nil
}

func (r *channelRepository) Attach(
	ctx context.Context,
	runnerID uuid.UUID,
	epoch string,
	seenAt time.Time,
) error {
	return r.hold(ctx, runnerID, epoch, entity.RunnerLoad{}, seenAt)
}

func (r *channelRepository) Renew(
	ctx context.Context,
	runnerID uuid.UUID,
	epoch string,
	load entity.RunnerLoad,
	seenAt time.Time,
) error {
	held, err := r.Presence(ctx, runnerID)
	if err != nil {
		return err
	}

	if held.Epoch != epoch {
		return entity.ErrChannelDisplaced
	}

	return r.hold(ctx, runnerID, epoch, load, seenAt)
}

func (r *channelRepository) Detach(ctx context.Context, runnerID uuid.UUID, epoch string) error {
	held, err := r.Presence(ctx, runnerID)
	if err != nil {
		return err
	}

	if held.Epoch != epoch {
		return nil
	}

	if err := r.writer.Del(ctx, presenceKey(runnerID)).Err(); err != nil {
		return fmt.Errorf("release the runner channel: %w", err)
	}

	return nil
}

func (r *channelRepository) Presence(
	ctx context.Context,
	runnerID uuid.UUID,
) (entity.RunnerPresence, error) {
	raw, err := r.writer.Get(ctx, presenceKey(runnerID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return entity.RunnerPresence{RunnerID: runnerID}, nil
		}

		return entity.RunnerPresence{}, fmt.Errorf("read the runner channel holder: %w", err)
	}

	var record presenceRecord

	if err := json.Unmarshal(raw, &record); err != nil {
		return entity.RunnerPresence{}, fmt.Errorf("decode the runner channel holder: %w", err)
	}

	return entity.RunnerPresence{
		RunnerID: runnerID,
		Epoch:    record.Epoch,
		SeenAt:   record.SeenAt,
		Load: entity.RunnerLoad{
			Capacity:     record.Capacity,
			Used:         record.Used,
			Paused:       record.Paused,
			DiskPressure: record.DiskPressure,
			CPUPressure:  record.CPUPressure,
		},
	}, nil
}

func (r *channelRepository) Seen(
	ctx context.Context,
	runnerID uuid.UUID,
	messageID string,
) (bool, error) {
	err := r.writer.SetArgs(ctx, seenKey(runnerID, messageID), seen, redis.SetArgs{
		Mode: "NX",
		TTL:  entity.ChannelSeenTTL,
	}).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return true, nil
		}

		return false, fmt.Errorf("remember a runner message: %w", err)
	}

	return false, nil
}

func (r *channelRepository) hold(
	ctx context.Context,
	runnerID uuid.UUID,
	epoch string,
	load entity.RunnerLoad,
	seenAt time.Time,
) error {
	record, err := json.Marshal(presenceRecord{
		Epoch:        epoch,
		SeenAt:       seenAt.UTC(),
		Capacity:     load.Capacity,
		Used:         load.Used,
		Paused:       load.Paused,
		DiskPressure: load.DiskPressure,
		CPUPressure:  load.CPUPressure,
	})
	if err != nil {
		return fmt.Errorf("encode the runner channel holder: %w", err)
	}

	if err := r.writer.Set(
		ctx, presenceKey(runnerID), record, entity.ChannelLeaseTTL,
	).Err(); err != nil {
		return fmt.Errorf("hold the runner channel: %w", err)
	}

	return nil
}

func decode(message redis.XMessage) entity.SpooledMessage {
	issued, _ := time.Parse(time.RFC3339Nano, text(message.Values[fieldIssued]))

	return entity.SpooledMessage{
		Cursor: message.ID,
		Message: entity.ChannelMessage{
			ID:          text(message.Values[fieldID]),
			Type:        entity.ChannelMessageType(text(message.Values[fieldType])),
			ExecutionID: text(message.Values[fieldExec]),
			IssuedAt:    issued,
			Payload:     []byte(text(message.Values[fieldPayload])),
		},
	}
}

func text(value any) string {
	if value == nil {
		return ""
	}

	raw, _ := value.(string)

	return raw
}
