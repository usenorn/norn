package eventstream

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewReadClient)
