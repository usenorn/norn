package dashboard

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/pkg/httpcookie"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type handler struct {
	accounts          service.Accounts
	workspaces        service.Workspaces
	teams             service.Teams
	invitations       service.Invitations
	issues            service.Issues
	issueRelations    service.IssueRelations
	issueComments     service.IssueComments
	delegations       service.Delegations
	questions         service.IssueQuestions
	attachments       service.Attachments
	bulkOperations    service.BulkOperations
	workflowStates    service.WorkflowStates
	labels            service.Labels
	apiTokens         service.APITokens
	webhooks          service.Webhooks
	webhookDeliveries service.WebhookDeliveries
	agents            service.Agents
	sessions          service.Sessions
	ssoConnections    service.SSOConnections
	cycles            service.Cycles
	projects          service.Projects
	savedViews        service.SavedViews
	triages           service.Triages
	notifications     service.Notifications
	searches          service.Searches
	auditLog          service.AuditLog
	directories       service.Directories
	licensing         service.Licensing
	imports           service.Imports
	sourceControl     service.SourceControl
	sourceControlApps service.SourceControlApps
	sourceControlCfg  config.SourceControl
	app               config.App
	instance          config.Instance
	password          config.Password
	session           config.Session
	importing         config.Imports
}

func New(
	accounts service.Accounts,
	workspaces service.Workspaces,
	teams service.Teams,
	invitations service.Invitations,
	issues service.Issues,
	issueRelations service.IssueRelations,
	issueComments service.IssueComments,
	delegations service.Delegations,
	questions service.IssueQuestions,
	attachments service.Attachments,
	bulkOperations service.BulkOperations,
	workflowStates service.WorkflowStates,
	labels service.Labels,
	apiTokens service.APITokens,
	webhooks service.Webhooks,
	webhookDeliveries service.WebhookDeliveries,
	agents service.Agents,
	sessions service.Sessions,
	ssoConnections service.SSOConnections,
	cycles service.Cycles,
	projects service.Projects,
	savedViews service.SavedViews,
	triages service.Triages,
	notifications service.Notifications,
	searches service.Searches,
	auditLog service.AuditLog,
	directories service.Directories,
	licensing service.Licensing,
	imports service.Imports,
	sourceControl service.SourceControl,
	sourceControlApps service.SourceControlApps,
	sourceControlCfg config.SourceControl,
	app config.App,
	instance config.Instance,
	password config.Password,
	session config.Session,
	importing config.Imports,
) api.StrictServerInterface {
	return &handler{
		accounts:          accounts,
		workspaces:        workspaces,
		teams:             teams,
		invitations:       invitations,
		issues:            issues,
		issueRelations:    issueRelations,
		issueComments:     issueComments,
		delegations:       delegations,
		questions:         questions,
		attachments:       attachments,
		bulkOperations:    bulkOperations,
		workflowStates:    workflowStates,
		labels:            labels,
		apiTokens:         apiTokens,
		webhooks:          webhooks,
		webhookDeliveries: webhookDeliveries,
		agents:            agents,
		sessions:          sessions,
		ssoConnections:    ssoConnections,
		cycles:            cycles,
		projects:          projects,
		savedViews:        savedViews,
		triages:           triages,
		notifications:     notifications,
		searches:          searches,
		auditLog:          auditLog,
		directories:       directories,
		licensing:         licensing,
		imports:           imports,
		sourceControl:     sourceControl,
		sourceControlApps: sourceControlApps,
		sourceControlCfg:  sourceControlCfg,
		app:               app,
		instance:          instance,
		password:          password,
		session:           session,
		importing:         importing,
	}
}

func (h *handler) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok, Version: h.app.Version}, nil
}

func (h *handler) GetInstance(_ context.Context, _ api.GetInstanceRequestObject) (api.GetInstanceResponseObject, error) {
	return api.GetInstance200JSONResponse{
		SignupsOpen: h.instance.SignupsOpen,
		Password:    h.instance.PasswordAuth,
		BreachCheck: h.password.BreachCheckEnabled,
		SelfHosted:  h.instance.SelfHosted,
		Host:        h.app.Host(),
		Version:     h.app.Version,
	}, nil
}

func (h *handler) SignIn(ctx context.Context, request api.SignInRequestObject) (api.SignInResponseObject, error) {
	if len(identity.SignedIn(ctx)) >= entity.MaxSignedInAccounts {
		return newProblem(http.StatusConflict, entity.ErrTooManySignedInAccounts.Error()), nil
	}

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

	httpcookie.Pending(ctx).Add(middleware.IssuedSessionCookie(h.session, issued.Session, issued.Token))

	return api.SignIn200JSONResponse{Slot: issued.Session.Slot}, nil
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
	accountID, err := h.accounts.ConfirmPasswordReset(ctx, request.Body.Token, request.Body.Password)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	h.expireSessionsOf(ctx, accountID)

	return api.ConfirmPasswordReset204Response{}, nil
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

	httpcookie.Pending(ctx).Add(middleware.ExpiredSessionCookie(h.session, session.Slot))

	return api.SignOut204Response{}, nil
}

// A password reset is unauthenticated, so the account whose sessions just ended is the only way
// to know which of the browser's cookies are now dead.
func (h *handler) expireSessionsOf(ctx context.Context, accountID uuid.UUID) {
	for _, session := range identity.SignedIn(ctx) {
		if session.AccountID == accountID {
			h.expireSession(ctx, session.Slot)
		}
	}
}

func (h *handler) expireSession(ctx context.Context, slot string) {
	httpcookie.Pending(ctx).Add(middleware.ExpiredSessionCookie(h.session, slot))
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
