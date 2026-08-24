package channelv1

import (
	"encoding/json"
	"time"
)

const (
	ResumeApproved = "approved"
	ResumeFeedback = "review_feedback"
	ResumeAnswer   = "answer"
)

const (
	QuestionDecision      = "decision"
	QuestionClarification = "clarification"
	QuestionApproval      = "approval"
)

const (
	DeclineAtCapacity   = "at_capacity"
	DeclineDiskPressure = "disk_pressure"
	DeclinePaused       = "paused"
)

const (
	ProfileStrict       = "strict"
	ProfileStandard     = "standard"
	ProfileUnrestricted = "unrestricted"
)

const (
	BaseRefOriginDefault = "origin/default"
	BaseRefHead          = "head"
)

type Params struct {
	Tool         string `json:"tool,omitempty"`
	Model        string `json:"model,omitempty"`
	Runtime      string `json:"runtime,omitempty"`
	BaseRef      string `json:"base_ref,omitempty"`
	IncludeDirty bool   `json:"include_dirty,omitempty"`
	Profile      string `json:"profile,omitempty"`
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
	Branch      string `json:"branch,omitempty"`
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
	QuestionID  string `json:"question_id,omitempty"`
	QuestionRef string `json:"question_ref,omitempty"`
}

type QuestionContext struct {
	Preview   string   `json:"preview,omitempty"`
	Files     []string `json:"files,omitempty"`
	Artifacts []string `json:"artifact_refs,omitempty"`
}

type Question struct {
	Ref           string          `json:"ref"`
	Kind          string          `json:"kind"`
	Blocking      bool            `json:"blocking"`
	Message       string          `json:"message"`
	Options       []string        `json:"options,omitempty"`
	AllowFreeText bool            `json:"allow_free_text"`
	Default       string          `json:"default,omitempty"`
	Wait          int             `json:"wait_seconds,omitempty"`
	Context       QuestionContext `json:"context,omitempty"`
	Asked         time.Time       `json:"ts"`
}

type Answer struct {
	QuestionID string    `json:"question_id"`
	Ref        string    `json:"ref"`
	Answer     string    `json:"answer"`
	AnsweredBy string    `json:"answered_by"`
	AnsweredAt time.Time `json:"ts"`
}

const (
	PreviewOpen   = "open"
	PreviewClosed = "closed"
)

type Preview struct {
	Name     string    `json:"name"`
	Service  string    `json:"service"`
	Path     string    `json:"path,omitempty"`
	Port     int       `json:"port,omitempty"`
	State    string    `json:"state"`
	Occurred time.Time `json:"ts"`
}

const (
	ServiceStarting  = "starting"
	ServiceHealthy   = "healthy"
	ServiceUnhealthy = "unhealthy"
	ServiceStopped   = "stopped"
)

const (
	ProbeNone = ""
	ProbeHTTP = "http"
	ProbeTCP  = "tcp"
	ProbeLog  = "log"
)

type Service struct {
	Name     string    `json:"name"`
	State    string    `json:"state"`
	Probe    string    `json:"probe,omitempty"`
	Port     int       `json:"port,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Occurred time.Time `json:"ts"`
}

type Retention struct {
	KeepUntil time.Time `json:"keep_until"`
}

type Leased struct {
	Executions []string `json:"executions"`
}

type Decline struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
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

const (
	ValidationPassed  = "passed"
	ValidationFailed  = "failed"
	ValidationSkipped = "skipped"
)

type RepoChange struct {
	Repository  string `json:"repo"`
	Branch      string `json:"branch,omitempty"`
	BaseSHA     string `json:"base_sha,omitempty"`
	HeadSHA     string `json:"head_sha,omitempty"`
	Commits     int    `json:"commits,omitempty"`
	Additions   int    `json:"additions,omitempty"`
	Deletions   int    `json:"deletions,omitempty"`
	Files       int    `json:"files_changed,omitempty"`
	Diff        string `json:"diff_artifact_id,omitempty"`
	PullRequest string `json:"pull_request_url,omitempty"`
}

type Validation struct {
	Check    string `json:"check"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Artifact string `json:"artifact_id,omitempty"`
}

type ChangeSet struct {
	Repos      []RepoChange `json:"repos,omitempty"`
	Validation []Validation `json:"validation,omitempty"`
}

type Result struct {
	Summary   string    `json:"summary,omitempty"`
	ChangeSet ChangeSet `json:"changeset"`
	Reported  time.Time `json:"ts"`
}
