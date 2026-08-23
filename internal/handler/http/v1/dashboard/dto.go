package dashboard

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func accountDTO(account entity.Account) api.Account {
	dto := api.Account{
		Id:          account.ID,
		Status:      api.AccountStatus(account.Status),
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Timezone:    account.Timezone,
		HasPassword: account.HasPassword(),
		CreatedAt:   account.CreatedAt,
	}

	if account.AvatarObjectKey != "" {
		avatarPath := "/v1/accounts/" + account.ID.String() + "/avatar"
		dto.AvatarUrl = &avatarPath
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

	if member.AccountKind != "" {
		kind := api.AccountKind(member.AccountKind)
		dto.Kind = &kind
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

	dto.ReadsAudit = &member.Membership.ReadsAudit
	dto.HasRunner = &member.HasRunner
	dto.DeactivatedAt = member.Membership.DeactivatedAt

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
	teams := preview.Teams
	if teams == nil {
		teams = []string{}
	}

	dto := api.InvitationPreview{
		Workspace: api.InvitationWorkspace{
			Slug: preview.Workspace.Slug,
			Name: preview.Workspace.Name,
		},
		Email:         preview.Email,
		Role:          api.MembershipRole(preview.Role),
		InvitedAt:     preview.InvitedAt,
		ExpiresAt:     preview.ExpiresAt,
		Teams:         teams,
		AccountExists: preview.AccountExists,
		SsoEnforced:   preview.SSOEnforced,
	}

	if preview.InvitedBy != nil {
		dto.InvitedBy = &api.InvitationInviter{
			Name:  preview.InvitedBy.DisplayName,
			Email: preview.InvitedBy.Email,
		}
	}

	return dto
}

func signUpRequestedDTO(requested service.RequestedSignUp) api.SignUpRequested {
	return api.SignUpRequested{
		Email:       requested.Email,
		RequestedAt: requested.RequestedAt,
		ExpiresAt:   requested.ExpiresAt,
	}
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

	if issue.CycleID != uuid.Nil {
		cycle := issue.CycleID
		number := int32(issue.CycleNumber)
		dto.CycleId = &cycle
		dto.CycleNumber = &number
	}

	if issue.ProjectID != uuid.Nil {
		project := issue.ProjectID
		dto.ProjectId = &project
		dto.ProjectName = nilIfEmpty(issue.ProjectName)
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

	if issue.TriageState != "" {
		state := api.TriageState(issue.TriageState)
		dto.TriageState = &state
	}

	if issue.TriageSource != "" {
		source := api.TriageSource(issue.TriageSource)
		dto.TriageSource = &source
	}

	if issue.TriageDecidedBy != uuid.Nil {
		decider := issue.TriageDecidedBy
		dto.TriageDecidedByAccountId = &decider
		dto.TriageDecidedByName = nilIfEmpty(issue.TriageDecidedName)
	}

	dto.TriageDecidedAt = issue.TriageDecidedAt

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

func bulkResultDTO(
	action entity.BulkAction,
	outcomes []entity.BulkActionOutcome,
) api.BulkActionResult {
	dtos := make([]api.BulkActionOutcome, 0, len(outcomes))

	for _, outcome := range outcomes {
		dtos = append(dtos, api.BulkActionOutcome{
			IssueId:   outcome.IssueID,
			Reference: outcome.Reference,
			Outcome:   api.BulkOutcome(outcome.Outcome),
		})
	}

	dto := api.BulkActionResult{
		Id:        action.ID,
		Status:    api.BulkActionStatus(action.Status),
		Processed: int32(action.Processed),
		Outcomes:  dtos,
	}

	if action.Expected != nil {
		expected := int32(*action.Expected)
		dto.Expected = &expected
	}

	return dto
}

func projectDTO(view service.ProjectView) api.Project {
	project := view.Project

	dto := api.Project{
		Id:            project.ID,
		WorkspaceId:   project.WorkspaceID,
		Slug:          project.Slug,
		Name:          project.Name,
		Description:   project.Description,
		State:         api.ProjectState(project.State),
		Archived:      project.Archived(),
		ConcealedWork: view.ConcealedWork,
		CreatedAt:     project.CreatedAt,
	}

	if project.LeadAccountID != uuid.Nil {
		lead := project.LeadAccountID
		dto.LeadAccountId = &lead
		dto.LeadName = nilIfEmpty(project.LeadName)
	}

	if project.TargetOn != "" {
		target := calendarDate(project.TargetOn)
		dto.TargetOn = &target
	}

	if project.ArchivedAt != nil {
		shelved := *project.ArchivedAt
		dto.ArchivedAt = &shelved
	}

	if project.Health != "" {
		health := api.ProjectHealth(project.Health)
		dto.Health = &health
	}

	return dto
}

func projectDTOs(views []service.ProjectView) []api.Project {
	dtos := make([]api.Project, 0, len(views))

	for _, view := range views {
		dtos = append(dtos, projectDTO(view))
	}

	return dtos
}

func projectMemberDTO(view service.ProjectMemberView) api.ProjectMember {
	return api.ProjectMember{
		ProjectId:   view.Membership.ProjectID,
		AccountId:   view.Membership.AccountID,
		DisplayName: view.DisplayName,
		Email:       view.Email,
		CreatedAt:   view.Membership.CreatedAt,
	}
}

func projectMemberDTOs(views []service.ProjectMemberView) []api.ProjectMember {
	dtos := make([]api.ProjectMember, 0, len(views))

	for _, view := range views {
		dtos = append(dtos, projectMemberDTO(view))
	}

	return dtos
}

func projectStatusDTO(update entity.ProjectStatusUpdate) api.ProjectStatusUpdate {
	dto := api.ProjectStatusUpdate{
		Id:        update.ID,
		ProjectId: update.ProjectID,
		Health:    api.ProjectHealth(update.Health),
		Body:      update.Body,
		CreatedAt: update.CreatedAt,
	}

	if update.AuthorAccountID != uuid.Nil {
		author := update.AuthorAccountID
		dto.AuthorAccountId = &author
		dto.AuthorName = nilIfEmpty(update.AuthorName)
	}

	return dto
}

func projectStatusDTOs(updates []entity.ProjectStatusUpdate) []api.ProjectStatusUpdate {
	dtos := make([]api.ProjectStatusUpdate, 0, len(updates))

	for _, update := range updates {
		dtos = append(dtos, projectStatusDTO(update))
	}

	return dtos
}

func cycleDTO(view service.CycleView) api.Cycle {
	dto := api.Cycle{
		Id:          view.Cycle.ID,
		WorkspaceId: view.Cycle.WorkspaceID,
		TeamId:      view.Cycle.TeamID,
		TeamKey:     view.Cycle.TeamKey,
		Number:      int32(view.Cycle.Number),
		Name:        "Cycle " + strconv.Itoa(view.Cycle.Number),
		StartsOn:    calendarDate(view.Cycle.StartsOn),
		EndsOn:      calendarDate(view.Cycle.EndsOn),
		Phase:       api.CyclePhase(view.Phase),
	}

	if view.Cycle.ClosedAt != nil {
		closed := *view.Cycle.ClosedAt
		dto.ClosedAt = &closed
	}

	if view.Cycle.ClosedByAccountID != uuid.Nil {
		account := view.Cycle.ClosedByAccountID
		dto.ClosedByAccountId = &account
	}

	if view.Cycle.Rollover != entity.CycleRolloverNone {
		rollover := api.CycleRollover(view.Cycle.Rollover)
		dto.Rollover = &rollover
	}

	return dto
}

func calendarDate(date string) openapi_types.Date {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return openapi_types.Date{}
	}

	return openapi_types.Date{Time: parsed}
}

func cycleDTOs(views []service.CycleView) []api.Cycle {
	dtos := make([]api.Cycle, 0, len(views))

	for _, view := range views {
		dtos = append(dtos, cycleDTO(view))
	}

	return dtos
}

func cadenceDTO(view service.CadenceView) api.CycleCadence {
	return api.CycleCadence{
		TeamId:      view.Cadence.TeamID,
		LengthWeeks: int32(view.Cadence.LengthWeeks),
		StartsOn:    int32(view.Cadence.Weekday()),
		Upcoming:    cycleDTOs(view.Upcoming),
	}
}

func cycleScopeDTO(scope service.CycleScope) api.CycleScope {
	changes := make([]api.CycleScopeChange, 0, len(scope.Changes))

	for _, change := range scope.Changes {
		dto := api.CycleScopeChange{
			Id:             change.ID,
			IssueId:        change.IssueID,
			IssueReference: change.IssueReference,
			IssueTitle:     change.IssueTitle,
			Change:         api.CycleScopeChangeKind(change.Change),
			ChangedAt:      change.ChangedAt,
		}

		if change.ActorAccountID != uuid.Nil {
			actor := change.ActorAccountID
			dto.ActorAccountId = &actor
		}

		changes = append(changes, dto)
	}

	return api.CycleScope{
		Original: issueDTOs(scope.Original),
		Added:    issueDTOs(scope.Added),
		Changes:  changes,
	}
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

func issueQueryResultDTO(result service.IssueQueryResult, grouped bool) api.IssueQueryResult {
	dto := api.IssueQueryResult{Issues: issueDTOs(result.Issues)}

	if result.NextCursor != "" {
		cursor := result.NextCursor
		dto.NextCursor = &cursor
	}

	if grouped {
		groups := make([]api.IssueGroupTally, len(result.Groups))
		for i, group := range result.Groups {
			groups[i] = issueGroupTallyDTO(group)
		}

		dto.Groups = &groups
	}

	return dto
}

func issueGroupTallyDTO(tally entity.IssueGroupTally) api.IssueGroupTally {
	dto := api.IssueGroupTally{Key: tally.Key, Issues: int32(tally.Issues)}

	if tally.NextCursor != "" {
		cursor := tally.NextCursor
		dto.NextCursor = &cursor
	}

	return dto
}

func triageQueueDTO(queue service.TriageQueue) api.TriageQueue {
	teams := make([]api.IssueGroupTally, 0, len(queue.Teams))
	for _, team := range queue.Teams {
		teams = append(teams, issueGroupTallyDTO(team))
	}

	dto := api.TriageQueue{Issues: issueDTOs(queue.Issues), Teams: teams}

	if queue.NextCursor != "" {
		cursor := queue.NextCursor
		dto.NextCursor = &cursor
	}

	return dto
}

func triageSettingsDTO(settings entity.TriageSettings) api.TriageSettings {
	return api.TriageSettings{
		TeamId:            settings.TeamID,
		RouteAgents:       settings.RouteAgents,
		RouteIntegrations: settings.RouteIntegrations,
		RouteNonMembers:   settings.RouteNonMembers,
	}
}

func savedViewDTO(summary service.SavedViewSummary) api.SavedView {
	view := summary.View

	dto := api.SavedView{
		Id:          view.ID,
		WorkspaceId: view.WorkspaceID,
		Name:        view.Name,
		Sharing:     api.SavedViewSharing(view.Sharing),
		Filter:      issueFilterDTO(view.Filter),
		Sort:        issueSortDTOs(view.Sort),
		Editable:    summary.Editable,
		CreatedAt:   view.CreatedAt,
		UpdatedAt:   view.UpdatedAt,
	}

	if view.TeamID != uuid.Nil {
		team := view.TeamID
		dto.TeamId = &team
		dto.TeamName = nilIfEmpty(view.TeamName)
	}

	if view.AuthorID != uuid.Nil {
		author := view.AuthorID
		dto.CreatedByAccountId = &author
		dto.CreatedByName = nilIfEmpty(view.AuthorName)
	}

	if view.GroupBy != "" {
		groupBy := api.IssueGroupBy(view.GroupBy)
		dto.GroupBy = &groupBy
	}

	return dto
}

func savedViewDTOs(summaries []service.SavedViewSummary) []api.SavedView {
	dtos := make([]api.SavedView, 0, len(summaries))
	for _, summary := range summaries {
		dtos = append(dtos, savedViewDTO(summary))
	}

	return dtos
}

func savedViewDetailDTO(detail service.SavedViewDetail) api.SavedViewDetail {
	references := make([]api.IssueFilterReference, 0, len(detail.References))

	for _, reference := range detail.References {
		dto := api.IssueFilterReference{
			Field: api.IssueFilterField(reference.Field),
			Value: reference.Value,
			State: api.IssueFilterReferenceState(reference.State),
		}

		dto.Name = nilIfEmpty(reference.Name)

		references = append(references, dto)
	}

	return api.SavedViewDetail{
		View:       savedViewDTO(detail.Summary),
		References: references,
	}
}

func issueFilterDTO(filter entity.IssueFilter) api.IssueFilter {
	dto := api.IssueFilter{}

	if len(filter.All) > 0 {
		all := issueFilterDTOs(filter.All)
		dto.All = &all
	}

	if len(filter.Any) > 0 {
		any := issueFilterDTOs(filter.Any)
		dto.Any = &any
	}

	if filter.Not != nil {
		negated := issueFilterDTO(*filter.Not)
		dto.Not = &negated
	}

	if filter.Field != "" {
		field := api.IssueFilterField(filter.Field)
		dto.Field = &field
	}

	if filter.Op != "" {
		op := api.IssueFilterOp(filter.Op)
		dto.Op = &op
	}

	if len(filter.Values) > 0 {
		values := filter.Values
		dto.Values = &values
	}

	return dto
}

func issueFilterDTOs(filters []entity.IssueFilter) []api.IssueFilter {
	dtos := make([]api.IssueFilter, 0, len(filters))
	for _, filter := range filters {
		dtos = append(dtos, issueFilterDTO(filter))
	}

	return dtos
}

func issueSortDTOs(sort []entity.IssueSort) []api.IssueSort {
	dtos := make([]api.IssueSort, 0, len(sort))

	for _, key := range sort {
		dto := api.IssueSort{Field: api.IssueSortField(key.Field)}

		if key.Descending {
			descending := true
			dto.Descending = &descending
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func issueFilterFrom(dto *api.IssueFilter) *entity.IssueFilter {
	if dto == nil {
		return nil
	}

	filter := entity.IssueFilter{Not: issueFilterFrom(dto.Not)}

	if dto.All != nil {
		filter.All = issueFiltersFrom(*dto.All)
	}

	if dto.Any != nil {
		filter.Any = issueFiltersFrom(*dto.Any)
	}

	if dto.Field != nil {
		filter.Field = entity.IssueFilterField(*dto.Field)
	}

	if dto.Op != nil {
		filter.Op = entity.IssueFilterOp(*dto.Op)
	}

	if dto.Values != nil {
		filter.Values = *dto.Values
	}

	return &filter
}

func issueFiltersFrom(dtos []api.IssueFilter) []entity.IssueFilter {
	filters := make([]entity.IssueFilter, 0, len(dtos))

	for _, dto := range dtos {
		filters = append(filters, *issueFilterFrom(&dto))
	}

	return filters
}

func issueSortFrom(dtos []api.IssueSort) []entity.IssueSort {
	sort := make([]entity.IssueSort, 0, len(dtos))

	for _, dto := range dtos {
		key := entity.IssueSort{Field: entity.IssueSortField(dto.Field)}

		if dto.Descending != nil {
			key.Descending = *dto.Descending
		}

		sort = append(sort, key)
	}

	return sort
}

func apiTokenDTO(token entity.APIToken) api.APIToken {
	dto := api.APIToken{
		Id:        token.ID,
		Name:      token.Name,
		Scopes:    token.Scopes.Strings(),
		Grants:    apiTokenGrantDTOs(token.Grants),
		CreatedAt: token.CreatedAt,
	}

	dto.ExpiresAt = token.ExpiresAt
	dto.LastUsedAt = token.LastUsedAt

	return dto
}

func apiTokenGrantDTOs(grants entity.APITokenGrants) []api.APITokenGrant {
	dtos := make([]api.APITokenGrant, 0, len(grants))

	for _, grant := range grants {
		dto := api.APITokenGrant{WorkspaceId: grant.WorkspaceID, AllTeams: grant.AllTeams}

		if len(grant.TeamIDs) > 0 {
			teamIDs := make([]uuid.UUID, 0, len(grant.TeamIDs))
			teamIDs = append(teamIDs, grant.TeamIDs...)
			dto.TeamIds = &teamIDs
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func apiTokenGrants(dtos []api.APITokenGrant) entity.APITokenGrants {
	grants := make(entity.APITokenGrants, 0, len(dtos))

	for _, dto := range dtos {
		grant := entity.APITokenGrant{WorkspaceID: dto.WorkspaceId, AllTeams: dto.AllTeams}

		if dto.TeamIds != nil {
			grant.TeamIDs = append(grant.TeamIDs, *dto.TeamIds...)
		}

		grants = append(grants, grant)
	}

	return grants
}

func workspaceAPITokenDTOs(tokens []service.OwnedAPIToken) []api.WorkspaceAPIToken {
	dtos := make([]api.WorkspaceAPIToken, 0, len(tokens))

	for _, owned := range tokens {
		dtos = append(dtos, api.WorkspaceAPIToken{
			Token:      apiTokenDTO(owned.Token),
			OwnerName:  owned.OwnerName,
			OwnerEmail: owned.OwnerEmail,
		})
	}

	return dtos
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

func activityEventDTO(event entity.ActivityEvent) api.ActivityEvent {
	dto := api.ActivityEvent{
		Id:          event.ID,
		SubjectKind: api.ActivitySubjectKind(event.Subject.Kind),
		ActorKind:   api.ActivityActorKind(event.Actor.Kind),
		Changes:     activityChangeDTOs(event.Changes),
		CreatedAt:   event.CreatedAt,
	}

	subject := event.Subject.ID

	if event.Subject.Kind == entity.ActivitySubjectProject {
		dto.ProjectId = &subject
	} else {
		dto.IssueId = &subject
	}

	if event.Actor.AccountID != uuid.Nil {
		actor := event.Actor.AccountID
		dto.ActorAccountId = &actor
		dto.ActorName = nilIfEmpty(event.ActorName)
	}

	dto.ActorTokenName = nilIfEmpty(event.Actor.TokenName)
	dto.ActorConnectionName = nilIfEmpty(event.Actor.ConnectionName)

	if event.BulkActionID != uuid.Nil {
		bulk := event.BulkActionID
		dto.BulkActionId = &bulk
	}

	return dto
}

func activityChangeDTOs(changes []entity.Activity) []api.ActivityChange {
	dtos := make([]api.ActivityChange, 0, len(changes))

	for _, change := range changes {
		dto := api.ActivityChange{
			Id:        change.ID,
			Kind:      api.ActivityKind(change.Kind),
			Field:     nilIfEmpty(change.Field),
			FromValue: nilIfEmpty(change.FromValue),
			ToValue:   nilIfEmpty(change.ToValue),
			FromState: nilIfEmpty(change.FromState),
			ToState:   nilIfEmpty(change.ToState),
		}

		if change.Version > 0 {
			version := int32(change.Version)
			dto.Version = &version
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func activityPageDTO(page service.ActivityPage) api.ActivityPage {
	events := make([]api.ActivityEvent, 0, len(page.Events))

	for _, event := range page.Events {
		events = append(events, activityEventDTO(event))
	}

	dto := api.ActivityPage{Events: events}

	if page.NextCursor != "" {
		cursor := page.NextCursor
		dto.NextCursor = &cursor
	}

	return dto
}

func textOf(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func commentDTO(comment entity.IssueComment) api.IssueComment {
	dto := api.IssueComment{
		Id:         comment.ID,
		IssueId:    comment.IssueID,
		AuthorKind: api.CommentAuthorKind(comment.AuthorKind),
		Body:       comment.Body,
		Edited:     comment.Edited(),
		Deleted:    comment.Deleted(),
		EditedAt:   comment.EditedAt,
		DeletedAt:  comment.DeletedAt,
		Mentions:   commentMentionDTOs(comment.Mentions),
		Reactions:  commentReactionDTOs(comment.Reactions),
		Replies:    commentDTOs(comment.Replies),
		CreatedAt:  comment.CreatedAt,
	}

	if comment.ParentCommentID != uuid.Nil {
		parent := comment.ParentCommentID
		dto.ParentCommentId = &parent
	}

	if comment.AuthorAccountID != uuid.Nil {
		author := comment.AuthorAccountID
		dto.AuthorAccountId = &author
		dto.AuthorName = nilIfEmpty(comment.AuthorName)
	}

	return dto
}

func commentDTOs(comments []entity.IssueComment) []api.IssueComment {
	dtos := make([]api.IssueComment, 0, len(comments))
	for _, comment := range comments {
		dtos = append(dtos, commentDTO(comment))
	}

	return dtos
}

func commentPageDTO(thread service.CommentThread) api.IssueCommentPage {
	return api.IssueCommentPage{
		Comments:   commentDTOs(thread.Comments),
		NextCursor: nilIfEmpty(thread.NextCursor),
	}
}

func postedCommentDTO(posted service.CommentPosted) api.PostedComment {
	return api.PostedComment{
		Comment:     commentDTO(posted.Comment),
		Unreachable: commentMentionDTOs(posted.Unreachable),
	}
}

func commentMentionDTOs(mentions []entity.CommentMention) []api.CommentMention {
	dtos := make([]api.CommentMention, 0, len(mentions))

	for _, mention := range mentions {
		dto := api.CommentMention{
			Kind:     api.CommentMentionKind(mention.Kind),
			Name:     mention.Name,
			Notified: mention.Visible,
		}

		if mention.AccountID != uuid.Nil {
			account := mention.AccountID
			dto.AccountId = &account
		}

		if mention.TeamID != uuid.Nil {
			team := mention.TeamID
			dto.TeamId = &team
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func commentReactionDTOs(tallies []entity.CommentReactionTally) []api.CommentReactionTally {
	dtos := make([]api.CommentReactionTally, 0, len(tallies))

	for _, tally := range tallies {
		dtos = append(dtos, api.CommentReactionTally{
			Reaction:   api.CommentReaction(tally.Reaction),
			AccountIds: tally.Accounts,
		})
	}

	return dtos
}

func mentionInputs(targets []api.MentionTarget) []service.CommentMentionInput {
	inputs := make([]service.CommentMentionInput, 0, len(targets))

	for _, target := range targets {
		input := service.CommentMentionInput{Kind: entity.MentionKind(target.Kind)}

		if target.AccountId != nil {
			input.AccountID = *target.AccountId
		}

		if target.TeamId != nil {
			input.TeamID = *target.TeamId
		}

		inputs = append(inputs, input)
	}

	return inputs
}

func attachmentDTO(attachment entity.Attachment) api.Attachment {
	dto := api.Attachment{
		Id:          attachment.ID,
		WorkspaceId: attachment.WorkspaceID,
		IssueId:     attachment.IssueID,
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		ByteSize:    attachment.SizeBytes,
		Status:      api.AttachmentStatus(attachment.Status),
		Inline:      attachment.Inline(),
		ContentPath: attachmentContentPath(attachment),
		CreatedAt:   attachment.CreatedAt,
	}

	if attachment.CommentID != uuid.Nil {
		comment := attachment.CommentID
		dto.CommentId = &comment
	}

	if attachment.UploaderID != uuid.Nil {
		uploader := attachment.UploaderID
		dto.UploadedByAccountId = &uploader
		dto.UploadedByName = nilIfEmpty(attachment.UploaderName)
	}

	return dto
}

func attachmentContentPath(attachment entity.Attachment) string {
	return entity.AttachmentContentPath(attachment.WorkspaceID, attachment.ID)
}

func attachmentDTOs(attachments []entity.Attachment) []api.Attachment {
	dtos := make([]api.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		dtos = append(dtos, attachmentDTO(attachment))
	}

	return dtos
}

func attachmentReservationDTO(reservation service.AttachmentReservation) api.AttachmentReservation {
	return api.AttachmentReservation{
		Attachment: attachmentDTO(reservation.Attachment),
		Transfer: api.AttachmentTransfer{
			Url:       reservation.Transfer.URL,
			Method:    reservation.Transfer.Method,
			Headers:   reservation.Transfer.Headers,
			ExpiresAt: reservation.Transfer.ExpiresAt,
		},
	}
}

func workspaceStorageDTO(ledger entity.WorkspaceStorage) api.WorkspaceStorage {
	dto := api.WorkspaceStorage{
		StoredBytes: ledger.StoredBytes,
		Unlimited:   ledger.Unlimited(),
		UpdatedAt:   &ledger.UpdatedAt,
	}

	if !ledger.Unlimited() {
		maxBytes := ledger.MaxBytes
		dto.MaxBytes = &maxBytes
	}

	return dto
}

func notificationPageDTO(inbox service.Inbox) api.NotificationPage {
	notifications := make([]api.Notification, 0, len(inbox.Notifications))
	for _, notification := range inbox.Notifications {
		notifications = append(notifications, notificationDTO(notification))
	}

	return api.NotificationPage{
		Notifications: notifications,
		Unread:        int32(inbox.Unread),
		NextCursor:    nilIfEmpty(inbox.NextCursor),
	}
}

func notificationDTO(notification entity.Notification) api.Notification {
	dto := api.Notification{
		SubjectKind: api.NotificationSubjectKind(notification.Subject.Kind),
		SubjectId:   notification.Subject.ID,
		Kind:        api.NotificationKind(notification.Kind),
		Reason:      api.NotificationReason(notification.Reason),
		ActorKind:   api.NotificationActorKind(notification.ActorKind),
		Title:       notification.Title,
		Reference:   nilIfEmpty(notification.Reference),
		TeamKey:     nilIfEmpty(notification.TeamKey),
		UnreadCount: int32(notification.UnreadCount),
		LastEventAt: notification.LastEventAt,
	}

	if notification.Actor != uuid.Nil {
		actor := notification.Actor
		dto.ActorAccountId = &actor
		dto.ActorName = nilIfEmpty(notification.ActorName)
	}

	if !notification.SnoozedUntil.IsZero() {
		snoozed := notification.SnoozedUntil
		dto.SnoozedUntil = &snoozed
	}

	return dto
}

func issueFollowDTO(follower entity.IssueFollower) api.IssueFollow {
	state := follower.State
	if state == "" {
		state = entity.FollowStateMuted
	}

	return api.IssueFollow{State: api.FollowState(state)}
}

func notificationSettingsDTO(view service.NotificationSettingsView) api.NotificationSettings {
	return api.NotificationSettings{
		Preferences: notificationPreferencesDTO(view.Team),
		Workspace:   notificationPreferencesDTO(view.Global),
		Overridden:  view.TeamOverridden,
	}
}

func notificationPreferencesDTO(preferences entity.NotificationPreferences) api.NotificationPreferences {
	return api.NotificationPreferences{
		Assigned:     notificationChannelsDTO(preferences.Assigned),
		Mentioned:    notificationChannelsDTO(preferences.Mentioned),
		Commented:    notificationChannelsDTO(preferences.Commented),
		StateChanged: notificationChannelsDTO(preferences.StateChanged),
		Membership:   notificationChannelsDTO(preferences.Membership),
		Approvals:    notificationChannelsDTO(preferences.Approvals),
		Agents:       notificationChannelsDTO(preferences.Agents),
	}
}

func notificationChannelsDTO(channels entity.NotificationChannels) api.NotificationChannels {
	return api.NotificationChannels{Inbox: channels.Inbox, Email: channels.Email}
}

func notificationPreferences(dto api.NotificationPreferences) entity.NotificationPreferences {
	return entity.NotificationPreferences{
		Assigned:     notificationChannels(dto.Assigned),
		Mentioned:    notificationChannels(dto.Mentioned),
		Commented:    notificationChannels(dto.Commented),
		StateChanged: notificationChannels(dto.StateChanged),
		Membership:   notificationChannels(dto.Membership),
		Approvals:    notificationChannels(dto.Approvals),
		Agents:       notificationChannels(dto.Agents),
	}
}

func notificationChannels(dto api.NotificationChannels) entity.NotificationChannels {
	return entity.NotificationChannels{Inbox: dto.Inbox, Email: dto.Email}
}

func searchResultsDTO(results entity.SearchResults) api.SearchResults {
	groups := make([]api.SearchGroup, 0, len(results.Groups))
	for _, group := range results.Groups {
		groups = append(groups, searchGroupDTO(group))
	}

	return api.SearchResults{Query: results.Query.Raw, Fuzzy: results.Fuzzy, Groups: groups}
}

func searchGroupDTO(group entity.SearchGroup) api.SearchGroup {
	results := make([]api.SearchResult, 0, len(group.Results))
	for _, result := range group.Results {
		results = append(results, searchResultDTO(result))
	}

	return api.SearchGroup{
		Kind:    api.SearchKind(group.Kind),
		More:    group.More,
		Results: results,
	}
}

func searchResultDTO(result entity.SearchResult) api.SearchResult {
	dto := api.SearchResult{
		Kind:      api.SearchKind(result.Kind),
		Id:        result.ID,
		Title:     result.Title,
		Excerpt:   nilIfEmpty(result.Excerpt),
		Reference: nilIfEmpty(result.Reference),
		TeamKey:   nilIfEmpty(result.TeamKey),
		Slug:      nilIfEmpty(result.Slug),
		Status:    nilIfEmpty(result.Status),
		TitleHit:  result.TitleHit,
		UpdatedAt: result.UpdatedAt,
	}

	if result.IssueID != uuid.Nil {
		issueID := result.IssueID
		dto.IssueId = &issueID
	}

	return dto
}

func IssueEvent(issue entity.Issue) api.Issue {
	return issueDTO(issue)
}

func CommentEvent(comment entity.IssueComment) api.IssueComment {
	return commentDTO(comment)
}

func ExecutionEventDTO(execution entity.Execution) api.Execution {
	return executionDTO(execution)
}

func QuestionEvent(question entity.IssueQuestion) api.IssueQuestion {
	return issueQuestionDTO(question)
}

func ExecutionTimelineEvent(event entity.ExecutionEvent) api.ExecutionEvent {
	return executionEventDTO(event)
}

func agentDTO(agent entity.Agent) api.Agent {
	dto := api.Agent{
		Id:             agent.ID,
		WorkspaceId:    agent.WorkspaceID,
		AccountId:      agent.AccountID,
		OwnerAccountId: agent.OwnerAccountID,
		Name:           agent.Name,
		Status:         api.AgentStatus(agent.Status),
		ActionLimit:    int32(agent.Allowance()),
		CreatedAt:      agent.CreatedAt,
	}

	dto.DisabledAt = agent.DisabledAt

	return dto
}

func workspaceAgentDTO(owned service.OwnedAgent) api.WorkspaceAgent {
	return api.WorkspaceAgent{
		Agent:      agentDTO(owned.Agent),
		OwnerName:  owned.OwnerName,
		OwnerEmail: owned.OwnerEmail,
	}
}

func workspaceAgentDTOs(agents []service.OwnedAgent) []api.WorkspaceAgent {
	dtos := make([]api.WorkspaceAgent, 0, len(agents))

	for _, owned := range agents {
		dtos = append(dtos, workspaceAgentDTO(owned))
	}

	return dtos
}

func agentSettingsDTO(settings entity.AgentSettings) api.AgentSettings {
	return api.AgentSettings{
		HoldComments:      api.AgentHold(settings.HoldComments),
		HoldStateChanges:  api.AgentHold(settings.HoldStateChanges),
		HoldIssueEdits:    api.AgentHold(settings.HoldIssueEdits),
		HoldIssueCreation: api.AgentHold(settings.HoldIssueCreation),
	}
}

func issueDelegationDTO(delegation entity.IssueDelegation) api.IssueDelegation {
	dto := api.IssueDelegation{
		Id:             delegation.ID,
		IssueId:        delegation.IssueID,
		AgentId:        delegation.AgentID,
		AgentName:      delegation.AgentName,
		AgentAccountId: delegation.AgentAccountID,
		Brief:          nilIfEmpty(delegation.Brief),
		DelegatedAt:    delegation.DelegatedAt,
		RecalledAt:     delegation.RecalledAt,
	}

	if delegation.DelegatedByAccountID != uuid.Nil {
		author := delegation.DelegatedByAccountID
		dto.DelegatedByAccountId = &author
	}

	if delegation.RecalledByAccountID != uuid.Nil {
		recaller := delegation.RecalledByAccountID
		dto.RecalledByAccountId = &recaller
	}

	return dto
}

func issueDelegationDTOs(delegations []entity.IssueDelegation) []api.IssueDelegation {
	dtos := make([]api.IssueDelegation, 0, len(delegations))

	for _, delegation := range delegations {
		dtos = append(dtos, issueDelegationDTO(delegation))
	}

	return dtos
}

func agentProposalDTO(proposal entity.AgentProposal) api.AgentProposal {
	dto := api.AgentProposal{
		Id:        proposal.ID,
		AgentId:   proposal.AgentID,
		AgentName: proposal.AgentName,
		TeamId:    proposal.TeamID,
		Action:    api.AgentAction(proposal.Action),
		Status:    api.AgentProposalStatus(proposal.Status),
		CreatedAt: proposal.CreatedAt,
	}

	if proposal.IssueID != uuid.Nil {
		issueID := proposal.IssueID
		dto.IssueId = &issueID
	}

	dto.Body = nilIfEmpty(proposal.Change.Body)
	dto.StateId = proposal.Change.StateID
	dto.Title = proposal.Change.Title
	dto.Description = proposal.Change.Description

	dto.Failure = nilIfEmpty(proposal.Failure)
	dto.Reasoning = agentReasoningDTO(proposal.Reasoning)
	dto.DecidedAt = proposal.DecidedAt

	if proposal.DecidedBy != uuid.Nil {
		decider := proposal.DecidedBy
		dto.DecidedByAccountId = &decider
	}

	return dto
}

func agentReasoningDTO(reasoning entity.AgentReasoning) *api.AgentReasoning {
	if reasoning.Empty() {
		return nil
	}

	dto := api.AgentReasoning{
		Observed:  nilIfEmpty(reasoning.Observed),
		Uncertain: nilIfEmpty(reasoning.Uncertain),
	}

	if len(reasoning.Consulted) > 0 {
		sources := make([]api.AgentSource, 0, len(reasoning.Consulted))

		for _, source := range reasoning.Consulted {
			sources = append(sources, api.AgentSource{
				Label: source.Label,
				Url:   nilIfEmpty(source.URL),
			})
		}

		dto.Consulted = &sources
	}

	return &dto
}

func waitingProposalDTO(waiting service.WaitingProposal) api.AgentProposal {
	dto := agentProposalDTO(waiting.Proposal)
	dto.Reasoning = agentReasoningDTO(waiting.Proposal.Reasoning)

	if waiting.Team.Key != "" {
		dto.TeamKey = &waiting.Team.Key
	}

	if waiting.Issue.ID != uuid.Nil {
		reference := waiting.Issue.Reference()
		dto.IssueReference = &reference
		dto.IssueTitle = &waiting.Issue.Title
	}

	if len(waiting.Questions) > 0 {
		questions := issueQuestionDTOs(waiting.Questions)
		dto.Questions = &questions
	}

	if waiting.State.Name != "" {
		dto.StateName = &waiting.State.Name
	}

	return dto
}

func issueQuestionDTO(question entity.IssueQuestion) api.IssueQuestion {
	dto := api.IssueQuestion{
		Id:            question.ID,
		IssueId:       question.IssueID,
		Kind:          api.IssueQuestionKind(question.Kind),
		State:         api.IssueQuestionState(question.State),
		Blocking:      question.Blocking,
		AllowFreeText: question.AllowFreeText,
		Question:      question.Question,
		Default:       question.DefaultAnswer,
		Deadline:      question.Deadline,
		Answered:      question.Answered(),
		Expired:       question.Expired(time.Now().UTC()),
		Standing:      question.Standing(),
		ActorKind:     api.NotificationActorKind(question.ActorKind),
		CreatedAt:     question.CreatedAt,
	}

	dto.Answer = nilIfEmpty(question.Answer)
	dto.AskedByName = nilIfEmpty(question.AskedByName)
	dto.AnsweredByName = nilIfEmpty(question.AnsweredByName)
	dto.AnsweredAt = question.AnsweredAt
	dto.SettledByName = nilIfEmpty(question.SettledByName)
	dto.SettledAt = question.SettledAt
	dto.ExecutionId = nilIfEmpty(question.ExecutionID)

	if len(question.Options) > 0 {
		options := question.Options
		dto.Options = &options
	}

	dto.Context = questionContextDTO(question.Context)

	return dto
}

func questionContextDTO(held entity.QuestionContext) *api.IssueQuestionContext {
	if held.Preview == "" && len(held.Files) == 0 && len(held.Artifacts) == 0 {
		return nil
	}

	dto := api.IssueQuestionContext{Preview: nilIfEmpty(held.Preview)}

	if len(held.Files) > 0 {
		files := held.Files
		dto.Files = &files
	}

	if len(held.Artifacts) > 0 {
		artifacts := make([]uuid.UUID, 0, len(held.Artifacts))

		for _, reference := range held.Artifacts {
			parsed, err := uuid.Parse(reference)
			if err != nil {
				continue
			}

			artifacts = append(artifacts, parsed)
		}

		dto.Artifacts = &artifacts
	}

	return &dto
}

func issueQuestionDTOs(questions []entity.IssueQuestion) []api.IssueQuestion {
	dtos := make([]api.IssueQuestion, 0, len(questions))

	for _, question := range questions {
		dtos = append(dtos, issueQuestionDTO(question))
	}

	return dtos
}

func waitingProposalDTOs(waiting []service.WaitingProposal) []api.AgentProposal {
	dtos := make([]api.AgentProposal, 0, len(waiting))

	for _, held := range waiting {
		dtos = append(dtos, waitingProposalDTO(held))
	}

	return dtos
}

func agentReasoningFrom(body *api.AgentReasoning) entity.AgentReasoning {
	if body == nil {
		return entity.AgentReasoning{}
	}

	reasoning := entity.AgentReasoning{}

	if body.Observed != nil {
		reasoning.Observed = *body.Observed
	}

	if body.Uncertain != nil {
		reasoning.Uncertain = *body.Uncertain
	}

	if body.Consulted != nil {
		for _, source := range *body.Consulted {
			consulted := entity.AgentSource{Label: source.Label}

			if source.Url != nil {
				consulted.URL = *source.Url
			}

			reasoning.Consulted = append(reasoning.Consulted, consulted)
		}
	}

	return reasoning
}

func runnerHostOf(host api.RunnerHost) entity.RunnerHost {
	return entity.RunnerHost{
		Hostname: host.Hostname,
		OS:       host.Os,
		Arch:     host.Arch,
		Version:  host.Version,
	}
}

func runnerDTO(runner entity.Runner) api.Runner {
	return api.Runner{
		Id:          runner.ID,
		WorkspaceId: runner.WorkspaceID,
		AgentId:     runner.AgentID,
		AgentName:   runner.AgentName,
		Name:        runner.Name,
		Host: api.RunnerHost{
			Hostname: runner.Host.Hostname,
			Os:       runner.Host.OS,
			Arch:     runner.Host.Arch,
			Version:  runner.Host.Version,
		},
		Status:     api.RunnerStatus(runner.Status),
		EnrolledAt: runner.EnrolledAt,
		LastSeenAt: runner.LastSeenAt,
		RevokedAt:  runner.RevokedAt,
	}
}

func runnerDTOs(runners []entity.Runner) []api.Runner {
	dtos := make([]api.Runner, 0, len(runners))

	for _, runner := range runners {
		dtos = append(dtos, runnerDTO(runner))
	}

	return dtos
}

func codebaseRepositoriesOf(repositories *[]api.CodebaseRepository) []entity.CodebaseRepository {
	if repositories == nil {
		return nil
	}

	held := make([]entity.CodebaseRepository, 0, len(*repositories))

	for _, repository := range *repositories {
		mapped := entity.CodebaseRepository{Name: repository.Name, RelPath: repository.RelPath}

		if repository.DefaultBranch != nil {
			mapped.DefaultBranch = *repository.DefaultBranch
		}

		if repository.Remote != nil {
			mapped.Remote = entity.RemoteFingerprint{
				Hash:     textOf(repository.Remote.Hash),
				Host:     textOf(repository.Remote.Host),
				PathTail: textOf(repository.Remote.PathTail),
			}
		}

		held = append(held, mapped)
	}

	return held
}

func sharedFilesOf(files *[]string) []string {
	if files == nil {
		return nil
	}

	return *files
}

func codebaseRuntimesOf(runtimes *[]api.CodebaseRuntime) []entity.CodebaseRuntime {
	if runtimes == nil {
		return nil
	}

	held := make([]entity.CodebaseRuntime, 0, len(*runtimes))
	for _, runtime := range *runtimes {
		held = append(held, entity.CodebaseRuntime(runtime))
	}

	return held
}

func codingToolsOf(tools *[]api.CodingTool) []entity.CodingTool {
	if tools == nil {
		return nil
	}

	held := make([]entity.CodingTool, 0, len(*tools))
	for _, tool := range *tools {
		held = append(held, entity.CodingTool{Name: tool.Name, Version: textOf(tool.Version)})
	}

	return held
}

func codebaseDTO(codebase entity.Codebase) api.Codebase {
	repositories := make([]api.CodebaseRepository, 0, len(codebase.Repositories))

	for _, repository := range codebase.Repositories {
		mapped := api.CodebaseRepository{
			Name:          repository.Name,
			RelPath:       repository.RelPath,
			DefaultBranch: nilIfEmpty(repository.DefaultBranch),
		}

		if repository.Remote != (entity.RemoteFingerprint{}) {
			mapped.Remote = &api.RemoteFingerprint{
				Hash:     nilIfEmpty(repository.Remote.Hash),
				Host:     nilIfEmpty(repository.Remote.Host),
				PathTail: nilIfEmpty(repository.Remote.PathTail),
			}
		}

		repositories = append(repositories, mapped)
	}

	runtimes := make([]api.CodebaseRuntime, 0, len(codebase.Runtimes))
	for _, runtime := range codebase.Runtimes {
		runtimes = append(runtimes, api.CodebaseRuntime(runtime))
	}

	tools := make([]api.CodingTool, 0, len(codebase.Tools))
	for _, tool := range codebase.Tools {
		tools = append(tools, api.CodingTool{Name: tool.Name, Version: nilIfEmpty(tool.Version)})
	}

	shared := codebase.SharedFiles
	if shared == nil {
		shared = []string{}
	}

	return api.Codebase{
		Id:             codebase.ID,
		RunnerId:       codebase.RunnerID,
		WorkspaceId:    codebase.WorkspaceID,
		AgentId:        codebase.AgentID,
		Name:           codebase.Name,
		RootPath:       codebase.RootPath,
		State:          api.CodebaseState(codebase.State),
		Repositories:   repositories,
		SharedFiles:    shared,
		Runtimes:       runtimes,
		Tools:          tools,
		ConnectedAt:    codebase.ConnectedAt,
		LastSeenAt:     codebase.LastSeenAt,
		DisconnectedAt: codebase.DisconnectedAt,
	}
}

func codebaseDTOs(codebases []entity.Codebase) []api.Codebase {
	dtos := make([]api.Codebase, 0, len(codebases))

	for _, codebase := range codebases {
		dtos = append(dtos, codebaseDTO(codebase))
	}

	return dtos
}

func nilIfNilID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}

func executionDTO(execution entity.Execution) api.Execution {
	restartable := execution.Restartable()

	return api.Execution{
		Id:             execution.ID,
		Reference:      execution.Reference(),
		WorkspaceId:    execution.WorkspaceID,
		IssueId:        execution.IssueID,
		IssueReference: execution.IssueReference,
		TeamId:         nilIfNilID(execution.TeamID),
		DelegationId:   nilIfNilID(execution.DelegationID),
		AgentId:        nilIfNilID(execution.AgentID),
		AgentName:      nilIfEmpty(execution.AgentName),
		RunnerId:       nilIfNilID(execution.RunnerID),
		CodebaseId:     nilIfNilID(execution.CodebaseID),
		Attempt:        execution.Attempt,
		State:          api.ExecutionState(execution.State),
		Reason:         nilIfEmpty(execution.Reason),
		Params:         executionParamsDTO(execution.Params),
		Restartable:    &restartable,
		LeaseExpiresAt: execution.LeaseExpiresAt,
		QueuedAt:       execution.QueuedAt,
		StartedAt:      execution.StartedAt,
		FinishedAt:     execution.FinishedAt,
	}
}

func executionDTOs(executions []entity.Execution) []api.Execution {
	dtos := make([]api.Execution, 0, len(executions))

	for _, execution := range executions {
		dtos = append(dtos, executionDTO(execution))
	}

	return dtos
}

func executionParamsDTO(params entity.ExecutionParams) api.ExecutionParams {
	dto := api.ExecutionParams{
		Tool:  nilIfEmpty(params.Tool),
		Model: nilIfEmpty(params.Model),
		Brief: nilIfEmpty(params.Brief),
	}

	if params.Runtime != "" {
		runtime := api.CodebaseRuntime(params.Runtime)
		dto.Runtime = &runtime
	}

	return dto
}

func executionActorDTO(actor entity.ExecutionActor) api.ExecutionActor {
	return api.ExecutionActor{
		Kind:      api.ActivityActorKind(actor.Kind),
		AccountId: nilIfNilID(actor.AccountID),
		AgentId:   nilIfNilID(actor.AgentID),
		RunnerId:  nilIfNilID(actor.RunnerID),
	}
}

func executionEventDTO(event entity.ExecutionEvent) api.ExecutionEvent {
	dto := api.ExecutionEvent{
		Id:          event.ID,
		ExecutionId: event.ExecutionID,
		Sequence:    event.Sequence,
		Kind:        api.ExecutionEventKind(event.Kind),
		Actor:       executionActorDTO(event.Actor),
		Reason:      nilIfEmpty(event.Reason),
		Detail:      executionDetailDTO(event.Detail),
		OccurredAt:  event.OccurredAt,
		RecordedAt:  &event.RecordedAt,
	}

	if event.FromState != "" {
		from := api.ExecutionState(event.FromState)
		dto.FromState = &from
	}

	if event.ToState != "" {
		to := api.ExecutionState(event.ToState)
		dto.ToState = &to
	}

	return dto
}

func executionDetailDTO(detail []byte) *map[string]any {
	if len(detail) == 0 {
		return nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(detail, &decoded); err != nil || decoded == nil {
		return nil
	}

	return &decoded
}

func executionEventDTOs(events []entity.ExecutionEvent) []api.ExecutionEvent {
	dtos := make([]api.ExecutionEvent, 0, len(events))

	for _, event := range events {
		dtos = append(dtos, executionEventDTO(event))
	}

	return dtos
}

func chunkPageOf(after *int64, limit *int) entity.ExecutionChunkPage {
	page := entity.ExecutionChunkPage{}

	if after != nil {
		page.After = *after
	}

	if limit != nil {
		page.Limit = *limit
	}

	return page
}

func logEntriesOf(entries []api.ExecutionLogEntry) []entity.ExecutionLogEntry {
	held := make([]entity.ExecutionLogEntry, 0, len(entries))

	for _, entry := range entries {
		held = append(held, entity.ExecutionLogEntry{
			At:     timeOrZero(entry.At),
			Stream: textOf(entry.Stream),
			Source: textOf(entry.Source),
			Text:   entry.Text,
		})
	}

	return held
}

func transcriptEntriesOf(entries []api.ExecutionTranscriptEntry) []entity.ExecutionTranscriptEntry {
	held := make([]entity.ExecutionTranscriptEntry, 0, len(entries))

	for _, entry := range entries {
		held = append(held, entity.ExecutionTranscriptEntry{
			At:      timeOrZero(entry.At),
			Type:    entry.Type,
			Payload: payloadOf(entry.Payload),
		})
	}

	return held
}

func payloadOf(payload *map[string]any) map[string]any {
	if payload == nil {
		return nil
	}

	return *payload
}

func timeOrZero(at *time.Time) time.Time {
	if at == nil {
		return time.Time{}
	}

	return *at
}

func chunkReceiptDTO(receipt service.ExecutionReceipt) api.ExecutionChunkReceipt {
	return api.ExecutionChunkReceipt{
		Stream:     api.ExecutionStream(receipt.Chunk.Stream),
		Sequence:   receipt.Chunk.Sequence,
		Digest:     receipt.Chunk.Digest,
		EntryCount: receipt.Chunk.Entries,
		Bytes:      receipt.Chunk.Bytes,
		Duplicate:  receipt.Duplicate,
		ReceivedAt: receipt.Chunk.ReceivedAt,
	}
}

func streamCursorDTOs(cursors []entity.ExecutionStreamCursor) []api.ExecutionStreamCursor {
	dtos := make([]api.ExecutionStreamCursor, 0, len(cursors))

	for _, cursor := range cursors {
		dtos = append(dtos, api.ExecutionStreamCursor{
			Stream:       api.ExecutionStream(cursor.Stream),
			LastSequence: cursor.LastSequence,
			Chunks:       cursor.Chunks,
			EntryCount:   cursor.Entries,
			Bytes:        cursor.Bytes,
		})
	}

	return dtos
}

func logChunkDTOs(chunks []entity.ExecutionLogChunk) []api.ExecutionLogChunk {
	dtos := make([]api.ExecutionLogChunk, 0, len(chunks))

	for _, chunk := range chunks {
		entries := make([]api.ExecutionLogEntry, 0, len(chunk.Entries))

		for _, entry := range chunk.Entries {
			at := entry.At
			entries = append(entries, api.ExecutionLogEntry{
				At:     &at,
				Stream: nilIfEmpty(entry.Stream),
				Source: nilIfEmpty(entry.Source),
				Text:   entry.Text,
			})
		}

		dtos = append(dtos, api.ExecutionLogChunk{
			Stream:     api.ExecutionStream(chunk.Stream),
			Sequence:   chunk.Sequence,
			Digest:     chunk.Digest,
			EntryCount: chunk.ExecutionChunk.Entries,
			Bytes:      chunk.Bytes,
			FirstAt:    chunk.FirstAt,
			LastAt:     chunk.LastAt,
			ReceivedAt: chunk.ReceivedAt,
			Entries:    entries,
		})
	}

	return dtos
}

func transcriptChunkDTOs(chunks []entity.ExecutionTranscriptChunk) []api.ExecutionTranscriptChunk {
	dtos := make([]api.ExecutionTranscriptChunk, 0, len(chunks))

	for _, chunk := range chunks {
		entries := make([]api.ExecutionTranscriptEntry, 0, len(chunk.Entries))

		for _, entry := range chunk.Entries {
			at := entry.At
			held := api.ExecutionTranscriptEntry{At: &at, Type: entry.Type}

			if entry.Payload != nil {
				payload := entry.Payload
				held.Payload = &payload
			}

			entries = append(entries, held)
		}

		dtos = append(dtos, api.ExecutionTranscriptChunk{
			Stream:     api.ExecutionStream(chunk.Stream),
			Sequence:   chunk.Sequence,
			Digest:     chunk.Digest,
			EntryCount: chunk.ExecutionChunk.Entries,
			Bytes:      chunk.Bytes,
			FirstAt:    chunk.FirstAt,
			LastAt:     chunk.LastAt,
			ReceivedAt: chunk.ReceivedAt,
			Entries:    entries,
		})
	}

	return dtos
}

func artifactDTO(artifact entity.ExecutionArtifact) api.ExecutionArtifact {
	return api.ExecutionArtifact{
		Id:          artifact.ID,
		ExecutionId: artifact.ExecutionID,
		Name:        artifact.Name,
		ContentType: artifact.ContentType,
		Bytes:       artifact.Bytes,
		Digest:      artifact.Digest,
		CreatedAt:   artifact.CreatedAt,
	}
}

func artifactDTOs(artifacts []entity.ExecutionArtifact) []api.ExecutionArtifact {
	dtos := make([]api.ExecutionArtifact, 0, len(artifacts))

	for _, artifact := range artifacts {
		dtos = append(dtos, artifactDTO(artifact))
	}

	return dtos
}

func artifactReceiptDTO(receipt service.ArtifactReceipt) api.ExecutionArtifactReceipt {
	return api.ExecutionArtifactReceipt{
		Id:          receipt.Artifact.ID,
		ExecutionId: receipt.Artifact.ExecutionID,
		Name:        receipt.Artifact.Name,
		ContentType: receipt.Artifact.ContentType,
		Bytes:       receipt.Artifact.Bytes,
		Digest:      receipt.Artifact.Digest,
		CreatedAt:   receipt.Artifact.CreatedAt,
		Duplicate:   receipt.Duplicate,
	}
}

func executionPolicyDTO(policy entity.WorkspaceExecutionPolicy) api.WorkspaceExecutionPolicy {
	return api.WorkspaceExecutionPolicy{
		WorkspaceId:         policy.WorkspaceID,
		Telemetry:           api.TelemetryMode(policy.Telemetry),
		UploadRetentionDays: policy.UploadRetentionDays,
	}
}

func changeSetDTO(executionID string, changeset entity.ExecutionChangeSet) api.ExecutionChangeSet {
	dto := api.ExecutionChangeSet{
		ExecutionId:  executionID,
		Repositories: repositoryChangeDTOs(changeset.Changes),
		Validation:   validationDTOs(changeset.Validations),
	}

	if changeset.Result.ExecutionID != "" {
		dto.Summary = nilIfEmpty(changeset.Result.Summary)
		dto.ReportedAt = &changeset.Result.ReportedAt
	}

	return dto
}

func repositoryChangeDTO(change entity.ExecutionChange) api.ExecutionRepositoryChange {
	return api.ExecutionRepositoryChange{
		Repository:     change.Repository,
		Branch:         nilIfEmpty(change.Branch),
		BaseSha:        nilIfEmpty(change.BaseSHA),
		HeadSha:        nilIfEmpty(change.HeadSHA),
		Commits:        change.Commits,
		Additions:      change.Additions,
		Deletions:      change.Deletions,
		FilesChanged:   change.FilesChanged,
		DiffArtifactId: nilIfNoID(change.DiffArtifactID),
		PullRequestUrl: nilIfEmpty(change.PullRequestURL),
		CodeLinkId:     nilIfNoID(change.CodeLinkID),
		ReportedAt:     change.ReportedAt,
	}
}

func repositoryChangeDTOs(changes []entity.ExecutionChange) []api.ExecutionRepositoryChange {
	dtos := make([]api.ExecutionRepositoryChange, 0, len(changes))

	for _, change := range changes {
		dtos = append(dtos, repositoryChangeDTO(change))
	}

	return dtos
}

func validationDTO(validation entity.ExecutionValidation) api.ExecutionValidation {
	return api.ExecutionValidation{
		Check:      validation.Check,
		Status:     api.ValidationStatus(validation.Status),
		Detail:     nilIfEmpty(validation.Detail),
		ArtifactId: nilIfNoID(validation.ArtifactID),
		ReportedAt: validation.ReportedAt,
	}
}

func validationDTOs(validations []entity.ExecutionValidation) []api.ExecutionValidation {
	dtos := make([]api.ExecutionValidation, 0, len(validations))

	for _, validation := range validations {
		dtos = append(dtos, validationDTO(validation))
	}

	return dtos
}

func issueChangeSetDTO(changeset entity.IssueChangeSet) api.IssueChangeSet {
	repositories := make([]api.IssueRepositoryChange, 0, len(changeset.Changes))

	for _, change := range changeset.Changes {
		one := repositoryChangeDTO(change.ExecutionChange)

		repositories = append(repositories, api.IssueRepositoryChange{
			Repository:     one.Repository,
			Branch:         one.Branch,
			BaseSha:        one.BaseSha,
			HeadSha:        one.HeadSha,
			Commits:        one.Commits,
			Additions:      one.Additions,
			Deletions:      one.Deletions,
			FilesChanged:   one.FilesChanged,
			DiffArtifactId: one.DiffArtifactId,
			PullRequestUrl: one.PullRequestUrl,
			CodeLinkId:     one.CodeLinkId,
			ReportedAt:     one.ReportedAt,
			ExecutionId:    change.ExecutionID,
			Attempt:        change.Attempt,
		})
	}

	return api.IssueChangeSet{IssueId: changeset.IssueID, Repositories: repositories}
}

func nilIfNoID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}
