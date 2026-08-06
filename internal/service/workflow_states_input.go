package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CreateWorkflowStateInput struct {
	WorkspaceID uuid.UUID
	TeamID      uuid.UUID
	Name        string
	Category    entity.StateCategory
	Origin      *entity.ImportOrigin
}

type UpdateWorkflowStateInput struct {
	Name     *string
	Category *entity.StateCategory
}
