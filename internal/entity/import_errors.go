package entity

import (
	"fmt"
	"time"
)

type ImportRateLimitedError struct {
	Resource   ImportResource
	RetryAfter time.Duration
}

func (e ImportRateLimitedError) Error() string {
	return fmt.Sprintf("import source is rate limiting %s for %s", e.Resource, e.RetryAfter)
}

type ImportSourceUnavailableError struct {
	Resource ImportResource
	Reason   string
}

func (e ImportSourceUnavailableError) Error() string {
	return fmt.Sprintf("import source could not be reached for %s: %s", e.Resource, e.Reason)
}

type ImportSourceRefusedError struct {
	Resource ImportResource
	Reason   string
}

func (e ImportSourceRefusedError) Error() string {
	return fmt.Sprintf("import source refused to hand over %s: %s", e.Resource, e.Reason)
}

// ClampImportBackoff keeps a source's own retry-after inside bounds we choose. A source
// asking to be left alone for four hours is asked again at the ceiling instead, and can
// say so again; a source asking for no wait at all still gets one.
func ClampImportBackoff(requested, floor, ceiling time.Duration) time.Duration {
	if floor <= 0 {
		floor = time.Second
	}

	if ceiling < floor {
		ceiling = floor
	}

	switch {
	case requested < floor:
		return floor
	case requested > ceiling:
		return ceiling
	default:
		return requested
	}
}
