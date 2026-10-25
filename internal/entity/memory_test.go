package entity_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheLifecycleMovesAMemoryBetweenStatuses(t *testing.T) {
	allowed := map[entity.MemoryStatus][]entity.MemoryStatus{
		entity.MemoryStatusProposed: {entity.MemoryStatusAccepted, entity.MemoryStatusDisputed, entity.MemoryStatusWithdrawn, entity.MemoryStatusRejected},
		entity.MemoryStatusAccepted: {entity.MemoryStatusProposed, entity.MemoryStatusDisputed, entity.MemoryStatusSuperseded, entity.MemoryStatusWithdrawn},
		entity.MemoryStatusDisputed: {entity.MemoryStatusAccepted, entity.MemoryStatusSuperseded, entity.MemoryStatusWithdrawn, entity.MemoryStatusRejected},
	}

	for _, from := range entity.MemoryStatuses() {
		for _, to := range entity.MemoryStatuses() {
			want := false

			for _, next := range allowed[from] {
				want = want || next == to
			}

			if got := from.CanBecome(to); got != want {
				t.Errorf("%s to %s allowed = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestSupersededWithdrawnAndRejectedMemoriesAreFinal(t *testing.T) {
	for _, status := range entity.MemoryStatuses() {
		want := status == entity.MemoryStatusSuperseded || status == entity.MemoryStatusWithdrawn ||
			status == entity.MemoryStatusRejected

		if got := status.Terminal(); got != want {
			t.Errorf("%s terminal = %v, want %v", status, got, want)
		}
	}
}

func TestARefusedTransitionLeavesTheMemoryAsItWas(t *testing.T) {
	memory := entity.Memory{Status: entity.MemoryStatusProposed}

	err := memory.Become(entity.MemoryStatusSuperseded)

	var refusal entity.MemoryTransitionError
	if !errors.As(err, &refusal) || refusal.From != entity.MemoryStatusProposed || refusal.To != entity.MemoryStatusSuperseded {
		t.Fatalf("superseding a proposal got %v, want a transition error naming both statuses", err)
	}

	if !errors.Is(err, entity.ErrMemoryTransitionInvalid) {
		t.Fatalf("the transition error does not unwrap to the sentinel: %v", err)
	}

	if memory.Status != entity.MemoryStatusProposed {
		t.Fatalf("a refused transition moved the memory to %s", memory.Status)
	}
}

func TestSupersedingRecordsTheReplacement(t *testing.T) {
	memory := entity.Memory{Status: entity.MemoryStatusAccepted}
	replacement := uuid.New()

	if err := memory.SupersededBy(replacement); err != nil {
		t.Fatalf("superseding an accepted memory: %v", err)
	}

	if memory.Status != entity.MemoryStatusSuperseded || memory.SupersededByID == nil || *memory.SupersededByID != replacement {
		t.Fatalf("got status %s replaced by %v, want superseded by %s", memory.Status, memory.SupersededByID, replacement)
	}
}

func TestAStaleMemoryVersionIsRefusedBeforeItsStatus(t *testing.T) {
	memory := entity.Memory{Version: 3, Status: entity.MemoryStatusWithdrawn}

	var stale entity.MemoryStaleError
	if err := memory.Expect(2); !errors.As(err, &stale) || stale.Version != 3 {
		t.Fatalf("expecting version 2 got %v, want a stale error naming 3", err)
	}

	if err := memory.Expect(3); !errors.Is(err, entity.ErrMemoryClosed) {
		t.Fatalf("a withdrawn memory at its own version got %v, want closed", err)
	}

	memory.Status = entity.MemoryStatusDisputed

	if err := memory.Expect(3); err != nil {
		t.Fatalf("a disputed memory refused a change: %v", err)
	}
}

func validMemory() entity.Memory {
	return entity.Memory{
		Kind:          entity.MemoryKindConstraint,
		Statement:     "Use SQLite",
		Conditions:    "local prototypes only",
		Basis:         entity.MemoryBasisObservation,
		EffectiveFrom: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC),
	}
}

func TestAMemoryNeedsAStatementKindBasisAndStart(t *testing.T) {
	before := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)

	for name, testCase := range map[string]struct {
		alter func(*entity.Memory)
		field string
		code  string
	}{
		"a whole memory":           {alter: func(*entity.Memory) {}},
		"a blank statement":        {alter: func(m *entity.Memory) { m.Statement = "  " }, field: entity.MemoryFieldStatement, code: entity.ValidationCodeRequired},
		"a statement past the cap": {alter: func(m *entity.Memory) { m.Statement = strings.Repeat("ж", entity.MemoryStatementMaxLen+1) }, field: entity.MemoryFieldStatement, code: entity.ValidationCodeTooLong},
		"an unknown kind":          {alter: func(m *entity.Memory) { m.Kind = "rumour" }, field: entity.MemoryFieldKind, code: entity.ValidationCodeUnsupportedValue},
		"an unknown basis":         {alter: func(m *entity.Memory) { m.Basis = "hunch" }, field: entity.MemoryFieldBasis, code: entity.ValidationCodeUnsupportedValue},
		"no effective date":        {alter: func(m *entity.Memory) { m.EffectiveFrom = time.Time{} }, field: entity.MemoryFieldEffectiveFrom, code: entity.ValidationCodeRequired},
		"ending before it starts":  {alter: func(m *entity.Memory) { m.EffectiveUntil = &before }, field: entity.MemoryFieldEffectiveUntil, code: entity.ValidationCodeOutOfRange},
		"ending the day it starts": {alter: func(m *entity.Memory) { day := m.EffectiveFrom; m.EffectiveUntil = &day }},
		"a component past the cap": {alter: func(m *entity.Memory) {
			m.Applicability.Component = strings.Repeat("c", entity.MemoryComponentMaxLen+1)
		}, field: entity.MemoryFieldComponent, code: entity.ValidationCodeTooLong},
		"too much evidence": {
			alter: func(m *entity.Memory) {
				m.Evidence = make([]entity.MemoryEvidence, entity.MemoryEvidenceMax+1)
				for i := range m.Evidence {
					m.Evidence[i] = entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "seen"}
				}
			},
			field: entity.MemoryFieldEvidence, code: entity.ValidationCodeTooLong,
		},
	} {
		t.Run(name, func(t *testing.T) {
			memory := validMemory()
			testCase.alter(&memory)

			assertFieldError(t, memory.Validate(), testCase.field, testCase.code)
		})
	}
}

func TestEvidenceCitesExactlyWhatItsKindNames(t *testing.T) {
	id := uuid.New()
	other := uuid.New()

	for name, testCase := range map[string]struct {
		evidence entity.MemoryEvidence
		code     string
	}{
		"an issue":                     {entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &id}, ""},
		"an issue with a doc":          {entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &id, DocID: &other}, entity.ValidationCodeMalformed},
		"an issue without its id":      {entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue}, entity.ValidationCodeMalformed},
		"a comment":                    {entity.MemoryEvidence{Kind: entity.MemoryEvidenceComment, CommentID: &id}, ""},
		"a comment with an excerpt":    {entity.MemoryEvidence{Kind: entity.MemoryEvidenceComment, CommentID: &id, Excerpt: "said"}, entity.ValidationCodeMalformed},
		"a doc revision":               {entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, DocID: &id, DocRevisionID: &other}, ""},
		"a doc revision without a doc": {entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, DocRevisionID: &other}, entity.ValidationCodeMalformed},
		"an excerpt":                   {entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "the build broke on SQLite"}, ""},
		"a blank excerpt":              {entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: " "}, entity.ValidationCodeRequired},
		"an excerpt past the cap":      {entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: strings.Repeat("e", entity.MemoryExcerptMaxLen+1)}, entity.ValidationCodeTooLong},
		"an excerpt naming an issue":   {entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "said", IssueID: &id}, entity.ValidationCodeMalformed},
		"an unknown kind":              {entity.MemoryEvidence{Kind: "rumour"}, entity.ValidationCodeUnsupportedValue},
	} {
		t.Run(name, func(t *testing.T) {
			if got := testCase.evidence.Validate("evidence[0]"); got.Code != testCase.code {
				t.Fatalf("got code %q, want %q", got.Code, testCase.code)
			}
		})
	}
}

