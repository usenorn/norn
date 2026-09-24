package dashboard_test

import (
	"net/http"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/service"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	agentcapabilitysvc "github.com/usenorn/norn/internal/service/agentcapability"
	aiprovidersvc "github.com/usenorn/norn/internal/service/aiprovider"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	bulkoperationsvc "github.com/usenorn/norn/internal/service/bulkoperation"
	changesetsvc "github.com/usenorn/norn/internal/service/changeset"
	codebasesvc "github.com/usenorn/norn/internal/service/codebase"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	delegationsvc "github.com/usenorn/norn/internal/service/delegation"
	directorysvc "github.com/usenorn/norn/internal/service/directory"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	executionservicesvc "github.com/usenorn/norn/internal/service/executionservice"
	executionuploadsvc "github.com/usenorn/norn/internal/service/executionupload"
	importssvc "github.com/usenorn/norn/internal/service/imports"
	intakesvc "github.com/usenorn/norn/internal/service/intake"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	issuecriterionsvc "github.com/usenorn/norn/internal/service/issuecriterion"
	issuedraftsvc "github.com/usenorn/norn/internal/service/issuedraft"
	issuequestionsvc "github.com/usenorn/norn/internal/service/issuequestion"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	issuetemplatesvc "github.com/usenorn/norn/internal/service/issuetemplate"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	licensingsvc "github.com/usenorn/norn/internal/service/licensing"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
	previewsvc "github.com/usenorn/norn/internal/service/preview"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
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

type edgeServices struct {
	imports           service.Imports
	aiProviders       service.AIProviders
	agentCapabilities service.AgentCapabilities
}

func newEdge(ctrl *gomock.Controller, services edgeServices) http.Handler {
	if services.imports == nil {
		services.imports = importssvc.NewMockImports(ctrl)
	}

	if services.aiProviders == nil {
		services.aiProviders = aiprovidersvc.NewMockAIProviders(ctrl)
	}

	if services.agentCapabilities == nil {
		services.agentCapabilities = agentcapabilitysvc.NewMockAgentCapabilities(ctrl)
	}

	edge := dashboard.New(
		accountsvc.NewMockAccounts(ctrl),
		workspacesvc.NewMockWorkspaces(ctrl),
		teamsvc.NewMockTeams(ctrl),
		invitationsvc.NewMockInvitations(ctrl),
		issuesvc.NewMockIssues(ctrl),
		issuedraftsvc.NewMockIssueDrafts(ctrl),
		issuetemplatesvc.NewMockIssueTemplates(ctrl),
		issuecriterionsvc.NewMockIssueCriteria(ctrl),
		issuerelationsvc.NewMockIssueRelations(ctrl),
		issuecommentsvc.NewMockIssueComments(ctrl),
		delegationsvc.NewMockDelegations(ctrl),
		issuequestionsvc.NewMockIssueQuestions(ctrl),
		runnersvc.NewMockRunners(ctrl),
		codebasesvc.NewMockCodebases(ctrl),
		executionsvc.NewMockExecutions(ctrl),
		executionservicesvc.NewMockExecutionServices(ctrl),
		executionuploadsvc.NewMockExecutionUploads(ctrl),
		changesetsvc.NewMockChangeSets(ctrl),
		previewsvc.NewMockPreviews(ctrl),
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
		intakesvc.NewMockIntakes(ctrl),
		notificationsvc.NewMockNotifications(ctrl),
		searchsvc.NewMockSearches(ctrl),
		auditsvc.NewMockAuditLog(ctrl),
		directorysvc.NewMockDirectories(ctrl),
		licensingsvc.NewMockLicensing(ctrl),
		services.imports,
		scmsvc.NewMockSourceControl(ctrl),
		scmsvc.NewMockSourceControlApps(ctrl),
		services.aiProviders,
		services.agentCapabilities,
		config.SourceControl{},
		config.App{Version: "test", BaseURL: "https://norn.test"},
		config.Instance{},
		config.Password{},
		config.Session{},
		config.Imports{MaxUploadBytes: testMaxUploadBytes},
		config.Previews{Scheme: "https"},
	)

	return api.Handler(api.NewStrictHandler(edge, nil))
}
