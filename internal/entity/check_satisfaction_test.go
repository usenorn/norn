package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

var checkID = uuid.New()

func at(minutes int) time.Time {
	return time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC).
		Add(time.Duration(minutes) * time.Minute)
}

func now() entity.EvidenceHorizon {
	return entity.EvidenceHorizon{Now: at(60)}
}

func filed(verdict entity.EvidenceVerdict, minutes int, kind entity.ActorKind) entity.Evidence {
	return entity.Evidence{
		ID:         uuid.New(),
		CheckID:    checkID,
		Verdict:    verdict,
		ReceivedAt: at(minutes),
		ObservedAt: at(minutes),
		Actor:      entity.ActivityAttribution{Kind: kind},
	}
}

func byAgent(verdict entity.EvidenceVerdict, minutes int) entity.Evidence {
	return filed(verdict, minutes, entity.ActorKindAgent)
}

func byPerson(verdict entity.EvidenceVerdict, minutes int) entity.Evidence {
	return filed(verdict, minutes, entity.ActorKindUser)
}

func checkOf(method entity.CheckMethod) entity.Check {
	return entity.Check{
		ID:        checkID,
		Statement: "payments retry without duplicating a charge",
		Method:    method,
		Approval:  entity.CheckApprovalApproved,
	}
}

func stateOf(t *testing.T, check entity.Check, evidence ...entity.Evidence) entity.CheckState {
	t.Helper()

	return entity.NewCheckReport(check, evidence, now()).State
}

func TestAbsenceOfANegativeSignalNeverProvesACheck(t *testing.T) {
	for _, method := range entity.CheckMethods() {
		t.Run(string(method), func(t *testing.T) {
			state := stateOf(t, checkOf(method),
				byAgent(entity.EvidenceAbsentNegative, 1),
				byAgent(entity.EvidenceAbsentNegative, 2),
			)

			if state != entity.CheckStateUnproven {
				t.Fatalf("nothing-bad-appeared left the check %q, want it unproven", state)
			}
		})
	}
}

func TestAQuietObservationCannotFailACheckEither(t *testing.T) {
	state := stateOf(t, checkOf(entity.CheckMethodCommand),
		byAgent(entity.EvidencePassed, 1),
		byAgent(entity.EvidenceAbsentNegative, 2),
	)

	if state != entity.CheckStateProven {
		t.Fatalf("a later quiet observation left the check %q, want it still proven", state)
	}
}

func TestAnInconclusiveObservationDecidesNothing(t *testing.T) {
	state := stateOf(t, checkOf(entity.CheckMethodCommand),
		byAgent(entity.EvidenceInconclusive, 1),
	)

	if state != entity.CheckStateUnproven {
		t.Fatalf("an inconclusive observation left the check %q, want it unproven", state)
	}
}

func TestTheNewestObservationDecidesBetweenProvenAndFailed(t *testing.T) {
	cases := []struct {
		name     string
		evidence []entity.Evidence
		want     entity.CheckState
	}{
		{
			"passed then failed",
			[]entity.Evidence{byAgent(entity.EvidencePassed, 1), byAgent(entity.EvidenceFailed, 2)},
			entity.CheckStateFailed,
		},
		{
			"failed then passed",
			[]entity.Evidence{byAgent(entity.EvidenceFailed, 1), byAgent(entity.EvidencePassed, 2)},
			entity.CheckStateProven,
		},
		{
			"both at the same moment",
			[]entity.Evidence{byAgent(entity.EvidencePassed, 1), byAgent(entity.EvidenceFailed, 1)},
			entity.CheckStateFailed,
		},
		{
			"failed alone",
			[]entity.Evidence{byAgent(entity.EvidenceFailed, 1)},
			entity.CheckStateFailed,
		},
		{
			"nothing at all",
			nil,
			entity.CheckStateUnproven,
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			if state := stateOf(t, checkOf(entity.CheckMethodCommand), testcase.evidence...); state != testcase.want {
				t.Fatalf("state is %q, want %q", state, testcase.want)
			}
		})
	}
}

