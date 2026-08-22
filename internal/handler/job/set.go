package job

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewSignUpVerificationHandler,
	NewEmailChangeConfirmationHandler,
	NewSignInCodeHandler,
	NewPasswordResetHandler,
	NewPasswordResetSSONoticeHandler,
	NewInvitationHandler,
	NewWorkspacePurgeHandler,
	NewIssuePurgeHandler,
	NewBulkApplyHandler,
	NewSSOCertificateSweepHandler,
	NewCycleGenerationHandler,
	NewAttachmentReclaimHandler,
	NewNotificationFanOutHandler,
	NewNotificationDigestHandler,
	NewAPITokenExpirySweepHandler,
	NewAuditSweepHandler,
	NewWebhookFanOutHandler,
	NewWebhookDeliverHandler,
	NewWebhookSweepHandler,
	NewImportStageHandler,
	NewImportExecuteHandler,
	NewImportRevertHandler,
	NewImportRescueHandler,
	NewSCMDeliveryHandler,
	NewSCMReconcileHandler,
	NewSCMBackfillHandler,
	NewSCMResumeHandler,
	NewExecutionLeaseSweepHandler,
)
