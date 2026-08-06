package webhook

import "github.com/goforj/wire"

var Set = wire.NewSet(New, NewOutbox, NewDeliveries, NewRetention)