func TestARegressionNeedsAFailingObservationBeforeThePassingOne(t *testing.T) {
	cases := []struct {
		name     string
		evidence []entity.Evidence
		want     entity.CheckState
	}{
		{
			"it failed, then it passed",
			[]entity.Evidence{byAgent(entity.EvidenceFailed, 1), byAgent(entity.EvidencePassed, 2)},
			entity.CheckStateProven,
		},
		{
			"it only ever passed",
			[]entity.Evidence{byAgent(entity.EvidencePassed, 2)},
			entity.CheckStateUnproven,
		},
		{
			"it passed, then it failed",
			[]entity.Evidence{byAgent(entity.EvidencePassed, 1), byAgent(entity.EvidenceFailed, 2)},
			entity.CheckStateFailed,
		},
		{
			"the failure was only quiet",
			[]entity.Evidence{
				byAgent(entity.EvidenceAbsentNegative, 1),
				byAgent(entity.EvidencePassed, 2),
			},
			entity.CheckStateUnproven,
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			state := stateOf(t, checkOf(entity.CheckMethodRegression), testcase.evidence...)

			if state != testcase.want {
				t.Fatalf("state is %q, want %q", state, testcase.want)
			}
		})
	}
}

func TestTheSameEvidenceProvesACommandCheckAndNotARegressionOne(t *testing.T) {
	only := []entity.Evidence{byAgent(entity.EvidencePassed, 2)}

	if state := stateOf(t, checkOf(entity.CheckMethodCommand), only...); state != entity.CheckStateProven {
		t.Fatalf("a command check with a passing run is %q, want proven", state)
	}

	if state := stateOf(t, checkOf(entity.CheckMethodRegression), only...); state != entity.CheckStateUnproven {
		t.Fatalf("a regression check with only a passing run is %q, want unproven", state)
	}
}

func TestAManualCheckNeedsAPersonsAttestation(t *testing.T) {
	cases := []struct {
		name     string
		evidence []entity.Evidence
		want     entity.CheckState
	}{
		{
			"an agent says it passed",
			[]entity.Evidence{byAgent(entity.EvidencePassed, 1)},
			entity.CheckStateUnproven,
		},
		{
			"a token says it passed",
			[]entity.Evidence{filed(entity.EvidencePassed, 1, entity.ActorKindToken)},
			entity.CheckStateUnproven,
		},
		{
			"a person says it passed",
			[]entity.Evidence{byPerson(entity.EvidencePassed, 1)},
			entity.CheckStateProven,
		},
		{
			"a person passed it and an agent later found it broken",
			[]entity.Evidence{byPerson(entity.EvidencePassed, 1), byAgent(entity.EvidenceFailed, 2)},
			entity.CheckStateFailed,
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			state := stateOf(t, checkOf(entity.CheckMethodManual), testcase.evidence...)

			if state != testcase.want {
				t.Fatalf("state is %q, want %q", state, testcase.want)
			}
		})
	}
}

func TestOnlyAPersonSettlesACheckAndTheEvidenceStopsMattering(t *testing.T) {
	for _, resolution := range []struct {
		resolution entity.CheckResolution
		want       entity.CheckState
	}{
		{entity.CheckResolutionWaived, entity.CheckStateWaived},
		{entity.CheckResolutionGap, entity.CheckStateGap},
	} {
		t.Run(string(resolution.resolution), func(t *testing.T) {
			check := checkOf(entity.CheckMethodCommand)
			check.Resolution = resolution.resolution

			state := stateOf(t, check, byAgent(entity.EvidenceFailed, 1))

			if state != resolution.want {
				t.Fatalf("state is %q, want %q whatever the evidence says", state, resolution.want)
			}
		})
	}
}

