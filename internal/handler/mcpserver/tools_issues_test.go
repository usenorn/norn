package mcpserver_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type issueScene struct {
	workspace entity.Workspace
	team      entity.Team
}

func sceneFor(t *testing.T, h *harness) issueScene {
	t.Helper()

	workspace := entity.Workspace{ID: uuid.New(), Slug: "acme", Name: "Acme"}
	team := entity.Team{ID: uuid.New(), WorkspaceID: workspace.ID, Key: "GAM", Status: entity.TeamStatusActive}

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), h.actor.AccountID).
		Return([]entity.Workspace{workspace}, nil).
		AnyTimes()

	h.teams.EXPECT().
		List(gomock.Any(), workspace.ID, gomock.Any()).
		Return([]entity.Team{team}, nil).
		AnyTimes()

	return issueScene{workspace: workspace, team: team}
}

func callTool(t *testing.T, h *harness, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()

	result, err := h.session(t).CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}

	return result
}

func refusalText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	payload, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}

	return string(payload)
}

func expectRefusal(t *testing.T, result *mcp.CallToolResult, phrases ...string) {
	t.Helper()

	if !result.IsError {
		t.Fatalf("the call succeeded, want a refusal carrying %v", phrases)
	}

	text := refusalText(t, result)

	for _, phrase := range phrases {
		if !strings.Contains(text, phrase) {
			t.Fatalf("the refusal %s does not carry %q", text, phrase)
		}
	}
}

func raisingInto(h *harness, scene issueScene, raised *service.CreateIssueInput, failure error) {
	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in service.CreateIssueInput) (entity.Issue, error) {
			*raised = in

			if failure != nil {
				return entity.Issue{}, failure
			}

			return entity.Issue{
				ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
				ReferenceKey: "GAM", Number: 3, Title: in.Title, AssigneeAccountID: in.AssigneeAccountID,
			}, nil
		})
}

func TestMeIsTheOwnerOfAnAgentAndNeverTheAgentAccount(t *testing.T) {
	h := newHarness(t)

	agentAccount := uuid.New()
	owner := uuid.New()

	h.actor = entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      agentAccount,
		OwnerAccountID: owner,
		Scopes:         toolScopes,
	}

	scene := sceneFor(t, h)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, nil)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "Mine to do", "assignee": "me",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if raised.AssigneeAccountID == agentAccount {
		t.Fatal(
			"me was assigned to the agent's own account. The person who asked an agent to " +
				"assign something to them means themselves, and an agent account cannot be assigned.",
		)
	}

	if raised.AssigneeAccountID != owner {
		t.Fatalf("me was assigned to %v, want the agent's owner %v", raised.AssigneeAccountID, owner)
	}
}

func TestMeFiltersAnAgentsIssueListByItsOwner(t *testing.T) {
	h := newHarness(t)

	owner := uuid.New()

	h.actor = entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      uuid.New(),
		OwnerAccountID: owner,
		Scopes:         toolScopes,
	}

	scene := sceneFor(t, h)

	var queried service.QueryIssuesInput

	h.issues.EXPECT().
		Query(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, in service.QueryIssuesInput) (service.IssueQueryResult, error) {
			queried = in

			return service.IssueQueryResult{}, nil
		})

	result := callTool(t, h, "norn_list_issues", map[string]any{"workspace": "acme", "assignee": "me"})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if queried.Filter == nil {
		t.Fatal("the list was not filtered by assignee at all")
	}

	matched := slices.ContainsFunc(queried.Filter.All, func(leaf entity.IssueFilter) bool {
		return leaf.Field == entity.IssueFilterFieldAssignee && slices.Contains(leaf.Values, owner.String())
	})

	if !matched {
		t.Fatalf("the assignee filter is %+v, want the agent's owner %v", queried.Filter.All, owner)
	}
}

func TestMeIsThePersonThemselvesForAPersonalToken(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, nil)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "Mine to do", "assignee": "me",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if raised.AssigneeAccountID != h.actor.AccountID {
		t.Fatalf("me was assigned to %v, want the token's own person %v", raised.AssigneeAccountID, h.actor.AccountID)
	}
}

