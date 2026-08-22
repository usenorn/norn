package channelv1

import (
	"encoding/json"
	"time"
)

const (
	ResumeApproved = "approved"
	ResumeFeedback = "review_feedback"
)

const (
	DeclineAtCapacity   = "at_capacity"
	DeclineDiskPressure = "disk_pressure"
	DeclinePaused       = "paused"
)

type Params struct {
	Tool    string `json:"tool,omitempty"`
	Model   string `json:"model,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

type Issue struct {
	ID          string `json:"id"`
	Reference   string `json:"reference"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Brief       string `json:"brief"`
}

type Offer struct {
	ExecutionID string `json:"execution_id"`
	Reference   string `json:"reference"`
	Attempt     int    `json:"attempt"`
	WorkspaceID string `json:"workspace_id"`
	Issue       Issue  `json:"issue"`
	Params      Params `json:"params"`
}

type Start struct {
	ExecutionID    string     `json:"execution_id"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at"`
	Params         Params     `json:"params"`
}

type Cancellation struct {
	Reason string `json:"reason"`
}

type Instruction struct {
	Reason      string `json:"reason"`
	Instruction string `json:"instruction,omitempty"`
}

type Leased struct {
	Executions []string `json:"executions"`
}

type Decline struct {
	Reason string `json:"reason"`
}

type Report struct {
	State    string    `json:"state"`
	Reason   string    `json:"reason"`
	Detail   string    `json:"detail"`
	Occurred time.Time `json:"ts"`
}

type Entry struct {
	Kind     string          `json:"kind"`
	Reason   string          `json:"reason"`
	Detail   json.RawMessage `json:"detail"`
	Occurred time.Time       `json:"ts"`
}

type Phase struct {
	ExecutionID string `json:"execution_id"`
	State       string `json:"state"`
}

type Hello struct {
	Version      string   `json:"version"`
	Protocol     int      `json:"protocol"`
	Capabilities []string `json:"capabilities,omitempty"`
	Capacity     int      `json:"capacity"`
	Executions   []string `json:"executions,omitempty"`
}

type Pulse struct {
	Capacity     int     `json:"capacity"`
	Used         int     `json:"used"`
	Paused       bool    `json:"paused"`
	DiskPressure bool    `json:"disk_pressure"`
	CPUPressure  bool    `json:"cpu_pressure"`
	Phases       []Phase `json:"phases,omitempty"`
}

type Configuration struct {
	Capacity *int `json:"capacity,omitempty"`
}
