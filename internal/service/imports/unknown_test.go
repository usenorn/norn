package imports_test

import (
	"context"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	lostLabel  = "label-never-exported"
	lostCsvRow = "row 4 of the export names 9 columns where the header names 12"
)

func withoutTriage(h *harness) *harness {
	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	h.triage.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.TriageSettings{}, entity.ErrTriageDisabled).
		AnyTimes()

	return h
}

func aLabelThatNeverArrived(
	t *testing.T,
	policy entity.ImportUnknownPolicy,
) (*harness, entity.Team) {
	t.Helper()

	h := newHarness(t).backed().resolving(policy)
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue,
		fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
			Title: "Rework the hub", Team: sourceTeam,
		}),
		fetched(t, sourceIssueLoose, "", service.ImportIssuePayload{
			Title: "Tidy the loose ends", Team: sourceTeam, Labels: []string{lostLabel},
		}),
	)
	h.decided(
		entity.ImportMapping{
			Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
			Decision: entity.ImportDecisionMap, TargetID: team.ID,
		},
		entity.ImportMapping{
			Kind: entity.ImportMapLabel, SourceKey: lostLabel,
			Decision: entity.ImportDecisionCreate,
		},
	)

	return h, team
}

func TestUnderSkipARowNamingALabelThatNeverArrivedIsLeftBehindWhole(t *testing.T) {
	h, team := aLabelThatNeverArrived(t, entity.ImportUnknownSkip)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if state := h.recordState(entity.ImportIssue, sourceIssueLoose); state != entity.ImportRecordSkipped {
		t.Fatalf(
			"the row reads as %q under the skip policy. Skip is what a workspace chooses when a "+
				"half-imported row is worse than none: the row waits for a second run with the "+
				"label in it rather than arriving quietly missing something.",
			state,
		)
	}

	line, found := h.lineOf(entity.ImportIssue, sourceIssueLoose)

	if !found || !strings.Contains(line.Detail["reason"], lostLabel) {
		t.Fatalf(
			"the report explains the skip as %q. Nothing else in the run says which reference was "+
				"missing, so the requester cannot tell what to add to the export before running it "+
				"again.",
			line.Detail["reason"],
		)
	}

	if len(made.issues) != 1 {
		t.Errorf(
			"the chunk created %d issues, want only the row that named nothing missing. One "+
				"unresolvable reference costs its own row and no other.",
			len(made.issues),
		)
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf("run ended %q, want imported", h.run().Status)
	}
}

func TestUnderCreateARowKeepsItsPlaceAndLosesOnlyTheLabelThatNeverArrived(t *testing.T) {
	h, team := aLabelThatNeverArrived(t, entity.ImportUnknownCreate)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if state := h.recordState(entity.ImportIssue, sourceIssueLoose); state != entity.ImportRecordApplied {
		t.Fatalf(
			"the row reads as %q under the create policy. The label phase has already run, so a "+
				"label still unresolved is never going to arrive; the work is worth more than the "+
				"tag on it.",
			state,
		)
	}

	arrived := made.issueCreated(t, "Tidy the loose ends")

	if len(arrived.LabelIDs) != 0 {
		t.Errorf(
			"the issue arrived carrying %d labels. The one it named does not exist here, so a "+
				"label id on this row would point at something no phase of the run created.",
			len(arrived.LabelIDs),
		)
	}

	line, found := h.lineOf(entity.ImportIssue, sourceIssueLoose)

	if !found || line.Outcome != entity.ImportOutcomeDropped {
		t.Fatalf(
			"the report says %q about the row that lost its label, want it named as dropped. A "+
				"label silently missing from an imported issue is the sort of thing found weeks "+
				"later by somebody filtering on it.",
			line.Outcome,
		)
	}

	if !strings.Contains(line.Detail["reason"], lostLabel) {
		t.Errorf("the dropped line reads %q and never names the label", line.Detail["reason"])
	}
}