func TestAnUnapprovedCheckNeverBlocksHoweverUnprovenItIs(t *testing.T) {
	for _, approval := range []entity.CheckApproval{
		entity.CheckApprovalPending,
		entity.CheckApprovalDeclined,
	} {
		t.Run(string(approval), func(t *testing.T) {
			check := checkOf(entity.CheckMethodCommand)
			check.Approval = approval

			report := entity.NewCheckReport(check, []entity.Evidence{
				byAgent(entity.EvidenceFailed, 1),
			}, now())

			if report.Blocks() {
				t.Fatal("a check nobody approved is blocking completion")
			}
		})
	}
}

func TestAnApprovedCheckBlocksUntilItIsProvenOrSetAside(t *testing.T) {
	cases := []struct {
		name     string
		evidence []entity.Evidence
		blocks   bool
	}{
		{"nothing filed", nil, true},
		{"it failed", []entity.Evidence{byAgent(entity.EvidenceFailed, 1)}, true},
		{"nothing bad appeared", []entity.Evidence{byAgent(entity.EvidenceAbsentNegative, 1)}, true},
		{"it passed", []entity.Evidence{byAgent(entity.EvidencePassed, 1)}, false},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			report := entity.NewCheckReport(checkOf(entity.CheckMethodCommand), testcase.evidence, now())

			if report.Blocks() != testcase.blocks {
				t.Fatalf("blocks is %v, want %v", report.Blocks(), testcase.blocks)
			}
		})
	}
}

func TestExpiredEvidenceNoLongerProvesAnything(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	window := 30 * time.Minute
	check.TimeLimit = &window

	stale := stateOf(t, check, byAgent(entity.EvidencePassed, 1))
	if stale != entity.CheckStateUnproven {
		t.Fatalf("proof older than its window left the check %q, want unproven", stale)
	}

	fresh := stateOf(t, check, byAgent(entity.EvidencePassed, 45))
	if fresh != entity.CheckStateProven {
		t.Fatalf("proof inside its window left the check %q, want proven", fresh)
	}
}

func TestACheckWithNoTimeLimitFallsBackToTheOneNornKeeps(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	if check.Window() != entity.CheckTimeLimitDefault {
		t.Fatalf("window is %s, want the default %s", check.Window(), entity.CheckTimeLimitDefault)
	}

	horizon := entity.EvidenceHorizon{Now: at(1).Add(entity.CheckTimeLimitDefault)}
	report := entity.NewCheckReport(check, []entity.Evidence{byAgent(entity.EvidencePassed, 1)}, horizon)

	if report.State != entity.CheckStateUnproven {
		t.Fatalf("proof past the default window left the check %q, want unproven", report.State)
	}
}

func TestARefutationDoesNotExpireSoARegressionStaysProvable(t *testing.T) {
	check := checkOf(entity.CheckMethodRegression)

	window := 30 * time.Minute
	check.TimeLimit = &window

	state := stateOf(t, check,
		byAgent(entity.EvidenceFailed, 1),
		byAgent(entity.EvidencePassed, 45),
	)

	if state != entity.CheckStateProven {
		t.Fatalf(
			"a regression whose failing half is older than the window is %q, want proven: "+
				"nobody can re-run the failing case once the bug is fixed",
			state,
		)
	}
}

func TestTheTimeLimitRunsFromWhenNornReceivedTheEvidence(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	window := 30 * time.Minute
	check.TimeLimit = &window

	backdated := byAgent(entity.EvidencePassed, 45)
	backdated.ObservedAt = at(-10_000)

	if state := stateOf(t, check, backdated); state != entity.CheckStateProven {
		t.Fatalf("a backdated observation left the check %q, want proven: only receipt counts", state)
	}

	forwarded := byAgent(entity.EvidencePassed, 1)
	forwarded.ObservedAt = at(10_000)

	if state := stateOf(t, check, forwarded); state != entity.CheckStateUnproven {
		t.Fatalf(
			"an observation claimed for the future left the check %q, want unproven: "+
				"misreporting when it looked cannot make evidence immortal",
			state,
		)
	}
}

