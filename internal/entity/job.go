package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TaskTypeSignInCode              = "session:sign_in_code"
	TaskTypeSignUpVerification      = "account:sign_up_verification"
	TaskTypeEmailChangeConfirmation = "account:email_change_confirmation"
	TaskTypePasswordReset           = "account:password_reset"
	TaskTypePasswordResetSSONotice  = "account:password_reset_sso_notice"
	TaskTypeInvitation              = "workspace:invitation"
	TaskTypeWorkspacePurge          = "workspace:purge"
	TaskTypeIssuePurge              = "issue:purge"
	TaskTypeSSOCertificateSweep     = "sso:certificate_sweep"
	TaskTypeBulkApply               = "issue:bulk_apply"
	TaskTypeCycleGeneration         = "cycle:generate"
	TaskTypeAttachmentReclaim       = "attachment:reclaim"
	TaskTypeNotificationFanOut      = "notification:fanout"
	TaskTypeNotificationDigest      = "notification:digest"
	TaskTypeAPITokenExpirySweep     = "api_token:expiry_sweep"
	TaskTypeAuditSweep              = "audit:retention_sweep"
	TaskTypeWebhookFanOut           = "webhook:fan_out"
	TaskTypeWebhookDeliver          = "webhook:deliver"
	TaskTypeWebhookSweep            = "webhook:retention_sweep"
	TaskTypeImportStage             = "import:stage"
	TaskTypeImportExecute           = "import:execute"
	TaskTypeImportRevert            = "import:revert"
	TaskTypeImportRescue            = "import:rescue"
	TaskTypeSCMDelivery             = "scm:delivery"
	TaskTypeSCMReconcile            = "scm:reconcile"
	TaskTypeSCMBackfill             = "scm:backfill"
	TaskTypeSCMResume               = "scm:resume"
	TaskTypeExecutionLeaseSweep     = "execution:lease_sweep"

	AttachmentReclaimTaskID = "attachment-reclaim"
	WebhookFanOutTaskID     = "webhook-fan-out"
	ImportRescueTaskID      = "import-rescue"
	SCMReconcileTaskID      = "scm-reconcile"

	WorkerHealthReadHeaderTimeout = 5 * time.Second

	QueueDefault = "default"
	QueueMail    = "mail"
	QueueWebhook = "webhook"
	QueueImport  = "import"
	QueueSCM     = "scm"
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

type SignInCodePayload struct {
	ChallengeID string
	Code        string
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
	IssueID     uuid.UUID
	WorkspaceID uuid.UUID
}

type BulkApplyPayload struct {
	BulkActionID uuid.UUID
	WorkspaceID  uuid.UUID
	IssueIDs     []uuid.UUID
	Filter       *BulkFilter
}

type WebhookDeliverPayload struct {
	DeliveryID uuid.UUID
	Attempt    int
}

type SCMResumePayload struct {
	WorkspaceID uuid.UUID
	IssueID     uuid.UUID
}

type SCMDeliveryPayload struct {
	DeliveryID uuid.UUID
	Attempt    int
}

type SCMBackfillPayload struct {
	RepositoryID uuid.UUID
}

type ImportStagePayload struct {
	ImportRunID uuid.UUID
	WorkspaceID uuid.UUID
	Attempt     int
}

type ImportExecutePayload struct {
	ImportRunID uuid.UUID
	WorkspaceID uuid.UUID
	Attempt     int
}

type ImportRevertPayload struct {
	ImportRunID uuid.UUID
	WorkspaceID uuid.UUID
	Attempt     int
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
