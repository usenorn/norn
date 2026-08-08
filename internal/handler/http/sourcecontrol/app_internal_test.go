package sourcecontrol

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnExchangeAlwaysLandsOnAScreenThatExists(t *testing.T) {
	for _, workspace := range []string{"", "northwind"} {
		target := screen(workspace)

		if target == sourceControlScreen {
			t.Fatalf(
				"a %q workspace sends the browser to %q. Every settings screen lives under a "+
					"workspace segment, so that address resolves to nothing and the exchange ends "+
					"on a not-found page instead of an explanation",
				workspace, target,
			)
		}

		if !strings.HasPrefix(target, "/") {
			t.Errorf("the target %q is not a path the browser can follow", target)
		}
	}
}

func TestEachWayAnExchangeCanFailIsNamedRatherThanLumpedTogether(t *testing.T) {
	cases := map[error]string{
		entity.ErrSCMAppStateNotFound:      "expired",
		entity.ErrSCMAppRefused:            "refused",
		entity.ErrSCMAppNotFound:           "unregistered",
		errors.New("something unforeseen"): "unavailable",
	}

	for err, want := range cases {
		if got := reason(err); got != want {
			t.Errorf("%v was reported as %q, want %q", err, got, want)
		}
	}
}