func TestEvidenceFiledAgainstAnotherCheckIsIgnored(t *testing.T) {
	elsewhere := byAgent(entity.EvidencePassed, 1)
	elsewhere.CheckID = uuid.New()

	report := entity.NewCheckReport(checkOf(entity.CheckMethodCommand), []entity.Evidence{elsewhere}, now())

	if len(report.Evidence) != 0 || report.State != entity.CheckStateUnproven {
		t.Fatal("a report picked up evidence filed against a different check")
	}
}

func TestACheckRestingOnlyOnAbsenceIsVisiblyDifferentFromOneWithNothing(t *testing.T) {
	quiet := entity.NewCheckReport(checkOf(entity.CheckMethodCommand), []entity.Evidence{
		byAgent(entity.EvidenceAbsentNegative, 1),
	}, now())

	if !quiet.RestsOnAbsence() {
		t.Fatal("a check with only a quiet observation does not report that it rests on absence")
	}

	bare := entity.NewCheckReport(checkOf(entity.CheckMethodCommand), nil, now())

	if bare.RestsOnAbsence() {
		t.Fatal("a check with no evidence at all reports that it rests on absence")
	}
}

func TestACheckThatLostItsProofIsVisiblyDifferentFromOneNeverProven(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	window := 30 * time.Minute
	check.TimeLimit = &window

	lapsed := entity.NewCheckReport(check, []entity.Evidence{byAgent(entity.EvidencePassed, 1)}, now())

	if !lapsed.Expired() {
		t.Fatal("a check whose proof timed out does not report that it expired")
	}

	bare := entity.NewCheckReport(check, nil, now())

	if bare.Expired() {
		t.Fatal("a check that was never proven reports that its proof expired")
	}
}

func TestASummaryCountsWhatSomebodyNeedsBeforeTheyApprove(t *testing.T) {
	proven := checkOf(entity.CheckMethodCommand)

	failed := checkOf(entity.CheckMethodCommand)
	failed.ID = uuid.New()

	waived := checkOf(entity.CheckMethodCommand)
	waived.ID = uuid.New()
	waived.Resolution = entity.CheckResolutionWaived

	proposed := checkOf(entity.CheckMethodCommand)
	proposed.ID = uuid.New()
	proposed.Approval = entity.CheckApprovalPending

	quiet := checkOf(entity.CheckMethodCommand)
	quiet.ID = uuid.New()

	evidence := []entity.Evidence{
		byAgent(entity.EvidencePassed, 1),
		withCheck(byAgent(entity.EvidenceFailed, 1), failed.ID),
		withCheck(byAgent(entity.EvidenceAbsentNegative, 1), quiet.ID),
	}

	summary := entity.Summarise(entity.ReportChecks(
		[]entity.Check{proven, failed, waived, proposed, quiet}, evidence, now(),
	))

	if summary.Total != 5 || summary.Proven != 1 || summary.Failed != 1 ||
		summary.Waived != 1 || summary.Unproven != 2 {
		t.Fatalf("summary tallied %+v", summary)
	}

	if summary.Unapproved != 1 {
		t.Fatalf("summary counted %d unapproved, want 1", summary.Unapproved)
	}

	if summary.Blocking != 2 {
		t.Fatalf(
			"summary counted %d blocking, want the failed one and the quiet one: the proposed "+
				"check is unproven too, but nobody has approved it",
			summary.Blocking,
		)
	}

	if summary.RestingOnAbsence != 1 {
		t.Fatalf("summary counted %d resting on absence, want 1", summary.RestingOnAbsence)
	}
}

func withCheck(evidence entity.Evidence, id uuid.UUID) entity.Evidence {
	evidence.CheckID = id

	return evidence
}

