package mcpserver_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) workspace(t *testing.T, times int) entity.Workspace {
	t.Helper()

	workspace := entity.Workspace{ID: uuid.New(), Slug: "acme", Name: "Acme"}

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), h.actor.AccountID).
		Return([]entity.Workspace{workspace}, nil).
		Times(times)

	return workspace
}

func (h *harness) issue(t *testing.T, workspaceID uuid.UUID) entity.Issue {
	t.Helper()

	issue := entity.Issue{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		TeamID:      uuid.New(),
		Title:       "Duplicate charge on retry",
		Version:     3,
	}

	h.issues.EXPECT().
		GetByReference(gomock.Any(), workspaceID, "ENG-1").
		Return(issue, nil).
		AnyTimes()

	return issue
}

func call(t *testing.T, h *harness, name string, args map[string]any) (string, bool) {
	t.Helper()

	result, err := h.session(t).CallTool(t.Context(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}

	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	if result.IsError {
		content, err := json.Marshal(result.Content)
		if err != nil {
			t.Fatalf("marshal content: %v", err)
		}

		return string(content), true
	}

	return string(payload), false
}

func TestAHeldCheckSetIsReportedToTheAgentAsSuccess(t *testing.T) {
	h := newHarness(t)
	workspace := h.workspace(t, 1)
	issue := h.issue(t, workspace.ID)

	h.checks.EXPECT().
		Add(gomock.Any(), workspace.ID, issue.ID, gomock.Any()).
		Return([]entity.Check{{
			ID:        uuid.New(),
			Statement: "the retry path charges once",
			Method:    entity.CheckMethodCommand,
			Proof:     "go test ./internal/service/billing/...",
			Approval:  entity.CheckApprovalPending,
		}}, nil)

	payload, failed := call(t, h, "norn_propose_checks", map[string]any{
		"workspace": "acme",
		"issue":     "ENG-1",
		"checks": []map[string]any{{
			"statement": "the retry path charges once",
			"method":    "command",
			"proof":     "go test ./internal/service/billing/...",
		}},
	})

	if failed {
		t.Fatalf(
			"proposing a check set answered as an error:\n%s\nwaiting for a person is the "+
				"intended outcome, and calling it an error teaches the agent to stop proposing",
			payload,
		)
	}

	if !strings.Contains(payload, `"waiting":true`) {
		t.Errorf("the agent is not told its criteria are waiting: %s", payload)
	}

	if !strings.Contains(payload, "not a failure") {
		t.Errorf("the reminder does not say plainly that waiting is expected: %s", payload)
	}
}

func TestARefusedCompletionNamesTheCriteriaAndBothWaysOut(t *testing.T) {
	h := newHarness(t)
	workspace := h.workspace(t, 1)
	issue := h.issue(t, workspace.ID)

	state := entity.WorkflowState{ID: uuid.New(), Name: "Done", Category: entity.StateCategoryComplete}

	h.issues.EXPECT().
		Update(gomock.Any(), workspace.ID, issue.ID, gomock.Any()).
		Return(entity.Issue{}, entity.IssueChecksUnprovenError{Checks: []entity.Check{
			{Statement: "the retry path charges once"},
		}})

	h.states.EXPECT().
		List(gomock.Any(), workspace.ID, issue.TeamID).
		Return([]entity.WorkflowState{state}, nil)

	message, failed := call(t, h, "norn_change_issue_state", map[string]any{
		"workspace": "acme",
		"issue":     "ENG-1",
		"state":     "Done",
	})

	if !failed {
		t.Fatal("an agent closed an issue whose criteria are unproven")
	}

	for _, expected := range []string{
		"the retry path charges once",
		"norn_submit_evidence",
		"gap",
		"Do not retry",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("the refusal never mentions %q, so the agent cannot act on it:\n%s", expected, message)
		}
	}
}

