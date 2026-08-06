package linear

import (
	"context"
	"sort"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const (
	linearBlocks    = "blocks"
	linearDuplicate = "duplicate"
	linearRelated   = "related"
)

// keptRelations ranks what Linear can say about a pair against what a workspace here can
// hold. Norn allows one relation per pair in either direction and Linear allows several, so
// the choice has to be made somewhere: made here, the preview counts exactly what apply will
// create. Made nowhere, apply creates the first to arrive and refuses the rest, which leaves
// the graph both arbitrary and quietly incomplete.
func keptRelations() map[string]int {
	return map[string]int{
		linearDuplicate: 3,
		linearBlocks:    2,
		linearRelated:   1,
	}
}

func relationKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case linearBlocks:
		return string(entity.IssueRelationBlocks)
	case linearDuplicate:
		return string(entity.IssueRelationDuplicates)
	case linearRelated:
		return string(entity.IssueRelationRelatesTo)
	default:
		return ""
	}
}

type pairedRelation struct {
	node relationNode
	rank int
}

func (s *Source) fetchIssueRelations(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	reply, err := query[issuesReply](ctx, s, request, settings, issueRelationsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	held := make(map[string]relationNode)

	for _, node := range reply.Issues.Nodes {
		warnTruncated(ctx, "relations", node.ID, node.Relations)
		warnTruncated(ctx, "relations", node.ID, node.Inverse)

		for _, relation := range append(append([]relationNode{}, node.Relations.Nodes...), node.Inverse.Nodes...) {
			held[relation.ID] = relation
		}
	}

	kept, dropped := chosen(held)

	for _, lost := range dropped {
		logging.From(ctx).InfoContext(ctx, "a linear link between two issues was not carried",
			"external_id", lost.ID,
			"issue", lost.Issue.key(),
			"related", lost.RelatedIssue.key(),
			"kind", lost.Type,
		)
	}

	records := make([]entity.ImportRecord, 0, len(kept))

	for _, relation := range kept {
		record, err := recordOf(
			relation.ID, "", relation.CreatedAt, relation.UpdatedAt,
			service.ImportIssueRelationPayload{
				Issue:   relation.Issue.key(),
				Related: relation.RelatedIssue.key(),
				Kind:    relationKind(relation.Type),
			},
		)
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, record)
	}

	return pageOf(records, reply.Issues.PageInfo), nil
}

// chosen keeps one relation per pair and names the rest. The winner is picked on rank and
// then on the source's own identifier rather than on the order a page happened to arrive in,
// because the two issues of a pair can land on different pages: an arbitrary winner would
// make the same pair resolve two different ways and stage two rows where a workspace holds
// one.
func chosen(held map[string]relationNode) ([]relationNode, []relationNode) {
	ranks := keptRelations()

	byPair := make(map[string][]pairedRelation)
	dropped := make([]relationNode, 0)

	for _, relation := range sorted(held) {
		issue, related := relation.Issue.key(), relation.RelatedIssue.key()

		if issue == "" || related == "" {
			continue
		}

		rank, carried := ranks[strings.ToLower(strings.TrimSpace(relation.Type))]
		if !carried {
			dropped = append(dropped, relation)

			continue
		}

		byPair[pairOf(issue, related)] = append(byPair[pairOf(issue, related)], pairedRelation{
			node: relation,
			rank: rank,
		})
	}

	kept := make([]relationNode, 0, len(byPair))

	for _, pair := range byPair {
		best := 0

		for index, candidate := range pair {
			if candidate.rank > pair[best].rank {
				best = index
			}
		}

		for index, candidate := range pair {
			if index == best {
				kept = append(kept, candidate.node)

				continue
			}

			dropped = append(dropped, candidate.node)
		}
	}

	sort.Slice(kept, func(one, other int) bool { return kept[one].ID < kept[other].ID })
	sort.Slice(dropped, func(one, other int) bool { return dropped[one].ID < dropped[other].ID })

	return kept, dropped
}

func sorted(held map[string]relationNode) []relationNode {
	order := make([]relationNode, 0, len(held))

	for _, relation := range held {
		order = append(order, relation)
	}

	sort.Slice(order, func(one, other int) bool { return order[one].ID < order[other].ID })

	return order
}

func pairOf(issue, related string) string {
	if issue < related {
		return issue + "\x00" + related
	}

	return related + "\x00" + issue
}
