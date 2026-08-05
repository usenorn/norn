package audit

import "github.com/goforj/wire"

var Set = wire.NewSet(NewRecorder, NewReader, NewSweeper)
