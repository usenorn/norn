package dashboard

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

// sourceControlDTO never carries the token. What leaves here is whether one is held and the
// few characters that tell a person which it is.
func sourceControlDTO(connection entity.SCMConnection) api.SourceControlConnection {
	dto := api.SourceControlConnection{
		Id:        connection.ID,
		Provider:  api.SourceControlProvider(connection.Provider),
		AuthKind:  api.SourceControlAuthKind(connection.AuthKind),
		TokenSet:  connection.TokenSet,
		TokenHint: connection.TokenHint,
		Status:    api.SourceControlStatus(connection.Status),
		CreatedAt: connection.CreatedAt,
		UpdatedAt: connection.UpdatedAt,
	}

	if connection.AccountLogin != "" {
		dto.AccountLogin = &connection.AccountLogin
	}

	if connection.BaseURL != "" {
		dto.BaseUrl = &connection.BaseURL
	}

	if connection.Label != "" {
		dto.Label = &connection.Label
	}

	if connection.IdentityLogin != "" {
		dto.IdentityLogin = &connection.IdentityLogin
	}

	if connection.BrokenReason != entity.SCMBrokenNone {
		reason := api.SourceControlBrokenReason(connection.BrokenReason)
		dto.BrokenReason = &reason
	}

	if connection.BrokenDetail != "" {
		dto.BrokenDetail = &connection.BrokenDetail
	}

	dto.BrokenAt = connection.BrokenAt
	dto.VerifiedAt = connection.VerifiedAt
	dto.AllowPrivateAddress = pointer(connection.Trust.AllowPrivateAddress)
	dto.CaCertificateSet = pointer(connection.Trust.CACertificate != "")

	dto.RepositoryCount = pointer(int32(connection.RepositoryCount))

	held := capabilityDTOs(connection.Capabilities)
	dto.Capabilities = &held

	missing := capabilityDTOs(connection.Capabilities.Missing())
	dto.MissingCapabilities = &missing

	return dto
}

func capabilityDTOs(capabilities []entity.SCMCapability) []api.SourceControlCapability {
	dtos := make([]api.SourceControlCapability, len(capabilities))
	for i, capability := range capabilities {
		dtos[i] = api.SourceControlCapability(capability)
	}

	return dtos
}

func sourceControlDTOs(connections []entity.SCMConnection) []api.SourceControlConnection {
	dtos := make([]api.SourceControlConnection, len(connections))
	for i, connection := range connections {
		dtos[i] = sourceControlDTO(connection)
	}

	return dtos
}

func sourceControlRepositoryDTO(stored entity.SCMRepository) api.SourceControlRepository {
	dto := api.SourceControlRepository{
		Id:            stored.ID,
		ConnectionId:  stored.ConnectionID,
		Provider:      api.SourceControlProvider(stored.Provider),
		FullName:      stored.FullName,
		MirrorLabel:   stored.MirrorLabel,
		HookInstalled: stored.HookInstalled(),
		CreatedAt:     stored.CreatedAt,
		UpdatedAt:     stored.UpdatedAt,
	}

	if stored.DefaultBranch != "" {
		dto.DefaultBranch = &stored.DefaultBranch
	}

	if stored.URL != "" {
		dto.Url = &stored.URL
	}

	direction := api.MirrorDirection(stored.Direction())
	dto.SyncDirection = &direction
	dto.WebhooksDisabled = pointer(stored.WebhooksDisabled)
	dto.PollIntervalSeconds = pointer(int32(stored.PollInterval / time.Second))
	dto.RouteCount = pointer(int32(stored.RouteCount))
	dto.LastSeenAt = stored.LastSeenAt
	dto.ReconciledAt = stored.ReconciledAt
	dto.ReconcileAfter = stored.ReconcileAfter

	return dto
}

func sourceControlRepositoryDTOs(stored []entity.SCMRepository) []api.SourceControlRepository {
	dtos := make([]api.SourceControlRepository, len(stored))
	for i, one := range stored {
		dtos[i] = sourceControlRepositoryDTO(one)
	}

	return dtos
}

