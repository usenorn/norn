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

func (s *directoriesService) PutGroup(
	ctx context.Context,
	workspaceID uuid.UUID,
	id *uuid.UUID,
	profile service.DirectoryGroupProfile,
) (entity.DirectoryGroup, error) {
	if !s.licensed(time.Now()) {
		return entity.DirectoryGroup{}, entity.ErrDirectoryUnlicensed
	}

	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		return entity.DirectoryGroup{}, entity.ErrDirectoryGroupNotFound
	}

	log := newRecorder(workspaceID, "PutGroup")

	var saved entity.DirectoryGroup

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		teams, err := s.teams.ListVisibleTo(ctx, workspaceID, uuid.Nil, entity.TeamStatusActive, true)
		if err != nil {
			return err
		}

		group := entity.DirectoryGroup{
			WorkspaceID: workspaceID,
			ExternalID:  strings.TrimSpace(profile.ExternalID),
			DisplayName: displayName,
			TeamID:      entity.MapGroupToTeam(displayName, teams),
		}

		if id != nil && *id != uuid.Nil {
			group.ID = *id
		}

		saved, err = s.directories.SaveGroup(ctx, group)
		if err != nil {
			return err
		}

		if !saved.Mapped() {
			log.skipped(displayName, "no team matches this group by key or name")
		}

		return s.applyMembers(ctx, log, connection, saved, service.DirectoryMembershipEdit{
			Add: profile.Members,
		}, true)
	})

	s.settle(ctx, log, err)

	if err != nil {
		return entity.DirectoryGroup{}, err
	}

	return saved, nil
}

func (s *directoriesService) EditGroupMembers(
	ctx context.Context,
	workspaceID, id uuid.UUID,
	edit service.DirectoryMembershipEdit,
) (entity.DirectoryGroup, error) {
	if !s.licensed(time.Now()) {
		return entity.DirectoryGroup{}, entity.ErrDirectoryUnlicensed
	}

	log := newRecorder(workspaceID, "EditGroupMembers")

	var group entity.DirectoryGroup

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		group, err = s.directories.GetGroup(ctx, workspaceID, id)
		if err != nil {
			return err
		}

		if !group.Mapped() {
			log.skipped(group.DisplayName, "no team matches this group by key or name")
		}

		return s.applyMembers(ctx, log, connection, group, edit, false)
	})

	s.settle(ctx, log, err)

	if err != nil {
		return entity.DirectoryGroup{}, err
	}

	return group, nil
}

func (s *directoriesService) applyMembers(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	group entity.DirectoryGroup,
	edit service.DirectoryMembershipEdit,
	replace bool,
) error {
	if replace {
		existing, err := s.directories.ListGroupMembers(ctx, group.ID)
		if err != nil {
			return err
		}

		wanted := make(map[uuid.UUID]struct{}, len(edit.Add))
		for _, memberID := range edit.Add {
			wanted[memberID] = struct{}{}
		}

		for _, member := range existing {
			if _, keep := wanted[member.ID]; !keep {
				edit.Remove = append(edit.Remove, member.ID)
			}
		}
	}

	for _, memberID := range edit.Add {
		if err := s.joinGroup(ctx, log, connection, group, memberID); err != nil {
			return err
		}
	}

	for _, memberID := range edit.Remove {
		if err := s.leaveGroup(ctx, log, connection, group, memberID); err != nil {
			return err
		}
	}

	return nil
}

func (s *directoriesService) joinGroup(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	group entity.DirectoryGroup,
	directoryUserID uuid.UUID,
) error {
	member, err := s.directories.GetUser(ctx, connection.WorkspaceID, directoryUserID)
	if err != nil {
		if errors.Is(err, entity.ErrDirectoryUserNotFound) {
			log.skipped(directoryUserID.String(), "the directory does not know this user here")

			return nil
		}

		return err
	}

	if err := s.directories.AddGroupMember(ctx, group.ID, member.ID); err != nil {
		return err
	}

	if err := s.settleRole(ctx, log, connection, member); err != nil {
		return err
	}

	if !group.Mapped() {
		return nil
	}

	if _, err := s.teamMembers.Get(ctx, *group.TeamID, member.AccountID); err == nil {
		return nil
	} else if !errors.Is(err, entity.ErrTeamMembershipNotFound) {
		return err
	}

	if _, err := s.teamMembers.Create(ctx, entity.TeamMembership{
		WorkspaceID: connection.WorkspaceID,
		TeamID:      *group.TeamID,
		AccountID:   member.AccountID,
	}); err != nil {
		if errors.Is(err, entity.ErrTeamMembershipExists) {
			return nil
		}

		return err
	}

	log.note(member.UserName, entity.DirectoryTeamJoined, map[string]string{
		"group": group.DisplayName,
	})

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditTeamMemberAdded,
		ResourceKind: string(entity.ResourceTeamMembership),
		ResourceID:   member.AccountID,
		ResourceName: member.UserName,
		Detail:       map[string]string{"team_id": group.TeamID.String(), "source": "directory"},
	})

	return nil
}

