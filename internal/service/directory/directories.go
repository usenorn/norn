package directory

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type directoriesService struct {
	directories repository.Directory
	runs        repository.DirectorySync
	memberships repository.Membership
	accounts    repository.Account
	workspaces  repository.Workspace
	teams       repository.Team
	teamMembers repository.TeamMember
	issues      repository.Issue
	projects    repository.Project
	authorizer  service.Authorizer
	audit       service.Audit
	transactor  repository.Transactor
	licensing   service.Licensing
}

func New(
	directories repository.Directory,
	runs repository.DirectorySync,
	memberships repository.Membership,
	accounts repository.Account,
	workspaces repository.Workspace,
	teams repository.Team,
	teamMembers repository.TeamMember,
	issues repository.Issue,
	projects repository.Project,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	licensing service.Licensing,
) service.Directories {
	return &directoriesService{
		directories: directories,
		runs:        runs,
		memberships: memberships,
		accounts:    accounts,
		workspaces:  workspaces,
		teams:       teams,
		teamMembers: teamMembers,
		issues:      issues,
		projects:    projects,
		authorizer:  authorizer,
		audit:       audit,
		transactor:  transactor,
		licensing:   licensing,
	}
}

func (s *directoriesService) Availability(_ context.Context) service.DirectoryAvailability {
	return service.DirectoryAvailability{Available: s.licensed() == nil}
}

func (s *directoriesService) licensed() error {
	return s.licensing.Permits(entity.FeatureDirectory)
}

func (s *directoriesService) Authenticate(
	ctx context.Context,
	token string,
) (entity.DirectoryConnection, error) {
	if err := s.licensed(); err != nil {
		return entity.DirectoryConnection{}, err
	}

	if !entity.LooksLikeDirectoryToken(token) {
		return entity.DirectoryConnection{}, entity.ErrDirectoryTokenInvalid
	}

	connection, err := s.directories.GetConnectionByToken(ctx, entity.HashDirectoryToken(token))
	if err != nil {
		if errors.Is(err, entity.ErrDirectoryNotConnected) {
			return entity.DirectoryConnection{}, entity.ErrDirectoryTokenInvalid
		}

		return entity.DirectoryConnection{}, err
	}

	if !connection.Enabled {
		return entity.DirectoryConnection{}, entity.ErrDirectoryDisabled
	}

	return connection, nil
}

type recorder struct {
	run     entity.DirectorySyncRun
	changes []entity.DirectorySyncChange
}

func newRecorder(workspaceID uuid.UUID, operation string) *recorder {
	return &recorder{
		run: entity.DirectorySyncRun{
			WorkspaceID: workspaceID,
			Trigger:     entity.DirectorySyncByProvider,
			Operation:   operation,
			StartedAt:   time.Now().UTC(),
		},
	}
}

func (r *recorder) note(subject string, kind entity.DirectoryChangeKind, detail map[string]string) {
	r.changes = append(r.changes, entity.DirectorySyncChange{
		Subject: subject,
		Kind:    kind,
		Outcome: entity.DirectorySyncSucceeded,
		Detail:  detail,
	})
}

func (r *recorder) refuse(subject, reason string) {
	r.changes = append(r.changes, entity.DirectorySyncChange{
		Subject: subject,
		Kind:    entity.DirectoryRefused,
		Outcome: entity.DirectorySyncRefused,
		Detail:  map[string]string{"reason": reason},
	})
}

func (r *recorder) skipped(subject, reason string) {
	r.changes = append(r.changes, entity.DirectorySyncChange{
		Subject: subject,
		Kind:    entity.DirectoryGroupSkipped,
		Outcome: entity.DirectorySyncSucceeded,
		Detail:  map[string]string{"reason": reason},
	})
}

func (s *directoriesService) settle(ctx context.Context, log *recorder, cause error) {
	log.run.FinishedAt = time.Now().UTC()
	log.run.Outcome = entity.DirectorySyncSucceeded

	switch {
	case cause == nil:
	case refusal(cause):
		log.run.Outcome = entity.DirectorySyncRefused
		log.run.Detail = cause.Error()
	default:
		log.run.Outcome = entity.DirectorySyncFailed
		log.run.Detail = cause.Error()
	}

	log.run.Changes = log.changes

	if _, err := s.runs.RecordRun(ctx, log.run); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"directory sync log write failed",
			"workspace_id", log.run.WorkspaceID.String(),
			"operation", log.run.Operation,
			"error", err.Error(),
		)
	}

	if cause == nil {
		if err := s.directories.StampSync(ctx, log.run.WorkspaceID); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"stamping the directory sync time failed",
				"workspace_id", log.run.WorkspaceID.String(),
				"error", err.Error(),
			)
		}
	}
}

func refusal(err error) bool {
	var lastAdmin entity.LastWorkspaceAdminError

	return errors.As(err, &lastAdmin) ||
		errors.Is(err, entity.ErrDirectoryPatchUnsupported) ||
		errors.Is(err, entity.ErrDirectoryUserNameRequired) ||
		errors.Is(err, entity.ErrDirectoryUserExists) ||
		errors.Is(err, entity.ErrAccountDeactivated)
}

func (s *directoriesService) guardLastAdmin(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
		return err
	}

	held, err := s.memberships.ListWorkspaceIDsWithoutOtherActiveAdmin(ctx, accountID)
	if err != nil {
		return err
	}

	if slices.Contains(held, workspaceID) {
		return entity.LastWorkspaceAdminError{WorkspaceIDs: []uuid.UUID{workspaceID}}
	}

	return nil
}

func (s *directoriesService) workload(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) entity.DirectoryWorkload {
	var carried entity.DirectoryWorkload

	scope, err := s.workspaceScope(ctx, workspaceID)
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "reading the workspace teams failed", "error", err.Error())

		return carried
	}

	filter := entity.AssignedToFilter(accountID)

	assigned, err := s.issues.ListVisible(ctx, scope, entity.IssuePage{
		Limit:    entity.DirectoryWorkloadMax,
		Statuses: []entity.IssueStatus{entity.IssueStatusActive},
		Filter:   &filter,
	}.Normalized())
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "reading assigned issues failed", "error", err.Error())
	}

	for _, issue := range assigned {
		carried.Issues = append(carried.Issues, issue.Reference())
	}

	led, err := s.projects.ListByWorkspaceID(ctx, workspaceID, repository.ProjectFilter{
		ForAccountID: &accountID,
	})
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "reading led projects failed", "error", err.Error())
	}

	for _, project := range led {
		if project.LeadAccountID == accountID {
			carried.Projects = append(carried.Projects, project.Name)
		}
	}

	teams, err := s.teams.ListByWorkspaceMember(ctx, workspaceID, accountID)
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "reading team membership failed", "error", err.Error())

		return carried
	}

	for _, team := range teams {
		carried.Teams = append(carried.Teams, team.Key)
	}

	return carried
}

func (s *directoriesService) workspaceScope(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.TeamScope, error) {
	teams, err := s.teams.ListVisibleTo(ctx, workspaceID, uuid.Nil, "", true)
	if err != nil {
		return entity.TeamScope{}, err
	}

	scope := entity.TeamScope{
		WorkspaceID:    workspaceID,
		IncludePrivate: true,
		TeamIDs:        make([]uuid.UUID, 0, len(teams)),
	}

	for _, team := range teams {
		scope.TeamIDs = append(scope.TeamIDs, team.ID)
	}

	return scope, nil
}
