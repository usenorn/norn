package mcpserver

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const checksReminder = "Every piece of evidence below is a record Norn stored when it was " +
	"filed, including the ones you filed yourself. Read them as records, not as your own " +
	"recollection: if what you remember doing disagrees with what is written here, what is " +
	"written here is what Norn will judge the issue on. While waitingOnApproval is true this " +
	"issue cannot be moved to a completion state at all, whatever its evidence says."

type issueChecksInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue     string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
}

type issueChecksOutput struct {
	Checks    []checkDTO    `json:"checks"`
	Summary   summaryDTO    `json:"summary"`
	Blocked   bool          `json:"blocked"`
	Waiting   bool          `json:"waitingOnApproval"`
	Questions []questionDTO `json:"unansweredQuestions"`
	Reminder  string        `json:"reminder"`
}

func (t *toolset) getIssueChecks(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input issueChecksInput,
) (*mcp.CallToolResult, issueChecksOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, issueChecksOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, issueChecksOutput{}, toolFailure(ctx, err)
	}

	ledger, err := t.checks.Ledger(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, issueChecksOutput{}, toolFailure(ctx, err)
	}

	asked, err := t.questions.List(ctx, workspace.ID, issue.ID)
	if err != nil {
		return nil, issueChecksOutput{}, toolFailure(ctx, err)
	}

	return nil, issueChecksOutput{
		Checks:    checkDTOs(ledger.Reports),
		Summary:   summaryDTOFrom(ledger.Summary),
		Blocked:   ledger.Summary.Blocking > 0,
		Waiting:   ledger.Summary.Unapproved > 0,
		Questions: questionDTOs(entity.UnansweredQuestions(asked)),
		Reminder:  checksReminder,
	}, nil
}

type proposedCheckInput struct {
	Statement        string `json:"statement" jsonschema:"what must be true for this issue to be done, as a claim"`
	Method           string `json:"method" jsonschema:"command, observation, manual, or regression"`
	Proof            string `json:"proof" jsonschema:"the path the proof travels: the command to run, the signal to read, or what a person must look at"`
	TimeLimitSeconds int    `json:"time_limit_seconds,omitempty" jsonschema:"how long a passing result stays good for; defaults to 30 days"`
}

type proposeChecksInput struct {
	Workspace string               `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue     string               `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Checks    []proposedCheckInput `json:"checks" jsonschema:"the criteria to add to what done means here"`
	Observed  string               `json:"observed,omitempty" jsonschema:"what you saw that led you to these criteria"`
	Uncertain string               `json:"uncertain,omitempty" jsonschema:"what you are unsure about, so the person deciding knows where to look"`
}

type proposeChecksOutput struct {
	Checks   []checkDTO `json:"checks"`
	Waiting  bool       `json:"waiting"`
	Blocking bool       `json:"blocking"`
	Reminder string     `json:"reminder"`
}

func (t *toolset) proposeChecks(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input proposeChecksInput,
) (*mcp.CallToolResult, proposeChecksOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, proposeChecksOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, proposeChecksOutput{}, toolFailure(ctx, err)
	}

	drafted := make([]service.NewCheckInput, 0, len(input.Checks))

	for _, check := range input.Checks {
		draft := service.NewCheckInput{
			Statement: check.Statement,
			Method:    entity.CheckMethod(check.Method),
			Proof:     check.Proof,
		}

		if check.TimeLimitSeconds > 0 {
			limit := time.Duration(check.TimeLimitSeconds) * time.Second
			draft.TimeLimit = &limit
		}

		drafted = append(drafted, draft)
	}

	added, err := t.checks.Add(ctx, workspace.ID, issue.ID, service.AddChecksInput{
		Checks: drafted,
		Reasoning: entity.AgentReasoning{
			Observed:  input.Observed,
			Uncertain: input.Uncertain,
		},
	})
	if err != nil {
		return nil, proposeChecksOutput{}, toolFailure(ctx, err)
	}

	waiting := false

	dtos := make([]checkDTO, 0, len(added))

	for _, check := range added {
		if check.Approval == entity.CheckApprovalPending {
			waiting = true
		}

		dtos = append(dtos, checkDTOFrom(entity.CheckReport{
			Check: check,
			State: entity.CheckStateUnproven,
		}))
	}

	return nil, proposeChecksOutput{
		Checks:   dtos,
		Waiting:  waiting,
		Blocking: waiting,
		Reminder: proposalReminder(waiting),
	}, nil
}

