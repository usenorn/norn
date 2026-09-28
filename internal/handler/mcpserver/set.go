package mcpserver

import (
	"github.com/goforj/wire"

	"github.com/usenorn/norn/internal/service"
)

var Set = wire.NewSet(NewTools, New, wire.Bind(new(service.NornTools), new(*Tools)))
