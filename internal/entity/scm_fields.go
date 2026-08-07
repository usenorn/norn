package entity

import "strings"

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

func LabelNames(labels []Label) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if trimmed := strings.TrimSpace(label.Name); trimmed != "" {
			names = append(names, trimmed)
		}
	}

	return names
}
