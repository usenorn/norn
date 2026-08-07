package entity

import (
	"errors"
	"fmt"
	"time"
)

type SCMRateLimitedError struct {
	Provider   SCMProvider
	RetryAfter time.Duration
}

func (e SCMRateLimitedError) Error() string {
	return fmt.Sprintf("%s is rate limiting this connection for %s", e.Provider.Label(), e.RetryAfter)
}

// SCMCredentialsRejectedError is the one failure that changes what a connection is rather
// than only what one call did, so it carries the reason a person has to act on. Everything
// else is transient and retried; this stops until somebody supplies a new token.
type SCMCredentialsRejectedError struct {
	Provider SCMProvider
	Reason   string
	Cause    error
}

func (e SCMCredentialsRejectedError) Error() string {
	return fmt.Sprintf("%s rejected this connection's token: %s", e.Provider.Label(), e.Reason)
}

func (e SCMCredentialsRejectedError) Unwrap() error {
	return e.Cause
}

type SCMRepositoryUnreachableError struct {
	Provider   SCMProvider
	Repository string
	Reason     string
	Cause      error
}

func (e SCMRepositoryUnreachableError) Error() string {
	return fmt.Sprintf("%s could not reach %s: %s", e.Provider.Label(), e.Repository, e.Reason)
}

func (e SCMRepositoryUnreachableError) Unwrap() error {
	return e.Cause
}

type SCMUnavailableError struct {
	Provider SCMProvider
	Reason   string
	Cause    error
}

func (e SCMUnavailableError) Error() string {
	return fmt.Sprintf("%s could not be reached: %s", e.Provider.Label(), e.Reason)
}

func (e SCMUnavailableError) Unwrap() error {
	return e.Cause
}

func SCMBrokenBy(err error) (SCMBrokenReason, string, bool) {
	var rejected SCMCredentialsRejectedError
	if errors.As(err, &rejected) {
		return SCMBrokenCredentialsRejected, rejected.Reason, true
	}

	var unreachable SCMRepositoryUnreachableError
	if errors.As(err, &unreachable) {
		return SCMBrokenRepositoryUnreachable, unreachable.Reason, true
	}

	return SCMBrokenNone, "", false
}
