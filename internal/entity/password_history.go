package entity

import (
	"time"

	"github.com/google/uuid"
)

const PasswordHistoryDepth = 5

type PasswordHistoryEntry struct {
	ID           uuid.UUID
	AccountID    uuid.UUID
	PasswordHash string
	CreatedAt    time.Time
}