func memberWith(workspaceID uuid.UUID, email string, kind entity.AccountKind) entity.WorkspaceMember {
	return entity.WorkspaceMember{
		AccountKind: kind,
		Membership: entity.Membership{
			ID: uuid.New(), WorkspaceID: workspaceID, AccountID: uuid.New(), Role: entity.MembershipRoleMember,
		},
		DisplayName: email,
		Email:       email,
	}
}

func TestAnAssigneeIsFoundByTheirExactEmailWhateverItsCaseOrSpacing(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	lookalike := memberWith(scene.workspace.ID, "rae@northwind.test.au", entity.AccountKindPerson)
	rae := memberWith(scene.workspace.ID, "rae@northwind.test", entity.AccountKindPerson)

	h.workspaces.EXPECT().
		ListMembers(gomock.Any(), scene.workspace.ID, gomock.Any()).
		Return(service.MemberPage{Members: []entity.WorkspaceMember{lookalike, rae}}, nil)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, nil)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "For Rae", "assignee": "  Rae@Northwind.TEST ",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if raised.AssigneeAccountID != rae.Membership.AccountID {
		t.Fatalf("assigned %v, want the member whose address is exactly rae@northwind.test", raised.AssigneeAccountID)
	}
}

func TestAnEmailThatOnlyResemblesAMembersAddressIsRefused(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	h.workspaces.EXPECT().
		ListMembers(gomock.Any(), scene.workspace.ID, gomock.Any()).
		Return(service.MemberPage{Members: []entity.WorkspaceMember{
			memberWith(scene.workspace.ID, "rae@northwind.test", entity.AccountKindPerson),
		}}, nil)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "For Rae", "assignee": "rae@northwind",
	})

	expectRefusal(t, result, "assignee_not_found")
}

func TestAnEmailBelongingToAnAgentCannotBeAssigned(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	h.workspaces.EXPECT().
		ListMembers(gomock.Any(), scene.workspace.ID, gomock.Any()).
		Return(service.MemberPage{Members: []entity.WorkspaceMember{
			memberWith(scene.workspace.ID, "helper@agents.northwind.test", entity.AccountKindAgent),
		}}, nil)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "For the helper", "assignee": "helper@agents.northwind.test",
	})

	expectRefusal(t, result, "assignee_not_assignable")
}

func TestLabelsCannotBeCombinedWithAddLabelsOrRemoveLabels(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"with add_labels":    {"add_labels": []string{"ui"}},
		"with remove_labels": {"remove_labels": []string{"ui"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			arguments := map[string]any{"workspace": "acme", "issue": "GAM-4", "labels": []string{"bug"}}
			for key, value := range extra {
				arguments[key] = value
			}

			expectRefusal(t, callTool(t, h, "norn_update_issue", arguments), "labels_conflict")
		})
	}
}

func TestOneLabelCannotBeAddedAndRemovedInTheSameCall(t *testing.T) {
	h := newHarness(t)

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace":     "acme",
		"issue":         "GAM-4",
		"add_labels":    []string{"Bug"},
		"remove_labels": []string{" bug "},
	})

	expectRefusal(t, result, "label_added_and_removed")
}

func TestOneLabelNamedByNameAndByIDIsStillCaughtAddedAndRemoved(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	bug := entity.Label{ID: uuid.New(), Name: "bug"}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, "GAM-4").
		Return(entity.Issue{ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID, Version: 2}, nil)

	h.labels.EXPECT().
		List(gomock.Any(), scene.workspace.ID).
		Return([]entity.Label{bug}, nil)

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace":     "acme",
		"issue":         "GAM-4",
		"add_labels":    []string{"bug"},
		"remove_labels": []string{bug.ID.String()},
	})

	expectRefusal(t, result, "label_added_and_removed")
}

func TestAProjectCannotBeSetAndClearedInTheSameCall(t *testing.T) {
	h := newHarness(t)

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace": "acme",
		"issue":     "GAM-4",
		"project":   "atlas",
		"clear":     []string{"project"},
	})

	expectRefusal(t, result, "project_set_and_cleared")
}

func TestACycleCannotBeSetAndClearedInTheSameCall(t *testing.T) {
	h := newHarness(t)

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace": "acme",
		"issue":     "GAM-4",
		"cycle":     uuid.NewString(),
		"clear":     []string{" Cycle "},
	})

	expectRefusal(t, result, "cycle_set_and_cleared")
}