func (s *directoriesService) leaveGroup(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	group entity.DirectoryGroup,
	directoryUserID uuid.UUID,
) error {
	member, err := s.directories.GetUser(ctx, connection.WorkspaceID, directoryUserID)
	if err != nil {
		if errors.Is(err, entity.ErrDirectoryUserNotFound) {
			return nil
		}

		return err
	}

	if err := s.directories.RemoveGroupMember(ctx, group.ID, member.ID); err != nil {
		return err
	}

	if err := s.settleRole(ctx, log, connection, member); err != nil {
		return err
	}

	if !group.Mapped() {
		return nil
	}

	if err := s.teamMembers.Delete(ctx, *group.TeamID, member.AccountID); err != nil {
		if errors.Is(err, entity.ErrTeamMembershipNotFound) {
			return nil
		}

		return err
	}

	log.note(member.UserName, entity.DirectoryTeamLeft, map[string]string{
		"group": group.DisplayName,
	})

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditTeamMemberRemoved,
		ResourceKind: string(entity.ResourceTeamMembership),
		ResourceID:   member.AccountID,
		ResourceName: member.UserName,
		Detail:       map[string]string{"team_id": group.TeamID.String(), "source": "directory"},
	})

	return nil
}

func (s *directoriesService) settleRole(
	ctx context.Context,
	log *recorder,
	connection entity.DirectoryConnection,
	member entity.DirectoryUser,
) error {
	if strings.TrimSpace(connection.AdminGroup) == "" {
		return nil
	}

	groups, err := s.directories.ListGroupsForUser(ctx, member.ID)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.DisplayName)
	}

	wanted := connection.RoleFor(names)

	membership, err := s.memberships.Get(ctx, connection.WorkspaceID, member.AccountID)
	if err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return nil
		}

		return err
	}

	if membership.Role == wanted {
		return nil
	}

	if membership.Role == entity.MembershipRoleAdmin {
		if err := s.guardLastAdmin(ctx, connection.WorkspaceID, member.AccountID); err != nil {
			log.refuse(member.UserName, "this is the last administrator of the workspace")

			return err
		}
	}

	if _, err := s.memberships.UpdateRole(
		ctx, connection.WorkspaceID, member.AccountID, wanted,
	); err != nil {
		return err
	}

	log.note(member.UserName, entity.DirectoryRoleChanged, map[string]string{
		"role": string(wanted),
	})

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditMemberRoleChanged,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   member.AccountID,
		ResourceName: member.UserName,
		Detail:       map[string]string{"role": string(wanted), "source": "directory"},
	})

	return nil
}

func (s *directoriesService) GetGroup(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (entity.DirectoryGroup, error) {
	if !s.licensed(time.Now()) {
		return entity.DirectoryGroup{}, entity.ErrDirectoryUnlicensed
	}

	return s.directories.GetGroup(ctx, workspaceID, id)
}

func (s *directoriesService) ListGroups(
	ctx context.Context,
	workspaceID uuid.UUID,
	displayName string,
	limit int,
) ([]entity.DirectoryGroup, error) {
	if !s.licensed(time.Now()) {
		return nil, entity.ErrDirectoryUnlicensed
	}

	if limit <= 0 || limit > entity.DirectoryPageMaxSize {
		limit = entity.DirectoryPageMaxSize
	}

	return s.directories.ListGroups(ctx, workspaceID, displayName, limit)
}

func (s *directoriesService) ListGroupMembers(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) ([]entity.DirectoryUser, error) {
	if !s.licensed(time.Now()) {
		return nil, entity.ErrDirectoryUnlicensed
	}

	group, err := s.directories.GetGroup(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	return s.directories.ListGroupMembers(ctx, group.ID)
}

func (s *directoriesService) DeleteGroup(ctx context.Context, workspaceID, id uuid.UUID) error {
	if !s.licensed(time.Now()) {
		return entity.ErrDirectoryUnlicensed
	}

	log := newRecorder(workspaceID, "DeleteGroup")

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		connection, err := s.directories.GetConnection(ctx, workspaceID)
		if err != nil {
			return err
		}

		group, err := s.directories.GetGroup(ctx, workspaceID, id)
		if err != nil {
			return err
		}

		members, err := s.directories.ListGroupMembers(ctx, group.ID)
		if err != nil {
			return err
		}

		for _, member := range members {
			if err := s.leaveGroup(ctx, log, connection, group, member.ID); err != nil {
				return err
			}
		}

		return s.directories.DeleteGroup(ctx, workspaceID, id)
	})

	s.settle(ctx, log, err)

	return err
}
