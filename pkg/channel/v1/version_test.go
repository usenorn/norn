package channelv1_test

import (
	"testing"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestAServerOnlyRefusesARunnerItCanActuallyCompareItselfAgainst(t *testing.T) {
	cases := map[string]struct {
		reported  string
		minimum   string
		supported bool
	}{
		"older than the minimum":     {reported: "0.9.0", minimum: "1.2.0", supported: false},
		"exactly the minimum":        {reported: "1.2.0", minimum: "1.2.0", supported: true},
		"newer than the minimum":     {reported: "1.3.0", minimum: "1.2.0", supported: true},
		"written with a leading v":   {reported: "v1.3.0", minimum: "1.2.0", supported: true},
		"a development build":        {reported: "dev", minimum: "1.2.0", supported: true},
		"no version at all":          {reported: "", minimum: "1.2.0", supported: true},
		"a server naming no minimum": {reported: "0.1.0", minimum: "", supported: true},
		"a prerelease of the floor":  {reported: "1.2.0-rc1", minimum: "1.2.0", supported: false},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got := channelv1.RunnerSupported(want.reported, want.minimum)
			if got != want.supported {
				t.Fatalf(
					"a runner at %q against a floor of %q was supported=%v, want %v",
					want.reported, want.minimum, got, want.supported,
				)
			}
		})
	}
}

func TestOnlyASemanticVersionCountsAsAReleasedBuild(t *testing.T) {
	for _, released := range []string{"1.0.0", "v1.0.0", "0.1.0-rc1", " 1.0.0 ", "1"} {
		if !channelv1.Released(released) {
			t.Errorf("%q was not read as a released build", released)
		}
	}

	for _, unreleased := range []string{"", "dev", "unknown", "not-a-version", "1.0.0.0"} {
		if channelv1.Released(unreleased) {
			t.Errorf("%q was read as a released build", unreleased)
		}
	}
}
