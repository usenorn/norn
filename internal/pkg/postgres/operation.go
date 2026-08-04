package postgres

import (
	"context"

	"github.com/google/uuid"
)

type operationKey struct{}

type operations struct {
	leaders map[uuid.UUID]uuid.UUID
}

func withOperations(ctx context.Context) context.Context {
	return context.WithValue(ctx, operationKey{}, &operations{leaders: map[uuid.UUID]uuid.UUID{}})
}

// Operation returns the identifier every activity row written about subjectID inside the current
// transaction shares, adopting leaderID as that identifier when this is the first such row. Outside
// a transaction each row stands alone, so leaderID is returned unchanged.
func Operation(ctx context.Context, subjectID, leaderID uuid.UUID) uuid.UUID {
	ambient, ok := ctx.Value(operationKey{}).(*operations)
	if !ok {
		return leaderID
	}

	if operation, found := ambient.leaders[subjectID]; found {
		return operation
	}

	ambient.leaders[subjectID] = leaderID

	return leaderID
}
