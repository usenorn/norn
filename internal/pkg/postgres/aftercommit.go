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

// AfterCommit defers fn until the transaction the context carries has committed, so nothing
// observes a change that then rolls back. WithTx is re-entrant and a joining caller cannot tell
// whether it owns the transaction, which is why registration has to be ambient rather than a
// return value. Outside a transaction there is nothing to wait for and fn runs at once.
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
