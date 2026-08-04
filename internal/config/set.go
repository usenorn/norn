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
	NewHTTP,
	NewPostgres,
	NewValkey,
	NewAsynq,
	NewSMTP,
	NewStorage,
	NewSession,
	NewCasbin,
	NewGeoIP,
	NewPassword,
	NewWorkspace,
)
