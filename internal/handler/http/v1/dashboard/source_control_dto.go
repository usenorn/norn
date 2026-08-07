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
		TokenSet:  connection.TokenSet,
		TokenHint: connection.TokenHint,
		Status:    api.SourceControlStatus(connection.Status),
		CreatedAt: connection.CreatedAt,
		UpdatedAt: connection.UpdatedAt,
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

	return dto
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

	dto.PollIntervalSeconds = pointer(int32(stored.PollInterval / time.Second))
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
	dto.ChecksFailed = pointer(link.Action == entity.CodeChangeActionCI)

	return dto
}

func codeLinkDTOs(links []entity.CodeLink) []api.CodeLink {
	dtos := make([]api.CodeLink, len(links))
	for i, link := range links {
		dtos[i] = codeLinkDTO(link)
	}

	return dtos
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
