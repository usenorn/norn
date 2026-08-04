package invitation

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type invitationsService struct {
	invitations  repository.Invitation
	memberships  repository.Membership
	workspaces   repository.Workspace
	accounts     repository.Account
	teams        repository.Team
	teamMembers  repository.TeamMember
	authPolicies repository.WorkspaceAuthPolicy
	producer     repository.JobProducer
	mailer       repository.Mailer
	transactor   repository.Transactor
	authorizer   service.Authorizer
	registration service.Accounts
	sessions     service.Sessions
	app          config.App
	smtp         config.SMTP
}

func New(
	invitations repository.Invitation,
	memberships repository.Membership,
	workspaces repository.Workspace,
	accounts repository.Account,
	teams repository.Team,
	teamMembers repository.TeamMember,
	authPolicies repository.WorkspaceAuthPolicy,
	producer repository.JobProducer,
	mailer repository.Mailer,
	transactor repository.Transactor,
	authorizer service.Authorizer,
	registration service.Accounts,
	sessions service.Sessions,
	app config.App,
	smtp config.SMTP,
) service.Invitations {
	return &invitationsService{
		invitations:  invitations,
		memberships:  memberships,
		workspaces:   workspaces,
		accounts:     accounts,
		teams:        teams,
		teamMembers:  teamMembers,
		authPolicies: authPolicies,
		producer:     producer,
		mailer:       mailer,
		transactor:   transactor,
		authorizer:   authorizer,
		registration: registration,
		sessions:     sessions,
		app:          app,
		smtp:         smtp,
	}
}

func (s *invitationsService) Create(ctx context.Context, input service.CreateInvitationsInput) ([]service.InvitationResult, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceInvitation,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
	}); err != nil {
		return nil, err
	}

	if err := validateRoles(input.Recipients); err != nil {
		return nil, err
	}

	if err := s.validateTeams(ctx, input.WorkspaceID, input.Recipients); err != nil {
		return nil, err
	}

	actor, _ := identity.From(ctx)
	delivery := s.initialDelivery()
	results := make([]service.InvitationResult, 0, len(input.Recipients))

	for _, recipient := range input.Recipients {
		result, token, err := s.invite(ctx, input.WorkspaceID, actor, recipient, delivery)
		if err != nil {
			return nil, err
		}

		if token != "" {
			if err := s.producer.EnqueueInvitation(ctx, entity.InvitationPayload{
				InvitationID: result.Invitation.ID,
				Token:        token,
			}); err != nil {
				return nil, err
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (s *invitationsService) invite(
	ctx context.Context,
	workspaceID, actor uuid.UUID,
	recipient service.InvitationRecipient,
	delivery entity.InvitationDelivery,
) (service.InvitationResult, string, error) {
	email := entity.NormalizeEmail(recipient.Email)

	if entity.ValidateEmail("email", email).Code != "" {
		return service.InvitationResult{
			Email:   recipient.Email,
			Outcome: entity.InvitationOutcomeInvalidEmail,
		}, "", nil
	}

	member, err := s.alreadyMember(ctx, workspaceID, email)
	if err != nil {
		return service.InvitationResult{}, "", err
	}

	if member {
		return service.InvitationResult{
			Email:   email,
			Outcome: entity.InvitationOutcomeAlreadyMember,
		}, "", nil
	}

	token, tokenHash, err := entity.NewInvitationToken()
	if err != nil {
		return service.InvitationResult{}, "", err
	}

	now := time.Now().UTC()

	invitation := entity.Invitation{
		WorkspaceID: workspaceID,
		Email:       email,
		Role:        recipient.Role,
		TeamIDs:     recipient.TeamIDs,
		Status:      entity.InvitationStatusPending,
		Delivery:    delivery,
		TokenHash:   tokenHash,
		InvitedAt:   now,
		ExpiresAt:   now.Add(entity.InvitationTokenTTL),
	}

	if actor != uuid.Nil {
		invitation.InvitedByAccountID = &actor
	}

	var created entity.Invitation

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.invitations.RevokePendingByEmail(ctx, workspaceID, email, now); err != nil {
			return err
		}

		stored, err := s.invitations.Create(ctx, invitation)
		if err != nil {
			return err
		}

		created = stored

		return nil
	})
	if err != nil {
		return service.InvitationResult{}, "", err
	}

	result := service.InvitationResult{
		Email:      email,
		Outcome:    entity.InvitationOutcomeCreated,
		Invitation: created,
		URL:        s.invitationURL(token),
	}

	if delivery == entity.InvitationDeliveryLinkOnly {
		return result, "", nil
	}

	return result, token, nil
}

func (s *invitationsService) alreadyMember(ctx context.Context, workspaceID uuid.UUID, email string) (bool, error) {
	account, err := s.accounts.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return false, nil
		}

		return false, err
	}

	if _, err := s.memberships.Get(ctx, workspaceID, account.ID); err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *invitationsService) List(ctx context.Context, workspaceID uuid.UUID, status entity.InvitationStatus) ([]entity.Invitation, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceInvitation,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return nil, err
	}

	if status != "" && !status.Valid() {
		return nil, entity.NewValidationError(entity.FieldError{
			Field: "status",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return s.invitations.ListByWorkspaceID(ctx, workspaceID, status)
}

func (s *invitationsService) Resend(ctx context.Context, workspaceID, invitationID uuid.UUID) (service.IssuedInvitation, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceInvitation,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	}); err != nil {
		return service.IssuedInvitation{}, err
	}

	invitation, err := s.scopedInvitation(ctx, workspaceID, invitationID)
	if err != nil {
		return service.IssuedInvitation{}, err
	}

	switch {
	case invitation.Revoked():
		return service.IssuedInvitation{}, entity.ErrInvitationRevoked
	case invitation.Accepted():
		return service.IssuedInvitation{}, entity.ErrInvitationAccepted
	}

	token, tokenHash, err := entity.NewInvitationToken()
	if err != nil {
		return service.IssuedInvitation{}, err
	}

	delivery := s.initialDelivery()
	expiresAt := time.Now().UTC().Add(entity.InvitationTokenTTL)

	refreshed, err := s.invitations.Refresh(ctx, invitation.ID, tokenHash, expiresAt, delivery)
	if err != nil {
		return service.IssuedInvitation{}, err
	}

	if delivery != entity.InvitationDeliveryLinkOnly {
		if err := s.producer.EnqueueInvitation(ctx, entity.InvitationPayload{
			InvitationID: refreshed.ID,
			Token:        token,
		}); err != nil {
			return service.IssuedInvitation{}, err
		}
	}

	return service.IssuedInvitation{Invitation: refreshed, URL: s.invitationURL(token)}, nil
}

