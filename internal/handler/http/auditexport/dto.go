package auditexport

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type record struct {
	ID            string            `json:"id"`
	OccurredAt    time.Time         `json:"occurredAt"`
	WorkspaceID   string            `json:"workspaceId,omitempty"`
	WorkspaceName string            `json:"workspaceName,omitempty"`
	Action        string            `json:"action"`
	Outcome       string            `json:"outcome"`
	AuthMethod    string            `json:"authMethod"`
	ActorKind     string            `json:"actorKind"`
	ActorID       string            `json:"actorId,omitempty"`
	ActorName     string            `json:"actorName,omitempty"`
	TokenName     string            `json:"tokenName,omitempty"`
	AgentName     string            `json:"agentName,omitempty"`
	SourceIP      string            `json:"sourceIp,omitempty"`
	UserAgent     string            `json:"userAgent,omitempty"`
	ResourceKind  string            `json:"resourceKind,omitempty"`
	ResourceID    string            `json:"resourceId,omitempty"`
	ResourceName  string            `json:"resourceName,omitempty"`
	Detail        map[string]string `json:"detail,omitempty"`
}

func exported(entry entity.AuditEntry) record {
	out := record{
		ID:            entry.ID.String(),
		OccurredAt:    entry.OccurredAt.UTC(),
		WorkspaceName: entry.WorkspaceName,
		Action:        string(entry.Action),
		Outcome:       string(entry.Outcome),
		AuthMethod:    string(entry.Actor.How()),
		ActorKind:     string(entry.Actor.Kind),
		ActorName:     entry.Actor.Name,
		TokenName:     entry.Actor.TokenName,
		AgentName:     entry.Actor.AgentName,
		UserAgent:     entry.UserAgent,
		ResourceKind:  entry.ResourceKind,
		ResourceName:  entry.ResourceName,
		Detail:        entry.Detail,
	}

	if entry.WorkspaceID != uuid.Nil {
		out.WorkspaceID = entry.WorkspaceID.String()
	}

	if entry.Actor.AccountID != uuid.Nil {
		out.ActorID = entry.Actor.AccountID.String()
	}

	if entry.ResourceID != uuid.Nil {
		out.ResourceID = entry.ResourceID.String()
	}

	if entry.SourceIP.IsValid() {
		out.SourceIP = entry.SourceIP.String()
	}

	return out
}
