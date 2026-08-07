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

var issueReferenceScan = regexp.MustCompile(`(?i)\b([a-z]{2,5})-([0-9]{1,9})\b`)

func ScanIssueReferences(text string) []IssueReference {
	if len(text) > CodeLinkScanMaxLen {
		text = text[:CodeLinkScanMaxLen]
	}

	found := make([]IssueReference, 0, CodeLinkMaxReferences)
	seen := make(map[IssueReference]bool, CodeLinkMaxReferences)

	for _, match := range issueReferenceScan.FindAllStringSubmatch(text, -1) {
		number, err := strconv.Atoi(match[2])
		if err != nil || number < 1 {
			continue
		}

		reference := IssueReference{Key: strings.ToUpper(match[1]), Number: number}

		if seen[reference] {
			continue
		}

		seen[reference] = true
		found = append(found, reference)

		if len(found) == CodeLinkMaxReferences {
			break
		}
	}

	return found
}