func (s *invitationsService) Revoke(ctx context.Context, workspaceID, invitationID uuid.UUID) error {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceInvitation,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	}); err != nil {
		return err
	}

	invitation, err := s.scopedInvitation(ctx, workspaceID, invitationID)
	if err != nil {
		return err
	}

	if invitation.Accepted() {
		return entity.ErrInvitationAccepted
	}

	return s.invitations.MarkRevoked(ctx, invitation.ID, time.Now().UTC())
}

func (s *invitationsService) Preview(ctx context.Context, token string) (service.InvitationPreview, error) {
	invitation, err := s.usableInvitation(ctx, token)
	if err != nil {
		return service.InvitationPreview{}, err
	}

	workspace, err := s.workspaces.GetByID(ctx, invitation.WorkspaceID)
	if err != nil {
		return service.InvitationPreview{}, err
	}

	accountExists := true

	if _, err := s.accounts.GetByEmail(ctx, invitation.Email); err != nil {
		if !errors.Is(err, entity.ErrAccountNotFound) {
			return service.InvitationPreview{}, err
		}

		accountExists = false
	}

	policy, err := s.authPolicies.Get(ctx, invitation.WorkspaceID)
	if err != nil {
		return service.InvitationPreview{}, err
	}

	return service.InvitationPreview{
		Workspace:     workspace,
		Email:         invitation.Email,
		Role:          invitation.Role,
		ExpiresAt:     invitation.ExpiresAt,
		AccountExists: accountExists,
		SSOEnforced:   policy.Enforcement == entity.AuthEnforcementSSO,
	}, nil
}

func (s *invitationsService) Accept(ctx context.Context, input service.AcceptInvitationInput) (service.AcceptedInvitation, error) {
	invitation, err := s.usableInvitation(ctx, input.Token)
	if err != nil {
		return service.AcceptedInvitation{}, err
	}

	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceInvitation,
		Action:      entity.ActionRead,
		WorkspaceID: invitation.WorkspaceID,
		Joining:     true,
	})
	if err != nil {
		return service.AcceptedInvitation{}, err
	}

	workspace := decision.Workspace

	if actor, ok := identity.From(ctx); ok {
		membership, err := s.joinAsActor(ctx, invitation, workspace, actor)
		if err != nil {
			return service.AcceptedInvitation{}, err
		}

		return service.AcceptedInvitation{Workspace: workspace, Membership: membership}, nil
	}

	if _, err := s.accounts.GetByEmail(ctx, invitation.Email); err == nil {
		return service.AcceptedInvitation{}, entity.ErrAccountEmailTaken
	} else if !errors.Is(err, entity.ErrAccountNotFound) {
		return service.AcceptedInvitation{}, err
	}

	account, err := s.registration.Register(ctx, service.RegisterAccountInput{
		Email:       invitation.Email,
		DisplayName: input.DisplayName,
		Timezone:    input.Timezone,
		Password:    input.Password,
	})
	if err != nil {
		return service.AcceptedInvitation{}, err
	}

	issued, err := s.sessions.SignIn(ctx, service.SignInInput{
		Email:    invitation.Email,
		Password: input.Password,
		Client:   input.Client,
	})
	if err != nil {
		return service.AcceptedInvitation{}, err
	}

	membership, err := s.join(identity.Into(ctx, account.ID), invitation, workspace, account.ID)
	if err != nil {
		return service.AcceptedInvitation{}, err
	}

	return service.AcceptedInvitation{
		Workspace:  workspace,
		Membership: membership,
		Session:    issued,
		SignedIn:   true,
	}, nil
}

