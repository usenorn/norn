package job

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewSignUpVerificationHandler,
	NewEmailChangeConfirmationHandler,
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
)
