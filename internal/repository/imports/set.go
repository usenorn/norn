package imports

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewImportRun,
	NewImportCursor,
	NewImportRecord,
	NewImportMapping,
	NewImportLedger,
	NewImportReport,
)