func TestUnderFailARowNamingSomethingUnknownIsCountedAsAFailureRatherThanAnOmission(t *testing.T) {
	h, team := aLabelThatNeverArrived(t, entity.ImportUnknownFail)

	applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf("run execute: %v", err)
	}

	if state := h.recordState(entity.ImportIssue, sourceIssueLoose); state != entity.ImportRecordFailed {
		t.Fatalf(
			"the row reads as %q under the fail policy. A workspace that chose fail asked to be "+
				"told loudly: a skip reads as a decision somebody made, and this one is the import "+
				"admitting it could not do what it was asked.",
			state,
		)
	}

	line, found := h.lineOf(entity.ImportIssue, sourceIssueLoose)

	if !found || line.Outcome != entity.ImportOutcomeFailed {
		t.Fatalf("the report says %q about the row, want failed", line.Outcome)
	}

	if !strings.Contains(line.Detail["reason"], lostLabel) {
		t.Errorf("the failure reads %q and never names the label", line.Detail["reason"])
	}

	if state := h.recordState(entity.ImportIssue, sourceIssueHub); state != entity.ImportRecordApplied {
		t.Errorf("the intact row reads as %q, want it applied beside the failure", state)
	}
}

func TestAnIssueWhoseTeamCannotBeReachedIsSkippedUnderEveryPolicy(t *testing.T) {
	for _, policy := range entity.ImportUnknownPolicies() {
		t.Run(string(policy), func(t *testing.T) {
			h := newHarness(t).backed().resolving(policy)
			team := teamNamed("Platform")

			h.scopedTo(team.ID)
			h.stage(entity.ImportIssue, fetched(t, hostileTeamless, "", service.ImportIssuePayload{
				Title: "Filed on a team this import is leaving behind", Team: sourceHidden,
			}))
			h.decided(entity.ImportMapping{
				Kind: entity.ImportMapTeam, SourceKey: sourceHidden,
				Decision: entity.ImportDecisionSkip,
			})

			made := applying(h, team)

			h.at(entity.ImportMapped)

			if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
				t.Fatalf("run execute: %v", err)
			}

			if state := h.recordState(entity.ImportIssue, hostileTeamless); state != entity.ImportRecordSkipped {
				t.Fatalf(
					"the row reads as %q under the %q policy. Every other reference an issue holds "+
						"is something it can arrive without, but an issue is filed on a team: there "+
						"is nowhere to put this one, so create has nothing to create it in and fail "+
						"would call a deliberate exclusion a fault of the run.",
					state, policy,
				)
			}

			if len(made.issues) != 0 {
				t.Errorf("the run created %d issues on a team it was told to leave behind", len(made.issues))
			}

			line, _ := h.lineOf(entity.ImportIssue, hostileTeamless)

			if line.Outcome != entity.ImportOutcomeSkipped {
				t.Errorf("the report says %q about the row, want skipped", line.Outcome)
			}
		})
	}
}

func TestARowTheSourceCouldNotReadIsStagedAndSkippedWithItsOwnExplanation(t *testing.T) {
	h := newHarness(t).backed()
	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stage(entity.ImportIssue,
		fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
			Title: "Rework the hub", Team: sourceTeam,
		}),
		fetched(t, "issue-unreadable", "", service.ImportIssuePayload{
			Team: sourceTeam, Defect: lostCsvRow,
		}),
		fetched(t, sourceIssueLoose, "", service.ImportIssuePayload{
			Title: "Tidy the loose ends", Team: sourceTeam,
		}),
	)
	onlyTheTeamIsDecided(h, team.ID)

	made := applying(h, team)

	h.at(entity.ImportMapped)

	if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
		t.Fatalf(
			"a row the source could not read ended the run: %v. Fetch returning an error retries "+
				"the whole resource and then fails the import, so a malformed row has to be staged "+
				"carrying its own explanation or one bad line in a spreadsheet costs the backlog.",
			err,
		)
	}

	if state := h.recordState(entity.ImportIssue, "issue-unreadable"); state != entity.ImportRecordSkipped {
		t.Fatalf(
			"the unreadable row reads as %q. Nothing downstream of the adapter knows the line was "+
				"malformed, so the row is attempted like any other and the run reports whatever it "+
				"tripped over next — an empty title, a missing reference, or nothing at all — "+
				"instead of the parse that caused it.",
			state,
		)
	}

	line, found := h.lineOf(entity.ImportIssue, "issue-unreadable")

	if !found || line.Detail["reason"] != lostCsvRow {
		t.Fatalf(
			"the report explains the row as %q, want the source's own %q. The adapter is the only "+
				"thing that saw the original line, and a reason invented downstream sends the "+
				"requester looking at the wrong column.",
			line.Detail["reason"], lostCsvRow,
		)
	}

	if len(made.issues) != 2 {
		t.Errorf(
			"the chunk created %d issues, want the two that read cleanly. A row nobody could parse "+
				"is worth exactly itself.",
			len(made.issues),
		)
	}

	if h.run().Status != entity.ImportImported {
		t.Errorf(
			"run ended %q, want imported. The good rows are in and each bad one is named; there is "+
				"nothing here to retry.",
			h.run().Status,
		)
	}
}

