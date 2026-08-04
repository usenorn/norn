package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateLabelInput struct {
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	GroupID     uuid.UUID
	Name        string
	Color       entity.LabelColor
}

type UpdateLabelInput struct {
	Name    *string
	Color   *entity.LabelColor
	GroupID *uuid.UUID
}
