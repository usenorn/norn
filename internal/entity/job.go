package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TaskTypeEmailChangeConfirmation = "account:email_change_confirmation"
	TaskTypePasswordReset           = "account:password_reset"
	TaskTypePasswordResetSSONotice  = "account:password_reset_sso_notice"

	QueueDefault = "default"
	QueueMail    = "mail"
)

var ErrTaskNotFound = errors.New("task not found")

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
