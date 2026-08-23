package runnerchannel_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/eventstream"
	"github.com/usenorn/norn/internal/repository/runnerchannel"
)

func newChannel(t *testing.T) (repository.RunnerChannel, uuid.UUID) {
	t.Helper()

	addr := strings.TrimSpace(os.Getenv("NORN_VALKEY_ADDR"))
	if addr == "" {
		t.Skip("NORN_VALKEY_ADDR is unset, so there is no valkey to spool against")
	}

	cfg := config.Valkey{Addr: addr, PoolSize: 4, DialTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second}

	writer, closeWriter, err := valkey.New(cfg)
	if err != nil {
		t.Skipf("no valkey at %s: %v", addr, err)
	}

	t.Cleanup(closeWriter)

	reader, closeReader, err := eventstream.NewReadClient(cfg)
	if err != nil {
		t.Fatalf("open the blocking reader: %v", err)
	}

	t.Cleanup(closeReader)

	return runnerchannel.New(writer, reader), uuid.New()
}

func message(id string) entity.ChannelMessage {
	return entity.ChannelMessage{
		ID:       id,
		Type:     entity.ChannelSync,
		IssuedAt: time.Now().UTC(),
		Payload:  []byte(`{"executions":[]}`),
	}
}

func TestASpooledMessageComesBackWithWhatWentIn(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	if _, err := channel.Append(ctx, runnerID, message("one")); err != nil {
		t.Fatalf("append: %v", err)
	}

	spooled, next, err := channel.Read(ctx, runnerID, "0-0")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(spooled) != 1 {
		t.Fatalf("read %d messages, want 1", len(spooled))
	}

	if spooled[0].Message.ID != "one" || spooled[0].Message.Type != entity.ChannelSync {
		t.Fatalf("the message did not survive the round trip: %+v", spooled[0].Message)
	}

	if string(spooled[0].Message.Payload) != `{"executions":[]}` {
		t.Fatalf("payload came back as %q", spooled[0].Message.Payload)
	}

	if next != spooled[0].Cursor {
		t.Fatalf("read returned cursor %q, want the last entry's %q", next, spooled[0].Cursor)
	}
}

func TestOnlyWhatFollowsTheAcknowledgedCursorIsReplayed(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	for _, id := range []string{"one", "two", "three"} {
		if _, err := channel.Append(ctx, runnerID, message(id)); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}

	spooled, _, err := channel.Read(ctx, runnerID, "0-0")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(spooled) != 3 {
		t.Fatalf("read %d messages, want 3", len(spooled))
	}

	if err := channel.Acknowledge(ctx, runnerID, spooled[0].Cursor); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}

	cursor, err := channel.Cursor(ctx, runnerID)
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}

	replayed, _, err := channel.Read(ctx, runnerID, cursor)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(replayed) != 2 {
		t.Fatalf(
			"replayed %d messages after acknowledging the first of three, want 2; a runner that "+
				"reconnects must not be handed work it already took",
			len(replayed),
		)
	}

	if replayed[0].Message.ID != "two" {
		t.Fatalf("replay began at %q, want it to resume after the acknowledged one", replayed[0].Message.ID)
	}
}