func TestAddingAndRemovingLabelsKeepsTheRestAndUsesTheVersionTheEditProduced(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	kept := entity.Label{ID: uuid.New(), Name: "bug"}
	added := entity.Label{ID: uuid.New(), Name: "ui"}
	dropped := entity.Label{ID: uuid.New(), Name: "stale"}

	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 7, Version: 5, Title: "Blurry", Labels: []entity.Label{kept, dropped},
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, "GAM-7").
		Return(issue, nil)

	var edited service.UpdateIssueInput

	h.issues.EXPECT().
		Update(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.UpdateIssueInput) (entity.Issue, error) {
			edited = in

			out := issue
			out.Version = 6
			out.Title = *in.Title

			return out, nil
		})

	h.labels.EXPECT().
		List(gomock.Any(), scene.workspace.ID).
		Return([]entity.Label{kept, added, dropped}, nil)

	var labelled service.SetIssueLabelsInput

	h.issues.EXPECT().
		SetLabels(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.SetIssueLabelsInput) (entity.Issue, error) {
			labelled = in

			out := issue
			out.Version = 7

			return out, nil
		})

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace":        "acme",
		"issue":            "GAM-7",
		"expected_version": 5,
		"title":            "Sharp",
		"add_labels":       []string{"ui"},
		"remove_labels":    []string{"stale"},
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if edited.ExpectedVersion != 5 {
		t.Fatalf("the edit claimed version %d, want the 5 the caller read", edited.ExpectedVersion)
	}

	if labelled.ExpectedVersion != 6 {
		t.Fatalf(
			"the label change claimed version %d, want 6. The edit before it moved the issue to "+
				"version 6, and claiming 5 again is a conflict with the call's own first step.",
			labelled.ExpectedVersion,
		)
	}

	if !slices.Equal(labelled.LabelIDs, []uuid.UUID{kept.ID, added.ID}) {
		t.Fatalf("the labels became %v, want the kept label and the added one", labelled.LabelIDs)
	}
}

func TestReplacingLabelsWithoutAVersionStillWorksAsItDidBefore(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	ui := entity.Label{ID: uuid.New(), Name: "ui"}

	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 9, Version: 9, Labels: []entity.Label{{ID: uuid.New(), Name: "bug"}},
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, "GAM-9").
		Return(issue, nil)

	h.labels.EXPECT().
		List(gomock.Any(), scene.workspace.ID).
		Return([]entity.Label{ui}, nil)

	var labelled service.SetIssueLabelsInput

	h.issues.EXPECT().
		SetLabels(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.SetIssueLabelsInput) (entity.Issue, error) {
			labelled = in

			return issue, nil
		})

	result := callTool(t, h, "norn_update_issue", map[string]any{
		"workspace": "acme", "issue": "GAM-9", "labels": []string{"ui"},
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if labelled.ExpectedVersion != 9 || !slices.Equal(labelled.LabelIDs, []uuid.UUID{ui.ID}) {
		t.Fatalf("the labels were set with %+v, want version 9 and only ui", labelled)
	}
}

func TestClosingAnIssueWithOpenSubIssuesNeedsAnExplicitAcknowledgement(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	done := entity.WorkflowState{ID: uuid.New(), TeamID: scene.team.ID, Name: "Done"}
	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 11, Version: 3,
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, "GAM-11").
		Return(issue, nil).
		Times(2)

	h.states.EXPECT().
		List(gomock.Any(), scene.workspace.ID, scene.team.ID).
		Return([]entity.WorkflowState{done}, nil).
		Times(2)

	acknowledged := []bool{}

	h.issues.EXPECT().
		Update(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.UpdateIssueInput) (entity.Issue, error) {
			acknowledged = append(acknowledged, in.AcknowledgeOpenChildren)

			if !in.AcknowledgeOpenChildren {
				return entity.Issue{}, entity.ErrIssueChildrenOpen
			}

			out := issue
			out.Version = 4

			return out, nil
		}).
		Times(2)

	refused := callTool(t, h, "norn_change_issue_state", map[string]any{
		"workspace": "acme", "issue": "GAM-11", "state": "Done",
	})

	expectRefusal(t, refused, "issue_children_open", "acknowledge_open_children")

	closed := callTool(t, h, "norn_change_issue_state", map[string]any{
		"workspace": "acme", "issue": "GAM-11", "state": "Done", "acknowledge_open_children": true,
	})
	if closed.IsError {
		t.Fatalf("closing with the acknowledgement errored: %s", refusalText(t, closed))
	}

	if !slices.Equal(acknowledged, []bool{false, true}) {
		t.Fatalf("the closes carried acknowledgements %v, want false then true", acknowledged)
	}
}

