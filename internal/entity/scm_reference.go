package entity

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	CodeLinkMaxReferences = 10
	CodeLinkScanMaxLen    = 8192
)

// issueReferenceScan optionally captures the verb in front of a reference. A bare mention
// links the change to the issue and nothing more; only `fixes`, `closes` or `resolves` says
// the change is meant to settle it, and only that drives the issue's state.
var issueReferenceScan = regexp.MustCompile(
	`(?i)(?:\b(close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+)?\b([a-z]{2,5})-([0-9]{1,9})\b`,
)

// ScannedReference is a reference and what the text asked for. The two travel together
// because the same scan produces both and separating them loses which mention was which.
type ScannedReference struct {
	Reference IssueReference
	Resolving bool
}

func ScanIssueReferences(text string) []ScannedReference {
	if len(text) > CodeLinkScanMaxLen {
		text = text[:CodeLinkScanMaxLen]
	}

	found := make([]ScannedReference, 0, CodeLinkMaxReferences)
	at := make(map[IssueReference]int, CodeLinkMaxReferences)

	for _, match := range issueReferenceScan.FindAllStringSubmatch(text, -1) {
		number, err := strconv.Atoi(match[3])
		if err != nil || number < 1 {
			continue
		}

		reference := IssueReference{Key: strings.ToUpper(match[2]), Number: number}
		resolving := match[1] != ""

		// The same issue named twice is one link. Naming it once plainly and once with a verb
		// means the change settles it, so the verb wins wherever it appears.
		if index, seen := at[reference]; seen {
			if resolving {
				found[index].Resolving = true
			}

			continue
		}

		at[reference] = len(found)
		found = append(found, ScannedReference{Reference: reference, Resolving: resolving})

		if len(found) == CodeLinkMaxReferences {
			break
		}
	}

	return found
}