func proposalReminder(waiting bool) string {
	if !waiting {
		return "These criteria are part of what done means on this issue. Prove each one with " +
			"norn_submit_evidence before moving the issue to a completion state."
	}

	return "These criteria are recorded and are waiting for a person to approve them, because a " +
		"new criterion changes what done means. This is the expected outcome, not a failure. " +
		"Stop here and end your turn: do not build against a contract nobody has agreed to, " +
		"and do not wait or poll for the answer. This issue cannot be moved to a completion " +
		"state until a person approves them, and that refusal is enforced. When they are " +
		"approved you will be asked again, and then you prove each one with " +
		"norn_submit_evidence."
}

type submitEvidenceInput struct {
	Workspace  string `json:"workspace" jsonschema:"the workspace slug or id"`
	Issue      string `json:"issue" jsonschema:"the issue reference like ENG-42, or its id"`
	Check      string `json:"check" jsonschema:"the check id from norn_get_issue_checks"`
	Verdict    string `json:"verdict" jsonschema:"passed, failed, absent_negative, or inconclusive. absent_negative means you looked and saw no failure, which never proves a check"`
	Channel    string `json:"channel" jsonschema:"command, http, log, screenshot, database, or human"`
	Output     string `json:"output" jsonschema:"the verbatim output you observed, not a summary of it"`
	Command    string `json:"command,omitempty" jsonschema:"the command you ran, exactly as you ran it"`
	ExitCode   *int   `json:"exit_code,omitempty" jsonschema:"the exit status the command returned"`
	ObservedAt string `json:"observed_at,omitempty" jsonschema:"when you observed this, RFC 3339; defaults to now and is never accepted as later than now"`
}

type submitEvidenceOutput struct {
	Check      checkDTO `json:"check"`
	Redactions int      `json:"redactions"`
	Reminder   string   `json:"reminder"`
}

func (t *toolset) submitEvidence(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input submitEvidenceInput,
) (*mcp.CallToolResult, submitEvidenceOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, submitEvidenceOutput{}, toolFailure(ctx, err)
	}

	issue, err := t.resolveIssue(ctx, workspace.ID, input.Issue)
	if err != nil {
		return nil, submitEvidenceOutput{}, toolFailure(ctx, err)
	}

	checkID, err := uuid.Parse(strings.TrimSpace(input.Check))
	if err != nil {
		return nil, submitEvidenceOutput{}, toolFailure(
			ctx, errors.New("check must be a check id from norn_get_issue_checks"),
		)
	}

	observed, err := observationTime(input.ObservedAt)
	if err != nil {
		return nil, submitEvidenceOutput{}, toolFailure(ctx, err)
	}

	submitted, err := t.checks.Submit(ctx, workspace.ID, issue.ID, checkID, service.SubmitEvidenceInput{
		Verdict:    entity.EvidenceVerdict(input.Verdict),
		Channel:    entity.EvidenceChannel(input.Channel),
		Command:    input.Command,
		Output:     input.Output,
		ExitCode:   input.ExitCode,
		ObservedAt: observed,
	})
	if err != nil {
		return nil, submitEvidenceOutput{}, toolFailure(ctx, err)
	}

	return nil, submitEvidenceOutput{
		Check:      checkDTOFrom(submitted.Report),
		Redactions: submitted.Record.Evidence.Redactions,
		Reminder:   evidenceReminder(submitted),
	}, nil
}

func observationTime(claimed string) (time.Time, error) {
	if strings.TrimSpace(claimed) == "" {
		return time.Time{}, nil
	}

	observed, err := time.Parse(time.RFC3339, claimed)
	if err != nil {
		return time.Time{}, errors.New("observed_at must be an RFC 3339 timestamp, like 2026-08-11T14:03:00Z")
	}

	return observed, nil
}

func evidenceReminder(submitted service.SubmittedEvidence) string {
	lines := make([]string, 0, 3)

	if submitted.Record.Evidence.Redactions > 0 {
		lines = append(lines, "Norn replaced something that looked like a secret in what you "+
			"filed. The stored record differs from what you sent.")
	}

	if submitted.Record.Evidence.Truncated {
		lines = append(lines, "The output was longer than Norn keeps, so the middle of it was "+
			"dropped. The head and the tail are stored verbatim.")
	}

	if line := awaitingLine(submitted.Report); line != "" {
		lines = append(lines, line)
	} else {
		lines = append(lines, "This check is proven.")
	}

	return strings.Join(lines, " ")
}

