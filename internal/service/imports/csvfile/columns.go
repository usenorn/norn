package csvfile

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

const (
	targetIgnore      = "ignore"
	targetTitle       = "title"
	targetDescription = "description"
	targetAssignee    = "assignee"
	targetState       = "state"
	targetLabels      = "labels"
	targetPriority    = "priority"
	targetEstimate    = "estimate"
	targetDue         = "due"
	targetParent      = "parent"
	targetExternalID  = "id"
	targetCreated     = "created"

	confidenceCertain   = "certain"
	confidenceLikely    = "likely"
	confidenceAmbiguous = "ambiguous"
	confidenceNone      = "none"

	headerCellMost = 64
)

func targets() []string {
	return []string{
		targetIgnore,
		targetTitle,
		targetDescription,
		targetAssignee,
		targetState,
		targetLabels,
		targetPriority,
		targetEstimate,
		targetDue,
		targetParent,
		targetExternalID,
		targetCreated,
	}
}

func headerTargets() map[string]string {
	return map[string]string{
		"title":       targetTitle,
		"summary":     targetTitle,
		"subject":     targetTitle,
		"description": targetDescription,
		"body":        targetDescription,
		"assignee":    targetAssignee,
		"owner":       targetAssignee,
		"state":       targetState,
		"status":      targetState,
		"labels":      targetLabels,
		"label":       targetLabels,
		"tags":        targetLabels,
		"tag":         targetLabels,
		"priority":    targetPriority,
		"estimate":    targetEstimate,
		"points":      targetEstimate,
		"due":         targetDue,
		"duedate":     targetDue,
		"parent":      targetParent,
		"id":          targetExternalID,
		"key":         targetExternalID,
		"created":     targetCreated,
		"createdat":   targetCreated,
	}
}

type binding map[string]int

func bound(settings Settings, header []string, resource entity.ImportResource) (binding, error) {
	if len(settings.Columns) > 0 {
		return chosen(settings.Columns, resource)
	}

	held := make(binding, len(header))

	for index, name := range header {
		target := proposed(name)

		if target == "" || held.holds(target) {
			continue
		}

		held[target] = index
	}

	return held, nil
}

func chosen(columns []Column, resource entity.ImportResource) (binding, error) {
	held := make(binding, len(columns))

	for _, column := range columns {
		target := strings.ToLower(strings.TrimSpace(column.Target))

		if !slices.Contains(targets(), target) {
			return nil, entity.ImportSourceRefusedError{
				Resource: resource,
				Reason: "this run maps a column onto " + strconv.Quote(column.Target) +
					", which is not something an issue holds; the catalogue lists what a column " +
					"can be read as",
			}
		}

		if target == targetIgnore || column.Index < 0 || held.holds(target) {
			continue
		}

		held[target] = column.Index
	}

	return held, nil
}

func (b binding) holds(target string) bool {
	_, at := b[target]

	return at
}

// cell hands back what the file holds and nothing else. A cell opening with =, +, - or @ is a
// formula to a spreadsheet that later re-exports it, but Norn never writes a CSV back out and
// renders every body through DOMPurify, while "-1" is a perfectly good issue title: rewriting
// the character would corrupt real data to answer a risk this application does not create.
func (b binding) cell(fields []string, target string) string {
	index, at := b[target]
	if !at || index >= len(fields) {
		return ""
	}

	return strings.TrimSpace(fields[index])
}

func headerRow(settings Settings, first []string) bool {
	if settings.Header != nil {
		return *settings.Header
	}

	return looksLikeHeader(first)
}

// looksLikeHeader asks whether the first row names the columns or is one of them. Names are
// short, none of them is a number, and no two of them repeat; a row of data fails at least one
// of those often enough that the guess can be overridden in settings rather than trusted.
func looksLikeHeader(fields []string) bool {
	if len(fields) == 0 {
		return false
	}

	seen := make(map[string]bool, len(fields))

	for _, cell := range fields {
		trimmed := strings.TrimSpace(cell)

		if trimmed == "" || utf8.RuneCountInString(trimmed) > headerCellMost {
			return false
		}

		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return false
		}

		lowered := strings.ToLower(trimmed)

		if seen[lowered] {
			return false
		}

		seen[lowered] = true
	}

	return true
}

func proposed(header string) string {
	return headerTargets()[normalized(header)]
}

func normalized(header string) string {
	return strings.Map(func(symbol rune) rune {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9':
			return symbol
		case symbol >= 'A' && symbol <= 'Z':
			return symbol + ('a' - 'A')
		default:
			return -1
		}
	}, header)
}
