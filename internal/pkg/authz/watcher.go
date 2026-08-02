package authz

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/usenorn/norn/internal/pkg/valkey"
)

type watcher struct {
	client   *valkey.Client
	channel  string
	localID  string
	callback atomic.Pointer[func(string)]
	pubsub   *redis.PubSub
	done     chan struct{}
}

func newWatcher(client *valkey.Client, channel string) (*watcher, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())

	pubsub := client.Subscribe(ctx, channel)

	if _, err := pubsub.Receive(ctx); err != nil {
		cancel()
		_ = pubsub.Close()

		return nil, nil, fmt.Errorf("subscribe to casbin policy channel: %w", err)
	}

	w := &watcher{
		client:  client,
		channel: channel,
		localID: uuid.NewString(),
		pubsub:  pubsub,
		done:    make(chan struct{}),
	}

	go w.consume(ctx)

	cleanup := func() {
		cancel()
		_ = pubsub.Close()
		<-w.done
	}

	return w, cleanup, nil
}

func (w *watcher) consume(ctx context.Context) {
	defer close(w.done)

	messages := w.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}

			if message.Payload == w.localID {
				continue
			}

			if callback := w.callback.Load(); callback != nil {
				(*callback)(message.Payload)
			}
		}
	}
}

func (w *watcher) SetUpdateCallback(callback func(string)) error {
	w.callback.Store(&callback)

	return nil
}

func (w *watcher) Update() error {
	if err := w.client.Publish(context.Background(), w.channel, w.localID).Err(); err != nil {
		return fmt.Errorf("publish casbin policy update: %w", err)
	}

	return nil
}

func (w *watcher) Close() {}