func TestTwoPreviewsOfTheSameRunUnderDifferentPoliciesFingerprintDifferently(t *testing.T) {
	h, _ := aLabelThatNeverArrived(t, entity.ImportUnknownSkip)

	withoutTriage(h)

	under := map[entity.ImportUnknownPolicy]string{}

	for _, policy := range entity.ImportUnknownPolicies() {
		h.resolving(policy)

		under[policy] = previewed(t, h).Digest()
	}

	for _, pair := range [][2]entity.ImportUnknownPolicy{
		{entity.ImportUnknownSkip, entity.ImportUnknownCreate},
		{entity.ImportUnknownSkip, entity.ImportUnknownFail},
		{entity.ImportUnknownCreate, entity.ImportUnknownFail},
	} {
		if under[pair[0]] == under[pair[1]] {
			t.Fatalf(
				"an import previewed under %q fingerprints the same as under %q. Execute admits a "+
					"run whose digest still matches, so a fingerprint blind to the policy lets a "+
					"requester approve a page describing rows being skipped and then run something "+
					"that fails them instead.",
				pair[0], pair[1],
			)
		}
	}

	h.wroteNothing("previewing under each policy")
}

func previewVerdict(
	t *testing.T,
	view entity.ImportPreview,
	externalID string,
) entity.ImportOutcome {
	t.Helper()

	for _, group := range [][]entity.ImportPreviewLine{view.Created, view.Skipped} {
		for _, line := range group {
			if line.Resource != entity.ImportIssue || line.ExternalID != externalID {
				continue
			}

			switch line.Outcome {
			case entity.ImportOutcomeCreated, entity.ImportOutcomeSkipped, entity.ImportOutcomeFailed:
				return line.Outcome
			}
		}
	}

	t.Fatalf("the preview reaches no verdict at all on %q", externalID)

	return ""
}

func previewAccount(view entity.ImportPreview, externalID string) []string {
	said := make([]string, 0)

	groups := [][]entity.ImportPreviewLine{
		view.Created, view.Changed, view.Skipped, view.Unattributed, view.Warnings,
	}

	for _, group := range groups {
		for _, line := range group {
			if line.Resource != entity.ImportIssue || line.ExternalID != externalID {
				continue
			}

			said = append(said, string(line.Outcome)+" "+line.Detail)
		}
	}

	sort.Strings(said)

	return said
}

func reportAccount(h *harness, externalID string) []string {
	recorded := make([]string, 0)

	for _, line := range h.world.lines {
		if line.Resource != entity.ImportIssue || line.ExternalID != externalID {
			continue
		}

		recorded = append(recorded, string(line.Outcome)+" "+detailOfLine(line))
	}

	sort.Strings(recorded)

	return recorded
}

