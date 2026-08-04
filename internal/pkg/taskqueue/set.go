package taskqueue

import "github.com/goforj/wire"

var Set = wire.NewSet(NewClient, NewInspector, NewServer, NewScheduler)
