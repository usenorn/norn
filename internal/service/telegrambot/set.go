package telegrambot

import "github.com/goforj/wire"

var Set = wire.NewSet(NewBots, NewUpdates)
