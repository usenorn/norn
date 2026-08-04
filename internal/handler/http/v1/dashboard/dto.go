package dashboard

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func accountDTO(account entity.Account, avatarURL string) api.Account {
	dto := api.Account{
		Id:          account.ID,
		Status:      api.AccountStatus(account.Status),
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Timezone:    account.Timezone,
		HasPassword: account.HasPassword(),
		CreatedAt:   account.CreatedAt,
	}

	if avatarURL != "" {
		dto.AvatarUrl = &avatarURL
	}

	return dto
}

func emailChangeDTO(change entity.EmailChange) api.EmailChange {
	return api.EmailChange{
		Id:          change.ID,
		NewEmail:    change.NewEmail,
		RequestedAt: change.RequestedAt,
		ExpiresAt:   change.ExpiresAt,
	}
}

func workspaceDTO(workspace entity.Workspace) api.Workspace {
	return api.Workspace{
		Id:                  workspace.ID,
		Slug:                workspace.Slug,
		Name:                workspace.Name,
		Status:              api.WorkspaceStatus(workspace.Status),
		Timezone:            workspace.Timezone,
		DefaultTeamId:       workspace.DefaultTeamID,
		DeletionRequestedAt: workspace.DeletionRequestedAt,
		PurgeAfter:          workspace.PurgeAfter,
		CreatedAt:           workspace.CreatedAt,
	}
}

func workspaceDTOs(workspaces []entity.Workspace) []api.Workspace {
	dtos := make([]api.Workspace, len(workspaces))
	for i, workspace := range workspaces {
		dtos[i] = workspaceDTO(workspace)
	}

	return dtos
}

func membershipDTO(member entity.WorkspaceMember) api.Membership {
	dto := api.Membership{
		WorkspaceId: member.Membership.WorkspaceID,
		AccountId:   member.Membership.AccountID,
		Role:        api.MembershipRole(member.Membership.Role),
		Source:      api.MembershipSource(member.Membership.Source),
	}

	if member.DisplayName != "" {
		dto.DisplayName = &member.DisplayName
	}

	if member.Email != "" {
		dto.Email = &member.Email
	}

	if !member.Membership.CreatedAt.IsZero() {
		joinedAt := member.Membership.CreatedAt
		dto.JoinedAt = &joinedAt
	}

	if member.Membership.LastActiveAt != nil {
		dto.LastActiveAt = member.Membership.LastActiveAt
	}

	if member.Membership.LastAuthMethod != "" {
		method := api.SessionAuthMethod(member.Membership.LastAuthMethod)
		dto.LastAuthMethod = &method
	}

	return dto
}

func memberPageDTO(page service.MemberPage) api.MemberPage {
	dto := api.MemberPage{Members: membershipDTOs(page.Members)}

	if page.NextCursor != "" {
		dto.NextCursor = &page.NextCursor
	}

	return dto
}

func memberRemovalPreviewDTO(preview service.MemberRemoval) api.MemberRemovalPreview {
	return api.MemberRemovalPreview{
		Member:           membershipDTO(preview.Member),
		Teams:            teamDTOs(preview.Teams),
		SoleAdmin:        preview.SoleAdmin,
		DirectoryManaged: preview.DirectoryManaged,
	}
}

func teamDTO(team entity.Team) api.Team {
	dto := api.Team{
		Id:          team.ID,
		WorkspaceId: team.WorkspaceID,
		Key:         team.Key,
		Name:        team.Name,
		Status:      api.TeamStatus(team.Status),
		Visibility:  api.TeamVisibility(team.Visibility),
		CreatedAt:   team.CreatedAt,
	}

	if team.ArchivedAt != nil {
		dto.ArchivedAt = team.ArchivedAt
	}

	return dto
}

func teamDTOs(teams []entity.Team) []api.Team {
	dtos := make([]api.Team, len(teams))
	for i, team := range teams {
		dtos[i] = teamDTO(team)
	}

	return dtos
}

func teamMemberDTO(member service.TeamMemberView) api.TeamMember {
	dto := api.TeamMember{
		TeamId:      member.Membership.TeamID,
		AccountId:   member.Membership.AccountID,
		DisplayName: member.DisplayName,
		Email:       member.Email,
	}

	if !member.Membership.CreatedAt.IsZero() {
		joinedAt := member.Membership.CreatedAt
		dto.JoinedAt = &joinedAt
	}

	return dto
}

