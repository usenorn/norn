package config

import "github.com/goforj/wire"

var Set = wire.NewSet(
	New,
	NewApp,
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
