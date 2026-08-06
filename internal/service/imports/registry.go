package imports

import (
	"sort"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type sourceRegistry struct {
	sources map[string]service.ImportSource
}

// NewSourceRegistry holds every adapter this instance can import from. It holds none:
// an adapter is source-specific logic and arrives with the slice that needs it, so
// until then Lookup refuses everything and no import can be started.
func NewSourceRegistry() service.ImportSources {
	return &sourceRegistry{sources: map[string]service.ImportSource{}}
}

func (r *sourceRegistry) Lookup(kind string) (service.ImportSource, error) {
	source, known := r.sources[kind]
	if !known {
		return nil, entity.ErrImportSourceUnknown
	}

	return source, nil
}

func (r *sourceRegistry) Kinds() []string {
	kinds := make([]string, 0, len(r.sources))

	for kind := range r.sources {
		kinds = append(kinds, kind)
	}

	sort.Strings(kinds)

	return kinds
}
