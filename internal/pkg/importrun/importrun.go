package importrun

import (
	"context"

	"github.com/google/uuid"
)

type runKey struct{}

type Run struct {
	WorkspaceID uuid.UUID
	RunID       uuid.UUID
}

// With names the run a staging pass belongs to. An import source is handed a resource, a
// cursor and its own configuration and nothing else, deliberately: the payloads it renders
// carry source identifiers only, so an adapter has no business holding a Norn identifier.
// An adapter that pulls a file's bytes while the source's signed URL is still alive is the
// exception, because the object it writes is addressed by workspace and run — those two
// opaque values and no others — and the job that starts the pass is the last place that
// knows them.
func With(ctx context.Context, run Run) context.Context {
	return context.WithValue(ctx, runKey{}, run)
}

func From(ctx context.Context) (Run, bool) {
	run, held := ctx.Value(runKey{}).(Run)

	return run, held && run.WorkspaceID != uuid.Nil && run.RunID != uuid.Nil
}
