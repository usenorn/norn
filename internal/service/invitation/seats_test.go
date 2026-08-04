package invitation_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func TestNoLayerExposesAnInvitationCountingOperation(t *testing.T) {
	forbidden := []string{"count", "total", "quota", "seat", "limit"}

	surfaces := map[string]reflect.Type{
		"repository.Invitation": reflect.TypeOf((*repository.Invitation)(nil)).Elem(),
		"service.Invitations":   reflect.TypeOf((*service.Invitations)(nil)).Elem(),
	}

	for name, surface := range surfaces {
		for i := range surface.NumMethod() {
			method := strings.ToLower(surface.Method(i).Name)

			for _, word := range forbidden {
				if strings.Contains(method, word) {
					t.Errorf("%s exposes %q, which counts or caps invitations", name, surface.Method(i).Name)
				}
			}
		}
	}
}
