//go:build wireinject

package internal

import (
	"github.com/goforj/wire"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/router"
	dashboardhandler "github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/handler/job"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/authz"
	"github.com/usenorn/norn/internal/pkg/geoip"
	"github.com/usenorn/norn/internal/pkg/objectstore"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/pkg/pwned"
	"github.com/usenorn/norn/internal/pkg/smtp"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
	"github.com/usenorn/norn/internal/pkg/valkey"
	"github.com/usenorn/norn/internal/repository"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	breachcheckrepo "github.com/usenorn/norn/internal/repository/breachcheck"
	emailchangerepo "github.com/usenorn/norn/internal/repository/emailchange"
	geolocationrepo "github.com/usenorn/norn/internal/repository/geolocation"
	invitationrepo "github.com/usenorn/norn/internal/repository/invitation"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issueactivityrepo "github.com/usenorn/norn/internal/repository/issueactivity"
	issuerelationrepo "github.com/usenorn/norn/internal/repository/issuerelation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	labelgrouprepo "github.com/usenorn/norn/internal/repository/labelgroup"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	passwordhistoryrepo "github.com/usenorn/norn/internal/repository/passwordhistory"
	passwordresetrepo "github.com/usenorn/norn/internal/repository/passwordreset"
	sessionrepo "github.com/usenorn/norn/internal/repository/session"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	signuprepo "github.com/usenorn/norn/internal/repository/signup"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	workspaceauthpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	apitokensvc "github.com/usenorn/norn/internal/service/apitoken"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	jobssvc "github.com/usenorn/norn/internal/service/jobs"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
	teamsvc "github.com/usenorn/norn/internal/service/team"
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
	objectstore.Set,
	authz.Set,
	geoip.Set,
	pwned.Set,

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
	issueactivityrepo.Set,
	issuerelationrepo.Set,
	labelrepo.Set,
	labelgrouprepo.Set,
	workflowstaterepo.Set,
	apitokenrepo.Set,
	passwordhistoryrepo.Set,
	signinthrottlerepo.Set,
	breachcheckrepo.Set,
	invitationrepo.Set,
	teamrepo.Set,
	teammemberrepo.Set,

	accountsvc.Set,
	workspacesvc.Set,
	invitationsvc.Set,
	teamsvc.Set,
	issuesvc.Set,
	issuerelationsvc.Set,
	labelsvc.Set,
	workflowstatesvc.Set,
	apitokensvc.Set,
	sessionsvc.Set,
	authorizersvc.Set,
	jobssvc.Set,

	dashboardhandler.Set,
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
