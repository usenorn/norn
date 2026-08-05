package directory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	directoryrepo "github.com/usenorn/norn/internal/repository/directory"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	directorysvc "github.com/usenorn/norn/internal/service/directory"
)

type harness struct {
	directories *directoryrepo.MockDirectory
	runs        *directoryrepo.MockDirectorySync
	memberships *membershiprepo.MockMembership
	accounts    *accountrepo.MockAccount
	workspaces  *workspacerepo.MockWorkspace
	teams       *teamrepo.MockTeam
	teamMembers *teammemberrepo.MockTeamMember
	issues      *issuerepo.MockIssue
	projects    *projectrepo.MockProject
	service     service.Directories
	recorded    []entity.DirectorySyncRun
}

func licensed() entity.Licence {
	return entity.Licence{
		Holder:    "Northwind Studio",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Features:  entity.LicenceFeatures{Directory: true},
	}
}

func newHarness(t *testing.T, licence entity.Licence) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		directories: directoryrepo.NewMockDirectory(ctrl),
		runs:        directoryrepo.NewMockDirectorySync(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		workspaces:  workspacerepo.NewMockWorkspace(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		teamMembers: teammemberrepo.NewMockTeamMember(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		projects:    projectrepo.NewMockProject(ctrl),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	recorder := auditsvc.NewMockAudit(ctrl)
	recorder.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	h.runs.EXPECT().RecordRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, run entity.DirectorySyncRun) (entity.DirectorySyncRun, error) {
			h.recorded = append(h.recorded, run)

			return run, nil
		}).
		AnyTimes()

	h.directories.EXPECT().StampSync(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.service = directorysvc.New(
		h.directories,
		h.runs,
		h.memberships,
		h.accounts,
		h.workspaces,
		h.teams,
		h.teamMembers,
		h.issues,
		h.projects,
		authorizersvc.NewMockAuthorizer(ctrl),
		recorder,
		transactor,
		licence,
	)

	return h
}

func (h *harness) connection(workspaceID uuid.UUID, adminGroup string) entity.DirectoryConnection {
	connection := entity.DirectoryConnection{
		WorkspaceID: workspaceID,
		Enabled:     true,
		OnUnknown:   entity.DefaultDirectoryUnknownPolicy,
		OnAbsent:    entity.DefaultDirectoryAbsentPolicy,
		AdminGroup:  adminGroup,
	}

	h.directories.EXPECT().GetConnection(gomock.Any(), workspaceID).Return(connection, nil).AnyTimes()

	return connection
}

func (h *harness) expectNoWorkload(workspaceID uuid.UUID) {
	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), workspaceID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	h.issues.EXPECT().ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	h.projects.EXPECT().
		ListByWorkspaceID(gomock.Any(), workspaceID, gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), workspaceID, gomock.Any()).
		Return(nil, nil).
		AnyTimes()
}

func (h *harness) changes() []entity.DirectorySyncChange {
	var all []entity.DirectorySyncChange

	for _, run := range h.recorded {
		all = append(all, run.Changes...)
	}

	return all
}
