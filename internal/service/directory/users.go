package directory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *directoriesService) PutUser(
	ctx context.Context,
	workspaceID uuid.UUID,
	id *uuid.UUID,
	profile service.DirectoryProfile,
) (service.DirectoryUserView, error) {
	if err := s.licensed(); err != nil {
		return service.DirectoryUserView{}, err
	}

	userName := entity.NormalizeDirectoryUserName(profile.UserName)
	if userName == "" {
		return service.DirectoryUserView{}, entity.ErrDirectoryUserNameRequired
	}

	log := newRecorder(workspaceID, "PutUser")

	var view service.DirectoryUserView

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		stored, err := s.existingUser(ctx, workspaceID, id, userName)
		if err != nil {
			return err
		}

		if stored.ID != uuid.Nil {
			view, err = s.reconcile(ctx, log, connection, stored, profile)

			return err
		}

		if connection.OnUnknown == entity.DirectoryIgnoreUnknown {
			log.skipped(userName, "unknown users are ignored on this connection")

			return entity.ErrDirectoryUserNotFound
		}

		view, err = s.admit(ctx, log, connection, userName, profile)

		return err
	})

	s.settle(ctx, log, err)

	if err != nil {
		return service.DirectoryUserView{}, err
	}

	return view, nil
}

func (s *directoriesService) existingUser(
	ctx context.Context,
	workspaceID uuid.UUID,
	id *uuid.UUID,
	userName string,
) (entity.DirectoryUser, error) {
	if id != nil && *id != uuid.Nil {
		stored, err := s.directories.GetUser(ctx, workspaceID, *id)
		if err != nil {
			if errors.Is(err, entity.ErrDirectoryUserNotFound) {
				return entity.DirectoryUser{}, err
			}

			return entity.DirectoryUser{}, err
		}

		return stored, nil
	}

	stored, err := s.directories.GetUserByName(ctx, workspaceID, userName)
	if err != nil {
		if errors.Is(err, entity.ErrDirectoryUserNotFound) {
			return entity.DirectoryUser{}, nil
		}

		return entity.DirectoryUser{}, err
	}

	return stored, nil
}

func (s *directoriesService) admit(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	userName string,
	profile service.DirectoryProfile,
) (service.DirectoryUserView, error) {
	account, err := s.accountFor(ctx, userName, profile.DisplayName)
	if err != nil {
		return service.DirectoryUserView{}, err
	}

	adopted := false

	role := connection.RoleFor(nil)

	membership, err := s.memberships.Get(ctx, connection.WorkspaceID, account.ID)
	switch {
	case err == nil:
		if !membership.Source.Managed() {
			membership, err = s.memberships.SetSource(
				ctx, connection.WorkspaceID, account.ID, entity.MembershipSourceDirectory,
			)
			if err != nil {
				return service.DirectoryUserView{}, err
			}

			adopted = true
		}

		if membership.Deactivated() {
			membership, err = s.memberships.SetDeactivated(
				ctx, connection.WorkspaceID, account.ID, nil,
			)
			if err != nil {
				return service.DirectoryUserView{}, err
			}
		}

		role = membership.Role

	case errors.Is(err, entity.ErrMembershipNotFound):
		membership, err = s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: connection.WorkspaceID,
			AccountID:   account.ID,
			Role:        role,
			Source:      entity.MembershipSourceDirectory,
		})
		if err != nil {
			return service.DirectoryUserView{}, err
		}

	default:
		return service.DirectoryUserView{}, err
	}

	user, err := s.directories.SaveUser(ctx, entity.DirectoryUser{
		WorkspaceID: connection.WorkspaceID,
		AccountID:   account.ID,
		ExternalID:  strings.TrimSpace(profile.ExternalID),
		UserName:    userName,
		DisplayName: strings.TrimSpace(profile.DisplayName),
		Active:      true,
	})
	if err != nil {
		return service.DirectoryUserView{}, err
	}

	kind := entity.DirectoryProvisioned
	if adopted {
		kind = entity.DirectoryAdopted
	}

	log.note(userName, kind, map[string]string{"role": string(role)})

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditMemberAdded,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   account.ID,
		ResourceName: account.DisplayName,
		Detail:       map[string]string{"role": string(role), "source": "directory"},
	})

	if err := s.emit(ctx, entity.WebhookMembershipAdded, membership); err != nil {
		return service.DirectoryUserView{}, err
	}

	return service.DirectoryUserView{User: user}, nil
}

