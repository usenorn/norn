package intake

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type intakesService struct {
	intake      repository.Intake
	mail        repository.InboundMail
	teams       repository.Team
	jobs        repository.JobProducer
	issues      service.Issues
	attachments service.Attachments
	authorizer  service.Authorizer
	transactor  repository.Transactor
	domain      string
}

func New(
	intake repository.Intake,
	mail repository.InboundMail,
	teams repository.Team,
	jobs repository.JobProducer,
	issues service.Issues,
	attachments service.Attachments,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	cfg config.Intake,
) service.Intakes {
	return &intakesService{
		intake:      intake,
		mail:        mail,
		teams:       teams,
		jobs:        jobs,
		issues:      issues,
		attachments: attachments,
		authorizer:  authorizer,
		transactor:  transactor,
		domain:      cfg.Domain,
	}
}

func (s *intakesService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *intakesService) Address(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.IntakeAddress, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return entity.IntakeAddress{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return entity.IntakeAddress{}, entity.ErrTeamNotFound
	}

	return s.intake.Address(ctx, workspaceID, teamID)
}

func (s *intakesService) Enable(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.IntakeAddress, error) {
	decision, err := s.settle(ctx, workspaceID, teamID)
	if err != nil {
		return entity.IntakeAddress{}, err
	}

	if address, err := s.intake.Address(ctx, workspaceID, teamID); err == nil {
		return address, nil
	} else if !errors.Is(err, entity.ErrIntakeDisabled) {
		return entity.IntakeAddress{}, err
	}

	return s.issue(ctx, workspaceID, teamID, decision.Actor.AccountID, nil)
}

func (s *intakesService) Rotate(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.IntakeAddress, error) {
	decision, err := s.settle(ctx, workspaceID, teamID)
	if err != nil {
		return entity.IntakeAddress{}, err
	}

	if _, err := s.intake.Address(ctx, workspaceID, teamID); err != nil {
		return entity.IntakeAddress{}, err
	}

	rotated := time.Now().UTC()

	return s.issue(ctx, workspaceID, teamID, decision.Actor.AccountID, &rotated)
}

func (s *intakesService) Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	if _, err := s.settle(ctx, workspaceID, teamID); err != nil {
		return err
	}

	return s.intake.Disable(ctx, workspaceID, teamID)
}

func (s *intakesService) settle(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.Decision, error) {
	if s.domain == "" {
		return entity.Decision{}, entity.ErrIntakeDomainUnset
	}

	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Decision{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return entity.Decision{}, entity.ErrTeamNotFound
	}

	return decision, nil
}

func (s *intakesService) issue(
	ctx context.Context,
	workspaceID, teamID, enabledBy uuid.UUID,
	rotated *time.Time,
) (entity.IntakeAddress, error) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.IntakeAddress{}, err
	}

	if team.WorkspaceID != workspaceID {
		return entity.IntakeAddress{}, entity.ErrTeamNotFound
	}

	label := entity.IntakeLabel(team)

	for range intakeAttempts {
		token, err := token()
		if err != nil {
			return entity.IntakeAddress{}, err
		}

		saved, err := s.intake.Save(ctx, entity.IntakeAddress{
			TeamID:      teamID,
			WorkspaceID: workspaceID,
			LocalPart:   entity.IntakeLocalPart(label, token),
			Domain:      s.domain,
			EnabledBy:   enabledBy,
			RotatedAt:   rotated,
		})
		if err != nil {
			if errors.Is(err, entity.ErrIntakeAddressTaken) {
				continue
			}

			return entity.IntakeAddress{}, err
		}

		return saved, nil
	}

	return entity.IntakeAddress{}, entity.ErrIntakeAddressTaken
}

const intakeAttempts = 5