func TestEvidenceIsShownOnlyToThoseWhoCanSeeItsSource(t *testing.T) {
	workspaceID := uuid.New()
	author := uuid.New()
	visible := uuid.New()
	hidden := uuid.New()
	reader := entity.KnowledgeReader{
		WorkspaceID: workspaceID,
		Identity:    entity.KnowledgeIdentity{Principal: uuid.New(), Personal: true, Role: entity.MembershipRoleMember},
	}

	for name, testCase := range map[string]struct {
		evidence entity.MemoryEvidence
		want     bool
	}{
		"a visible issue":           {entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &visible}, true},
		"a hidden issue":            {entity.MemoryEvidence{Kind: entity.MemoryEvidenceIssue, IssueID: &hidden}, false},
		"a comment on a hidden one": {entity.MemoryEvidence{Kind: entity.MemoryEvidenceComment, CommentID: &visible, IssueID: &hidden}, false},
		"a workspace doc":           {entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, DocAudience: entity.WorkspaceAudience(), DocAuthorID: author}, true},
		"someone's personal doc":    {entity.MemoryEvidence{Kind: entity.MemoryEvidenceDocRevision, DocAudience: entity.PersonalAudience(), DocAuthorID: author}, false},
		"a pasted excerpt":          {entity.MemoryEvidence{Kind: entity.MemoryEvidenceExcerpt, Excerpt: "said"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := testCase.evidence.SeenBy(reader, []uuid.UUID{visible}); got != testCase.want {
				t.Fatalf("seen = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAMemoryRevisionSnapshotsWhatTheMemoryHeld(t *testing.T) {
	memory := validMemory()
	memory.ID, memory.Version, memory.Status = uuid.New(), 4, entity.MemoryStatusDisputed
	memory.UpdatedAt = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	agentID := uuid.New()
	by := entity.KnowledgeAttribution{AccountID: uuid.New(), AgentID: &agentID, Source: entity.RevisionSourceAgent}

	revision := memory.Revision(entity.MemoryRevisionCorrected, by)

	if revision.MemoryID != memory.ID || revision.Version != 4 || revision.Status != entity.MemoryStatusDisputed {
		t.Fatalf("revision %+v does not describe version 4 of the memory", revision.MemoryRevisionSummary)
	}

	if revision.Conditions != memory.Conditions || !revision.CreatedAt.Equal(memory.UpdatedAt) {
		t.Fatalf("revision lost the memory's conditions or time: %+v", revision)
	}

	if revision.By.AgentID == nil || *revision.By.AgentID != agentID {
		t.Fatalf("revision lost the acting agent: %+v", revision.By)
	}
}

func assertFieldError(t *testing.T, err error, field, code string) {
	t.Helper()

	if field == "" {
		if err != nil {
			t.Fatalf("got %v, want no error", err)
		}

		return
	}

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("got %v, want a validation error on %s", err, field)
	}

	for _, fieldError := range validation.Fields {
		if fieldError.Field == field && fieldError.Code == code {
			return
		}
	}

	t.Fatalf("got %v, want %s on %s", validation, code, field)
}

func TestAMemoryRestatesAnotherOnlyWhenEveryContentFieldAgrees(t *testing.T) {
	issueID := uuid.New()
	until := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	for name, testCase := range map[string]struct {
		alter func(*entity.Memory)
		want  bool
	}{
		"the same memory":        {alter: func(*entity.Memory) {}, want: true},
		"its status alone moved": {alter: func(m *entity.Memory) { m.Status = entity.MemoryStatusDisputed }, want: true},
		"another statement":      {alter: func(m *entity.Memory) { m.Statement = "Use Postgres" }, want: false},
		"another component":      {alter: func(m *entity.Memory) { m.Applicability.Component = "billing" }, want: false},
		"an end date":            {alter: func(m *entity.Memory) { m.EffectiveUntil = &until }, want: false},
		"another basis":          {alter: func(m *entity.Memory) { m.Basis = entity.MemoryBasisInference }, want: false},
		"evidence cited": {alter: func(m *entity.Memory) {
			m.Evidence = []entity.MemoryEvidence{{Kind: entity.MemoryEvidenceIssue, IssueID: &issueID}}
		}, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			memory := validMemory()
			changed := validMemory()
			testCase.alter(&changed)

			if got := changed.Restates(memory); got != testCase.want {
				t.Fatalf("restates = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestOnlyAPersonWhoMaintainsASharedAudienceReviewsItsProposals(t *testing.T) {
	workspaceID, projectID, author := uuid.New(), uuid.New(), uuid.New()
	admin := entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}
	lead := entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}
	member := entity.Actor{Kind: entity.ActorKindUser, AccountID: author}
	adminAgent := entity.Agent{
		ID: uuid.New(), AccountID: uuid.New(), OwnerAccountID: admin.AccountID, Scope: entity.AgentScopeMember,
	}.Attribute(entity.Actor{Kind: entity.ActorKindAgent, AccountID: uuid.New()})

	readers := map[string]entity.KnowledgeReader{
		"admin": {WorkspaceID: workspaceID, Identity: entity.KnowledgeIdentityOf(admin, entity.MembershipRoleAdmin), AllProjects: true},
		"lead": {
			WorkspaceID: workspaceID, Identity: entity.KnowledgeIdentityOf(lead, entity.MembershipRoleMember),
			ProjectIDs: []uuid.UUID{projectID}, LedProjectIDs: []uuid.UUID{projectID},
		},
		"author": {
			WorkspaceID: workspaceID, Identity: entity.KnowledgeIdentityOf(member, entity.MembershipRoleMember),
			ProjectIDs: []uuid.UUID{projectID},
		},
		"admin's agent": {
			WorkspaceID: workspaceID, Identity: entity.KnowledgeIdentityOf(adminAgent, entity.MembershipRoleAdmin),
			AllProjects: true,
		},
	}

	for name, testCase := range map[string]struct {
		audience entity.KnowledgeAudience
		settles  []string
	}{
		"a workspace memory": {audience: entity.WorkspaceAudience(), settles: []string{"admin"}},
		"a project memory":   {audience: entity.ProjectAudience(projectID), settles: []string{"admin", "lead"}},
		"a personal memory":  {audience: entity.PersonalAudience(), settles: []string{"admin", "lead", "author", "admin's agent"}},
	} {
		memory := entity.Memory{
			WorkspaceID: workspaceID, AuthorID: author, Audience: testCase.audience, Status: entity.MemoryStatusProposed,
		}

		for who, reader := range readers {
			want := slices.Contains(testCase.settles, who)

			if got := memory.SettledBy(reader); got != want {
				t.Errorf("%s settled by %s = %v, want %v", name, who, got, want)
			}

			reviewable := want && testCase.audience.Visibility != entity.KnowledgeVisibilityPersonal
			if err := memory.Review(reader); (err == nil) != reviewable {
				t.Errorf("%s reviewed by %s: %v, want allowed %v", name, who, err, reviewable)
			}
		}
	}
}

func TestOnlyAPendingMemoryAwaitsReview(t *testing.T) {
	admin := entity.KnowledgeReader{
		WorkspaceID: uuid.New(),
		Identity:    entity.KnowledgeIdentityOf(entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}, entity.MembershipRoleAdmin),
	}

	for _, status := range entity.MemoryStatuses() {
		memory := entity.Memory{WorkspaceID: admin.WorkspaceID, Audience: entity.WorkspaceAudience(), Status: status}

		err := memory.Review(admin)
		if memory.Pending() != (err == nil) {
			t.Errorf("reviewing a %s memory: %v", status, err)
		}

		if err != nil && !errors.Is(err, entity.ErrMemoryNotPending) {
			t.Errorf("reviewing a %s memory: %v, want not pending", status, err)
		}
	}
}

func TestARejectionNeedsAReasonOfBoundedLength(t *testing.T) {
	for name, testCase := range map[string]struct {
		reason string
		code   string
	}{
		"blank":    {reason: "   ", code: entity.ValidationCodeRequired},
		"too long": {reason: strings.Repeat("x", entity.MemoryReasonMaxLen+1), code: entity.ValidationCodeTooLong},
		"given":    {reason: "contradicts the release policy"},
	} {
		err := entity.ValidateMemoryReason(testCase.reason)

		if testCase.code == "" {
			if err != nil {
				t.Errorf("%s reason: %v", name, err)
			}

			continue
		}

		var invalid entity.ValidationError
		if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Code != testCase.code ||
			invalid.Fields[0].Field != entity.MemoryFieldReason {
			t.Errorf("%s reason: %v, want %s on reason", name, err, testCase.code)
		}
	}
}

func TestARejectionRecordsItsTrimmedReason(t *testing.T) {
	memory := entity.Memory{ID: uuid.New(), Status: entity.MemoryStatusRejected, Version: 2}

	revision := memory.Rejection("  contradicts the release policy ", entity.KnowledgeAttribution{AccountID: uuid.New()})

	if revision.Action != entity.MemoryRevisionRejected || revision.Reason != "contradicts the release policy" {
		t.Fatalf("got %s with reason %q, want a rejection with the trimmed reason", revision.Action, revision.Reason)
	}
}