func (s *directoriesService) accountFor(
	ctx context.Context,
	userName, displayName string,
) (entity.Account, error) {
	account, err := s.accounts.GetByEmail(ctx, userName)
	if err == nil {
		if account.Status != entity.AccountStatusActive {
			return entity.Account{}, entity.ErrAccountDeactivated
		}

		return account, nil
	}

	if !errors.Is(err, entity.ErrAccountNotFound) {
		return entity.Account{}, err
	}

	name := strings.TrimSpace(displayName)
	if name == "" {
		local, _, _ := strings.Cut(userName, "@")
		name = local
	}

	created, err := s.accounts.Create(ctx, entity.Account{
		Status:      entity.AccountStatusActive,
		Kind:        entity.AccountKindPerson,
		Email:       userName,
		DisplayName: name,
		Timezone:    entity.DefaultTimezone,
	})
	if err != nil {
		return entity.Account{}, err
	}

	return created, nil
}

func (s *directoriesService) reconcile(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	stored entity.DirectoryUser,
	profile service.DirectoryProfile,
) (service.DirectoryUserView, error) {
	if !profile.Active {
		return s.retire(ctx, log, connection, stored)
	}

	membership, err := s.memberships.Get(ctx, connection.WorkspaceID, stored.AccountID)
	if err != nil && !errors.Is(err, entity.ErrMembershipNotFound) {
		return service.DirectoryUserView{}, err
	}

	if errors.Is(err, entity.ErrMembershipNotFound) {
		created, err := s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: connection.WorkspaceID,
			AccountID:   stored.AccountID,
			Role:        connection.RoleFor(nil),
			Source:      entity.MembershipSourceDirectory,
		})
		if err != nil {
			return service.DirectoryUserView{}, err
		}

		if err := s.emit(ctx, entity.WebhookMembershipAdded, created); err != nil {
			return service.DirectoryUserView{}, err
		}
	} else if membership.Deactivated() {
		reactivated, err := s.memberships.SetDeactivated(
			ctx, connection.WorkspaceID, stored.AccountID, nil,
		)
		if err != nil {
			return service.DirectoryUserView{}, err
		}

		log.note(stored.UserName, entity.DirectoryReactivated, nil)

		if err := s.emit(ctx, entity.WebhookMembershipChanged, reactivated); err != nil {
			return service.DirectoryUserView{}, err
		}
	}

	userName := entity.NormalizeDirectoryUserName(profile.UserName)

	updated, err := s.directories.SaveUser(ctx, entity.DirectoryUser{
		ID:          stored.ID,
		WorkspaceID: stored.WorkspaceID,
		AccountID:   stored.AccountID,
		ExternalID:  strings.TrimSpace(profile.ExternalID),
		UserName:    userName,
		DisplayName: strings.TrimSpace(profile.DisplayName),
		Active:      true,
	})
	if err != nil {
		return service.DirectoryUserView{}, err
	}

	log.note(userName, entity.DirectoryUpdated, nil)

	return service.DirectoryUserView{User: updated}, nil
}

func (s *directoriesService) SetUserActive(
	ctx context.Context,
	workspaceID, id uuid.UUID,
	active bool,
) (service.DirectoryUserView, error) {
	if err := s.licensed(); err != nil {
		return service.DirectoryUserView{}, err
	}

	log := newRecorder(workspaceID, "SetUserActive")

	var view service.DirectoryUserView

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		stored, err := s.directories.GetUser(ctx, workspaceID, id)
		if err != nil {
			return err
		}

		if active {
			view, err = s.reconcile(ctx, log, connection, stored, service.DirectoryProfile{
				ExternalID:  stored.ExternalID,
				UserName:    stored.UserName,
				DisplayName: stored.DisplayName,
				Active:      true,
			})

			return err
		}

		view, err = s.retire(ctx, log, connection, stored)

		return err
	})

	s.settle(ctx, log, err)

	if err != nil {
		return service.DirectoryUserView{}, err
	}

	return view, nil
}

