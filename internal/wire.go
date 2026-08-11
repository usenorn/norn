//go:build wireinject

package internal

import (
	"github.com/goforj/wire"

	"github.com/usenorn/norn/internal/config"
	auditexportedge "github.com/usenorn/norn/internal/handler/http/auditexport"
	blobedge "github.com/usenorn/norn/internal/handler/http/blob"
	eventsedge "github.com/usenorn/norn/internal/handler/http/events"
	"github.com/usenorn/norn/internal/handler/http/router"
	scimedge "github.com/usenorn/norn/internal/handler/http/scim"
	sourcecontroledge "github.com/usenorn/norn/internal/handler/http/sourcecontrol"
	ssohandler "github.com/usenorn/norn/internal/handler/http/sso"
	dashboardhandler "github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/handler/job"
	mcpserveredge "github.com/usenorn/norn/internal/handler/mcpserver"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/authz"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/pkg/geoip"
	licencepkg "github.com/usenorn/norn/internal/pkg/licence"
	"github.com/usenorn/norn/internal/pkg/lineargraph"
	oidcproviderpkg "github.com/usenorn/norn/internal/pkg/oidcprovider"
	"github.com/usenorn/norn/internal/pkg/outbound"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/pkg/pwned"
	samlproviderpkg "github.com/usenorn/norn/internal/pkg/samlprovider"
	"github.com/usenorn/norn/internal/pkg/smtp"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	agentthrottlerepo "github.com/usenorn/norn/internal/repository/agentthrottle"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	attachmentrepo "github.com/usenorn/norn/internal/repository/attachment"
	auditrepo "github.com/usenorn/norn/internal/repository/audit"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	blobgrantrepo "github.com/usenorn/norn/internal/repository/blobgrant"
	breachcheckrepo "github.com/usenorn/norn/internal/repository/breachcheck"
	breakglassrepo "github.com/usenorn/norn/internal/repository/breakglass"
	bulkactionrepo "github.com/usenorn/norn/internal/repository/bulkaction"
	checkrepo "github.com/usenorn/norn/internal/repository/check"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	directoryrepo "github.com/usenorn/norn/internal/repository/directory"
	emailchangerepo "github.com/usenorn/norn/internal/repository/emailchange"
	eventstreamrepo "github.com/usenorn/norn/internal/repository/eventstream"
	geolocationrepo "github.com/usenorn/norn/internal/repository/geolocation"
	importsrepo "github.com/usenorn/norn/internal/repository/imports"
	invitationrepo "github.com/usenorn/norn/internal/repository/invitation"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuecommentrepo "github.com/usenorn/norn/internal/repository/issuecomment"
	issuedelegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	issuefilterreferencerepo "github.com/usenorn/norn/internal/repository/issuefilterreference"
	issuefollowerrepo "github.com/usenorn/norn/internal/repository/issuefollower"
	issuerelationrepo "github.com/usenorn/norn/internal/repository/issuerelation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	labelgrouprepo "github.com/usenorn/norn/internal/repository/labelgroup"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	mcpthrottlerepo "github.com/usenorn/norn/internal/repository/mcpthrottle"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	notificationrepo "github.com/usenorn/norn/internal/repository/notification"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	notificationsettingrepo "github.com/usenorn/norn/internal/repository/notificationsetting"
	oidcproviderrepo "github.com/usenorn/norn/internal/repository/oidcprovider"
	oidcstaterepo "github.com/usenorn/norn/internal/repository/oidcstate"
	passwordhistoryrepo "github.com/usenorn/norn/internal/repository/passwordhistory"
	passwordresetrepo "github.com/usenorn/norn/internal/repository/passwordreset"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	samlreplayrepo "github.com/usenorn/norn/internal/repository/samlreplay"
	samlrequestrepo "github.com/usenorn/norn/internal/repository/samlrequest"
	savedviewrepo "github.com/usenorn/norn/internal/repository/savedview"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	scmappstaterepo "github.com/usenorn/norn/internal/repository/scmappstate"
	searchrepo "github.com/usenorn/norn/internal/repository/search"
	sessionrepo "github.com/usenorn/norn/internal/repository/session"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	signuprepo "github.com/usenorn/norn/internal/repository/signup"
	ssoconnectionrepo "github.com/usenorn/norn/internal/repository/ssoconnection"
	ssoidentityrepo "github.com/usenorn/norn/internal/repository/ssoidentity"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	webhookrepo "github.com/usenorn/norn/internal/repository/webhook"
	webhooksenderrepo "github.com/usenorn/norn/internal/repository/webhooksender"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	workspaceauthpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	agentholdsvc "github.com/usenorn/norn/internal/service/agenthold"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	bulkoperationsvc "github.com/usenorn/norn/internal/service/bulkoperation"
	checksvc "github.com/usenorn/norn/internal/service/check"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	delegationsvc "github.com/usenorn/norn/internal/service/delegation"
	directorysvc "github.com/usenorn/norn/internal/service/directory"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	importssvc "github.com/usenorn/norn/internal/service/imports"
	csvfilesvc "github.com/usenorn/norn/internal/service/imports/csvfile"
	linearsvc "github.com/usenorn/norn/internal/service/imports/linear"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	jobssvc "github.com/usenorn/norn/internal/service/jobs"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	licensingsvc "github.com/usenorn/norn/internal/service/licensing"
	notificationsvc "github.com/usenorn/norn/internal/service/notification"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	savedviewsvc "github.com/usenorn/norn/internal/service/savedview"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
	giteasvc "github.com/usenorn/norn/internal/service/scm/gitea"
	githubsvc "github.com/usenorn/norn/internal/service/scm/github"
	gitlabsvc "github.com/usenorn/norn/internal/service/scm/gitlab"
	searchsvc "github.com/usenorn/norn/internal/service/search"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	ssoconnectionsvc "github.com/usenorn/norn/internal/service/ssoconnection"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	triagesvc "github.com/usenorn/norn/internal/service/triage"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
)

