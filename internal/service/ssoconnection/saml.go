package ssoconnection

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/samlkey"
	"github.com/usenorn/norn/internal/pkg/samlprovider"
	"github.com/usenorn/norn/internal/service"
)

const (
	samlMetadataPath = "/v1/sso/saml/"
	samlACSSuffix    = "/acs"
	samlMetaSuffix   = "/metadata"
)

func (s *connectionsService) samlEndpoints(workspaceSlug string) samlprovider.Endpoints {
	base := strings.TrimRight(s.app.BaseURL, "/") + samlMetadataPath + workspaceSlug

	return samlprovider.Endpoints{
		MetadataURL: base + samlMetaSuffix,
		ACSURL:      base + samlACSSuffix,
	}
}

func (s *connectionsService) Protocol(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SSOProtocol, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return "", err
	}

	return s.connections.Protocol(ctx, workspaceID)
}

func (s *connectionsService) GetSAML(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SAMLConnection, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return entity.SAMLConnection{}, err
	}

	return s.connections.GetSAML(ctx, workspaceID)
}

func (s *connectionsService) ReadSAMLMetadata(
	ctx context.Context,
	input service.ReadSAMLMetadataInput,
) (entity.SAMLDescriptor, error) {
	if err := s.authorize(ctx, input.WorkspaceID, entity.ActionUpdate); err != nil {
		return entity.SAMLDescriptor{}, err
	}

	return s.readMetadata(ctx, input.MetadataURL, input.Metadata)
}

func (s *connectionsService) readMetadata(
	ctx context.Context,
	metadataURL, document string,
) (entity.SAMLDescriptor, error) {
	if trimmed := strings.TrimSpace(document); trimmed != "" {
		return samlprovider.Parse([]byte(trimmed))
	}

	if strings.TrimSpace(metadataURL) == "" {
		return entity.SAMLDescriptor{}, entity.NewSSOError(
			entity.SSOStageMetadata,
			"Give Norn a metadata address to fetch, or paste the metadata itself.",
		)
	}

	return s.saml.Fetch(ctx, metadataURL)
}

func (s *connectionsService) SaveSAML(
	ctx context.Context,
	input service.SaveSAMLConnectionInput,
) (entity.SAMLConnection, error) {
	if err := s.authorize(ctx, input.WorkspaceID, entity.ActionUpdate); err != nil {
		return entity.SAMLConnection{}, err
	}

	workspace, err := s.workspaces.GetByID(ctx, input.WorkspaceID)
	if err != nil {
		return entity.SAMLConnection{}, err
	}

	descriptor, err := s.descriptorFor(ctx, input)
	if err != nil {
		return entity.SAMLConnection{}, err
	}

	connection := entity.SAMLConnection{
		WorkspaceID:       input.WorkspaceID,
		Descriptor:        descriptor,
		MetadataURL:       strings.TrimSpace(input.MetadataURL),
		SPEntityID:        s.samlEndpoints(workspace.Slug).MetadataURL,
		AllowIDPInitiated: input.AllowIDPInitiated,
		Mapping:           input.Mapping,
		Provisioning:      input.Provisioning,
	}

	if err := s.keepOrMakeKeypair(ctx, &connection); err != nil {
		return entity.SAMLConnection{}, err
	}

	if err := connection.Validate(); err != nil {
		return entity.SAMLConnection{}, err
	}

	return s.connections.SaveSAML(ctx, connection)
}

func (s *connectionsService) descriptorFor(
	ctx context.Context,
	input service.SaveSAMLConnectionInput,
) (entity.SAMLDescriptor, error) {
	if input.Descriptor == nil {
		return s.readMetadata(ctx, input.MetadataURL, input.Metadata)
	}

	descriptor := *input.Descriptor

	if err := descriptor.Validate(); err != nil {
		return entity.SAMLDescriptor{}, err
	}

	expiry, err := samlkey.EarliestExpiry(descriptor.Certificates)
	if err != nil {
		return entity.SAMLDescriptor{}, entity.SSOFailure(
			entity.SSOStageCertificate,
			"That signing certificate could not be read.",
			err,
		)
	}

	descriptor.ExpiresAt = expiry

	return descriptor, nil
}

func (s *connectionsService) keepOrMakeKeypair(
	ctx context.Context,
	connection *entity.SAMLConnection,
) error {
	existing, err := s.connections.GetSAML(ctx, connection.WorkspaceID)
	if err == nil && len(existing.SPPrivateKey) > 0 && existing.SPCertificate != "" {
		connection.SPPrivateKey = existing.SPPrivateKey
		connection.SPCertificate = existing.SPCertificate

		return nil
	}

	if err != nil && !errors.Is(err, entity.ErrSSOConnectionNotFound) {
		return err
	}

	pair, err := samlkey.Generate(connection.SPEntityID, time.Now().UTC())
	if err != nil {
		return entity.SSOFailure(
			entity.SSOStageCertificate,
			"Norn could not generate a keypair for this workspace.",
			err,
		)
	}

	connection.SPPrivateKey = samlkey.MarshalPrivateKey(pair.PrivateKey)
	connection.SPCertificate = samlkey.MarshalCertificate(pair.Certificate)

	return nil
}

func (s *connectionsService) PublishSAMLMetadata(
	ctx context.Context,
	workspaceSlug string,
) ([]byte, error) {
	workspace, connection, err := s.samlFor(ctx, workspaceSlug)
	if err != nil {
		return nil, err
	}

	return s.saml.Metadata(connection, s.samlEndpoints(workspace.Slug))
}

