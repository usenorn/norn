package jobqueue

import "github.com/goforj/wire"

var Set = wire.NewSet(NewClient, AsProducer, AsInspector)
