package entity

import "strings"

// MapLabels pairs what a forge calls a label with what this workspace calls one. Matching is
// by name, case-insensitively, because that is the only thing the two systems share; a
// platform label nobody here has is reported rather than created, since inventing labels in
// somebody's workspace from an outside list is not a decision this integration gets to make.
func MapLabels(remote []string, available []Label) ([]Label, []string) {
	byName := make(map[string]Label, len(available))
	for _, label := range available {
		byName[strings.ToLower(strings.TrimSpace(label.Name))] = label
	}

	matched := make([]Label, 0, len(remote))
	unmapped := make([]string, 0)
	seen := make(map[string]bool, len(remote))

	for _, name := range remote {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}

		seen[key] = true

		if label, found := byName[key]; found {
			matched = append(matched, label)

			continue
		}

		unmapped = append(unmapped, strings.TrimSpace(name))
	}

	return matched, unmapped
}

// LabelNames is what goes back the other way. A Norn label the forge does not have is
// created there by the forge itself when it is applied, which is the one direction where
// creating is the platform's own behaviour rather than a decision Norn makes.
func LabelNames(labels []Label) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if trimmed := strings.TrimSpace(label.Name); trimmed != "" {
			names = append(names, trimmed)
		}
	}

	return names
}
