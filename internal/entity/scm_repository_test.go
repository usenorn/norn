package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestARouteOwnsWholePathSegmentsOnly(t *testing.T) {
	route := SCMRoute{PathPrefix: "api"}

	cases := map[string]bool{
		"api":            true,
		"api/main.go":    true,
		"api/v1/main.go": true,
		"/api/main.go":   true,
		"apiary/main.go": false,
		"web/api.go":     false,
		"":               false,
	}

	for path, want := range cases {
		if got := route.Covers(path); got != want {
			t.Errorf("route %q covering %q = %t, want %t", route.PathPrefix, path, got, want)
		}
	}
}

func TestTheDefaultRouteOwnsEverything(t *testing.T) {
	route := SCMRoute{PathPrefix: ""}

	if !route.Default() {
		t.Fatal("a route with no prefix is the repository default")
	}

	for _, path := range []string{"", "api/main.go", "anything/at/all"} {
		if !route.Covers(path) {
			t.Errorf("the default route should cover %q", path)
		}
	}
}

func TestTheLongestMatchingPrefixTakesThePath(t *testing.T) {
	platform, api, web := uuid.New(), uuid.New(), uuid.New()

	routes := SCMRoutes{
		{TeamID: platform, PathPrefix: ""},
		{TeamID: api, PathPrefix: "services/api"},
		{TeamID: web, PathPrefix: "services/web"},
	}

	cases := []struct {
		name  string
		paths []string
		want  []uuid.UUID
	}{
		{"a path under a routed prefix goes to that team alone", []string{"services/api/main.go"}, []uuid.UUID{api}},
		{"an unrouted path falls to the default", []string{"README.md"}, []uuid.UUID{platform}},
		{"a change spanning two areas reaches both", []string{"services/api/main.go", "services/web/app.ts"}, []uuid.UUID{api, web}},
		{"a change with no paths reaches the default", nil, []uuid.UUID{platform}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := routes.Teams(test.paths)

			if len(got) != len(test.want) {
				t.Fatalf("reached %d team(s), want %d", len(got), len(test.want))
			}

			for _, wanted := range test.want {
				if !containsID(got, wanted) {
					t.Errorf("team %s should have been reached", wanted)
				}
			}
		})
	}
}

// The default route is usually created first and so is read last as often as not. Ordering
// must not decide the owner, or the same repository routes differently after a reordering.
func TestTheLongestPrefixWinsWhateverOrderTheRoutesArrivedIn(t *testing.T) {
	api, platform := uuid.New(), uuid.New()

	orders := map[string]SCMRoutes{
		"default first": {{TeamID: platform, PathPrefix: ""}, {TeamID: api, PathPrefix: "services/api"}},
		"default last":  {{TeamID: api, PathPrefix: "services/api"}, {TeamID: platform, PathPrefix: ""}},
	}

	for name, routes := range orders {
		t.Run(name, func(t *testing.T) {
			got := routes.Teams([]string{"services/api/main.go"})

			if len(got) != 1 || got[0] != api {
				t.Fatalf("routed to %v, want the api team alone", got)
			}
		})
	}
}

func TestTeamsSharingAPrefixBothReceiveTheChange(t *testing.T) {
	first, second := uuid.New(), uuid.New()

	routes := SCMRoutes{
		{TeamID: first, PathPrefix: "api"},
		{TeamID: second, PathPrefix: "api"},
	}

	if got := routes.Teams([]string{"api/main.go"}); len(got) != 2 {
		t.Fatalf("reached %d team(s), want both", len(got))
	}
}

func TestAChangeWithNoRouteAtAllReachesNobody(t *testing.T) {
	routes := SCMRoutes{{TeamID: uuid.New(), PathPrefix: "api"}}

	if got := routes.Teams([]string{"web/app.ts"}); len(got) != 0 {
		t.Fatalf("reached %d team(s), want none: an unrouted path has no owner", len(got))
	}
}

func TestAnUnroutedTeamIsOutOfReach(t *testing.T) {
	routed, other := uuid.New(), uuid.New()
	routes := SCMRoutes{{TeamID: routed, PathPrefix: "api"}}

	if !routes.Reaches(routed) {
		t.Error("a routed team is reachable")
	}

	if routes.Reaches(other) {
		t.Error("a team with no route must stay out of reach however the issue was named")
	}
}

func TestASweepReadsARepositoryOnceItsIntervalHasPassed(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	old := now.Add(-time.Hour)
	later := now.Add(time.Minute)

	cases := []struct {
		name string
		repo SCMRepository
		want bool
	}{
		{"never read", SCMRepository{PollInterval: time.Minute * 5}, true},
		{"read within the interval", SCMRepository{PollInterval: time.Minute * 5, ReconciledAt: &recent}, false},
		{"read before the interval", SCMRepository{PollInterval: time.Minute * 5, ReconciledAt: &old}, true},
		{"parked after a rate limit", SCMRepository{PollInterval: time.Minute * 5, ReconciledAt: &old, ReconcileAfter: &later}, false},
		{"no interval falls back to the default", SCMRepository{ReconciledAt: &old}, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.repo.Due(now); got != test.want {
				t.Errorf("Due = %t, want %t", got, test.want)
			}
		})
	}
}

func TestARepositoryWrittenBeforeDirectionExistedStillSyncsBothWays(t *testing.T) {
	cases := map[MirrorDirection]MirrorDirection{
		"":                         MirrorBoth,
		MirrorDirection("rubbish"): MirrorBoth,
		MirrorInbound:              MirrorInbound,
		MirrorOutbound:             MirrorOutbound,
		MirrorBoth:                 MirrorBoth,
	}

	for stored, want := range cases {
		t.Run(string(stored), func(t *testing.T) {
			repository := SCMRepository{SyncDirection: stored}

			if got := repository.Direction(); got != want {
				t.Fatalf("Direction = %q, want %q", got, want)
			}
		})
	}
}

func TestADirectionSaysWhichWayWorkMayFlow(t *testing.T) {
	cases := []struct {
		direction     MirrorDirection
		pulls, pushes bool
	}{
		{MirrorInbound, true, false},
		{MirrorOutbound, false, true},
		{MirrorBoth, true, true},
	}

	for _, test := range cases {
		t.Run(string(test.direction), func(t *testing.T) {
			if test.direction.Pulls() != test.pulls {
				t.Errorf("Pulls = %t, want %t", test.direction.Pulls(), test.pulls)
			}

			if test.direction.Pushes() != test.pushes {
				t.Errorf("Pushes = %t, want %t", test.direction.Pushes(), test.pushes)
			}
		})
	}
}