func teamMemberDTOs(members []service.TeamMemberView) []api.TeamMember {
	dtos := make([]api.TeamMember, len(members))
	for i, member := range members {
		dtos[i] = teamMemberDTO(member)
	}

	return dtos
}

func sessionDTO(session entity.Session, currentID uuid.UUID) api.Session {
	client := api.SessionClient{
		UserAgent: &session.Client.UserAgent,
		Location: &api.SessionLocation{
			CountryCode: &session.Client.Location.CountryCode,
			City:        &session.Client.Location.City,
		},
	}

	if session.Client.IP.IsValid() {
		ip := session.Client.IP.String()
		client.Ip = &ip
	}

	return api.Session{
		Id:         session.ID,
		AuthMethod: api.SessionAuthMethod(session.AuthMethod),
		Client:     client,
		IssuedAt:   session.IssuedAt,
		LastUsedAt: session.LastUsedAt,
		ExpiresAt:  session.ExpiresAt(),
		Current:    session.ID == currentID,
	}
}

func sessionDTOs(sessions []entity.Session, currentID uuid.UUID) []api.Session {
	dtos := make([]api.Session, len(sessions))
	for i, session := range sessions {
		dtos[i] = sessionDTO(session, currentID)
	}

	return dtos
}

func workspaceAuthPolicyDTO(policy entity.WorkspaceAuthPolicy) api.WorkspaceAuthPolicy {
	return api.WorkspaceAuthPolicy{
		WorkspaceId: policy.WorkspaceID,
		Enforcement: api.AuthEnforcement(policy.Enforcement),
	}
}

func membershipDTOs(members []entity.WorkspaceMember) []api.Membership {
	dtos := make([]api.Membership, len(members))
	for i, member := range members {
		dtos[i] = membershipDTO(member)
	}

	return dtos
}

func invitationDTO(invitation entity.Invitation) api.Invitation {
	dto := api.Invitation{
		Id:          invitation.ID,
		WorkspaceId: invitation.WorkspaceID,
		Email:       invitation.Email,
		Role:        api.MembershipRole(invitation.Role),
		Status:      api.InvitationStatus(invitation.Status),
		Delivery:    api.InvitationDelivery(invitation.Delivery),
		InvitedAt:   invitation.InvitedAt,
		ExpiresAt:   invitation.ExpiresAt,
	}

	if invitation.AcceptedAt != nil {
		dto.AcceptedAt = invitation.AcceptedAt
	}

	if len(invitation.TeamIDs) > 0 {
		teamIDs := invitation.TeamIDs
		dto.TeamIds = &teamIDs
	}

	return dto
}

func invitationDTOs(invitations []entity.Invitation) []api.Invitation {
	dtos := make([]api.Invitation, len(invitations))
	for i, invitation := range invitations {
		dtos[i] = invitationDTO(invitation)
	}

	return dtos
}

func invitationResultDTO(result service.InvitationResult) api.InvitationResult {
	dto := api.InvitationResult{
		Email:   result.Email,
		Outcome: api.InvitationOutcome(result.Outcome),
	}

	if result.Outcome == entity.InvitationOutcomeCreated {
		invitation := invitationDTO(result.Invitation)
		dto.Invitation = &invitation
		dto.Url = &result.URL
	}

	return dto
}

func invitationResultDTOs(results []service.InvitationResult) []api.InvitationResult {
	dtos := make([]api.InvitationResult, len(results))
	for i, result := range results {
		dtos[i] = invitationResultDTO(result)
	}

	return dtos
}

func invitationPreviewDTO(preview service.InvitationPreview) api.InvitationPreview {
	return api.InvitationPreview{
		Workspace: api.InvitationWorkspace{
			Slug: preview.Workspace.Slug,
			Name: preview.Workspace.Name,
		},
		Email:         preview.Email,
		Role:          api.MembershipRole(preview.Role),
		ExpiresAt:     preview.ExpiresAt,
		AccountExists: preview.AccountExists,
		SsoEnforced:   preview.SSOEnforced,
	}
}

