package job

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewEmailChangeConfirmationHandler,
	NewPasswordResetHandler,
	NewPasswordResetSSONoticeHandler,
)