func TestBlockingChecksNamesTheOnesInTheWay(t *testing.T) {
	proven := checkOf(entity.CheckMethodCommand)

	unproven := checkOf(entity.CheckMethodCommand)
	unproven.ID = uuid.New()
	unproven.Statement = "the retry runs without a duplicate charge"

	blocking := entity.BlockingChecks(entity.ReportChecks(
		[]entity.Check{proven, unproven},
		[]entity.Evidence{byAgent(entity.EvidencePassed, 1)},
		now(),
	))

	if len(blocking) != 1 || blocking[0].ID != unproven.ID {
		t.Fatalf("blocking checks are %v, want only the unproven one", blocking)
	}
}

func awaitingOf(t *testing.T, check entity.Check, evidence ...entity.Evidence) entity.CheckAwaiting {
	t.Helper()

	return entity.NewCheckReport(check, evidence, now()).Awaiting()
}

func TestAnUnprovenCheckSaysWhichOfTheRulesItIsStuckOn(t *testing.T) {
	timedOut := entity.CheckTimeLimitDefault + time.Hour

	for _, tc := range []struct {
		name     string
		check    entity.Check
		evidence []entity.Evidence
		want     entity.CheckAwaiting
	}{
		{
			name:  "nothing filed at all",
			check: checkOf(entity.CheckMethodCommand),
			want:  entity.CheckAwaitingEvidence,
		},
		{
			name:     "only the absence of a failure",
			check:    checkOf(entity.CheckMethodObservation),
			evidence: []entity.Evidence{byAgent(entity.EvidenceAbsentNegative, 5)},
			want:     entity.CheckAwaitingPositiveResult,
		},
		{
			name:     "an agent attesting a manual check",
			check:    checkOf(entity.CheckMethodManual),
			evidence: []entity.Evidence{byAgent(entity.EvidencePassed, 5)},
			want:     entity.CheckAwaitingAttestation,
		},
		{
			name:     "a regression passing without ever having failed",
			check:    checkOf(entity.CheckMethodRegression),
			evidence: []entity.Evidence{byAgent(entity.EvidencePassed, 5)},
			want:     entity.CheckAwaitingPriorFailure,
		},
		{
			name:  "a proof that timed out",
			check: checkOf(entity.CheckMethodCommand),
			evidence: []entity.Evidence{
				withReceipt(byAgent(entity.EvidencePassed, 5), at(5).Add(-timedOut)),
			},
			want: entity.CheckAwaitingFreshProof,
		},
		{
			name:     "the newest result disproves it",
			check:    checkOf(entity.CheckMethodCommand),
			evidence: []entity.Evidence{byAgent(entity.EvidencePassed, 5), byAgent(entity.EvidenceFailed, 9)},
			want:     entity.CheckAwaitingCorrection,
		},
		{
			name:     "proven, so nothing",
			check:    checkOf(entity.CheckMethodCommand),
			evidence: []entity.Evidence{byAgent(entity.EvidencePassed, 5)},
			want:     entity.CheckAwaitingNothing,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if awaiting := awaitingOf(t, tc.check, tc.evidence...); awaiting != tc.want {
				t.Fatalf(
					"a check %s reports it is awaiting %q, want %q; this is the sentence an "+
						"agent is given to act on, so a wrong one sends it to do the wrong work",
					tc.name, awaiting, tc.want,
				)
			}
		})
	}
}

func TestAWaivedCheckIsAwaitingNothingHoweverLittleProvesIt(t *testing.T) {
	check := checkOf(entity.CheckMethodManual)
	check.Resolution = entity.CheckResolutionWaived

	if awaiting := awaitingOf(t, check); awaiting != entity.CheckAwaitingNothing {
		t.Fatalf("a waived check still asks for %q", awaiting)
	}
}

func withReceipt(evidence entity.Evidence, received time.Time) entity.Evidence {
	evidence.ReceivedAt = received

	return evidence
}

func atHead(evidence entity.Evidence, linkID uuid.UUID, sha string) entity.Evidence {
	evidence.CodeLinkID = linkID
	evidence.CommitSHA = sha

	return evidence
}

