package signinmethod

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	ssoconnectionrepo "github.com/usenorn/norn/internal/repository/ssoconnection"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	policyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	workspacedomainrepo "github.com/usenorn/norn/internal/repository/workspacedomain"
	"github.com/usenorn/norn/internal/service"
)

var started = time.Date(2026, time.October, 26, 9, 0, 0, 0, time.UTC)

type provider struct {
	protocol entity.SSOProtocol
	address  string
	verified bool
}

type harness struct {
	service *methodsService

	workspaces map[uuid.UUID]entity.Workspace
	enforced   map[uuid.UUID]entity.AuthEnforcement
	providers  map[uuid.UUID]provider
	verifiedBy map[string][]uuid.UUID

	attempts  int
	looked    []string
	deadlines []time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)
	workspaces := workspacerepo.NewMockWorkspace(ctrl)
	policies := policyrepo.NewMockWorkspaceAuthPolicy(ctrl)
	connections := ssoconnectionrepo.NewMockSSOConnection(ctrl)
	domains := workspacedomainrepo.NewMockWorkspaceDomain(ctrl)
	throttle := signinthrottlerepo.NewMockSignInThrottle(ctrl)

	h := &harness{
		workspaces: map[uuid.UUID]entity.Workspace{},
		enforced:   map[uuid.UUID]entity.AuthEnforcement{},
		providers:  map[uuid.UUID]provider{},
		verifiedBy: map[string][]uuid.UUID{},
	}

	workspaces.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.Workspace, error) {
			workspace, ok := h.workspaces[id]
			if !ok {
				return entity.Workspace{}, entity.ErrWorkspaceNotFound
			}

			return workspace, nil
		}).AnyTimes()

	workspaces.EXPECT().GetBySlug(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, slug string) (entity.Workspace, error) {
			for _, workspace := range h.workspaces {
				if workspace.Slug == slug {
					return workspace, nil
				}
			}

			return entity.Workspace{}, entity.ErrWorkspaceNotFound
		}).AnyTimes()

	policies.EXPECT().Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
			enforcement, ok := h.enforced[id]
			if !ok {
				return entity.DefaultWorkspaceAuthPolicy(id), nil
			}

			return entity.WorkspaceAuthPolicy{WorkspaceID: id, Enforcement: enforcement}, nil
		}).AnyTimes()

	connections.EXPECT().Protocol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.SSOProtocol, error) {
			configured, ok := h.providers[id]
			if !ok {
				return "", entity.ErrSSOConnectionNotFound
			}

			return configured.protocol, nil
		}).AnyTimes()

	connections.EXPECT().GetOIDC(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.OIDCConnection, error) {
			configured := h.providers[id]
			connection := entity.OIDCConnection{WorkspaceID: id, Endpoints: entity.OIDCEndpoints{Issuer: configured.address}}

			if configured.verified {
				connection.VerifiedAt = &started
			}

			return connection, nil
		}).AnyTimes()

	connections.EXPECT().GetSAML(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (entity.SAMLConnection, error) {
			configured := h.providers[id]
			connection := entity.SAMLConnection{WorkspaceID: id, Descriptor: entity.SAMLDescriptor{SSOURL: configured.address}}

			if configured.verified {
				connection.VerifiedAt = &started
			}

			return connection, nil
		}).AnyTimes()

	domains.EXPECT().VerifiedWorkspaceIDs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, domain string) ([]uuid.UUID, error) {
			h.looked = append(h.looked, domain)

			return h.verifiedBy[domain], nil
		}).AnyTimes()

	throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, netip.Addr) (int, error) {
			h.attempts++

			return h.attempts, nil
		}).AnyTimes()

	h.service = &methodsService{
		workspaces:  workspaces,
		policies:    policies,
		connections: connections,
		domains:     domains,
		throttle:    throttle,
		instance:    config.Instance{PasswordAuth: true},
		now:         func() time.Time { return started },
		waitUntil: func(_ context.Context, deadline time.Time) error {
			h.deadlines = append(h.deadlines, deadline)

			return nil
		},
	}

	return h
}

func (h *harness) workspace(slug string, enforcement entity.AuthEnforcement, configured *provider) uuid.UUID {
	id := uuid.New()
	h.workspaces[id] = entity.Workspace{ID: id, Slug: slug, Name: slug, Status: entity.WorkspaceStatusActive}
	h.enforced[id] = enforcement

	if configured != nil {
		h.providers[id] = *configured
	}

	return id
}

func (h *harness) discover(t *testing.T, email string) entity.SignInOptions {
	t.Helper()

	options, err := h.service.Discover(context.Background(), service.DiscoverSignInInput{
		Email:  email,
		Client: entity.SessionClient{IP: netip.MustParseAddr("203.0.113.7")},
	})
	if err != nil {
		t.Fatalf("discover %q: %v", email, err)
	}

	return options
}

func methods(options entity.SignInOptions) []string {
	listed := []string{}

	for _, option := range options.Options {
		method := string(option.Method)
		if option.Workspace != nil {
			method += ":" + option.Workspace.Slug + "@" + option.Workspace.Host
		}

		listed = append(listed, method)
	}

	return listed
}