func sourceControlRouteDTO(route entity.SCMRoute) api.SourceControlRoute {
	return api.SourceControlRoute{
		Id:           route.ID,
		RepositoryId: route.RepositoryID,
		TeamId:       route.TeamID,
		PathPrefix:   route.PathPrefix,
		CreatedAt:    route.CreatedAt,
	}
}

func sourceControlRouteDTOs(routes entity.SCMRoutes) []api.SourceControlRoute {
	dtos := make([]api.SourceControlRoute, len(routes))
	for i, route := range routes {
		dtos[i] = sourceControlRouteDTO(route)
	}

	return dtos
}

func transitionRuleDTO(rule service.TeamTransitionRule) api.SourceControlTransitionRule {
	dto := api.SourceControlTransitionRule{
		Id:      rule.Rule.ID,
		TeamId:  rule.Rule.TeamID,
		Trigger: api.CodeChangeState(rule.Rule.Trigger),
		StateId: rule.Rule.StateID,
	}

	if rule.StateName != "" {
		dto.StateName = &rule.StateName
	}

	return dto
}

func transitionRuleDTOs(rules []service.TeamTransitionRule) []api.SourceControlTransitionRule {
	dtos := make([]api.SourceControlTransitionRule, len(rules))
	for i, rule := range rules {
		dtos[i] = transitionRuleDTO(rule)
	}

	return dtos
}

func codeLinkDTO(link entity.CodeLink) api.CodeLink {
	dto := api.CodeLink{
		Id:         link.ID,
		Provider:   api.SourceControlProvider(link.Provider),
		Repository: link.RepositoryName,
		Kind:       api.CodeLinkKind(link.Kind),
		ExternalId: link.ExternalID,
		Url:        link.URL,
		State:      api.CodeChangeState(link.State),
		Connected:  !link.Disconnected(),
		CreatedAt:  link.CreatedAt,
		MergedAt:   link.MergedAt,
		ClosedAt:   link.ClosedAt,
	}

	if link.Number > 0 {
		number := int32(link.Number)
		dto.Number = &number
	}

	if link.Title != "" {
		dto.Title = &link.Title
	}

	if link.Author != "" {
		dto.Author = &link.Author
	}

	if link.HeadBranch != "" {
		dto.HeadBranch = &link.HeadBranch
	}

	if link.BaseBranch != "" {
		dto.BaseBranch = &link.BaseBranch
	}

	if link.DetectedIn != "" {
		dto.DetectedIn = &link.DetectedIn
	}

	dto.Resolving = pointer(link.Resolving)

	if link.Checks != entity.CodeChecksUnknown {
		checks := api.CodeChecks(link.Checks)
		dto.Checks = &checks
	}

	return dto
}

func codeLinkDTOs(
	links []entity.CodeLink,
	reviewers map[uuid.UUID]entity.CodeReviewers,
) []api.CodeLink {
	dtos := make([]api.CodeLink, len(links))

	for i, link := range links {
		dtos[i] = codeLinkDTO(link)

		found := reviewers[link.ID]
		if len(found) == 0 {
			continue
		}

		listed := make([]api.CodeReviewer, len(found))
		for j, reviewer := range found {
			listed[j] = api.CodeReviewer{
				Login:      reviewer.Login,
				Verdict:    api.ReviewVerdict(reviewer.Verdict),
				ReviewedAt: reviewer.ReviewedAt,
			}

			if reviewer.URL != "" {
				listed[j].Url = &reviewer.URL
			}
		}

		dtos[i].Reviewers = &listed
	}

	return dtos
}

func teamSCMSettingsDTO(settings entity.SCMTeamSettings) api.TeamSourceControlSettings {
	return api.TeamSourceControlSettings{
		TeamId:         settings.TeamID,
		BranchTemplate: settings.Template(),
	}
}

func issueMirrorDTO(mirror entity.IssueMirror) api.IssueMirror {
	dto := api.IssueMirror{
		Id:             mirror.ID,
		Provider:       api.SourceControlProvider(mirror.Provider),
		Repository:     mirror.RepositoryName,
		ExternalId:     mirror.ExternalID,
		ExternalNumber: int32(mirror.ExternalNumber),
		Url:            mirror.URL,
		Origin:         api.IssueMirrorOrigin(mirror.Origin),
		Connected:      mirror.RepositoryID != uuid.Nil,
		PulledAt:       mirror.PulledAt,
		PushedAt:       mirror.PushedAt,
	}

	if mirror.Direction != "" {
		direction := api.IssueMirrorDirection(mirror.Direction)
		dto.Direction = &direction
	}

	return dto
}

