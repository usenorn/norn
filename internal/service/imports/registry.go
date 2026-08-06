package imports

import (
	"sort"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/imports/csvfile"
	"github.com/usenorn/norn/internal/service/imports/linear"
)

type sourceRegistry struct {
	sources map[string]service.ImportSource
}

// NewSourceRegistry holds every adapter this instance can import from, keyed by the kind it
// answers to. An adapter is source-specific logic and is declared here rather than discovered:
// a source nothing named is a source nothing has taken responsibility for, and Lookup refuses
// every kind but the ones handed in. The adapters arrive as themselves rather than as the port
// they implement because injection tells two parameters apart by type, and two of the same
// interface are one ambiguity.
func NewSourceRegistry(tracker *linear.Source, file *csvfile.Source) service.ImportSources {
	return &sourceRegistry{sources: map[string]service.ImportSource{
		tracker.Kind(): tracker,
		file.Kind():    file,
	}}
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
