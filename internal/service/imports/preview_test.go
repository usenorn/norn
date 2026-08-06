package imports_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type answers struct {
	team     uuid.UUID
	accounts map[string]uuid.UUID
}

func answeredFully(h *harness, held answers) *harness {
	onto := entity.ImportMapping{
		Kind: entity.ImportMapTeam, SourceKey: sourceTeam, Decision: entity.ImportDecisionCreate,
	}

	if held.team != uuid.Nil {
		onto.Decision = entity.ImportDecisionMap
		onto.TargetID = held.team
	}

	decisions := []entity.ImportMapping{
		onto,
		{
			Kind: entity.ImportMapPriority, SourceKey: sourcePriority,
			Decision: entity.ImportDecisionMap, TargetValue: string(entity.IssuePriorityHigh),
		},
		{
			Kind: entity.ImportMapProject, SourceKey: sourceProject,
			Decision: entity.ImportDecisionCreate,
		},
	}

	for _, key := range []string{sourceStateTodo, sourceStateDoing, sourceStateDone} {
		decisions = append(decisions, entity.ImportMapping{
			Kind: entity.ImportMapState, SourceKey: key, Decision: entity.ImportDecisionCreate,
		})
	}

	for _, key := range []string{sourceLabelBug, sourceLabelWork, sourceLabelOps} {
		decisions = append(decisions, entity.ImportMapping{
			Kind: entity.ImportMapLabel, SourceKey: key, Decision: entity.ImportDecisionCreate,
		})
	}

	for _, person := range []service.ImportUser{rae, ida} {
		decisions = append(decisions, entity.ImportMapping{
			Kind: entity.ImportMapUser, SourceKey: person.Key,
			Decision: entity.ImportDecisionMap, TargetID: held.accounts[person.Key],
		})
	}

	for _, person := range []service.ImportUser{otto, sam} {
		decisions = append(decisions, entity.ImportMapping{
			Kind: entity.ImportMapUser, SourceKey: person.Key,
			Decision: entity.ImportDecisionUnattributed,
		})
	}

	return h.decided(decisions...)
}