func appliedVerdict(t *testing.T, h *harness, externalID string) entity.ImportOutcome {
	t.Helper()

	switch state := h.recordState(entity.ImportIssue, externalID); state {
	case entity.ImportRecordApplied:
		return entity.ImportOutcomeCreated
	case entity.ImportRecordSkipped:
		return entity.ImportOutcomeSkipped
	case entity.ImportRecordFailed:
		return entity.ImportOutcomeFailed
	default:
		t.Fatalf("%q reads as %q after the run, so the apply pass reached no verdict on it", externalID, state)

		return ""
	}
}

func TestThePreviewAndTheApplyPassReachTheSameVerdictUnderEveryPolicy(t *testing.T) {
	rows := []string{
		sourceIssueHub,
		sourceIssueLoose,
		sourceIssueDrift,
		sourceIssueCadence,
		hostileTeamless,
		"issue-unreadable",
	}

	for _, policy := range entity.ImportUnknownPolicies() {
		t.Run(string(policy), func(t *testing.T) {
			h := newHarness(t).backed().resolving(policy)
			team := teamNamed("Platform")

			h.scopedTo(team.ID)
			withoutTriage(h)

			h.stage(entity.ImportIssue,
				fetched(t, sourceIssueHub, "", service.ImportIssuePayload{
					Title: "Rework the hub", Team: sourceTeam,
				}),
				fetched(t, sourceIssueLoose, "", service.ImportIssuePayload{
					Title: "Tidy the loose ends", Team: sourceTeam, Labels: []string{lostLabel},
				}),
				fetched(t, sourceIssueDrift, "", service.ImportIssuePayload{
					Title: "Chase the drift", Team: sourceTeam, Assignee: otto,
				}),
				fetched(t, sourceIssueCadence, "", service.ImportIssuePayload{
					Title: "Set the cadence", Team: sourceTeam, Cycle: sourceCycle,
				}),
				fetched(t, hostileTeamless, "", service.ImportIssuePayload{
					Title: "Filed on a team this import is leaving behind", Team: sourceHidden,
				}),
				fetched(t, "issue-unreadable", "", service.ImportIssuePayload{
					Team: sourceTeam, Defect: lostCsvRow,
				}),
			)
			h.decided(
				entity.ImportMapping{
					Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
					Decision: entity.ImportDecisionMap, TargetID: team.ID,
				},
				entity.ImportMapping{
					Kind: entity.ImportMapTeam, SourceKey: sourceHidden,
					Decision: entity.ImportDecisionSkip,
				},
				entity.ImportMapping{
					Kind: entity.ImportMapLabel, SourceKey: lostLabel,
					Decision: entity.ImportDecisionCreate,
				},
				entity.ImportMapping{
					Kind: entity.ImportMapUser, SourceKey: otto.Key,
					Decision: entity.ImportDecisionCreate,
				},
			)

			view := previewed(t, h)

			applying(h, team)

			h.at(entity.ImportMapped)

			if err := h.runner.RunExecute(context.Background(), executePayload(h)); err != nil {
				t.Fatalf("run execute: %v", err)
			}

			for _, external := range rows {
				promised := previewVerdict(t, view, external)
				delivered := appliedVerdict(t, h, external)

				if promised != delivered {
					t.Errorf(
						"the preview promised %q for %q and the run delivered %q under the %q "+
							"policy. The preview and the apply pass are two hand-written readings "+
							"of one decision tree, and the digest the requester approves is "+
							"computed from the first: where they disagree, the page somebody read "+
							"is not the import that ran.",
						promised, external, delivered, policy,
					)
				}

				said, recorded := previewAccount(view, external), reportAccount(h, external)

				if !slices.Equal(said, recorded) {
					t.Errorf(
						"the preview accounts for %q as %v and the run reports it as %v. The "+
							"requester reads the first and lives with the second, so every line "+
							"one of them writes about a row the other has to write too.",
						external, said, recorded,
					)
				}
			}
		})
	}
}

func detailOfLine(line entity.ImportReportLine) string {
	keys := make([]string, 0, len(line.Detail))
	for key := range line.Detail {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+line.Detail[key])
	}

	return strings.Join(parts, "; ")
}
