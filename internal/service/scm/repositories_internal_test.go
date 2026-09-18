package scm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
	"github.com/usenorn/norn/internal/service"
	authorizermock "github.com/usenorn/norn/internal/service/authorizer"
)

type repositorySettingsHarness struct {
	service      *connections
	repositories *scmrepo.MockSCMRepository
	workspace    uuid.UUID
	stored       entity.SCMRepository
	saved        repository.SCMRepositorySettings
}

func repositorySettingsFor(t *testing.T, stored entity.SCMRepository) *repositorySettingsHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspace := uuid.New()
	authorizer := authorizermock.NewMockAuthorizer(ctrl)
	repositories := scmrepo.NewMockSCMRepository(ctrl)

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Role:      entity.MembershipRoleAdmin,
			Actor:     entity.Actor{AccountID: uuid.New()},
			Workspace: entity.Workspace{ID: workspace, Slug: "northwind"},
		}, nil).
		AnyTimes()

	stored.ID = uuid.New()
	stored.WorkspaceID = workspace

	harness := &repositorySettingsHarness{
		service:      &connections{authorizer: authorizer, repositories: repositories},
		repositories: repositories,
		workspace:    workspace,
		stored:       stored,
	}

	repositories.EXPECT().
		GetByID(gomock.Any(), workspace, stored.ID).
		Return(stored, nil).
		AnyTimes()

	repositories.EXPECT().
		UpdateSettings(gomock.Any(), stored.ID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			settings repository.SCMRepositorySettings,
		) (entity.SCMRepository, error) {
			harness.saved = settings

			return stored, nil
		}).
		AnyTimes()

	return harness
}

func (h *repositorySettingsHarness) update(
	t *testing.T,
	input service.UpdateRepositoryInput,
) repository.SCMRepositorySettings {
	t.Helper()

	if _, err := h.service.UpdateRepository(
		context.Background(), h.workspace, h.stored.ID, input,
	); err != nil {
		t.Fatalf("updating the repository: %v", err)
	}

	return h.saved
}

func TestDescribingAChangeIsOnlyTurnedOverWhenSomebodySaysSo(t *testing.T) {
	cases := []struct {
		name  string
		held  bool
		asked *bool
		want  bool
	}{
		{name: "left alone while off", held: false, asked: nil, want: false},
		{name: "left alone while on", held: true, asked: nil, want: true},
		{name: "turned on", held: false, asked: pointerTo(true), want: true},
		{name: "turned off", held: true, asked: pointerTo(false), want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := repositorySettingsFor(t, entity.SCMRepository{
				AnnounceDescription: testCase.held,
			})

			saved := h.update(t, service.UpdateRepositoryInput{
				AnnounceDescription: testCase.asked,
			})

			if saved.AnnounceDescription != testCase.want {
				t.Fatalf(
					"the repository was saved announcing descriptions = %t, want %t — an update "+
						"that says nothing about the description must leave it where it stood",
					saved.AnnounceDescription, testCase.want,
				)
			}
		})
	}
}

func TestTurningDescriptionsOnLeavesTheOtherRepositorySettingsWhereTheyWere(t *testing.T) {
	stored := entity.SCMRepository{
		MirrorLabel:      "northwind",
		SyncDirection:    entity.MirrorInbound,
		WebhooksDisabled: true,
		PollInterval:     17 * time.Minute,
	}

	h := repositorySettingsFor(t, stored)

	saved := h.update(t, service.UpdateRepositoryInput{
		AnnounceDescription: pointerTo(true),
	})

	if saved.MirrorLabel != stored.MirrorLabel {
		t.Errorf("mirror label saved as %q, want %q", saved.MirrorLabel, stored.MirrorLabel)
	}

	if saved.SyncDirection != stored.SyncDirection {
		t.Errorf("direction saved as %q, want %q", saved.SyncDirection, stored.SyncDirection)
	}

	if !saved.WebhooksDisabled {
		t.Error("the repository stopped polling, want the schedule it was left on")
	}

	if saved.PollInterval != stored.PollInterval {
		t.Errorf("interval saved as %s, want %s", saved.PollInterval, stored.PollInterval)
	}

	if !saved.AnnounceDescription {
		t.Error("the description was not turned on, want it on")
	}
}

func pointerTo[T any](value T) *T {
	return &value
}
