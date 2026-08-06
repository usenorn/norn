package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"
)

type txKey struct{}

func (c *Client) Querier(ctx context.Context) boil.ContextExecutor {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}

	return c.DB
}

func (c *Client) WithTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := c.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	deferred := withCallbacks(ctx)

	if err := fn(withOperations(context.WithValue(deferred, txKey{}, tx))); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	runCallbacks(deferred)

	return nil
}

// WithSavepoint isolates one unit of work inside the ambient transaction. Postgres aborts the
// whole transaction on any statement error and refuses every command after it with 25P02, so a
// caller that means to treat a failure as an outcome and carry on has to roll back to a point
// before it; nothing else makes the transaction usable again. Outside a transaction there is
// nothing to protect and the work runs as it stands.
func (c *Client) WithSavepoint(ctx context.Context, fn func(context.Context) error) error {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	if !ok {
		return fn(ctx)
	}

	name := savepointName()

	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		return fmt.Errorf("open savepoint: %w", err)
	}

	if err := fn(ctx); err != nil {
		if _, unwindErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name); unwindErr != nil {
			return errors.Join(err, fmt.Errorf("roll back to savepoint: %w", unwindErr))
		}

		return err
	}

	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+name); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}

	return nil
}

// savepointName is drawn fresh rather than counted because a savepoint name is an identifier
// that cannot be parameterized and because reusing one inside a transaction silently displaces
// the earlier point of the same name, which would let a nested rollback unwind further than its
// caller asked.
func savepointName() string {
	return "norn_sp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