func signUpRequestedDTO(requested service.RequestedSignUp) api.SignUpRequested {
	dto := api.SignUpRequested{
		Email:     requested.Email,
		Delivery:  api.SignUpDelivery(requested.Delivery),
		ExpiresAt: requested.ExpiresAt,
	}

	if requested.URL != "" {
		dto.Url = &requested.URL
	}

	return dto
}

func issueDTO(issue entity.Issue) api.Issue {
	depth := int32(issue.Depth)
	blocked := issue.Blocked
	children := issueProgressDTO(issue.Children)

	dto := api.Issue{
		State: api.IssueState{
			Id:       issue.State.ID,
			Name:     issue.State.Name,
			Category: api.StateCategory(issue.State.Category),
			Position: int32(issue.State.Position),
		},
		Id:             issue.ID,
		WorkspaceId:    issue.WorkspaceID,
		TeamId:         issue.TeamID,
		TeamKey:        issue.TeamKey,
		ReferenceKey:   issue.ReferenceKey,
		Version:        int32(issue.Version),
		Number:         int32(issue.Number),
		Reference:      issue.Reference(),
		Title:          issue.Title,
		Labels:         labelDTOs(issue.Labels),
		Description:    issue.Description,
		Priority:       api.IssuePriority(issue.Priority),
		Status:         api.IssueStatus(issue.Status),
		Depth:          &depth,
		Blocked:        &blocked,
		ChildProgress:  &children,
		StateEnteredAt: issue.StateEnteredAt,
		CreatedAt:      issue.CreatedAt,
	}

	if issue.ParentIssueID != uuid.Nil {
		parent := issue.ParentIssueID
		dto.ParentId = &parent
	}

	dto.ParentReference = nilIfEmpty(issue.ParentReference)

	if issue.ArchivedAt != nil {
		shelved := *issue.ArchivedAt
		dto.ArchivedAt = &shelved
	}

	if issue.AssigneeAccountID != uuid.Nil {
		assignee := issue.AssigneeAccountID
		dto.AssigneeAccountId = &assignee
	}

	if issue.Estimate > 0 {
		estimate := int32(issue.Estimate)
		dto.Estimate = &estimate
	}

	if issue.DueOn != "" {
		due, err := time.Parse(time.DateOnly, issue.DueOn)
		if err == nil {
			day := openapi_types.Date{Time: due}
			dto.DueOn = &day
		}
	}

	dto.CompletedAt = issue.CompletedAt

	if issue.CreatedByAccountID != uuid.Nil {
		author := issue.CreatedByAccountID
		dto.CreatedByAccountId = &author
	}

	return dto
}

func issueRelationDTO(relation entity.IssueRelation) api.IssueRelation {
	dto := api.IssueRelation{
		Id:    relation.ID,
		Kind:  api.IssueRelationKind(relation.Kind),
		Issue: issueDTO(relation.Issue),
	}

	if !relation.CreatedAt.IsZero() {
		created := relation.CreatedAt
		dto.CreatedAt = &created
	}

	return dto
}

func issueRelationGroupDTOs(groups []entity.IssueRelationGroup) []api.IssueRelationGroup {
	dtos := make([]api.IssueRelationGroup, 0, len(groups))

	for _, group := range groups {
		relations := make([]api.IssueRelation, 0, len(group.Relations))

		for _, relation := range group.Relations {
			relations = append(relations, issueRelationDTO(relation))
		}

		dtos = append(dtos, api.IssueRelationGroup{
			Kind:      api.IssueRelationKind(group.Kind),
			Relations: relations,
		})
	}

	return dtos
}

func issueDTOs(issues []entity.Issue) []api.Issue {
	dtos := make([]api.Issue, 0, len(issues))

	for _, issue := range issues {
		dtos = append(dtos, issueDTO(issue))
	}

	return dtos
}

func issuePageDTO(page service.IssuePage) api.IssuePage {
	dto := api.IssuePage{Issues: issueDTOs(page.Issues)}

	if page.NextCursor != "" {
		cursor := page.NextCursor
		dto.NextCursor = &cursor
	}

	return dto
}

func apiTokenDTO(token entity.APIToken) api.APIToken {
	dto := api.APIToken{
		Id:          token.ID,
		WorkspaceId: token.WorkspaceID,
		Name:        token.Name,
		Scopes:      token.Scopes.Strings(),
		CreatedAt:   token.CreatedAt,
	}

	dto.ExpiresAt = token.ExpiresAt
	dto.LastUsedAt = token.LastUsedAt

	return dto
}

