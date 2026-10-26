package dashboard_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	signinmethodsvc "github.com/usenorn/norn/internal/service/signinmethod"
	workspacedomainsvc "github.com/usenorn/norn/internal/service/workspacedomain"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func TestDiscoveryNamesEachWorkspaceAndPrefersItsProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	methods := signinmethodsvc.NewMockSignInMethods(ctrl)
	routes := newEdge(ctrl, edgeServices{signInMethods: methods})

	methods.EXPECT().
		Discover(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, input service.DiscoverSignInInput) (entity.SignInOptions, error) {
			if input.Email != "rae@northwind.co" {
				t.Errorf("email = %q", input.Email)
			}

			return entity.SignInOptions{
				Options: []entity.SignInOption{
					{
						Method: entity.SessionAuthMethodSSO,
						Workspace: &entity.SignInWorkspace{
							Slug: "northwind", Name: "Northwind", Protocol: entity.SSOProtocolSAML, Host: "idp.northwind.co",
						},
					},
					{Method: entity.SessionAuthMethodPassword},
				},
				Preferred: entity.SessionAuthMethodSSO,
			}, nil
		})

	recorder := send(t, routes, http.MethodPost, "/auth/discover", api.DiscoverSignInRequest{Email: "rae@northwind.co"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.SignInOptions](t, recorder)

	if got.Preferred == nil || *got.Preferred != api.SignInMethodSso || len(got.Options) != 2 {
		t.Fatalf("options = %+v", got)
	}

	workspace := got.Options[0].Workspace
	if workspace == nil || workspace.Workspace != "northwind" || workspace.Host == nil || *workspace.Host != "idp.northwind.co" ||
		workspace.Protocol == nil || *workspace.Protocol != api.Saml {
		t.Errorf("sso option = %+v", workspace)
	}

	if got.Options[1].Method != api.SignInMethodPassword || got.Options[1].Workspace != nil {
		t.Errorf("password option = %+v", got.Options[1])
	}
}

func TestDiscoveryAnswersARateLimitWithRetryAfter(t *testing.T) {
	ctrl := gomock.NewController(t)
	methods := signinmethodsvc.NewMockSignInMethods(ctrl)
	routes := newEdge(ctrl, edgeServices{signInMethods: methods})

	methods.EXPECT().Discover(gomock.Any(), gomock.Any()).Return(entity.SignInOptions{}, entity.ErrSignInRateLimited)

	recorder := send(t, routes, http.MethodPost, "/auth/discover", api.DiscoverSignInRequest{Email: "rae@northwind.co"})
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("status = %d retry-after %q", recorder.Code, recorder.Header().Get("Retry-After"))
	}
}

func TestAWorkspaceSignInListsItsMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	methods := signinmethodsvc.NewMockSignInMethods(ctrl)
	routes := newEdge(ctrl, edgeServices{signInMethods: methods})

	methods.EXPECT().ForWorkspace(gomock.Any(), "northwind").Return(entity.WorkspaceSignIn{
		Slug: "northwind",
		Name: "Northwind",
		SignInOptions: entity.SignInOptions{
			Options:   []entity.SignInOption{{Method: entity.SessionAuthMethodPassword}},
			Preferred: entity.SessionAuthMethodPassword,
		},
	}, nil)

	recorder := send(t, routes, http.MethodGet, "/sso/sign-in?workspace=northwind", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.WorkspaceSignIn](t, recorder)
	if got.Workspace != "northwind" || len(got.Options) != 1 || got.Options[0].Method != api.SignInMethodPassword ||
		got.Preferred == nil || *got.Preferred != api.SignInMethodPassword {
		t.Errorf("signIn = %+v", got)
	}
}

func TestADomainClaimShowsTheRecordToPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	domains := workspacedomainsvc.NewMockWorkspaceDomains(ctrl)
	routes := newEdge(ctrl, edgeServices{workspaceDomains: domains})
	workspaceID := uuid.New()
	claimID := uuid.New()

	domains.EXPECT().Add(gomock.Any(), workspaceID, "northwind.co").Return(entity.WorkspaceDomain{
		ID: claimID, WorkspaceID: workspaceID, Domain: "northwind.co", Token: "abc", CreatedAt: time.Now(),
	}, nil)

	recorder := send(t, routes, http.MethodPost, "/workspaces/"+workspaceID.String()+"/sso/domains",
		api.AddWorkspaceDomainRequest{Domain: "northwind.co"})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.WorkspaceDomain](t, recorder)
	if got.Id != claimID || got.Record != "norn-verification=abc" || got.VerifiedAt != nil {
		t.Errorf("domain = %+v", got)
	}
}

func TestDomainFailuresMapToTheirStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "a domain claimed twice conflicts", err: entity.ErrWorkspaceDomainTaken, want: http.StatusConflict},
		{
			name: "a personal domain is a validation failure",
			err:  entity.NewValidationError(entity.FieldError{Field: "domain", Code: entity.ValidationCodePersonalDomain}),
			want: http.StatusUnprocessableEntity,
		},
		{name: "a member who is not an admin is forbidden", err: entity.ErrAccountForbidden, want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domains := workspacedomainsvc.NewMockWorkspaceDomains(ctrl)
			routes := newEdge(ctrl, edgeServices{workspaceDomains: domains})
			workspaceID := uuid.New()

			domains.EXPECT().Add(gomock.Any(), workspaceID, gomock.Any()).Return(entity.WorkspaceDomain{}, tc.err)

			recorder := send(t, routes, http.MethodPost, "/workspaces/"+workspaceID.String()+"/sso/domains",
				api.AddWorkspaceDomainRequest{Domain: "gmail.com"})
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d, body %s", recorder.Code, tc.want, recorder.Body)
			}
		})
	}

	ctrl := gomock.NewController(t)
	domains := workspacedomainsvc.NewMockWorkspaceDomains(ctrl)
	routes := newEdge(ctrl, edgeServices{workspaceDomains: domains})
	workspaceID := uuid.New()
	claimID := uuid.New()

	domains.EXPECT().Verify(gomock.Any(), workspaceID, claimID).Return(entity.WorkspaceDomain{}, entity.ErrWorkspaceDomainLookupFailed)

	recorder := send(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/sso/domains/"+claimID.String()+"/verify", nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("lookup failure: status = %d", recorder.Code)
	}
}