func (s *connectionsService) samlFor(
	ctx context.Context,
	workspaceSlug string,
) (entity.Workspace, entity.SAMLConnection, error) {
	workspace, err := s.workspaces.GetBySlug(ctx, workspaceSlug)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return entity.Workspace{}, entity.SAMLConnection{}, entity.ErrSSOConnectionNotFound
		}

		return entity.Workspace{}, entity.SAMLConnection{}, err
	}

	connection, err := s.connections.GetSAML(ctx, workspace.ID)
	if err != nil {
		return entity.Workspace{}, entity.SAMLConnection{}, err
	}

	return workspace, connection, nil
}

func (s *connectionsService) BeginSAMLTest(
	ctx context.Context,
	workspaceID uuid.UUID,
) (string, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return "", err
	}

	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	connection, err := s.connections.GetSAML(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	return s.beginSAML(ctx, workspace, connection, entity.SSOPurposeTest, "")
}

func (s *connectionsService) BeginSAMLLogin(
	ctx context.Context,
	input service.BeginOIDCLoginInput,
) (string, error) {
	workspace, connection, err := s.samlFor(ctx, input.WorkspaceSlug)
	if err != nil {
		return "", err
	}

	return s.beginSAML(ctx, workspace, connection, entity.SSOPurposeLogin, input.ReturnTo)
}

func (s *connectionsService) beginSAML(
	ctx context.Context,
	workspace entity.Workspace,
	connection entity.SAMLConnection,
	purpose entity.SSOPurpose,
	returnTo string,
) (string, error) {
	relayState, err := opaque()
	if err != nil {
		return "", err
	}

	target, requestID, err := s.saml.AuthnRequest(
		connection,
		s.samlEndpoints(workspace.Slug),
		relayState,
	)
	if err != nil {
		return "", err
	}

	if err := s.requests.Put(ctx, relayState, entity.SAMLAttempt{
		Purpose:     purpose,
		WorkspaceID: workspace.ID,
		RequestID:   requestID,
		ReturnTo:    safeReturnTo(returnTo),
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return target, nil
}

func (s *connectionsService) CompleteSAML(
	ctx context.Context,
	input service.CompleteSAMLInput,
) (entity.SSOExchange, error) {
	workspace, connection, err := s.samlFor(ctx, input.WorkspaceSlug)
	if err != nil {
		return entity.SSOExchange{}, err
	}

	exchange := entity.SSOExchange{
		Protocol:      entity.SSOProtocolSAML,
		Purpose:       entity.SSOPurposeLogin,
		WorkspaceID:   workspace.ID,
		WorkspaceSlug: workspace.Slug,
	}

	attempt, solicited, err := s.attemptFor(ctx, input.RelayState, connection)
	if err != nil {
		return exchange, err
	}

	exchange.Purpose = attempt.Purpose
	exchange.ReturnTo = attempt.ReturnTo

	requestIDs := []string{attempt.RequestID}
	if !solicited {
		requestIDs = []string{""}
	}

	assertion, err := s.saml.Parse(
		connection,
		s.samlEndpoints(workspace.Slug),
		input.Response,
		requestIDs,
	)
	if err != nil {
		return exchange, err
	}

	fresh, err := s.replays.Claim(ctx, workspace.ID, assertion.ID)
	if err != nil {
		return exchange, err
	}

	if !fresh {
		return exchange, entity.SAMLReplayFailure()
	}

	identity, err := entity.ResolveSAMLIdentity(assertion, connection.Mapping)
	if err != nil {
		return exchange, err
	}

	exchange.Email = identity.Email

	if attempt.Purpose == entity.SSOPurposeTest {
		if err := s.connections.MarkVerified(ctx, workspace.ID, time.Now().UTC()); err != nil {
			return exchange, err
		}

		return exchange, nil
	}

	account, provisioned, err := s.admitIdentity(ctx, admission{
		WorkspaceID:  workspace.ID,
		Provisioning: connection.Provisioning,
		Issuer:       connection.Descriptor.EntityID,
		Subject:      identity.Subject,
		Email:        identity.Email,
		Name:         identity.Name,
	})
	if err != nil {
		return exchange, err
	}

	issued, err := s.sessions.Start(ctx, service.StartSessionInput{
		AccountID:  account.ID,
		AuthMethod: entity.SessionAuthMethodSSO,
		Client:     input.Client,
	})
	if err != nil {
		return exchange, err
	}

	exchange.Session = issued.Session
	exchange.Token = issued.Token
	exchange.Provisioned = provisioned

	return exchange, nil
}

func (s *connectionsService) attemptFor(
	ctx context.Context,
	relayState string,
	connection entity.SAMLConnection,
) (entity.SAMLAttempt, bool, error) {
	if strings.TrimSpace(relayState) == "" {
		if !connection.AllowIDPInitiated {
			return entity.SAMLAttempt{}, false, entity.NewSSOError(
				entity.SSOStageResponse,
				"This workspace does not accept sign-ins started at the provider. Start from Norn instead.",
			)
		}

		return entity.SAMLAttempt{Purpose: entity.SSOPurposeLogin}, false, nil
	}

	attempt, err := s.requests.Take(ctx, relayState)
	if err != nil {
		if errors.Is(err, entity.ErrSSOStateNotFound) && connection.AllowIDPInitiated {
			return entity.SAMLAttempt{Purpose: entity.SSOPurposeLogin}, false, nil
		}

		return entity.SAMLAttempt{}, false, err
	}

	if !attempt.Purpose.Valid() {
		return entity.SAMLAttempt{}, false, entity.ErrSSOStateNotFound
	}

	return attempt, true, nil
}
