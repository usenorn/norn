package scm

import (
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/scm/gitea"
	"github.com/usenorn/norn/internal/service/scm/github"
	"github.com/usenorn/norn/internal/service/scm/gitlab"
)

type registry struct {
	forges map[entity.SCMProvider]service.Forge
}

func NewForges(hub *github.Forge, lab *gitlab.Forge, tea *gitea.Forge) service.Forges {
	return &registry{
		forges: map[entity.SCMProvider]service.Forge{
			hub.Provider(): hub,
			lab.Provider(): lab,
			tea.Provider(): tea,
		},
	}
}

func (r *registry) Lookup(provider entity.SCMProvider) (service.Forge, error) {
	forge, known := r.forges[provider]
	if !known {
		return nil, entity.ErrSCMProviderUnsupported
	}

	return forge, nil
}

func (r *registry) Providers() []entity.SCMProvider {
	providers := make([]entity.SCMProvider, 0, len(r.forges))

	for _, provider := range entity.SCMProviders() {
		if _, known := r.forges[provider]; known {
			providers = append(providers, provider)
		}
	}

	return providers
}
