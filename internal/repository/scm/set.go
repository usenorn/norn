package scm

import "github.com/goforj/wire"

var Set = wire.NewSet(
	NewSCMConnection,
	NewSCMDelivery,
	NewCodeLink,
	NewIssueMirror,
	NewSCMTransitionRule,
	NewSCMRepository,
	NewSCMRoute,
)