func issueMirrorDTOs(mirrors []entity.IssueMirror) []api.IssueMirror {
	dtos := make([]api.IssueMirror, len(mirrors))
	for i, mirror := range mirrors {
		dtos[i] = issueMirrorDTO(mirror)
	}

	return dtos
}

func pointer[T any](value T) *T {
	return &value
}

func sourceControlDeliveryDTO(delivery entity.SCMDelivery) api.SourceControlDelivery {
	dto := api.SourceControlDelivery{
		Id:          delivery.ID,
		Event:       delivery.Event,
		ReceivedAt:  delivery.ReceivedAt,
		ProcessedAt: delivery.ProcessedAt,
		RetryAfter:  delivery.RetryAfter,
		Attempt:     pointer(int32(delivery.Attempt)),
	}

	if delivery.ExternalID != "" {
		dto.ExternalId = &delivery.ExternalID
	}

	if delivery.Outcome.Settled() {
		outcome := api.SourceControlDeliveryOutcome(delivery.Outcome)
		dto.Outcome = &outcome
	}

	if delivery.Detail != "" {
		dto.Detail = &delivery.Detail
	}

	return dto
}

func sourceControlDeliveryDTOs(deliveries []entity.SCMDelivery) []api.SourceControlDelivery {
	dtos := make([]api.SourceControlDelivery, len(deliveries))
	for i, delivery := range deliveries {
		dtos[i] = sourceControlDeliveryDTO(delivery)
	}

	return dtos
}

func scmIdentityDTO(identity entity.SCMIdentity) api.SCMIdentity {
	return api.SCMIdentity{
		Id:        identity.ID,
		AccountId: identity.AccountID,
		Provider:  api.SourceControlProvider(identity.Provider),
		Login:     identity.Login,
	}
}

func scmIdentityDTOs(identities entity.SCMIdentities) []api.SCMIdentity {
	dtos := make([]api.SCMIdentity, len(identities))
	for i, identity := range identities {
		dtos[i] = scmIdentityDTO(identity)
	}

	return dtos
}

func mirrorConflictDTO(conflict entity.MirrorConflict) api.MirrorConflict {
	return api.MirrorConflict{
		Id:         conflict.ID,
		Field:      conflict.Field,
		Winner:     api.MirrorConflictWinner(conflict.Winner),
		Discarded:  conflict.Discarded,
		Kept:       conflict.Kept,
		OccurredAt: conflict.OccurredAt,
	}
}

func mirrorConflictDTOs(conflicts []entity.MirrorConflict) []api.MirrorConflict {
	dtos := make([]api.MirrorConflict, len(conflicts))
	for i, conflict := range conflicts {
		dtos[i] = mirrorConflictDTO(conflict)
	}

	return dtos
}

func issueShippingDTO(shipping service.IssueShipping) api.IssueShipping {
	releases := make([]api.SCMRelease, len(shipping.Releases))

	for i, release := range shipping.Releases {
		releases[i] = api.SCMRelease{
			Id:          release.ID,
			Tag:         release.Tag,
			Name:        release.DisplayName(),
			Prerelease:  pointer(release.Prerelease),
			PublishedAt: release.PublishedAt,
		}

		if release.URL != "" {
			releases[i].Url = &release.URL
		}
	}

	deployments := make([]api.SCMDeployment, 0, len(shipping.Deployments))

	for _, environment := range shipping.Deployments.Environments() {
		latest, found := shipping.Deployments.Latest(environment)
		if !found {
			continue
		}

		one := api.SCMDeployment{
			Id:          latest.ID,
			Environment: latest.Environment,
			State:       api.SCMDeploymentState(latest.State),
			OccurredAt:  latest.OccurredAt,
		}

		if latest.URL != "" {
			one.Url = &latest.URL
		}

		deployments = append(deployments, one)
	}

	return api.IssueShipping{Releases: releases, Deployments: deployments}
}