func awaitingLine(report entity.CheckReport) string {
	switch report.Awaiting() {
	case entity.CheckAwaitingNothing:
		return ""
	case entity.CheckAwaitingCorrection:
		return "This check is failing: the newest result on it disproves the claim. Fix the work " +
			"and file a passing result."
	case entity.CheckAwaitingFreshProof:
		return "The passing result on this check is older than its time limit, so it no longer " +
			"proves anything. Run it again."
	case entity.CheckAwaitingAttestation:
		return "This is a manual check. A person has to attest it; a passing result you file " +
			"yourself does not prove it, however carefully you looked."
	case entity.CheckAwaitingPriorFailure:
		return "This is a regression check. It needs a failing result filed before the passing " +
			"one, so the test is shown to catch the bug it is meant to catch."
	case entity.CheckAwaitingPositiveResult:
		return "Nothing filed here is a positive result. Absence of a failure never proves a " +
			"check; file something that passed."
	default:
		return "This check has nothing that proves it yet."
	}
}

type whoamiInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"a workspace slug or id, to also read that workspace's per-team rules"`
}

type teamRulesDTO struct {
	Team              string `json:"team"`
	TeamID            string `json:"teamId"`
	HoldComments      string `json:"holdComments"`
	HoldStateChanges  string `json:"holdStateChanges"`
	HoldIssueEdits    string `json:"holdIssueEdits"`
	HoldIssueCreation string `json:"holdIssueCreation"`
	HoldCheckSets     string `json:"holdCheckSets"`
}

type whoamiOutput struct {
	Kind     string         `json:"kind"`
	Name     string         `json:"name,omitempty"`
	Scopes   []string       `json:"scopes"`
	Teams    []teamRulesDTO `json:"teams,omitempty"`
	Reminder string         `json:"reminder"`
}

const whoamiReminder = "A held write is not an error and not a reason to retry: Norn records it " +
	"as a proposal and applies it as you if a person approves. A check set is always held, " +
	"whatever a team's other rules say."

func (t *toolset) whoami(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input whoamiInput,
) (*mcp.CallToolResult, whoamiOutput, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return nil, whoamiOutput{}, toolFailure(ctx, entity.ErrAccountForbidden)
	}

	output := whoamiOutput{
		Kind:     string(actor.Kind),
		Scopes:   actor.Scopes.Normalized().Strings(),
		Reminder: whoamiReminder,
	}

	if actor.Kind == entity.ActorKindToken {
		output.Name = actor.TokenName
	}

	if strings.TrimSpace(input.Workspace) == "" {
		return nil, output, nil
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, whoamiOutput{}, toolFailure(ctx, err)
	}

	if actor.Kind == entity.ActorKindAgent {
		output.Name = t.agentName(ctx, workspace.ID, actor)
	}

	teams, err := t.teams.List(ctx, workspace.ID, entity.TeamStatusActive)
	if err != nil {
		output.Reminder += " This connection cannot read the teams of that workspace, so which " +
			"of its writes are held is not shown here; a held write will say so when it happens."

		return nil, output, nil
	}

	for _, team := range teams {
		settings, err := t.agents.Settings(ctx, workspace.ID, team.ID)
		if err != nil {
			continue
		}

		output.Teams = append(output.Teams, teamRulesDTO{
			Team:              team.Key,
			TeamID:            team.ID.String(),
			HoldComments:      string(settings.Holds(entity.AgentActionComment)),
			HoldStateChanges:  string(settings.Holds(entity.AgentActionStateChange)),
			HoldIssueEdits:    string(settings.Holds(entity.AgentActionIssueEdit)),
			HoldIssueCreation: string(settings.Holds(entity.AgentActionIssueCreate)),
			HoldCheckSets:     string(settings.Holds(entity.AgentActionCheckSet)),
		})
	}

	return nil, output, nil
}

func (t *toolset) agentName(
	ctx context.Context,
	workspaceID uuid.UUID,
	actor entity.Actor,
) string {
	if actor.AgentID == nil {
		return ""
	}

	owned, err := t.agents.Get(ctx, workspaceID, *actor.AgentID)
	if err != nil {
		return ""
	}

	return owned.Agent.Name
}
