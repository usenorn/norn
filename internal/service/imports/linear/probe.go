package linear

import (
	"context"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	noteOneLinkPerPair = "Linear lets two issues hold several links at once and a workspace " +
		"here holds one. Where a pair is both duplicated and blocked, the duplicate is what " +
		"arrives and the rest are left behind."
	noteMostAttachmentsAreLinks = "Most of what Linear calls an attachment is a link to " +
		"somewhere else — a pull request, an alert, a conversation. Only a real upload comes " +
		"across as a file; the rest are listed in the report so they can be re-linked by hand."
	noteCyclesAreRenumbered = "A cycle here is known by the number its team is up to rather " +
		"than by a name, so the names and numbers Linear gave them are reported rather than kept."
)

func catalogueNotes() []string {
	return []string{noteOneLinkPerPair, noteMostAttachmentsAreLinks, noteCyclesAreRenumbered}
}

// Probe answers the one question a run has to settle before anything is staged: which teams
// this key can see. It reads a page of teams and no records at all, which is what keeps the
// one call made outside the staging job bounded.
func (s *Source) Probe(
	ctx context.Context,
	held service.ImportSourceConfig,
) (entity.ImportCatalogue, error) {
	if strings.TrimSpace(held.Secret) == "" {
		return entity.ImportCatalogue{}, entity.ImportSourceRefusedError{
			Resource: entity.ImportTeam,
			Reason:   "this run has no Linear key, so the source has nothing to answer to",
		}
	}

	data, err := s.client.Query(
		ctx, held.Secret, scopesQuery, map[string]any{"first": pageMost}, entity.ImportTeam,
	)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	reply, err := decoded[teamsReply](data, entity.ImportTeam)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	scopes := make([]entity.ImportScope, 0, len(reply.Teams.Nodes))

	for _, node := range reply.Teams.Nodes {
		scopes = append(scopes, entity.ImportScope{
			Key:    node.ID,
			Name:   node.Name,
			Detail: node.Key,
		})
	}

	return entity.ImportCatalogue{Scopes: scopes, Notes: catalogueNotes()}, nil
}
