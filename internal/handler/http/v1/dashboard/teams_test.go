package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	bulkoperationsvc "github.com/usenorn/norn/internal/service/bulkoperation"
	checksvc "github.com/usenorn/norn/internal/service/check"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	delegationsvc "github.com/usenorn/norn/internal/service/delegation"
	directorysvc "github.com/usenorn/norn/internal/service/directory"
	importssvc "github.com/usenorn/norn/internal/service/imports"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	issuequestionsvc "github.com/usenorn/norn/internal/service/issuequestion"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	licensingsvc "github.com/usenorn/norn/internal/service/licensing"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
	searchsvc "github.com/usenorn/norn/internal/service/search"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	ssoconnectionsvc "github.com/usenorn/norn/internal/service/ssoconnection"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	triagesvc "github.com/usenorn/norn/internal/service/triage"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type teamHarness struct {
	teams     *teamsvc.MockTeams
	routes    http.Handler
	workspace uuid.UUID
	account   uuid.UUID
}

func newTeamHarness(t *testing.T) *teamHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &teamHarness{
		teams:     teamsvc.NewMockTeams(ctrl),
		workspace: uuid.New(),
		account:   uuid.New(),
	}

	edge := dashboard.New(
		accountsvc.NewMockAccounts(ctrl),
		workspacesvc.NewMockWorkspaces(ctrl),
		h.teams,
		invitationsvc.NewMockInvitations(ctrl),
		issuesvc.NewMockIssues(ctrl),
		issuerelationsvc.NewMockIssueRelations(ctrl),
		issuecommentsvc.NewMockIssueComments(ctrl),
		delegationsvc.NewMockDelegations(ctrl),
		checksvc.NewMockChecks(ctrl),
		issuequestionsvc.NewMockIssueQuestions(ctrl),
		attachmentsvc.NewMockAttachments(ctrl),
		bulkoperationsvc.NewMockBulkOperations(ctrl),
		workflowstatesvc.NewMockWorkflowStates(ctrl),
		labelsvc.NewMockLabels(ctrl),
		apitokensvc.NewMockAPITokens(ctrl),
		webhooksvc.NewMockWebhooks(ctrl),
		webhooksvc.NewMockWebhookDeliveries(ctrl),
		agentsvc.NewMockAgents(ctrl),
		sessionsvc.NewMockSessions(ctrl),
		ssoconnectionsvc.NewMockSSOConnections(ctrl),
		cyclesvc.NewMockCycles(ctrl),
		projectsvc.NewMockProjects(ctrl),
		savedviewsvc.NewMockSavedViews(ctrl),
		triagesvc.NewMockTriages(ctrl),
		notificationsvc.NewMockNotifications(ctrl),
		searchsvc.NewMockSearches(ctrl),
		auditsvc.NewMockAuditLog(ctrl),
		directorysvc.NewMockDirectories(ctrl),
		licensingsvc.NewMockLicensing(ctrl),
		importssvc.NewMockImports(ctrl),
		scmsvc.NewMockSourceControl(ctrl),
		scmsvc.NewMockSourceControlApps(ctrl),
		config.SourceControl{},
		config.App{Version: "test"},
		config.Instance{},
		config.Password{},
		config.Session{},
		config.Imports{},
	)

	h.routes = api.Handler(api.NewStrictHandler(edge, nil))

	return h
}

func (h *teamHarness) list(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+h.workspace.String()+"/teams"+query, nil)
	recorder := httptest.NewRecorder()

	h.routes.ServeHTTP(recorder, request.WithContext(identity.Into(request.Context(), h.account)))

	return recorder
}

func TestTheTeamListLeavesArchivedTeamsOutUntilACallerAsksForThem(t *testing.T) {
	cases := map[string]struct {
		query string
		want  entity.TeamStatus
	}{
		"no status at all":   {query: "", want: entity.TeamStatusActive},
		"an empty status":    {query: "?status=", want: entity.TeamStatusActive},
		"active asked for":   {query: "?status=active", want: entity.TeamStatusActive},
		"archived asked for": {query: "?status=archived", want: entity.TeamStatusArchived},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTeamHarness(t)

			h.teams.EXPECT().
				List(gomock.Any(), h.workspace, each.want).
				Return([]entity.Team{}, nil)

			recorder := h.list(t, each.query)
			if recorder.Code != http.StatusOK {
				t.Fatalf("list %q = %d %s, want 200", each.query, recorder.Code, recorder.Body.String())
			}
		})
	}
}
