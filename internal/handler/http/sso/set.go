package sso

import "github.com/goforj/wire"

var Set = wire.NewSet(NewCallback, NewSAML)