func TestAProofStopsCountingWhenTheChangeItWasTakenAtMovesOn(t *testing.T) {
	linkID := uuid.New()
	check := checkOf(entity.CheckMethodCommand)

	proof := atHead(byAgent(entity.EvidencePassed, 5), linkID, "aaaaaaa")

	report := entity.NewCheckReport(check, []entity.Evidence{proof}, entity.EvidenceHorizon{
		Now:   at(60),
		Heads: map[uuid.UUID]string{linkID: "bbbbbbb"},
	})

	if report.State != entity.CheckStateUnproven {
		t.Fatalf(
			"state = %q, want unproven; a proof taken at one commit says nothing about the code "+
				"that replaced it",
			report.State,
		)
	}

	if report.Evidence[0].Expiry != entity.EvidenceHeadMoved {
		t.Errorf("expiry = %q, want head_moved", report.Evidence[0].Expiry)
	}

	if report.Awaiting() != entity.CheckAwaitingFreshProof {
		t.Errorf("awaiting = %q, want fresh_proof", report.Awaiting())
	}
}

func TestAProofSurvivesWhileTheChangeItWasTakenAtStandsStill(t *testing.T) {
	linkID := uuid.New()
	check := checkOf(entity.CheckMethodCommand)

	proof := atHead(byAgent(entity.EvidencePassed, 5), linkID, "aaaaaaa")

	report := entity.NewCheckReport(check, []entity.Evidence{proof}, entity.EvidenceHorizon{
		Now:   at(60),
		Heads: map[uuid.UUID]string{linkID: "aaaaaaa"},
	})

	if report.State != entity.CheckStateProven {
		t.Fatalf("state = %q, want proven", report.State)
	}
}

func TestAProofNornCouldNotBindToAChangeIsJudgedOnTimeAlone(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	report := entity.NewCheckReport(
		check,
		[]entity.Evidence{byAgent(entity.EvidencePassed, 5)},
		entity.EvidenceHorizon{Now: at(60), Heads: map[uuid.UUID]string{uuid.New(): "bbbbbbb"}},
	)

	if report.State != entity.CheckStateProven {
		t.Fatalf(
			"state = %q, want proven; evidence with no linked change has no head to compare "+
				"against, and inventing one would expire proofs nobody can refresh",
			report.State,
		)
	}
}

func TestARefutationNeverExpiresWhenTheHeadMoves(t *testing.T) {
	linkID := uuid.New()
	check := checkOf(entity.CheckMethodRegression)

	failure := atHead(byAgent(entity.EvidenceFailed, 1), linkID, "aaaaaaa")
	proof := atHead(byAgent(entity.EvidencePassed, 5), linkID, "bbbbbbb")

	report := entity.NewCheckReport(check, []entity.Evidence{failure, proof}, entity.EvidenceHorizon{
		Now:   at(60),
		Heads: map[uuid.UUID]string{linkID: "bbbbbbb"},
	})

	if report.Evidence[0].Expiry.Expired() {
		t.Fatal(
			"the failing observation expired when the head moved, which is exactly what it is " +
				"supposed to outlive: a regression can never be proven again once its before is gone",
		)
	}

	if report.State != entity.CheckStateProven {
		t.Fatalf("state = %q, want proven; the passing result is at the current head", report.State)
	}
}

func TestAProofAtAChangeNornNoLongerTracksIsLeftAlone(t *testing.T) {
	check := checkOf(entity.CheckMethodCommand)

	proof := atHead(byAgent(entity.EvidencePassed, 5), uuid.New(), "aaaaaaa")

	report := entity.NewCheckReport(check, []entity.Evidence{proof}, entity.EvidenceHorizon{
		Now:   at(60),
		Heads: map[uuid.UUID]string{},
	})

	if report.State != entity.CheckStateProven {
		t.Fatalf(
			"state = %q, want proven; the link was removed, so there is no head to compare and "+
				"nothing honest to say about whether the proof still holds",
			report.State,
		)
	}
}
