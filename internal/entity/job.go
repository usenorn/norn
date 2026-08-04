package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TaskTypeSignUpVerification      = "account:sign_up_verification"
	TaskTypeEmailChangeConfirmation = "account:email_change_confirmation"
	TaskTypePasswordReset           = "account:password_reset"
	TaskTypePasswordResetSSONotice  = "account:password_reset_sso_notice"
	TaskTypeInvitation              = "workspace:invitation"
	TaskTypeWorkspacePurge          = "workspace:purge"
	TaskTypeIssuePurge              = "issue:purge"

	QueueDefault = "default"
	QueueMail    = "mail"
)

var ErrTaskNotFound = errors.New("task not found")

type SignUpVerificationPayload struct {
	SignUpID uuid.UUID
	Token    string
}

type EmailChangeConfirmationPayload struct {
	EmailChangeID uuid.UUID
	Token         string
}

type PasswordResetPayload struct {
	PasswordResetID uuid.UUID
	Token           string
}

type PasswordResetSSONoticePayload struct {
	AccountID uuid.UUID
}

type InvitationPayload struct {
	InvitationID uuid.UUID
	Token        string
}

type WorkspacePurgePayload struct {
	WorkspaceID uuid.UUID
}

type IssuePurgePayload struct {
	IssueID uuid.UUID
}

type TaskState string

const (
	TaskStateActive    TaskState = "active"
	TaskStatePending   TaskState = "pending"
	TaskStateScheduled TaskState = "scheduled"
	TaskStateRetry     TaskState = "retry"
	TaskStateArchived  TaskState = "archived"
	TaskStateCompleted TaskState = "completed"
)

func (s TaskState) Valid() bool {
	switch s {
	case TaskStateActive, TaskStatePending, TaskStateScheduled, TaskStateRetry, TaskStateArchived, TaskStateCompleted:
		return true
	default:
		return false
	}
}

type QueueStat struct {
	Name      string
	Size      int
	Active    int
	Pending   int
	Scheduled int
	Retry     int
	Archived  int
	Completed int
}

type TaskSummary struct {
	ID        string
	Queue     string
	Type      string
	State     TaskState
	Retried   int
	MaxRetry  int
	LastError string
	NextRetry time.Time
}
