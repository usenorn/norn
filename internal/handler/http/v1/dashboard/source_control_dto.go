package dashboard

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

// sourceControlDTO never carries the token. What leaves here is whether one is held and the
// few characters that tell a person which it is.
func sourceControlDTO(connection entity.SCMConnection) api.SourceControlConnection {
	dto := api.SourceControlConnection{
		Id:            connection.ID,
		Provider:      api.SourceControlProvider(connection.Provider),
		Repository:    connection.Repository,
		TokenSet:      connection.TokenSet,
		TokenHint:     connection.TokenHint,
		MirrorLabel:   connection.MirrorLabel,
		Status:        api.SourceControlStatus(connection.Status),
		CreatedAt:     connection.CreatedAt,
		UpdatedAt:     connection.UpdatedAt,
		HookInstalled: pointer(connection.ExternalHookID != ""),
	}

	if connection.BaseURL != "" {
		dto.BaseUrl = &connection.BaseURL
	}

	if connection.TeamID != uuid.Nil {
		teamID := connection.TeamID
		dto.TeamId = &teamID
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
	dto.LastSeenAt = connection.LastSeenAt

	return dto
}

func sourceControlDTOs(connections []entity.SCMConnection) []api.SourceControlConnection {
	dtos := make([]api.SourceControlConnection, len(connections))
	for i, connection := range connections {
		dtos[i] = sourceControlDTO(connection)
	}

	return dtos
}

func teamSourceControlDTO(
	teamID uuid.UUID,
	settings service.TeamSourceControlSettings,
) api.TeamSourceControlSettings {
	dto := api.TeamSourceControlSettings{
		TeamId:         teamID,
		AdvanceOnMerge: settings.Settings.AdvanceOnMerge,
		TargetResolved: settings.TargetResolved,
	}

	if settings.Settings.MergedStateID != uuid.Nil {
		stateID := settings.Settings.MergedStateID
		dto.MergedStateId = &stateID
	}

	if settings.TargetName != "" {
		dto.TargetName = &settings.TargetName
	}

	return dto
}

func codeLinkDTO(link entity.CodeLink) api.CodeLink {
	dto := api.CodeLink{
		Id:         link.ID,
		Provider:   api.SourceControlProvider(link.Provider),
		Repository: link.Repository,
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

	if link.DetectedIn != "" {
		dto.DetectedIn = &link.DetectedIn
	}

	dto.AdvancedIssue = pointer(link.AdvancedIssue)

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
	return api.IssueMirror{
		Id:             mirror.ID,
		Provider:       api.SourceControlProvider(mirror.Provider),
		Repository:     mirror.Repository,
		ExternalId:     mirror.ExternalID,
		ExternalNumber: int32(mirror.ExternalNumber),
		Url:            mirror.URL,
		Origin:         api.IssueMirrorOrigin(mirror.Origin),
		Connected:      mirror.ConnectionID != uuid.Nil,
		PulledAt:       mirror.PulledAt,
		PushedAt:       mirror.PushedAt,
	}
}

func pointer[T any](value T) *T {
	return &value
}
