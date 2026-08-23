package changeset_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	changesetrepo "github.com/usenorn/norn/internal/repository/changeset"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	changesetsvc "github.com/usenorn/norn/internal/service/changeset"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	scmsvc "github.com/usenorn/norn/internal/service/scm"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

type harness struct {
	changesets *changesetrepo.MockChangeSet
	issues     *issuerepo.MockIssue
	executions *executionsvc.MockExecutions
	source     *scmsvc.MockSourceControl
	events     *eventsvc.MockEvents
	authorizer *authorizersvc.MockAuthorizer
	service    service.ChangeSets

	workspaceID uuid.UUID
	issue       entity.Issue
	runner      entity.Runner
	execution   entity.Execution

	changes     []entity.ExecutionChange
	validations []entity.ExecutionValidation
	result      entity.ExecutionResult
	published   []entity.Event
	linked      []service.LinkIssueCodeInput
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		changesets:  changesetrepo.NewMockChangeSet(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		source:      scmsvc.NewMockSourceControl(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: workspaceID,
		issue: entity.Issue{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			TeamID:       teamID,
			ReferenceKey: "NORN",
			Number:       38,
			Title:        "ChangeSet",
		},
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
	}

	h.execution = entity.Execution{
		ID:          "exec-01ABC",
		WorkspaceID: workspaceID,
		IssueID:     h.issue.ID,
		TeamID:      teamID,
		AgentID:     agentID,
		RunnerID:    h.runner.ID,
		State:       entity.ExecutionFinalizing,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.expectStore()

	h.events.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, event entity.Event) { h.published = append(h.published, event) }).
		AnyTimes()

	h.service = changesetsvc.New(
		h.changesets, h.issues, h.executions, h.source, h.events, h.authorizer, transactor,
	)

	return h
}

func (h *harness) expectStore() {
	h.changesets.EXPECT().
		SaveChange(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, change entity.ExecutionChange,
		) (entity.ExecutionChange, error) {
			for index, held := range h.changes {
				if held.ExecutionID != change.ExecutionID || held.Repository != change.Repository {
					continue
				}

				if change.ReportedAt.Before(held.ReportedAt) {
					return held, nil
				}

				change.ID = held.ID
				change.CodeLinkID = held.CodeLinkID
				h.changes[index] = change

				return change, nil
			}

			change.ID = uuid.New()
			h.changes = append(h.changes, change)

			return change, nil
		}).
		AnyTimes()

	h.changesets.EXPECT().
		SaveValidation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, validation entity.ExecutionValidation,
		) (entity.ExecutionValidation, error) {
			for index, held := range h.validations {
				if held.ExecutionID != validation.ExecutionID || held.Check != validation.Check {
					continue
				}

				if validation.ReportedAt.Before(held.ReportedAt) {
					return held, nil
				}

				validation.ID = held.ID
				h.validations[index] = validation

				return validation, nil
			}

			validation.ID = uuid.New()
			h.validations = append(h.validations, validation)

			return validation, nil
		}).
		AnyTimes()

	h.changesets.EXPECT().
		SaveResult(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, result entity.ExecutionResult,
		) (entity.ExecutionResult, error) {
			if h.result.ExecutionID != "" && result.ReportedAt.Before(h.result.ReportedAt) {
				return h.result, nil
			}

			h.result = result

			return result, nil
		}).
		AnyTimes()

	h.changesets.EXPECT().
		LinkChange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, changeID, linkID uuid.UUID) error {
			for index, held := range h.changes {
				if held.ID == changeID {
					h.changes[index].CodeLinkID = linkID
				}
			}

			return nil
		}).
		AnyTimes()

	h.changesets.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, executionID string) (entity.ExecutionChangeSet, error) {
			return entity.ExecutionChangeSet{
				Result:      h.result,
				Changes:     h.changes,
				Validations: h.validations,
			}, nil
		}).
		AnyTimes()
}

func (h *harness) holding() {
	h.executions.EXPECT().
		Held(gomock.Any(), h.runner, h.execution.ID).
		Return(h.execution, nil).
		AnyTimes()
}

func (h *harness) reading() {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{}, nil).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, h.issue.ID, gomock.Any()).
		Return(h.issue, nil).
		AnyTimes()
}

func (h *harness) links(id uuid.UUID) {
	h.source.EXPECT().
		Link(gomock.Any(), h.workspaceID, h.issue.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.LinkIssueCodeInput,
		) (entity.CodeLink, error) {
			h.linked = append(h.linked, input)

			return entity.CodeLink{ID: id, URL: input.URL, Kind: entity.CodeLinkChange}, nil
		}).
		AnyTimes()
}

func (h *harness) refusesLinks(cause error) {
	h.source.EXPECT().
		Link(gomock.Any(), h.workspaceID, h.issue.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.LinkIssueCodeInput,
		) (entity.CodeLink, error) {
			h.linked = append(h.linked, input)

			return entity.CodeLink{}, cause
		}).
		AnyTimes()
}

func (h *harness) message(kind entity.ChannelMessageType, payload any) entity.ChannelMessage {
	return h.messageAt(kind, payload, time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC))
}

func (h *harness) messageAt(
	kind entity.ChannelMessageType,
	payload any,
	issuedAt time.Time,
) entity.ChannelMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return entity.ChannelMessage{
		ID:          uuid.NewString(),
		Type:        kind,
		ExecutionID: h.execution.ID,
		IssuedAt:    issuedAt,
		Payload:     body,
	}
}

func (h *harness) change(repositoryName string) (entity.ExecutionChange, bool) {
	for _, held := range h.changes {
		if held.Repository == repositoryName {
			return held, true
		}
	}

	return entity.ExecutionChange{}, false
}

func (h *harness) validation(check string) (entity.ExecutionValidation, bool) {
	for _, held := range h.validations {
		if held.Check == check {
			return held, true
		}
	}

	return entity.ExecutionValidation{}, false
}

func repoChange(name, branch string) channelv1.RepoChange {
	return channelv1.RepoChange{
		Repository: name,
		Branch:     branch,
		Commits:    3,
		Additions:  412,
		Deletions:  77,
		Files:      9,
	}
}