var baseSet = wire.NewSet(
	config.Set,
	logging.Set,

	postgres.Set,
	valkey.Set,
	taskqueue.Set,
	smtp.Set,
	authz.Set,
	geoip.Set,
	pwned.Set,
	crypter.Set,
	licencepkg.Set,
	lineargraph.Set,
	forge.Set,
	outbound.Set,
	oidcproviderpkg.Set,
	samlproviderpkg.Set,

	wire.Bind(new(repository.Transactor), new(*postgres.Client)),

	accountrepo.Set,
	emailchangerepo.Set,
	workspacerepo.Set,
	membershiprepo.Set,
	sessionrepo.Set,
	blobrepo.Set,
	mailerrepo.Set,
	jobqueuerepo.Set,
	geolocationrepo.Set,
	workspaceauthpolicyrepo.Set,
	passwordresetrepo.Set,
	signuprepo.Set,
	issuerepo.Set,
	activityrepo.Set,
	issuedelegationrepo.Set,
	issuerelationrepo.Set,
	bulkactionrepo.Set,
	checkrepo.Set,
	cyclerepo.Set,
	projectrepo.Set,
	attachmentrepo.Set,
	blobgrantrepo.Set,
	issuecommentrepo.Set,
	issuefollowerrepo.Set,
	notificationrepo.Set,
	notificationeventrepo.Set,
	notificationsettingrepo.Set,
	savedviewrepo.Set,
	eventstreamrepo.Set,
	searchrepo.Set,
	triagerepo.Set,
	issuefilterreferencerepo.Set,
	labelrepo.Set,
	labelgrouprepo.Set,
	workflowstaterepo.Set,
	agentrepo.Set,
	agentproposalrepo.Set,
	agentsettingrepo.Set,
	agentthrottlerepo.Set,
	apitokenrepo.Set,
	auditrepo.Set,
	directoryrepo.Set,
	passwordhistoryrepo.Set,
	signinthrottlerepo.Set,
	breachcheckrepo.Set,
	invitationrepo.Set,
	teamrepo.Set,
	teammemberrepo.Set,
	ssoconnectionrepo.Set,
	ssoidentityrepo.Set,
	breakglassrepo.Set,
	samlrequestrepo.Set,
	samlreplayrepo.Set,
	oidcstaterepo.Set,
	scmappstaterepo.Set,
	oidcproviderrepo.Set,
	mcpthrottlerepo.Set,
	webhookrepo.Set,
	webhooksenderrepo.Set,
	importsrepo.Set,
	scmrepo.Set,

	accountsvc.Set,
	workspacesvc.Set,
	invitationsvc.Set,
	teamsvc.Set,
	issuesvc.Set,
	delegationsvc.Set,
	issuerelationsvc.Set,
	bulkoperationsvc.Set,
	checksvc.Set,
	cyclesvc.Set,
	projectsvc.Set,
	attachmentsvc.Set,
	issuecommentsvc.Set,
	notificationsvc.Set,
	savedviewsvc.Set,
	eventsvc.Set,
	searchsvc.Set,
	triagesvc.Set,
	labelsvc.Set,
	workflowstatesvc.Set,
	agentsvc.Set,
	agentholdsvc.Set,
	apitokensvc.Set,
	webhooksvc.Set,
	sessionsvc.Set,
	authorizersvc.Set,
	jobssvc.Set,
	ssoconnectionsvc.Set,
	auditsvc.Set,
	licensingsvc.Set,
	directorysvc.Set,
	importssvc.Set,
	linearsvc.Set,
	csvfilesvc.Set,
	scmsvc.Set,
	githubsvc.Set,
	giteasvc.Set,
	gitlabsvc.Set,

	dashboardhandler.Set,
	ssohandler.Set,
	blobedge.Set,
	eventsedge.Set,
	auditexportedge.Set,
	scimedge.Set,
	sourcecontroledge.Set,
	mcpserveredge.Set,
	router.Set,
	job.Set,

	NewApp,
	NewServeMux,
	NewWorker,
	NewMigrator,
	NewSeeder,
	NewJobsAdmin,
)

func InitApp(cfgFile string) (*App, func(), error) {
	wire.Build(baseSet)

	return nil, nil, nil
}

func InitWorker(cfgFile string) (*Worker, func(), error) {
	wire.Build(baseSet)

	return nil, nil, nil
}

func InitMigrator(cfgFile string) (*Migrator, func(), error) {
	wire.Build(baseSet)

	return nil, nil, nil
}

func InitSeeder(cfgFile string) (*Seeder, func(), error) {
	wire.Build(baseSet)

	return nil, nil, nil
}

func InitJobsAdmin(cfgFile string) (*JobsAdmin, func(), error) {
	wire.Build(baseSet)

	return nil, nil, nil
}