func previewed(t *testing.T, h *harness) entity.ImportPreview {
	t.Helper()

	view, err := h.imports.Preview(context.Background(), h.run().WorkspaceID, h.run().ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	return view
}

func previewLine(
	t *testing.T,
	lines []entity.ImportPreviewLine,
	externalID string,
) entity.ImportPreviewLine {
	t.Helper()

	for _, line := range lines {
		if line.ExternalID == externalID {
			return line
		}
	}

	t.Fatalf("%q is not among the %d lines this group holds", externalID, len(lines))

	return entity.ImportPreviewLine{}
}

func previewLineOf(
	t *testing.T,
	lines []entity.ImportPreviewLine,
	resource entity.ImportResource,
	externalID string,
) entity.ImportPreviewLine {
	t.Helper()

	for _, line := range lines {
		if line.Resource == resource && line.ExternalID == externalID {
			return line
		}
	}

	t.Fatalf("no %s line for %q is among the %d this group holds", resource, externalID, len(lines))

	return entity.ImportPreviewLine{}
}

func TestTwoPreviewsOfTheSameImportAgreeAndLeaveNoTraceOfHavingRun(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	h.stageEverything(newStaticSource(t))

	first := previewed(t, h)
	second := previewed(t, h)

	if first.Empty() {
		t.Fatal("the preview describes nothing at all, so agreeing with itself proves nothing")
	}

	if first.Digest() != second.Digest() {
		t.Fatalf(
			"two previews of an unchanged import fingerprinted differently:\n  %s\n  %s\nExecute "+
				"refuses a digest that has moved, so a fingerprint that wanders on its own would "+
				"turn every import into a race between reading the preview and accepting it.",
			first.Digest(), second.Digest(),
		)
	}

	h.wroteNothing("previewing")
}

func TestAnIssueOnATeamTheRequesterCannotSeeIsSkippedForBeingOutOfSight(t *testing.T) {
	h := newHarness(t).backed()

	elsewhere := uuid.New()

	h.scopedTo(uuid.New())
	h.stageEverything(newStaticSource(t))
	h.decided(entity.ImportMapping{
		Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
		Decision: entity.ImportDecisionMap, TargetID: elsewhere,
	})

	line := previewLine(t, previewed(t, h).Skipped, sourceIssueHub)

	if !strings.Contains(line.Detail, "not visible to the importing account") {
		t.Fatalf(
			"the preview explains the skip as %q. The team is there and somebody else can see it; "+
				"telling the requester it does not exist sends them to create a second one.",
			line.Detail,
		)
	}
}

func TestAColourNornDoesNotHaveIsShownAsTheOneItWillBecome(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	h.stageEverything(newStaticSource(t))

	line := previewLine(t, previewed(t, h).Changed, sourceLabelBug)

	if line.Outcome != entity.ImportOutcomeAdjusted {
		t.Fatalf("the off-palette label previewed as %q, want adjusted", line.Outcome)
	}

	for _, named := range []string{sourceOffPalet, string(entity.LabelColorMagenta)} {
		if !strings.Contains(line.Detail, named) {
			t.Errorf(
				"the adjustment reads %q and never mentions %q. A label arriving in a colour "+
					"nobody chose is the sort of thing that is only noticed weeks later, so both "+
					"the colour asked for and the colour given have to be on the page.",
				line.Detail, named,
			)
		}
	}
}

func TestACycleIsNamedAsSomethingTheImportCannotCarry(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	h.stageEverything(newStaticSource(t))

	line := previewLine(t, previewed(t, h).Warnings, sourceIssueCadence)

	if line.Outcome != entity.ImportOutcomeDropped {
		t.Fatalf(
			"an issue that belonged to a cycle previewed as %q. Cycles run on a team's own cadence "+
				"and cannot be back-dated, so the issue arrives without one — silently dropping it "+
				"would leave somebody hunting for a sprint that never came over.",
			line.Outcome,
		)
	}
}

func TestAnImportThatWouldLandEverythingInTriageHasToBeAcknowledged(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stageEverything(newStaticSource(t))
	answeredFully(h, answers{team: team.ID, accounts: map[string]uuid.UUID{
		rae.Key: uuid.New(),
		ida.Key: uuid.New(),
	}})

	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), h.run().WorkspaceID, h.run().RequestedByAccount).
		Return(nil, nil).
		AnyTimes()
	h.triage.EXPECT().
		Settings(gomock.Any(), h.run().WorkspaceID, team.ID).
		Return(entity.TriageSettings{TeamID: team.ID, RouteNonMembers: true}, nil).
		AnyTimes()
	h.teams.EXPECT().GetByID(gomock.Any(), team.ID).Return(team, nil).AnyTimes()

	view := previewed(t, h)

	if len(view.TriageTeams) != 1 || view.TriageTeams[0] != team.Name {
		t.Fatalf(
			"the preview named %v as triage-bound, want just %q. The requester is not on this team "+
				"and the team routes strangers to triage, so every issue this import carries lands "+
				"in a queue somebody has to empty by hand.",
			view.TriageTeams, team.Name,
		)
	}

	h.at(entity.ImportMapped)

	_, err := h.imports.Execute(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ExecuteImportInput{PreviewDigest: view.Digest()})

	if !errors.Is(err, entity.ErrImportWouldTriage) {
		t.Fatalf("execute returned %v, want the triage warning", err)
	}

	if !strings.Contains(err.Error(), team.Name) {
		t.Errorf(
			"the refusal reads %q and never names the team. The fix is a triage setting on one "+
				"particular team, and the requester cannot go and change it unless they are told "+
				"which one.",
			err.Error(),
		)
	}

	h.jobs.EXPECT().EnqueueImportExecute(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.imports.Execute(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ExecuteImportInput{PreviewDigest: view.Digest(), AcknowledgeTriage: true},
	); err != nil {
		t.Fatalf("execute after acknowledging triage: %v", err)
	}

	if !h.run().AcknowledgeTriage {
		t.Error("the acknowledgement was not recorded on the run, so the worker cannot know it was given")
	}
}

