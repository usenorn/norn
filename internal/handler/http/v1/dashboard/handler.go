package dashboard

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type handler struct {
	accounts       service.Accounts
	workspaces     service.Workspaces
	teams          service.Teams
	invitations    service.Invitations
	issues         service.Issues
	issueRelations service.IssueRelations
	issueComments  service.IssueComments
	attachments    service.Attachments
	bulkOperations service.BulkOperations
	workflowStates service.WorkflowStates
	labels         service.Labels
	apiTokens      service.APITokens
	agents         service.Agents
	sessions       service.Sessions
	ssoConnections service.SSOConnections
	cycles         service.Cycles
	projects       service.Projects
	savedViews     service.SavedViews
	triages        service.Triages
	notifications  service.Notifications
	searches       service.Searches
	app            config.App
	instance       config.Instance
	session        config.Session
}

func New(
	accounts service.Accounts,
	workspaces service.Workspaces,
	teams service.Teams,
	invitations service.Invitations,
	issues service.Issues,
	issueRelations service.IssueRelations,
	issueComments service.IssueComments,
	attachments service.Attachments,
	bulkOperations service.BulkOperations,
	workflowStates service.WorkflowStates,
	labels service.Labels,
	apiTokens service.APITokens,
	agents service.Agents,
	sessions service.Sessions,
	ssoConnections service.SSOConnections,
	cycles service.Cycles,
	projects service.Projects,
	savedViews service.SavedViews,
	triages service.Triages,
	notifications service.Notifications,
	searches service.Searches,
	app config.App,
	instance config.Instance,
	session config.Session,
) api.StrictServerInterface {
	return &handler{
		accounts:       accounts,
		workspaces:     workspaces,
		teams:          teams,
		invitations:    invitations,
		issues:         issues,
		issueRelations: issueRelations,
		issueComments:  issueComments,
		attachments:    attachments,
		bulkOperations: bulkOperations,
		workflowStates: workflowStates,
		labels:         labels,
		apiTokens:      apiTokens,
		agents:         agents,
		sessions:       sessions,
		ssoConnections: ssoConnections,
		cycles:         cycles,
		projects:       projects,
		savedViews:     savedViews,
		triages:        triages,
		notifications:  notifications,
		searches:       searches,
		app:            app,
		instance:       instance,
		session:        session,
	}
}

func (h *handler) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok, Version: h.app.Version}, nil
}

func (h *handler) GetInstance(_ context.Context, _ api.GetInstanceRequestObject) (api.GetInstanceResponseObject, error) {
	return api.GetInstance200JSONResponse{
		SignupsOpen: h.instance.SignupsOpen,
		Password:    h.instance.PasswordAuth,
		SelfHosted:  h.instance.SelfHosted,
		Host:        h.app.Host(),
		Version:     h.app.Version,
	}, nil
}

func (h *handler) SignIn(ctx context.Context, request api.SignInRequestObject) (api.SignInResponseObject, error) {
	issued, err := h.sessions.SignIn(ctx, service.SignInInput{
		Email:    request.Body.Email,
		Password: request.Body.Password,
		Client:   middleware.ClientFrom(ctx),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	cookie := middleware.IssuedSessionCookie(h.session, issued.Token).String()

	return api.SignIn204Response{Headers: api.SignIn204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handler) RequestPasswordReset(ctx context.Context, request api.RequestPasswordResetRequestObject) (api.RequestPasswordResetResponseObject, error) {
	expiresAt, err := h.accounts.RequestPasswordReset(ctx, service.RequestPasswordResetInput{
		Email:  request.Body.Email,
		Client: middleware.ClientFrom(ctx),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RequestPasswordReset202JSONResponse{ExpiresAt: expiresAt}, nil
}

func (h *handler) ConfirmPasswordReset(ctx context.Context, request api.ConfirmPasswordResetRequestObject) (api.ConfirmPasswordResetResponseObject, error) {
	if err := h.accounts.ConfirmPasswordReset(ctx, request.Body.Token, request.Body.Password); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	cookie := middleware.ExpiredSessionCookie(h.session).String()

	return api.ConfirmPasswordReset204Response{
		Headers: api.ConfirmPasswordReset204ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (h *handler) SignOut(ctx context.Context, _ api.SignOutRequestObject) (api.SignOutResponseObject, error) {
	session, ok := identity.CurrentSession(ctx)
	if !ok {
		return unauthorized(), nil
	}

	if err := h.sessions.SignOut(ctx, session.ID); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	cookie := middleware.ExpiredSessionCookie(h.session).String()

	return api.SignOut204Response{Headers: api.SignOut204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handler) currentAccountID(ctx context.Context) (uuid.UUID, bool) {
	return identity.From(ctx)
}

func (h *handler) oidcRedirectBase() string {
	return strings.TrimRight(h.app.BaseURL, "/")
}

func (h *handler) oidcRedirectURI() string {
	return h.oidcRedirectBase() + oidcCallbackPath
}

func (h *handler) workspaceSlug(ctx context.Context, workspaceID uuid.UUID) string {
	workspace, err := h.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return workspaceID.String()
	}

	return workspace.Slug
}
