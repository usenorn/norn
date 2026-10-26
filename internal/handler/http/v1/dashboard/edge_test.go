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
	docsvc "github.com/usenorn/norn/internal/service/doc"
	doclinksvc "github.com/usenorn/norn/internal/service/doclink"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	executionservicesvc "github.com/usenorn/norn/internal/service/executionservice"
	executionuploadsvc "github.com/usenorn/norn/internal/service/executionupload"
	hostedagentsvc "github.com/usenorn/norn/internal/service/hostedagent"
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
	memorysvc "github.com/usenorn/norn/internal/service/memory"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
	previewsvc "github.com/usenorn/norn/internal/service/preview"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	roadmapsvc "github.com/usenorn/norn/internal/service/roadmap"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
	searchsvc "github.com/usenorn/norn/internal/service/search"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	signinmethodsvc "github.com/usenorn/norn/internal/service/signinmethod"
	ssoconnectionsvc "github.com/usenorn/norn/internal/service/ssoconnection"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	telegrambotsvc "github.com/usenorn/norn/internal/service/telegrambot"
	triagesvc "github.com/usenorn/norn/internal/service/triage"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
	workspacedomainsvc "github.com/usenorn/norn/internal/service/workspacedomain"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type edgeServices struct {
	imports           service.Imports
	aiProviders       service.AIProviders
	agentCapabilities service.AgentCapabilities
	workspaces        service.Workspaces
	projects          service.Projects
	roadmaps          service.Roadmaps
	publicRoadmaps    service.PublicRoadmaps
	docs              service.Docs
	docLinks          service.DocLinks
	memories          service.Memories
	agents            service.Agents
	hostedAgents      service.HostedAgents
	telegramBots      service.TelegramBots
	sessions          service.Sessions
	signInMethods     service.SignInMethods
	workspaceDomains  service.WorkspaceDomains
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

	if services.workspaces == nil {
		services.workspaces = workspacesvc.NewMockWorkspaces(ctrl)
	}

	if services.projects == nil {
		services.projects = projectsvc.NewMockProjects(ctrl)
	}

	if services.roadmaps == nil {
		services.roadmaps = roadmapsvc.NewMockRoadmaps(ctrl)
	}

	if services.publicRoadmaps == nil {
		services.publicRoadmaps = roadmapsvc.NewMockPublicRoadmaps(ctrl)
	}

	if services.docs == nil {
		services.docs = docsvc.NewMockDocs(ctrl)
	}

	if services.docLinks == nil {
		services.docLinks = doclinksvc.NewMockDocLinks(ctrl)
	}

	if services.memories == nil {
		services.memories = memorysvc.NewMockMemories(ctrl)
	}

	if services.agents == nil {
		services.agents = agentsvc.NewMockAgents(ctrl)
	}

	if services.hostedAgents == nil {
		services.hostedAgents = hostedagentsvc.NewMockHostedAgents(ctrl)
	}

	if services.telegramBots == nil {
		services.telegramBots = telegrambotsvc.NewMockTelegramBots(ctrl)
	}

	if services.sessions == nil {
		services.sessions = sessionsvc.NewMockSessions(ctrl)
	}

	if services.signInMethods == nil {
		services.signInMethods = signinmethodsvc.NewMockSignInMethods(ctrl)
	}

	if services.workspaceDomains == nil {
		services.workspaceDomains = workspacedomainsvc.NewMockWorkspaceDomains(ctrl)
	}

	edge := dashboard.New(
		accountsvc.NewMockAccounts(ctrl),
		services.workspaces,
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
		services.agents,
		services.sessions,
		ssoconnectionsvc.NewMockSSOConnections(ctrl),
		services.signInMethods,
		services.workspaceDomains,
		cyclesvc.NewMockCycles(ctrl),
		services.projects,
		services.roadmaps,
		services.publicRoadmaps,
		services.docs,
		services.docLinks,
		services.memories,
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
		services.hostedAgents,
		services.telegramBots,
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
