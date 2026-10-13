package roadmap

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewPublic)
