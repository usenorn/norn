package entity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestADocTitleIsRequiredAndBounded(t *testing.T) {
	for name, testCase := range map[string]struct {
		title string
		code  string
	}{
		"a plain title":       {title: "Onboarding", code: ""},
		"blank":               {title: "   ", code: entity.ValidationCodeRequired},
		"a NUL inside":        {title: "On\x00boarding", code: entity.ValidationCodeMalformed},
		"at the limit":        {title: strings.Repeat("ж", entity.DocTitleMaxLen), code: ""},
		"one rune past it":    {title: strings.Repeat("ж", entity.DocTitleMaxLen+1), code: entity.ValidationCodeTooLong},
		"padding not counted": {title: "  " + strings.Repeat("a", entity.DocTitleMaxLen) + "  ", code: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := entity.ValidateDocTitle("title", testCase.title); got.Code != testCase.code {
				t.Fatalf("title %q got code %q, want %q", testCase.title, got.Code, testCase.code)
			}
		})
	}
}

func TestDocContentHoldsMoreThanAnIssueDescription(t *testing.T) {
	longer := strings.Repeat("a", entity.IssueDescriptionMaxLen+1)

	if got := entity.ValidateDocContent("content", longer); got.Code != "" {
		t.Fatalf("a doc refused content an issue description could not hold: %q", got.Code)
	}

	if got := entity.ValidateDocContent("content", longer+strings.Repeat("a", entity.DocContentMaxLen)); got.Code != entity.ValidationCodeTooLong {
		t.Fatalf("content past the doc limit got %q", got.Code)
	}

	deep := entity.Node{Type: entity.NodeBlockquote}
	nested := &deep

	for range entity.IssueDocumentLimits.MaxDepth {
		nested.Content = []entity.Node{{Type: entity.NodeBlockquote}}
		nested = &nested.Content[0]
	}

	if err := entity.ValidateDocument(entity.NewDocument(deep), entity.DocLimits); err != nil {
		t.Fatalf("a doc refused nesting an issue description would refuse: %v", err)
	}
}

func TestAStaleDocVersionIsRefusedBeforeAnythingElse(t *testing.T) {
	doc := entity.Doc{Version: 4, Status: entity.DocStatusArchived}

	var stale entity.DocStaleError
	if err := doc.Expect(3); !errors.As(err, &stale) || stale.Version != 4 || !errors.Is(err, entity.ErrDocStale) {
		t.Fatalf("a write against version 3 of a version-4 doc got %v, want a stale error naming 4", err)
	}

	if err := doc.Expect(4); !errors.Is(err, entity.ErrDocArchived) {
		t.Fatalf("a current write against an archived doc got %v", err)
	}

	doc.Status = entity.DocStatusActive
	if err := doc.Expect(4); err != nil {
		t.Fatalf("a current write against an active doc got %v", err)
	}
}

