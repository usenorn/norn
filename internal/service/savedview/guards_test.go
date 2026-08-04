package savedview_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
)

func serviceSources(t *testing.T) map[string]string {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list saved view service sources: %v", err)
	}

	sources := map[string]string{}

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		sources[name] = string(source)
	}

	if len(sources) == 0 {
		t.Fatal("found no saved view service sources to guard")
	}

	return sources
}

func TestTheSavedViewServiceCannotRunAnIssueQuery(t *testing.T) {
	forbidden := map[reflect.Type]string{
		reflect.TypeOf((*repository.Issue)(nil)).Elem(): "repository.Issue",
		reflect.TypeOf((*service.Issues)(nil)).Elem():   "service.Issues",
	}

	constructor := reflect.TypeOf(savedviewsvc.New)

	for i := range constructor.NumIn() {
		if name, banned := forbidden[constructor.In(i)]; banned {
			t.Fatalf(
				"the saved view service was handed %s. A shared view must be evaluated with the "+
					"permissions of whoever opens it; the moment this service can run a query "+
					"itself, there is a code path that could run one as somebody else.",
				name,
			)
		}
	}
}

func TestNoSavedViewSurfaceCarriesAnIssue(t *testing.T) {
	issueTypes := map[reflect.Type]string{
		reflect.TypeOf(entity.Issue{}):             "entity.Issue",
		reflect.TypeOf(entity.IssuePage{}):         "entity.IssuePage",
		reflect.TypeOf(service.IssueQueryResult{}): "service.IssueQueryResult",
	}

	mentions := func(t reflect.Type) string {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}

		return issueTypes[t]
	}

	surfaces := map[string]reflect.Type{
		"repository.SavedView": reflect.TypeOf((*repository.SavedView)(nil)).Elem(),
		"service.SavedViews":   reflect.TypeOf((*service.SavedViews)(nil)).Elem(),
	}

	for surfaceName, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := surface.Method(i)

			for j := range method.Type.NumIn() {
				if name := mentions(method.Type.In(j)); name != "" {
					t.Errorf("%s.%s takes %s", surfaceName, method.Name, name)
				}
			}

			for j := range method.Type.NumOut() {
				if name := mentions(method.Type.Out(j)); name != "" {
					t.Errorf(
						"%s.%s returns %s. A saved view is a configuration, not a result set: "+
							"the moment it can hand back issues, the issues it hands back were "+
							"chosen by this service rather than by the caller's own query.",
						surfaceName, method.Name, name,
					)
				}
			}
		}
	}
}

func TestTheSavedViewServiceNeverReplacesTheActorInTheContext(t *testing.T) {
	for name, source := range serviceSources(t) {
		for _, replacement := range []string{"identity.WithActor", "identity.Into"} {
			if strings.Contains(source, replacement) {
				t.Errorf(
					"%s calls %s. internal/service/bulkoperation does that deliberately, because a "+
						"queued job has to run as whoever queued it. A saved view is not a queued "+
						"job: it is opened by whoever is holding it and must be evaluated as them.",
					name, replacement,
				)
			}
		}
	}
}

func TestTheSavedViewServiceNeverBuildsATeamScopeOfItsOwn(t *testing.T) {
	for name, source := range serviceSources(t) {
		if strings.Contains(source, "entity.TeamScope{") {
			t.Errorf(
				"%s builds a TeamScope. The only visibility this service may act on is the one the "+
					"authorizer just computed for this request; a scope assembled here is a scope "+
					"that could belong to somebody else.",
				name,
			)
		}
	}
}

func TestSavedViewsNeverPutACountOnTheWire(t *testing.T) {
	carriesInt := func(t reflect.Type) bool {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}

		if t.Kind() == reflect.Int {
			return true
		}

		if t.Kind() != reflect.Struct {
			return false
		}

		for i := range t.NumField() {
			if t.Field(i).Type.Kind() == reflect.Int {
				return true
			}
		}

		return false
	}

	surfaces := map[string]reflect.Type{
		"repository.SavedView": reflect.TypeOf((*repository.SavedView)(nil)).Elem(),
		"service.SavedViews":   reflect.TypeOf((*service.SavedViews)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := surface.Method(i)

			for j := range method.Type.NumOut() {
				if carriesInt(method.Type.Out(j)) {
					t.Errorf(
						"%s.%s returns a tally. A view must never say how many issues it matches "+
							"nor how many people hold it; the first counts across a scope this "+
							"surface does not carry, the second counts colleagues.",
						name, method.Name,
					)
				}
			}
		}
	}

	for name, carrier := range map[string]reflect.Type{
		"entity.SavedView":            reflect.TypeOf(entity.SavedView{}),
		"entity.IssueFilterReference": reflect.TypeOf(entity.IssueFilterReference{}),
		"service.SavedViewSummary":    reflect.TypeOf(service.SavedViewSummary{}),
	} {
		for i := range carrier.NumField() {
			if carrier.Field(i).Type.Kind() == reflect.Int {
				t.Errorf(
					"%s.%s is a plain int. Ordering is expressed by the order of the rows, so "+
						"nothing on this surface needs a number.",
					name, carrier.Field(i).Name,
				)
			}
		}
	}
}
