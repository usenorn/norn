package config

import "github.com/goforj/wire"

var Set = wire.NewSet(
	New,
	NewApp,
	NewSecurity,
	NewOIDC,
	NewSAML,
	NewCycles,
	NewInstance,
	NewLicence,
	NewAudit,
	NewHTTP,
	NewPostgres,
	NewValkey,
	NewAsynq,
	NewSMTP,
	NewStorage,
	NewAttachments,
	NewNotifications,
	NewRealtime,
	NewAPITokens,
	NewMCP,
	NewSession,
	NewCasbin,
	NewGeoIP,
	NewPassword,
	NewWorkspace,
)
