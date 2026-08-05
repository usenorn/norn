package entity

import "time"

var (
	SAMLExpiryNoticeDays     = []int{30, 14, 7, 1}
	APITokenExpiryNoticeDays = []int{30, 14, 7, 1}
)

func DaysUntil(expiry, now time.Time) int {
	return int(expiry.Sub(now).Round(time.Hour) / (24 * time.Hour))
}

// ExpiryNoticeDue reports the tightest notice threshold that has been crossed and not yet warned
// about. Recording the returned threshold is what stops a daily sweep mailing the same warning
// every morning: notified only ever ratchets tighter, so each band warns once and never regresses.
func ExpiryNoticeDue(thresholds []int, daysLeft int, notified *int) (int, bool) {
	threshold, crossed := 0, false

	for _, candidate := range thresholds {
		if daysLeft <= candidate && (!crossed || candidate < threshold) {
			threshold, crossed = candidate, true
		}
	}

	if !crossed {
		return 0, false
	}

	if notified != nil && *notified <= threshold {
		return 0, false
	}

	return threshold, true
}
