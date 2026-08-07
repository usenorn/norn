package scm

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewForges,
	NewConnections,
	NewSync,
)