func TestAnImportRunsOnlyFromThePreviewItsRequesterActuallySaw(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Platform")

	h.scopedTo(team.ID)
	h.stageEverything(newStaticSource(t))
	answeredFully(h, answers{team: team.ID, accounts: map[string]uuid.UUID{
		rae.Key: uuid.New(),
		ida.Key: uuid.New(),
	}})

	h.triage.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.TriageSettings{}, entity.ErrTriageDisabled).
		AnyTimes()
	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	h.at(entity.ImportMapped)

	_, err := h.imports.Execute(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ExecuteImportInput{PreviewDigest: previewed(t, h).Digest() + "x"})

	if !errors.Is(err, entity.ErrImportPreviewStale) {
		t.Fatalf(
			"execute returned %v for a digest that does not describe this import. The preview is "+
				"the whole of what the requester agreed to; running from a stale one applies "+
				"something nobody read.",
			err,
		)
	}

	_, err = h.imports.Execute(context.Background(), h.run().WorkspaceID, h.run().ID,
		service.ExecuteImportInput{})

	if !errors.Is(err, entity.ErrImportPreviewRequired) {
		t.Fatalf("execute with no digest at all returned %v, want a demand to preview first", err)
	}
}

func TestThePreviewNamesTheHierarchyItWouldBuild(t *testing.T) {
	h := newHarness(t).backed().scopedTo()

	h.offering(newStaticSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	line := previewLineOf(t, previewed(t, h).Created, entity.ImportIssueParent, sourceIssueChild)

	if line.Subject != "Rework the hub header → Rework the hub" {
		t.Fatalf(
			"the preview describes the link as %q. The requester is being asked to approve a "+
				"hierarchy before anything exists to point at, and the source's own identifiers "+
				"are the one vocabulary they have never seen.",
			line.Subject,
		)
	}
}

func TestEverythingHostileAboutASourceSurvivesAsAPreviewLine(t *testing.T) {
	h := newHarness(t).backed()

	team := teamNamed("Platform")

	h.scopedTo(team.ID)

	h.offering(newHostileSource(t))

	if err := h.runner.RunStage(context.Background(), stagePayload(h)); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	h.forget()
	h.decided(entity.ImportMapping{
		Kind: entity.ImportMapTeam, SourceKey: sourceTeam,
		Decision: entity.ImportDecisionMap, TargetID: team.ID,
	}, entity.ImportMapping{
		Kind: entity.ImportMapTeam, SourceKey: sourceHidden,
		Decision: entity.ImportDecisionMap, TargetID: uuid.New(),
	})

	h.triage.EXPECT().
		Settings(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.TriageSettings{}, entity.ErrTriageDisabled).
		AnyTimes()
	h.teams.EXPECT().
		ListByWorkspaceMember(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	view := previewed(t, h)

	for _, dangling := range []string{hostileAnchor, hostileOrphanKin, hostileOrphanSay} {
		line := previewLine(t, view.Skipped, dangling)

		if !strings.Contains(line.Detail, "was not imported") {
			t.Errorf(
				"%q was skipped for the reason %q, want it named as pointing at an issue that "+
					"never arrived. An export that references a row it did not include is the "+
					"normal case, not a corruption.",
				dangling, line.Detail,
			)
		}
	}

	previewLine(t, view.Skipped, hostileTeamless)
	previewLine(t, view.Changed, hostileLabel)

	if len(previewed(t, h).Created) != len(view.Created) {
		t.Error("a second preview of the same hostile source disagreed with the first")
	}

	h.wroteNothing("previewing a hostile source")
}