func TestADocIsPlacedOnlyWhereTheTreeHoldsIt(t *testing.T) {
	docID := uuid.New()
	ancestry := func(depth int) []uuid.UUID {
		chain := make([]uuid.UUID, depth)
		for index := range chain {
			chain[index] = uuid.New()
		}

		return chain
	}

	for name, testCase := range map[string]struct {
		ancestry []uuid.UUID
		height   int
		want     error
	}{
		"at the root":                {ancestry: nil, height: 0, want: nil},
		"beneath itself":             {ancestry: []uuid.UUID{docID}, height: 0, want: entity.ErrDocParentCycle},
		"beneath its own child":      {ancestry: []uuid.UUID{uuid.New(), docID}, height: 1, want: entity.ErrDocParentCycle},
		"filling the tree exactly":   {ancestry: ancestry(entity.DocTreeMaxDepth - 2), height: 1, want: nil},
		"its subtree past the floor": {ancestry: ancestry(entity.DocTreeMaxDepth - 2), height: 2, want: entity.ErrDocTreeTooDeep},
	} {
		t.Run(name, func(t *testing.T) {
			if err := entity.ValidateDocPlacement(docID, testCase.ancestry, testCase.height); !errors.Is(err, testCase.want) {
				t.Fatalf("placement got %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestADocIsJudgedByItsOwnAudience(t *testing.T) {
	projectID := uuid.New()
	doc := entity.Doc{
		WorkspaceID:    uuid.New(),
		AuthorID:       uuid.New(),
		Audience:       entity.ProjectAudience(projectID),
		ContributorIDs: []uuid.UUID{uuid.New()},
	}

	access := doc.Access()

	if access.Audience != doc.Audience || access.AuthorID != doc.AuthorID || access.WorkspaceID != doc.WorkspaceID {
		t.Fatalf("the doc's access lost who wrote it or who reads it: %+v", access)
	}

	if access.Applicability.ProjectID == nil || *access.Applicability.ProjectID != projectID {
		t.Fatalf("a project doc does not apply to its project: %+v", access.Applicability)
	}

	if !access.ContributedBy(doc.ContributorIDs[0]) {
		t.Fatal("somebody who edited the doc is not counted as a contributor")
	}

	doc.Audience = entity.WorkspaceAudience()
	if doc.Access().Applicability.ProjectID != nil {
		t.Fatal("a workspace doc still claims to apply to a project")
	}
}

func TestARevisionRecordsWhoChangedTheDocAndHow(t *testing.T) {
	owner := uuid.New()
	connection := uuid.New()
	agent := entity.Agent{ID: uuid.New(), AccountID: uuid.New(), OwnerAccountID: owner, Scope: entity.AgentScopeMember}
	agentActor := agent.Attribute(entity.Actor{Kind: entity.ActorKindAgent, AccountID: agent.AccountID})

	for name, testCase := range map[string]struct {
		actor      entity.Actor
		principal  uuid.UUID
		agentID    *uuid.UUID
		connection *uuid.UUID
		source     entity.RevisionSource
	}{
		"a person": {
			actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: owner},
			principal: owner,
			source:    entity.RevisionSourcePerson,
		},
		"a person through a connection": {
			actor:      entity.Actor{Kind: entity.ActorKindToken, AccountID: owner, OwnerAccountID: owner, ConnectionID: &connection},
			principal:  owner,
			connection: &connection,
			source:     entity.RevisionSourcePerson,
		},
		"an agent acting for its owner": {
			actor:     agentActor,
			principal: owner,
			agentID:   &agent.ID,
			source:    entity.RevisionSourceAgent,
		},
	} {
		t.Run(name, func(t *testing.T) {
			decision := entity.KnowledgeDecision{
				Decision: entity.Decision{Actor: testCase.actor},
				Reader:   entity.KnowledgeReader{Identity: entity.KnowledgeIdentityOf(testCase.actor, entity.MembershipRoleMember)},
			}

			by := entity.DocAttributionOf(decision)

			if by.AccountID != testCase.principal || by.Source != testCase.source {
				t.Fatalf("attribution %+v, want account %s and source %s", by, testCase.principal, testCase.source)
			}

			if !sameID(by.AgentID, testCase.agentID) || !sameID(by.ConnectionID, testCase.connection) {
				t.Fatalf("attribution %+v lost the agent or the connection", by)
			}
		})
	}
}

func TestARestoredRevisionIsAttributedAsARestore(t *testing.T) {
	parent := uuid.New()
	doc := entity.Doc{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		ParentID:    &parent,
		Title:       "Runbook",
		Markdown:    "Steps.",
		Audience:    entity.WorkspaceAudience(),
		Version:     7,
		UpdatedAt:   time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
	}
	by := entity.DocAttribution{AccountID: uuid.New(), Source: entity.RevisionSourceAgent}

	edited := doc.Revision(entity.DocRevisionEdited, by)
	restored := doc.Revision(entity.DocRevisionRestored, by)

	if edited.By.Source != entity.RevisionSourceAgent || restored.By.Source != entity.RevisionSourceRestore {
		t.Fatalf("edit source %s, restore source %s", edited.By.Source, restored.By.Source)
	}

	if restored.Version != doc.Version || restored.Title != doc.Title || restored.ParentID != doc.ParentID ||
		!restored.CreatedAt.Equal(doc.UpdatedAt) || restored.ID == edited.ID {
		t.Fatalf("the revision is not a snapshot of the doc it records: %+v", restored)
	}
}

func TestDocCursorsSurviveTheRoundTrip(t *testing.T) {
	cursor := entity.DocCursor{UpdatedAt: time.Date(2026, 10, 24, 9, 0, 0, 123, time.UTC), DocID: uuid.New()}

	decoded, err := entity.DecodeDocCursor(cursor.Encode())
	if err != nil || decoded.DocID != cursor.DocID || !decoded.UpdatedAt.Equal(cursor.UpdatedAt) {
		t.Fatalf("doc cursor round trip got %+v, %v", decoded, err)
	}

	revision, err := entity.DecodeDocRevisionCursor(entity.DocRevisionCursor{Version: 12}.Encode())
	if err != nil || revision.Version != 12 {
		t.Fatalf("revision cursor round trip got %+v, %v", revision, err)
	}

	for _, raw := range []string{"%%%", "c2hvcnQ"} {
		if _, err := entity.DecodeDocCursor(raw); !errors.Is(err, entity.ErrDocCursorInvalid) {
			t.Fatalf("doc cursor %q got %v", raw, err)
		}
	}

	if _, err := entity.DecodeDocRevisionCursor("MA"); !errors.Is(err, entity.ErrDocRevisionCursorInvalid) {
		t.Fatalf("a version-0 revision cursor got %v", err)
	}
}

func TestDocEnumsKnowTheirOwnValues(t *testing.T) {
	for _, status := range entity.DocStatuses() {
		if !status.Valid() {
			t.Fatalf("status %q is listed but not valid", status)
		}
	}

	for _, action := range entity.DocRevisionActions() {
		if !action.Valid() {
			t.Fatalf("action %q is listed but not valid", action)
		}
	}

	if entity.DocStatus("deleted").Valid() || entity.DocRevisionAction("purged").Valid() {
		t.Fatal("an unknown status or action was accepted")
	}
}

func sameID(got, want *uuid.UUID) bool {
	if got == nil || want == nil {
		return got == want
	}

	return *got == *want
}
