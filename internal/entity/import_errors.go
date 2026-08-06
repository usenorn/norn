package entity

import (
	"fmt"
	"strings"
	"time"
)

type ImportRateLimitedError struct {
	Resource   ImportResource
	RetryAfter time.Duration
}

func (e ImportRateLimitedError) Error() string {
	return fmt.Sprintf("import source is rate limiting %s for %s", e.Resource, e.RetryAfter)
}

// ImportWouldTriageError names the teams whose triage would swallow what an import creates.
// The requester has to acknowledge that before the run may proceed, and cannot decide
// without knowing which teams are involved, so the refusal carries them rather than only
// saying that some exist.
type ImportWouldTriageError struct {
	Teams []string
}

func (e ImportWouldTriageError) Error() string {
	return fmt.Sprintf("%s: %s", ErrImportWouldTriage, strings.Join(e.Teams, ", "))
}

func (e ImportWouldTriageError) Unwrap() error {
	return ErrImportWouldTriage
}

// ImportUnknownReferenceError is one sentence shared by two hand-maintained trees: the preview
// says it about a row it has not touched, and the apply pass returns it as the failure of the
// same row, so the page a requester approves and the report a run records read alike. What it
// costs — a dropped reference, a skipped row, or a failed one — is the run's own
// unknown-reference policy rather than anything this carries.
type ImportUnknownReferenceError struct {
	Kind      string
	Reference string
}

func (e ImportUnknownReferenceError) Error() string {
	return fmt.Sprintf("the %s %s this row names did not arrive with the import", e.Kind, e.Reference)
}

// Cause on these two carries whatever the adapter was holding when it classified the
// failure. The classification is what the framework acts on — park, retry, or stop — but
// an adapter's own sentinel is what tells a person which knob to turn, and without an
// Unwrap it would be readable only as prose inside Reason.
type ImportSourceUnavailableError struct {
	Resource ImportResource
	Reason   string
	Cause    error
}

func (e ImportSourceUnavailableError) Error() string {
	return fmt.Sprintf("import source could not be reached for %s: %s", e.Resource, e.Reason)
}

func (e ImportSourceUnavailableError) Unwrap() error {
	return e.Cause
}

type ImportSourceRefusedError struct {
	Resource ImportResource
	Reason   string
	Cause    error
}

func (e ImportSourceRefusedError) Error() string {
	return fmt.Sprintf("import source refused to hand over %s: %s", e.Resource, e.Reason)
}

func (e ImportSourceRefusedError) Unwrap() error {
	return e.Cause
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