func token() (string, error) {
	buffer := make([]byte, entity.IntakeTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate intake address: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}

func (s *intakesService) Accept(
	ctx context.Context,
	header http.Header,
	body []byte,
) (uuid.UUID, error) {
	notice, err := s.mail.Verify(header, body)
	if err != nil {
		return uuid.Nil, err
	}

	if !notice.Carries() {
		return uuid.Nil, nil
	}

	message, err := s.mail.Fetch(ctx, notice.MessageID)
	if err != nil {
		return uuid.Nil, err
	}

	local, err := entity.IntakeRecipient(message.Recipients, s.domain)
	if err != nil {
		return uuid.Nil, err
	}

	address, err := s.intake.AddressOf(ctx, local, s.domain)
	if err != nil {
		return uuid.Nil, err
	}

	deliveryID, err := s.intake.Record(ctx, entity.IntakeDelivery{
		WorkspaceID: address.WorkspaceID,
		TeamID:      address.TeamID,
		ExternalID:  message.ExternalID,
		Recipient:   address.Email(),
		Sender:      entity.EmailAddressOf(message.Sender),
		Subject:     entity.IntakeTitle(message.Subject),
		ReceivedAt:  message.ReceivedAt,
	})
	if err != nil {
		// A provider redelivers what it could not hand over, and the message it repeats is the
		// one whose queueing failed. Recording it again is refused by the external id, so the
		// redelivery has to adopt the stored delivery and queue that, or the mail is lost.
		deliveryID, err = s.stored(ctx, err, message.ExternalID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	if err := s.jobs.EnqueueIntakeDelivery(ctx, entity.IntakeDeliveryPayload{
		DeliveryID: deliveryID,
	}); err != nil {
		return uuid.Nil, err
	}

	return deliveryID, nil
}

func (s *intakesService) stored(
	ctx context.Context,
	recorded error,
	externalID string,
) (uuid.UUID, error) {
	if !errors.Is(recorded, entity.ErrIntakeDeliveryDuplicate) {
		return uuid.Nil, recorded
	}

	delivery, err := s.intake.DeliveryOf(ctx, externalID)
	if err != nil {
		return uuid.Nil, err
	}

	if delivery.ProcessedAt != nil {
		return uuid.Nil, entity.ErrIntakeDeliveryDuplicate
	}

	return delivery.ID, nil
}

// Apply files the message and settles the delivery in one transaction, so a failure anywhere
// leaves the delivery unprocessed and the retry files it once rather than twice. The row is
// held for the transaction, which is what stops a second worker filing it alongside.
// Apply reads the message outside any transaction, then decides and settles inside one. Every
// terminal write happens under the delivery's own lock and after re-reading it, so a worker
// held up behind another cannot overwrite what that one already filed.
func (s *intakesService) Apply(ctx context.Context, deliveryID uuid.UUID) error {
	delivery, err := s.intake.Delivery(ctx, deliveryID)
	if err != nil {
		return err
	}

	if delivery.ProcessedAt != nil {
		return nil
	}

	message, err := s.mail.Fetch(ctx, delivery.ExternalID)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.file(ctx, deliveryID, message)
	})
}

func (s *intakesService) file(
	ctx context.Context,
	deliveryID uuid.UUID,
	message entity.InboundMessage,
) error {
	delivery, err := s.intake.LockDelivery(ctx, deliveryID)
	if err != nil {
		return err
	}

	if delivery.ProcessedAt != nil {
		return nil
	}

	// The address is read again here rather than before the provider call: it can be rotated or
	// retired while a message is being fetched, and the decision has to be the one that holds
	// when the delivery is settled.
	address, err := s.intake.Address(ctx, delivery.WorkspaceID, delivery.TeamID)
	if err != nil {
		if errors.Is(err, entity.ErrIntakeDisabled) {
			return s.ignore(ctx, delivery, "the team no longer takes issues by email")
		}

		return err
	}

	if !strings.EqualFold(address.Email(), delivery.Recipient) {
		return s.ignore(ctx, delivery, "the address the message reached has been rotated")
	}

	if address.EnabledBy == uuid.Nil {
		return s.fail(ctx, delivery, "the account that turned this address on is gone")
	}

	// Mail is filed in the name of whoever turned the address on, as an integration rather
	// than as them: the workspace may demand a particular sign-in method of its people, and
	// nobody signed in to send this.
	acting := identity.WithActor(ctx, entity.Actor{
		Kind:           entity.ActorKindToken,
		AccountID:      address.EnabledBy,
		OwnerAccountID: address.EnabledBy,
	})

	description := entity.IntakeDescription(message)

	created, err := s.issues.Create(acting, service.CreateIssueInput{
		WorkspaceID: delivery.WorkspaceID,
		TeamID:      delivery.TeamID,
		Title:       entity.IntakeTitle(message.Subject),
		Description: description,
		Source:      entity.TriageSourceEmail,
	})
	if err != nil {
		return err
	}

	if err := s.keep(acting, created, description, message); err != nil {
		return err
	}

	return s.intake.Settle(
		ctx, delivery.ID, entity.IntakeDeliveryFiled, created.ID, "", time.Now().UTC(),
	)
}

// keep stores what came with the message and points the body at the files. A picture the
// sender wrote into the message only becomes visible here: until the issue exists there is
// nowhere to put the file, and until the file is stored the body has nothing to point at.
func (s *intakesService) keep(
	ctx context.Context,
	issue entity.Issue,
	description string,
	message entity.InboundMessage,
) error {
	if len(message.Attachments) == 0 {
		return nil
	}

	var (
		stored  = make(map[string]string, len(message.Attachments))
		refused []string
	)

	for _, attachment := range message.Attachments {
		if len(attachment.Content) == 0 {
			refused = append(refused, attachment.FileName)

			continue
		}

		kept, err := s.attachments.Receive(ctx, issue.WorkspaceID, issue.ID, service.ReceiveAttachmentInput{
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			Content:     attachment.Content,
		})
		if err != nil {
			// A file too big for this instance, or for what the workspace has left, is named in
			// the issue rather than dropped in silence. Anything else is a failure to store and
			// the delivery is retried.
			if !errors.Is(err, entity.ErrAttachmentTooLarge) && !errors.Is(err, entity.ErrStorageExhausted) {
				return err
			}

			logging.From(ctx).InfoContext(
				ctx,
				"a file from an inbound message was not kept",
				"issue_id", issue.ID.String(),
				"file_name", attachment.FileName,
				"reason", err.Error(),
			)

			refused = append(refused, attachment.FileName)

			continue
		}

		if attachment.Embedded() {
			stored[entity.IntakeContentReference(attachment.ContentID)] =
				entity.AttachmentContentPath(issue.WorkspaceID, kept.ID)
		}
	}

	embedded := entity.IntakeEmbed(description, stored, refused)
	if embedded == description {
		return nil
	}

	_, err := s.issues.Update(ctx, issue.WorkspaceID, issue.ID, service.UpdateIssueInput{
		ExpectedVersion: issue.Version,
		Description:     &embedded,
	})

	return err
}

func (s *intakesService) ignore(
	ctx context.Context,
	delivery entity.IntakeDelivery,
	reason string,
) error {
	logging.From(ctx).InfoContext(
		ctx,
		"an inbound message was accepted and ignored",
		"delivery_id", delivery.ID.String(),
		"reason", reason,
	)

	return s.intake.Settle(
		ctx, delivery.ID, entity.IntakeDeliveryIgnored, uuid.Nil, reason, time.Now().UTC(),
	)
}

func (s *intakesService) fail(
	ctx context.Context,
	delivery entity.IntakeDelivery,
	reason string,
) error {
	logging.From(ctx).WarnContext(
		ctx,
		"an inbound message could not be filed",
		"delivery_id", delivery.ID.String(),
		"reason", reason,
	)

	return s.intake.Settle(
		ctx, delivery.ID, entity.IntakeDeliveryFailed, uuid.Nil, reason, time.Now().UTC(),
	)
}