func TestAWorkspaceReportsWhatItAcceptsBeforeAnyoneHasSignedIn(t *testing.T) {
	cases := []struct {
		name        string
		enforcement entity.AuthEnforcement
		provider    *provider
		want        []string
	}{
		{
			name:        "a workspace that requires a ready provider offers only that provider",
			enforcement: entity.AuthEnforcementSSO,
			provider:    &provider{protocol: entity.SSOProtocolOIDC, address: "https://northwind.okta.com", verified: true},
			want:        []string{"sso:northwind@northwind.okta.com"},
		},
		{
			name:        "a workspace without a provider still accepts a password",
			enforcement: entity.AuthEnforcementAny,
			want:        []string{"password"},
		},
		{
			name:        "a provider nobody has completed a round trip with is not offered",
			enforcement: entity.AuthEnforcementAny,
			provider:    &provider{protocol: entity.SSOProtocolOIDC, address: "https://northwind.okta.com"},
			want:        []string{"password"},
		},
		{
			name:        "a SAML provider is named by the host it sends people to",
			enforcement: entity.AuthEnforcementAny,
			provider:    &provider{protocol: entity.SSOProtocolSAML, address: "https://idp.northwind.co/saml/sso", verified: true},
			want:        []string{"sso:northwind@idp.northwind.co", "password"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.workspace("northwind", tc.enforcement, tc.provider)

			signIn, err := h.service.ForWorkspace(context.Background(), "northwind")
			if err != nil {
				t.Fatalf("ForWorkspace: %v", err)
			}

			if got := methods(signIn.SignInOptions); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("methods = %v, want %v", got, tc.want)
			}

			if signIn.Slug != "northwind" || signIn.Name != "northwind" {
				t.Errorf("signIn = %+v", signIn)
			}
		})
	}
}

func TestDiscoveryRoutesOnTheDomainAndNeverOnTheAddress(t *testing.T) {
	h := newHarness(t)
	ready := &provider{protocol: entity.SSOProtocolOIDC, address: "https://northwind.okta.com", verified: true}
	northwind := h.workspace("northwind", entity.AuthEnforcementAny, ready)
	h.verifiedBy["northwind.co"] = []uuid.UUID{northwind}

	member := h.discover(t, "rae@northwind.co")
	stranger := h.discover(t, "nobody-by-this-name@northwind.co")

	if !reflect.DeepEqual(member, stranger) {
		t.Fatalf("two addresses on one domain got different answers:\n%+v\n%+v", member, stranger)
	}

	if got := methods(member); !reflect.DeepEqual(got, []string{"sso:northwind@northwind.okta.com", "password"}) {
		t.Errorf("methods = %v", got)
	}

	if member.Preferred != entity.SessionAuthMethodSSO {
		t.Errorf("preferred = %q, want sso", member.Preferred)
	}

	personal := h.discover(t, "rae@gmail.com")
	if got := methods(personal); !reflect.DeepEqual(got, []string{"password"}) {
		t.Errorf("an unclaimed domain got %v, want password only", got)
	}
}

func TestEveryWorkspaceVerifyingTheDomainIsOffered(t *testing.T) {
	h := newHarness(t)
	ready := func(host string) *provider {
		return &provider{protocol: entity.SSOProtocolOIDC, address: "https://" + host, verified: true}
	}

	northwind := h.workspace("northwind", entity.AuthEnforcementSSO, ready("northwind.okta.com"))
	labs := h.workspace("northwind-labs", entity.AuthEnforcementSSO, ready("labs.okta.com"))
	closing := h.workspace("northwind-old", entity.AuthEnforcementAny, ready("old.okta.com"))

	gone := h.workspaces[closing]
	gone.Status = entity.WorkspaceStatusPendingDeletion
	h.workspaces[closing] = gone

	h.verifiedBy["northwind.co"] = []uuid.UUID{northwind, labs, closing}

	got := methods(h.discover(t, "rae@northwind.co"))
	want := []string{"sso:northwind@northwind.okta.com", "sso:northwind-labs@labs.okta.com"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("methods = %v, want %v", got, want)
	}
}

func TestEveryDiscoveryTakesTheSameTime(t *testing.T) {
	h := newHarness(t)
	ready := &provider{protocol: entity.SSOProtocolOIDC, address: "https://northwind.okta.com", verified: true}
	h.verifiedBy["northwind.co"] = []uuid.UUID{h.workspace("northwind", entity.AuthEnforcementAny, ready)}

	h.discover(t, "rae@northwind.co")
	h.discover(t, "rae@unclaimed.example")

	_, err := h.service.Discover(context.Background(), service.DiscoverSignInInput{Email: "not an address"})
	if err == nil {
		t.Fatal("a malformed address was accepted")
	}

	h.attempts = entity.SignInAddressMaxAttempts

	_, err = h.service.Discover(context.Background(), service.DiscoverSignInInput{Email: "rae@northwind.co"})
	if !errors.Is(err, entity.ErrSignInRateLimited) {
		t.Fatalf("err = %v, want ErrSignInRateLimited", err)
	}

	want := started.Add(entity.SignInDiscoveryFloor)

	if len(h.deadlines) != 4 {
		t.Fatalf("waited %d times, want once for each of 4 discoveries", len(h.deadlines))
	}

	for i, deadline := range h.deadlines {
		if !deadline.Equal(want) {
			t.Errorf("discovery %d waited until %s, want %s", i, deadline, want)
		}
	}
}

func TestAMissStillDoesTheLookupAHitDoes(t *testing.T) {
	h := newHarness(t)

	h.discover(t, "rae@unclaimed.example")
	h.discover(t, "rae@intranet")

	if !reflect.DeepEqual(h.looked, []string{"unclaimed.example", ""}) {
		t.Fatalf("looked up %v, want both addresses to reach the domain lookup", h.looked)
	}

	if h.attempts != 2 {
		t.Errorf("throttle counted %d attempts, want 2", h.attempts)
	}
}
