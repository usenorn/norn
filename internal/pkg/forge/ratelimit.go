package forge

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

const (
	retryAfterHeader = "Retry-After"
	remainingHeaders = "X-RateLimit-Remaining,RateLimit-Remaining"
	resetHeaders     = "X-RateLimit-Reset,RateLimit-Reset"

	// maxWait bounds what a forge can talk this instance into. A reset header read as the
	// wrong unit, or simply wrong, would otherwise park a connection past the heat death of
	// the sun and it would never be tried again.
	maxWait = 24 * time.Hour
)

// exhausted answers the question GitHub makes ambiguous: a 403 is both "you may not" and
// "you have used your hour". Two signals say it is the second, and both must be read —
// a primary limit sends the remaining count at zero, while a secondary limit sends only
// Retry-After. Treating either as a refusal breaks a busy connection every hour.
func exhausted(header http.Header) bool {
	if header.Get(retryAfterHeader) != "" {
		return true
	}

	for _, name := range strings.Split(remainingHeaders, ",") {
		value := header.Get(name)
		if value == "" {
			continue
		}

		if remaining, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && remaining <= 0 {
			return true
		}
	}

	return false
}

func waitFrom(header http.Header, now time.Time) time.Duration {
	if wait, found := retryAfter(header, now); found {
		return bound(wait)
	}

	for _, name := range strings.Split(resetHeaders, ",") {
		value := strings.TrimSpace(header.Get(name))
		if value == "" {
			continue
		}

		epoch, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}

		return bound(time.Unix(epoch, 0).Sub(now))
	}

	return 0
}

func retryAfter(header http.Header, now time.Time) (time.Duration, bool) {
	value := strings.TrimSpace(header.Get(retryAfterHeader))
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}

	if at, err := http.ParseTime(value); err == nil {
		return at.Sub(now), true
	}

	return 0, false
}

func bound(wait time.Duration) time.Duration {
	switch {
	case wait < 0:
		return 0
	case wait > maxWait:
		return maxWait
	default:
		return wait
	}
}

func classify(provider entity.SCMProvider, response *http.Response, body []byte) error {
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		return entity.SCMRateLimitedError{
			Provider:   provider,
			RetryAfter: waitFrom(response.Header, time.Now()),
		}

	case response.StatusCode == http.StatusForbidden && exhausted(response.Header):
		return entity.SCMRateLimitedError{
			Provider:   provider,
			RetryAfter: waitFrom(response.Header, time.Now()),
		}

	case response.StatusCode == http.StatusUnauthorized,
		response.StatusCode == http.StatusForbidden:
		return entity.SCMCredentialsRejectedError{Provider: provider, Reason: excerpt(body)}

	case response.StatusCode >= http.StatusInternalServerError:
		return entity.SCMUnavailableError{
			Provider: provider,
			Reason:   strconv.Itoa(response.StatusCode) + ": " + excerpt(body),
		}

	default:
		return nil
	}
}