func apiTokenDTOs(tokens []entity.APIToken) []api.APIToken {
	dtos := make([]api.APIToken, 0, len(tokens))

	for _, token := range tokens {
		dtos = append(dtos, apiTokenDTO(token))
	}

	return dtos
}

func labelDTO(label entity.Label) api.Label {
	dto := api.Label{
		Id:          label.ID,
		WorkspaceId: label.WorkspaceID,
		Name:        label.Name,
		Color:       api.LabelColor(label.Color),
	}

	if label.TeamID != uuid.Nil {
		teamID := label.TeamID
		dto.TeamId = &teamID
	}

	if label.GroupID != uuid.Nil {
		groupID := label.GroupID
		dto.GroupId = &groupID
	}

	return dto
}

func labelDTOs(labels []entity.Label) []api.Label {
	dtos := make([]api.Label, 0, len(labels))

	for _, label := range labels {
		dtos = append(dtos, labelDTO(label))
	}

	return dtos
}

func labelGroupDTO(group entity.LabelGroup) api.LabelGroup {
	return api.LabelGroup{
		Id:          group.ID,
		WorkspaceId: group.WorkspaceID,
		Name:        group.Name,
	}
}

func labelGroupDTOs(groups []entity.LabelGroup) []api.LabelGroup {
	dtos := make([]api.LabelGroup, 0, len(groups))

	for _, group := range groups {
		dtos = append(dtos, labelGroupDTO(group))
	}

	return dtos
}

func labelUsageDTO(usage entity.LabelUsage) api.LabelUsage {
	return api.LabelUsage{Issues: int32(usage.Issues)}
}

func workflowStateDTO(state entity.WorkflowState) api.WorkflowState {
	return api.WorkflowState{
		Id:           state.ID,
		TeamId:       state.TeamID,
		Name:         state.Name,
		Category:     api.StateCategory(state.Category),
		Position:     int32(state.Position),
		IsDefault:    state.IsDefault,
		IsCompletion: state.IsCompletion,
	}
}

func workflowStateDTOs(states []entity.WorkflowState) []api.WorkflowState {
	dtos := make([]api.WorkflowState, 0, len(states))

	for _, state := range states {
		dtos = append(dtos, workflowStateDTO(state))
	}

	return dtos
}

func issueProgressDTO(progress entity.IssueProgress) api.IssueProgress {
	return api.IssueProgress{
		NotStarted: int32(progress.NotStarted),
		Active:     int32(progress.Active),
		Complete:   int32(progress.Complete),
		Abandoned:  int32(progress.Abandoned),
	}
}

func issueActivityDTO(activity entity.IssueActivity) api.IssueActivity {
	dto := api.IssueActivity{
		Id:        activity.ID,
		IssueId:   activity.IssueID,
		Kind:      api.IssueActivityKind(activity.Kind),
		CreatedAt: activity.CreatedAt,
	}

	if activity.ActorAccountID != uuid.Nil {
		actor := activity.ActorAccountID
		dto.ActorAccountId = &actor
	}

	if activity.ActorName != "" {
		name := activity.ActorName
		dto.ActorName = &name
	}

	dto.Field = nilIfEmpty(activity.Field)
	dto.FromValue = nilIfEmpty(activity.FromValue)
	dto.ToValue = nilIfEmpty(activity.ToValue)

	if activity.Version > 0 {
		version := int32(activity.Version)
		dto.Version = &version
	}

	if activity.FromState != "" {
		from := activity.FromState
		dto.FromState = &from
	}

	if activity.ToState != "" {
		to := activity.ToState
		dto.ToState = &to
	}

	return dto
}

func issueActivityPageDTO(page service.IssueActivityPage) api.IssueActivityPage {
	entries := make([]api.IssueActivity, 0, len(page.Entries))

	for _, entry := range page.Entries {
		entries = append(entries, issueActivityDTO(entry))
	}

	dto := api.IssueActivityPage{Entries: entries}

	if page.NextCursor != "" {
		cursor := page.NextCursor
		dto.NextCursor = &cursor
	}

	return dto
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