func TestAnUnreadRunnerSpoolDoesNotLiveForever(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	if _, err := channel.Append(ctx, runnerID, message("one")); err != nil {
		t.Fatalf("append: %v", err)
	}

	addr := strings.TrimSpace(os.Getenv("NORN_VALKEY_ADDR"))

	client, closeClient, err := valkey.New(config.Valkey{
		Addr: addr, PoolSize: 2, DialTimeout: 2 * time.Second,
		WriteTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("open valkey: %v", err)
	}

	defer closeClient()

	ttl, err := client.TTL(ctx, "runner-out:"+runnerID.String()).Result()
	if err != nil {
		t.Fatalf("read ttl: %v", err)
	}

	if ttl <= 0 {
		t.Fatalf(
			"the spool key has no expiry, so a machine that never comes back leaves its stream "+
				"in valkey forever (ttl %s)",
			ttl,
		)
	}
}

func TestTheNewestConnectionHoldsTheChannel(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	now := time.Now().UTC()

	if err := channel.Attach(ctx, runnerID, "first", now); err != nil {
		t.Fatalf("attach first: %v", err)
	}

	if err := channel.Renew(ctx, runnerID, "first", entity.RunnerLoad{}, now); err != nil {
		t.Fatalf("the only connection could not renew: %v", err)
	}

	if err := channel.Attach(ctx, runnerID, "second", now); err != nil {
		t.Fatalf("attach second: %v", err)
	}

	if err := channel.Renew(ctx, runnerID, "first", entity.RunnerLoad{}, now); err == nil {
		t.Fatalf("a displaced connection renewed its hold; both would stay open")
	}

	if err := channel.Renew(ctx, runnerID, "second", entity.RunnerLoad{}, now); err != nil {
		t.Fatalf("the newest connection could not renew: %v", err)
	}

	held, err := channel.Presence(ctx, runnerID)
	if err != nil {
		t.Fatalf("presence: %v", err)
	}

	if !held.Live() || held.Epoch != "second" {
		t.Fatalf("presence reports %+v, want the second connection holding it", held)
	}
}

func TestDetachingLeavesTheRunnerAbsent(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	if err := channel.Attach(ctx, runnerID, "first", time.Now().UTC()); err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := channel.Detach(ctx, runnerID, "stranger"); err != nil {
		t.Fatalf("detach by a stranger: %v", err)
	}

	held, err := channel.Presence(ctx, runnerID)
	if err != nil {
		t.Fatalf("presence: %v", err)
	}

	if !held.Live() {
		t.Fatalf("a connection that never held the channel was able to release it")
	}

	if err := channel.Detach(ctx, runnerID, "first"); err != nil {
		t.Fatalf("detach: %v", err)
	}

	held, err = channel.Presence(ctx, runnerID)
	if err != nil {
		t.Fatalf("presence: %v", err)
	}

	if held.Live() {
		t.Fatalf("the runner still reports a live channel after detaching")
	}
}

func TestAMessageIsOnlyEverSeenOnce(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	spent, err := channel.Seen(ctx, runnerID, "abc")
	if err != nil {
		t.Fatalf("first seen: %v", err)
	}

	if spent {
		t.Fatalf("a message the server had never seen was reported as already handled")
	}

	spent, err = channel.Seen(ctx, runnerID, "abc")
	if err != nil {
		t.Fatalf("second seen: %v", err)
	}

	if !spent {
		t.Fatalf("a redelivered message was not recognised, so it would be handled twice")
	}
}

func TestWhatAMachineReportsAboutItselfSurvivesTheNextHeartbeat(t *testing.T) {
	channel, runnerID := newChannel(t)
	ctx := context.Background()

	now := time.Now().UTC()

	if err := channel.Attach(ctx, runnerID, "epoch", now); err != nil {
		t.Fatalf("attach: %v", err)
	}

	load := entity.RunnerLoad{Capacity: 4, Used: 3, DiskPressure: true}

	if err := channel.Renew(ctx, runnerID, "epoch", load, now); err != nil {
		t.Fatalf("renew: %v", err)
	}

	held, err := channel.Presence(ctx, runnerID)
	if err != nil {
		t.Fatalf("presence: %v", err)
	}

	if held.Load != load {
		t.Fatalf(
			"the machine reported %+v and presence reads %+v. Routing reads this to decide "+
				"where work goes, so a figure that does not survive the write sends work to a "+
				"machine that is already full",
			load, held.Load,
		)
	}

	if held.Free() != 1 || held.Available() {
		t.Fatalf(
			"a machine with one slot left but no disk reads free=%d available=%v; disk "+
				"pressure is exactly the case where a free slot is not an invitation",
			held.Free(), held.Available(),
		)
	}
}