func (s *directoriesService) DeleteUser(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (service.DirectoryUserView, error) {
	if err := s.licensed(); err != nil {
		return service.DirectoryUserView{}, err
	}

	log := newRecorder(workspaceID, "DeleteUser")

	var view service.DirectoryUserView

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		stored, err := s.directories.GetUser(ctx, workspaceID, id)
		if err != nil {
			return err
		}

		view, err = s.retire(ctx, log, connection, stored)

		return err
	})

	s.settle(ctx, log, err)

	if err != nil {
		return service.DirectoryUserView{}, err
	}

	return view, nil
}

func (s *directoriesService) retire(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	stored entity.DirectoryUser,
) (service.DirectoryUserView, error) {
	carried := s.workload(ctx, connection.WorkspaceID, stored.AccountID)

	if connection.OnAbsent == entity.DirectoryFlagAbsent {
		log.note(stored.UserName, entity.DirectoryGroupSkipped, map[string]string{
			"reason":   "this connection flags departures instead of deactivating them",
			"issues":   strings.Join(carried.Issues, ", "),
			"projects": strings.Join(carried.Projects, ", "),
		})

		return service.DirectoryUserView{User: stored, Workload: carried}, nil
	}

	membership, err := s.memberships.Get(ctx, connection.WorkspaceID, stored.AccountID)
	if err != nil && !errors.Is(err, entity.ErrMembershipNotFound) {
		return service.DirectoryUserView{}, err
	}

	if err == nil && !membership.Deactivated() {
		if membership.Role == entity.MembershipRoleAdmin {
			if err := s.guardLastAdmin(ctx, connection.WorkspaceID, stored.AccountID); err != nil {
				log.refuse(stored.UserName, "this is the last administrator of the workspace")

				return service.DirectoryUserView{}, err
			}
		}

		stoppedAt := time.Now().UTC()

		stopped, err := s.memberships.SetDeactivated(
			ctx, connection.WorkspaceID, stored.AccountID, &stoppedAt,
		)
		if err != nil {
			return service.DirectoryUserView{}, err
		}

		if err := s.emit(ctx, entity.WebhookMembershipRemoved, stopped); err != nil {
			return service.DirectoryUserView{}, err
		}
	}

	retired, err := s.directories.SaveUser(ctx, entity.DirectoryUser{
		ID:          stored.ID,
		WorkspaceID: stored.WorkspaceID,
		AccountID:   stored.AccountID,
		ExternalID:  stored.ExternalID,
		UserName:    stored.UserName,
		DisplayName: stored.DisplayName,
		Active:      false,
	})
	if err != nil {
		return service.DirectoryUserView{}, err
	}

	log.note(stored.UserName, entity.DirectoryDeactivated, map[string]string{
		"issues":   strings.Join(carried.Issues, ", "),
		"projects": strings.Join(carried.Projects, ", "),
		"teams":    strings.Join(carried.Teams, ", "),
	})

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditMemberRemoved,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   stored.AccountID,
		ResourceName: stored.UserName,
		Detail: map[string]string{
			"source": "directory",
			"issues": strings.Join(carried.Issues, ", "),
		},
	})

	return service.DirectoryUserView{User: retired, Workload: carried}, nil
}

func (s *directoriesService) GetUser(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (entity.DirectoryUser, error) {
	if err := s.licensed(); err != nil {
		return entity.DirectoryUser{}, err
	}

	return s.directories.GetUser(ctx, workspaceID, id)
}

func (s *directoriesService) ListUsers(
	ctx context.Context,
	workspaceID uuid.UUID,
	userName string,
	limit int,
) ([]entity.DirectoryUser, error) {
	if err := s.licensed(); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > entity.DirectoryPageMaxSize {
		limit = entity.DirectoryPageMaxSize
	}

	return s.directories.ListUsers(ctx, workspaceID, userName, limit)
}
