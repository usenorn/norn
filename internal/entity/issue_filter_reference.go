package entity

import "slices"

type IssueFilterReferenceState string

const (
	IssueFilterReferenceResolved   IssueFilterReferenceState = "resolved"
	IssueFilterReferenceRestricted IssueFilterReferenceState = "restricted"
	IssueFilterReferenceMissing    IssueFilterReferenceState = "missing"
)

type IssueFilterReference struct {
	Field IssueFilterField
	Value string
	State IssueFilterReferenceState
	Name  string
}

func IssueFilterReferences(filter IssueFilter) []IssueFilterReference {
	references := make([]IssueFilterReference, 0)
	seen := make(map[IssueFilterReference]bool)

	collectReferences(filter, &references, seen)

	return references
}

func collectReferences(
	filter IssueFilter,
	references *[]IssueFilterReference,
	seen map[IssueFilterReference]bool,
) {
	if filter.Not != nil {
		collectReferences(*filter.Not, references, seen)
	}

	for _, branch := range slices.Concat(filter.All, filter.Any) {
		collectReferences(branch, references, seen)
	}

	if !filter.Leaf() || !filter.Field.Names() {
		return
	}

	for _, value := range filter.Values {
		reference := IssueFilterReference{
			Field: filter.Field,
			Value: value,
			State: IssueFilterReferenceMissing,
		}

		if seen[reference] {
			continue
		}

		seen[reference] = true
		*references = append(*references, reference)
	}
}

func (f IssueFilterField) Names() bool {
	kind, known := f.Kind()

	return known && (kind == issueFilterKindID || kind == issueFilterKindLabelSet)
}