func TestAnAgentReadsItsOwnEvidenceBackAsARecordSomebodyFiled(t *testing.T) {
	h := newHarness(t)
	h.actor.Kind = entity.ActorKindAgent
	workspace := h.workspace(t, 1)
	issue := h.issue(t, workspace.ID)

	observed := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	received := observed.Add(time.Minute)

	check := entity.Check{
		ID:        uuid.New(),
		Statement: "no duplicate charges since the fix",
		Method:    entity.CheckMethodObservation,
		Proof:     "the billing error log",
		Approval:  entity.CheckApprovalApproved,
	}

	report := entity.CheckReport{
		Check: check,
		State: entity.CheckStateUnproven,
		Evidence: []entity.EvidenceRecord{{
			Evidence: entity.Evidence{
				ID:         uuid.New(),
				CheckID:    check.ID,
				Verdict:    entity.EvidenceAbsentNegative,
				Channel:    entity.EvidenceChannelLog,
				Output:     "no matching lines since 02:00",
				ObservedAt: observed,
				ReceivedAt: received,
				Actor:      entity.ActivityAttribution{Kind: entity.ActorKindAgent},
				ActorName:  "opsy",
			},
		}},
	}

	h.checks.EXPECT().
		Ledger(gomock.Any(), workspace.ID, issue.ID).
		Return(service.IssueChecks{
			Reports: []entity.CheckReport{report},
			Summary: entity.Summarise([]entity.CheckReport{report}),
		}, nil)

	payload, failed := call(t, h, "norn_get_issue_checks", map[string]any{
		"workspace": "acme",
		"issue":     "ENG-1",
	})

	if failed {
		t.Fatalf("reading the criteria failed: %s", payload)
	}

	if !strings.Contains(payload, `"submittedBy":"opsy (agent)"`) {
		t.Errorf(
			"evidence comes back without naming who filed it:\n%s\nan agent reading its own "+
				"claim unlabelled reads it as memory rather than as a record",
			payload,
		)
	}

	if !strings.Contains(payload, `"blocked":true`) {
		t.Errorf("an approved unproven criterion does not report the issue as blocked: %s", payload)
	}

	if !strings.Contains(payload, string(entity.CheckAwaitingPositiveResult)) {
		t.Errorf(
			"absence-of-a-failure evidence does not say what the check still needs:\n%s",
			payload,
		)
	}

	if !strings.Contains(payload, `"reminder"`) {
		t.Errorf("the response carries no reminder from Norn: %s", payload)
	}
}

func TestWhoamiTellsAnAgentWhichOfItsWritesAreHeld(t *testing.T) {
	h := newHarness(t)

	agentID := uuid.New()
	h.actor.Kind = entity.ActorKindAgent
	h.actor.AgentID = &agentID

	workspace := h.workspace(t, 1)
	team := entity.Team{ID: uuid.New(), Key: "ENG", Name: "Engineering"}

	h.agents.EXPECT().
		Get(gomock.Any(), workspace.ID, agentID).
		Return(service.OwnedAgent{Agent: entity.Agent{ID: agentID, Name: "opsy"}}, nil)

	h.teams.EXPECT().
		List(gomock.Any(), workspace.ID, entity.TeamStatusActive).
		Return([]entity.Team{team}, nil)

	h.agents.EXPECT().
		Settings(gomock.Any(), workspace.ID, team.ID).
		Return(entity.AgentSettings{HoldStateChanges: entity.AgentHoldUnlessProven}, nil)

	payload, failed := call(t, h, "norn_whoami", map[string]any{"workspace": "acme"})

	if failed {
		t.Fatalf("whoami failed: %s", payload)
	}

	for _, expected := range []string{
		`"name":"opsy"`,
		`"holdStateChanges":"unless_proven"`,
		`"holdComments":"never"`,
		`"holdCheckSets":"always"`,
	} {
		if !strings.Contains(payload, expected) {
			t.Errorf("whoami does not report %s:\n%s", expected, payload)
		}
	}
}

func TestNornsOwnReminderIsNotCoveredByTheUntrustedContentRule(t *testing.T) {
	h := newHarness(t)

	session := h.session(t)

	instructions := session.InitializeResult().Instructions
	if !strings.Contains(instructions, "reminder") {
		t.Fatalf(
			"the instructions do not name the reminder field:\n%s\nevery checks tool answers "+
				"with one, and an agent told to treat all tool output as untrusted user data "+
				"will discount the very field that states how Norn judges the work",
			instructions,
		)
	}
}
