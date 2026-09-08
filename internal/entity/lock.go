package entity

import (
	"bytes"
	"slices"

	"github.com/google/uuid"
)

func LockOrder(ids []uuid.UUID) []uuid.UUID {
	ordered := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		if id == uuid.Nil || slices.Contains(ordered, id) {
			continue
		}

		ordered = append(ordered, id)
	}

	slices.SortFunc(ordered, func(first, second uuid.UUID) int {
		return bytes.Compare(first[:], second[:])
	})

	return ordered
}
