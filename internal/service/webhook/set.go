package webhook

import "github.com/goforj/wire"

var Set = wire.NewSet(NewEmitter, NewRegistry, NewLog, NewFanOut, NewDeliverer, NewSweeper)
