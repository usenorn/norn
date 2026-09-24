package agentcapability

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewToolkits)
