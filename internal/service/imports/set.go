package imports

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewImports, NewImportRunner, NewImportRescue, NewSourceRegistry)