func (s *invitationsService) joinAsActor(
	ctx context.Context,
	invitation entity.Invitation,
	workspace entity.Workspace,
	actor uuid.UUID,
) (entity.Membership, error) {
	account, err := s.accounts.GetByID(ctx, actor)
	if err != nil {
		return entity.Membership{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.Membership{}, entity.ErrAccountDeactivated
	}

	if entity.NormalizeEmail(account.Email) != invitation.Email {
		return entity.Membership{}, entity.ErrInvitationAddressMismatch
	}

	return s.join(ctx, invitation, workspace, actor)
}

func (s *invitationsService) join(
	ctx context.Context,
	invitation entity.Invitation,
	workspace entity.Workspace,
	accountID uuid.UUID,
) (entity.Membership, error) {
	var membership entity.Membership

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.invitations.MarkAccepted(ctx, invitation.ID, accountID, time.Now().UTC()); err != nil {
			return err
		}

		created, err := s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: invitation.WorkspaceID,
			AccountID:   accountID,
			Role:        invitation.Role,
			Source:      entity.MembershipSourceManual,
		})
		if err != nil {
			return err
		}

		for _, teamID := range grantedTeams(invitation, workspace) {
			if _, err := s.teamMembers.Create(ctx, entity.TeamMembership{
				WorkspaceID: invitation.WorkspaceID,
				TeamID:      teamID,
				AccountID:   accountID,
			}); err != nil {
				return err
			}
		}

		membership = created

		return nil
	})
	if err != nil {
		return entity.Membership{}, err
	}

	return membership, nil
}

func grantedTeams(invitation entity.Invitation, workspace entity.Workspace) []uuid.UUID {
	if len(invitation.TeamIDs) > 0 {
		return invitation.TeamIDs
	}

	if workspace.DefaultTeamID != nil {
		return []uuid.UUID{*workspace.DefaultTeamID}
	}

	return nil
}

func (s *invitationsService) usableInvitation(ctx context.Context, token string) (entity.Invitation, error) {
	if token == "" {
		return entity.Invitation{}, entity.ErrInvitationTokenInvalid
	}

	invitation, err := s.invitations.GetByTokenHash(ctx, entity.HashInvitationToken(token))
	if err != nil {
		return entity.Invitation{}, err
	}

	if err := invitation.UsableAt(time.Now().UTC()); err != nil {
		return entity.Invitation{}, err
	}

	return invitation, nil
}

func (s *invitationsService) scopedInvitation(ctx context.Context, workspaceID, invitationID uuid.UUID) (entity.Invitation, error) {
	invitation, err := s.invitations.GetByID(ctx, invitationID)
	if err != nil {
		return entity.Invitation{}, err
	}

	if invitation.WorkspaceID != workspaceID {
		return entity.Invitation{}, entity.ErrInvitationNotFound
	}

	return invitation, nil
}

func (s *invitationsService) initialDelivery() entity.InvitationDelivery {
	if s.smtp.Configured() {
		return entity.InvitationDeliveryPending
	}

	return entity.InvitationDeliveryLinkOnly
}

func (s *invitationsService) validateTeams(ctx context.Context, workspaceID uuid.UUID, recipients []service.InvitationRecipient) error {
	fields := make([]entity.FieldError, 0, len(recipients))

	for i, recipient := range recipients {
		for _, teamID := range recipient.TeamIDs {
			team, err := s.teams.GetByID(ctx, teamID)
			if err != nil && !errors.Is(err, entity.ErrTeamNotFound) {
				return err
			}

			if err != nil || team.WorkspaceID != workspaceID || team.Archived() {
				fields = append(fields, entity.FieldError{
					Field: "invitations." + strconv.Itoa(i) + ".teamIds",
					Code:  entity.ValidationCodeUnsupportedValue,
				})

				break
			}
		}
	}

	return entity.NewValidationError(fields...)
}

func validateRoles(recipients []service.InvitationRecipient) error {
	fields := make([]entity.FieldError, 0, len(recipients))

	for i, recipient := range recipients {
		if !recipient.Role.Valid() {
			fields = append(fields, entity.FieldError{
				Field: "invitations." + strconv.Itoa(i) + ".role",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	return entity.NewValidationError(fields...)
}
