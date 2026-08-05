package postgres

import (
	"context"
	"sync"
)

type commitKey struct{}

type callbacks struct {
	mu      sync.Mutex
	pending []func(context.Context)
}

func withCallbacks(ctx context.Context) context.Context {
	return context.WithValue(ctx, commitKey{}, &callbacks{})
}

func AfterCommit(ctx context.Context, fn func(context.Context)) {
	ambient, ok := ctx.Value(commitKey{}).(*callbacks)
	if !ok {
		fn(ctx)

		return
	}

	ambient.mu.Lock()
	defer ambient.mu.Unlock()

	ambient.pending = append(ambient.pending, fn)
}

func runCallbacks(ctx context.Context) {
	ambient, ok := ctx.Value(commitKey{}).(*callbacks)
	if !ok {
		return
	}

	for {
		ambient.mu.Lock()
		pending := ambient.pending
		ambient.pending = nil
		ambient.mu.Unlock()

		if len(pending) == 0 {
			return
		}

		for _, fn := range pending {
			fn(ctx)
		}
	}
}
