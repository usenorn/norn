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

var issueReferenceScan = regexp.MustCompile(
	`(?i)(?:\b(close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+)?\b([a-z]{2,5})-([0-9]{1,9})\b`,
)

type ScannedReference struct {
	Reference IssueReference
	Resolving bool
}

func ScanChangeReferences(branch, title, body string) []ScannedReference {
	found := ScanIssueReferences(branch)

	at := make(map[IssueReference]int, len(found))

	for index, scanned := range found {
		found[index].Resolving = true
		at[scanned.Reference] = index
	}

	for _, scanned := range ScanIssueReferences(strings.Join([]string{title, body}, "\n")) {
		if index, seen := at[scanned.Reference]; seen {
			found[index].Resolving = found[index].Resolving || scanned.Resolving

			continue
		}

		if len(found) == CodeLinkMaxReferences {
			break
		}

		at[scanned.Reference] = len(found)
		found = append(found, scanned)
	}

	return found
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
