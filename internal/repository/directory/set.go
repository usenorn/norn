package directory

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewSync)