func TestALabelFromAnotherTeamIsRefusedWithAStableCode(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	h.labels.EXPECT().
		List(gomock.Any(), scene.workspace.ID).
		Return([]entity.Label{{ID: uuid.New(), Name: "payments"}}, nil)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, entity.ErrLabelOutOfScope)

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "Wrong label", "labels": []string{"payments"},
	})

	expectRefusal(t, result, "label_out_of_scope", "another team")
}

func TestAMissingPermissionIsNamedInTheRefusal(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, entity.AccessDeniedError{
		Reason:   entity.DenyReasonTokenPermissionMissing,
		Resource: entity.ResourceIssue,
		Action:   entity.ActionManage,
	})

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "Not allowed",
	})

	expectRefusal(t, result, "permission_denied", "issue:manage")
}

func TestARefusedAssigneeIsExplainedInWordsAndKeepsItsFieldCode(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	var raised service.CreateIssueInput

	raisingInto(h, scene, &raised, entity.NewValidationError(entity.FieldError{
		Field: "assigneeId",
		Code:  entity.ValidationCodeUnsupportedValue,
	}))

	result := callTool(t, h, "norn_create_issue", map[string]any{
		"workspace": "acme", "team": "GAM", "title": "For a bot", "assignee": uuid.NewString(),
	})

	expectRefusal(t, result, "invalid_input", "person", "assigneeId: unsupported_value")
}

func TestAnIssueIsDeletedArchivedOrRestoredByStatus(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 12, Version: 8,
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, "GAM-12").
		Return(issue, nil)

	var set service.SetIssueStatusInput

	h.issues.EXPECT().
		SetStatus(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.SetIssueStatusInput) (entity.Issue, error) {
			set = in

			return issue, nil
		})

	result := callTool(t, h, "norn_set_issue_status", map[string]any{
		"workspace": "acme", "issue": "GAM-12", "status": "deleted",
	})
	if result.IsError {
		t.Fatalf("tool errored: %s", refusalText(t, result))
	}

	if set.Status != entity.IssueStatusPendingDeletion || set.ExpectedVersion != 8 {
		t.Fatalf("the status was set with %+v, want pending deletion at version 8", set)
	}

	expectRefusal(t, callTool(t, h, "norn_set_issue_status", map[string]any{
		"workspace": "acme", "issue": "GAM-12", "status": "gone",
	}), "status_invalid")
}

func TestAnIssueIsFiledUnderAParentAndTakenOutAgain(t *testing.T) {
	h := newHarness(t)
	scene := sceneFor(t, h)

	child := entity.Issue{ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID, Number: 2, Version: 4}
	parent := entity.Issue{ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID, Number: 1, Version: 1}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, reference string) (entity.Issue, error) {
			if reference == "GAM-1" {
				return parent, nil
			}

			return child, nil
		}).
		AnyTimes()

	filings := []*uuid.UUID{}

	h.issues.EXPECT().
		SetParent(gomock.Any(), scene.workspace.ID, child.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.SetIssueParentInput) (entity.Issue, error) {
			filings = append(filings, in.ParentID)

			return child, nil
		}).
		Times(2)

	filed := callTool(t, h, "norn_set_issue_parent", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "parent": "GAM-1",
	})
	if filed.IsError {
		t.Fatalf("filing errored: %s", refusalText(t, filed))
	}

	freed := callTool(t, h, "norn_set_issue_parent", map[string]any{
		"workspace": "acme", "issue": "GAM-2", "parent": "",
	})
	if freed.IsError {
		t.Fatalf("taking it out errored: %s", refusalText(t, freed))
	}

	if len(filings) != 2 || filings[0] == nil || *filings[0] != parent.ID || filings[1] != nil {
		t.Fatalf("the parent was set to %v, want GAM-1 and then no parent", filings)
	}
}
